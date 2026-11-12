package flow

import (
	"sort"
	"strings"
	"unicode"

	"github.com/DIYgod/pinky/internal/screen"
)

// SelectorFor writes a selector that finds e on sc, strongest first: its
// id when the screen has it once, with the role and label as the second
// way; otherwise the role and label, counted among their like; and the
// place on the screen when the element has neither.
func SelectorFor(sc *screen.Screen, e screen.Element) Selector {
	if e.Source == screen.SourceOCR {
		if t := strings.TrimSpace(e.Label); t != "" {
			s := Selector{Text: t}
			s.Index = indexAmong(sc, e, &s)
			if finds(&s, sc, e) {
				return s
			}
		}
		return pointSelector(sc, e)
	}
	var byLabel *Selector
	if strings.TrimSpace(e.Label) != "" {
		s := Selector{Role: e.Role, Label: strings.TrimSpace(e.Label)}
		s.Index = indexAmong(sc, e, &s)
		if finds(&s, sc, e) {
			byLabel = &s
		}
	}
	if e.ID != "" {
		s := Selector{ID: e.ID}
		s.Index = indexAmong(sc, e, &s)
		if finds(&s, sc, e) {
			// An id the screen has several times (the rows of a list) is
			// weaker than a label that names one of them.
			if s.Index > 0 && byLabel != nil && byLabel.Index == 0 {
				byLabel.Or = []Selector{s}
				return *byLabel
			}
			if byLabel != nil {
				s.Or = []Selector{*byLabel}
			}
			return s
		}
	}
	if byLabel != nil {
		return *byLabel
	}
	return pointSelector(sc, e)
}

// indexAmong is e's place among the elements s matches, counted from 1,
// or 0 when s matches only e.
func indexAmong(sc *screen.Screen, e screen.Element, s *Selector) int {
	found := s.matches(sc)
	if len(found) <= 1 {
		return 0
	}
	for i, f := range found {
		if same(f, e) {
			return i + 1
		}
	}
	return 0
}

func same(a, b screen.Element) bool {
	if a.Ref != 0 || b.Ref != 0 {
		return a.Ref == b.Ref
	}
	return a.Bounds == b.Bounds && a.Label == b.Label
}

func finds(s *Selector, sc *screen.Screen, e screen.Element) bool {
	m, err := s.Find(sc)
	return err == nil && m.Point == nil && same(m.Element, e)
}

func pointSelector(sc *screen.Screen, e screen.Element) Selector {
	return Selector{Point: PointAt(sc, e.Bounds.Center())}
}

// PointAt is a place on the screen as fractions of its size.
func PointAt(sc *screen.Screen, p screen.Point) *Point {
	if sc == nil || sc.Width <= 0 || sc.Height <= 0 {
		return &Point{X: 0.5, Y: 0.5}
	}
	return &Point{X: round3(float64(p.X) / float64(sc.Width)), Y: round3(float64(p.Y) / float64(sc.Height))}
}

func round3(f float64) float64 {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return float64(int(f*1000+0.5)) / 1000
}

// checkpointMarks is how many elements a checkpoint names at most. A few
// are enough to tell a screen from the one before it; more only make the
// checkpoint break on changes that do not matter.
const checkpointMarks = 2

