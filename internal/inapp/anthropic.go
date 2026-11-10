package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// anthropicClient speaks the Messages API, streamed. The system prompt and
// the tools are the same every turn, and the thread only grows, so the
// request marks them, and the thread's end, for the prompt cache: a turn
// pays full price only for what is new since the last.
type anthropicClient struct {
	model, key, base string
}

type anthBlock = map[string]any

func (c *anthropicClient) messages(items []item) []map[string]any {
	var msgs []map[string]any
	add := func(role string, b anthBlock) {
		if n := len(msgs); n > 0 && msgs[n-1]["role"] == role {
			msgs[n-1]["content"] = append(msgs[n-1]["content"].([]anthBlock), b)
			return
		}
		msgs = append(msgs, map[string]any{"role": role, "content": []anthBlock{b}})
	}
	for _, it := range items {
		switch it.Kind {
		case itemUser:
			add("user", anthBlock{"type": "text", "text": nonEmpty(it.Text)})
		case itemAssistant:
			if strings.TrimSpace(it.Text) != "" {
				add("assistant", anthBlock{"type": "text", "text": it.Text})
			}
			for _, call := range it.Calls {
				args := call.Args
				if args == nil {
					args = map[string]any{}
				}
				add("assistant", anthBlock{"type": "tool_use", "id": call.ID, "name": call.Name, "input": args})
			}
			if strings.TrimSpace(it.Text) == "" && len(it.Calls) == 0 {
				add("assistant", anthBlock{"type": "text", "text": "…"})
			}
		case itemResult:
			add("user", anthBlock{"type": "tool_result", "tool_use_id": it.CallID, "content": nonEmpty(it.Output)})
		}
	}
	// The cache breakpoint at the thread's end.
	if n := len(msgs); n > 0 {
		blocks := msgs[n-1]["content"].([]anthBlock)
		last := anthBlock{}
		for k, v := range blocks[len(blocks)-1] {
			last[k] = v
		}
		last["cache_control"] = map[string]string{"type": "ephemeral"}
		blocks[len(blocks)-1] = last
	}
	return msgs
}

func nonEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(empty)"
	}
	return s
}

func (c *anthropicClient) turn(ctx context.Context, req turnRequest, h turnHandlers) (turnResult, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	body := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"stream":     true,
		"system":     []anthBlock{{"type": "text", "text": req.System, "cache_control": map[string]string{"type": "ephemeral"}}},
		"messages":   c.messages(req.Items),
	}
	if len(req.Tools) > 0 {
		tools := make([]anthBlock, 0, len(req.Tools))
		for i, t := range req.Tools {
			b := anthBlock{"name": t.Name, "description": t.Description, "input_schema": t.Schema}
			if i == len(req.Tools)-1 {
				b["cache_control"] = map[string]string{"type": "ephemeral"}
			}
			tools = append(tools, b)
		}
		body["tools"] = tools
	}
	headers := map[string]string{"x-api-key": c.key, "anthropic-version": "2023-06-01"}
	return streamTurn(ctx, h, func(said *bool) (turnResult, error) {
		res, err := postStream(ctx, c.base+"/v1/messages", headers, body, retryPolicy{attempts: 1}, nil)
		if err != nil {
			return turnResult{}, err
		}
		defer res.Body.Close()
		return c.read(ctx, res.Body, h, said)
	})
}

// read gathers one streamed reply: text blocks as they stream, tool_use
// blocks from their partial JSON.
func (c *anthropicClient) read(ctx context.Context, body io.ReadCloser, h turnHandlers, said *bool) (turnResult, error) {
	var out turnResult
	type block struct {
		kind, id, name string
		text, json     strings.Builder
	}
	blocks := map[int]*block{}
	var order []int
	done := false
	err := readSSE(ctx, body, func(event, data string) error {
		var ev struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Block struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					Input      int `json:"input_tokens"`
					Output     int `json:"output_tokens"`
					CacheRead  int `json:"cache_read_input_tokens"`
					CacheWrite int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				Output int `json:"output_tokens"`
			} `json:"usage"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			out.Usage.add(usage{Input: u.Input, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite})
		case "content_block_start":
			b := &block{kind: ev.Block.Type, id: ev.Block.ID, name: ev.Block.Name}
			b.text.WriteString(ev.Block.Text)
			blocks[ev.Index] = b
			order = append(order, ev.Index)
		case "content_block_delta":
			b := blocks[ev.Index]
			if b == nil {
				return nil
			}
			switch ev.Delta.Type {
			case "text_delta":
				b.text.WriteString(ev.Delta.Text)
				if ev.Delta.Text != "" {
					*said = true
					if h.text != nil {
						h.text(ev.Delta.Text)
					}
				}
			case "input_json_delta":
				b.json.WriteString(ev.Delta.PartialJSON)
			}
		case "message_delta":
			out.Usage.Output += ev.Usage.Output
			if ev.Delta.StopReason == "max_tokens" {
				out.Truncated = true
			}
		case "message_stop":
			done = true
			return errStop
		case "error":
			// Overloaded mid-stream counts as the server's error, worth a try
			// when nothing was said.
			if ev.Error.Type == "overloaded_error" {
				return &httpError{status: 529, body: data}
			}
			return &httpError{status: 500, body: data}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if !done {
		return out, &streamError{fmt.Errorf("it ended before message_stop")}
	}
	var text []string
	for _, i := range order {
		b := blocks[i]
		switch b.kind {
		case "text":
			if t := b.text.String(); strings.TrimSpace(t) != "" {
				text = append(text, t)
			}
		case "tool_use":
			args := map[string]any{}
			if s := strings.TrimSpace(b.json.String()); s != "" {
				if err := json.Unmarshal([]byte(s), &args); err != nil {
					return out, fmt.Errorf("inapp: the model's arguments for %s are not JSON: %w", b.name, err)
				}
			}
			out.Calls = append(out.Calls, toolCall{ID: b.id, Name: b.name, Args: args})
		}
	}
	out.Text = strings.Join(text, "\n")
	return out, nil
}
