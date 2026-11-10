package inapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sse writes Server-Sent Events.
func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		fmt.Fprint(w, e)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func anthEvent(typ string, v map[string]any) string {
	v["type"] = typ
	b, _ := json.Marshal(v)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", typ, b)
}

// anthReply is a streamed Messages API reply: text, then tool calls.
func anthReply(text string, calls ...toolCall) []string {
	ev := []string{anthEvent("message_start", map[string]any{"message": map[string]any{"usage": map[string]any{"input_tokens": 10, "cache_read_input_tokens": 90}}})}
	i := 0
	if text != "" {
		ev = append(ev, anthEvent("content_block_start", map[string]any{"index": i, "content_block": map[string]any{"type": "text", "text": ""}}))
		// Two deltas, to see them streamed.
		half := len(text) / 2
		ev = append(ev,
			anthEvent("content_block_delta", map[string]any{"index": i, "delta": map[string]any{"type": "text_delta", "text": text[:half]}}),
			anthEvent("content_block_delta", map[string]any{"index": i, "delta": map[string]any{"type": "text_delta", "text": text[half:]}}),
			anthEvent("content_block_stop", map[string]any{"index": i}))
		i++
	}
	for _, c := range calls {
		args, _ := json.Marshal(c.Args)
		half := len(args) / 2
		ev = append(ev,
			anthEvent("content_block_start", map[string]any{"index": i, "content_block": map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name}}),
			anthEvent("content_block_delta", map[string]any{"index": i, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(args[:half])}}),
			anthEvent("content_block_delta", map[string]any{"index": i, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(args[half:])}}),
			anthEvent("content_block_stop", map[string]any{"index": i}))
		i++
	}
	stop := "end_turn"
	if len(calls) > 0 {
		stop = "tool_use"
	}
	return append(ev,
		anthEvent("message_delta", map[string]any{"delta": map[string]any{"stop_reason": stop}, "usage": map[string]any{"output_tokens": 7}}),
		anthEvent("message_stop", map[string]any{}))
}

func TestAnthropicStreamsTextAndToolCalls(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.URL.Path != "/v1/messages" {
			t.Errorf("request: %s %v", r.URL.Path, r.Header)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sse(w, anthReply("Opening Settings.", toolCall{ID: "tu1", Name: "phone_launch", Args: map[string]any{"app": "Settings", "device": "p9"}})...)
	}))
	defer srv.Close()
	c := &anthropicClient{model: "m", key: "k", base: srv.URL}
	var deltas []string
	res, err := c.turn(context.Background(), turnRequest{
		System: "sys", Tools: []agentTool{{Name: "phone_launch", Schema: map[string]any{"type": "object"}}},
		Items: []item{{Kind: itemUser, Text: "open settings"}},
	}, turnHandlers{text: func(d string) { deltas = append(deltas, d) }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Opening Settings." || len(deltas) != 2 {
		t.Errorf("text %q, deltas %q", res.Text, deltas)
	}
	if len(res.Calls) != 1 || res.Calls[0].Name != "phone_launch" || res.Calls[0].Args["app"] != "Settings" || res.Calls[0].ID != "tu1" {
		t.Errorf("calls: %+v", res.Calls)
	}
	if res.Usage.Input != 10 || res.Usage.CacheRead != 90 || res.Usage.Output != 7 {
		t.Errorf("usage: %+v", res.Usage)
	}
	// The system prompt, the tools and the thread's end are marked for the
	// prompt cache.
	raw, _ := json.Marshal(body)
	if n := strings.Count(string(raw), `"cache_control"`); n != 3 || body["stream"] != true {
		t.Errorf("%d cache marks, stream %v: %s", n, body["stream"], raw)
	}
}

func openAIChunk(v map[string]any) string {
	b, _ := json.Marshal(v)
	return "data: " + string(b) + "\n\n"
}

