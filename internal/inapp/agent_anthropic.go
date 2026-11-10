package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func runAnthropicAgent(ctx context.Context, cfg AgentConfig, system string, tools []agentTool, phone *PhoneTools, device string, ev *events) AgentOutcome {
	anthTools := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		anthTools = append(anthTools, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Schema,
		})
	}
	type block map[string]any
	msgs := make([]map[string]any, 0, len(cfg.History)+1)
	for _, h := range historyTurns(cfg.History) {
		msgs = append(msgs, map[string]any{"role": h.Role, "content": h.Text})
	}
	msgs = append(msgs, map[string]any{"role": "user", "content": cfg.Prompt})
	for turn := 0; turn < agentMaxTurns; turn++ {
		if ctx.Err() != nil {
			return AgentOutcome{Err: ctx.Err()}
		}
		body := map[string]any{
			"model":      cfg.ModelID,
			"max_tokens": 4096,
			"system":     system,
			"tools":      anthTools,
			"messages":   msgs,
		}
		raw, err := postJSON(ctx, "https://api.anthropic.com/v1/messages", cfg.APIKey, map[string]string{
			"x-api-key":         cfg.APIKey,
			"anthropic-version": "2023-06-01",
		}, body)
		if err != nil {
			return AgentOutcome{Err: err}
		}
		var out struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
			StopReason string `json:"stop_reason"`
			Error      *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return AgentOutcome{Err: err}
		}
		if out.Error != nil && out.Error.Message != "" {
			return AgentOutcome{Err: fmt.Errorf("%s", out.Error.Message)}
		}
		var textParts []string
		var toolResults []block
		assistantContent := make([]block, 0, len(out.Content))
		withTools := false
		for _, c := range out.Content {
			withTools = withTools || c.Type == "tool_use"
		}
		for _, c := range out.Content {
			switch c.Type {
			case "text":
				if strings.TrimSpace(c.Text) != "" {
					textParts = append(textParts, c.Text)
				}
				// Said alongside tool calls, it is the agent thinking aloud;
				// said alone, it is the reply, which the outcome carries.
				if withTools {
					ev.text(c.Text)
				}
				assistantContent = append(assistantContent, block{"type": "text", "text": c.Text})
			case "tool_use":
				var input any
				_ = json.Unmarshal(c.Input, &input)
				assistantContent = append(assistantContent, block{
					"type": "tool_use", "id": c.ID, "name": c.Name, "input": input,
				})
				args, err := parseToolArgs(c.Input)
				if err != nil {
					return AgentOutcome{Err: err}
				}
				if args == nil {
					args = map[string]any{}
				}
				if strings.TrimSpace(str(args, "device")) == "" {
					args["device"] = device
				}
				id := ev.start(c.ID, c.Name, args)
				result := phone.Call(ctx, c.Name, args)
				ev.end(id, c.Name, result)
				if c.Name == "phone_release" {
					summary := str(args, "summary")
					if summary == "" {
						summary = strings.Join(textParts, "\n")
					}
					if summary == "" {
						summary = "Task finished."
					}
					return AgentOutcome{Reply: summary}
				}
				toolResults = append(toolResults, block{
					"type": "tool_result", "tool_use_id": c.ID, "content": result,
				})
			}
		}
		msgs = append(msgs, map[string]any{"role": "assistant", "content": assistantContent})
		if len(toolResults) == 0 {
			reply := strings.TrimSpace(strings.Join(textParts, "\n"))
			if reply == "" {
				reply = "Done."
			}
			return AgentOutcome{Reply: reply}
		}
		msgs = append(msgs, map[string]any{"role": "user", "content": toolResults})
		if out.StopReason == "end_turn" && len(toolResults) == 0 {
			break
		}
	}
	return AgentOutcome{Reply: "Stopped after many steps. Say if you want me to continue."}
}
