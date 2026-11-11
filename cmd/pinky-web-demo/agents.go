package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/DIYgod/pinky/internal/agents"
)

// demoAgents is Settings › Agents › Connect an agent for the demo: a few
// agents in each state, kept in memory. Nothing on this computer changes.
type demoAgents struct {
	mu   sync.Mutex
	list []agents.Status
}

func newDemoAgents() *demoAgents {
	return &demoAgents{list: []agents.Status{
		{ID: "claude-code", Name: "Claude Code", Found: true, State: agents.Connected, Restart: agents.RestartSession},
		{ID: "claude-desktop", Name: "Claude Desktop", Found: true, Restart: agents.RestartApp, Opens: true},
		{ID: "codex", Name: "Codex", Found: true, Restart: agents.RestartSession},
		{ID: "cursor", Name: "Cursor", Found: true, State: agents.Outdated, Restart: agents.RestartApp},
		{ID: "vscode", Name: "VS Code", Found: true, Restart: agents.RestartApp},
		{ID: "copilot", Name: "Copilot CLI", URL: "https://github.com/features/copilot/cli"},
		{ID: "windsurf", Name: "Windsurf", URL: "https://windsurf.com/download"},
		{ID: "gemini", Name: "Gemini CLI", URL: "https://github.com/google-gemini/gemini-cli"},
	}}
}

func (d *demoAgents) Find(context.Context) []agents.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]agents.Status(nil), d.list...)
}

func (d *demoAgents) set(id string, st agents.State) (agents.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.list {
		if d.list[i].ID == id {
			d.list[i].State = st
			return agents.Result{Restart: d.list[i].Restart}, nil
		}
	}
	return agents.Result{}, fmt.Errorf("no agent %q", id)
}

func (d *demoAgents) Connect(_ context.Context, id string) (agents.Result, error) {
	return d.set(id, agents.Connected)
}

func (d *demoAgents) Remove(_ context.Context, id string) (agents.Result, error) {
	return d.set(id, agents.NotConnected)
}
