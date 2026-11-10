package inapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/mcp"
)

const agentMaxTurns = 48

// AgentConfig is one in-app agent run (same tools as MCP / Codex).
type AgentConfig struct {
	Gateway    *gateway.Gateway
	Provider   Provider
	ModelID    string
	APIKey     string
	OllamaBase string
	DeviceID   string
	DeviceName string
	TaskTitle  string
	Prompt     string
	// History is the chat so far, oldest first, so a follow-up keeps its
	// context. The agent sees it before Prompt.
	History    []Message
	LocaleIsJA bool
}

// AgentEvent reports progress for the UI. Text is always set: one line
// for clients that show a plain log. Kind says what happened, for clients
// that draw each tool call as a row: "tool_start" and "tool_end" share ID,
// "text" is something the model said between tool calls.
type AgentEvent struct {
	Text string `json:"text"`

	Kind   string         `json:"kind,omitempty"`
	ID     string         `json:"id,omitempty"`
	Tool   string         `json:"tool,omitempty"`
	Intent string         `json:"intent,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Status string         `json:"status,omitempty"` // tool_end: the result's status, or "error: <code>"
	Detail string         `json:"detail,omitempty"` // tool_end: the result's first sentence
}

// events turns a run's progress into AgentEvents for onEvent.
type events struct {
	on  func(AgentEvent)
	seq int
}

func (e *events) send(ev AgentEvent) {
	if e != nil && e.on != nil && strings.TrimSpace(ev.Text) != "" {
		e.on(ev)
	}
}

// text reports what the model said between tool calls.
func (e *events) text(s string) {
	if s = strings.TrimSpace(s); s != "" {
		e.send(AgentEvent{Kind: "text", Text: s})
	}
}

// start reports a tool call about to run and returns its id for end.
func (e *events) start(id, tool string, args map[string]any) string {
	if id == "" {
		e.seq++
		id = fmt.Sprintf("t%d", e.seq)
	}
	shown := make(map[string]any, len(args))
	for k, v := range args {
		if k != "device" && k != "intent" && k != "approval_id" {
			shown[k] = v
		}
	}
	e.send(AgentEvent{Kind: "tool_start", ID: id, Tool: tool, Intent: strings.TrimSpace(str(args, "intent")), Args: shown, Text: toolCallLabel(tool)})
	return id
}

// end reports a tool call's result, as FormatResult wrote it.
func (e *events) end(id, tool, result string) {
	first, rest, _ := strings.Cut(result, "\n")
	status, _, _ := strings.Cut(first, " · ")
	detail, _, _ := strings.Cut(strings.TrimSpace(rest), "\n")
	e.send(AgentEvent{Kind: "tool_end", ID: id, Tool: tool, Status: strings.TrimSpace(status), Detail: detail, Text: shortStatus(result)})
}

// AgentOutcome is the final assistant text or error.
type AgentOutcome struct {
	Reply string
	Err   error
}

// RunAgent drives the phone through gateway tools until the model stops or phone_release.
func RunAgent(ctx context.Context, cfg AgentConfig, onEvent func(AgentEvent)) AgentOutcome {
	ev := &events{on: onEvent}
	if cfg.Gateway == nil {
		return AgentOutcome{Err: fmt.Errorf("inapp: no gateway")}
	}
	dev := strings.TrimSpace(cfg.DeviceID)
	if dev == "" {
		return AgentOutcome{Err: fmt.Errorf("inapp: no phone selected")}
	}
	tools, err := agentTools()
	if err != nil {
		return AgentOutcome{Err: err}
	}
	sess := cfg.Gateway.OpenSession(gateway.AgentInfo{Name: "Pinky", Client: "in-app-chat"})
	defer sess.Close()
	phone := &PhoneTools{Sess: sess}

	task := strings.TrimSpace(cfg.TaskTitle)
	if task == "" {
		task = taskFromPrompt(cfg.Prompt)
	}
	sys := mcp.Instructions + "\n\nYou are Pinky's built-in chat agent. Work on one phone per tool call; pass device: \"" + dev + "\" on every phone_* call."
	if cfg.DeviceName != "" {
		sys += " The phone's name is " + cfg.DeviceName + "."
	}
	if cfg.LocaleIsJA {
		sys += "\nReply to the person in Japanese."
	}
	sys += "\n\nFirst call phone_connect with task \"" + task + "\" and device \"" + dev + "\". End with phone_release when done."

	connectArgs := map[string]any{"device": dev, "task": task}
	cid := ev.start("", "phone_connect", connectArgs)
	connectOut := phone.Call(ctx, "phone_connect", connectArgs)
	ev.end(cid, "phone_connect", connectOut)
	if strings.HasPrefix(connectOut, "error:") {
		return AgentOutcome{Reply: connectOut}
	}

	switch cfg.Provider {
	case Anthropic:
		return runAnthropicAgent(ctx, cfg, sys, tools, phone, dev, ev)
	case Ollama:
		if cfg.OllamaBase == "" {
			cfg.OllamaBase = DefaultOllamaBaseURL
		}
		return runOpenAIAgent(ctx, openAIAgentCfg{
			url:    NormalizeOllamaBaseURL(cfg.OllamaBase) + "/v1/chat/completions",
			apiKey: "ollama",
			model:  strings.TrimPrefix(cfg.ModelID, ollamaModelPrefix),
		}, sys, tools, phone, dev, cfg.History, cfg.Prompt, ev)
	case OpenAI:
		return runOpenAIAgent(ctx, openAIAgentCfg{
			url:    "https://api.openai.com/v1/chat/completions",
			apiKey: cfg.APIKey,
			model:  cfg.ModelID,
		}, sys, tools, phone, dev, cfg.History, cfg.Prompt, ev)
	default:
		return AgentOutcome{Err: fmt.Errorf("inapp: agent needs Anthropic, OpenAI, or Ollama with tool support")}
	}
}

// historyTurns keeps the user and assistant turns of a chat with text,
// and drops a leading assistant turn, since a conversation must start with
// the person.
func historyTurns(h []Message) []Message {
	out := make([]Message, 0, len(h))
	for _, m := range h {
		if (m.Role != "user" && m.Role != "assistant") || strings.TrimSpace(m.Text) == "" {
			continue
		}
		if len(out) == 0 && m.Role != "user" {
			continue
		}
		// Two turns of one role in a row (a run that failed) merge.
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Text += "\n\n" + m.Text
			continue
		}
		out = append(out, m)
	}
	// The prompt follows as a user turn: end on the assistant's.
	if n := len(out); n > 0 && out[n-1].Role == "user" {
		out = append(out, Message{Role: "assistant", Text: "(No reply.)"})
	}
	return out
}

func taskFromPrompt(p string) string {
	return chatTitleFromPrompt(p)
}

func chatTitleFromPrompt(p string) string {
	p = strings.Join(strings.Fields(p), " ")
	r := []rune(p)
	if len(r) > 48 {
		return string(r[:47]) + "…"
	}
	if p == "" {
		return "In-app task"
	}
	return p
}

func shortStatus(toolOut string) string {
	line, _, _ := strings.Cut(toolOut, "\n")
	if len(line) > 120 {
		return line[:119] + "…"
	}
	return line
}