func TestOpenAIStreamsToolCallsInPieces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true || body["parallel_tool_calls"] != true {
			t.Errorf("body: %v", body)
		}
		tc := func(i int, id, name, args string) map[string]any {
			return map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": i, "id": id, "function": map[string]any{"name": name, "arguments": args}}}}}}}
		}
		sse(w,
			openAIChunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "Both "}}}}),
			openAIChunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "phones."}}}}),
			openAIChunk(tc(0, "c1", "phone_tap", `{"device":"a",`)),
			openAIChunk(tc(1, "c2", "phone_tap", `{"device":"b","ref":4}`)),
			openAIChunk(tc(0, "", "", `"ref":3}`)),
			openAIChunk(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "tool_calls"}}}),
			openAIChunk(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 9, "prompt_tokens_details": map[string]any{"cached_tokens": 60}}}),
			"data: [DONE]\n\n")
	}))
	defer srv.Close()
	c := &openAIClient{url: srv.URL, model: "m"}
	res, err := c.turn(context.Background(), turnRequest{Tools: []agentTool{{Name: "phone_tap"}}, Items: []item{{Kind: itemUser, Text: "tap"}}}, turnHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Both phones." || len(res.Calls) != 2 || res.Calls[0].Args["ref"] != float64(3) || res.Calls[1].Args["device"] != "b" {
		t.Errorf("result: %+v", res)
	}
	if res.Usage.Input != 40 || res.Usage.CacheRead != 60 || res.Usage.Output != 9 {
		t.Errorf("usage: %+v", res.Usage)
	}
}

func TestModelCallsTryAgain(t *testing.T) {
	old := defaultRetry
	defaultRetry = retryPolicy{attempts: 4, base: time.Millisecond, max: 5 * time.Millisecond}
	defer func() { defaultRetry = old }()

	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch n.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"slow down"}}`)
		case 2:
			// A stream that breaks before anything was said.
			sse(w, anthEvent("message_start", map[string]any{"message": map[string]any{}}))
		default:
			sse(w, anthReply("ok")...)
		}
	}))
	defer srv.Close()
	c := &anthropicClient{model: "m", key: "k", base: srv.URL}
	var retries []string
	res, err := c.turn(context.Background(), turnRequest{Items: []item{{Kind: itemUser, Text: "hi"}}}, turnHandlers{
		retry: func(_ int, _ time.Duration, err error) { retries = append(retries, err.Error()) },
	})
	if err != nil || res.Text != "ok" || n.Load() != 3 {
		t.Fatalf("res %+v, err %v, %d calls", res, err, n.Load())
	}
	if len(retries) != 2 || !strings.Contains(retries[0], "slow down") {
		t.Errorf("retries: %q", retries)
	}

	// A refusal is final.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"model does not support tools"}}`)
	}))
	defer bad.Close()
	_, err = (&openAIClient{url: bad.URL, model: "m"}).turn(context.Background(), turnRequest{Items: []item{{Kind: itemUser, Text: "hi"}}}, turnHandlers{})
	if err == nil || !ErrToolsNotSupported(err) {
		t.Errorf("err: %v", err)
	}
}

func TestBrokenStreamAfterWordsIsAnError(t *testing.T) {
	old := defaultRetry
	defaultRetry = retryPolicy{attempts: 4, base: time.Millisecond, max: time.Millisecond}
	defer func() { defaultRetry = old }()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		ev := anthReply("words that were shown")
		sse(w, ev[:3]...) // start, block, first delta; then the stream ends
	}))
	defer srv.Close()
	_, err := (&anthropicClient{model: "m", key: "k", base: srv.URL}).turn(context.Background(), turnRequest{Items: []item{{Kind: itemUser, Text: "hi"}}}, turnHandlers{})
	var se *streamError
	if !errors.As(err, &se) || n.Load() != 1 {
		t.Errorf("err %v after %d calls", err, n.Load())
	}
}

