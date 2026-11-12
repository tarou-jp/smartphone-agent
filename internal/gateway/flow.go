package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/approval"
	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/screen"
)

// This file replays flows (internal/flow, docs/notes/flows.md): steps an
// agent worked out once and that run again without a model. A replay is
// not a way around anything: every step goes through act, the same
// pipeline as phone_tap and its like, with the session's lease, scope,
// rules and approvals, and lands in history like any other step. What a
// replay adds is finding targets by selector, and waiting for a step's
// checkpoint instead of for the screen to settle.

// FlowRequest is phone_flow_run.
type FlowRequest struct {
	Call
	// Name is a saved flow; YAML is a flow given with the call instead.
	// Steps is a plan the agent made for this task: steps in the flow
	// format, as decoded JSON, most of them naming parts
	// ({"part": "p1a2b3c"}); Parts is the short form for a plan of parts
	// only, their ids in order (flowparts.go).
	Name  string
	YAML  string
	Steps []any
	Parts []string
	// Params are the flow's parameter values.
	Params map[string]string
	// FromStep starts the replay at that step, counted from 1: after a
	// mismatch the agent fixed by hand, or with ApprovalID to finish a
	// step that waits for the person.
	FromStep int
}

// FlowSaveRequest is phone_flow_save.
type FlowSaveRequest struct {
	Call
	Name        string
	Description string
	// YAML saves that flow as it is. Without it, the flow is made of the
	// actions the task ran so far.
	YAML string
	// Params turns literals the task typed into parameters: name to the
	// text as it was typed.
	Params map[string]string
}

// FlowListRequest is phone_flow_list.
type FlowListRequest struct {
	Call
	// Name returns that flow's YAML instead of the list.
	Name string
	// App lists the parts kept for that app instead, by id or name.
	App string
}

// FlowReport says how a replay went.
type FlowReport struct {
	Name    string
	Version int
	Total   int // steps the flow has
	Steps   []FlowStep
	Elapsed time.Duration
}

// Flow step outcomes other than a Status or an ErrorCode.
const (
	FlowStepOK      = "ok"
	FlowStepSkipped = "skipped" // an optional step whose target was not there
)

// FlowStep is one step of a replay.
type FlowStep struct {
	Index   int    // counted from 1
	Text    string // "tap button “Orders”"
	Status  string // ok, skipped, mismatch, or the status or error that stopped the replay
	Detail  string // why it stopped, or which fallback selector matched
	Elapsed time.Duration
	// acted is set on a mismatch that came after the step's action ran.
	acted bool
}

// FlowInfo is a row of phone_flow_list.
type FlowInfo struct {
	Name        string
	Description string
	App         string
	Version     int
	Steps       int
	Params      []string // "dish", "note (required)"
}

// optionalWait is how long a replay looks for an optional step's target
// before skipping the step.
const optionalWait = time.Second

// fullReadAfter is how long a replay looks for a target, or waits for a
// checkpoint, with quick reads before it reads the screen the way a step
// by hand does: settled, and with OCR. Right after an input an iPhone
// answers with the tree it pushed, which is fast and can lack what sits
// over the page (a bar of tabs above the tab bar was missing from it for
// ten seconds, while every read a moment later, by hand, had it). A
// replay acts at once, so it is the one that meets that tree.
const fullReadAfter = 1500 * time.Millisecond

// launchWait is how long the step after a launch waits for its target
// unless it says otherwise: an app that starts cold shows a splash screen
// and loads for longer than a screen change takes.
const launchWait = 30 * time.Second

