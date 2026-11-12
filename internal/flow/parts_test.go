package flow

import (
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/screen"
)

// partSteps are a task's steps as the gateway records them: open the
// app, go to the orders, and back.
func partSteps() ([]Step, []*screen.Screen) {
	home := demoScreen()
	orders := demoScreen()
	orders.Elements = append(orders.Elements,
		screen.Element{Role: screen.RoleText, Label: "Your orders", ID: "orders_title"},
		screen.Element{Role: screen.RoleButton, Label: "Back"})
	steps := []Step{
		{Launch: "app.dinely", Expect: &Condition{App: "app.dinely"}, Intent: "Open Dinely"},
		{Tap: &Selector{Role: "tab", Label: "Orders"}, Expect: &Condition{App: "app.dinely", Visible: Selectors{{ID: "orders_title"}}}},
		{Swipe: "up"}, // no checkpoint: not a part
		{Tap: &Selector{Role: "button", Label: "Back"}, Expect: &Condition{App: "app.dinely", Gone: Selectors{{ID: "orders_title"}}}},
	}
	return steps, []*screen.Screen{home, orders, orders, home}
}

func TestParts(t *testing.T) {
	st := NewStore(t.TempDir())
	if apps := st.PartApps(); len(apps) != 0 {
		t.Fatalf("apps of an empty store: %v", apps)
	}
	steps, afters := partSteps()
	day := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	added, err := st.LearnParts(steps, afters, day)
	if err != nil || added != 3 {
		t.Fatalf("learned %d parts: %v", added, err)
	}
	ps, err := st.Parts("app.dinely")
	if err != nil || len(ps.Parts) != 3 {
		t.Fatalf("parts: %+v %v", ps, err)
	}
	open, orders, back := ps.Parts[0], ps.Parts[1], ps.Parts[2]
	for _, p := range ps.Parts {
		if !ValidPartID(p.ID) || p.Seen != 1 || p.Last != "2026-11-12" {
			t.Errorf("part: %+v", p)
		}
	}
	if len(open.After) != 0 || len(orders.After) != 1 || orders.After[0] != open.ID || back.After[0] != orders.ID {
		t.Errorf("after: %v, %v, %v", open.After, orders.After, back.After)
	}
	if got := orders.Text(); got != `tap tab "Orders" → #orders_title shows` {
		t.Errorf("text: %s", got)
	}
	if got := open.Text(); got != "open app.dinely → app app.dinely" {
		t.Errorf("text: %s", got)
	}
	if apps := st.PartApps(); len(apps) != 1 || apps[0] != "app.dinely" {
		t.Fatalf("apps: %v", apps)
	}

	// The same tap from another screen names other elements in its
	// checkpoint, and leads to the screen the part describes: it is that
	// part again, not a second one.
	again := []Step{{Tap: &Selector{Role: "tab", Label: "Orders"}, Intent: "See the orders",
		Expect: &Condition{App: "app.dinely", Visible: Selectors{{Role: "button", Label: "Back"}}}}}
	added, err = st.LearnParts(again, afters[1:2], day.AddDate(0, 0, 1))
	if err != nil || added != 0 {
		t.Fatalf("the same step made %d new parts: %v", added, err)
	}
	ps, _ = st.Parts("app.dinely")
	if p := ps.find(orders.ID); len(ps.Parts) != 3 || p.Seen != 2 || p.Last != "2026-11-13" || p.Intent != "See the orders" || p.Expect.Visible[0].ID != "orders_title" {
		t.Fatalf("the part after it was seen again: %+v", p)
	}
	// The same tap leading somewhere else is another part.
	else_ := []Step{{Tap: &Selector{Role: "tab", Label: "Orders"},
		Expect: &Condition{App: "app.dinely", Visible: Selectors{{Role: "text", Label: "Sign in"}}}}}
	if added, _ = st.LearnParts(else_, []*screen.Screen{demoScreen()}, day); added != 1 {
		t.Fatalf("a tap that leads elsewhere made %d parts", added)
	}

	// A flow names parts; resolving puts their steps in.
	f, err := Parse([]byte("flow: orders\nsteps:\n  - part: " + open.ID + "\n  - part: " + orders.ID + "\n    timeout_ms: 500\n  - assert: { visible: Back }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Steps[0].Kind() != "part" {
		t.Fatalf("kind: %s", f.Steps[0].Kind())
	}
	find := func(id string) (Step, bool) {
		_, p, ok := st.Part(id)
		if !ok {
			return Step{}, false
		}
		return p.Step, true
	}
	r, err := f.Resolve(find)
	if err != nil {
		t.Fatal(err)
	}
	if r.Steps[0].Launch != "app.dinely" || r.Steps[0].PartID != open.ID || r.Steps[1].Tap.Label != "Orders" ||
		r.Steps[1].TimeoutMS != 500 || r.Steps[1].Expect == nil || r.Steps[2].Kind() != "assert" || f.Steps[0].Launch != "" {
		t.Fatalf("resolved: %+v", r.Steps)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	f.Steps[1].Part = "pffffff"
	if _, err := f.Resolve(find); err == nil || !strings.Contains(err.Error(), "pffffff") {
		t.Fatalf("an unknown part: %v", err)
	}
	for _, bad := range []string{
		"flow: x\nsteps:\n  - part: orders\n",
		"flow: x\nsteps:\n  - part: p1a2b3c\n    expect: { app: a }\n",
		"flow: x\nsteps:\n  - part: p1a2b3c\n    tap: Orders\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("parsed:\n%s", bad)
		}
	}

	// A part whose action leads elsewhere twice in a row is dropped; being
	// seen in between starts the count again.
	if st.BreakPart(back.ID) {
		t.Fatal("dropped at the first break")
	}
	st.LearnParts(steps[3:], afters[3:], day)
	if st.BreakPart(back.ID) {
		t.Fatal("dropped although it was seen since")
	}
	if !st.BreakPart(back.ID) {
		t.Fatal("not dropped at the second break in a row")
	}
	if _, _, ok := st.Part(back.ID); ok {
		t.Fatal("the dropped part is still there")
	}
	if st.BreakPart("p000000") {
		t.Fatal("broke a part that is not there")
	}
}

func TestPartsInMemoryAndLimits(t *testing.T) {
	st := NewStore("")
	steps, afters := partSteps()
	if added, err := st.LearnParts(steps, afters, time.Now()); err != nil || added != 3 {
		t.Fatalf("%d %v", added, err)
	}
	if apps := st.PartApps(); len(apps) != 1 {
		t.Fatalf("apps: %v", apps)
	}
	// Steps in an app whose id is no file name are not kept.
	odd := []Step{{Launch: "../x", Expect: &Condition{App: "../x"}}}
	if added, err := st.LearnParts(odd, nil, time.Now()); err != nil || added != 0 {
		t.Fatalf("%d %v", added, err)
	}
	// Past the limit, the parts seen longest ago go.
	ps := &Parts{App: "app.big"}
	for i := 0; i < MaxParts+5; i++ {
		s := Step{Tap: &Selector{Role: "button", Label: strings.Repeat("a", i+1)}, Expect: &Condition{App: "app.big"}}
		day := "2026-01-02"
		if i < 5 {
			day = "2025-01-01"
		}
		ps.learn(s, nil, "", day)
	}
	old := ps.Parts[0].ID
	ps.trim()
	if len(ps.Parts) != MaxParts || ps.find(old) != nil {
		t.Fatalf("%d parts after trimming", len(ps.Parts))
	}
}

func TestPartable(t *testing.T) {
	text := "hi"
	exp := &Condition{App: "a"}
	for _, c := range []struct {
		s    Step
		want bool
	}{
		{Step{Tap: &Selector{Label: "x"}, Expect: exp}, true},
		{Step{Tap: &Selector{Label: "x"}}, false},
		{Step{Tap: &Selector{Point: &Point{X: 0.5, Y: 0.5}}, Expect: exp}, false},
		{Step{Type: &text, Expect: exp}, false},
		{Step{Key: "back", Expect: exp}, true},
		{Step{Key: "home", Expect: exp}, false},
		{Step{Swipe: "up", Expect: exp}, true},
		{Step{From: &Point{}, To: &Point{}, Expect: exp}, false},
		{Step{Launch: "a", Expect: exp}, true},
	} {
		if got := Partable(c.s); got != c.want {
			t.Errorf("%s: %v", c.s.Describe(), got)
		}
	}
}
