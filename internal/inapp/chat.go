package inapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is one turn for Complete.
type Message struct {
	Role string // "user" or "assistant"
	Text string
}

// Complete asks the provider for one assistant reply. system may be empty.
// It does not send phone tools; that comes with the agent loop.
func Complete(ctx context.Context, p Provider, modelID, apiKey, system string, history []Message) (string, error) {
	if !Valid(p) {
		return "", fmt.Errorf("inapp: unknown provider %q", p)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", fmt.Errorf("inapp: empty API key")
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		modelID = DefaultChatModel().ID
	}
	switch p {
	case Anthropic:
		return completeAnthropic(ctx, modelID, apiKey, system, history)
	case OpenAI:
		return completeOpenAI(ctx, modelID, apiKey, system, history)
	default:
		return "", fmt.Errorf("inapp: unknown provider %q", p)
	}
}

func completeAnthropic(ctx context.Context, model, apiKey, system string, history []Message) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages":   make([]msg, 0, len(history)),
	}
	if strings.TrimSpace(system) != "" {
		body["system"] = system
	}
	msgs := body["messages"].([]msg)
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" {
			continue
		}
		msgs = append(msgs, msg{Role: role, Content: m.Text})
	}
	body["messages"] = msgs
	raw, err := postJSON(ctx, "https://api.anthropic.com/v1/messages", apiKey, map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("%s", out.Error.Message)
	}
	var b strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", fmt.Errorf("inapp: empty Anthropic reply")
	}
	return text, nil
}

func completeOpenAI(ctx context.Context, model, apiKey, system string, history []Message) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]msg, 0, len(history)+1)
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, msg{Role: "system", Content: system})
	}
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" {
			continue
		}
		msgs = append(msgs, msg{Role: role, Content: m.Text})
	}
	body := map[string]any{
		"model":    model,
		"messages": msgs,
	}
	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	raw, err := postJSON(ctx, "https://api.openai.com/v1/chat/completions", apiKey, headers, body)
	if err != nil {
		return "", err
	}
	return parseOpenAIChat(raw)
}

func completeOpenAICompat(ctx context.Context, url string, headers map[string]string, model, system string, history []Message) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]msg, 0, len(history)+1)
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, msg{Role: "system", Content: system})
	}
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" {
			continue
		}
		msgs = append(msgs, msg{Role: role, Content: m.Text})
	}
	body := map[string]any{
		"model":    model,
		"messages": msgs,
	}
	raw, err := postJSON(ctx, url, "", headers, body)
	if err != nil {
		return "", err
	}
	return parseOpenAIChat(raw)
}

func parseOpenAIChat(raw []byte) (string, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("%s", out.Error.Message)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("inapp: empty OpenAI reply")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// CompleteOpenAICompat is for tests and custom OpenAI-compatible URLs.
func CompleteOpenAICompat(ctx context.Context, url, model, apiKey, system string, history []Message) (string, error) {
	h := map[string]string{}
	if strings.TrimSpace(apiKey) != "" {
		h["Authorization"] = "Bearer " + strings.TrimSpace(apiKey)
	}
	return completeOpenAICompat(ctx, url, h, model, system, history)
}

func postJSON(ctx context.Context, url, apiKey string, headers map[string]string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	_ = apiKey // callers put auth in headers; keep signature stable for tests
	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 280 {
			msg = msg[:280] + "…"
		}
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, msg)
	}
	return raw, nil
}
