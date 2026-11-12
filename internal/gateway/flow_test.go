package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/flow"
)

func runFlow(s *Session, req FlowRequest) *Result { return s.RunFlow(context.Background(), req) }

func flowStatuses(res *Result) string {
	if res.Flow == nil {
		return ""
	}
	var out []string
	for _, st := range res.Flow.Steps {
		out = append(out, st.Status)
	}
	return strings.Join(out, " ")
}

// An agent walks to the cart once, saves the steps, and a later task
// replays them without the agent choosing a single ref.
func TestFlowSaveAndReplay(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond })
	s, _ := connect(t, g, "Reorder last week's lunch")
	toCart(t, s)
	res := s.SaveFlow(context.Background(), FlowSaveRequest{Name: "dinely.to-cart", Description: "Open the cart with last week's lunch"})
	mustStatus(t, res, StatusSettled)
	f, err := flow.Parse([]byte(res.FlowYAML))
	if err != nil {
		t.Fatalf("%v\n%s", err, res.FlowYAML)
	}
	if f.Version != 1 || len(f.Steps) != 4 || f.App != "com.dinely.app" {
		t.Fatalf("flow: version %d, %d steps, app %q\n%s", f.Version, len(f.Steps), f.App, res.FlowYAML)
	}
	if f.Steps[0].Launch != "com.dinely.app" {
		t.Errorf("a launch is kept by app id: %q", f.Steps[0].Launch)
	}
	if tap := f.Steps[1].Tap; tap == nil || tap.ID != "dinely:tab:orders" || len(tap.Or) != 1 || tap.Or[0].Label != "Orders" {
		t.Errorf("a target is its id first and its role and label second: %+v", tap)
	}
	for i, st := range f.Steps {
		if st.Expect == nil {
			t.Errorf("step %d has no checkpoint", i+1)
		}
	}
	if c := f.Steps[2].Expect; len(c.Visible) != 1 || c.Visible[0].ID != "dinely:cart" {
		t.Errorf("the checkpoint after Reorder is the cart button, by id, since its label counts: %+v", c)
	}
	mustStatus(t, s.Release(context.Background(), ReleaseRequest{}), StatusSettled)

	// Another session, another day: the cart already holds one bowl, so
	// the cart button's label differs from the recorded one.
	s2 := g.OpenSession(agent)
	t.Cleanup(s2.Close)
	mustStatus(t, s2.Connect(context.Background(), ConnectRequest{Task: "Replay"}), StatusSettled)
	inputs := len(dev.Inputs())
	res = runFlow(s2, FlowRequest{Name: "dinely.to-cart"})
	mustStatus(t, res, StatusSettled)
	if got := flowStatuses(res); got != "ok ok ok ok" {
		t.Fatalf("steps: %s\n%s", got, res.Message)
	}
	if dev.Current() != "cart" || dev.Cart() != 2 {
		t.Fatalf("the phone is on %q with %d in the cart", dev.Current(), dev.Cart())
	}
	if n := len(dev.Inputs()) - inputs; n != 4 {
		t.Errorf("%d inputs for 4 steps", n)
	}
	if !res.Full || res.Screen == nil {
		t.Fatalf("a replay returns the whole screen: %+v", res)
	}
	screenText := res.Screen.Render()
	if !strings.Contains(screenText, "Place order") || !strings.Contains(screenText, `toast: "Added to cart"`) {
		t.Errorf("the screen misses the cart or the toast a step met on the way:\n%s", screenText)
	}
	// The replay is in the task's history, step by step.
	var tools []string
	for _, task := range g.Tasks() {
		if task.Title == "Replay" {
			for _, st := range task.Steps {
				tools = append(tools, st.Tool)
			}
		}
	}
	if got := strings.Join(tools, " "); got != "phone_connect phone_launch phone_tap phone_tap phone_tap phone_flow_run" {
		t.Errorf("history: %s", got)
	}
	// Saving again bumps the version.
	res = s2.SaveFlow(context.Background(), FlowSaveRequest{YAML: res2yaml(t, g, "dinely.to-cart")})
	mustStatus(t, res, StatusSettled)
	if got := g.Flows(); len(got) != 1 || got[0].Version != 2 || got[0].Steps != 4 {
		t.Errorf("flows: %+v", got)
	}
}

