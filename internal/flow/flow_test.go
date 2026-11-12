package flow

import (
	"errors"
	"strings"
	"testing"

	"github.com/DIYgod/pinky/internal/screen"
)

const sample = `
flow: dinely.reorder
description: Put last week's lunch in the cart
app: app.dinely
params:
  dish:
    default: Chicken rice bowl
  note: {}
steps:
  - launch: Dinely
    expect: { app: app.dinely, visible: { id: tab_orders } }
  - tap: Orders
    expect:
      visible:
        - { role: button, contains: "${dish}" }
  - tap: { id: reorder_1, or: [{ role: button, contains: "Reorder · ${dish}" }] }
  - type: "${note}"
    into: { role: input }
    submit: true
    optional: true
  - swipe: up
    in: { role: list }
  - key: back
  - assert: { gone: "Loading" }
    timeout_ms: 2000
`

func TestParseAndBind(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "dinely.reorder" || len(f.Steps) != 7 {
		t.Fatalf("flow: %+v", f)
	}
	if got := f.Steps[1].Tap; got.Label != "Orders" {
		t.Errorf("a string selector is a label: %+v", got)
	}
	if got := f.Steps[6].Assert.Gone; len(got) != 1 || got[0].Label != "Loading" {
		t.Errorf("one selector is a list of one: %+v", got)
	}
	if _, err := f.Bind(nil); err == nil || !strings.Contains(err.Error(), "note") {
		t.Errorf("a required parameter must be passed: %v", err)
	}
	if _, err := f.Bind(map[string]string{"note": "x", "nope": "y"}); err == nil {
		t.Error("an unknown parameter is an error")
	}
	b, err := f.Bind(map[string]string{"note": "no cilantro"})
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Steps[2].Tap.Or[0].Contains; got != "Reorder · Chicken rice bowl" {
		t.Errorf("default not bound: %q", got)
	}
	if got := *b.Steps[3].Type; got != "no cilantro" {
		t.Errorf("value not bound: %q", got)
	}
	if got := *f.Steps[3].Type; got != "${note}" {
		t.Errorf("Bind changed the flow it was given: %q", got)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "tap: Orders") {
		t.Errorf("a label selector should be written as a string:\n%s", data)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	again, _ := g.Marshal()
	if string(again) != string(data) {
		t.Errorf("not stable:\n%s\n---\n%s", data, again)
	}
}

func TestParseRefuses(t *testing.T) {
	for name, src := range map[string]string{
		"bad name":           "flow: Bad Name\nsteps: [{key: back}]",
		"no steps":           "flow: a\nsteps: []",
		"two actions":        "flow: a\nsteps: [{key: back, launch: X}]",
		"unknown step key":   "flow: a\nsteps: [{key: back, expct: {app: x}}]",
		"unknown selector":   "flow: a\nsteps: [{tap: {labl: x}}]",
		"unknown condition":  "flow: a\nsteps: [{key: back, expect: {visble: x}}]",
		"empty selector":     "flow: a\nsteps: [{tap: {}}]",
		"empty condition":    "flow: a\nsteps: [{key: back, expect: {}}]",
		"bad direction":      "flow: a\nsteps: [{swipe: sideways}]",
		"half a swipe":       "flow: a\nsteps: [{from: {x: 0.5, y: 0.5}}]",
		"point outside":      "flow: a\nsteps: [{tap: {point: {x: 2, y: 0.5}}}]",
		"undeclared param":   "flow: a\nsteps: [{type: \"${q}\"}]",
		"submit on a tap":    "flow: a\nsteps: [{tap: x, submit: true}]",
		"optional key":       "flow: a\nsteps: [{key: back, optional: true}]",
		"assert with expect": "flow: a\nsteps: [{assert: {app: x}, expect: {app: x}}]",
		"nested or":          "flow: a\nsteps: [{tap: {id: a, or: [{id: b, or: [{id: c}]}]}}]",
		"timeout too long":   "flow: a\nsteps: [{key: back, timeout_ms: 99999}]",
	} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func demoScreen() *screen.Screen {
	s := &screen.Screen{Width: 1000, Height: 2000, App: screen.App{Name: "Dinely", ID: "app.dinely"}}
	add := func(role, label, id string, y int, src screen.Source) {
		s.Elements = append(s.Elements, screen.Element{Role: role, Label: label, ID: id, Source: src,
			Bounds: screen.Rect{X: 100, Y: y, W: 800, H: 100}})
	}
	add(screen.RoleText, "Your orders", "title", 100, screen.SourceTree)
	add(screen.RoleButton, "Reorder", "row_action", 300, screen.SourceTree)
	add(screen.RoleButton, "Reorder", "row_action", 500, screen.SourceTree)
	add(screen.RoleButton, "Cart · 1", "cart", 700, screen.SourceTree)
	add(screen.RoleButton, "", "", 900, screen.SourceTree)
	add(screen.RoleText, "Delivered 12:30", "", 1100, screen.SourceOCR)
	screen.NewRefs().Assign(s)
	return s
}

func TestFind(t *testing.T) {
	s := demoScreen()
	find := func(sel Selector) (Match, error) { return sel.Find(s) }

	if m, err := find(Selector{ID: "cart"}); err != nil || m.Element.Label != "Cart · 1" {
		t.Errorf("by id: %+v %v", m, err)
	}
	if _, err := find(Selector{Label: "Reorder"}); err == nil || !strings.Contains(err.Error(), "2 elements") {
		t.Errorf("two matches without an index must not resolve: %v", err)
	}
	if m, err := find(Selector{Label: "Reorder", Index: 2}); err != nil || m.Element.Bounds.Y != 500 {
		t.Errorf("index: %+v %v", m, err)
	}
	if _, err := find(Selector{Label: "Reorder", Index: 3}); err == nil {
		t.Error("an index past the matches resolved")
	}
	if m, err := find(Selector{ID: "gone", Or: []Selector{{Role: "button", Contains: "cart"}}}); err != nil || m.By != 1 || m.Element.ID != "cart" {
		t.Errorf("fallback: %+v %v", m, err)
	}
	if m, err := find(Selector{Text: "delivered"}); err != nil || m.Element.Source != screen.SourceOCR {
		t.Errorf("text reads OCR: %+v %v", m, err)
	}
	if _, err := find(Selector{Contains: "delivered"}); err == nil {
		t.Error("contains must not match OCR text")
	}
	if m, err := find(Selector{Point: &Point{X: 0.5, Y: 0.25}}); err != nil || m.Point == nil || *m.Point != (screen.Point{X: 500, Y: 500}) {
		t.Errorf("point: %+v %v", m, err)
	}
	if _, err := find(Selector{Label: "Checkout"}); err == nil {
		t.Error("found what is not there")
	}
}

func TestCondition(t *testing.T) {
	s := demoScreen()
	ok := func(c Condition) bool { r, _ := c.Check(s); return r }
	if !ok(Condition{App: "app.dinely", Visible: Selectors{{Label: "Reorder"}, {ID: "cart"}}, Gone: Selectors{{Label: "Loading"}}}) {
		t.Error("a holding condition failed; several matches are fine for visible")
	}
	if !ok(Condition{App: "dinely"}) {
		t.Error("the app's name should match without case")
	}
	if _, why := (&Condition{App: "Chatter"}).Check(s); !strings.Contains(why, "Dinely") {
		t.Errorf("why: %q", why)
	}
	if ok(Condition{Visible: Selectors{{Label: "Checkout"}}}) || ok(Condition{Gone: Selectors{{ID: "cart"}}}) {
		t.Error("a failing condition held")
	}
}

func TestSelectorFor(t *testing.T) {
	s := demoScreen()
	for i, e := range s.Elements {
		sel := SelectorFor(s, e)
		if err := sel.validate(); err != nil {
			t.Errorf("element %d: invalid selector %+v: %v", i, sel, err)
			continue
		}
		m, err := sel.Find(s)
		if err != nil {
			t.Errorf("element %d: %+v does not resolve: %v", i, sel, err)
			continue
		}
		if m.Point != nil {
			if !e.Bounds.Contains(*m.Point) {
				t.Errorf("element %d: point %v outside %v", i, *m.Point, e.Bounds)
			}
		} else if !same(m.Element, e) {
			t.Errorf("element %d: %+v found another element", i, sel)
		}
	}
	if sel := SelectorFor(s, s.Elements[3]); sel.ID != "cart" || len(sel.Or) != 1 || sel.Or[0].Label != "Cart · 1" {
		t.Errorf("a unique id comes first, the label second: %+v", sel)
	}
	if sel := SelectorFor(s, s.Elements[2]); sel.Index != 2 || len(sel.Or) != 1 || sel.Or[0].Index != 2 {
		t.Errorf("a repeated id and label need an index: %+v", sel)
	}
	if sel := SelectorFor(s, s.Elements[4]); sel.Point == nil {
		t.Errorf("an element without id and label is a point: %+v", sel)
	}
}

func TestCheckpoint(t *testing.T) {
	before := demoScreen()
	after := demoScreen()
	after.Elements = append(after.Elements,
		screen.Element{Role: screen.RoleText, Label: "Total $12.80"},
		screen.Element{Role: screen.RoleButton, Label: "Place order", ID: "place"},
		screen.Element{Role: screen.RoleText, Label: "Your cart"},
		screen.Element{Role: screen.RoleText, Label: "Delivery"})
	c := Checkpoint(before, after)
	if c == nil || c.App != "app.dinely" || len(c.Visible) != 1 || c.Visible[0].ID != "place" {
		t.Fatalf("an id marks the screen, and text beside it is left out: %+v", c)
	}
	if ok, why := c.Check(after); !ok {
		t.Errorf("the checkpoint does not hold on its own screen: %s", why)
	}
	if ok, _ := c.Check(before); ok {
		t.Error("the checkpoint holds on the screen before")
	}
	if c := Checkpoint(before, demoScreen()); c != nil {
		t.Errorf("the same screen has no checkpoint: %+v", c)
	}
	// A feed: what people wrote is not what the screen is. Controls mark
	// it; text does only when there is nothing else, and never a sentence
	// or anything long.
	feed := demoScreen()
	feed.Elements = append(feed.Elements,
		screen.Element{Role: screen.RoleText, Label: "shinno"},
		screen.Element{Role: screen.RoleText, Label: "Beautiful, though the flow is not my type."},
		screen.Element{Role: screen.RoleButton, Label: "More"},
		screen.Element{Role: screen.RoleTab, Label: "Following"})
	if c := Checkpoint(before, feed); c == nil || len(c.Visible) != 2 || c.Visible[0].Label != "More" || c.Visible[1].Label != "Following" {
		t.Errorf("controls mark a screen before text does: %+v", c)
	}
	words := demoScreen()
	words.Elements = append(words.Elements,
		screen.Element{Role: screen.RoleText, Label: "Beautiful, though the flow is not my type."},
		screen.Element{Role: screen.RoleText, Label: "A title that goes on and on for far too long"},
		screen.Element{Role: screen.RoleText, Label: "Trending"})
	if c := Checkpoint(before, words); c == nil || len(c.Visible) != 1 || c.Visible[0].Label != "Trending" {
		t.Errorf("short text that is no sentence, when the screen has nothing else: %+v", c)
	}
	// An element that kept its id and changed its label was there before:
	// its id would hold before the step ran.
	counted := demoScreen()
	counted.Elements[3].Label = "Cart · 2"
	counted.Elements[0].Label = "All orders"
	if c := Checkpoint(before, counted); c != nil {
		t.Errorf("a changed label under a known id is no checkpoint: %+v", c)
	}
	other := demoScreen()
	other.App = screen.App{Name: "Chatter", ID: "app.chatter"}
	if c := Checkpoint(before, other); c == nil || c.App != "app.chatter" {
		t.Errorf("another app is a checkpoint by itself: %+v", c)
	}
}

func TestCompile(t *testing.T) {
	text := "pad thai, no peanuts"
	steps := []Step{{Launch: "app.dinely"}, {Type: &text, Into: &Selector{ID: "search"}}}
	f, err := Compile("dinely.search", "Search a dish", "app.dinely", steps, map[string]string{"dish": "pad thai", "unused": "zzz"})
	if err != nil {
		t.Fatal(err)
	}
	if got := *f.Steps[1].Type; got != "${dish}, no peanuts" {
		t.Errorf("typed text: %q", got)
	}
	if p, ok := f.Params["dish"]; !ok || p.Default == nil || *p.Default != "pad thai" {
		t.Errorf("params: %+v", f.Params)
	}
	if _, ok := f.Params["unused"]; ok {
		t.Error("a value no step types became a parameter")
	}
	if text != "pad thai, no peanuts" {
		t.Error("Compile changed the recorded step")
	}
}

func TestStore(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		st := NewStore(dir)
		if _, err := st.Load("a.b"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing flow: %v", err)
		}
		if _, err := st.Load("../etc"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("a path is not a name: %v", err)
		}
		f := &Flow{Name: "a.b", Steps: []Step{{Key: "back"}}}
		saved, err := st.Save(f)
		if err != nil || saved.Version != 1 {
			t.Fatalf("save: %+v %v", saved, err)
		}
		if saved, err = st.Save(f); err != nil || saved.Version != 2 {
			t.Fatalf("second save: %+v %v", saved, err)
		}
		got, err := st.Load("a.b")
		if err != nil || got.Version != 2 || got.Steps[0].Key != "back" {
			t.Fatalf("load: %+v %v", got, err)
		}
		if flows, broken := st.List(); len(flows) != 1 || len(broken) != 0 {
			t.Fatalf("list: %v %v", flows, broken)
		}
		if _, err := st.Save(&Flow{Name: "Bad"}); err == nil {
			t.Fatal("saved an invalid flow")
		}
	}
}
