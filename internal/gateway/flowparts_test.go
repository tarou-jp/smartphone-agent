package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/flow"
)

const dinely = "com.dinely.app"

// partID finds, among the parts a result lists, the one whose text holds
// all of the given words.
func partID(t *testing.T, parts []PartInfo, words ...string) string {
	t.Helper()
	for _, p := range parts {
		ok := true
		for _, w := range words {
			ok = ok && strings.Contains(p.Text, w)
		}
		if ok {
			return p.ID
		}
	}
	t.Fatalf("no part with %q among %+v", words, parts)
	return ""
}

// One task by hand teaches Pinky the app's parts; another task, which no
// flow replays, is put together from them and runs in one call.
func TestPartsPutTogether(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, first := connect(t, g, "Reorder last week's lunch")
	if len(first.Parts) != 0 || strings.Contains(first.Message, "Parts") {
		t.Fatalf("nothing to offer yet: %s", first.Message)
	}
	toCart(t, s)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	ps, err := g.flows.Parts(dinely)
	if err != nil || len(ps.Parts) != 4 {
		t.Fatalf("parts after the first task: %+v %v", ps, err)
	}
	if open := ps.Parts[0]; open.Launch != dinely || open.Expect == nil || open.Expect.App != dinely || len(open.Expect.Visible) != 0 {
		t.Fatalf("opening an app leads to the app, wherever it was left: %+v", open.Expect)
	}

	// Another task: no flow is it, so the app's parts are offered.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	res := s.Connect(ctx, ConnectRequest{Task: "Look at my orders", Brief: "Open Dinely and show me my orders"})
	mustStatus(t, res, StatusSettled)
	if len(res.Flows) != 1 || len(res.Parts) != 4 || !strings.Contains(res.Message, "Parts of "+dinely) {
		t.Fatalf("connect: %d flows, %d parts: %s", len(res.Flows), len(res.Parts), res.Message)
	}
	open, orders := partID(t, res.Parts, "open "+dinely), partID(t, res.Parts, "tap", "Orders")
	for _, p := range res.Parts {
		if p.ID == orders && (len(p.After) != 1 || p.After[0] != open) {
			t.Errorf("Orders was recorded after opening the app: %+v", p)
		}
	}
	inputs := len(dev.Inputs())
	res = runFlow(s, FlowRequest{Parts: []string{open, orders}})
	mustStatus(t, res, StatusSettled)
	if got := flowStatuses(res); got != "ok ok" || !strings.Contains(res.Message, "The plan ran") {
		t.Fatalf("steps: %s\n%s", got, res.Message)
	}
	if dev.Current() != "orders" || len(dev.Inputs())-inputs != 2 {
		t.Fatalf("on %q after %d inputs", dev.Current(), len(dev.Inputs())-inputs)
	}
	if !res.Full || res.Screen == nil || !strings.Contains(res.Screen.Render(), "Reorder") {
		t.Fatalf("the run returns the screen it ended on: %+v", res)
	}
	// The task the agent put together is kept as a flow of its own: a
	// list of the parts it ran, by name. Its steps were seen once more.
	res = s.Release(ctx, ReleaseRequest{})
	if flows := g.Flows(); len(flows) != 2 || !strings.Contains(res.Message, "kept the task's steps") {
		t.Fatalf("flows: %+v\n%s", flows, res.Message)
	}
	for _, fi := range g.Flows() {
		f, err := g.flows.Load(fi.Name)
		if err != nil {
			t.Fatal(err)
		}
		for i, st := range f.Steps {
			if st.Part == "" || st.Tap != nil || st.Launch != "" || st.Name == "" {
				t.Errorf("%s step %d is a copy, not a part by name: %+v", f.Name, i+1, st)
			}
		}
		if f.Task == taskKey("Open Dinely and show me my orders") && (len(f.Steps) != 2 || f.Steps[0].Part != open || f.Steps[1].Part != orders) {
			t.Errorf("the kept plan: %+v", f.Steps)
		}
	}
	ps, _ = g.flows.Parts(dinely)
	if len(ps.Parts) != 4 {
		t.Fatalf("running parts made new ones: %d", len(ps.Parts))
	}
	for _, p := range ps.Parts {
		if want := map[bool]int{true: 2, false: 1}[p.ID == open || p.ID == orders]; p.Seen != want {
			t.Errorf("%s seen %d times, want %d", p.Text(), p.Seen, want)
		}
	}

	// The same request again: its flow is named, and the parts are
	// listed all the same, for a task that differs a little.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	res = s.Connect(ctx, ConnectRequest{Task: "Orders", Brief: "open Dinely and show me my orders"})
	if !strings.Contains(res.Message, "This task ran before") || len(res.Parts) != 4 {
		t.Fatalf("connect: %d parts: %s", len(res.Parts), res.Message)
	}
	// The kept flow runs as the parts it names.
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	for _, fi := range g.Flows() {
		if f, _ := g.flows.Load(fi.Name); len(f.Steps) == 2 {
			res = runFlow(s, FlowRequest{Name: f.Name})
			mustStatus(t, res, StatusSettled)
			if dev.Current() != "orders" || !strings.Contains(res.Flow.Steps[1].Text, "Orders") {
				t.Fatalf("the kept flow: on %q, %+v", dev.Current(), res.Flow.Steps)
			}
		}
	}
	// A plan mixes parts with steps of the agent's own.
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	res = runFlow(s, FlowRequest{Steps: []any{
		map[string]any{"part": open},
		map[string]any{"tap": map[string]any{"role": "tab", "label": "Orders"}},
		map[string]any{"assert": map[string]any{"visible": map[string]any{"contains": "Reorder"}}},
	}})
	mustStatus(t, res, StatusSettled)
	if got := flowStatuses(res); got != "ok ok ok" || dev.Current() != "orders" {
		t.Fatalf("mixed plan: %s on %q\n%s", got, dev.Current(), res.Message)
	}
	if res = runFlow(s, FlowRequest{Steps: []any{map[string]any{"press": "Orders"}}}); res.Error != ErrInvalidArgument {
		t.Fatalf("a step of no kind: %s %s", res.Status, res.Message)
	}
	// phone_flow_list shows an app's parts on request.
	res = s.ListFlows(ctx, FlowListRequest{App: "Dinely"})
	if len(res.Parts) != 4 || res.Parts[0].Seen != 2 {
		t.Fatalf("list: %+v", res.Parts)
	}
	if res = s.ListFlows(ctx, FlowListRequest{App: "chatter"}); len(res.Parts) != 0 || !strings.Contains(res.Message, dinely) {
		t.Fatalf("list of an app without parts: %s", res.Message)
	}

	// Arguments.
	for _, req := range []FlowRequest{
		{Parts: []string{"pffffff"}},
		{Parts: []string{"orders"}},
		{Parts: []string{open}, Name: "x"},
		{Parts: []string{open}, FromStep: 2},
	} {
		if res := runFlow(s, req); res.Error != ErrInvalidArgument {
			t.Errorf("%+v: %s %s", req, res.Status, res.Message)
		}
	}
}

