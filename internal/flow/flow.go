// Package flow is Pinky's format for steps that replay without a model: a
// flow is a named list of taps, typing, swipes, keys and launches whose
// targets are selectors (an id, a role and label, text) instead of refs,
// with a checkpoint after each step that says which screen to expect.
//
// The package is pure: it parses and validates flows, binds parameters,
// resolves selectors and checks conditions against a screen.Screen, and
// turns recorded steps into a flow. The gateway runs flows (through the
// same pipeline as an agent's steps) and keeps them; see
// docs/notes/flows.md.
package flow

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Limits of a flow.
const (
	MaxSteps   = 200
	MaxNameLen = 64
	// DefaultTimeoutMS is how long a step waits for its target and for its
	// checkpoint when neither the step nor the flow says.
	DefaultTimeoutMS = 10000
	MaxTimeoutMS     = 45000
)

// Flow is a replayable list of steps.
type Flow struct {
	Name        string `yaml:"flow"`
	Description string `yaml:"description,omitempty"`
	// App is the app the flow is for, by id. It is a note for people and
	// agents; steps name the apps they open themselves.
	App     string `yaml:"app,omitempty"`
	Version int    `yaml:"version,omitempty"`
	// Task is the name of the task the flow was recorded from, when Pinky
	// kept a finished task's steps by itself: the same task replays it.
	Task string `yaml:"task,omitempty"`
	// Params are the values a run may pass; steps use them as ${name}.
	Params map[string]Param `yaml:"params,omitempty"`
	// TimeoutMS is the default for steps without their own.
	TimeoutMS int    `yaml:"timeout_ms,omitempty"`
	Steps     []Step `yaml:"steps"`
}

// Param is a value a run passes to a flow.
type Param struct {
	Description string `yaml:"description,omitempty"`
	// Default is used when the run does not pass the value; a parameter
	// without one is required.
	Default *string `yaml:"default,omitempty"`
}

// Step is one step: exactly one of Tap, Type, Swipe, Key, Launch, Assert
// and Part is set.
type Step struct {
	// Name says what the step does, for reports. Optional.
	Name string `yaml:"name,omitempty"`

	// Tap taps the element the selector finds.
	Tap       *Selector `yaml:"tap,omitempty"`
	LongPress bool      `yaml:"long_press,omitempty"`

	// Type types text into the field Into finds, or the focused field.
	Type   *string   `yaml:"type,omitempty"`
	Into   *Selector `yaml:"into,omitempty"`
	Append bool      `yaml:"append,omitempty"`
	Submit bool      `yaml:"submit,omitempty"`

	// Swipe moves the finger up, down, left or right, inside the element
	// In finds or on the whole screen; or from From to To.
	Swipe      string    `yaml:"swipe,omitempty"`
	In         *Selector `yaml:"in,omitempty"`
	From       *Point    `yaml:"from,omitempty"`
	To         *Point    `yaml:"to,omitempty"`
	DurationMS int       `yaml:"duration_ms,omitempty"`

	// Key presses a key by its phone_key name.
	Key string `yaml:"key,omitempty"`

	// Launch opens an app by id or name.
	Launch string `yaml:"launch,omitempty"`

	// Assert waits until the condition holds and does nothing else.
	Assert *Condition `yaml:"assert,omitempty"`

	// Part runs a part Pinky keeps for an app (parts.go), by its id: the
	// part's action and checkpoint take the step's place when the flow
	// runs. PartID is the part a step came from once that happened.
	Part   string `yaml:"part,omitempty"`
	PartID string `yaml:"-"`

	// Expect is the step's checkpoint: the screen the action leads to. The
	// run moves on as soon as it holds, and stops when it does not within
	// the timeout. Without it the run waits for the screen to settle.
	Expect *Condition `yaml:"expect,omitempty"`
	// TimeoutMS bounds the wait for the target and for the checkpoint.
	TimeoutMS int `yaml:"timeout_ms,omitempty"`
	// Optional steps are skipped when their target is not on the screen,
	// such as closing a dialog that shows only sometimes.
	Optional bool `yaml:"optional,omitempty"`
	// Intent is shown to the person when the step needs their approval.
	Intent string `yaml:"intent,omitempty"`
}

// Point is a place on the screen as fractions of its width and height, so
// that it holds on another screen size: {x: 0.5, y: 0.9}.
type Point struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
}

// Kind names the step's action: tap, type, swipe, key, launch, assert or
// part.
func (s Step) Kind() string {
	switch {
	case s.Part != "":
		return "part"
	case s.Tap != nil:
		return "tap"
	case s.Type != nil:
		return "type"
	case s.Swipe != "" || s.From != nil || s.To != nil:
		return "swipe"
	case s.Key != "":
		return "key"
	case s.Launch != "":
		return "launch"
	case s.Assert != nil:
		return "assert"
	}
	return ""
}

