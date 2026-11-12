//go:build device

package gateway

// Flows on a real phone: the routine an agent would do step by step, saved
// as a flow and replayed, with the time each way takes. Like device_test.go
// it uses only the Calculator and Settings (read-only: it opens one of the
// allowed rows and goes back), behind the same guard.
//
//	go test -c -tags device -o /tmp/pinky-gw/gateway.test ./internal/gateway
//	scripts/devlock.sh <id> env PINKY_DEVICE=<id> /tmp/pinky-gw/gateway.test -test.run TestDeviceFlow -test.v

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/driver/android"
	"github.com/DIYgod/pinky/internal/driver/ios"
	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/screen"
)

func TestDeviceFlow(t *testing.T) {
	id := os.Getenv("PINKY_DEVICE")
	if id == "" {
		t.Skip("set PINKY_DEVICE to a phone id and run through scripts/devlock.sh")
	}
	data, err := os.MkdirTemp("/tmp", "pinky-gw-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(data)
	var inner driver.Provider
	if isIOSID(id) {
		inner = ios.NewProvider()
	} else {
		inner = android.NewProvider(android.Config{})
	}
	prov := &guardProvider{Provider: inner, id: id, navRows: navRows}
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	g, err := New(Options{Providers: []driver.Provider{prov}, DataDir: data, TrustNewAgents: true, Policy: testPolicy(t),
		ApprovalTimeout: 60 * time.Second, MaxBlock: 30 * time.Second, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for deadline := time.Now().Add(60 * time.Second); !deviceOnline(g, id); time.Sleep(500 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not come online: %+v", id, g.Devices())
		}
	}
	if locked, _ := prov.Guard().Locked(context.Background()); locked {
		t.Skip("the phone is locked; left as it is")
	}
	r := &deviceRun{t: t, g: g, prov: prov, id: id}
	r.s = g.OpenSession(AgentInfo{Name: "Pinky device test"})
	defer r.s.Close()
	r.connect("Device test: flow on the Calculator")
	defer func() {
		// Leave the phone on its home screen, whatever happened.
		r.s.Act(context.Background(), Call{Device: id}, Action{Tool: "key", Key: "home"})
		r.s.Release(context.Background(), ReleaseRequest{})
	}()

	var rows []string
	t.Run("Calculator", func(t *testing.T) { r.t = t; rows = append(rows, r.flowCalculator()...) })
	t.Run("Settings", func(t *testing.T) { r.t = t; rows = append(rows, r.flowSettings()...) })
	t.Run("Mismatch", func(t *testing.T) { r.t = t; r.flowMismatch() })
	r.t = t
	t.Logf("\n%-34s %8s %8s %8s\n%s", "routine", "by hand", "replay", "settle", strings.Join(rows, "\n"))
}

func (r *deviceRun) connect(task string) {
	r.t.Helper()
	res := r.s.Connect(context.Background(), ConnectRequest{Call: Call{Device: r.id}, Task: task})
	switch {
	case res.Status == StatusLocked || screenOf(res).Locked:
		r.t.Skip("the phone is locked; left as it is")
	case res.IsError() || res.Status != StatusSettled:
		r.t.Fatalf("connect: %+v", res)
	case !allowedApp(screenOf(res).App):
		r.t.Fatalf("the phone shows %s, not the home screen, Settings or the Calculator; left as it is", screenOf(res).App.ID)
	}
}

// handTime is what the steps since from took in the gateway: what an agent
// waits for between its own thinking, without the test's pacing.
func (r *deviceRun) handTime(from int) time.Duration {
	var d time.Duration
	for _, a := range r.acts[from:] {
		d += a.took
	}
	return d
}

// saveAndReplay saves the task's steps as a flow and replays it twice from
// the home screen: as saved, and with its checkpoints taken out, so that
// every step waits for the screen to settle as an agent's steps do.
func (r *deviceRun) saveAndReplay(name string, hand time.Duration, steps int, check func(*Result)) string {
	t := r.t
	ctx := context.Background()
	res := r.s.SaveFlow(ctx, FlowSaveRequest{Call: Call{Device: r.id}, Name: name})
	if res.Status != StatusSettled {
		t.Fatalf("save: %+v", res)
	}
	f, err := flow.Parse([]byte(res.FlowYAML))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("saved %s:\n%s", name, res.FlowYAML)
	if len(f.Steps) != steps {
		t.Errorf("%d steps were saved, %d ran", len(f.Steps), steps)
	}
	var byID, byLabel, byPoint, checkpoints int
	for _, st := range f.Steps {
		if sel := st.Target(); sel != nil {
			switch {
			case sel.ID != "":
				byID++
			case sel.Point != nil:
				byPoint++
			default:
				byLabel++
			}
		}
		if st.Expect != nil {
			checkpoints++
		}
	}
	t.Logf("targets: %d by id, %d by label, %d by point; %d of %d steps have a checkpoint", byID, byLabel, byPoint, checkpoints, len(f.Steps))

	replay := func(what string, req FlowRequest) time.Duration {
		r.act("home", Action{Tool: "key", Key: "home"})
		time.Sleep(400 * time.Millisecond)
		req.Call = Call{Device: r.id}
		start := time.Now()
		res := r.s.RunFlow(ctx, req)
		took := time.Since(start)
		r.check(res)
		var lines []string
		if res.Flow != nil {
			for _, st := range res.Flow.Steps {
				lines = append(lines, fmt.Sprintf("  step %d: %-8s %5d ms  %s  %s", st.Index, st.Status, st.Elapsed.Milliseconds(), st.Text, st.Detail))
			}
		}
		t.Logf("%s: %s in %.2fs\n%s", what, res.Status, took.Seconds(), strings.Join(lines, "\n"))
		if res.Status != StatusSettled {
			t.Fatalf("%s: %s %s: %s", what, res.Status, res.Error, res.Message)
		}
		check(res)
		return took
	}
	withCheckpoints := replay("replay", FlowRequest{Name: name})
	for i := range f.Steps {
		f.Steps[i].Expect = nil
	}
	plain, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	settling := replay("replay without checkpoints", FlowRequest{YAML: string(plain)})
	return fmt.Sprintf("%-34s %7.2fs %7.2fs %7.2fs", fmt.Sprintf("%s (%d steps)", name, steps), hand.Seconds(), withCheckpoints.Seconds(), settling.Seconds())
}

func (r *deviceRun) flowCalculator() []string {
	t := r.t
	from := len(r.acts)
	res := r.launch("com.android.calculator2", "com.google.android.calculator", "com.apple.calculator", "Calculator", "计算器")
	if res.Status != StatusSettled {
		t.Fatalf("launch: %+v", res)
	}
	cur := res
	equals := []string{"=", "equals", "Equals", "等于"}
	keys := [][]string{equals, {"6"}, {"×", "*", "x", "multiply", "Multiply", "乘", "乘以"}, {"7"}, equals}
	for _, names := range keys {
		// The keys do not move: every tap is found on the first screen.
		var res *Result
		if e, ok := find(cur.Screen, names...); ok {
			res = r.act("tap "+e.Label, Action{Tool: "tap", Ref: e.Ref})
		} else if p, ok := calcKey(cur.Screen, names[0]); ok {
			res = r.act(fmt.Sprintf("tap %s at %d,%d", names[0], p.X, p.Y), Action{Tool: "tap", Point: &p})
		} else {
			t.Fatalf("no %v key on the Calculator:\n%s", names, cur.Screen.Render())
		}
		if res.Status != StatusSettled {
			t.Fatalf("tap %v: %+v", names, res)
		}
	}
	steps := len(r.acts) - from
	hand := r.handTime(from)
	return []string{r.saveAndReplay("calculator.six-times-seven", hand, steps, func(res *Result) {
		if !shows42(res.Screen) {
			t.Errorf("42 is not on the screen after the replay:\n%s", res.Screen.Render())
		}
	})}
}

func shows42(s *screen.Screen) bool {
	if s == nil {
		return false
	}
	for _, e := range s.Elements {
		l := strings.TrimSpace(e.Label)
		if l == "42" || strings.HasSuffix(l, "= 42") || strings.HasSuffix(l, ", 42") || strings.HasPrefix(l, "42,") || strings.HasSuffix(l, "value:42") {
			return true
		}
	}
	return false
}

// flowSettings opens one allowed row of Settings and goes back: steps that
// change the screen, which is where checkpoints matter.
func (r *deviceRun) flowSettings() []string {
	t := r.t
	// Bring Settings to its main page first, outside the routine.
	res := r.launch("com.android.settings", "com.apple.Preferences", "Settings", "设置")
	if res.Status != StatusSettled || !isSettings(screenOf(res).App) {
		t.Fatalf("launch Settings: %+v", res)
	}
	top := r.observe()
	for i := 0; i < 3 && !hasSearch(top.Screen); i++ {
		if res = r.act("back to the main page", Action{Tool: "key", Key: "back"}); !isSettings(screenOf(res).App) {
			r.launch("com.android.settings", "com.apple.Preferences")
		}
		top = r.observe()
	}
	var row *screen.Element
	for i, e := range top.Screen.Elements {
		c := e.Bounds.Center()
		if e.Ref != 0 && !e.Bounds.Empty() && !isToggle(e) && matchesRow(e, navRows) && c.Y > top.Screen.Height*12/100 && c.Y < top.Screen.Height*88/100 {
			row = &top.Screen.Elements[i]
			break
		}
	}
	if row == nil {
		t.Skipf("none of the allowed rows is on the first page of Settings:\n%s", top.Screen.Render())
	}
	r.act("home", Action{Tool: "key", Key: "home"})

	// A new task, so that the flow holds this routine only.
	r.connect("Device test: flow in Settings")
	from := len(r.acts)
	res = r.launch("com.android.settings", "com.apple.Preferences", "Settings", "设置")
	if res.Status != StatusSettled {
		t.Fatalf("launch Settings: %+v", res)
	}
	e, ok := find(res.Screen, row.Label)
	if !ok {
		t.Skipf("%q is not on the screen after opening Settings again:\n%s", row.Label, res.Screen.Render())
	}
	if res = r.act("open "+e.Label, Action{Tool: "tap", Ref: e.Ref}); res.Status != StatusSettled {
		t.Fatalf("open %s: %+v", e.Label, res)
	}
	if res = r.act("back", Action{Tool: "key", Key: "back"}); res.Status != StatusSettled {
		t.Fatalf("back: %+v", res)
	}
	steps := len(r.acts) - from
	hand := r.handTime(from)
	return []string{r.saveAndReplay("settings.open-a-row", hand, steps, func(res *Result) {
		if !isSettings(screenOf(res).App) {
			t.Errorf("the replay ended in %s", screenOf(res).App.ID)
		}
	})}
}

// flowMismatch checks that a replay stops, without input, at a target the
// screen does not have.
func (r *deviceRun) flowMismatch() {
	t := r.t
	r.act("home", Action{Tool: "key", Key: "home"})
	start := time.Now()
	res := r.s.RunFlow(context.Background(), FlowRequest{Call: Call{Device: r.id},
		YAML: "flow: nothing-there\nsteps:\n  - tap: { role: button, label: \"No such button 4711\" }\n    timeout_ms: 1500\n"})
	t.Logf("mismatch: %s in %.2fs: %s", res.Status, time.Since(start).Seconds(), res.Message)
	r.check(res)
	if res.Status != StatusMismatch || res.Screen == nil || !res.Full {
		t.Fatalf("want a mismatch with the screen: %+v", res)
	}
}
