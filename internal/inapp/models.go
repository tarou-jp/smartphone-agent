package inapp

// ChatModel is one LLM the in-app chat can call with a provider API key.
type ChatModel struct {
	ID       string
	Provider Provider
	Label    string
}

// ChatModels are the models offered in the composer menu (newest first).
var ChatModels = []ChatModel{
	{ID: "claude-sonnet-4-6", Provider: Anthropic, Label: "Claude Sonnet 4.6"},
	{ID: "claude-haiku-4-5", Provider: Anthropic, Label: "Claude Haiku 4.5"},
	{ID: "claude-opus-4-6", Provider: Anthropic, Label: "Claude Opus 4.6"},
	{ID: "gpt-6-sol", Provider: OpenAI, Label: "GPT-6 Sol"},
	{ID: "gpt-6.1-terra", Provider: OpenAI, Label: "GPT-6.1 Terra"},
	{ID: "gpt-6-luna", Provider: OpenAI, Label: "GPT-6 Luna"},
	{ID: "gpt-5.6-sol", Provider: OpenAI, Label: "GPT-5.6 Sol"},
	{ID: "gpt-5.6-terra", Provider: OpenAI, Label: "GPT-5.6 Terra"},
}

// chatModelMigrations maps retired menu ids to their replacements.
var chatModelMigrations = map[string]string{
	"claude-sonnet-4-5": "claude-sonnet-4-6",
	"gpt-4.1":           "gpt-6.1-terra",
	"gpt-4.1-mini":      "gpt-6-luna",
	"gpt-4o":            "gpt-5.6-terra",
}

// ChatModelByID finds a known model id.
func ChatModelByID(id string) *ChatModel {
	if id != "" {
		if to, ok := chatModelMigrations[id]; ok {
			id = to
		}
	}
	for i := range ChatModels {
		if ChatModels[i].ID == id {
			return &ChatModels[i]
		}
	}
	return nil
}

// DefaultChatModel is the composer default when nothing is saved yet.
func DefaultChatModel() ChatModel { return ChatModels[0] }

// NormalizeChatModelID returns a current menu id, migrating or replacing unknown ids.
func NormalizeChatModelID(id string) string {
	if id == "" {
		return DefaultChatModel().ID
	}
	if to, ok := chatModelMigrations[id]; ok {
		id = to
	}
	for i := range ChatModels {
		if ChatModels[i].ID == id {
			return id
		}
	}
	return DefaultChatModel().ID
}
