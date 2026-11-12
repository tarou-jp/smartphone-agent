package flow

import (
	"os"
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

// ran pairs recorded steps with the screens they led to.
func ran(steps []Step, afters []*screen.Screen) []Ran {
	out := make([]Ran, len(steps))
	for i, s := range steps {
		out[i].Step = s
		if i < len(afters) {
			out[i].After = afters[i]
		}
	}
	return out
}

func TestParts(t *testing.T) {
	st := NewStore(t.TempDir())
	if apps := st.PartApps(); len(apps) != 0 {
		t.Fatalf("apps of an empty store: %v", apps)
	}
	steps, afters := partSteps()
	day := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	ids, added, err := st.LearnParts(ran(steps, afters), day)
	if err != nil || added != 3 || len(ids) != 4 || ids[2] != "" {
		t.Fatalf("learned %d parts, ids %v: %v", added, ids, err)
	}
	ps, err := st.Parts("app.dinely")
	if err != nil || len(ps.Parts) != 3 {
		t.Fatalf("parts: %+v %v", ps, err)
	}
	open, orders, back := ps.Parts[0], ps.Parts[1], ps.Parts[2]
	if ids[0] != open.ID || ids[1] != orders.ID || ids[3] != back.ID {
		t.Fatalf("ids %v", ids)
	}
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
	ids, added, err = st.LearnParts(ran(again, afters[1:2]), day.AddDate(0, 0, 1))
	if err != nil || added != 0 || ids[0] != orders.ID {
		t.Fatalf("the same step made %d new parts: %v", added, err)
	}
	ps, _ = st.Parts("app.dinely")
	if p := ps.find(orders.ID); len(ps.Parts) != 3 || p.Seen != 2 || p.Last != "2026-11-13" || p.Intent != "See the orders" || p.Expect.Visible[0].ID != "orders_title" {
		t.Fatalf("the part after it was seen again: %+v", p)
	}
	// The same tap leading somewhere else is another part.
	else_ := []Step{{Tap: &Selector{Role: "tab", Label: "Orders"},
		Expect: &Condition{App: "app.dinely", Visible: Selectors{{Role: "text", Label: "Sign in"}}}}}
	if _, added, _ = st.LearnParts(ran(else_, []*screen.Screen{demoScreen()}), day); added != 1 {
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

	// A part whose action leads elsewhere twice in a row is no longer
	// offered; being seen in between starts the count again. It stays for
	// the flows that name it.
	if st.BreakPart(back.ID) {
		t.Fatal("hidden at the first break")
	}
	st.LearnParts(ran(steps[3:], afters[3:]), day)
	if st.BreakPart(back.ID) {
		t.Fatal("hidden although it was seen since")
	}
	if !st.BreakPart(back.ID) {
		t.Fatal("not hidden at the second break in a row")
	}
	ps, _ = st.Parts("app.dinely")
	if _, p, ok := st.Part(back.ID); !ok || p.Broke != 2 {
		t.Fatalf("a part that broke is kept for the flows that name it: %+v", p)
	}
	for _, p := range ps.Sorted() {
		if p.ID == back.ID {
			t.Fatal("a part that broke twice is still offered")
		}
	}
	if st.BreakPart("p000000") {
		t.Fatal("broke a part that is not there")
	}

	// A saved flow ran the part, it led to another screen, and the task
	// went on and ended well: the part says where it leads now, under the
	// id it had. Without that leave, the step is another part.
	moved := demoScreen()
	moved.Elements = append(moved.Elements, screen.Element{Role: screen.RoleText, Label: "Order history", ID: "history_title"})
	step := Step{Tap: &Selector{Role: "tab", Label: "Orders"}, Expect: &Condition{App: "app.dinely", Visible: Selectors{{ID: "history_title"}}}}
	n := len(ps.Parts)
	ids, added, _ = st.LearnParts([]Ran{{Step: step, After: moved, Part: orders.ID}}, day)
	if added != 1 || ids[0] == orders.ID {
		t.Fatalf("a step that led elsewhere without leave to repair: %d new, %v", added, ids)
	}
	ids, added, _ = st.LearnParts([]Ran{{Step: Step{Tap: step.Tap, Expect: &Condition{App: "app.dinely", Visible: Selectors{{ID: "cart"}}}},
		After: demoScreen(), Part: back.ID, Repair: true}}, day)
	if ids[0] == back.ID {
		t.Fatal("a part was repaired from a step with another action")
	}
	st.BreakPart(orders.ID)
	gone := demoScreen()
	gone.Elements = append(gone.Elements, screen.Element{Role: screen.RoleText, Label: "Past orders", ID: "past_title"})
	step.Expect = &Condition{App: "app.dinely", Visible: Selectors{{ID: "past_title"}}}
	ids, added, _ = st.LearnParts([]Ran{{Step: step, After: gone, Part: orders.ID, Repair: true}}, day)
	ps, _ = st.Parts("app.dinely")
	if p := ps.find(orders.ID); added != 0 || ids[0] != orders.ID || p.Expect.Visible[0].ID != "past_title" || p.Broke != 0 || len(ps.Parts) != n+2 {
		t.Fatalf("the part was not repaired in place: %d new, %v, %+v", added, ids, p)
	}
}

// Typing into a field is a part, whatever the text: the step that names
// the part gives the text.
func TestTypingParts(t *testing.T) {
	st := NewStore("")
	hello, bye := "hello", "bye"
	field := &Selector{Role: "input", Label: "Search"}
	steps := []Step{
		{Launch: "app.dinely", Expect: &Condition{App: "app.dinely"}},
		{Type: &hello, Into: field, Submit: true},
		{Type: &bye, Into: field, Submit: true},
		{Type: &bye}, // into the focused field: nothing to find again
	}
	ids, added, err := st.LearnParts(ran(steps, nil), time.Now())
	if err != nil || added != 2 || ids[1] == "" || ids[1] != ids[2] || ids[3] != "" {
		t.Fatalf("%d new, %v: %v", added, ids, err)
	}
	_, p, _ := st.Part(ids[1])
	if *p.Type != PartText || p.Seen != 2 || p.Text() != `type <text> into input "Search"` {
		t.Fatalf("typing part: %+v %s", p, p.Text())
	}
	// A typing step by itself, as when the person typed: the app is
	// given, since no step around it says.
	alone := []Ran{{Step: Step{Type: &bye, Into: &Selector{Role: "input", Label: "Note"}}, App: "app.notes"}}
	if ids, added, err := st.LearnParts(alone, time.Now()); err != nil || added != 1 || ids[0] == "" {
		t.Fatalf("a typing step by itself: %d new, %v: %v", added, ids, err)
	}
	if ps, err := st.Parts("app.notes"); err != nil || len(ps.Parts) != 1 || *ps.Parts[0].Type != PartText {
		t.Fatalf("parts of the app given: %+v %v", ps, err)
	}
	find := func(id string) (Step, bool) {
		_, p, ok := st.Part(id)
		if !ok {
			return Step{}, false
		}
		return p.Step, true
	}
	f, err := Parse([]byte("flow: search\nparams: { q: { default: ramen } }\nsteps:\n  - part: " + ids[0] + "\n  - part: " + ids[1] + "\n    type: \"${q} near me\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.Resolve(find)
	if err != nil {
		t.Fatal(err)
	}
	if r, err = r.Bind(nil); err != nil || *r.Steps[1].Type != "ramen near me" || r.Steps[1].Into.Label != "Search" || !r.Steps[1].Submit {
		t.Fatalf("resolved: %+v %v", r.Steps[1], err)
	}
	for _, bad := range []string{
		"flow: x\nsteps:\n  - part: " + ids[1] + "\n",                  // no text for a part that types
		"flow: x\nsteps:\n  - part: " + ids[0] + "\n    type: hello\n", // text for a part that does not
	} {
		f, err := Parse([]byte(bad))
		if err != nil {
			t.Fatalf("%v:\n%s", err, bad)
		}
		if _, err := f.Resolve(find); err == nil {
			t.Errorf("resolved:\n%s", bad)
		}
	}
}

func TestPartsInMemoryAndLimits(t *testing.T) {
	st := NewStore("")
	steps, afters := partSteps()
	if _, added, err := st.LearnParts(ran(steps, afters), time.Now()); err != nil || added != 3 {
		t.Fatalf("%d %v", added, err)
	}
	if apps := st.PartApps(); len(apps) != 1 {
		t.Fatalf("apps: %v", apps)
	}
	// Steps in an app whose id is no file name are not kept.
	odd := []Step{{Launch: "../x", Expect: &Condition{App: "../x"}}}
	if _, added, err := st.LearnParts(ran(odd, nil), time.Now()); err != nil || added != 0 {
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
		ps.learn(Ran{Step: s}, "", day)
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
		{Step{Tap: &Selector{Role: "button", Label: "x"}, Expect: exp}, true},
		{Step{Tap: &Selector{ID: "x", Or: []Selector{{Role: "image", Label: "x"}}}, Expect: exp}, true},
		{Step{Tap: &Selector{Role: "button", Label: "x"}}, false},
		// What a screen shows, not what it is made of.
		{Step{Tap: &Selector{Role: "text", Label: "A song"}, Expect: exp}, false},
		{Step{Tap: &Selector{Text: "A song"}, Expect: exp}, false},
		{Step{Tap: &Selector{Point: &Point{X: 0.5, Y: 0.5}}, Expect: exp}, false},
		{Step{Type: &text, Expect: exp}, false},
		{Step{Type: &text, Into: &Selector{Role: "input"}}, true},
		{Step{Key: "back", Expect: exp}, true},
		{Step{Key: "home", Expect: exp}, false},
		{Step{Swipe: "up", Expect: exp}, false},
		{Step{Swipe: "up", In: &Selector{Role: "list"}, Expect: exp}, false},
		{Step{From: &Point{}, To: &Point{}, Expect: exp}, false},
		{Step{Launch: "a", Expect: exp}, true},
	} {
		if got := Partable(c.s); got != c.want {
			t.Errorf("%s: %v", c.s.Describe(), got)
		}
	}
}

// A parts file from before Pinky stopped keeping swipes and taps on text
// still reads: those parts are dropped, the others stay.
func TestPartsOfKindsNoLongerKept(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	steps, afters := partSteps()
	if _, _, err := st.LearnParts(ran(steps, afters), time.Now()); err != nil {
		t.Fatal(err)
	}
	path := st.partsPath("app.dinely")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := string(data) + `  - id: pc4f79b
    swipe: up
    expect:
      app: app.dinely
      visible:
        - role: button
          label: Community
  - id: pcd2d27
    tap:
      role: text
      label: UMAKEMEFEELLIKE
    expect:
      app: app.dinely
`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	ps, err := st.Parts("app.dinely")
	if err != nil || len(ps.Parts) != 3 {
		t.Fatalf("parts: %+v %v", ps, err)
	}
	if _, _, ok := st.Part("pcd2d27"); ok {
		t.Fatal("a tap on a title is still a part")
	}
}
