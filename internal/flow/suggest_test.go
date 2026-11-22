package flow

import (
	"testing"
	"time"
)

// Two flows that do the same steps have one signature, whatever text they
// type and whatever words are around them; other steps, another one.
func TestSignature(t *testing.T) {
	a, b := "ramen", "${text}"
	f1 := &Flow{Name: "a", Description: "Find ramen", Steps: []Step{
		{Part: "p000001", Name: "open"},
		{Tap: &Selector{ID: "search"}, Intent: "search", Expect: &Condition{Visible: Selectors{{Label: "Results"}}}},
		{Type: &a, Into: &Selector{ID: "q"}},
	}}
	f2 := &Flow{Name: "b", Description: "Look for food", Steps: []Step{
		{Part: "p000001"},
		{Tap: &Selector{ID: "search"}},
		{Type: &b, Into: &Selector{ID: "q"}},
	}}
	if f1.Signature() != f2.Signature() {
		t.Fatal("the same steps have two signatures")
	}
	f2.Steps[0].Part = "p000002"
	if f1.Signature() == f2.Signature() {
		t.Fatal("other steps have the same signature")
	}
}

func TestAsks(t *testing.T) {
	f := &Flow{}
	if !f.AddAsk("open my orders") || f.Task != "open my orders" || f.AddAsk("open my orders") {
		t.Fatalf("the first request is the task: %+v", f)
	}
	if !f.AddAsk("show me my orders") || !f.Asked("show me my orders") || f.Asked("") || len(f.Asks) != 1 {
		t.Fatalf("another request: %+v", f)
	}
	for i := range MaxAsks + 3 {
		f.AddAsk(string(rune('a' + i)))
	}
	if len(f.Asks) != MaxAsks {
		t.Fatalf("%d requests kept", len(f.Asks))
	}
}

func TestDismissAndPut(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		s := NewStore(dir)
		if s.Dismissed("abc") {
			t.Fatal("nothing was dismissed")
		}
		if err := s.Dismiss("abc"); err != nil || !s.Dismissed("abc") || s.Dismissed("abd") {
			t.Fatalf("dismiss: %v", err)
		}
		f, err := s.Save(&Flow{Name: "x", Steps: []Step{{Key: "back"}}})
		if err != nil || f.Version != 1 {
			t.Fatalf("save: %+v %v", f, err)
		}
		f.Runs = 2
		if f, err = s.Put(f); err != nil || f.Version != 1 {
			t.Fatalf("put changed the version: %+v %v", f, err)
		}
		if f, _ = s.Load("x"); f.Runs != 2 {
			t.Fatalf("put did not write: %+v", f)
		}
	}
}

// What a page shows today is no part: a link without an id, a label with
// a count or a sentence in it.
func TestPartableControls(t *testing.T) {
	expect := &Condition{Visible: Selectors{{Label: "Next"}}}
	for _, c := range []struct {
		sel  Selector
		want bool
	}{
		{Selector{Role: "button", Label: "Search"}, true},
		{Selector{Role: "tab", Label: "Orders"}, true},
		{Selector{ID: "story:1", Role: "link", Label: "侍Jが台湾に5-0で勝利"}, true},
		{Selector{Role: "link", Label: "衆院憲法審 各党9条と自衛隊討議"}, false},
		{Selector{Role: "link", Label: "Terms"}, false},
		{Selector{Role: "button", Label: "Inbox 3"}, false},
		{Selector{Role: "button", Label: "Read more, it is good."}, false},
		{Selector{Role: "text", Label: "Hello"}, false},
	} {
		sel := c.sel
		if got := Partable(Step{Tap: &sel, Expect: expect}); got != c.want {
			t.Errorf("%+v: %v, want %v", c.sel, got, c.want)
		}
	}
}

// A step is offered once it worked twice, when the person showed it, or
// when it opens an app; one that worked once and not again goes.
func TestProvenAndPruned(t *testing.T) {
	s := NewStore("")
	tap := Step{Tap: &Selector{Role: "tab", Label: "Orders"}, Expect: &Condition{App: "a.b", Visible: Selectors{{Label: "Last week"}}}}
	other := Step{Tap: &Selector{Role: "tab", Label: "Account"}, Expect: &Condition{App: "a.b", Visible: Selectors{{Label: "Sign out"}}}}
	shown := Step{Tap: &Selector{Role: "tab", Label: "Home"}, Expect: &Condition{App: "a.b", Visible: Selectors{{Label: "Hi"}}}}
	open := Step{Launch: "a.b", Expect: &Condition{App: "a.b"}}
	old := time.Now().AddDate(0, 0, -40)
	if _, _, err := s.LearnParts([]Ran{{Step: open}, {Step: tap}, {Step: other}, {Step: shown, Taught: true}}, old); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LearnParts([]Ran{{Step: tap, App: "a.b"}}, old); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Parts("a.b")
	proven := map[string]bool{}
	for _, p := range ps.Parts {
		proven[p.Does()] = p.Proven()
	}
	if !proven[`open a.b`] || !proven[`tap tab "Orders"`] || proven[`tap tab "Account"`] || !proven[`tap tab "Home"`] {
		t.Fatalf("proven: %v", proven)
	}
	keep := map[string]bool{}
	if n := s.PruneParts(keep, time.Now()); n != 1 {
		t.Fatalf("pruned %d parts", n)
	}
	if ps, _ = s.Parts("a.b"); len(ps.Parts) != 3 {
		t.Fatalf("left: %d", len(ps.Parts))
	}
}
