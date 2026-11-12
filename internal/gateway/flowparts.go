package gateway

import (
	"fmt"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/screen"
)

// This file keeps and offers the apps' parts (internal/flow/parts.go):
// single steps that worked, each with the screen it leads to. A whole
// flow replays one task; parts let an agent replay a task nobody ran
// before, by putting together what earlier tasks found out about the app.
//
//   - When a task ends, every step it ran goes into the parts of the app
//     it ran in (keepParts), whatever the outcome: where a tap led is a
//     fact either way.
//   - When an agent connects, or opens an app, the result lists that
//     app's parts (offerParts), and phone_flow_run { parts: [...] } runs
//     the ones the agent picks, in order, in one call.
//   - A part whose action stops leading where it did is dropped after
//     two replays (RunFlow), and learned again from the next task that
//     does the step by hand.

// planFlow names the replay phone_flow_run makes of an agent's plan.
const planFlow = "plan"

const (
	// offeredParts bounds the parts of one app a result lists; the ones
	// seen most often come first.
	offeredParts = 40
	// offeredPartApps bounds the apps a connect result lists parts of
	// when the task names none.
	offeredPartApps = 2
)

// PartInfo is a part as a result lists it.
type PartInfo struct {
	ID  string
	App string
	// Text says what the part does and where it leads.
	Text string
	// After lists the parts it was recorded right after.
	After []string
	Seen  int
}

func partInfo(app string, p *flow.Part) PartInfo {
	return PartInfo{ID: p.ID, App: app, Text: p.Text(), After: append([]string(nil), p.After...), Seen: p.Seen}
}

// appKey is the app in front on a screen as checkpoints name it.
func appKey(sc *screen.Screen) string {
	if sc == nil {
		return ""
	}
	if sc.App.ID != "" {
		return sc.App.ID
	}
	return sc.App.Name
}

// partStep finds a part's step for a flow that names it.
func (g *Gateway) partStep(id string) (flow.Step, bool) {
	_, p, ok := g.flows.Part(id)
	if !ok {
		return flow.Step{}, false
	}
	return p.Step, true
}

// keepParts takes the steps of the session's tasks that are about to end
// (all of them, or the one on the named device) into the apps' parts, and
// returns, per lease, the part each step of the task's trace is. done
// says the tasks end well: a part that a saved flow ran and that led
// elsewhere than it says is then repaired from what the step led to. A
// test run teaches nothing.
func (s *Session) keepParts(device string, done bool) map[*lease][]string {
	g := s.g
	type job struct {
		l     *lease
		title string
		ran   []flow.Ran
		shots [][]byte // per step, the picture of the screen it led to
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
		if t.test || len(t.trace) == 0 {
			continue
		}
		j := job{l: l, title: t.title}
		for i, st := range t.trace {
			j.ran = append(j.ran, flow.Ran{Step: st, After: t.traceAfter[i], Part: t.tracePart[i], Repair: done && t.traceSaved[i]})
			var thumb []byte
			// Only a screenshot a later read confirmed shows where the
			// step led.
			if i < len(t.traceShot) && t.traceShot[i] != nil && t.traceShot[i].state == shotKept {
				thumb = t.traceShot[i].thumb
			}
			j.shots = append(j.shots, thumb)
		}
		jobs = append(jobs, j)
	}
	g.mu.Unlock()
	out := map[*lease][]string{}
	for _, j := range jobs {
		ids, added, err := g.flows.LearnParts(j.ran, time.Now())
		if err != nil {
			g.log.Warn("flows: keeping a task's steps as parts", "task", j.title, "err", err)
		} else if added > 0 {
			g.log.Info("flows: new parts", "task", j.title, "parts", added)
		}
		for i, id := range ids {
			if id == "" || i >= len(j.shots) || j.shots[i] == nil {
				continue
			}
			if err := g.flows.SaveShot(id, j.shots[i]); err != nil {
				g.log.Warn("flows: keeping a part's picture", "part", id, "err", err)
			}
		}
		out[j.l] = ids
	}
	return out
}

