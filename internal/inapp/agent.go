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
	Gateway      *gateway.Gateway
	Provider     Provider
	ModelID      string
	APIKey       string
	OllamaBase   string
	DeviceID     string
	DeviceName   string
	TaskTitle    string
	Prompt       string
	LocaleIsJA   bool
}

// AgentEvent reports progress for the UI.
type AgentEvent struct {
	Text string
}

// AgentOutcome is the final assistant text or error.
type AgentOutcome struct {
	Reply string
	Err   error
}

// RunAgent drives the phone through gateway tools until the model stops or phone_release.
func RunAgent(ctx context.Context, cfg AgentConfig, onEvent func(AgentEvent)) AgentOutcome {
	emit := func(s string) {
		if onEvent != nil && strings.TrimSpace(s) != "" {
			onEvent(AgentEvent{Text: s})
		}
	}
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

	emit("phone_connect…")
	connectArgs := map[string]any{"device": dev, "task": task}
	connectOut := phone.Call(ctx, "phone_connect", connectArgs)
	emit(shortStatus(connectOut))
	if strings.HasPrefix(connectOut, "error:") {
		return AgentOutcome{Reply: connectOut}
	}

	switch cfg.Provider {
	case Anthropic:
		return runAnthropicAgent(ctx, cfg, sys, tools, phone, dev, emit)
	case Ollama:
		if cfg.OllamaBase == "" {
			cfg.OllamaBase = DefaultOllamaBaseURL
		}
		return runOpenAIAgent(ctx, openAIAgentCfg{
			url:    NormalizeOllamaBaseURL(cfg.OllamaBase) + "/v1/chat/completions",
			apiKey: "ollama",
			model:  strings.TrimPrefix(cfg.ModelID, ollamaModelPrefix),
		}, sys, tools, phone, dev, cfg.Prompt, emit)
	case OpenAI:
		return runOpenAIAgent(ctx, openAIAgentCfg{
			url:    "https://api.openai.com/v1/chat/completions",
			apiKey: cfg.APIKey,
			model:  cfg.ModelID,
		}, sys, tools, phone, dev, cfg.Prompt, emit)
	default:
		return AgentOutcome{Err: fmt.Errorf("inapp: agent needs Anthropic, OpenAI, or Ollama with tool support")}
	}
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
