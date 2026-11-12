package flow

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestShotsKeptAndPruned(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		s := NewStore(dir)
		steps, afters := partSteps()
		ids, _, err := s.LearnParts(ran(steps, afters), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		var id string
		for _, i := range ids {
			if i != "" {
				id = i
			}
		}
		if id == "" {
			t.Fatalf("no part learned: %v", ids)
		}
		pic := []byte("\xff\xd8\xff fake jpeg")
		if err := s.SaveShot(id, pic); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Shot(id); err != nil || !bytes.Equal(got, pic) {
			t.Fatalf("Shot = %q, %v", got, err)
		}
		if !s.ShotIDs()[id] {
			t.Errorf("ShotIDs lacks %s", id)
		}
		// A picture of no part goes when pruned.
		if err := s.SaveShot("p000000", pic); err != nil {
			t.Fatal(err)
		}
		s.PruneShots()
		if _, err := s.Shot("p000000"); !errors.Is(err, ErrNotFound) {
			t.Errorf("orphan picture kept: %v", err)
		}
		if _, err := s.Shot(id); err != nil {
			t.Errorf("part's picture pruned: %v", err)
		}
		// Removing the part removes its picture.
		if err := s.RemovePart(id); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Shot(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("picture of a removed part: %v", err)
		}
		if err := s.SaveShot("../x", pic); err == nil {
			t.Error("SaveShot took a bad id")
		}
	}
}