func res2yaml(t *testing.T, g *Gateway, name string) string {
	t.Helper()
	f, err := g.flows.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A target that is not there stops the replay before anything is done,
// with the screen as it is; the agent goes on from that step.
func TestFlowMismatch(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond })
	s, _ := connect(t, g, "Order")
	src := `
flow: dinely.checkout
timeout_ms: 60
steps:
  - launch: Dinely
    expect: { app: Dinely, visible: { id: "dinely:title" } }
  - tap: { role: button, label: Checkout }
  - tap: Orders
`
	inputs := len(dev.Inputs())
	res := runFlow(s, FlowRequest{YAML: src})
	mustStatus(t, res, StatusMismatch)
	if got := flowStatuses(res); got != "ok mismatch" {
		t.Fatalf("steps: %s", got)
	}
	if n := len(dev.Inputs()) - inputs; n != 1 {
		t.Errorf("%d inputs; only the launch should have run", n)
	}
	for _, want := range []string{"step 2 of 3", "Checkout", "from_step 2", "or 3"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("message misses %q: %s", want, res.Message)
		}
	}
	if !res.Full || res.Screen == nil || !strings.Contains(res.Screen.Render(), "Search dishes") {
		t.Fatalf("the mismatch should show the screen: %+v", res)
	}
	// The agent skips the broken step and the replay goes on.
	res = runFlow(s, FlowRequest{YAML: src, FromStep: 3})
	mustStatus(t, res, StatusSettled)
	if dev.Current() != "orders" {
		t.Errorf("on %q", dev.Current())
	}

	// A checkpoint that does not hold stops the replay after the step.
	res = runFlow(s, FlowRequest{YAML: `
flow: a
steps:
  - tap: Home
    expect: { visible: Nope }
    timeout_ms: 60
  - key: back
`})
	mustStatus(t, res, StatusMismatch)
	if got := flowStatuses(res); got != "mismatch" || !strings.Contains(res.Message, "the step ran") {
		t.Errorf("steps %q: %s", got, res.Message)
	}
	if dev.Current() != "dinely" {
		t.Errorf("the tap should have run; on %q", dev.Current())
	}

	// Two elements that match are a mismatch, not a guess.
	res = runFlow(s, FlowRequest{YAML: "flow: a\ntimeout_ms: 60\nsteps:\n  - tap: Orders\n  - tap: { role: button, contains: Reorder }\n"})
	mustStatus(t, res, StatusMismatch)
	if !strings.Contains(res.Message, "2 elements match") || dev.Cart() != 0 {
		t.Errorf("cart %d: %s", dev.Cart(), res.Message)
	}
}

// A step waits for its target while the screen loads, and a checkpoint is
// what the replay waits for after an action.
func TestFlowWaits(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond })
	s, _ := connect(t, g, "Order")
	mustStatus(t, s.Act(context.Background(), Call{}, Action{Tool: "launch", App: "Dinely"}), StatusSettled)
	go func() {
		time.Sleep(80 * time.Millisecond)
		dev.Show("orders")
	}()
	res := runFlow(s, FlowRequest{YAML: `
flow: a
steps:
  - assert: { visible: { id: "dinely:reorder:1" }, gone: Account }
    timeout_ms: 2000
  - tap: { contains: beef }
    expect: { visible: { id: "dinely:cart" } }
`})
	mustStatus(t, res, StatusSettled)
	if dev.Cart() != 1 {
		t.Errorf("cart %d", dev.Cart())
	}
}

func TestFlowParamsAndOptional(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond })
	s, _ := connect(t, g, "Order")
	toCart(t, s)
	src := `
flow: dinely.note
params:
  note: {}
steps:
  - tap: { label: "Got it" }
    optional: true
    name: close the tip, if it shows
  - type: "${note}"
    into: { id: "dinely:note" }
`
	if res := runFlow(s, FlowRequest{YAML: src}); res.Error != ErrInvalidArgument || !strings.Contains(res.Message, "note") {
		t.Fatalf("a missing parameter: %+v", res)
	}
	res := runFlow(s, FlowRequest{YAML: src, Params: map[string]string{"note": "no cilantro"}})
	mustStatus(t, res, StatusSettled)
	if got := flowStatuses(res); got != "skipped ok" {
		t.Fatalf("steps: %s", got)
	}
	if !strings.Contains(res.Screen.Render(), "no cilantro") {
		t.Errorf("the note was not typed:\n%s", res.Screen.Render())
	}
	_ = dev

	// Typed text becomes a parameter when the agent names it.
	res = s.SaveFlow(context.Background(), FlowSaveRequest{Name: "dinely.cart-note", Params: map[string]string{"note": "no cilantro"}})
	mustStatus(t, res, StatusSettled)
	f, err := flow.Parse([]byte(res.FlowYAML))
	if err != nil {
		t.Fatal(err)
	}
	last := f.Steps[len(f.Steps)-1]
	if last.Type == nil || *last.Type != "${note}" || last.Into == nil || last.Into.ID != "dinely:note" || last.Expect != nil {
		t.Errorf("last step: %+v", last)
	}
	if p := f.Params["note"]; p.Default == nil || *p.Default != "no cilantro" {
		t.Errorf("params: %+v", f.Params)
	}
}

