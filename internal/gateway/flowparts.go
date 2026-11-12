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

// partsFlow names the replay phone_flow_run makes of parts.
const partsFlow = "parts"

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
// (all of them, or the one on the named device) into the apps' parts.
func (s *Session) keepParts(device string) {
	g := s.g
	type job struct {
		title  string
		steps  []flow.Step
		afters []*screen.Screen
	}
	var jobs []job
	g.mu.Lock()
	for _, l := range s.leases {
		if device != "" {
			if d := g.findDeviceLocked(device); d == nil || d != l.dev {
				continue
			}
		}
		if t := l.task; len(t.trace) > 0 {
			jobs = append(jobs, job{t.title, append([]flow.Step(nil), t.trace...), append([]*screen.Screen(nil), t.traceAfter...)})
		}
	}
	g.mu.Unlock()
	for _, j := range jobs {
		added, err := g.flows.LearnParts(j.steps, j.afters, time.Now())
		if err != nil {
			g.log.Warn("flows: keeping a task's steps as parts", "task", j.title, "err", err)
		} else if added > 0 {
			g.log.Info("flows: new parts", "task", j.title, "parts", added)
		}
	}
}

// offerParts adds the parts of apps to a result, for the apps the agent
// was not shown yet in this task. Without apps it lists the parts of the
// apps the task is held to, or of the apps seen last.
func (g *Gateway) offerParts(res *Result, l *lease, apps ...string) *Result {
	if res == nil || res.IsError() {
		return res
	}
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
		res.Message = strings.TrimSpace(res.Message + fmt.Sprintf(" Parts of %s are listed below: single steps that worked before, each with the screen it leads to. For what no flow covers, pick the parts the task needs and run them in one call, in order, with phone_flow_run { parts: [\"p…\", \"p…\"] }, instead of tapping step by step: each part finds its target, acts and checks the screen it leads to. A part works wherever its target is on the screen (“after” names a part it was recorded after), and the call stops with mismatch, showing the screen, where a part does not fit. Do by hand only what no part covers.",
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
	out.Message = fmt.Sprintf("%s of %s. phone_flow_run { parts: [ids] } runs the ones you pick, in order.", plural(len(out.Parts), "part"), app)
	return out
}
