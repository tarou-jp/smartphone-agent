package flow

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/DIYgod/pinky/internal/screen"
)

// Selector finds one element on a screen. Every field that is set must
// match. Or lists weaker selectors to try, in order, when this one finds
// nothing: a flow names an element by its id first and by its role and
// label second, so that a replay survives a renamed id or label.
//
// In YAML a plain string is a label: `tap: Orders`.
type Selector struct {
	// ID is the platform's resource id or accessibility identifier.
	ID string `yaml:"id,omitempty"`
	// Role is a normalized role: button, input, text, tab, …
	Role string `yaml:"role,omitempty"`
	// Label is the element's whole label.
	Label string `yaml:"label,omitempty"`
	// Contains is a part of the label, matched without case.
	Contains string `yaml:"contains,omitempty"`
	// Text is a part of any text on the screen, including text that only
	// OCR reads, matched without case.
	Text string `yaml:"text,omitempty"`
	// Index picks one of several matches, counted from 1 in screen order.
	// Without it, several matches are a mismatch.
	Index int `yaml:"index,omitempty"`
	// Point is a place on the screen instead of an element, the last
	// resort for things the screen does not describe.
	Point *Point `yaml:"point,omitempty"`
	// Or are the selectors to try when this one finds nothing.
	Or []Selector `yaml:"or,omitempty"`
}

// UnmarshalYAML reads a selector from a mapping, or from a string as a
// label.
func (s *Selector) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*s = Selector{Label: n.Value}
		return nil
	}
	type plain Selector
	var p plain
	if err := strictDecode(n, &p); err != nil {
		return err
	}
	*s = Selector(p)
	return nil
}

// strictDecode decodes a node and refuses keys the target does not have.
// (Decoder.KnownFields does not reach types with their own UnmarshalYAML.)
func strictDecode(n *yaml.Node, out any) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: expected a mapping", n.Line)
	}
	known := map[string]bool{}
	for _, k := range yamlKeys(out) {
		known[k] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i]; !known[k.Value] {
			return fmt.Errorf("line %d: unknown key %q", k.Line, k.Value)
		}
	}
	return n.Decode(out)
}

func (s *Selector) empty() bool {
	return s.ID == "" && s.Role == "" && s.Label == "" && s.Contains == "" && s.Text == "" && s.Point == nil
}

func (s *Selector) validate() error {
	if s == nil || s.empty() {
		return errors.New("the selector is empty; give an id, role, label, contains, text or point")
	}
	if s.Index < 0 {
		return errors.New("index counts from 1")
	}
	if s.Point != nil {
		if !s.Point.valid() {
			return errors.New("point is a fraction of the screen, between 0 and 1")
		}
		if s.ID != "" || s.Role != "" || s.Label != "" || s.Contains != "" || s.Text != "" || s.Index != 0 {
			return errors.New("point stands alone; put it under or as the last resort")
		}
	}
	for i := range s.Or {
		if len(s.Or[i].Or) > 0 {
			return errors.New("or does not nest")
		}
		if err := s.Or[i].validate(); err != nil {
			return fmt.Errorf("or %d: %w", i+1, err)
		}
	}
	return nil
}

func (s *Selector) clone() *Selector {
	if s == nil {
		return nil
	}
	out := *s
	if s.Point != nil {
		p := *s.Point
		out.Point = &p
	}
	out.Or = append([]Selector(nil), s.Or...)
	for i := range out.Or {
		out.Or[i] = *out.Or[i].clone()
	}
	return &out
}

// String is the selector in a few words, for reports: `button "Orders"`.
func (s *Selector) String() string {
	if s == nil {
		return "nothing"
	}
	var parts []string
	if s.Role != "" {
		parts = append(parts, s.Role)
	}
	switch {
	case s.Label != "":
		parts = append(parts, fmt.Sprintf("%q", clip(s.Label, 40)))
	case s.Contains != "":
		parts = append(parts, fmt.Sprintf("“…%s…”", clip(s.Contains, 40)))
	case s.Text != "":
		parts = append(parts, fmt.Sprintf("text %q", clip(s.Text, 40)))
	}
	if s.ID != "" {
		parts = append(parts, "#"+s.ID)
	}
	if s.Point != nil {
		parts = append(parts, fmt.Sprintf("the point %.2f,%.2f", s.Point.X, s.Point.Y))
	}
	if s.Index > 0 {
		parts = append(parts, fmt.Sprintf("(match %d)", s.Index))
	}
	if len(parts) == 0 && len(s.Or) > 0 {
		return s.Or[0].String()
	}
	return strings.Join(parts, " ")
}

