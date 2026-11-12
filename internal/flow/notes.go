package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// This file keeps the notes on apps: what a person, or an agent while it
// worked, found out about how an app behaves and the screen does not say.
// "The row of tabs above the tab bar hides when the page scrolls." "A tap
// on one of those tabs scrolls the page back to its top." A part says what
// one step does; a note says what to expect of the app around it.
//
// The notes on an app are one text file, <store>/notes/<app id>.md, that
// people read, edit and keep under version control like the flows. Pinky
// shows them to an agent with the app's parts, and an agent adds to them
// one line at a time (AddNote).

// Limits of an app's notes.
const (
	// MaxNotes bounds the notes on one app, in bytes: they go to an agent
	// with every task in the app.
	MaxNotes = 6000
	// MaxNote bounds a note an agent adds, in characters.
	MaxNote = 300
)

// ErrNotesFull is returned for a note that does not fit in MaxNotes.
var ErrNotesFull = errors.New("the notes on the app are full")

const notesDir = "notes"

func (s *Store) notesPath(app string) string {
	return filepath.Join(s.dir, notesDir, app+".md")
}

// ValidApp reports whether app can name an app's parts and notes: an app
// id, or a name without anything a file name cannot hold.
func ValidApp(app string) bool {
	return partAppRE.MatchString(app) && !strings.Contains(app, "..")
}

// cleanNotes is the text as it is kept: with \n line ends, without spaces
// at the ends of lines or empty lines at either end.
func cleanNotes(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func (s *Store) readNotes(app string) (string, error) {
	if !ValidApp(app) {
		return "", ErrNotFound
	}
	if s.dir == "" {
		return s.memNotes[app], nil
	}
	data, err := os.ReadFile(s.notesPath(app))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("notes: %s.md is not text", app)
	}
	return cleanNotes(string(data)), nil
}

func (s *Store) writeNotes(app, text string) error {
	if !ValidApp(app) {
		return fmt.Errorf("notes: %q cannot name an app", app)
	}
	text = cleanNotes(text)
	if len(text) > MaxNotes {
		return fmt.Errorf("%w: %d bytes, and they may take %d", ErrNotesFull, len(text), MaxNotes)
	}
	if s.dir == "" {
		if s.memNotes == nil {
			s.memNotes = map[string]string{}
		}
		if text == "" {
			delete(s.memNotes, app)
		} else {
			s.memNotes[app] = text
		}
		return nil
	}
	if text == "" {
		if err := os.Remove(s.notesPath(app)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	dir := filepath.Join(s.dir, notesDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".notes-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.notesPath(app))
}

// Notes returns the notes on an app, "" when there are none.
func (s *Store) Notes(app string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readNotes(app)
}

// SaveNotes replaces the notes on an app; an empty text removes them.
func (s *Store) SaveNotes(app, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeNotes(app, text)
}

// AddNote adds one note to the notes on an app, as a line of its own, and
// reports whether it was added: a note the app already has is not. The
// note is one line of at most MaxNote characters.
func (s *Store) AddNote(app, note string) (added bool, err error) {
	note = strings.Join(strings.Fields(note), " ")
	note = strings.TrimSpace(strings.TrimLeft(note, "-*• "))
	if note == "" {
		return false, errors.New("notes: the note is empty")
	}
	if n := utf8.RuneCountInString(note); n > MaxNote {
		return false, fmt.Errorf("notes: the note has %d characters, and may have %d", n, MaxNote)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	text, err := s.readNotes(app)
	if err != nil {
		return false, err
	}
	for _, l := range strings.Split(text, "\n") {
		if strings.EqualFold(strings.TrimSpace(strings.TrimLeft(l, "-*• ")), note) {
			return false, nil
		}
	}
	if text != "" {
		text += "\n"
	}
	if err := s.writeNotes(app, text+"- "+note); err != nil {
		return false, err
	}
	return true, nil
}

// NoteApps lists the apps that have notes, sorted by name.
func (s *Store) NoteApps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var apps []string
	if s.dir == "" {
		for a := range s.memNotes {
			apps = append(apps, a)
		}
	} else if entries, err := os.ReadDir(filepath.Join(s.dir, notesDir)); err == nil {
		for _, e := range entries {
			if a, ok := strings.CutSuffix(e.Name(), ".md"); ok && !e.IsDir() && ValidApp(a) {
				apps = append(apps, a)
			}
		}
	}
	sort.Strings(apps)
	return apps
}

// Remove deletes the flow of that name.
func (s *Store) Remove(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("%q is not a flow name: %w", name, ErrNotFound)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		if _, ok := s.mem[name]; !ok {
			return ErrNotFound
		}
		delete(s.mem, name)
		return nil
	}
	err := os.Remove(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

// RemovePart deletes a part. Flows that name it stop at that step until
// they are repaired or the step is learned again.
func (s *Store) RemovePart(id string) error {
	app, _, ok := s.Part(id)
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.readParts(app)
	if err != nil {
		return err
	}
	kept := ps.Parts[:0]
	for _, p := range ps.Parts {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	if len(kept) == len(ps.Parts) {
		return ErrNotFound
	}
	ps.Parts = kept
	// Parts recorded after it no longer are.
	for _, p := range ps.Parts {
		after := p.After[:0]
		for _, a := range p.After {
			if a != id {
				after = append(after, a)
			}
		}
		p.After = after
	}
	if err := s.writeParts(ps); err != nil {
		return err
	}
	if s.dir == "" {
		delete(s.memShots, id)
	} else if err := os.Remove(s.shotPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
