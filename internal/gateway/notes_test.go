package gateway

import (
	"context"
	"strings"
	"testing"
	"time"
)

// An agent writes down what it found out about an app, and the next task
// in the app is shown it, once.
func TestNotes(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look around")
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}), StatusSettled)
	res := s.Note(ctx, NoteRequest{Note: "  The row of tabs hides\nwhen the page scrolls. "})
	mustStatus(t, res, StatusSettled)
	if !strings.Contains(res.Message, dinely) {
		t.Fatalf("note: %s", res.Message)
	}
	if res = s.Note(ctx, NoteRequest{Note: "the row of tabs hides when the page scrolls."}); !strings.Contains(res.Message, "already") {
		t.Fatalf("the same note again: %s", res.Message)
	}
	mustStatus(t, s.Note(ctx, NoteRequest{App: "dinely", Note: "Orders take a moment to load."}), StatusSettled)
	if res = s.Note(ctx, NoteRequest{Note: " "}); !res.IsError() {
		t.Fatalf("an empty note: %+v", res)
	}
	if res = s.Note(ctx, NoteRequest{App: "No such app", Note: "It is slow."}); !res.IsError() {
		t.Fatalf("a note on an app nobody knows: %+v", res)
	}
	text, _ := g.flows.Notes(dinely)
	if text != "- The row of tabs hides when the page scrolls.\n- Orders take a moment to load." {
		t.Fatalf("notes: %q", text)
	}
	var noted int
	for _, st := range g.Tasks()[0].Steps {
		if st.Tool == "phone_note" {
			noted++
		}
	}
	if noted != 2 {
		t.Fatalf("%d notes show in the task's steps, want 2", noted)
	}
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)

	// The next task is shown them when it opens the app, and only then.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	res = s.Connect(ctx, ConnectRequest{Task: "Order lunch", Apps: []string{"Dinely"}})
	if len(res.Notes) != 1 || res.Notes[0].App != dinely || res.Notes[0].Text != text || !strings.Contains(res.Message, "Notes on "+dinely) {
		t.Fatalf("connect: %+v %s", res.Notes, res.Message)
	}
	if res = s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}); len(res.Notes) != 0 {
		t.Fatalf("the notes were shown twice: %s", res.Message)
	}
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)

	// A test run keeps nothing.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	mustStatus(t, s.Connect(ctx, ConnectRequest{Task: "Run the tests", Test: true}), StatusSettled)
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}), StatusSettled)
	if res = s.Note(ctx, NoteRequest{Note: "A test found this."}); !strings.Contains(res.Message, "test run") {
		t.Fatalf("a note in a test run: %s", res.Message)
	}
	if after, _ := g.flows.Notes(dinely); after != text {
		t.Fatalf("a test run changed the notes: %q", after)
	}
}

// The person sees what Pinky keeps of an app, and changes it.
func TestLibrary(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look at my orders")
	res := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
	mustStatus(t, tap(s, ref(t, res, "Orders")), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	if err := g.SetNotes(dinely, "Orders take a moment to load.\n"); err != nil {
		t.Fatal(err)
	}

	lib := g.Library()
	if len(lib.Apps) != 1 || lib.Apps[0].App != dinely {
		t.Fatalf("apps: %+v", lib.Apps)
	}
	a := lib.Apps[0]
	if a.Notes != "Orders take a moment to load." || len(a.Flows) != 1 || len(a.Parts) != 2 {
		t.Fatalf("library: notes %q, %d flows, %d parts", a.Notes, len(a.Flows), len(a.Parts))
	}
	f := a.Flows[0]
	if len(f.Steps) != 2 || f.Steps[0].Kind != "launch" || f.Steps[1].Kind != "tap" || f.Steps[1].Part == "" || f.Steps[1].Missing ||
		!strings.Contains(f.Steps[1].Text, "Orders") || f.Steps[1].Leads == "" {
		t.Fatalf("flow steps: %+v", f.Steps)
	}
	orders := f.Steps[1].Part
	var part *LibraryPart
	for i := range a.Parts {
		if a.Parts[i].ID == orders {
			part = &a.Parts[i]
		}
	}
	if part == nil || part.Kind != "tap" || part.Seen != 1 || part.Broken || len(part.Flows) != 1 || part.Flows[0] != f.Name {
		t.Fatalf("the part of the flow's step: %+v", part)
	}

	// A part that broke is still listed, as broken.
	for range 2 {
		g.flows.BreakPart(orders)
	}
	for _, p := range g.Library().Apps[0].Parts {
		if p.ID == orders && !p.Broken {
			t.Fatalf("a broken part does not say so: %+v", p)
		}
	}

	// Without the part the flow's step says that it is gone.
	if err := g.RemovePart(orders); err != nil {
		t.Fatal(err)
	}
	lib = g.Library()
	if st := lib.Apps[0].Flows[0].Steps[1]; !st.Missing || st.Part != orders || len(lib.Apps[0].Parts) != 1 {
		t.Fatalf("after removing the part: %+v, %d parts", st, len(lib.Apps[0].Parts))
	}
	if err := g.RemoveFlow(f.Name); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveFlow(f.Name); err == nil {
		t.Fatal("a flow was removed twice")
	}
	if err := g.SetNotes(dinely, ""); err != nil {
		t.Fatal(err)
	}
	if err := g.SetNotes("../x", "y"); err == nil {
		t.Fatal("notes for a name that is no app")
	}
	lib = g.Library()
	if len(lib.Apps) != 1 || lib.Apps[0].Notes != "" || len(lib.Apps[0].Flows) != 0 {
		t.Fatalf("after removing the flow and the notes: %+v", lib.Apps)
	}
}