// NeedsOCR reports whether finding the selector may need text that only
// OCR reads.
func (s *Selector) NeedsOCR() bool {
	if s == nil {
		return false
	}
	if s.Text != "" {
		return true
	}
	for i := range s.Or {
		if s.Or[i].Text != "" {
			return true
		}
	}
	return false
}

// Match is what a selector found.
type Match struct {
	// Element is the element found; its Ref is 0 for text only OCR reads.
	// Unset when Point is.
	Element screen.Element
	// Point is set when the selector names a place instead of an element.
	Point *screen.Point
	// By is the alternative that matched: 0 for the selector itself, n for
	// its nth or. A replay that keeps matching by a fallback has a
	// selector worth repairing.
	By int
}

// Find resolves the selector on a screen: the selector itself first, then
// each or. An alternative that matches several elements without an index
// does not count; when nothing else matches either, the error says so.
func (s *Selector) Find(sc *screen.Screen) (Match, error) {
	alts := append([]Selector{*s}, s.Or...)
	var ambiguous error
	for i := range alts {
		a := &alts[i]
		if a.empty() {
			continue // a selector that only lists alternatives
		}
		if a.Point != nil {
			p := screen.Point{X: int(a.Point.X * float64(sc.Width)), Y: int(a.Point.Y * float64(sc.Height))}
			return Match{Point: &p, By: i}, nil
		}
		found := a.matches(sc)
		switch {
		case len(found) == 0:
		case a.Index > 0:
			if a.Index <= len(found) {
				return Match{Element: found[a.Index-1], By: i}, nil
			}
		case len(found) == 1:
			return Match{Element: found[0], By: i}, nil
		case ambiguous == nil:
			ambiguous = fmt.Errorf("%d elements match %s; add an index or a stronger selector", len(found), a.String())
		}
	}
	if ambiguous != nil {
		return Match{}, ambiguous
	}
	return Match{}, fmt.Errorf("nothing on the screen matches %s", s.String())
}

// Present reports whether any alternative matches at least one element.
// Conditions use it: a checkpoint asks whether something shows, not which
// one of several.
func (s *Selector) Present(sc *screen.Screen) bool {
	alts := append([]Selector{*s}, s.Or...)
	for i := range alts {
		a := &alts[i]
		if a.empty() || a.Point != nil {
			continue
		}
		found := a.matches(sc)
		if a.Index > 0 && a.Index <= len(found) || a.Index == 0 && len(found) > 0 {
			return true
		}
	}
	return false
}

// matches lists the elements one alternative matches, in screen order.
func (s *Selector) matches(sc *screen.Screen) []screen.Element {
	var out []screen.Element
	for _, e := range sc.Elements {
		if s.match(e) {
			out = append(out, e)
		}
	}
	return out
}

func (s *Selector) match(e screen.Element) bool {
	ocr := e.Source == screen.SourceOCR
	// Only text looks at what OCR reads: ids, roles and labels describe
	// the app's own elements.
	if ocr && (s.Text == "" || s.ID != "" || s.Role != "" || s.Label != "" || s.Contains != "") {
		return false
	}
	if s.ID != "" && e.ID != s.ID {
		return false
	}
	if s.Role != "" && e.Role != s.Role {
		return false
	}
	if s.Label != "" && strings.TrimSpace(e.Label) != strings.TrimSpace(s.Label) {
		return false
	}
	if s.Contains != "" && !containsFold(e.Label, s.Contains) {
		return false
	}
	if s.Text != "" && !containsFold(e.Label, s.Text) {
		return false
	}
	return true
}

func containsFold(s, part string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(strings.TrimSpace(part)))
}

// Selectors is one selector or a list of them in YAML.
type Selectors []Selector

// UnmarshalYAML reads a list, or one selector as a list of one.
func (l *Selectors) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		out := make([]Selector, len(n.Content))
		for i, c := range n.Content {
			if err := out[i].UnmarshalYAML(c); err != nil {
				return err
			}
		}
		*l = out
		return nil
	}
	var one Selector
	if err := one.UnmarshalYAML(n); err != nil {
		return err
	}
	*l = Selectors{one}
	return nil
}

