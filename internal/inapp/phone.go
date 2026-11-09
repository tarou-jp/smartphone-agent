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
	if p == nil || p.Sess == nil {
		return "error: session · in-app agent not connected"
	}
	tool = strings.TrimSpace(tool)
	var fn gwCall
	switch tool {
	case "phone_connect":
		task, apps, ro := str(args, "task"), strSlice(args, "apps"), boolArg(args, "read_only")
		fn = func(ctx context.Context, c gateway.Call) *gateway.Result {
			return p.Sess.Connect(ctx, gateway.ConnectRequest{Call: c, Task: task, Apps: apps, ReadOnly: ro})
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
				From: pointArg(args, "from"), To: pointArg(args, "to"),
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
	default:
		return fmt.Sprintf("error: invalid_argument · unknown tool %q", tool)
	}
	return mcp.FormatResult(p.callPending(ctx, args, fn))
}

func (p *PhoneTools) callPending(ctx context.Context, args map[string]any, fn gwCall) *gateway.Result {
	res := fn(ctx, callArgs(args))
	deadline := time.Now().Add(90 * time.Second)
	for res != nil && res.Status == gateway.StatusPending && time.Now().Before(deadline) {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			return res
		case <-time.After(400 * time.Millisecond):
		}
		c := callArgs(args)
		c.ApprovalID = res.ApprovalID
		res = fn(ctx, c)
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