// A replay grants nothing: the rules ask and refuse as they do for an
// agent's own steps.
func TestFlowKeepsTheRules(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.MaxBlock = 80 * time.Millisecond })
	s, _ := connect(t, g, "Order")
	toCart(t, s)
	src := `
flow: dinely.pay
steps:
  - tap: { id: "dinely:place" }
    expect: { visible: { id: "dinely:placed" } }
    intent: Pay for the order
  - tap: Back to home
`
	res := runFlow(s, FlowRequest{YAML: src})
	mustStatus(t, res, StatusPending)
	if res.ApprovalID == "" || res.Rule != "Payments" || !strings.Contains(res.Message, "from_step 1") {
		t.Fatalf("pending: %+v", res)
	}
	if dev.Current() != "cart" {
		t.Fatalf("paid without asking; on %q", dev.Current())
	}
	n := waitNeed(t, g)
	if n.Detail == "" || !strings.Contains(n.Detail, "Pay for the order") {
		t.Errorf("the approval should show the step's intent: %+v", n.Detail)
	}
	if err := g.Decide(n.ID, AllowOnce, ""); err != nil {
		t.Fatal(err)
	}
	res = runFlow(s, FlowRequest{YAML: src, FromStep: 1, Call: Call{ApprovalID: res.ApprovalID}})
	mustStatus(t, res, StatusSettled)
	if got := flowStatuses(res); got != "ok ok" || dev.Current() != "dinely" {
		t.Fatalf("steps %q, on %q: %s", got, dev.Current(), res.Message)
	}

	// Password fields stay off limits.
	res = runFlow(s, FlowRequest{YAML: `
flow: dinely.signin
steps:
  - tap: Account
  - type: hunter2
    into: { id: "dinely:password" }
`})
	mustStatus(t, res, StatusBlocked)
	if got := flowStatuses(res); got != "ok blocked_by_policy" {
		t.Errorf("steps: %s", got)
	}

	// A read-only task replays nothing its rules would ask about.
	s2 := g.OpenSession(AgentInfo{Name: "Reader"})
	t.Cleanup(s2.Close)
	mustStatus(t, s.Release(context.Background(), ReleaseRequest{}), StatusSettled)
	mustStatus(t, s2.Connect(context.Background(), ConnectRequest{Task: "Look", ReadOnly: true}), StatusSettled)
	res = runFlow(s2, FlowRequest{YAML: "flow: chat\nsteps:\n  - launch: Chatter\n  - tap: Sam\n  - type: hi\n    into: { id: \"chatter:input\" }\n    submit: true\n"})
	if res.Status != StatusBlocked || res.Rule != "Read-only" {
		t.Errorf("read-only: %+v", res)
	}
}

func TestFlowArguments(t *testing.T) {
	g, _ := newTestGateway(t)
	s, _ := connect(t, g, "Order")
	for name, req := range map[string]FlowRequest{
		"neither":       {},
		"both":          {Name: "a", YAML: "flow: a\nsteps: [{key: back}]"},
		"bad yaml":      {YAML: "flow: a\nsteps: [{keyy: back}]"},
		"from too far":  {YAML: "flow: a\nsteps: [{key: back}]", FromStep: 2},
		"unknown param": {YAML: "flow: a\nsteps: [{key: back}]", Params: map[string]string{"x": "y"}},
	} {
		if res := runFlow(s, req); res.Error != ErrInvalidArgument {
			t.Errorf("%s: %+v", name, res)
		}
	}
	if res := runFlow(s, FlowRequest{Name: "nope"}); res.Error != ErrNotFound {
		t.Errorf("unknown flow: %+v", res)
	}
	if res := s.SaveFlow(context.Background(), FlowSaveRequest{Name: "a"}); res.Error != ErrInvalidArgument {
		t.Errorf("saving a task without steps: %+v", res)
	}
	if res := s.ListFlows(context.Background(), FlowListRequest{}); res.Status != StatusSettled || len(res.Flows) != 0 {
		t.Errorf("list: %+v", res)
	}
	// A session without a phone reads and saves no flows.
	s2 := g.OpenSession(AgentInfo{Name: "Stranger"})
	t.Cleanup(s2.Close)
	if res := s2.ListFlows(context.Background(), FlowListRequest{}); res.Error != ErrInvalidArgument {
		t.Errorf("list without a lease: %+v", res)
	}
	if res := s2.SaveFlow(context.Background(), FlowSaveRequest{YAML: "flow: a\nsteps: [{key: back}]"}); res.Error != ErrInvalidArgument {
		t.Errorf("save without a lease: %+v", res)
	}
}

