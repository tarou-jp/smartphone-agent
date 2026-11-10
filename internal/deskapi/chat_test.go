package deskapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/deskapi"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/policy"
	"github.com/DIYgod/pinky/internal/secret"
	"github.com/DIYgod/pinky/ui/fakeapi"
)

// ollamaMock is an OpenAI-compatible chat server, as Ollama serves one:
// with tools, it releases every phone the system prompt names; without,
// it writes a reply.
func ollamaMock(t *testing.T, requests *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		released := false
		for _, m := range body.Messages {
			if m.Role == "tool" {
				released = true
			}
		}
		if len(body.Tools) == 0 || released {
			for _, part := range []string{"All ", "done."} {
				chunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": part}}}})
			}
			chunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "stop"}}})
		} else {
			devs := regexp.MustCompile(`device "([^"]+)"`).FindAllStringSubmatch(body.Messages[0].Content, -1)
			seen := map[string]bool{}
			var calls []any
			for _, d := range devs {
				if seen[d[1]] {
					continue
				}
				seen[d[1]] = true
				args, _ := json.Marshal(map[string]any{"device": d[1], "outcome": "done", "summary": "ok"})
				calls = append(calls, map[string]any{"index": len(calls), "id": "c" + d[1], "function": map[string]any{"name": "phone_release", "arguments": string(args)}})
			}
			chunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": calls}}}})
			chunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "tool_calls"}}})
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func agentHarness(t *testing.T, llm string) *harness {
	t.Helper()
	pol := policy.Default()
	g, err := gateway.New(gateway.Options{
		Providers: []driver.Provider{
			demo.NewProvider(driver.Android, demo.NewDevice("a", "A", driver.Android)),
			demo.NewProvider(driver.Android, demo.NewDevice("b", "B", driver.Android)),
		},
		Policy: pol, TrustNewAgents: true, DataDir: t.TempDir(), NoScreenshots: true,
		SettleMax: 100 * time.Millisecond, PollInterval: 5 * time.Millisecond, DiscoverEvery: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	for len(g.Devices()) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	settings := deskapi.DefaultSettings()
	settings.OllamaBaseURL = llm
	settings.OllamaModels = []string{"tool-model"}
	settings.ChatModel = inapp.OllamaChatModelID("tool-model")
	h := &harness{t: t, api: fakeapi.New(fakeapi.Options{})}
	h.srv = deskapi.New(deskapi.Options{API: h.api, Agent: g, Version: "test", Settings: &settings, Secrets: &secret.Memory{}})
	t.Cleanup(h.srv.Close)
	url, err := h.srv.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.base = url[:strings.Index(url, "/?")]
	h.tok = h.srv.Token()
	return h
}

// chat posts a chat message and returns the stream's events by name.
func (h *harness) chat(body string) (map[string][]map[string]any, string) {
	h.t.Helper()
	req, _ := http.NewRequest("POST", h.base+"/api/chat", strings.NewReader(body))
	req.Header.Set("X-Pinky-Token", h.tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string][]map[string]any{}
	for _, block := range strings.Split(string(raw), "\n\n") {
		name, data := "", ""
		for _, l := range strings.Split(block, "\n") {
			if v, ok := strings.CutPrefix(l, "event: "); ok {
				name = v
			} else if v, ok := strings.CutPrefix(l, "data: "); ok {
				data = v
			}
		}
		if name == "" {
			continue
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(data), &m)
		out[name] = append(out[name], m)
	}
	return out, string(raw)
}

func stepKinds(evs []map[string]any) map[string]int {
	k := map[string]int{}
	for _, e := range evs {
		k[fmt.Sprint(e["kind"])]++
	}
	return k
}

func TestChatAskStreams(t *testing.T) {
	var n atomic.Int32
	llm := ollamaMock(t, &n)
	defer llm.Close()
	h := agentHarness(t, llm.URL)
	ev, raw := h.chat(`{"prompt":"hi","locale":"en"}`)
	if len(ev["done"]) != 1 || ev["done"][0]["reply"] != "All done." || stepKinds(ev["step"])["text_delta"] != 2 {
		t.Errorf("ask: %s", raw)
	}
}

func TestChatPhonesSideBySide(t *testing.T) {
	var n atomic.Int32
	llm := ollamaMock(t, &n)
	defer llm.Close()
	h := agentHarness(t, llm.URL)
	ev, raw := h.chat(`{"prompt":"go","locale":"en","devices":[{"id":"a","name":"A"},{"id":"b","name":"B"}]}`)
	if len(ev["run_end"]) != 2 || len(ev["done"]) != 1 || len(ev["error"]) != 0 {
		t.Fatalf("side by side: %s", raw)
	}
	for _, e := range ev["run_end"] {
		if e["reply"] != "All done." || e["code"] != nil {
			t.Errorf("run_end: %v", e)
		}
	}
	// Each phone's steps carry its id.
	by := map[string]int{}
	for _, e := range ev["step"] {
		if e["kind"] == "tool_end" {
			by[fmt.Sprint(e["deviceId"])]++
		}
	}
	if by["a"] != 2 || by["b"] != 2 {
		t.Errorf("tool ends by phone: %v", by)
	}
}

func TestChatPhonesTogether(t *testing.T) {
	var n atomic.Int32
	llm := ollamaMock(t, &n)
	defer llm.Close()
	h := agentHarness(t, llm.URL)
	ev, raw := h.chat(`{"prompt":"go","locale":"en","together":true,"devices":[{"id":"a","name":"A"},{"id":"b","name":"B"}]}`)
	if len(ev["run_end"]) != 0 || len(ev["done"]) != 1 || ev["done"][0]["reply"] != "All done." {
		t.Fatalf("together: %s", raw)
	}
	// One agent: two connects and two releases, each naming its phone,
	// from three model calls (the releases, then the summary).
	by := map[string]int{}
	for _, e := range ev["step"] {
		if e["kind"] == "tool_end" {
			by[fmt.Sprint(e["deviceId"])]++
		}
	}
	if by["a"] != 2 || by["b"] != 2 || n.Load() != 2 {
		t.Errorf("tool ends by phone %v from %d model calls: %s", by, n.Load(), raw)
	}
}
