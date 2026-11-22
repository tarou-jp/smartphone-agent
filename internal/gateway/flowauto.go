package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/flow"
)

// This file makes flows happen without anyone having to know about them,
// and without making more of them than the person wants
// (Options.AutoFlows):
//
//   - When an agent ends a task with outcome done, its steps are not a
//     flow yet: Pinky suggests keeping them (in the suggested store), and
//     the person keeps the suggestion as a flow or turns it down, in
//     Pinky's window. A suggestion replays like a flow while it waits, so
//     the same request is fast the second time either way.
//   - Two tasks did the same when their steps are the same actions
//     (flow.Signature), whatever words they were asked in. A task whose
//     steps a kept flow has adds its request to that flow; one whose
//     steps a suggestion has counts as another run of it. Steps the
//     person turned down are not suggested again.
//   - When an agent connects, the result lists the kept flows and says
//     which flow or suggestion answers the same request (offerFlows).
//   - A task that replayed one flow, hit a mismatch and finished by hand
//     keeps its steps as that flow's next version: the flow follows the
//     app without anyone repairing it.
//
// A flow is a list of parts: its steps name the parts of the app they are
// (flowparts.go) instead of copying them, so that a part that is repaired
// repairs every flow that uses it.

// offeredFlows bounds the flows a connect result lists.
const offeredFlows = 20

// suggestionDays is how long a suggestion waits without running again.
const suggestionDays = 30

// offerFlows adds the saved flows to the result of a new connection.
func (g *Gateway) offerFlows(res *Result, l *lease) *Result {
	if res == nil || res.IsError() {
		return res
	}
	flows, _ := g.flows.List()
	g.mu.Lock()
	key := flowKey(l.task)
	g.mu.Unlock()
	var same *flow.Flow
	for _, f := range flows {
		if f.Asked(key) {
			same = f
		}
	}
	if same == nil {
		// A suggestion the person has not decided on yet replays too.
		sugg, _ := g.suggested.List()
		for _, f := range sugg {
			if f.Asked(key) {
				same = f
			}
		}
	}
	if len(flows) > offeredFlows {
		flows = flows[:offeredFlows]
	}
	for _, f := range flows {
		res.Flows = append(res.Flows, flowInfo(f))
	}
	switch {
	case same != nil:
		res.Message += fmt.Sprintf(" This task ran before: the flow “%s” holds its plan. When the task is the same, run it again with phone_flow_run { flow: %q } instead of doing the steps by hand; it checks each screen on the way.",
			same.Name, same.Name)
	case len(flows) > 0:
		res.Message += " Flows the person kept are listed below. If one does what this task needs, replay it with phone_flow_run instead of doing its steps by hand."
	}
	// The apps' parts are what any task can be put together from, also
	// one that differs a little from a flow.
	return g.offerParts(res, l)
}

// loadAnyFlow loads a kept flow or, when none has the name, a suggestion.
func (g *Gateway) loadAnyFlow(name string) (*flow.Flow, *flow.Store, error) {
	f, err := g.flows.Load(name)
	if errors.Is(err, flow.ErrNotFound) {
		if s, serr := g.suggested.Load(name); serr == nil || !errors.Is(serr, flow.ErrNotFound) {
			return s, g.suggested, serr
		}
	}
	return f, g.flows, err
}

// taskKeyRunes bounds what a task is known by.
const taskKeyRunes = 500

// taskKey is the form in which two requests are compared: without case,
// with single spaces, and no longer than taskKeyRunes.
func taskKey(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(s)), " ")
	if r := []rune(s); len(r) > taskKeyRunes {
		s = string(r[:taskKeyRunes])
	}
	return s
}

// flowKey is what a task's flow belongs to: what the person asked for
// when the agent passed it, else the task's name.
func flowKey(t *task) string {
	if t.brief != "" {
		return t.brief
	}
	return taskKey(t.title)
}

// How a task that ended well left its steps (keptFlow.how).
const (
	keptSuggested = "suggested" // a new suggestion
	keptAgain     = "again"     // another run of a suggestion
	keptAsked     = "asked"     // a kept flow's steps, for another request
	keptRan       = "ran"       // a replay of a flow or suggestion, as it is
	keptRepaired  = "repaired"  // a flow's corrected steps, as its next version
)

