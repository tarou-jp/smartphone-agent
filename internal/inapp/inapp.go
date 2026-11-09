// Package inapp is the fork experiment for running an LLM from Pinky's UI
// and fanning one prompt out to several phones. See docs/notes/in-app-agent.md.
//
// Keys live in the OS keychain (internal/secret), never in Settings JSON.
// The agent loop must talk to the gateway's agent-facing surface so the
// person still approves, stops and takes over; it must not use gateway.API.
package inapp

import (
	"fmt"
	"strings"
)

// Provider is an LLM API Pinky can call with a saved key.
type Provider string

// Providers the MVP knows how to name. Wire HTTP clients in later commits.
const (
	Anthropic Provider = "anthropic"
	OpenAI    Provider = "openai"
	Ollama    Provider = "ollama"
)

// Providers is the order Settings should list them (API keys in Settings).
var Providers = []Provider{Anthropic, OpenAI}

// Valid reports whether p is a known provider.
func Valid(p Provider) bool {
	switch p {
	case Anthropic, OpenAI, Ollama:
		return true
	default:
		return false
	}
}

// Label is a short English name for Settings and logs.
func (p Provider) Label() string {
	switch p {
	case Anthropic:
		return "Anthropic"
	case OpenAI:
		return "OpenAI"
	case Ollama:
		return "Ollama"
	default:
		return string(p)
	}
}

// FanOut is one instruction to run on several phones the same way.
type FanOut struct {
	// Prompt is what the person typed: the goal for every phone.
	Prompt string
	// DeviceIDs are the phones to run on, each with its own task/lease.
	DeviceIDs []string
	// Provider selects which saved API key and HTTP API to use.
	Provider Provider
	// Title is the task name shown in the UI; empty means derived from Prompt.
	Title string
	// Agent is the agent name recorded on each task (default "Pinky").
	Agent string
	// ReadOnly asks the gateway for a read-only lease on every phone.
	ReadOnly bool
}

// Normalize trims fields and fills defaults. It does not talk to the network
// or the keychain.
func (f FanOut) Normalize() (FanOut, error) {
	f.Prompt = strings.TrimSpace(f.Prompt)
	f.Title = strings.TrimSpace(f.Title)
	f.Agent = strings.TrimSpace(f.Agent)
	if f.Agent == "" {
		f.Agent = "Pinky"
	}
	if f.Prompt == "" {
		return f, fmt.Errorf("inapp: empty prompt")
	}
	if len(f.DeviceIDs) == 0 {
		return f, fmt.Errorf("inapp: no phones selected")
	}
	if !Valid(f.Provider) {
		return f, fmt.Errorf("inapp: unknown provider %q", f.Provider)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(f.DeviceIDs))
	for _, id := range f.DeviceIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return f, fmt.Errorf("inapp: no phones selected")
	}
	f.DeviceIDs = out
	if f.Title == "" {
		f.Title = titleFromPrompt(f.Prompt)
	}
	return f, nil
}

// TaskTitle returns the per-phone task title: shared title plus device name.
func TaskTitle(shared, deviceName string) string {
	shared = strings.TrimSpace(shared)
	deviceName = strings.TrimSpace(deviceName)
	switch {
	case shared == "" && deviceName == "":
		return "In-app task"
	case deviceName == "":
		return shared
	case shared == "":
		return deviceName
	default:
		return shared + " · " + deviceName
	}
}

func titleFromPrompt(prompt string) string {
	const max = 48
	p := strings.Join(strings.Fields(prompt), " ")
	if len([]rune(p)) <= max {
		return p
	}
	r := []rune(p)
	return string(r[:max-1]) + "…"
}