// A part whose action stops leading where it did is no longer offered
// after two replays; a part that does not fit the screen is not the
// part's fault.
func TestPartsBreak(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Reorder last week's lunch")
	toCart(t, s)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	// The app changed: Orders is said to lead to a screen it does not.
	wrong := flow.Step{Tap: &flow.Selector{Role: "tab", Label: "Orders"},
		Expect: &flow.Condition{App: dinely, Visible: flow.Selectors{{ID: "dinely:gone"}}}}
	if _, n, err := g.flows.LearnParts([]flow.Ran{{Step: wrong}}, time.Now()); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	bad := flow.PartID(dinely, wrong)
	ps, _ := g.flows.Parts(dinely)
	open := ps.Parts[0].ID
	src := "flow: t\nsteps:\n  - part: " + open + "\n  - part: " + bad + "\n    timeout_ms: 60\n"

	s, _ = connect(t, g, "Another task")
	res := runFlow(s, FlowRequest{YAML: src})
	mustStatus(t, res, StatusMismatch)
	if got := flowStatuses(res); got != "ok mismatch" || !strings.Contains(res.Message, "the step ran, but") {
		t.Fatalf("steps: %s\n%s", got, res.Message)
	}
	if _, p, _ := g.flows.Part(bad); p.Broke != 1 {
		t.Fatalf("broke %d times", p.Broke)
	}
	mustStatus(t, runFlow(s, FlowRequest{YAML: src}), StatusMismatch)
	if _, p, ok := g.flows.Part(bad); !ok || p.Broke != 2 {
		t.Fatalf("a part that broke twice stays for the flows that name it: %+v", p)
	}
	if res = s.ListFlows(ctx, FlowListRequest{App: dinely}); len(res.Parts) != 4 {
		t.Fatalf("a part that broke twice is still offered: %+v", res.Parts)
	}
	if res = runFlow(s, FlowRequest{Parts: []string{"pffffff"}}); res.Error != ErrInvalidArgument || !strings.Contains(res.Message, "pffffff") {
		t.Fatalf("a plan naming a part that is not there: %s %s", res.Status, res.Message)
	}
	// A part whose target is not on the screen stops the run, and stays.
	cart := ""
	for _, p := range ps.Parts {
		if p.Tap != nil && strings.Contains(p.Tap.String(), "dinely:cart") {
			cart = p.ID
		}
	}
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	res = runFlow(s, FlowRequest{YAML: "flow: t\nsteps:\n  - part: " + cart + "\n    timeout_ms: 60\n"})
	mustStatus(t, res, StatusMismatch)
	if _, p, ok := g.flows.Part(cart); !ok || p.Broke != 0 {
		t.Fatalf("a part that did not fit was blamed: %+v", p)
	}
	// What the task did all the same is learned, also when it failed.
	mustStatus(t, s.Release(ctx, ReleaseRequest{Outcome: "failed"}), StatusSettled)
	if _, p, _ := g.flows.Part(open); p.Seen != 3 {
		t.Fatalf("opening the app was seen %d times", p.Seen)
	}
	if len(g.Flows()) != 1 {
		t.Fatalf("a failed task was kept as a flow: %+v", g.Flows())
	}
}

