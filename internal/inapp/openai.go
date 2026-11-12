package inapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// openAIClient speaks Chat Completions, streamed: OpenAI's, and Ollama's
// OpenAI-compatible one. OpenAI caches a request's unchanged start on its
// own, which a thread that only grows keeps.
type openAIClient struct {
	url, key, model string
	// reasoningOff sends reasoning_effort "none": OpenAI's reasoning
	// models take tools on Chat Completions only so.
	reasoningOff bool
	// images says the model can be shown pictures: a screenshot is sent
	// to it, and only named to a model that cannot.
	images bool
}

func (c *openAIClient) messages(system string, items []item) []map[string]any {
	msgs := []map[string]any{{"role": "system", "content": system}}
	// A tool's answer is text only on Chat Completions, and the answers
	// to one turn's calls must follow each other: the pictures they came
	// with go in a user message after the last of them.
	var pics []map[string]any
	flush := func() {
		if len(pics) > 0 {
			content := append([]map[string]any{{"type": "text", "text": "The screenshot the tool result above came with:"}}, pics...)
			msgs = append(msgs, map[string]any{"role": "user", "content": content})
			pics = nil
		}
	}
	for _, it := range items {
		if it.Kind != itemResult {
			flush()
		}
		switch it.Kind {
		case itemUser:
			msgs = append(msgs, map[string]any{"role": "user", "content": nonEmpty(it.Text)})
		case itemAssistant:
			m := map[string]any{"role": "assistant"}
			if strings.TrimSpace(it.Text) != "" || len(it.Calls) == 0 {
				m["content"] = nonEmpty(it.Text)
			}
			if len(it.Calls) > 0 {
				calls := make([]map[string]any, 0, len(it.Calls))
				for _, call := range it.Calls {
					calls = append(calls, map[string]any{
						"id": call.ID, "type": "function",
						"function": map[string]any{"name": call.Name, "arguments": call.argsJSON()},
					})
				}
				m["tool_calls"] = calls
			}
			msgs = append(msgs, m)
		case itemResult:
			out := nonEmpty(it.Output)
			switch {
			case it.Image != nil && c.images:
				pics = append(pics, map[string]any{"type": "image_url",
					"image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(it.Image)}})
			case it.Image != nil:
				out += "\n[this model is not shown pictures: go by the elements and the text above]"
			}
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": it.CallID, "content": out})
		}
	}
	flush()
	return msgs
}

func (c *openAIClient) turn(ctx context.Context, req turnRequest, h turnHandlers) (turnResult, error) {
	body := map[string]any{
		"model":          c.model,
		"messages":       c.messages(req.System, req.Items),
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type":     "function",
				"function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Schema},
			})
		}
		body["tools"] = tools
		body["tool_choice"] = "auto"
		body["parallel_tool_calls"] = true
		if c.reasoningOff {
			body["reasoning_effort"] = "none"
		}
	}
	if req.MaxTokens > 0 {
		// OpenAI's reasoning models refuse max_tokens; Ollama knows only it.
		if c.reasoningOff {
			body["max_completion_tokens"] = req.MaxTokens
		} else {
			body["max_tokens"] = req.MaxTokens
		}
	}
	headers := map[string]string{}
	if c.key != "" {
		headers["Authorization"] = "Bearer " + c.key
	}
	return streamTurn(ctx, h, func(said *bool) (turnResult, error) {
		res, err := postStream(ctx, c.url, headers, body, retryPolicy{attempts: 1}, nil)
		if err != nil {
			return turnResult{}, err
		}
		defer res.Body.Close()
		return c.read(ctx, res.Body, h, said)
	})
}

func (c *openAIClient) read(ctx context.Context, body io.ReadCloser, h turnHandlers, said *bool) (turnResult, error) {
	var out turnResult
	var text strings.Builder
	type call struct {
		id, name string
		args     strings.Builder
	}
	calls := map[int]*call{}
	finished := false
	err := readSSE(ctx, body, func(_, data string) error {
		if data == "[DONE]" {
			finished = true
			return errStop
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				Prompt     int `json:"prompt_tokens"`
				Completion int `json:"completion_tokens"`
				Details    struct {
					Cached int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil
		}
		if ev.Error != nil && ev.Error.Message != "" {
			return fmt.Errorf("%s", ev.Error.Message)
		}
		if ev.Usage != nil {
			out.Usage.add(usage{Input: ev.Usage.Prompt - ev.Usage.Details.Cached, CacheRead: ev.Usage.Details.Cached, Output: ev.Usage.Completion})
		}
		for _, ch := range ev.Choices {
			if d := ch.Delta.Content; d != "" {
				text.WriteString(d)
				*said = true
				if h.text != nil {
					h.text(d)
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				cl := calls[tc.Index]
				if cl == nil {
					cl = &call{}
					calls[tc.Index] = cl
				}
				if tc.ID != "" {
					cl.id = tc.ID
				}
				if tc.Function.Name != "" {
					cl.name += tc.Function.Name
				}
				cl.args.WriteString(tc.Function.Arguments)
			}
			switch ch.FinishReason {
			case "":
			case "length":
				out.Truncated = true
				finished = true
			default:
				finished = true
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
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for n, i := range idx {
		cl := calls[i]
		args := map[string]any{}
		if s := strings.TrimSpace(cl.args.String()); s != "" {
			if err := json.Unmarshal([]byte(s), &args); err != nil {
				return out, fmt.Errorf("inapp: the model's arguments for %s are not JSON: %w", cl.name, err)
			}
		}
		id := cl.id
		if id == "" {
			id = fmt.Sprintf("call_%d", n)
		}
		out.Calls = append(out.Calls, toolCall{ID: id, Name: cl.name, Args: args})
	}
	out.Text = strings.TrimSpace(text.String())
	return out, nil
}
