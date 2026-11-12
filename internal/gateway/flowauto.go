package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/DIYgod/pinky/internal/flow"
)

// This file makes flows happen by themselves (Options.AutoFlows), so that
// nobody has to know about them: a person gives an agent the same task
// twice, and the second time it replays.
//
//   - When an agent ends a task with outcome done, the task's steps are
//     kept as a flow named after the task (keepFlows).
//   - When an agent connects, the result lists the flows there are and
//     says which one was recorded from a task of the same name
//     (offerFlows). Matching a task to a flow whose name differs is the
//     agent's part: it reads the list.
//   - A task that replayed one flow, hit a mismatch and finished by hand
//     keeps its steps as that flow's next version: the flow follows the
//     app without anyone repairing it.
//
// A kept flow is a list of parts: its steps name the parts of the app
// they are (flowparts.go) instead of copying them, so that a part that is
// repaired repairs every flow that uses it. Any task, also one no flow
// replays, is put together from the same parts.

// offeredFlows bounds the flows a connect result lists.
const offeredFlows = 20

// offerFlows adds the saved flows to the result of a new connection.
func (g *Gateway) offerFlows(res *Result, l *lease) *Result {
	if res == nil || res.IsError() {
		return res
	}
	flows, _ := g.flows.List()
	if len(flows) == 0 {
		return g.offerParts(res, l)
	}
	g.mu.Lock()
	key := flowKey(l.task)
	g.mu.Unlock()
	var same *flow.Flow
	for _, f := range flows {
		if f.Task != "" && taskKey(f.Task) == key {
			same = f
		}
	}
	if len(flows) > offeredFlows {
		flows = flows[:offeredFlows]
	}
	for _, f := range flows {
		res.Flows = append(res.Flows, flowInfo(f))
	}
	if same != nil {
		res.Message += fmt.Sprintf(" This task ran before: the flow “%s” holds its plan. When the task is the same, run it again with phone_flow_run { flow: %q } instead of doing the steps by hand; it checks each screen on the way.",
			same.Name, same.Name)
	} else {
		res.Message += " Flows kept from earlier tasks are listed below. If one does what this task needs, replay it with phone_flow_run instead of doing its steps by hand."
	}
	// The apps' parts are what any task can be put together from, also
	// one that differs a little from a flow.
	return g.offerParts(res, l)
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

// flowKey is what a task's kept flow belongs to: what the person asked
// for when the agent passed it, else the task's name.
func flowKey(t *task) string {
	if t.brief != "" {
		return t.brief
	}
	return taskKey(t.title)
}

// keepFlows saves the steps of the session's tasks that are about to end
// well (all of them, or the one on the named device) as flows, and returns
// what it saved per lease. A task is kept when the agent made steps of its
// own in it, at least two steps ran, and none are missing; a task that
// only replayed a flow leaves the flow as it is.
func (s *Session) keepFlows(device string, parts map[*lease][]string) map[*lease]*flow.Flow {
	g := s.g
	type job struct {
		l        *lease
		title    string
		key      string
		steps    []flow.Step
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
		if t.test || t.byHand == 0 || len(t.trace) < 2 || t.traceGap {
			continue
		}
		j := job{l: l, title: t.title, key: flowKey(t), steps: append([]flow.Step(nil), t.trace...), replayed: map[string]bool{}}
		for name, broke := range t.replayed {
			j.replayed[name] = broke
		}
		jobs = append(jobs, j)
	}
	g.mu.Unlock()

	kept := map[*lease]*flow.Flow{}
	for _, j := range jobs {
		f := autoFlow(j.title, j.key, j.steps, parts[j.l], g.flows)
		// One flow replayed, stopped, and the agent finished by hand: the
		// task's steps are that flow as it works now.
		repair := false
		if len(j.replayed) == 1 {
			for name, broke := range j.replayed {
				if old, err := g.flows.Load(name); err == nil && broke {
					f.Name, f.Description, f.Task = old.Name, old.Description, old.Task
					repair = true
				}
			}
		}
		// A flow that is there and was not tried stays: the same task on
		// two phones at once, or done by hand once more, must not replace
		// steps that work with another run's. Only a repair replaces.
		if _, err := g.flows.Load(f.Name); err == nil && !repair {
			continue
		}
		if err := f.Validate(); err != nil {
			g.log.Warn("flows: a task's steps do not make a flow", "task", j.title, "err", err)
			continue
		}
		saved, err := g.flows.Save(f)
		if err != nil {
			g.log.Warn("flows: keeping a task's steps", "task", j.title, "err", err)
			continue
		}
		kept[j.l] = saved
	}
	return kept
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
