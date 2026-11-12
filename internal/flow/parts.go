package flow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/DIYgod/pinky/internal/screen"
)

// A part is one step that worked, with the screen it led to: "in this
// app, tapping Orders shows the list of orders". Pinky keeps the parts of
// every app as tasks run, and an agent puts them together into a replay
// for a task nobody ran before: open the app, this part, then that one.
// A whole flow replays one task; parts are what flows are made of.
//
// A part is known by its action and where the action leads. The same tap
// that leads to another screen (Back, from two places) is another part.
// A part keeps its id when it is repaired, so that the flows that name
// it follow the app with it: flows are lists of parts, and fixing a part
// fixes every flow that uses it.

// Limits of an app's parts.
const (
	// MaxParts bounds the parts kept for one app; the ones not seen for
	// the longest go first.
	MaxParts = 300
	// partBreaks is how many replays in a row may find a part's action
	// leading somewhere else before the part is no longer offered. It
	// stays, for the flows that name it, until a task repairs it.
	partBreaks = 2
	// PartText stands for the text a typing part types: the step that
	// names the part gives it.
	PartText = "${text}"
	// partAfter bounds the parts a part remembers being recorded after.
	partAfter = 3
)

// Part is a step with its checkpoint and what Pinky knows about it.
type Part struct {
	// ID names the part in a flow's "part:" step and in phone_flow_run's
	// steps: "p" and six hex digits of the app, the action and the
	// checkpoint it had when it was first kept.
	ID   string `yaml:"id"`
	Step `yaml:",inline"`
	// After lists the parts this one was recorded right after, newest
	// first: a hint at where its target shows.
	After []string `yaml:"after,omitempty"`
	// Seen counts the times the step ran and led to its checkpoint.
	Seen int `yaml:"seen,omitempty"`
	// Broke counts the replays since then that ran the action and did not
	// reach the checkpoint.
	Broke int `yaml:"broke,omitempty"`
	// Last is the day the part was last seen, as 2006-01-02.
	Last string `yaml:"last,omitempty"`
}

// Parts are the parts of one app.
type Parts struct {
	App   string  `yaml:"app"`
	Parts []*Part `yaml:"parts"`
}