// offerParts adds the parts of apps to a result, for the apps the agent
// was not shown yet in this task. Without apps it lists the parts of the
// apps the task is held to, or of the apps seen last.
func (g *Gateway) offerParts(res *Result, l *lease, apps ...string) *Result {
	if res == nil || res.IsError() {
		return res
	}
	// What people and agents wrote down about the apps goes with them.
	g.offerNotes(res, l, apps...)
	known := g.flows.PartApps()
	if len(known) == 0 {
		return res
	}
	if len(apps) == 0 {
		g.mu.Lock()
		scope := append([]string(nil), l.apps...)
		g.mu.Unlock()
		for _, a := range known {
			if len(scope) == 0 && len(apps) < offeredPartApps || containsFoldAny(scope, a) {
				apps = append(apps, a)
			}
		}
	}
	var names []string
	for _, app := range apps {
		g.mu.Lock()
		shown := l.task.partsShown[app]
		g.mu.Unlock()
		if shown || app == "" {
			continue
		}
		ps, err := g.flows.Parts(app)
		if err != nil || len(ps.Parts) == 0 {
			continue
		}
		list := ps.Sorted()
		if len(list) > offeredParts {
			list = list[:offeredParts]
		}
		for _, p := range list {
			res.Parts = append(res.Parts, partInfo(app, p))
		}
		names = append(names, app)
		g.mu.Lock()
		if l.task.partsShown == nil {
			l.task.partsShown = map[string]bool{}
		}
		l.task.partsShown[app] = true
		g.mu.Unlock()
	}
	if len(names) > 0 {
		res.Message = strings.TrimSpace(res.Message + fmt.Sprintf(" Parts of %s are listed below: single steps that worked before, each with the screen it leads to. Plan the task from them and run the whole plan in one call, in order, with phone_flow_run { parts: [\"p…\", \"p…\"] }, instead of tapping and reading the screen step by step: each part finds its target, acts and checks the screen it leads to, so you need not look in between. To mix in other steps, pass steps instead: [{\"part\": \"p…\"}, {\"tap\": \"A label on the screen\"}, {\"part\": \"p…\", \"type\": \"text for a part that types\"}, {\"assert\": {\"visible\": \"Text that must show\"}}]. A part works wherever its target is on the screen (“after” names a part it was recorded after), and the call stops with mismatch, showing the screen, where a step does not fit. Do one at a time only what you cannot plan.",
			strings.Join(names, " and ")))
	}
	return res
}

func containsFoldAny(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// listParts is phone_flow_list with app: the parts kept for the app whose
// id is, ends with or holds the given text.
func (g *Gateway) listParts(want string) *Result {
	known := g.flows.PartApps()
	want = strings.ToLower(strings.TrimSpace(want))
	app := ""
	for _, pass := range []func(a string) bool{
		func(a string) bool { return a == want },
		func(a string) bool { return strings.HasSuffix(a, "."+want) },
		func(a string) bool { return strings.Contains(a, want) },
	} {
		for _, a := range known {
			if app == "" && pass(strings.ToLower(a)) {
				app = a
			}
		}
	}
	if app == "" {
		msg := fmt.Sprintf("No parts are kept for an app like %q.", want)
		if len(known) > 0 {
			msg += " Apps with parts: " + strings.Join(known, ", ") + "."
		} else {
			msg += " Parts are kept as tasks run: every step that worked, with the screen it led to."
		}
		return &Result{Status: StatusSettled, Message: msg}
	}
	out := &Result{Status: StatusSettled}
	ps, err := g.flows.Parts(app)
	if err != nil {
		return errResult(ErrInvalidArgument, "The parts of %s cannot be read: %v.", app, err)
	}
	for _, p := range ps.Sorted() {
		out.Parts = append(out.Parts, partInfo(app, p))
	}
	out.Message = fmt.Sprintf("%s of %s. phone_flow_run { parts: [ids] } runs the ones you pick, in order; steps mixes them with other steps.", plural(len(out.Parts), "part"), app)
	return out
}
