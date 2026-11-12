package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/screen"
)

// waitFor polls until ok holds.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// personTap taps the mirror at the middle of the element with the label.
func personTap(t *testing.T, g *Gateway, sc *screen.Screen, label string) {
	t.Helper()
	for _, e := range sc.Elements {
		if e.Label == label {
			p := e.Bounds.Center()
			if err := g.Touch("pixel", driver.TouchDown, p); err != nil {
				t.Fatal(err)
			}
			if err := g.Touch("pixel", driver.TouchUp, p); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("%q is not on the screen:\n%s", label, sc.Render())
}

// What the agent could not do and the person did on the mirror is a step
// the task learns: it becomes a part, and the next agent runs it.
func TestLearnsFromThePerson(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look at my orders")
	res := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
	mustStatus(t, res, StatusSettled)
	l := s.leases["pixel"]
	steps := func() int {
		g.mu.Lock()
		defer g.mu.Unlock()
		return len(l.task.trace)
	}
	view := func() *screen.Screen {
		g.mu.Lock()
		defer g.mu.Unlock()
		if l.dev.pview != nil && l.dev.pviewSeq == l.dev.pseq {
			return l.dev.pview
		}
		return nil
	}

	// The person takes over and taps Orders on the mirror.
	if err := g.TakeOver("pixel"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the screen the person starts from", func() bool { return view() != nil })
	personTap(t, g, view(), "Orders")
	waitFor(t, "the person's tap to be kept", func() bool { return steps() == 2 })
	if dev.Current() != "orders" {
		t.Fatalf("on %q", dev.Current())
	}
	// Two taps faster than the screen can be read cannot be told apart:
	// they leave a gap, not two guesses.
	waitFor(t, "the screen after the tap", func() bool { return view() != nil })
	sc := view()
	personTap(t, g, sc, "Home")
	personTap(t, g, sc, "Orders")
	waitFor(t, "the recorder to finish", func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return !l.dev.recording && len(l.dev.pending) == 0
	})
	g.mu.Lock()
	gap := l.task.traceGap
	g.mu.Unlock()
	if n := steps(); n > 3 || !gap && n != 4 {
		t.Fatalf("%d steps kept of two taps in a hurry, gap %v", n, gap)
	}
	if err := g.HandBack("pixel", "I opened the orders"); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, s.Release(ctx, ReleaseRequest{Outcome: "failed"}), StatusSettled)

	// The next agent is offered what the person did, and runs it.
	ps, err := g.flows.Parts(dinely)
	if err != nil {
		t.Fatal(err)
	}
	orders := ""
	for _, p := range ps.Parts {
		if p.Tap != nil && strings.Contains(p.Text(), "Orders") {
			orders = p.ID
			if p.Expect == nil || len(p.Expect.Visible) == 0 {
				t.Errorf("the person's step has no checkpoint: %+v", p.Expect)
			}
		}
	}
	if orders == "" {
		t.Fatalf("the person's tap is no part: %+v", ps.Parts)
	}
	s2 := g.OpenSession(agent)
	t.Cleanup(s2.Close)
	res = s2.Connect(ctx, ConnectRequest{Task: "Orders again"})
	mustStatus(t, res, StatusSettled)
	open := partID(t, res.Parts, "open "+dinely)
	mustStatus(t, s2.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	res = runFlow(s2, FlowRequest{Parts: []string{open, orders}})
	mustStatus(t, res, StatusSettled)
	if dev.Current() != "orders" {
		t.Fatalf("on %q", dev.Current())
	}
}

// A finger lands on pixels: the button under it is what was pressed, not
// the text inside the button, and not the page around it.
func TestElementAt(t *testing.T) {
	sc := &screen.Screen{Width: 1000, Height: 2000, Elements: []screen.Element{
		{Role: screen.RoleOther, Label: "Page", Bounds: screen.Rect{X: 0, Y: 0, W: 1000, H: 2000}},
		{Role: screen.RoleButton, Label: "Post review", Bounds: screen.Rect{X: 100, Y: 100, W: 400, H: 120}},
		{Role: screen.RoleText, Label: "Post review", Bounds: screen.Rect{X: 150, Y: 130, W: 200, H: 60}},
		{Role: screen.RoleText, Label: "A song", Bounds: screen.Rect{X: 100, Y: 400, W: 300, H: 60}},
		{Role: screen.RoleText, Label: "read by OCR", Source: screen.SourceOCR, Bounds: screen.Rect{X: 100, Y: 600, W: 300, H: 60}},
	}}
	for _, c := range []struct {
		p    screen.Point
		want string
	}{
		{screen.Point{X: 200, Y: 150}, "button Post review"},
		{screen.Point{X: 450, Y: 110}, "button Post review"},
		{screen.Point{X: 200, Y: 420}, "text A song"},
		{screen.Point{X: 200, Y: 620}, "other Page"},
		{screen.Point{X: 900, Y: 1900}, "other Page"},
	} {
		e, ok := elementAt(sc, c.p)
		if got := e.Role + " " + e.Label; !ok || got != c.want {
			t.Errorf("at %v: %q, want %q", c.p, got, c.want)
		}
	}
	if _, ok := elementAt(sc, screen.Point{X: 5000, Y: 5}); ok {
		t.Error("an element off the screen")
	}
}
