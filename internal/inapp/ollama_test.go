package inapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompleteOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "hello local"}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	reply, err := CompleteOllama(context.Background(), srv.URL, "llama3.2", "sys", []Message{
		{Role: "user", Text: "hi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hello local" {
		t.Fatalf("reply %q", reply)
	}
}

func TestListOllamaModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"a:latest"},{"name":"b"}]}`))
	}))
	t.Cleanup(srv.Close)

	names, err := ListOllamaModels(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a:latest" {
		t.Fatalf("names %v", names)
	}
}

func TestOllamaChatModelID(t *testing.T) {
	id := OllamaChatModelID("llama3.2:latest")
	name, ok := OllamaModelName(id)
	if !ok || name != "llama3.2:latest" {
		t.Fatalf("parse %q ok=%v", name, ok)
	}
	if !strings.HasPrefix(id, "ollama:") {
		t.Fatal(id)
	}
}