// Condition describes a screen: the app in front, what shows and what
// does not. Every part that is set must hold.
type Condition struct {
	// App is the app in front, by id or name.
	App string `yaml:"app,omitempty"`
	// Visible lists selectors that each match something.
	Visible Selectors `yaml:"visible,omitempty"`
	// Gone lists selectors that match nothing.
	Gone Selectors `yaml:"gone,omitempty"`
}

// UnmarshalYAML refuses unknown keys.
func (c *Condition) UnmarshalYAML(n *yaml.Node) error {
	type plain Condition
	var p plain
	if err := strictDecode(n, &p); err != nil {
		return err
	}
	*c = Condition(p)
	return nil
}

func (c *Condition) validate() error {
	if c == nil || (c.App == "" && len(c.Visible) == 0 && len(c.Gone) == 0) {
		return errors.New("the condition is empty; give app, visible or gone")
	}
	for i := range c.Visible {
		if err := c.Visible[i].validate(); err != nil {
			return fmt.Errorf("visible %d: %w", i+1, err)
		}
		if c.Visible[i].Point != nil {
			return errors.New("visible: a point is always there; name an element")
		}
	}
	for i := range c.Gone {
		if err := c.Gone[i].validate(); err != nil {
			return fmt.Errorf("gone %d: %w", i+1, err)
		}
		if c.Gone[i].Point != nil {
			return errors.New("gone: a point is always there; name an element")
		}
	}
	return nil
}

func (c *Condition) clone() *Condition {
	if c == nil {
		return nil
	}
	out := Condition{App: c.App}
	for i := range c.Visible {
		out.Visible = append(out.Visible, *c.Visible[i].clone())
	}
	for i := range c.Gone {
		out.Gone = append(out.Gone, *c.Gone[i].clone())
	}
	return &out
}

// NeedsOCR reports whether checking the condition may need text that only
// OCR reads.
func (c *Condition) NeedsOCR() bool {
	if c == nil {
		return false
	}
	for i := range c.Visible {
		if c.Visible[i].NeedsOCR() {
			return true
		}
	}
	for i := range c.Gone {
		if c.Gone[i].NeedsOCR() {
			return true
		}
	}
	return false
}

// String is the condition in a few words, for reports.
func (c *Condition) String() string {
	if c == nil {
		return "nothing"
	}
	var parts []string
	if c.App != "" {
		parts = append(parts, "app "+c.App)
	}
	for i := range c.Visible {
		parts = append(parts, c.Visible[i].String()+" shows")
	}
	for i := range c.Gone {
		parts = append(parts, c.Gone[i].String()+" is gone")
	}
	return strings.Join(parts, ", ")
}

// Check reports whether the condition holds on the screen; when it does
// not, why says which part fails first.
func (c *Condition) Check(sc *screen.Screen) (ok bool, why string) {
	if sc == nil {
		return false, "there is no screen"
	}
	if c.App != "" && !strings.EqualFold(sc.App.ID, c.App) && !strings.EqualFold(sc.App.Name, c.App) {
		app := sc.App.Name
		if app == "" {
			app = sc.App.ID
		}
		return false, fmt.Sprintf("the app in front is %s, not %s", app, c.App)
	}
	for i := range c.Visible {
		if !c.Visible[i].Present(sc) {
			return false, c.Visible[i].String() + " does not show"
		}
	}
	for i := range c.Gone {
		if c.Gone[i].Present(sc) {
			return false, c.Gone[i].String() + " still shows"
		}
	}
	return true, ""
}

// MarshalYAML writes a selector that is only a label as a plain string.
func (s Selector) MarshalYAML() (any, error) {
	if s.Label != "" && s.ID == "" && s.Role == "" && s.Contains == "" && s.Text == "" && s.Index == 0 && s.Point == nil && len(s.Or) == 0 {
		return s.Label, nil
	}
	type plain Selector
	return plain(s), nil
}

// yamlKeys lists the YAML keys of the struct out points to.
func yamlKeys(out any) []string {
	t := reflect.TypeOf(out)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			keys = append(keys, name)
		}
	}
	return keys
}