func TestViewCutsOldScreens(t *testing.T) {
	screen := func(n int) string {
		return fmt.Sprintf("settled · Pixel 9\nOpened screen %d.\n", n) + strings.Repeat("[3] button \"Something\"\n", 200)
	}
	var items []item
	for i := 0; i < 6; i++ {
		items = append(items, item{Kind: itemAssistant, Calls: []toolCall{{ID: fmt.Sprint(i), Name: "phone_observe"}}},
			item{Kind: itemResult, CallID: fmt.Sprint(i), Tool: "phone_observe", Output: screen(i)})
	}
	v := view(items)
	for i, it := range v {
		if it.Kind != itemResult {
			continue
		}
		old := i < len(v)-2*keepWhole
		cut := strings.Contains(it.Output, "older screen left out")
		if old != cut || !strings.HasPrefix(it.Output, "settled · Pixel 9") {
			t.Errorf("item %d: old %v, cut %v: %.80q", i, old, cut, it.Output)
		}
	}
	if items[1].Output != screen(0) {
		t.Error("view changed the thread")
	}
	long := strings.Repeat("x\n", maxOutput)
	if c := clip(long, maxOutput); len(c) > maxOutput+100 || !strings.Contains(c, "characters left out") {
		t.Errorf("clip: %d", len(c))
	}
	if cut := compactCut(items); cut <= 0 || items[cut].Kind == itemResult || len(items)-cut < keepRecent {
		t.Errorf("compactCut: %d", cut)
	}
}

// script is a model that answers turns from a list, and records what it
// was asked.
type script struct {
	mu    sync.Mutex
	turns []turnResult
	reqs  []turnRequest
}

func (s *script) turn(_ context.Context, req turnRequest, h turnHandlers) (turnResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, req)
	if len(s.turns) == 0 {
		return turnResult{}, errors.New("script: no more turns")
	}
	r := s.turns[0]
	s.turns = s.turns[1:]
	if r.Text != "" && h.text != nil {
		h.text(r.Text)
	}
	return r, nil
}

// fakePhones answers phone tools after a delay, and records the calls.
type fakePhones struct {
	mu    sync.Mutex
	calls []string
	delay time.Duration
}

func (f *fakePhones) call(ctx context.Context, tool string, args map[string]any, progress func(string)) string {
	f.mu.Lock()
	f.calls = append(f.calls, tool+"@"+str(args, "device"))
	f.mu.Unlock()
	if tool == "phone_tap" && progress != nil {
		progress("Waiting for the person: Pay")
	}
	time.Sleep(f.delay)
	switch tool {
	case "phone_release":
		return "released · " + str(args, "device")
	default:
		return "settled · " + str(args, "device") + "\nDid " + tool + ".\n[1] button \"OK\""
	}
}

func TestLoopRunsPhonesSideBySide(t *testing.T) {
	model := &script{turns: []turnResult{
		{Calls: []toolCall{{ID: "p", Name: "update_plan", Args: map[string]any{"plan": []any{
			map[string]any{"step": "Tap OK on both", "status": "in_progress"},
		}}}}},
		{Text: "Tapping on both.", Calls: []toolCall{
			{ID: "1", Name: "phone_tap", Args: map[string]any{"device": "a", "ref": 1}},
			{ID: "2", Name: "phone_tap", Args: map[string]any{"device": "b", "ref": 1}},
		}},
		{Calls: []toolCall{
			{ID: "3", Name: "phone_release", Args: map[string]any{"device": "a"}},
			{ID: "4", Name: "phone_release", Args: map[string]any{"device": "b"}},
		}},
		{Text: "Both phones show OK."},
	}}
	phones := &fakePhones{delay: 150 * time.Millisecond}
	var mu sync.Mutex
	var evs []AgentEvent
	start := time.Now()
	out := runLoop(context.Background(), loopConfig{
		AgentConfig: AgentConfig{Prompt: "tap OK on both"},
		devices:     []Device{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}},
		tools:       []agentTool{{Name: "phone_tap"}, {Name: "phone_release"}},
		model:       model, phones: phones,
	}, func(e AgentEvent) {
		mu.Lock()
		evs = append(evs, e)
		mu.Unlock()
	})
	took := time.Since(start)
	if out.Err != nil || out.Reply != "Both phones show OK." {
		t.Fatalf("outcome: %+v", out)
	}
	// connect, tap and release, each on two phones at once: three rounds.
	if took > 3*150*time.Millisecond+300*time.Millisecond {
		t.Errorf("took %s: the phones did not run side by side", took)
	}
	if len(phones.calls) != 6 {
		t.Errorf("calls: %v", phones.calls)
	}
	kinds := map[string]int{}
	for _, e := range evs {
		kinds[e.Kind]++
	}
	if kinds["plan"] != 1 || kinds["tool_start"] != 6 || kinds["tool_end"] != 6 || kinds["tool_progress"] != 2 || kinds["text"] != 1 || kinds["usage"] != 1 {
		t.Errorf("events: %v", kinds)
	}
	// The summary turn has no tools; the system prompt names both phones.
	last := model.reqs[len(model.reqs)-1]
	if len(last.Tools) != 0 || !strings.Contains(model.reqs[0].System, `device "b"`) {
		t.Errorf("last request has %d tools; system: %.200s", len(last.Tools), model.reqs[0].System)
	}
	// The first prompt carries what connecting showed.
	if !strings.Contains(model.reqs[0].Items[0].Text, "[phone_connect on A]") {
		t.Errorf("first item: %q", model.reqs[0].Items[0].Text)
	}
}