// loadFlow returns the flow a request names or carries, with its
// parameters bound.
func (g *Gateway) loadFlow(req FlowRequest) (*flow.Flow, *Result) {
	name, src := strings.TrimSpace(req.Name), strings.TrimSpace(req.YAML)
	var f *flow.Flow
	var err error
	given := 0
	for _, set := range []bool{name != "", src != "", len(req.Parts) > 0, len(req.Steps) > 0} {
		if set {
			given++
		}
	}
	switch {
	case given != 1:
		return nil, errResult(ErrInvalidArgument, "Pass one of flow, the name of a saved flow; steps, a plan of parts and other steps; parts, the ids of parts to run in order; and yaml, a flow itself.")
	case len(req.Parts) > 0:
		if len(req.Parts) > flow.MaxSteps {
			return nil, errResult(ErrInvalidArgument, "%d parts; one call runs %d at most.", len(req.Parts), flow.MaxSteps)
		}
		f = &flow.Flow{Name: planFlow}
		for _, id := range req.Parts {
			f.Steps = append(f.Steps, flow.Step{Part: strings.TrimSpace(id)})
		}
		if err = f.Validate(); err != nil {
			return nil, errResult(ErrInvalidArgument, "%v.", err)
		}
	case len(req.Steps) > 0:
		// The flow format is YAML, of which JSON is a part: the plan is
		// read as a flow's steps, with the same checks.
		data, merr := json.Marshal(map[string]any{"flow": planFlow, "steps": req.Steps})
		if merr != nil {
			return nil, errResult(ErrInvalidArgument, "steps cannot be read: %v.", merr)
		}
		if f, err = flow.Parse(data); err != nil {
			return nil, errResult(ErrInvalidArgument, "%v. A step is an object such as {\"part\": \"p1a2b3c\"}, {\"tap\": \"Label\"} or {\"assert\": {\"visible\": \"Text\"}}.", capitalizeErr(err))
		}
	case src != "":
		if f, err = flow.Parse([]byte(src)); err != nil {
			return nil, errResult(ErrInvalidArgument, "%v.", err)
		}
	default:
		f, err = g.flows.Load(name)
		if errors.Is(err, flow.ErrNotFound) {
			return nil, errResult(ErrNotFound, "No flow is named %q. phone_flow_list lists the saved flows.", name)
		}
		if err != nil {
			return nil, errResult(ErrInvalidArgument, "The flow %q cannot be read: %v.", name, err)
		}
	}
	// A step that names a part becomes the part as it is kept now.
	if f, err = f.Resolve(g.partStep); err != nil {
		return nil, errResult(ErrInvalidArgument, "%v. phone_flow_list with app lists an app's parts.", capitalizeErr(err))
	}
	bound, err := f.Bind(req.Params)
	if err != nil {
		return nil, errResult(ErrInvalidArgument, "%v.", capitalizeErr(err))
	}
	if req.FromStep < 0 || req.FromStep > len(bound.Steps) {
		return nil, errResult(ErrInvalidArgument, "from_step must be between 1 and %d, the flow's last step.", len(bound.Steps))
	}
	return bound, nil
}

func capitalizeErr(err error) string { return capitalize(err.Error()) }

// RunFlow implements phone_flow_run: the flow's steps run in order, and
// the replay stops at the first step that does not go as the flow says.
func (s *Session) RunFlow(ctx context.Context, req FlowRequest) *Result {
	g := s.g
	f, res := g.loadFlow(req)
	if res != nil {
		return res
	}
	l, res := s.acquire(req.Device)
	if res != nil {
		return res
	}
	defer s.done(l)
	ctx, cancel := mergeCtx(ctx, l.ctx)
	defer cancel()

	first := 0
	if req.FromStep > 0 {
		first = req.FromStep - 1
	}
	start := time.Now()
	report := &FlowReport{Name: f.Name, Version: f.Version, Total: len(f.Steps)}
	var last *Result         // the result of the newest step that ran
	var shown *screen.Screen // the newest screen a step returned
	var events []screen.Event
	var lastAction time.Time
	stopped := -1
	// A replay the agent put together is its own work, and the task's
	// steps are kept like steps made by hand; a saved flow is somebody's
	// flow already. (A flow run from a file is a test run, which keeps
	// nothing.)
	own := strings.TrimSpace(req.Name) == ""
	plan := len(req.Parts)+len(req.Steps) > 0
	g.mu.Lock()
	test := l.task.test
	g.mu.Unlock()
	// want is the app the next step runs in, as far as the flow says.
	want := f.App
	for _, st := range f.Steps[:min(first, len(f.Steps))] {
		want = appAfter(st, want)
	}
	for i := first; i < len(f.Steps); i++ {
		st := f.Steps[i]
		c := Call{Device: req.Device, Intent: st.Intent, Progress: req.Progress, reuse: true, flow: f.Name, own: own, part: st.PartID}
		if c.Intent == "" {
			c.Intent = req.Intent
		}
		if c.Intent == "" {
			c.Intent = fmt.Sprintf("Flow “%s”: %s", f.Name, st.Describe())
		}
		if i == first {
			c.ApprovalID = req.ApprovalID
		}
		began := time.Now()
		at := flowPlace{app: want, first: i == first}
		if i+1 < len(f.Steps) {
			at.next = &f.Steps[i+1]
		}
		r, fs := g.flowStep(ctx, l, c, f, st, at, &lastAction)
		want = appAfter(st, want)
		if r != nil {
			// The person may have pressed Stop meanwhile.
			if r = g.finish(l, r); r.Status == StatusStopped {
				fs.Status, fs.Detail = string(StatusStopped), r.Message
			}
			last = r
			if r.Screen != nil {
				events = append(events, r.Screen.Events...)
				shown = r.Screen
			}
		}
		fs.Index, fs.Text, fs.Elapsed = i+1, st.Describe(), time.Since(began)
		report.Steps = append(report.Steps, fs)
		// In a test run a part that leads elsewhere is the finding, not
		// something to learn from.
		if fs.acted && st.PartID != "" && !test && g.flows.BreakPart(st.PartID) {
			g.log.Info("flows: a part led elsewhere twice and is no longer offered", "part", st.PartID)
		}
		if fs.Status != FlowStepOK && fs.Status != FlowStepSkipped {
			stopped = i
			break
		}
	}
	report.Elapsed = time.Since(start)
	if strings.TrimSpace(req.Name) != "" {
		g.mu.Lock()
		if l.task.replayed == nil {
			l.task.replayed = map[string]bool{}
		}
		l.task.replayed[f.Name] = l.task.replayed[f.Name] || stopped >= 0
		g.mu.Unlock()
	}

	out := &Result{Status: StatusSettled, Flow: report}
	title := fmt.Sprintf("the flow “%s”", f.Name)
	if f.Version > 0 {
		title += fmt.Sprintf(" (version %d)", f.Version)
	}
	if plan {
		title = "the plan"
	}
	text := "Ran " + title
	if stopped >= 0 {
		fs := report.Steps[len(report.Steps)-1]
		out.Status, out.Error = last.Status, last.Error
		out.ApprovalID, out.Rule, out.Note, out.Setup = last.ApprovalID, last.Rule, last.Note, last.Setup
		out.Message = fmt.Sprintf("%s stopped at step %d of %d (%s): %s", capitalize(title), stopped+1, len(f.Steps), fs.Text, flowAdvice(out, fs, stopped+1, len(f.Steps)))
		text = fmt.Sprintf("%s stopped at step %d of %d: %s", capitalize(title), stopped+1, len(f.Steps), fs.Text)
	} else {
		out.Message = fmt.Sprintf("%s ran: %s in %s. Every step led where it says below (reached), so the phone is where the last step leads: go on from the screen below, without doing these steps again.", capitalize(title), plural(len(report.Steps), "step"), approval.Human(report.Elapsed))
	}
	if shown != nil && (out.Status == StatusSettled || out.Status == StatusMismatch) && !out.IsError() {
		// A step with a checkpoint ends on the first read the checkpoint
		// holds on, which may be of a screen that is still coming up: the
		// rest of it is not there yet, and nothing was read by OCR. The
		// agent plans its next steps from the screen the replay returns,
		// so that one is read as after any step of its own.
		g.settle(ctx, l.dev)
		if cur, err := g.observe(ctx, l.dev, true); err == nil && cur != nil {
			cur = g.finishRead(ctx, l, cur)
			g.mu.Lock()
			l.after, l.afterAt = cur, time.Now()
			g.mu.Unlock()
			if r := g.screenResult(l, cur, true); r.Screen != nil {
				shown = r.Screen
				events = append(events, r.Screen.Events...)
			}
		}
	}
	if shown != nil {
		// The agent last saw the screen before the replay: send it whole,
		// with every toast and dialog the steps met on the way.
		view := *shown
		view.Events = events
		out.Screen, out.Full = &view, true
	}
	status := string(out.Status)
	if out.Error != "" {
		status = string(out.Error)
	}
	args := map[string]any{"flow": f.Name}
	if len(req.Parts) > 0 {
		args = map[string]any{"parts": req.Parts}
	} else if len(req.Steps) > 0 {
		args = map[string]any{"steps": req.Steps}
	}
	if f.Version > 0 {
		args["version"] = f.Version
	}
	if req.FromStep > 0 {
		args["from_step"] = req.FromStep
	}
	if len(req.Params) > 0 {
		args["params"] = req.Params
	}
	g.addStep(l, StepView{Tool: "phone_flow_run", Args: argsJSON(args), Text: text, Status: status, At: start, Elapsed: report.Elapsed})
	return g.finish(l, out)
}

