package inapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// openAIResponsesClient speaks OpenAI's Responses API, streamed. It is the
// one to use with OpenAI's models, for two things Chat Completions cannot
// do there: a tool's answer may hold a picture (the screen, when the
// model asked to see it), where Chat Completions takes text only and the
// picture had to follow as if the person had sent it; and the model
// reasons while it uses tools, where Chat Completions makes reasoning
// models choose.
//
// The thread is sent whole each turn (store: false), like the other
// clients' threads, so Pinky's own trimming and compaction apply. What
// the model put out in a turn, its reasoning included (encrypted), goes
// back as it came (item.Raw): the API wants a call's reasoning next to
// the call.
type openAIResponsesClient struct {
	url, key, model string
	// fallback is the Chat Completions client, used for the rest of the
	// run once the Responses API refuses a request outright: a model or
	// an account without it should not leave the person without an agent.
	fallback *openAIClient
	mu       sync.Mutex
	fellBack bool
}

func (c *openAIResponsesClient) input(items []item) []any {
	var in []any
	for _, it := range items {
		switch it.Kind {
		case itemUser:
			in = append(in, map[string]any{"role": "user", "content": []map[string]any{{"type": "input_text", "text": nonEmpty(it.Text)}}})
		case itemAssistant:
			if len(it.Raw) > 0 {
				for _, raw := range it.Raw {
					in = append(in, raw)
				}
				continue
			}
			if strings.TrimSpace(it.Text) != "" || len(it.Calls) == 0 {
				in = append(in, map[string]any{"role": "assistant", "content": []map[string]any{{"type": "output_text", "text": nonEmpty(it.Text)}}})
			}
			for _, call := range it.Calls {
				in = append(in, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.argsJSON()})
			}
		case itemResult:
			var out any = nonEmpty(it.Output)
			if it.Image != nil {
				// The picture of the screen is part of the tool's answer.
				out = []map[string]any{
					{"type": "input_text", "text": nonEmpty(it.Output)},
					{"type": "input_image", "image_url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(it.Image)},
				}
			}
			in = append(in, map[string]any{"type": "function_call_output", "call_id": it.CallID, "output": out})
		}
	}
	return in
}

func (c *openAIResponsesClient) turn(ctx context.Context, req turnRequest, h turnHandlers) (turnResult, error) {
	c.mu.Lock()
	fell := c.fellBack
	c.mu.Unlock()
	if fell {
		return c.fallback.turn(ctx, req, h)
	}
	body := map[string]any{
		"model":        c.model,
		"instructions": req.System,
		"input":        c.input(req.Items),
		"stream":       true,
		"store":        false,
		"include":      []string{"reasoning.encrypted_content"},
		"reasoning":    map[string]any{"effort": "low"},
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			// Not strict: the tools' arguments are optional, and a strict
			// call sends every one of them, null where it has no value.
			tools = append(tools, map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": t.Schema, "strict": false})
		}
		body["tools"] = tools
		body["tool_choice"] = "auto"
		body["parallel_tool_calls"] = true
	}
	if req.MaxTokens > 0 {
		body["max_output_tokens"] = req.MaxTokens
	}
	headers := map[string]string{}
	if c.key != "" {
		headers["Authorization"] = "Bearer " + c.key
	}
	anySaid := false
	res, err := streamTurn(ctx, h, func(said *bool) (turnResult, error) {
		res, err := postStream(ctx, c.url, headers, body, retryPolicy{attempts: 1}, nil)
		if err != nil {
			return turnResult{}, err
		}
		defer res.Body.Close()
		out, err := c.read(ctx, res.Body, h, said)
		anySaid = anySaid || *said
		return out, err
	})
	if err != nil && !anySaid && ctx.Err() == nil && c.fallback != nil && !retryable(err) {
		// Refused, and not for a reason that waiting mends: the other API.
		c.mu.Lock()
		c.fellBack = true
		c.mu.Unlock()
		return c.fallback.turn(ctx, req, h)
	}
	return res, err
}

func (c *openAIResponsesClient) read(ctx context.Context, body io.ReadCloser, h turnHandlers, said *bool) (turnResult, error) {
	var out turnResult
	var text strings.Builder
	finished := false
	err := readSSE(ctx, body, func(_, data string) error {
		var ev struct {
			Type    string          `json:"type"`
			Delta   string          `json:"delta"`
			Message string          `json:"message"`
			Item    json.RawMessage `json:"item"`
			Resp    *struct {
				Status string `json:"status"`
				Usage  *struct {
					Input   int `json:"input_tokens"`
					Output  int `json:"output_tokens"`
					Details struct {
						Cached int `json:"cached_tokens"`
					} `json:"input_tokens_details"`
				} `json:"usage"`
				Incomplete *struct {
					Reason string `json:"reason"`
				} `json:"incomplete_details"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil
		}
		switch ev.Type {
		case "response.output_text.delta":
			if ev.Delta != "" {
				text.WriteString(ev.Delta)
				*said = true
				if h.text != nil {
					h.text(ev.Delta)
				}
			}
		case "response.output_item.done":
			if len(ev.Item) == 0 {
				return nil
			}
			out.Raw = append(out.Raw, append(json.RawMessage(nil), ev.Item...))
			var it struct {
				Type, Name, Arguments string
				CallID                string `json:"call_id"`
			}
			if json.Unmarshal(ev.Item, &it) != nil || it.Type != "function_call" {
				return nil
			}
			args := map[string]any{}
			if s := strings.TrimSpace(it.Arguments); s != "" {
				if err := json.Unmarshal([]byte(s), &args); err != nil {
					return fmt.Errorf("inapp: the model's arguments for %s are not JSON: %w", it.Name, err)
				}
			}
			id := it.CallID
			if id == "" {
				id = fmt.Sprintf("call_%d", len(out.Calls))
			}
			out.Calls = append(out.Calls, toolCall{ID: id, Name: it.Name, Args: args})
		case "response.completed", "response.incomplete":
			finished = true
			if ev.Resp != nil {
				if u := ev.Resp.Usage; u != nil {
					out.Usage.add(usage{Input: u.Input - u.Details.Cached, CacheRead: u.Details.Cached, Output: u.Output})
				}
				if ev.Resp.Incomplete != nil && ev.Resp.Incomplete.Reason == "max_output_tokens" {
					out.Truncated = true
				}
			}
			return errStop
		case "response.failed":
			if ev.Resp != nil && ev.Resp.Error != nil && ev.Resp.Error.Message != "" {
				return fmt.Errorf("%s", ev.Resp.Error.Message)
			}
			return fmt.Errorf("the model's reply failed")
		case "error":
			if ev.Message != "" {
				return fmt.Errorf("%s", ev.Message)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if !finished {
		return out, &streamError{fmt.Errorf("it ended before the reply finished")}
	}
	out.Text = text.String()
	return out, nil
}
