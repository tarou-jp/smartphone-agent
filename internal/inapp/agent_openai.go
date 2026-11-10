package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type openAIAgentCfg struct {
	url, apiKey, model string
}

type oaiMsg struct {
	Role       string    `json:"role"`
	Content    any       `json:"content,omitempty"`
	ToolCalls  []oaiTool `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	Name       string    `json:"name,omitempty"`
}

type oaiTool struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiToolFunction `json:"function"`
}

type oaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func runOpenAIAgent(ctx context.Context, cfg openAIAgentCfg, system string, tools []agentTool, phone *PhoneTools, device string, history []Message, userPrompt string, ev *events) AgentOutcome {
	oaiTools := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		oaiTools = append(oaiTools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Schema,
			},
		})
	}
	msgs := []oaiMsg{{Role: "system", Content: system}}
	for _, h := range historyTurns(history) {
		msgs = append(msgs, oaiMsg{Role: h.Role, Content: h.Text})
	}
	msgs = append(msgs, oaiMsg{Role: "user", Content: userPrompt})
	for turn := 0; turn < agentMaxTurns; turn++ {
		if ctx.Err() != nil {
			return AgentOutcome{Err: ctx.Err()}
		}
		body := map[string]any{
			"model":       cfg.model,
			"messages":    msgs,
			"tools":       oaiTools,
			"tool_choice": "auto",
		}
		// Reasoning models (e.g. gpt-6-luna) reject tools unless effort is none on chat/completions.
		if strings.Contains(cfg.url, "api.openai.com") {
			body["reasoning_effort"] = "none"
		}
		raw, err := postJSON(ctx, cfg.url, cfg.apiKey, map[string]string{
			"Authorization": "Bearer " + cfg.apiKey,
		}, body)
		if err != nil {
			return AgentOutcome{Err: err}
		}
		var out struct {
			Choices []struct {
				Message struct {
					Role      string    `json:"role"`
					Content   string    `json:"content"`
					ToolCalls []oaiTool `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return AgentOutcome{Err: err}
		}
		if out.Error != nil && out.Error.Message != "" {
			return AgentOutcome{Err: fmt.Errorf("%s", out.Error.Message)}
		}
		if len(out.Choices) == 0 {
			return AgentOutcome{Err: fmt.Errorf("inapp: empty model reply")}
		}
		msg := out.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			text := strings.TrimSpace(msg.Content)
			if text == "" {
				text = "Done."
			}
			return AgentOutcome{Reply: text}
		}
		assist := oaiMsg{Role: "assistant", ToolCalls: msg.ToolCalls}
		if c := strings.TrimSpace(msg.Content); c != "" {
			assist.Content = c
		}
		msgs = append(msgs, assist)
		ev.text(msg.Content)
		for _, tc := range msg.ToolCalls {
			if tc.Type != "" && tc.Type != "function" {
				continue
			}
			name := tc.Function.Name
			args, err := parseToolArgs(json.RawMessage(tc.Function.Arguments))
			if err != nil {
				return AgentOutcome{Err: err}
			}
			if args == nil {
				args = map[string]any{}
			}
			if strings.TrimSpace(str(args, "device")) == "" {
				args["device"] = device
			}
			id := ev.start(tc.ID, name, args)
			result := phone.Call(ctx, name, args)
			ev.end(id, name, result)
			if name == "phone_release" {
				summary := str(args, "summary")
				if summary == "" {
					summary = strings.TrimSpace(msg.Content)
				}
				if summary == "" {
					summary = "Task finished."
				}
				return AgentOutcome{Reply: summary}
			}
			msgs = append(msgs, oaiMsg{Role: "tool", ToolCallID: tc.ID, Name: name, Content: result})
		}
	}
	return AgentOutcome{Reply: "Stopped after many steps. Say if you want me to continue."}
}
