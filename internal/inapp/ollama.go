package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultOllamaBaseURL is the usual Ollama listen address.
const DefaultOllamaBaseURL = "http://127.0.0.1:11434"

const ollamaModelPrefix = "ollama:"

// OllamaChatModelID is the settings id for an Ollama model name from `ollama list`.
func OllamaChatModelID(name string) string {
	return ollamaModelPrefix + strings.TrimSpace(name)
}

// OllamaModelName parses a chat model id from OllamaChatModelID.
func OllamaModelName(id string) (string, bool) {
	if !strings.HasPrefix(id, ollamaModelPrefix) {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimPrefix(id, ollamaModelPrefix))
	if name == "" {
		return "", false
	}
	return name, true
}

// OllamaChatModels builds composer entries for names saved in Settings.
func OllamaChatModels(names []string) []ChatModel {
	out := make([]ChatModel, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, ChatModel{
			ID:       OllamaChatModelID(n),
			Provider: Ollama,
			Label:    n,
		})
	}
	return out
}

// NormalizeOllamaBaseURL trims and drops a trailing slash.
func NormalizeOllamaBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return DefaultOllamaBaseURL
	}
	return strings.TrimRight(base, "/")
}

// CompleteOllama calls Ollama's OpenAI-compatible chat API (POST /v1/chat/completions).
func CompleteOllama(ctx context.Context, baseURL, model, system string, history []Message) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("inapp: empty Ollama model")
	}
	baseURL = NormalizeOllamaBaseURL(baseURL)
	url := baseURL + "/v1/chat/completions"
	headers := map[string]string{}
	return completeOpenAICompat(ctx, url, headers, model, system, history)
}

// ListOllamaModels asks GET /api/tags for model names.
func ListOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	baseURL = NormalizeOllamaBaseURL(baseURL)
	url := baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, msg)
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		if n := strings.TrimSpace(m.Name); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}
