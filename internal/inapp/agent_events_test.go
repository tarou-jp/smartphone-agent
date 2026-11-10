package inapp

import (
	"reflect"
	"testing"
)

func TestHistoryTurns(t *testing.T) {
	got := historyTurns([]Message{
		{Role: "assistant", Text: "hello"}, // a leading reply is dropped
		{Role: "user", Text: "open Settings"},
		{Role: "assistant", Text: "Opened."},
		{Role: "user", Text: "and Wi-Fi"}, // a run that failed: no reply
		{Role: "user", Text: "try again"},
		{Role: "error", Text: "boom"},
	})
	want := []Message{
		{Role: "user", Text: "open Settings"},
		{Role: "assistant", Text: "Opened."},
		{Role: "user", Text: "and Wi-Fi\n\ntry again"},
		{Role: "assistant", Text: "(No reply.)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("historyTurns:\n got %#v\nwant %#v", got, want)
	}
}

func TestEventsToolCall(t *testing.T) {
	var got []AgentEvent
	ev := &events{on: func(e AgentEvent) { got = append(got, e) }}
	id := ev.start("", "phone_tap", map[string]any{"device": "p9", "intent": "Tap Pay", "ref": 3})
	ev.end(id, "phone_tap", "approval_denied · Pixel 9 · rule: Payments\nThe person said no.\nscreen…")
	ev.text("  thinking  ")
	if len(got) != 3 {
		t.Fatalf("got %d events", len(got))
	}
	if s := got[0]; s.Kind != "tool_start" || s.ID != "t1" || s.Intent != "Tap Pay" || s.Args["device"] != nil || s.Args["ref"] != 3 {
		t.Errorf("start: %#v", s)
	}
	if e := got[1]; e.Kind != "tool_end" || e.ID != "t1" || e.Status != "approval_denied" || e.Detail != "The person said no." {
		t.Errorf("end: %#v", e)
	}
	if x := got[2]; x.Kind != "text" || x.Text != "thinking" {
		t.Errorf("text: %#v", x)
	}
}
