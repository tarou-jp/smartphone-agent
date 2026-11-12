package flow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotes(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		s := NewStore(dir)
		const app = "com.dinely.app"
		if text, err := s.Notes(app); err != nil || text != "" {
			t.Fatalf("no notes yet: %q %v", text, err)
		}
		if added, err := s.AddNote(app, "  The tabs above the tab bar hide\nwhen the page scrolls. "); err != nil || !added {
			t.Fatalf("add: %v %v", added, err)
		}
		// The same note again, as a list item, is not kept twice.
		if added, err := s.AddNote(app, "- the tabs above the tab bar hide when the page scrolls."); err != nil || added {
			t.Fatalf("add again: %v %v", added, err)
		}
		if _, err := s.AddNote(app, "A tap on one of them scrolls the page to its top."); err != nil {
			t.Fatal(err)
		}
		text, _ := s.Notes(app)
		if text != "- The tabs above the tab bar hide when the page scrolls.\n- A tap on one of them scrolls the page to its top." {
			t.Fatalf("notes: %q", text)
		}
		// The person writes them as they like.
		if err := s.SaveNotes(app, "# Dinely\r\n\r\nOrders load slowly.  \r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		if text, _ = s.Notes(app); text != "# Dinely\n\nOrders load slowly." {
			t.Fatalf("saved: %q", text)
		}
		if apps := s.NoteApps(); len(apps) != 1 || apps[0] != app {
			t.Fatalf("apps: %v", apps)
		}
		if _, err := s.AddNote(app, strings.Repeat("x", MaxNote+1)); err == nil {
			t.Fatal("a note that is too long was kept")
		}
		if _, err := s.AddNote(app, " \n "); err == nil {
			t.Fatal("an empty note was kept")
		}
		if err := s.SaveNotes(app, strings.Repeat("x", MaxNotes+1)); !errors.Is(err, ErrNotesFull) {
			t.Fatalf("notes that are too long: %v", err)
		}
		if err := s.SaveNotes("../rules", "x"); err == nil {
			t.Fatal("notes were written outside the folder")
		}
		// An empty text removes them, and the file.
		if err := s.SaveNotes(app, " \n"); err != nil {
			t.Fatal(err)
		}
		if apps := s.NoteApps(); len(apps) != 0 {
			t.Fatalf("apps after removing: %v", apps)
		}
		if dir != "" {
			if _, err := os.Stat(filepath.Join(dir, "notes", app+".md")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("the file stayed: %v", err)
			}
		}
	}
}

func TestRemove(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		s := NewStore(dir)
		const app = "com.dinely.app"
		open := Step{Launch: app, Expect: &Condition{App: app}}
		orders := Step{Tap: &Selector{Role: "tab", Label: "Orders"}, Expect: &Condition{App: app, Visible: Selectors{{Role: "button", Label: "Reorder"}}}}
		ids, _, err := s.LearnParts([]Ran{{Step: open}, {Step: orders, App: app}}, time.Now())
		if err != nil || len(ids) != 2 || ids[0] == "" || ids[1] == "" {
			t.Fatalf("learn: %v %v", ids, err)
		}
		if ps, _ := s.Parts(app); len(ps.Parts[1].After) != 1 {
			t.Fatalf("after: %v", ps.Parts[1].After)
		}
		if err := s.RemovePart(ids[0]); err != nil {
			t.Fatal(err)
		}
		ps, _ := s.Parts(app)
		if len(ps.Parts) != 1 || ps.Parts[0].ID != ids[1] || len(ps.Parts[0].After) != 0 {
			t.Fatalf("parts after removing one: %+v", ps.Parts[0])
		}
		if err := s.RemovePart(ids[0]); !errors.Is(err, ErrNotFound) {
			t.Fatalf("removing it again: %v", err)
		}

		f, err := Parse([]byte("flow: dinely.orders\nsteps:\n  - launch: com.dinely.app\n"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Save(f); err != nil {
			t.Fatal(err)
		}
		if got := s.File("dinely.orders"); (got == "") != (dir == "") {
			t.Fatalf("file: %q", got)
		}
		if err := s.Remove("dinely.orders"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Load("dinely.orders"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("the flow stayed: %v", err)
		}
		if err := s.Remove("dinely.orders"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("removing it again: %v", err)
		}
	}
}
