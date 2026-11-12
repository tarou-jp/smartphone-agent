package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/mcp"
	"github.com/DIYgod/pinky/internal/screen"
)

// PhoneTools runs Pinky's phone_* tools on one gateway session.
type PhoneTools struct {
	Sess *gateway.Session
}

type gwCall func(ctx context.Context, c gateway.Call) *gateway.Result

// Call runs one tool by MCP name with JSON arguments (same shape as MCP).
func (p *PhoneTools) Call(ctx context.Context, tool string, args map[string]any) string {
	return p.call(ctx, tool, args, nil)
}

// call is Call that reports what a blocking call waits for (the person's
// approval, a handoff) to progress.
func (p *PhoneTools) call(ctx context.Context, tool string, args map[string]any, progress func(string)) string {
	if p == nil || p.Sess == nil {
		return "error: session · in-app agent not connected"
	}
	tool = strings.TrimSpace(tool)
	var fn gwCall
	switch tool {
	case "phone_connect":
		task, apps, ro, brief := str(args, "task"), strSlice(args, "apps"), boolArg(args, "read_only"), str(args, "brief")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Connect(ctx, gateway.ConnectRequest{Call: c, Task: task, Apps: apps, ReadOnly: ro, Brief: brief})
		}
	case "phone_release":
		out, sum := str(args, "outcome"), str(args, "summary")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Release(ctx, gateway.ReleaseRequest{Call: c, Outcome: out, Summary: sum})
		}
	case "phone_observe":
		wait, shot := intArg(args, "wait_ms"), boolArg(args, "screenshot")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Observe(ctx, gateway.ObserveRequest{Call: c, WaitMS: wait, Screenshot: shot})
		}
	case "phone_tap":
		ref, lp := intArg(args, "ref"), boolArg(args, "long_press")
		var pt *screen.Point
		if x, y, ok := xy(args); ok {
			pt = &screen.Point{X: x, Y: y}
		}
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Act(ctx, c, gateway.Action{Tool: "tap", Ref: ref, Point: pt, LongPress: lp})
		}
	case "phone_type":
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Act(ctx, c, gateway.Action{
				Tool: "type", Ref: intArg(args, "ref"), Text: str(args, "text"),
				Append: boolArg(args, "append"), Submit: boolArg(args, "submit"),
			})
		}
	case "phone_swipe":
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Act(ctx, c, gateway.Action{
				Tool: "swipe", Ref: intArg(args, "ref"),
				Direction: strings.ToLower(str(args, "direction")),
				From:      pointArg(args, "from"), To: pointArg(args, "to"),
				DurationMS: intArg(args, "duration_ms"),
			})
		}
	case "phone_key":
		key := str(args, "key")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Act(ctx, c, gateway.Action{Tool: "key", Key: key})
		}
	case "phone_launch":
		app := str(args, "app")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Act(ctx, c, gateway.Action{Tool: "launch", App: app})
		}
	case "phone_apps":
		q := str(args, "query")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Apps(ctx, gateway.AppsRequest{Call: c, Query: q})
		}
	case "phone_handoff":
		reason := str(args, "reason")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Handoff(ctx, gateway.HandoffRequest{Call: c, Reason: reason})
		}
	case "phone_wait_for":
		text := str(args, "text")
		gone := boolArg(args, "gone")
		timeout := intArg(args, "timeout_ms")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.WaitFor(ctx, gateway.WaitForRequest{Call: c, Text: text, Gone: gone, TimeoutMS: timeout})
		}
	case "phone_flow_run":
		name, src, params, from := str(args, "flow"), str(args, "yaml"), strMap(args, "params"), intArg(args, "from_step")
		parts := strSlice(args, "parts")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			req := gateway.FlowRequest{Call: c, Name: name, YAML: src, Parts: parts, Params: params, FromStep: from}
			res := p.Sess.RunFlow(ctx, req)
			// callPending repeats the call with the approval's id: the
			// replay then goes on at the step that waits.
			if res != nil && res.Status == gateway.StatusPending && res.Flow != nil && len(res.Flow.Steps) > 0 {
				from = res.Flow.Steps[len(res.Flow.Steps)-1].Index
			}
			return res
		}
	case "phone_flow_save":
		req := gateway.FlowSaveRequest{Name: str(args, "name"), Description: str(args, "description"), YAML: str(args, "yaml"), Params: strMap(args, "params")}
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			req.Call = c
			return p.Sess.SaveFlow(ctx, req)
		}
	case "phone_flow_list":
		name, app := str(args, "name"), str(args, "app")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.ListFlows(ctx, gateway.FlowListRequest{Call: c, Name: name, App: app})
		}
	default:
		return fmt.Sprintf("error: invalid_argument · unknown tool %q", tool)
	}
	return mcp.FormatResult(p.callPending(ctx, args, fn, progress))
}

// callPending waits out an approval the call opened: the gateway blocks a
// while on each call and answers approval_pending while the person has
// not decided, so the call is repeated with the approval's id until they
// decide, the approval expires, or ctx ends. An MCP agent does the same,
// one round trip at a time.
func (p *PhoneTools) callPending(ctx context.Context, args map[string]any, fn gwCall, progress func(string)) *gateway.Result {
	c := callArgs(args)
	if progress != nil {
		c.Progress = func(_, _ float64, msg string) { progress(msg) }
	}
	res := fn(ctx, c)
	for res != nil && res.Status == gateway.StatusPending && res.ApprovalID != "" && ctx.Err() == nil {
		if progress != nil {
			progress(res.Message)
		}
		c.ApprovalID = res.ApprovalID
		start := time.Now()
		res = fn(ctx, c)
		// A gateway that answers at once would make this a busy loop.
		if res != nil && res.Status == gateway.StatusPending && time.Since(start) < 200*time.Millisecond {
			select {
			case <-ctx.Done():
			case <-time.After(400 * time.Millisecond):
			}
		}
	}
	return res
}

func callArgs(m map[string]any) gateway.Call {
	return gateway.Call{
		Device:     strings.TrimSpace(str(m, "device")),
		Intent:     strings.TrimSpace(str(m, "intent")),
		ApprovalID: strings.TrimSpace(str(m, "approval_id")),
	}
}

func str(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func intArg(m map[string]any, k string) int {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}

func boolArg(m map[string]any, k string) bool {
	v, ok := m[k]
	if !ok || v == nil {
		return false
	}
	b, _ := v.(bool)
	return b
}

func strSlice(m map[string]any, k string) []string {
	v, ok := m[k]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprint(e))
		}
		return out
	default:
		return nil
	}
}

// strMap reads an object of strings, as phone_flow_run's params.
func strMap(m map[string]any, k string) map[string]string {
	sub, ok := m[k].(map[string]any)
	if !ok || len(sub) == 0 {
		return nil
	}
	out := make(map[string]string, len(sub))
	for name := range sub {
		out[name] = str(sub, name)
	}
	return out
}

func xy(m map[string]any) (int, int, bool) {
	_, xok := m["x"]
	_, yok := m["y"]
	if !xok || !yok {
		return 0, 0, false
	}
	return intArg(m, "x"), intArg(m, "y"), true
}

func pointArg(m map[string]any, k string) *screen.Point {
	v, ok := m[k]
	if !ok || v == nil {
		return nil
	}
	sub, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return &screen.Point{X: intArg(sub, "x"), Y: intArg(sub, "y")}
}
