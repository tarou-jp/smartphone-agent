package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sseData(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		fmt.Fprintf(w, "data: %s\n\n", e)
	}
}

// The Responses API: the thread goes out as input items, with the picture
// of the screen inside the tool's answer it belongs to and the model's
// earlier output as it came; text, calls, usage and the turn's own output
// come back.
func TestOpenAIResponses(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("request to %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &got)
		sseData(w,
			`{"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_1","encrypted_content":"abc"}}`,
			`{"type":"response.output_text.delta","delta":"Five "}`,
			`{"type":"response.output_text.delta","delta":"stars."}`,
			`{"type":"response.output_item.done","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Five stars."}]}}`,
			`{"type":"response.output_item.done","item":{"type":"function_call","call_id":"c2","name":"phone_tap","arguments":"{\"ref\":7}"}}`,
			`{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":60}}}}`,
		)
	}))
	defer srv.Close()
	c := &openAIResponsesClient{url: srv.URL + "/v1/responses", key: "k", model: "m"}
	items := []item{
		{Kind: itemUser, Text: "Check the stars"},
		{Kind: itemAssistant, Calls: []toolCall{{ID: "c1", Name: "phone_observe", Args: map[string]any{"screenshot": true}}},
			Raw: []json.RawMessage{json.RawMessage(`{"type":"reasoning","id":"rs_0","encrypted_content":"xyz"}`), json.RawMessage(`{"type":"function_call","call_id":"c1","name":"phone_observe","arguments":"{}"}`)}},
		resultWithImage("c1"),
	}
	var said strings.Builder
	res, err := c.turn(context.Background(), turnRequest{System: "sys", Items: items, MaxTokens: 500,
		Tools: []agentTool{{Name: "phone_tap", Description: "Tap", Schema: map[string]any{"type": "object"}}}},
		turnHandlers{text: func(d string) { said.WriteString(d) }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Five stars." || said.String() != "Five stars." || len(res.Calls) != 1 || res.Calls[0].ID != "c2" || res.Calls[0].Name != "phone_tap" || res.Calls[0].Args["ref"] != float64(7) {
		t.Fatalf("turn: %+v", res)
	}
	if len(res.Raw) != 3 || !strings.Contains(string(res.Raw[0]), "encrypted_content") {
		t.Fatalf("the turn's own output, to send back: %s", res.Raw)
	}
	if res.Usage.Input != 40 || res.Usage.CacheRead != 60 || res.Usage.Output != 20 {
		t.Fatalf("usage: %+v", res.Usage)
	}

	if got["instructions"] != "sys" || got["store"] != false || got["stream"] != true || got["max_output_tokens"] != float64(500) {
		t.Fatalf("request: %v", got)
	}
	if r, _ := got["reasoning"].(map[string]any); r["effort"] != "low" {
		t.Fatalf("the model is not asked to reason: %v", got["reasoning"])
	}
	in, _ := json.Marshal(got["input"])
	for _, want := range []string{`"text":"Check the stars"`, `"encrypted_content":"xyz"`, `"call_id":"c1"`} {
		if !strings.Contains(string(in), want) {
			t.Fatalf("input misses %s:\n%s", want, in)
		}
	}
	// The picture is inside the answer to the call that took it, not a
	// message of the person's.
	inside := false
	for _, raw := range got["input"].([]any) {
		it := raw.(map[string]any)
		b, _ := json.Marshal(it)
		pic := strings.Contains(string(b), `"type":"input_image"`) && strings.Contains(string(b), "data:image/jpeg;base64,/9j/")
		switch {
		case pic && it["type"] == "function_call_output" && it["call_id"] == "c1":
			inside = true
		case pic:
			t.Fatalf("the picture went out in %s", b)
		}
	}
	if !inside {
		t.Fatalf("no picture in the tool's answer:\n%s", in)
	}
	tools, _ := json.Marshal(got["tools"])
	if !strings.Contains(string(tools), `"name":"phone_tap"`) || strings.Contains(string(tools), `"function":{`) {
		t.Fatalf("tools: %s", tools)
	}
}

// An account or a model the Responses API refuses is not left without an
// agent: the run goes on over Chat Completions.
func TestOpenAIResponsesFallsBack(t *testing.T) {
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		if r.URL.Path == "/v1/responses" {
			http.Error(w, `{"error":{"message":"this model is not available on the Responses API"}}`, http.StatusBadRequest)
			return
		}
		sseData(w, `{"choices":[{"delta":{"content":"Hello."}}]}`, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer srv.Close()
	chat := &openAIClient{url: srv.URL + "/v1/chat/completions", model: "m", reasoningOff: true}
	c := &openAIResponsesClient{url: srv.URL + "/v1/responses", model: "m", fallback: chat}
	for range 2 {
		res, err := c.turn(context.Background(), turnRequest{System: "sys", Items: []item{{Kind: itemUser, Text: "Hi"}}}, turnHandlers{})
		if err != nil || res.Text != "Hello." {
			t.Fatalf("turn: %+v %v", res, err)
		}
	}
	if calls["/v1/responses"] != 1 || calls["/v1/chat/completions"] != 2 {
		t.Fatalf("requests: %v", calls)
	}
}