// Target is the selector the step acts on, nil when it has none.
func (s Step) Target() *Selector {
	switch s.Kind() {
	case "tap":
		return s.Tap
	case "type":
		return s.Into
	case "swipe":
		return s.In
	}
	return nil
}

// Describe says what the step does in a few words, for reports.
func (s Step) Describe() string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	switch s.Kind() {
	case "tap":
		if s.LongPress {
			return "long-press " + s.Tap.String()
		}
		return "tap " + s.Tap.String()
	case "type":
		t := fmt.Sprintf("type %q", clip(*s.Type, 40))
		if s.Into != nil {
			t += " into " + s.Into.String()
		}
		return t
	case "swipe":
		if s.From != nil {
			return "swipe between two points"
		}
		t := "swipe " + s.Swipe
		if s.In != nil {
			t += " in " + s.In.String()
		}
		return t
	case "key":
		return "press " + s.Key
	case "launch":
		return "open " + s.Launch
	case "assert":
		return "check " + s.Assert.String()
	case "part":
		return "part " + s.Part
	}
	return "step"
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ValidName reports whether name can name a flow: lower-case letters,
// digits, dots, dashes and underscores, starting with a letter or digit.
func ValidName(name string) bool {
	return len(name) <= MaxNameLen && nameRE.MatchString(name) && !strings.Contains(name, "..")
}