var (
	partIDRE  = regexp.MustCompile(`^p[0-9a-f]{6}$`)
	partAppRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,150}$`)
)

// ValidPartID reports whether id has the form of a part's id.
func ValidPartID(id string) bool { return partIDRE.MatchString(id) }

// action is the step's action alone: what it does and to what, without
// its checkpoint and the words around it.
func (s Step) action() Step {
	a := Step{Tap: s.Tap, LongPress: s.LongPress, Swipe: s.Swipe, In: s.In,
		DurationMS: s.DurationMS, Key: s.Key, Launch: s.Launch}
	if s.Type != nil {
		// Typing is the field and how, whatever the text.
		text := PartText
		a.Type, a.Into, a.Append, a.Submit = &text, s.Into, s.Append, s.Submit
	}
	return a
}

func actionKey(s Step) string {
	data, _ := yaml.Marshal(s.action())
	return string(data)
}

func expectKey(s Step) string {
	data, _ := yaml.Marshal(s.Expect)
	return string(data)
}

// PartID is the id a new part gets for the step in the app, when no part
// has it yet.
func PartID(app string, s Step) string { return partID(app, s, 0) }

func partID(app string, s Step, n int) string {
	key := s.action()
	key.Expect = s.Expect
	data, _ := yaml.Marshal(key)
	salt := ""
	if n > 0 {
		salt = fmt.Sprintf("\n%d", n)
	}
	sum := sha256.Sum256(append([]byte(app+salt+"\n"), data...))
	return "p" + hex.EncodeToString(sum[:3])
}

// freeID is an id for a new part made of the step. A part keeps its id
// when it is repaired, so the id the step would get may be taken by a
// part that has since come to mean something else.
func (ps *Parts) freeID(s Step) string {
	for n := 0; ; n++ {
		if id := partID(ps.App, s, n); ps.find(id) == nil {
			return id
		}
	}
}

// Partable reports whether a recorded step is worth keeping as a part: it
// has a checkpoint, so that a replay can tell whether it led where it
// did before, and a target that belongs to the app rather than to what
// the app shows today. Typing into a field is a part without a checkpoint
// (what it leads to depends on the text, which the step that names the
// part gives). Left out are:
//
//   - taps on what a screen shows: text and images without an id, such as
//     a title in a list or a post in a feed. Tomorrow another title is
//     there, and a part that taps "the newest song" by its name of today
//     would tap the wrong one;
//   - taps at a place rather than on an element;
//   - swipes: scrolling shows more of the same screen, and what comes
//     into view is content;
//   - the home key, which nobody needs to be told about.
func Partable(s Step) bool {
	if s.Kind() == "type" {
		return s.Into != nil && s.Into.Point == nil
	}
	if s.Expect == nil {
		return false
	}
	switch s.Kind() {
	case "launch":
		return true
	case "key":
		return s.Key != "home"
	case "tap":
		return s.Tap.Point == nil && (s.Tap.ID != "" || Control(s.Tap.Role))
	}
	return false
}

// partApp is the app a recorded step's part belongs to: the app the step
// was made in, which is the app the step before led to.
func partApp(s Step, cur string) string {
	if s.Kind() == "launch" || cur == "" {
		if s.Expect != nil {
			return s.Expect.App
		}
		return ""
	}
	return cur
}

// find returns the part of that id.
func (ps *Parts) find(id string) *Part {
	for _, p := range ps.Parts {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Ran is a recorded step with what is known of how it ran.
type Ran struct {
	Step Step
	// After is the screen the step led to, nil when it is not known.
	After *screen.Screen
	// App is the app the step was made in, when the steps around it do
	// not say (a single step, without the launch before it).
	App string
	// Part is the part the step was run as, when a replay ran it.
	Part string
	// Repair lets the step say where Part leads now when it led elsewhere
	// than the part says: the step ran in a saved flow, and the task that
	// replayed it went on and ended well.
	Repair bool
}

// learn takes one recorded step into the app's parts and returns its
// part. An action that led to a screen a part already describes is that
// part again, even when the step's own checkpoint names other elements of
// the screen (a checkpoint names what the screen before did not have).
func (ps *Parts) learn(r Ran, prev, day string) (p *Part, added bool) {
	s, after := r.Step, r.After
	key := actionKey(s)
	typing := s.Type != nil
	if old := ps.find(r.Part); old != nil && !typing && r.Repair && after != nil && actionKey(old.Step) == key {
		// The part's own action, run by a flow that names it, led to
		// another screen than it says, and the task was fine with it:
		// this is where the part leads today. It keeps its id, so every
		// flow that names it follows.
		if ok, _ := old.Expect.Check(after); !ok && s.Expect != nil {
			old.Expect = s.Expect.clone()
		}
		p = old
	}
	for _, old := range ps.Parts {
		if p != nil || actionKey(old.Step) != key {
			continue
		}
		switch {
		case typing:
			p = old // typing into a field is one part, whatever it led to
		case after != nil:
			if ok, _ := old.Expect.Check(after); ok {
				p = old
			}
		case expectKey(old.Step) == expectKey(s):
			p = old // the screen is not known: the same checkpoint, then
		}
	}
	if p == nil {
		step := (&Flow{Steps: []Step{s}}).clone().Steps[0]
		step.Name, step.Optional, step.TimeoutMS, step.Part, step.PartID = "", false, 0, "", ""
		if typing {
			text := PartText
			step.Type, step.Expect = &text, nil
		}
		p = &Part{ID: ps.freeID(s), Step: step}
		ps.Parts = append(ps.Parts, p)
		added = true
	} else if strings.TrimSpace(s.Intent) != "" {
		p.Intent = s.Intent
	}
	p.Seen++
	p.Broke, p.Last = 0, day
	if prev != "" && prev != p.ID {
		list := []string{prev}
		for _, id := range p.After {
			if id != prev && len(list) < partAfter {
				list = append(list, id)
			}
		}
		p.After = list
	}
	return p, added
}

// trim drops the parts past MaxParts, longest unseen first, and the
// mentions of parts that are gone.
func (ps *Parts) trim() {
	if len(ps.Parts) > MaxParts {
		sort.SliceStable(ps.Parts, func(i, j int) bool {
			if a, b := ps.Parts[i].Last, ps.Parts[j].Last; a != b {
				return a > b
			}
			return ps.Parts[i].Seen > ps.Parts[j].Seen
		})
		ps.Parts = ps.Parts[:MaxParts]
	}
	have := map[string]bool{}
	for _, p := range ps.Parts {
		have[p.ID] = true
	}
	for _, p := range ps.Parts {
		kept := p.After[:0]
		for _, id := range p.After {
			if have[id] {
				kept = append(kept, id)
			}
		}
		if p.After = kept; len(kept) == 0 {
			p.After = nil
		}
	}
}

// Sorted returns the parts to offer, most often seen first, the newest
// first among those. Parts that broke and were not repaired are left out.
func (ps *Parts) Sorted() []*Part {
	var out []*Part
	for _, p := range ps.Parts {
		if p.Broke < partBreaks {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Seen != out[j].Seen {
			return out[i].Seen > out[j].Seen
		}
		return out[i].Last > out[j].Last
	})
	return out
}

// Text says what the part does and where it leads: "tap button “Orders” →
// button “Reorder” shows".
func (p *Part) Text() string {
	lead := ""
	if p.Expect != nil {
		c := *p.Expect
		if len(c.Visible)+len(c.Gone) > 0 {
			c.App = "" // the parts are listed under their app
		}
		lead = " → " + c.String()
	}
	return p.Does() + lead
}

// Does says what the part does, without where it leads: "tap button
// “Orders”".
func (p *Part) Does() string {
	st := p.Step
	st.Name = ""
	// A target found by id is named by its label as well, which says more
	// to whoever picks the part.
	st.Tap, st.In, st.Into = named(st.Tap), named(st.In), named(st.Into)
	if st.Type != nil {
		return "type <text> into " + st.Into.String()
	}
	return st.Describe()
}

// named is the selector as a list shows it: with the role and label of
// its fallback when it has none of its own.
func named(s *Selector) *Selector {
	if s == nil || s.Label != "" || s.Contains != "" || s.Text != "" || len(s.Or) == 0 || s.Or[0].Label == "" {
		return s
	}
	out := *s
	out.Role, out.Label = s.Or[0].Role, s.Or[0].Label
	return &out
}

func (ps *Parts) validate() error {
	if !partAppRE.MatchString(ps.App) {
		return fmt.Errorf("parts: %q is not an app id", ps.App)
	}
	seen := map[string]bool{}
	for i, p := range ps.Parts {
		if p == nil || !ValidPartID(p.ID) {
			return fmt.Errorf("parts: part %d has no id of the form p1a2b3c", i+1)
		}
		if seen[p.ID] {
			return fmt.Errorf("parts: the id %s is there twice", p.ID)
		}
		seen[p.ID] = true
		if err := p.Step.validate(); err != nil {
			return fmt.Errorf("parts: %s: %w", p.ID, err)
		}
		if !Partable(p.Step) {
			return fmt.Errorf("parts: %s: a part is a tap on a control, a key or a launch with an expect, or typing into a field", p.ID)
		}
	}
	return nil
}

func parseParts(data []byte) (*Parts, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var ps Parts
	if err := dec.Decode(&ps); err != nil {
		return nil, fmt.Errorf("parts: %w", err)
	}
	// Parts of a kind Pinky kept once and no longer does (swipes, taps on
	// text) are dropped when the file is read.
	kept := ps.Parts[:0]
	for _, p := range ps.Parts {
		if p != nil && Partable(p.Step) {
			kept = append(kept, p)
		}
	}
	ps.Parts = kept
	if err := ps.validate(); err != nil {
		return nil, err
	}
	return &ps, nil
}

func (ps *Parts) marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(ps); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Resolve returns a copy of the flow whose "part:" steps are the parts
// they name: the part's action and checkpoint, with the step's own name,
// intent, timeout and optional when it sets them, and for a part that
// types, the step's text. find looks a part up by id.
func (f *Flow) Resolve(find func(id string) (Step, bool)) (*Flow, error) {
	out := f.clone()
	for i, s := range out.Steps {
		if s.Part == "" {
			continue
		}
		st, ok := find(s.Part)
		if !ok {
			return nil, fmt.Errorf("step %d: no part has the id %q", i+1, s.Part)
		}
		st = (&Flow{Steps: []Step{st}}).clone().Steps[0]
		switch {
		case st.Type != nil && s.Type == nil:
			return nil, fmt.Errorf("step %d: the part %s types text: give the text as type", i+1, s.Part)
		case st.Type == nil && s.Type != nil:
			return nil, fmt.Errorf("step %d: the part %s does not type: leave type out", i+1, s.Part)
		case s.Type != nil:
			text := *s.Type
			st.Type = &text
		}
		st.Part, st.PartID = "", s.Part
		st.Name, st.Optional, st.TimeoutMS = s.Name, s.Optional, s.TimeoutMS
		if s.Intent != "" {
			st.Intent = s.Intent
		}
		if st.Optional && st.Target() == nil {
			return nil, fmt.Errorf("step %d: the part %s has no target to look for, so it cannot be optional", i+1, s.Part)
		}
		out.Steps[i] = st
	}
	return out, nil
}

// The store keeps an app's parts in parts/<app id>.yaml beside the flows.

const partsDir = "parts"

func (s *Store) partsPath(app string) string {
	return filepath.Join(s.dir, partsDir, app+".yaml")
}

func (s *Store) readParts(app string) (*Parts, error) {
	if !partAppRE.MatchString(app) || strings.Contains(app, "..") {
		return nil, ErrNotFound
	}
	var data []byte
	if s.dir == "" {
		d, ok := s.memParts[app]
		if !ok {
			return nil, ErrNotFound
		}
		data = d
	} else {
		d, err := os.ReadFile(s.partsPath(app))
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		data = d
	}
	ps, err := parseParts(data)
	if err != nil {
		return nil, err
	}
	if ps.App != app {
		return nil, fmt.Errorf("parts: %s.yaml holds the parts of %q", app, ps.App)
	}
	return ps, nil
}

func (s *Store) writeParts(ps *Parts) error {
	ps.trim()
	if err := ps.validate(); err != nil {
		return err
	}
	data, err := ps.marshal()
	if err != nil {
		return err
	}
	if s.dir == "" {
		if s.memParts == nil {
			s.memParts = map[string][]byte{}
		}
		s.memParts[ps.App] = data
		return nil
	}
	dir := filepath.Join(s.dir, partsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".parts-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.partsPath(ps.App))
}

// Parts returns the parts of an app, ErrNotFound when it has none.
func (s *Store) Parts(app string) (*Parts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readParts(app)
}

// SaveParts writes an app's parts as given, for whoever edits them: a
// person fixing a part by hand, or a tool.
func (s *Store) SaveParts(ps *Parts) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeParts(ps)
}

// PartApps lists the apps that have parts, the one seen last first.
func (s *Store) PartApps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var apps []string
	if s.dir == "" {
		for a := range s.memParts {
			apps = append(apps, a)
		}
	} else if entries, err := os.ReadDir(filepath.Join(s.dir, partsDir)); err == nil {
		for _, e := range entries {
			if a, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && !e.IsDir() && partAppRE.MatchString(a) {
				apps = append(apps, a)
			}
		}
	}
	last := map[string]string{}
	kept := apps[:0]
	for _, a := range apps {
		ps, err := s.readParts(a)
		if err != nil || len(ps.Parts) == 0 {
			continue
		}
		for _, p := range ps.Parts {
			if p.Last > last[a] {
				last[a] = p.Last
			}
		}
		kept = append(kept, a)
	}
	sort.Slice(kept, func(i, j int) bool {
		if last[kept[i]] != last[kept[j]] {
			return last[kept[i]] > last[kept[j]]
		}
		return kept[i] < kept[j]
	})
	return kept
}

// Part finds a part by id among the parts of every app.
func (s *Store) Part(id string) (app string, p *Part, ok bool) {
	if !ValidPartID(id) {
		return "", nil, false
	}
	for _, a := range s.PartApps() {
		ps, err := s.Parts(a)
		if err != nil {
			continue
		}
		if p := ps.find(id); p != nil {
			return a, p, true
		}
	}
	return "", nil, false
}

// LearnParts takes a task's recorded steps, in the order they ran, into
// the parts of the apps they ran in. It returns, for each step, the id of
// its part ("" for a step that is none), and how many parts are new.
func (s *Store) LearnParts(ran []Ran, now time.Time) (ids []string, added int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	day := now.Format("2006-01-02")
	ids = make([]string, len(ran))
	apps := map[string]*Parts{}
	var order []string
	cur, prevApp, prev := "", "", ""
	for i, r := range ran {
		st := r.Step
		app := partApp(st, cur)
		if r.App != "" {
			app = r.App
		}
		if st.Expect != nil && st.Expect.App != "" {
			cur = st.Expect.App
		}
		if !Partable(st) || !partAppRE.MatchString(app) || strings.Contains(app, "..") {
			continue
		}
		ps := apps[app]
		if ps == nil {
			var rerr error
			if ps, rerr = s.readParts(app); errors.Is(rerr, ErrNotFound) {
				ps = &Parts{App: app}
			} else if rerr != nil {
				// A file somebody broke by hand is not replaced: its parts
				// may be worth more than this task's.
				return ids, added, rerr
			}
			apps[app] = ps
			order = append(order, app)
		}
		if app != prevApp {
			prev = ""
		}
		p, isNew := ps.learn(r, prev, day)
		if isNew {
			added++
		}
		prevApp, prev, ids[i] = app, p.ID, p.ID
	}
	for _, app := range order {
		if werr := s.writeParts(apps[app]); werr != nil && err == nil {
			err = werr
		}
	}
	return ids, added, err
}

// BreakPart notes that a replay ran the part's action and did not reach
// its checkpoint. A part that breaks twice in a row without being seen
// in between is no longer offered (it reports true then): the app
// changed. The part stays for the flows that name it, and the next task
// that makes the same action by hand says where it leads now.
func (s *Store) BreakPart(id string) (hidden bool) {
	app, _, ok := s.Part(id)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.readParts(app)
	if err != nil {
		return false
	}
	p := ps.find(id)
	if p == nil {
		return false
	}
	p.Broke++
	if err := s.writeParts(ps); err != nil {
		return false
	}
	return p.Broke >= partBreaks
}