// keptFlow is what keepFlows did with a task's steps.
type keptFlow struct {
	f   *flow.Flow
	how string
	// suggestion is set when f waits for the person in the suggested
	// store.
	suggestion bool
}

// stepText is the task's step that says what became of its steps, for
// the person; "" when there is nothing to say.
func (k keptFlow) stepText() string {
	name := k.f.Description
	if name == "" {
		name = k.f.Name
	}
	switch k.how {
	case keptSuggested:
		return "Suggested keeping these steps as a flow"
	case keptAgain:
		return fmt.Sprintf("Did the same steps as a suggested flow (%s)", plural(k.f.Runs, "run"))
	case keptAsked:
		return fmt.Sprintf("Did the steps of the flow “%s”", name)
	case keptRepaired:
		return fmt.Sprintf("Updated the flow “%s” with the steps that work now (version %d)", name, k.f.Version)
	}
	return ""
}

// agentText tells the agent what became of the task's steps.
func (k keptFlow) agentText() string {
	switch k.how {
	case keptSuggested, keptAgain:
		return fmt.Sprintf(" Pinky suggests to the person to keep the task's steps as a flow (“%s”, %s); the same request replays them meanwhile. You need not ask the person about it.",
			k.f.Name, plural(len(k.f.Steps), "step"))
	case keptAsked:
		return fmt.Sprintf(" The task's steps are those of the flow “%s”, which now answers this request too: phone_flow_run replays it the next time.", k.f.Name)
	case keptRepaired:
		return fmt.Sprintf(" Pinky updated the flow “%s” with the task's steps (version %d).", k.f.Name, k.f.Version)
	}
	return ""
}

// keepFlows takes the steps of the session's tasks that are about to end
// well (all of them, or the one on the named device) into the flows and
// suggestions, and returns what it did per lease. A task's steps count
// when none are missing; a new suggestion needs at least two steps the
// agent made itself or put together.
func (s *Session) keepFlows(device string, parts map[*lease][]string) map[*lease]keptFlow {
	g := s.g
	type job struct {
		l        *lease
		title    string
		key      string
		steps    []flow.Step
		byHand   int
		replayed map[string]bool
	}
	var jobs []job
	g.mu.Lock()
	for _, l := range s.leases {
		if device != "" {
			if d := g.findDeviceLocked(device); d == nil || d != l.dev {
				continue
			}
		}
		t := l.task
		if t.test || t.traceGap {
			continue
		}
		j := job{l: l, title: t.title, key: flowKey(t), steps: append([]flow.Step(nil), t.trace...), byHand: t.byHand, replayed: map[string]bool{}}
		for name, broke := range t.replayed {
			j.replayed[name] = broke
		}
		jobs = append(jobs, j)
	}
	g.mu.Unlock()

	day := time.Now().Format("2006-01-02")
	kept := map[*lease]keptFlow{}
	for _, j := range jobs {
		if k, ok := g.keepTaskFlow(j.title, j.key, j.steps, parts[j.l], j.byHand, j.replayed, day); ok {
			kept[j.l] = k
			if k.suggestion {
				g.mu.Lock()
				j.l.task.suggestion = k.f.Name
				g.mu.Unlock()
			}
		}
	}
	g.trimSuggestions(time.Now())
	return kept
}