// Parse reads a flow from YAML and validates it. Unknown keys are errors,
// so a typo does not silently drop a checkpoint.
func Parse(data []byte) (*Flow, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f Flow
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("flow: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Marshal writes the flow as YAML.
func (f *Flow) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var directions = map[string]bool{"up": true, "down": true, "left": true, "right": true}

// Validate checks the flow's shape. It does not know which keys or apps a
// phone has; the run finds that out.
func (f *Flow) Validate() error {
	if !ValidName(f.Name) {
		return fmt.Errorf("flow: the name %q must be lower-case letters, digits, dots, dashes or underscores, at most %d characters", f.Name, MaxNameLen)
	}
	if len(f.Steps) == 0 {
		return errors.New("flow: steps is empty")
	}
	if len(f.Steps) > MaxSteps {
		return fmt.Errorf("flow: %d steps; the limit is %d", len(f.Steps), MaxSteps)
	}
	if err := checkTimeout(f.TimeoutMS); err != nil {
		return fmt.Errorf("flow: %w", err)
	}
	for name := range f.Params {
		if !paramNameRE.MatchString(name) {
			return fmt.Errorf("flow: the parameter name %q must be letters, digits or underscores", name)
		}
	}
	for i, s := range f.Steps {
		if err := s.validate(); err != nil {
			return fmt.Errorf("flow: step %d: %w", i+1, err)
		}
	}
	for _, name := range f.used() {
		if _, ok := f.Params[name]; !ok {
			return fmt.Errorf("flow: ${%s} is used but params does not list it", name)
		}
	}
	return nil
}

func checkTimeout(ms int) error {
	if ms < 0 || ms > MaxTimeoutMS {
		return fmt.Errorf("timeout_ms must be between 0 and %d", MaxTimeoutMS)
	}
	return nil
}

func (s Step) validate() error {
	if s.Part != "" {
		// A step that names a part takes the part's place: only the text
		// a typing part types, and the words around a step, are its own.
		if s.Tap != nil || s.Swipe != "" || s.From != nil || s.To != nil || s.Key != "" || s.Launch != "" || s.Assert != nil ||
			s.Into != nil || s.In != nil || s.LongPress || s.Append || s.Submit || s.DurationMS != 0 {
			return errors.New("a part brings its own action; besides part, a step may have type (the text a typing part types), name, intent, timeout_ms and optional")
		}
		if !ValidPartID(s.Part) {
			return fmt.Errorf("part: %q is not a part's id, such as p1a2b3c", s.Part)
		}
		if s.Expect != nil {
			return errors.New("a part brings its own checkpoint; leave expect out")
		}
		return checkTimeout(s.TimeoutMS)
	}
	n := 0
	for _, set := range []bool{s.Tap != nil, s.Type != nil, s.Swipe != "" || s.From != nil || s.To != nil, s.Key != "", s.Launch != "", s.Assert != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return errors.New("needs exactly one of tap, type, swipe, key, launch, assert and part")
	}
	if err := checkTimeout(s.TimeoutMS); err != nil {
		return err
	}
	kind := s.Kind()
	only := func(set bool, field, with string) error {
		if set && kind != with {
			return fmt.Errorf("%s goes with %s only", field, with)
		}
		return nil
	}
	for _, err := range []error{
		only(s.LongPress, "long_press", "tap"), only(s.Into != nil, "into", "type"),
		only(s.Append, "append", "type"), only(s.Submit, "submit", "type"),
		only(s.In != nil, "in", "swipe"), only(s.DurationMS != 0, "duration_ms", "swipe"),
	} {
		if err != nil {
			return err
		}
	}
	switch kind {
	case "tap":
		if err := s.Tap.validate(); err != nil {
			return fmt.Errorf("tap: %w", err)
		}
	case "type":
		if *s.Type == "" && !s.Submit {
			return errors.New("type: the text is empty; set submit to only press Enter")
		}
		if s.Into != nil {
			if err := s.Into.validate(); err != nil {
				return fmt.Errorf("into: %w", err)
			}
			if s.Into.Point != nil {
				return errors.New("into: a text field cannot be found by point")
			}
		}
	case "swipe":
		pts := s.From != nil || s.To != nil
		switch {
		case pts && s.Swipe != "":
			return errors.New("swipe takes a direction or from and to, not both")
		case pts && (s.From == nil || s.To == nil):
			return errors.New("swipe needs both from and to")
		case pts && s.In != nil:
			return errors.New("in goes with a direction, not with from and to")
		case pts && (!s.From.valid() || !s.To.valid()):
			return errors.New("from and to are fractions of the screen, between 0 and 1")
		case !pts && !directions[s.Swipe]:
			return errors.New("swipe must be up, down, left or right")
		}
		if s.DurationMS != 0 && (s.DurationMS < 80 || s.DurationMS > 1200) {
			return errors.New("duration_ms must be between 80 and 1200")
		}
		if s.In != nil {
			if err := s.In.validate(); err != nil {
				return fmt.Errorf("in: %w", err)
			}
			if s.In.Point != nil {
				return errors.New("in: an area cannot be found by point")
			}
		}
	case "assert":
		if err := s.Assert.validate(); err != nil {
			return fmt.Errorf("assert: %w", err)
		}
		if s.Expect != nil {
			return errors.New("assert is its own checkpoint; leave expect out")
		}
		if s.Optional {
			return errors.New("an assert cannot be optional")
		}
	}
	if s.Optional && s.Target() == nil {
		return errors.New("optional needs a step with a target to look for")
	}
	if s.Expect != nil {
		if err := s.Expect.validate(); err != nil {
			return fmt.Errorf("expect: %w", err)
		}
	}
	return nil
}

func (p *Point) valid() bool { return p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1 }

var (
	paramNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	paramRE     = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

// walk calls f on every string of the flow that may hold ${params}.
func (f *Flow) walk(fn func(*string)) {
	var sel func(*Selector)
	sel = func(s *Selector) {
		if s == nil {
			return
		}
		fn(&s.ID)
		fn(&s.Label)
		fn(&s.Contains)
		fn(&s.Text)
		for i := range s.Or {
			sel(&s.Or[i])
		}
	}
	cond := func(c *Condition) {
		if c == nil {
			return
		}
		fn(&c.App)
		for i := range c.Visible {
			sel(&c.Visible[i])
		}
		for i := range c.Gone {
			sel(&c.Gone[i])
		}
	}
	for i := range f.Steps {
		s := &f.Steps[i]
		sel(s.Tap)
		sel(s.Into)
		sel(s.In)
		if s.Type != nil {
			fn(s.Type)
		}
		fn(&s.Launch)
		fn(&s.Intent)
		cond(s.Assert)
		cond(s.Expect)
	}
}

// used lists the parameters the steps use, sorted.
func (f *Flow) used() []string {
	seen := map[string]bool{}
	f.walk(func(s *string) {
		for _, m := range paramRE.FindAllStringSubmatch(*s, -1) {
			seen[m[1]] = true
		}
	})
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Bind returns a copy of the flow with ${name} replaced by the values, or
// by the parameters' defaults. A value for a parameter the flow does not
// have, and a required parameter without a value, are errors.
func (f *Flow) Bind(values map[string]string) (*Flow, error) {
	for name := range values {
		if _, ok := f.Params[name]; !ok {
			return nil, fmt.Errorf("the flow %q has no parameter %q", f.Name, name)
		}
	}
	vals := map[string]string{}
	var missing []string
	for name, p := range f.Params {
		switch v, ok := values[name]; {
		case ok:
			vals[name] = v
		case p.Default != nil:
			vals[name] = *p.Default
		default:
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("the flow %q needs the parameter %s", f.Name, strings.Join(missing, ", "))
	}
	out := f.clone()
	out.walk(func(s *string) {
		*s = paramRE.ReplaceAllStringFunc(*s, func(m string) string {
			return vals[paramRE.FindStringSubmatch(m)[1]]
		})
	})
	return out, nil
}

// clone copies the flow deeply enough for Bind to change its strings.
func (f *Flow) clone() *Flow {
	out := *f
	out.Steps = make([]Step, len(f.Steps))
	for i, s := range f.Steps {
		s.Tap, s.Into, s.In = s.Tap.clone(), s.Into.clone(), s.In.clone()
		if s.Type != nil {
			t := *s.Type
			s.Type = &t
		}
		s.Assert, s.Expect = s.Assert.clone(), s.Expect.clone()
		out.Steps[i] = s
	}
	return &out
}