// Nobody saves or names anything: a task that worked replays the next
// time the same task comes up, and a flow that broke repairs itself.
func TestFlowsByThemselves(t *testing.T) {
	g, dev := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, first := connect(t, g, "Reorder last week's lunch")
	if len(first.Flows) != 0 || strings.Contains(first.Message, "flow") {
		t.Fatalf("nothing to offer yet: %s", first.Message)
	}
	toCart(t, s)
	res := s.Release(ctx, ReleaseRequest{})
	mustStatus(t, res, StatusSettled)
	flows := g.Flows()
	if len(flows) != 1 || flows[0].Steps != 4 || flows[0].Description != "Reorder last week's lunch" {
		t.Fatalf("flows after the first task: %+v", flows)
	}
	name := flows[0].Name
	if !strings.HasPrefix(name, "app.reorder-last-week-s-lunc-") || !strings.Contains(res.Message, name) {
		t.Fatalf("name %q; release said: %s", name, res.Message)
	}

	// The same task again: connecting says which flow replays it.
	open := func(task string) (*Session, *Result) {
		s := g.OpenSession(agent)
		t.Cleanup(s.Close)
		res := s.Connect(ctx, ConnectRequest{Task: task})
		mustStatus(t, res, StatusSettled)
		return s, res
	}
	s, res = open("reorder last  week's lunch")
	if len(res.Flows) != 1 || !strings.Contains(res.Message, "This task ran before") || !strings.Contains(res.Message, name) {
		t.Fatalf("connect: %+v\n%s", res.Flows, res.Message)
	}
	mustStatus(t, runFlow(s, FlowRequest{Name: name}), StatusSettled)
	if dev.Current() != "cart" {
		t.Fatalf("on %q", dev.Current())
	}
	// A task that only replayed leaves the flow as it is.
	if res = s.Release(ctx, ReleaseRequest{}); strings.Contains(res.Message, "kept") || g.Flows()[0].Version != 1 {
		t.Fatalf("a replay alone made a new version: %s", res.Message)
	}

	// The same task done by hand once more does not replace the flow.
	s, _ = open("Reorder last week's lunch")
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}), StatusSettled)
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	if res = s.Release(ctx, ReleaseRequest{}); strings.Contains(res.Message, "kept") || g.Flows()[0].Version != 1 || g.Flows()[0].Steps != 4 {
		t.Fatalf("a run by hand replaced the flow: %s %+v", res.Message, g.Flows())
	}

	// Another task sees the flow in the list, without the hint.
	s, res = open("Check the account page")
	if len(res.Flows) != 1 || strings.Contains(res.Message, "ran before") || !strings.Contains(res.Message, "listed below") {
		t.Fatalf("connect for another task: %s", res.Message)
	}
	// It fails: nothing is kept.
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"}), StatusSettled)
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "key", Key: "home"}), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{Outcome: "failed"}), StatusSettled)
	if len(g.Flows()) != 1 {
		t.Fatalf("a failed task was kept: %+v", g.Flows())
	}

	// The app changes: the flow's second step no longer finds its target.
	f, err := g.flows.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Steps[1].Tap = &flow.Selector{Role: "tab", Label: "Order history"}
	f.Steps[1].TimeoutMS = 60
	if _, err := g.flows.Save(f); err != nil {
		t.Fatal(err)
	}
	s, _ = open("Reorder last week's lunch")
	res = runFlow(s, FlowRequest{Name: name})
	mustStatus(t, res, StatusMismatch)
	// The agent does the step by hand and lets the flow go on.
	mustStatus(t, tap(s, ref(t, res, "Orders")), StatusSettled)
	mustStatus(t, runFlow(s, FlowRequest{Name: name, FromStep: 3}), StatusSettled)
	res = s.Release(ctx, ReleaseRequest{})
	if !strings.Contains(res.Message, name) {
		t.Fatalf("the repaired flow was not kept: %s", res.Message)
	}
	f, err = g.flows.Load(name)
	if err != nil || f.Version != 3 || len(f.Steps) != 4 || f.Steps[1].Tap.ID != "dinely:tab:orders" || f.Task != taskKey("Reorder last week's lunch") || strings.Contains(f.Steps[0].Intent, "Flow") {
		t.Fatalf("the flow did not repair itself: %+v %v", f, err)
	}
	if len(g.Flows()) != 1 {
		t.Fatalf("the repair made another flow: %+v", g.Flows())
	}
	// And it replays again.
	s, _ = open("Reorder last week's lunch")
	mustStatus(t, runFlow(s, FlowRequest{Name: name}), StatusSettled)
}