// keepTaskFlow does what keepFlows does for one task.
func (g *Gateway) keepTaskFlow(title, key string, steps []flow.Step, parts []string, byHand int, replayed map[string]bool, day string) (keptFlow, bool) {
	// The flow the task replayed, when it replayed one.
	var old *flow.Flow
	var oldStore *flow.Store
	broke := false
	if len(replayed) == 1 {
		for name, b := range replayed {
			if f, st, err := g.loadAnyFlow(name); err == nil {
				old, oldStore, broke = f, st, b
			}
		}
	}
	note := func(f *flow.Flow) {
		f.AddAsk(key)
		f.Runs++
		f.Last = day
	}
	if byHand == 0 {
		// The task only replayed: a flow that ran to its end ran once
		// more, and answers this request too.
		if old == nil || broke {
			return keptFlow{}, false
		}
		note(old)
		saved, err := oldStore.Put(old)
		if err != nil {
			g.log.Warn("flows: noting a replay", "flow", old.Name, "err", err)
			return keptFlow{}, false
		}
		return keptFlow{f: saved, how: keptRan, suggestion: oldStore == g.suggested}, true
	}
	if len(steps) < 2 {
		return keptFlow{}, false
	}
	f := autoFlow(title, key, steps, parts, g.flows)
	if err := f.Validate(); err != nil {
		g.log.Warn("flows: a task's steps do not make a flow", "task", title, "err", err)
		return keptFlow{}, false
	}
	if old != nil && broke {
		// One flow replayed, stopped, and the agent finished by hand: the
		// task's steps are that flow as it works now.
		f.Name, f.Description, f.Task, f.Asks, f.Runs, f.Last = old.Name, old.Description, old.Task, old.Asks, old.Runs, old.Last
		note(f)
		saved, err := oldStore.Save(f)
		if err != nil {
			g.log.Warn("flows: repairing a flow", "flow", old.Name, "err", err)
			return keptFlow{}, false
		}
		return keptFlow{f: saved, how: keptRepaired, suggestion: oldStore == g.suggested}, true
	}
	sig := f.Signature()
	for _, st := range []*flow.Store{g.flows, g.suggested} {
		list, _ := st.List()
		for _, same := range list {
			if same.Signature() != sig {
				continue
			}
			note(same)
			saved, err := st.Put(same)
			if err != nil {
				g.log.Warn("flows: noting a run", "flow", same.Name, "err", err)
				return keptFlow{}, false
			}
			how := keptAsked
			if st == g.suggested {
				how = keptAgain
			}
			return keptFlow{f: saved, how: how, suggestion: st == g.suggested}, true
		}
	}
	if g.suggested.Dismissed(sig) {
		return keptFlow{}, false
	}
	// The same request with other steps: a suggestion of it stays (the
	// person may keep it or turn it down), and next to a kept flow the
	// suggestion gets a name of its own.
	if _, err := g.suggested.Load(f.Name); err == nil {
		return keptFlow{}, false
	}
	base := f.Name
	for n := 2; ; n++ {
		if _, err := g.flows.Load(f.Name); err != nil {
			break
		}
		f.Name = fmt.Sprintf("%s-%d", base, n)
	}
	f.Runs, f.Last = 1, day
	saved, err := g.suggested.Save(f)
	if err != nil {
		g.log.Warn("flows: suggesting a task's steps", "task", title, "err", err)
		return keptFlow{}, false
	}
	return keptFlow{f: saved, how: keptSuggested, suggestion: true}, true
}

// trimSuggestions drops the suggestions that did not run again for
// suggestionDays, and the oldest past flow.MaxSuggestions.
func (g *Gateway) trimSuggestions(now time.Time) {
	list, _ := g.suggested.List()
	cutoff := now.AddDate(0, 0, -suggestionDays).Format("2006-01-02")
	sort.SliceStable(list, func(i, j int) bool { return list[i].Last > list[j].Last })
	for i, f := range list {
		if i >= flow.MaxSuggestions || f.Last != "" && f.Last < cutoff {
			_ = g.suggested.Remove(f.Name)
			g.clearSuggestion(f.Name)
		}
	}
}

// usedParts lists the parts that kept flows and suggestions name.
func (g *Gateway) usedParts() map[string]bool {
	used := map[string]bool{}
	for _, st := range []*flow.Store{g.flows, g.suggested} {
		list, _ := st.List()
		for _, f := range list {
			for _, s := range f.Steps {
				if s.Part != "" {
					used[s.Part] = true
				}
			}
		}
	}
	return used
}