// Opening an app shows its parts to an agent that was not shown them when
// it connected, once.
func TestPartsOfferedAtLaunch(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look around")
	step := flow.Step{Launch: dinely, Expect: &flow.Condition{App: dinely}}
	if _, _, err := g.flows.LearnParts([]flow.Ran{{Step: step}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	res := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
	mustStatus(t, res, StatusSettled)
	if len(res.Parts) != 1 || res.Parts[0].App != dinely || !strings.Contains(res.Message, "Parts of "+dinely) {
		t.Fatalf("launch: %+v %s", res.Parts, res.Message)
	}
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	if res = s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}); len(res.Parts) != 0 {
		t.Fatalf("the parts were listed twice: %s", res.Message)
	}
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)

	// A task held to the app is shown them when it connects.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	res = s.Connect(ctx, ConnectRequest{Task: "Order lunch", Apps: []string{"Dinely"}})
	if len(res.Parts) == 0 {
		t.Fatalf("connect with the app in scope: %s", res.Message)
	}
	if res = s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}); len(res.Parts) != 0 {
		t.Fatalf("listed again at launch: %s", res.Message)
	}
}

// A flow whose launch expects a screen the app does not open on any more
// (a splash screen, when the app was already running): the step ran, so
// the agent is told to go on after it, and when it starts the replay over
// instead, the kept steps still open the app once.
func TestFlowLaunchRanElsewhere(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look at my orders")
	res := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
	mustStatus(t, tap(s, ref(t, res, "Orders")), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	name := g.Flows()[0].Name
	f, err := g.flows.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	// The flow is two parts by name. The one that opens the app says, as
	// flows kept before did, that a splash screen shows.
	launch, orders := f.Steps[0].Part, f.Steps[1].Part
	ps, _ := g.flows.Parts(dinely)
	for _, p := range ps.Parts {
		if p.ID == launch {
			p.Expect.Visible = flow.Selectors{{Role: "image", Label: "Splash"}}
		}
	}
	if launch == "" || orders == "" || g.flows.SaveParts(ps) != nil {
		t.Fatalf("the kept flow: %+v", f.Steps)
	}
	f.Steps[0].TimeoutMS = 60
	if _, err := g.flows.Save(f); err != nil {
		t.Fatal(err)
	}

	// The next step looks for Orders: finding it is the check, and the
	// splash screen the launch expects stops nothing.
	s, _ = connect(t, g, "Look at my orders")
	res = runFlow(s, FlowRequest{Name: name})
	mustStatus(t, res, StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{Outcome: "cancelled"}), StatusSettled)

	// A next step that does not look first (here, one that may be left
	// out) needs the launch's checkpoint.
	f.Steps[1].Optional = true
	if _, err := g.flows.Save(f); err != nil {
		t.Fatal(err)
	}
	s, _ = connect(t, g, "Look at my orders")
	res = runFlow(s, FlowRequest{Name: name})
	mustStatus(t, res, StatusMismatch)
	if !strings.Contains(res.Message, "do not repeat it") || !strings.Contains(res.Message, "from_step 2") || strings.Contains(res.Message, "from_step 1") {
		t.Fatalf("advice after a step that ran: %s", res.Message)
	}
	// The agent starts over all the same, then goes on by hand.
	res = runFlow(s, FlowRequest{Name: name, FromStep: 1})
	mustStatus(t, res, StatusMismatch)
	mustStatus(t, tap(s, ref(t, res, "Orders")), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	// The task went on and ended well: the part that opens the app says
	// where it leads now, under the id it had, and the flow names the
	// same two parts.
	// (Version 3 is the one saved with the optional step above.)
	f, err = g.flows.Load(name)
	if err != nil || f.Version != 4 || len(f.Steps) != 2|| f.Steps[0].Part != launch || f.Steps[1].Part != orders {
		t.Fatalf("the repaired flow: %+v %v", f, err)
	}
	if _, p, _ := g.flows.Part(launch); len(p.Expect.Visible) != 0 || p.Expect.App != dinely || p.Broke != 0 {
		t.Fatalf("the part was not repaired in place: %+v", p.Expect)
	}
	// And it replays whole the next time.
	s, _ = connect(t, g, "Look at my orders")
	mustStatus(t, runFlow(s, FlowRequest{Name: name}), StatusSettled)
}

// A test run reports what it finds and changes nothing: a part that leads
// elsewhere is the finding, not something to learn from, and the run
// keeps no flow.
func TestTestRunsChangeNothing(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Look at my orders")
	res := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
	mustStatus(t, tap(s, ref(t, res, "Orders")), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	name := g.Flows()[0].Name
	f, _ := g.flows.Load(name)
	orders := f.Steps[1].Part
	// The app has a bug today: Orders leads somewhere else.
	ps, _ := g.flows.Parts(dinely)
	for _, p := range ps.Parts {
		if p.ID == orders {
			p.Expect.Visible = flow.Selectors{{ID: "dinely:gone"}}
		}
	}
	if err := g.flows.SaveParts(ps); err != nil {
		t.Fatal(err)
	}
	f.Steps[1].TimeoutMS = 60
	g.flows.Save(f)
	before, _ := g.flows.Parts(dinely)

	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	mustStatus(t, s.Connect(ctx, ConnectRequest{Task: "Run the tests", Test: true}), StatusSettled)
	for range 3 {
		mustStatus(t, runFlow(s, FlowRequest{Name: name}), StatusMismatch)
	}
	// Steps of its own do not make it a task to keep either.
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "back"}), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	after, _ := g.flows.Parts(dinely)
	if len(after.Parts) != len(before.Parts) || len(g.Flows()) != 1 {
		t.Fatalf("a test run left %d parts (were %d) and %d flows", len(after.Parts), len(before.Parts), len(g.Flows()))
	}
	for i, p := range after.Parts {
		if p.Broke != 0 || p.Seen != before.Parts[i].Seen {
			t.Errorf("a test run changed %s: seen %d, broke %d", p.Text(), p.Seen, p.Broke)
		}
	}
	if f, _ := g.flows.Load(name); f.Version != 2 {
		t.Fatalf("a test run made version %d of the flow", f.Version)
	}
}