// flowAdvice ends a stopped replay's message: why, and what the agent
// does next.
func flowAdvice(out *Result, fs FlowStep, step, total int) string {
	detail := strings.TrimSuffix(fs.Detail, ".")
	next := ""
	if step < total {
		next = fmt.Sprintf(" or %d to go on after it", step+1)
	}
	switch {
	case out.Status == StatusMismatch && fs.acted:
		// The action ran and led somewhere else than the flow says, which
		// is often the same place looking different (an app that opens
		// without its splash screen). Repeating the step would act twice.
		then := "the replay is done"
		if step < total {
			then = fmt.Sprintf("call phone_flow_run again with from_step %d to go on with the steps after it", step+1)
		}
		return fmt.Sprintf("%s. Earlier steps ran, and so did this step's action: do not repeat it. This is the screen now: when it is where the task should be after this step, %s; otherwise do by hand what is missing first.",
			detail, then)
	case out.Status == StatusMismatch:
		return fmt.Sprintf("%s. Earlier steps ran. This is the screen now: do the step by hand, then call phone_flow_run again with from_step %d to repeat it%s. If the app changed for good, save the corrected flow with phone_flow_save.",
			detail, step, next)
	case out.Status == StatusPending:
		return fmt.Sprintf("it waits for the person. Call phone_flow_run again with from_step %d and approval_id %q.", step, out.ApprovalID)
	case out.Status == StatusUncertain:
		return fmt.Sprintf("%s. The step may have happened: observe before anything else, then go on by hand or with from_step.", detail)
	case detail != "":
		return detail + "."
	}
	return "see the status."
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// stepTimeout is how long a step waits for its target and for its
// checkpoint: what the step says, else what the flow says, else ten
// seconds, and launchWait for the step right after a launch.
func stepTimeout(f *flow.Flow, st flow.Step, afterLaunch bool) time.Duration {
	switch {
	case st.TimeoutMS > 0:
		return time.Duration(st.TimeoutMS) * time.Millisecond
	case f.TimeoutMS > 0:
		return time.Duration(f.TimeoutMS) * time.Millisecond
	case afterLaunch:
		return launchWait
	}
	return time.Duration(flow.DefaultTimeoutMS) * time.Millisecond
}

// flowPlace is where a step of a replay runs: the app the flow says is in
// front ("" when it does not say), whether the step is the first that
// runs, and the step after it (nil for the last).
type flowPlace struct {
	app   string
	first bool
	next  *flow.Step
}

// checkpoint is what a replay checks after st. A step's checkpoint says
// what its screen shows, and waiting for that is needed only where nothing
// else would notice a wrong screen: after the last step, and before a step
// that acts without looking (a place on the screen, a key). Before a step
// that looks for its target, finding the target is the check and the wait:
// the screen shown is checked again only for its app, which every read
// names. A checkpoint that names what changes from one day to the next (a
// trending headline) then stops nothing it does not have to.
func checkpoint(st flow.Step, next *flow.Step) *flow.Condition {
	if st.Expect == nil || next == nil || !looksFirst(*next) {
		return st.Expect
	}
	if st.Expect.App == "" {
		return nil
	}
	return &flow.Condition{App: st.Expect.App}
}

// looksFirst reports whether st finds what it acts on on the screen
// before it acts, and so stops at a screen that is not the one it expects:
// a target by id or label (with no place on the screen to fall back to),
// an assert, or a launch, which acts on no screen.
func looksFirst(st flow.Step) bool {
	switch st.Kind() {
	case "assert", "launch":
		return true
	}
	sel := st.Target()
	return sel != nil && !st.Optional && !hasPoint(sel)
}

// hasPoint reports whether sel or a fallback of it is a place on the
// screen, which matches whatever is shown.
func hasPoint(sel *flow.Selector) bool {
	if sel.Point != nil {
		return true
	}
	for i := range sel.Or {
		if hasPoint(&sel.Or[i]) {
			return true
		}
	}
	return false
}

// appAfter is the app in front after st, as far as the flow says: the app
// it launches or its checkpoint names, none after a key (home leaves the
// app), else the app before it.
func appAfter(st flow.Step, before string) string {
	switch {
	case st.Launch != "":
		return st.Launch
	case st.Expect != nil && st.Expect.App != "":
		return st.Expect.App
	case st.Kind() == "key":
		return ""
	}
	return before
}

// inApp reports whether app is in front on cur; why says what is instead.
func inApp(cur *screen.Screen, app string) (ok bool, why string) {
	if app == "" {
		return true, ""
	}
	return (&flow.Condition{App: app}).Check(cur)
}

// flowStep runs one step of a replay. It returns the step's result, nil
// for a step that was skipped, and how the step went.
func (g *Gateway) flowStep(ctx context.Context, l *lease, c Call, f *flow.Flow, st flow.Step, at flowPlace, lastAction *time.Time) (*Result, FlowStep) {
	g.mu.Lock()
	n := len(l.task.trace)
	afterLaunch := n > 0 && l.task.trace[n-1].Launch != ""
	g.mu.Unlock()
	timeout := stepTimeout(f, st, afterLaunch)
	stop := func(r *Result, detail string) (*Result, FlowStep) {
		status := string(r.Status)
		if r.Error != "" {
			status = string(r.Error)
		}
		if detail == "" {
			detail = r.Message
		}
		return r, FlowStep{Status: status, Detail: detail}
	}

	// A flow kept from a task that began in its app has no launch: open
	// the app when another one is in front, or the first taps land on
	// whatever that is (the home screen's icons, say).
	if at.first && at.app != "" && st.Kind() != "launch" {
		cur, err := g.startScreen(ctx, l, c)
		if err != nil {
			return stop(g.readError(l, err), "")
		}
		if ok, _ := inApp(cur, at.app); !ok {
			lc := c
			lc.Intent = fmt.Sprintf("Open %s, where the flow begins", at.app)
			lc.part = ""
			res := g.act(ctx, l, lc, Action{Tool: "launch", App: at.app})
			if res.IsError() || res.Status != StatusSettled {
				return stop(res, "")
			}
			*lastAction = time.Now()
			cur, why, r := g.flowAwait(ctx, l, &flow.Condition{App: at.app}, launchWait)
			if r != nil {
				return stop(r, "")
			}
			if why != "" {
				return g.flowMismatch(l, cur, st, why+" after opening it")
			}
			timeout = stepTimeout(f, st, true)
		}
	}

	if st.Kind() == "assert" {
		began := time.Now()
		cur, why, res := g.flowAwait(ctx, l, st.Assert, timeout)
		if res != nil {
			return stop(res, "")
		}
		text := "Checked: " + st.Assert.String()
		if why != "" {
			res = g.screenResult(l, cur, true)
			res.Status = StatusMismatch
			g.addStep(l, StepView{Tool: "phone_flow_run", Text: "Not as the flow expects: " + why, Status: string(StatusMismatch), At: began, Elapsed: time.Since(began)})
			return stop(res, fmt.Sprintf("%s after %s", why, approval.Human(timeout)))
		}
		res = g.screenResult(l, cur, false)
		res.Status = StatusSettled
		g.addStep(l, StepView{Tool: "phone_flow_run", Text: text, Status: string(StatusSettled), At: began, Elapsed: time.Since(began)})
		return res, FlowStep{Status: FlowStepOK}
	}

	a := Action{Tool: st.Kind(), LongPress: st.LongPress, Append: st.Append, Submit: st.Submit,
		Direction: st.Swipe, DurationMS: st.DurationMS, Key: st.Key, App: st.Launch}
	if st.Type != nil {
		a.Text = *st.Type
	}
	detail := ""
	if sel := st.Target(); sel != nil {
		wait := timeout
		if st.Optional {
			wait = optionalWait
		}
		m, cur, err, res := g.flowFind(ctx, l, sel, wait, c.ApprovalID)
		if res != nil {
			return stop(res, "")
		}
		if err != nil {
			if st.Optional {
				return nil, FlowStep{Status: FlowStepSkipped, Detail: "its target is not on the screen"}
			}
			return g.flowMismatch(l, cur, st, fmt.Sprintf("%v after %s", err, approval.Human(wait)))
		}
		if m.Point != nil {
			// A place on the screen means something only in its app.
			if ok, why := inApp(cur, at.app); !ok {
				return g.flowMismatch(l, cur, st, why)
			}
		}
		switch {
		case m.Point != nil:
			a.Point = m.Point
		case m.Element.Ref > 0:
			a.Ref = m.Element.Ref
		case st.Kind() == "tap" && !m.Element.Bounds.Empty():
			// Text only OCR reads has no ref: tap where it is.
			p := m.Element.Bounds.Center()
			a.Point = &p
		default:
			return g.flowMismatch(l, cur, st, fmt.Sprintf("%s is text read from the pixels, which cannot be typed into or swiped in", sel.String()))
		}
		if m.By > 0 {
			detail = fmt.Sprintf("found by fallback selector %d (%s)", m.By, sel.Or[m.By-1].String())
		}
		// The action starts from the read that found its target.
		g.mu.Lock()
		l.after, l.afterAt = cur, time.Now()
		g.mu.Unlock()
	}
	if st.From != nil {
		cur, err := g.startScreen(ctx, l, c)
		if err != nil {
			return stop(g.readError(l, err), "")
		}
		if ok, why := inApp(cur, at.app); !ok {
			return g.flowMismatch(l, cur, st, why)
		}
		a.From = &screen.Point{X: int(st.From.X * float64(cur.Width)), Y: int(st.From.Y * float64(cur.Height))}
		a.To = &screen.Point{X: int(st.To.X * float64(cur.Width)), Y: int(st.To.Y * float64(cur.Height))}
	}
	if res := a.check(); res != nil {
		return stop(res, "")
	}
	expect := checkpoint(st, at.next)
	if expect != nil {
		cond := expect
		c.until = func(view *screen.Screen) bool { ok, _ := cond.Check(view); return ok }
		c.untilMax, c.untilOCR = timeout, cond.NeedsOCR()
	}
	// Keep a floor between actions, as a quick person would.
	if gap := g.opt.FlowPace - time.Since(*lastAction); gap > 0 && !lastAction.IsZero() {
		sleepCtx(ctx, gap, nil)
	}
	*lastAction = time.Now()
	res := g.act(ctx, l, c, a)
	if res.IsError() || res.Status != StatusSettled {
		return stop(res, "")
	}
	if expect != nil {
		if ok, why := expect.Check(res.Screen); !ok {
			res.Status, res.Full, res.Diff = StatusMismatch, true, nil
			g.addStep(l, StepView{Tool: "phone_flow_run", Text: "Not as the flow expects: " + why, Status: string(StatusMismatch), At: time.Now()})
			r, fs := stop(res, fmt.Sprintf("the step ran, but %s after %s", why, approval.Human(timeout)))
			fs.acted = true
			return r, fs
		}
	}
	if expect != nil {
		// Say what the step was checked against: an agent that reads only
		// "ok" does not know that the screen it asked for is the one it
		// got, and goes looking for it.
		c := *expect
		if len(c.Visible)+len(c.Gone) > 0 {
			c.App = ""
		}
		reached := "reached: " + c.String()
		if detail != "" {
			reached = detail + "; " + reached
		}
		detail = reached
	}
	return res, FlowStep{Status: FlowStepOK, Detail: detail}
}

// flowMismatch is the result of a step whose target the screen does not
// have: nothing was done, and the agent gets the screen as it is.
func (g *Gateway) flowMismatch(l *lease, cur *screen.Screen, st flow.Step, why string) (*Result, FlowStep) {
	res := &Result{Status: StatusMismatch}
	if cur != nil {
		res = g.screenResult(l, cur, true)
		res.Status = StatusMismatch
	}
	g.addStep(l, StepView{Tool: "phone_flow_run", Text: "Not as the flow expects: " + capitalize(st.Describe()) + " — " + why,
		Status: string(StatusMismatch), At: time.Now()})
	return res, FlowStep{Status: string(StatusMismatch), Detail: why + "; nothing was done"}
}

// flowFind waits until the selector finds its element, as a person looks
// for a button before pressing it: the screen may still be loading. It
// returns the match and the read it was found on; err says why nothing
// matched within max, with the last read; res refuses the step (the
// person took over, the phone is locked or gone, …).
func (g *Gateway) flowFind(ctx context.Context, l *lease, sel *flow.Selector, max time.Duration, approvalID string) (flow.Match, *screen.Screen, error, *Result) {
	g.readyScreen(ctx, l)
	began := time.Now()
	deadline := began.Add(max)
	ocr := sel.NeedsOCR()
	full := false
	for first := true; ; first = false {
		if !full && !first && time.Since(began) > fullReadAfter {
			// Not found in quick reads: look as a step by hand would.
			full, ocr = true, true
			g.settle(ctx, l.dev)
		}
		if res := g.gate(l, approvalID, true); res != nil {
			return flow.Match{}, nil, nil, res
		}
		var cur *screen.Screen
		if first && !ocr {
			// The read that ended the step before is fresh enough.
			g.mu.Lock()
			if l.after != nil && time.Since(l.afterAt) < reuseAge {
				cur = l.after
			}
			g.mu.Unlock()
		}
		if cur == nil {
			var err error
			if cur, err = g.observe(ctx, l.dev, ocr); err != nil {
				return flow.Match{}, nil, nil, g.readError(l, err)
			}
		}
		if cur.Locked {
			return flow.Match{}, nil, nil, g.attachScreen(l, statusResult(StatusLocked,
				"The phone is locked. Nothing was done. Call phone_handoff so the person can unlock it."), cur)
		}
		view, _ := g.agentView(l, cur)
		m, err := sel.Find(view)
		if err == nil {
			return m, cur, nil, nil
		}
		if !time.Now().Before(deadline) {
			return flow.Match{}, cur, err, nil
		}
		g.mu.Lock()
		wake := l.dev.wake
		g.mu.Unlock()
		if !sleepCtx(ctx, g.opt.PollInterval, wake) && ctx.Err() != nil {
			return flow.Match{}, nil, nil, errResult(ErrInternal, "The call was cancelled.")
		}
	}
}

// flowAwait waits until the condition holds. why is empty when it does;
// otherwise it says what does not hold on cur, the last read.
func (g *Gateway) flowAwait(ctx context.Context, l *lease, cond *flow.Condition, max time.Duration) (cur *screen.Screen, why string, res *Result) {
	g.readyScreen(ctx, l)
	began := time.Now()
	deadline := began.Add(max)
	ocr, full := cond.NeedsOCR(), false
	for {
		if res := g.gate(l, "", false); res != nil {
			return nil, "", res
		}
		if !full && time.Since(began) > fullReadAfter {
			full, ocr = true, true
			g.settle(ctx, l.dev)
		}
		var err error
		if cur, err = g.observe(ctx, l.dev, ocr); err != nil {
			return nil, "", g.readError(l, err)
		}
		view, _ := g.agentView(l, cur)
		ok, why := cond.Check(view)
		if ok {
			g.mu.Lock()
			l.after, l.afterAt = cur, time.Now()
			g.mu.Unlock()
			return cur, "", nil
		}
		if !time.Now().Before(deadline) {
			return cur, why, nil
		}
		g.mu.Lock()
		wake := l.dev.wake
		g.mu.Unlock()
		if !sleepCtx(ctx, g.opt.PollInterval, wake) && ctx.Err() != nil {
			return nil, "", errResult(ErrInternal, "The call was cancelled.")
		}
	}
}

// awaitScreen waits, after an action, for a screen the step's checkpoint
// holds on (Call.until). It returns the last read and whether it holds.
func (g *Gateway) awaitScreen(ctx context.Context, l *lease, c Call) (*screen.Screen, bool, error) {
	d := l.dev
	began := time.Now()
	deadline := began.Add(c.untilMax)
	ocr, full := c.untilOCR, false
	for {
		if !full && time.Since(began) > fullReadAfter {
			// The checkpoint does not show in quick reads: read as a step
			// by hand would before saying it is not there.
			full, ocr = true, true
			g.settle(ctx, d)
		}
		cur, err := g.observe(ctx, d, ocr)
		if err != nil {
			return nil, false, err
		}
		view, _ := g.agentView(l, cur)
		if c.until(view) {
			return cur, true, nil
		}
		if !time.Now().Before(deadline) || ctx.Err() != nil {
			return cur, false, nil
		}
		g.mu.Lock()
		wake := d.wake
		g.mu.Unlock()
		sleepCtx(ctx, g.opt.PollInterval, wake)
	}
}

// traceStep keeps an action that ran as a flow step: its target as a
// selector written on the screen it was found on, and the screen it led
// to as the step's checkpoint. Steps around an app the rules block are
// left out even when the person allowed the app for the task: Pinky keeps
// no record of such an app's screen.
func (g *Gateway) traceStep(l *lease, c Call, a Action, p *prep, before, after *screen.Screen) {
	_, hidden := g.policy.Blocked(after.App)
	if before != nil && !hidden {
		_, hidden = g.policy.Blocked(before.App)
	}
	st, ok := flowStepOf(c, a, p, before, after)
	if c.flow != "" && strings.HasPrefix(st.Intent, "Flow “") {
		st.Intent = "" // the replay's own words for a step without an intent
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	t := l.task
	if hidden || !ok || len(t.trace) >= flow.MaxSteps {
		t.traceGap = true
		return
	}
	saved := c.flow != "" && !c.own
	if n := len(t.trace); n > 0 && st.Launch != "" && t.trace[n-1].Launch == st.Launch {
		// Opening the app that was just opened changes nothing: a replay
		// that stopped at its launch and was started again is one step.
		t.trace[n-1], t.traceAfter[n-1], t.tracePart[n-1], t.traceSaved[n-1] = st, after, c.part, saved
		t.traceShot[n-1] = nil
	} else {
		t.trace, t.traceAfter = append(t.trace, st), append(t.traceAfter, after)
		t.tracePart, t.traceSaved = append(t.tracePart, c.part), append(t.traceSaved, saved)
		t.traceShot = append(t.traceShot, nil)
	}
	if c.flow == "" || c.own {
		t.byHand++
	}
}

// flowStepOf writes an action as a flow step.
func flowStepOf(c Call, a Action, p *prep, before, after *screen.Screen) (flow.Step, bool) {
	st := flow.Step{Intent: c.Intent}
	target := func() *flow.Selector {
		if a.Ref == 0 || p == nil || p.target == nil || before == nil {
			return nil
		}
		sel := flow.SelectorFor(before, *p.target)
		return &sel
	}
	switch a.Tool {
	case "tap":
		st.LongPress = a.LongPress
		if st.Tap = target(); st.Tap == nil {
			if a.Point == nil || before == nil {
				return st, false
			}
			// A tap by x and y names a place, not the element under it.
			st.Tap = &flow.Selector{Point: flow.PointAt(before, *a.Point)}
		}
	case "type":
		text := a.Text
		st.Type, st.Into, st.Append, st.Submit = &text, target(), a.Append, a.Submit
		if a.Ref > 0 && st.Into == nil || st.Into != nil && st.Into.Point != nil {
			return st, false
		}
	case "swipe":
		st.DurationMS = a.DurationMS
		if a.From != nil {
			if before == nil {
				return st, false
			}
			st.From, st.To = flow.PointAt(before, *a.From), flow.PointAt(before, *a.To)
		} else {
			st.Swipe, st.In = a.Direction, target()
			if a.Ref > 0 && st.In == nil || st.In != nil && st.In.Point != nil {
				return st, false
			}
		}
	case "key":
		st.Key = a.Key
	case "launch":
		st.Launch = a.App
		if p != nil && !p.url && p.launch.ID != "" {
			st.Launch = p.launch.ID
		}
	default:
		return st, false
	}
	switch a.Tool {
	case "type":
		// What typing leads to depends on the text, which a flow may take
		// as a parameter: such steps wait for the screen to settle instead.
	case "launch":
		// An app opens where it was left, so which screen shows is not
		// the launch's doing: the checkpoint is the app in front, and the
		// next step waits for its own target.
		if st.Expect = flow.Checkpoint(before, after); st.Expect != nil {
			st.Expect.Visible, st.Expect.Gone = nil, nil
		} else if app := appKey(after); app != "" {
			st.Expect = &flow.Condition{App: app}
		}
		if st.Expect != nil && st.Expect.App == "" {
			st.Expect = nil
		}
	default:
		st.Expect = flow.Checkpoint(before, after)
	}
	return st, true
}

// SaveFlow implements phone_flow_save: it keeps the actions of the task
// so far, or a flow the agent wrote, under a name.
func (s *Session) SaveFlow(ctx context.Context, req FlowSaveRequest) *Result {
	g := s.g
	name, src := strings.TrimSpace(req.Name), strings.TrimSpace(req.YAML)
	l, res := s.acquire(req.Device)
	if res != nil {
		return res
	}
	defer s.done(l)
	if res := g.gate(l, "", false); res != nil {
		return g.finish(l, res)
	}
	var f *flow.Flow
	note := ""
	if src != "" {
		var err error
		if f, err = flow.Parse([]byte(src)); err != nil {
			return g.finish(l, errResult(ErrInvalidArgument, "%v.", err))
		}
		if name != "" && name != f.Name {
			return g.finish(l, errResult(ErrInvalidArgument, "name is %q but the yaml names the flow %q.", name, f.Name))
		}
		if len(req.Params) > 0 {
			return g.finish(l, errResult(ErrInvalidArgument, "params goes with a flow made of the task's steps; a flow given as yaml lists its own."))
		}
		if d := strings.TrimSpace(req.Description); d != "" {
			f.Description = d
		}
	} else {
		if !flow.ValidName(name) {
			return g.finish(l, errResult(ErrInvalidArgument, "name must be lower-case letters, digits, dots, dashes or underscores, such as settings.open-display."))
		}
		g.mu.Lock()
		steps := append([]flow.Step(nil), l.task.trace...)
		gap := l.task.traceGap
		g.mu.Unlock()
		if len(steps) == 0 {
			return g.finish(l, errResult(ErrInvalidArgument, "This task has no steps to save yet: run the steps first, then save them."))
		}
		var err error
		if f, err = flow.Compile(name, req.Description, flowApp(steps), steps, req.Params); err != nil {
			return g.finish(l, errResult(ErrInvalidArgument, "%v.", err))
		}
		if gap {
			note = " Some of the task's steps are not in it: steps in an app Pinky keeps no record of, or past a flow's length."
		}
	}
	saved, err := g.flows.Save(f)
	if err != nil {
		return g.finish(l, errResult(ErrInternal, "The flow could not be saved: %v.", err))
	}
	data, _ := saved.Marshal()
	g.addStep(l, StepView{Tool: "phone_flow_save", Args: argsJSON(map[string]any{"name": saved.Name, "version": saved.Version}),
		Text:   fmt.Sprintf("Saved the flow “%s” (version %d, %s)", saved.Name, saved.Version, plural(len(saved.Steps), "step")),
		Status: string(StatusSettled), At: time.Now()})
	return g.finish(l, &Result{Status: StatusSettled, FlowYAML: string(data),
		Message: fmt.Sprintf("Saved the flow “%s”, version %d, %s. Read it below: drop detours, name what should be a parameter, and save it again as yaml if it needs changes. phone_flow_run replays it.%s",
			saved.Name, saved.Version, plural(len(saved.Steps), "step"), note)})
}

// flowApp is the app most of the steps' checkpoints are in.
func flowApp(steps []flow.Step) string {
	count := map[string]int{}
	for _, st := range steps {
		if st.Expect != nil && st.Expect.App != "" {
			count[st.Expect.App]++
		}
	}
	apps := make([]string, 0, len(count))
	for a := range count {
		apps = append(apps, a)
	}
	sort.Slice(apps, func(i, j int) bool {
		if count[apps[i]] != count[apps[j]] {
			return count[apps[i]] > count[apps[j]]
		}
		return apps[i] < apps[j]
	})
	if len(apps) == 0 {
		return ""
	}
	return apps[0]
}

// ListFlows implements phone_flow_list.
func (s *Session) ListFlows(ctx context.Context, req FlowListRequest) *Result {
	g := s.g
	l, res := s.acquire(req.Device)
	if res != nil {
		return res
	}
	defer s.done(l)
	if strings.TrimSpace(req.App) != "" {
		return g.finish(l, g.listParts(req.App))
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		f, err := g.flows.Load(name)
		if errors.Is(err, flow.ErrNotFound) {
			return g.finish(l, errResult(ErrNotFound, "No flow is named %q.", name))
		}
		if err != nil {
			return g.finish(l, errResult(ErrInvalidArgument, "The flow %q cannot be read: %v.", name, err))
		}
		data, _ := f.Marshal()
		return g.finish(l, &Result{Status: StatusSettled, FlowYAML: string(data)})
	}
	flows, broken := g.flows.List()
	out := &Result{Status: StatusSettled}
	for _, f := range flows {
		out.Flows = append(out.Flows, flowInfo(f))
	}
	switch {
	case len(flows) == 0:
		out.Message = "No flows are saved yet. After a task's steps worked, phone_flow_save keeps them for next time."
	default:
		out.Message = plural(len(flows), "flow") + ". phone_flow_run replays one; phone_flow_list with name shows its steps."
	}
	if len(broken) > 0 {
		out.Message += fmt.Sprintf(" Not readable: %s.", strings.Join(broken, ", "))
	}
	return g.finish(l, out)
}

func flowInfo(f *flow.Flow) FlowInfo {
	info := FlowInfo{Name: f.Name, Description: f.Description, App: f.App, Version: f.Version, Steps: len(f.Steps)}
	for name, p := range f.Params {
		if p.Default == nil {
			name += " (required)"
		} else {
			name += fmt.Sprintf(" (default %q)", clip(*p.Default, 40))
		}
		info.Params = append(info.Params, name)
	}
	sort.Strings(info.Params)
	return info
}

// Flows lists the saved flows, for the client and the command line.
func (g *Gateway) Flows() []FlowInfo {
	flows, _ := g.flows.List()
	out := make([]FlowInfo, 0, len(flows))
	for _, f := range flows {
		out = append(out, flowInfo(f))
	}
	return out
}