// Checkpoint describes the screen a step led to, for the step's expect:
// the app in front and a few elements that the screen before did not
// have. It names what belongs to the screen rather than what the screen
// happens to show: ids first, then the labels of controls (buttons,
// tabs, switches), and plain text only when the screen has neither, and
// then only short text that is no sentence. A feed's posts and a list's
// rows change from run to run, as do digits (prices, times, counts). It
// returns nil when nothing tells the two screens apart, and the replay
// then waits for the screen to settle instead.
func Checkpoint(before, after *screen.Screen) *Condition {
	if after == nil {
		return nil
	}
	// What the screen before had, by id and by role and label: a mark must
	// be new in both senses. An element that kept its id and changed its
	// label (a counter, a display) was there before, so waiting for its id
	// would not wait for anything.
	hadID, hadLabel := map[string]bool{}, map[string]bool{}
	if before != nil {
		for _, e := range before.Elements {
			if e.ID != "" {
				hadID[e.ID] = true
			}
			hadLabel[labelKey(e)] = true
		}
	}
	type mark struct {
		sel  Selector
		rank int
	}
	var marks []mark
	seen := map[string]bool{}
	for _, e := range after.Elements {
		if e.Source == screen.SourceOCR {
			continue
		}
		label := strings.TrimSpace(e.Label)
		var m mark
		switch {
		case e.ID != "" && !hadID[e.ID] && countID(after, e.ID) == 1:
			m = mark{Selector{ID: e.ID}, 0}
		case e.ID != "" && hadID[e.ID]:
			continue
		case e.Role == screen.RoleInput:
			continue // a field's label is what was typed into it
		case label == "" || hadLabel[labelKey(e)] || volatile(label) || len([]rune(label)) > markRunes:
			continue
		case Control(e.Role):
			m = mark{Selector{Role: e.Role, Label: label}, 1}
		case !prose(label):
			m = mark{Selector{Role: e.Role, Label: label}, 2}
		default:
			continue
		}
		if k := m.sel.String(); !seen[k] {
			seen[k] = true
			marks = append(marks, m)
		}
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].rank < marks[j].rank })
	// Text is what a screen shows, which may be someone's post: it marks
	// a screen only when nothing of the screen's own does.
	if len(marks) > 0 && marks[0].rank < 2 {
		for i, m := range marks {
			if m.rank == 2 {
				marks = marks[:i]
				break
			}
		}
	}
	if len(marks) > checkpointMarks {
		marks = marks[:checkpointMarks]
	}
	c := &Condition{}
	for _, m := range marks {
		c.Visible = append(c.Visible, m.sel)
	}
	appChanged := before == nil || before.App.ID != after.App.ID || before.App.Name != after.App.Name
	if len(c.Visible) == 0 && !appChanged {
		return nil
	}
	if c.App = after.App.ID; c.App == "" {
		c.App = after.App.Name
	}
	if c.App == "" && len(c.Visible) == 0 {
		return nil
	}
	return c
}

func labelKey(e screen.Element) string { return e.Role + "\x00" + strings.TrimSpace(e.Label) }

func countID(sc *screen.Screen, id string) int {
	n := 0
	for _, e := range sc.Elements {
		if e.ID == id {
			n++
		}
	}
	return n
}

// markRunes bounds the label of an element a checkpoint names: longer
// labels are content, not names of things.
const markRunes = 30

// Control reports whether a role is something an app's screen is made of
// (a button, a tab, a switch, a checkbox, a link) rather than something
// it shows (text, an image).
func Control(role string) bool {
	switch role {
	case screen.RoleButton, screen.RoleTab, screen.RoleSwitch, screen.RoleCheckbox, screen.RoleLink:
		return true
	}
	return false
}

// prose reports whether a label reads like a sentence somebody wrote.
func prose(label string) bool {
	return strings.ContainsAny(label, "。、！？!?,;:…\n") || strings.Count(label, " ") > 3
}

// volatile reports whether a label looks like it changes between runs.
func volatile(label string) bool {
	return strings.IndexFunc(label, unicode.IsDigit) >= 0
}

// Compile makes a flow of recorded steps. values names literals to turn
// into parameters: wherever a step types one, the flow takes ${name}
// instead, with the literal as its default.
func Compile(name, description, app string, steps []Step, values map[string]string) (*Flow, error) {
	f := &Flow{Name: name, Description: strings.TrimSpace(description), App: app}
	f.Steps = (&Flow{Steps: steps}).clone().Steps
	names := make([]string, 0, len(values))
	for n, v := range values {
		if v != "" {
			names = append(names, n)
		}
	}
	// Longer literals first, so that one inside another is not cut up.
	sort.Slice(names, func(i, j int) bool {
		if a, b := len(values[names[i]]), len(values[names[j]]); a != b {
			return a > b
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		used := false
		for i := range f.Steps {
			if t := f.Steps[i].Type; t != nil && strings.Contains(*t, values[n]) {
				*t = strings.ReplaceAll(*t, values[n], "${"+n+"}")
				used = true
			}
		}
		if used {
			if f.Params == nil {
				f.Params = map[string]Param{}
			}
			v := values[n]
			f.Params[n] = Param{Default: &v}
		}
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return f, nil
}