func TestLoopCompactsALongThread(t *testing.T) {
	big := strings.Repeat("y", compactAt/3)
	var turns []turnResult
	for i := 0; i < 6; i++ {
		turns = append(turns, turnResult{Text: big, Calls: []toolCall{{ID: fmt.Sprint(i), Name: "phone_observe", Args: map[string]any{}}}})
	}
	model := &script{turns: append(turns, turnResult{Text: "the summary"}, turnResult{Text: "done"})}
	// The summary is asked for when the thread passes compactAt: after a
	// few turns of big text; script answers it with "the summary".
	var compacted bool
	out := runLoop(context.Background(), loopConfig{
		AgentConfig: AgentConfig{Prompt: "go"},
		devices:     []Device{{ID: "a", Name: "A"}},
		tools:       []agentTool{{Name: "phone_observe"}},
		model:       model, phones: &fakePhones{},
	}, func(e AgentEvent) {
		if e.Kind == "compact" {
			compacted = true
		}
	})
	if out.Err != nil || !compacted {
		t.Fatalf("outcome %+v, compacted %v", out, compacted)
	}
	// After the compaction the model sees the summary first.
	var after *turnRequest
	for i := range model.reqs {
		if len(model.reqs[i].Items) > 0 && strings.HasPrefix(model.reqs[i].Items[0].Text, "Summary of the work so far") {
			after = &model.reqs[i]
			break
		}
	}
	if after == nil {
		t.Fatal("no request starts from the summary")
	}
	if size(after.Items) >= compactAt {
		t.Errorf("thread still %d characters", size(after.Items))
	}
}

func TestLoopStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := &script{turns: []turnResult{{Calls: []toolCall{{ID: "1", Name: "phone_observe", Args: map[string]any{}}}}}}
	phones := &fakePhones{delay: 50 * time.Millisecond}
	go func() { time.Sleep(80 * time.Millisecond); cancel() }()
	out := runLoop(ctx, loopConfig{AgentConfig: AgentConfig{Prompt: "go"}, devices: []Device{{ID: "a"}}, model: model, phones: phones}, nil)
	if !errors.Is(out.Err, context.Canceled) {
		t.Errorf("outcome: %+v", out)
	}
}

func TestTalkStreams(t *testing.T) {
	model := &script{turns: []turnResult{{Text: "**Hi**"}}}
	var got strings.Builder
	reply, err := talk(context.Background(), model, TalkConfig{System: "s", Prompt: "hello", History: []Message{{Role: "user", Text: "a"}, {Role: "assistant", Text: "b"}}},
		func(e AgentEvent) { got.WriteString(e.Delta) })
	if err != nil || reply != "**Hi**" || got.String() != "**Hi**" || len(model.reqs[0].Items) != 3 || len(model.reqs[0].Tools) != 0 {
		t.Errorf("reply %q, err %v, streamed %q, req %+v", reply, err, got.String(), model.reqs)
	}
}

var _ = io.EOF
