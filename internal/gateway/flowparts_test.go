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
	if got := flowStatuses(res); got != "ok ok" || !strings.Contains(res.Message, "The 2 parts ran") {
		t.Fatalf("steps: %s\n%s", got, res.Message)
	}
	if dev.Current() != "orders" || len(dev.Inputs())-inputs != 2 {
		t.Fatalf("on %q after %d inputs", dev.Current(), len(dev.Inputs())-inputs)
	}
	if !res.Full || res.Screen == nil || !strings.Contains(res.Screen.Render(), "Reorder") {
		t.Fatalf("the run returns the screen it ended on: %+v", res)
	}
	// The task the agent put together is kept as a flow of its own, and
	// its steps were seen once more.
	res = s.Release(ctx, ReleaseRequest{})
	if flows := g.Flows(); len(flows) != 2 || !strings.Contains(res.Message, "kept the task's steps") {
		t.Fatalf("flows: %+v\n%s", flows, res.Message)
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

	// The same request again is that flow: the parts are not listed.
	s = g.OpenSession(agent)
	t.Cleanup(s.Close)
	res = s.Connect(ctx, ConnectRequest{Task: "Orders", Brief: "open Dinely and show me my orders"})
	if !strings.Contains(res.Message, "This task ran before") || len(res.Parts) != 0 {
		t.Fatalf("connect: %d parts: %s", len(res.Parts), res.Message)
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

// A part whose action stops leading where it did is dropped after two
// replays; a part that does not fit the screen is not the part's fault.
func TestPartsBreak(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "Reorder last week's lunch")
	toCart(t, s)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	// The app changed: Orders is said to lead to a screen it does not.
	wrong := flow.Step{Tap: &flow.Selector{Role: "tab", Label: "Orders"},
		Expect: &flow.Condition{App: dinely, Visible: flow.Selectors{{ID: "dinely:gone"}}}}
	if n, err := g.flows.LearnParts([]flow.Step{wrong}, nil, time.Now()); err != nil || n != 1 {
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
	if _, _, ok := g.flows.Part(bad); !ok {
		t.Fatal("dropped at the first break")
	}
	mustStatus(t, runFlow(s, FlowRequest{YAML: src}), StatusMismatch)
	if _, _, ok := g.flows.Part(bad); ok {
		t.Fatal("still there after breaking twice")
	}
	if res = runFlow(s, FlowRequest{YAML: src}); res.Error != ErrInvalidArgument || !strings.Contains(res.Message, bad) {
		t.Fatalf("a flow naming a part that is gone: %s %s", res.Status, res.Message)
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
	if _, err := g.flows.LearnParts([]flow.Step{step}, nil, time.Now()); err != nil {
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