// An app that starts cold takes longer than a screen change: the step
// after a launch waits longer for it, unless the step or the flow says
// how long to wait.
func TestStepTimeout(t *testing.T) {
	plain, quick := &flow.Flow{}, &flow.Flow{TimeoutMS: 100}
	tap, own := flow.Step{Tap: &flow.Selector{Label: "Orders"}}, flow.Step{Tap: &flow.Selector{Label: "Orders"}, TimeoutMS: 500}
	for _, c := range []struct {
		f           *flow.Flow
		st          flow.Step
		afterLaunch bool
		want        time.Duration
	}{
		{plain, tap, false, 10 * time.Second},
		{plain, tap, true, launchWait},
		{quick, tap, true, 100 * time.Millisecond},
		{plain, own, true, 500 * time.Millisecond},
		{quick, own, false, 500 * time.Millisecond},
	} {
		if got := stepTimeout(c.f, c.st, c.afterLaunch); got != c.want {
			t.Errorf("flow %d ms, step %d ms, after a launch %v: %s, want %s", c.f.TimeoutMS, c.st.TimeoutMS, c.afterLaunch, got, c.want)
		}
	}
	if launchWait <= time.Duration(flow.DefaultTimeoutMS)*time.Millisecond {
		t.Errorf("the wait after a launch (%s) is no longer than any other", launchWait)
	}
}
