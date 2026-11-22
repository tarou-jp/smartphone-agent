package flow

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// A task that went well is not a flow by itself: Pinky suggests keeping
// it, and the person decides. Suggestions are flows in a store of their
// own (the suggested folder beside the flows), so that they replay like
// flows while they wait, and keeping one moves its file over.
//
// Two tasks did the same when their steps are the same actions, whatever
// words they were asked in and whatever text they typed: Signature says
// so, and a task whose steps a flow already has adds its request to that
// flow (Asks) instead of making another.

// SuggestedDir is the folder, inside a store's folder, that holds the
// suggestions.
const SuggestedDir = "suggested"

// MaxSuggestions bounds the suggestions kept; the oldest go first.
const MaxSuggestions = 30

// Signature names what the flow does: its steps' actions in order, the
// parts they name by id, without the text they type, the words around
// them and their checkpoints. Two flows with the same signature replay
// the same way.
func (f *Flow) Signature() string {
	h := sha256.New()
	for _, s := range f.Steps {
		if s.Part != "" {
			h.Write([]byte("part " + s.Part))
		} else {
			h.Write([]byte(actionKey(s)))
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// Asked reports whether the request (in the form two requests are
// compared in, see the gateway's taskKey) is the flow's.
func (f *Flow) Asked(key string) bool {
	if key == "" {
		return false
	}
	if f.Task == key {
		return true
	}
	for _, a := range f.Asks {
		if a == key {
			return true
		}
	}
	return false
}

// AddAsk adds a request the flow's steps were run for, and reports
// whether it is new. The newest come first, and MaxAsks are kept.
func (f *Flow) AddAsk(key string) bool {
	if key == "" || f.Asked(key) {
		return false
	}
	if f.Task == "" {
		f.Task = key
		return true
	}
	f.Asks = append([]string{key}, f.Asks...)
	if len(f.Asks) > MaxAsks {
		f.Asks = f.Asks[:MaxAsks]
	}
	return true
}

// MaxAsks bounds the other requests a flow keeps.
const MaxAsks = 10

// Put writes the flow under its name as it is, keeping its version: for
// what Pinky notes about a flow (a request, a run) without changing its
// steps.
func (s *Store) Put(f *Flow) (*Flow, error) {
	if f.Version < 1 {
		return s.Save(f)
	}
	return s.write(f.clone())
}

// dismissedFile lists, one a line, the signatures of the suggestions the
// person turned down.
const dismissedFile = "dismissed.txt"

// maxDismissed bounds the signatures dismissedFile keeps.
const maxDismissed = 200

func (s *Store) dismissed() []string {
	if s.dir == "" {
		return append([]string(nil), s.memDismissed...)
	}
	data, err := os.ReadFile(filepath.Join(s.dir, dismissedFile))
	if err != nil {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Dismissed reports whether the person turned down a suggestion of that
// signature.
func (s *Store) Dismissed(sig string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.dismissed() {
		if d == sig {
			return true
		}
	}
	return false
}

// Dismiss remembers that the person turned down suggestions of that
// signature, so that the same steps are not suggested again.
func (s *Store) Dismiss(sig string) error {
	if sig == "" {
		return errors.New("flow: no signature to dismiss")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []string{sig}
	for _, d := range s.dismissed() {
		if d != sig && len(list) < maxDismissed {
			list = append(list, d)
		}
	}
	if s.dir == "" {
		s.memDismissed = list
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, dismissedFile), []byte(strings.Join(list, "\n")+"\n"), 0o600)
}