// autoFlow makes a flow of a task's steps without anyone naming anything:
// it is named after the app and the task, described by the task's name,
// and takes the text the task typed as parameters, with the typed text as
// their defaults, so that a replay can type something else. A step that
// is a part of its app (parts[i], as keepParts returns them) is kept as
// the part's name, not as a copy: when the app changes and the part is
// repaired, the flow follows, and so does every other flow that names it.
func autoFlow(title, key string, steps []flow.Step, parts []string, store *flow.Store) *flow.Flow {
	f := &flow.Flow{Description: title, Task: key, App: flowApp(steps)}
	f.Name = autoFlowName(f.App, title, key)
	n := 0
	for i, st := range steps {
		var part *flow.Part
		if i < len(parts) && parts[i] != "" {
			_, part, _ = store.Part(parts[i])
		}
		typed := ""
		if st.Type != nil && *st.Type != "" {
			n++
			name := "text"
			if n > 1 {
				name = fmt.Sprintf("text%d", n)
			}
			typed = *st.Type
			where := "the focused field"
			if st.Into != nil {
				where = st.Into.String()
			}
			if f.Params == nil {
				f.Params = map[string]flow.Param{}
			}
			text := typed
			f.Params[name] = flow.Param{Description: "Text typed into " + where, Default: &text}
			ref := "${" + name + "}"
			st.Type = &ref
		}
		if part != nil {
			ref := flow.Step{Part: part.ID, Name: part.Does()}
			if part.Type != nil {
				ref.Type = st.Type
			}
			st = ref
		}
		f.Steps = append(f.Steps, st)
	}
	return f
}

// autoFlowName names a kept flow "<app>.<task>-<hash>": the last part of
// the app's id, the task's name as far as it is plain letters and digits,
// and a few digits of its hash, so that the same task name always gives
// the same flow and two tasks never share one.
func autoFlowName(app, title, key string) string {
	slug := func(s string, max int) string {
		var b strings.Builder
		dash := false
		for _, r := range strings.ToLower(s) {
			switch {
			case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
				if dash && b.Len() > 0 {
					b.WriteByte('-')
				}
				b.WriteRune(r)
				dash = false
			default:
				dash = true
			}
			if b.Len() >= max {
				break
			}
		}
		return b.String()
	}
	if i := strings.LastIndex(app, "."); i >= 0 {
		app = app[i+1:]
	}
	if app = slug(app, 20); app == "" {
		app = "task"
	}
	sum := sha256.Sum256([]byte(key))
	name := app + "."
	if t := slug(title, 24); t != "" {
		name += t + "-"
	}
	return name + hex.EncodeToString(sum[:3])
}

// clearSuggestion forgets, on the tasks that show it, a suggestion that
// is no longer one.
func (g *Gateway) clearSuggestion(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	clear := func(t *task) {
		if t != nil && t.suggestion == name {
			t.suggestion = ""
		}
	}
	for _, t := range g.ended {
		clear(t)
	}
	for _, d := range g.devices {
		if d.lease != nil {
			clear(d.lease.task)
		}
	}
}

// KeepSuggestion keeps a suggested flow as a flow, under the description
// the person gave it ("" keeps the suggested one), and returns it.
func (g *Gateway) KeepSuggestion(name, description string) (*flow.Flow, error) {
	name = strings.TrimSpace(name)
	f, err := g.suggested.Load(name)
	if err != nil {
		return nil, err
	}
	if d := strings.TrimSpace(description); d != "" {
		f.Description = d
	}
	// A flow kept meanwhile with the same steps takes the suggestion's
	// requests instead.
	sig := f.Signature()
	flows, _ := g.flows.List()
	var out *flow.Flow
	for _, same := range flows {
		if same.Signature() == sig {
			for _, a := range append([]string{f.Task}, f.Asks...) {
				same.AddAsk(a)
			}
			same.Runs += f.Runs
			out, err = g.flows.Put(same)
			break
		}
	}
	if out == nil && err == nil {
		base := f.Name
		for n := 2; ; n++ {
			if _, lerr := g.flows.Load(f.Name); lerr != nil {
				break
			}
			f.Name = fmt.Sprintf("%s-%d", base, n)
		}
		f.Version = 0
		out, err = g.flows.Save(f)
	}
	if err != nil {
		return nil, err
	}
	if err := g.suggested.Remove(name); err != nil && !errors.Is(err, flow.ErrNotFound) {
		return nil, err
	}
	g.clearSuggestion(name)
	g.emit(Event{Kind: EventTasks})
	return out, nil
}

// DismissSuggestion turns a suggestion down: it goes, and the same steps
// are not suggested again.
func (g *Gateway) DismissSuggestion(name string) error {
	name = strings.TrimSpace(name)
	f, err := g.suggested.Load(name)
	if err != nil {
		return err
	}
	if err := g.suggested.Dismiss(f.Signature()); err != nil {
		return err
	}
	if err := g.suggested.Remove(name); err != nil {
		return err
	}
	g.clearSuggestion(name)
	g.emit(Event{Kind: EventTasks})
	return nil
}