// What a task typed becomes a parameter of the flow kept from it, with the
// typed text as its default, and a task named in another script gets a
// name all the same.
func TestKeptFlowParams(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	s, _ := connect(t, g, "カートにメモを書く")
	cart := toCart(t, s)
	mustStatus(t, s.Act(ctx, Call{}, Action{Tool: "type", Ref: ref(t, cart, "Note for the restaurant"), Text: "no cilantro"}), StatusSettled)
	mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
	flows := g.Flows()
	if len(flows) != 1 || !strings.HasPrefix(flows[0].Name, "app.") || len(flows[0].Params) != 1 || flows[0].Params[0] != `text (default "no cilantro")` {
		t.Fatalf("flows: %+v", flows)
	}
	f, err := g.flows.Load(flows[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if last := f.Steps[len(f.Steps)-1]; last.Type == nil || *last.Type != "${text}" {
		t.Fatalf("last step: %+v", last)
	}
	s2 := g.OpenSession(agent)
	t.Cleanup(s2.Close)
	mustStatus(t, s2.Connect(ctx, ConnectRequest{Task: "カートにメモを書く"}), StatusSettled)
	res := runFlow(s2, FlowRequest{Name: f.Name, Params: map[string]string{"text": "extra spicy"}})
	mustStatus(t, res, StatusSettled)
	if !strings.Contains(res.Screen.Render(), "extra spicy") {
		t.Fatalf("the replay did not type the new text:\n%s", res.Screen.Render())
	}
}

// A task is known by what the person asked for, when the agent passes it:
// a chat's short title can be the same for two requests.
func TestKeptFlowsGoByTheBrief(t *testing.T) {
	g, _ := newTestGateway(t, func(o *Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	ctx := context.Background()
	run := func(brief string, steps func(*Session)) *Result {
		s := g.OpenSession(agent)
		t.Cleanup(s.Close)
		res := s.Connect(ctx, ConnectRequest{Task: "Test Dinely…", Brief: brief})
		mustStatus(t, res, StatusSettled)
		if steps != nil {
			steps(s)
		}
		mustStatus(t, s.Release(ctx, ReleaseRequest{}), StatusSettled)
		return res
	}
	run("Test Dinely: open the cart with last week's lunch", func(s *Session) { toCart(t, s) })
	// Another request under the same title is another task.
	res := run("Test Dinely: open the account page", func(s *Session) {
		r := s.Act(ctx, Call{}, Action{Tool: "launch", App: "Dinely"})
		mustStatus(t, tap(s, ref(t, r, "Account")), StatusSettled)
	})
	if strings.Contains(res.Message, "ran before") {
		t.Fatalf("another request counted as the same task: %s", res.Message)
	}
	flows := g.Flows()
	if len(flows) != 2 || flows[0].Name == flows[1].Name || flows[0].Steps+flows[1].Steps != 6 {
		t.Fatalf("flows: %+v", flows)
	}
	// The first request again, spaced and cased differently.
	res = run("test dinely:  open the cart with last week's lunch", nil)
	if !strings.Contains(res.Message, "This task ran before") || len(res.Flows) != 2 {
		t.Fatalf("the same request was not recognized: %s", res.Message)
	}
	for _, f := range flows {
		if f.Steps == 4 && !strings.Contains(res.Message, f.Name) {
			t.Fatalf("the hint names another flow than %s: %s", f.Name, res.Message)
		}
	}
}
