package deskapi

import (
	"context"

	"github.com/DIYgod/pinky/internal/agents"
)

// Settings are what Settings changes, kept in settings.json. They moved here
// from the MyGo client (package ui), which keeps aliases to them, so both
// clients and cmd/pinky share one definition and one file format.

// Settings are what the Settings screen changes.
type Settings struct {
	Preset             int // 1: payments, messages and deletions; 2: payments only (the default, recommended); 3: every action
	BlockedApps        []BlockedApp
	ReadOnly           bool
	UnlockFromComputer bool
	HideCodes          bool
	KeepDays           int
	Record             bool
	DeveloperMode      bool
	// RemoteAgents serves MCP over Streamable HTTP at RemoteAddr, for agents
	// that cannot start `pinky mcp`; each request needs the agent token.
	RemoteAgents bool
	RemoteAddr   string
	// UsageStats shares anonymous usage statistics and crash reports
	// (docs/TELEMETRY.md).
	UsageStats bool
	// AutoUpdate installs new versions of Pinky, and of the companion app
	// on Android phones, by themselves while no one uses them.
	AutoUpdate bool
	// ModelProvider is read from older settings.json; migrated into ModelKeys.
	ModelProvider string `json:"modelProvider,omitempty"`
	// ChatModel is the inapp.ChatModels id used for API calls.
	ChatModel string
	// ChatAgentModel is the cloud model for phone control when ChatModel is Ollama without tools.
	ChatAgentModel string `json:"chatAgentModel,omitempty"`
	// ModelKeyActive is the id of the API key profile the chat uses.
	ModelKeyActive string
	// ModelKeys lists saved API key profiles (labels and providers). Secrets
	// live in the keychain at inapp.KeyAccount(provider, id).
	ModelKeys []ModelKeyProfile
	// OllamaBaseURL is the Ollama HTTP root (default inapp.DefaultOllamaBaseURL).
	OllamaBaseURL string `json:"ollamaBaseURL,omitempty"`
	// OllamaModels are model names from `ollama list` offered in chat.
	OllamaModels []string `json:"ollamaModels,omitempty"`
}

// ModelKeyProfile is one saved API key (per provider).
type ModelKeyProfile struct {
	ID       string `json:"id"`
	Provider string `json:"provider"` // anthropic or openai
	Label    string `json:"label"`
}

// DefaultRemoteAddr is where Pinky serves MCP over HTTP: this computer
// only, on port 7465 (PINK on a phone keypad).
const DefaultRemoteAddr = "127.0.0.1:7465"

// BlockedApp is an app agents may not open. ID is its package name or
// bundle id when Pinky knows it, so that unblocking takes the app out of
// every entry that blocks it, not only its name.
type BlockedApp struct {
	Name, Kind string
	ID         string
}

// AgentInfo is a row of Settings › Agents.
type AgentInfo struct {
	Name      string
	Scope     string // "Can use Dinely, Chatter and Calendar"
	Connected bool
}

// DefaultSettings are the design's defaults.
func DefaultSettings() Settings {
	return Settings{
		Preset: 2,
		BlockedApps: []BlockedApp{
			{Name: "Northbank", Kind: "Banking"}, {Name: "Vaultly", Kind: "Passwords"}, {Name: "Coinpouch", Kind: "Wallet"},
		},
		UnlockFromComputer: true, HideCodes: true, KeepDays: 30, Record: true, RemoteAddr: DefaultRemoteAddr,
		UsageStats: UsageStatsByDefault,
		AutoUpdate: true,
	}
}

// UsageStatsByDefault is whether usage statistics and crash reports start
// on. They are on, without a note on the first run; Settings › Privacy
// turns them off, and so does DO_NOT_TRACK (docs/TELEMETRY.md).
const UsageStatsByDefault = true

// AgentSetup finds the agents on this computer and adds Pinky to them, as
// internal/agents.Setup does. Its methods block: clients call them off the
// UI thread.
type AgentSetup interface {
	Find(ctx context.Context) []agents.Status
	Connect(ctx context.Context, id string) (agents.Result, error)
	Remove(ctx context.Context, id string) (agents.Result, error)
}

// Clone returns s with lists of its own.
func (s Settings) Clone() Settings {
	s.BlockedApps = append([]BlockedApp(nil), s.BlockedApps...)
	s.ModelKeys = append([]ModelKeyProfile(nil), s.ModelKeys...)
	s.OllamaModels = append([]string(nil), s.OllamaModels...)
	return s
}
