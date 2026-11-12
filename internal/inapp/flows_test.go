package inapp

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
)

// The in-app agent reaches the flow tools like any other: the same task
// replays the second time, through the tools the model is told about.
func TestPhoneToolsFlows(t *testing.T) {
	dev := demo.NewDevice("pixel", "Pixel 9", driver.Android)
	g := testGateway(t, dev)
	open := func() *PhoneTools {
		s := g.OpenSession(gateway.AgentInfo{Name: "Pinky"})
		t.Cleanup(s.Close)
		return &PhoneTools{Sess: s}
	}
	ctx := context.Background()
	p := open()
	out := p.Call(ctx, "phone_connect", map[string]any{"task": "Open orders…", "brief": "Open the orders of Dinely"})
	if !strings.HasPrefix(out, "settled") {
		t.Fatalf("connect: %s", out)
	}
	out = p.Call(ctx, "phone_launch", map[string]any{"app": "Dinely"})
	m := regexp.MustCompile(`\[(\d+)\] tab "Orders"`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no Orders tab:\n%s", out)
	}
	ref, _ := strconv.Atoi(m[1])
	if out = p.Call(ctx, "phone_tap", map[string]any{"ref": float64(ref)}); !strings.HasPrefix(out, "settled") {
		t.Fatalf("tap: %s", out)
	}
	out = p.Call(ctx, "phone_flow_save", map[string]any{"name": "dinely.orders"})
	if !strings.HasPrefix(out, "settled") || !strings.Contains(out, "flow: dinely.orders") {
		t.Fatalf("save: %s", out)
	}
	if out = p.Call(ctx, "phone_flow_list", map[string]any{}); !strings.Contains(out, "dinely.orders · version 1 · 2 steps") {
		t.Fatalf("list: %s", out)
	}
	p.Call(ctx, "phone_key", map[string]any{"key": "home"})
	out = p.Call(ctx, "phone_flow_run", map[string]any{"flow": "dinely.orders", "params": map[string]any{}})
	if !strings.HasPrefix(out, "settled") || !strings.Contains(out, "step 2: ok") || dev.Current() != "orders" {
		t.Fatalf("run (on %s): %s", dev.Current(), out)
	}
	if out = p.Call(ctx, "phone_flow_run", map[string]any{"flow": "nope"}); !strings.HasPrefix(out, "error: not_found") {
		t.Fatalf("unknown flow: %s", out)
	}
}

// A task nobody ran before is put together from the parts an earlier task
// left behind: the model reads them in the connect result and runs the
// ones it needs in one call.
func TestPhoneToolsParts(t *testing.T) {
	dev := demo.NewDevice("pixel", "Pixel 9", driver.Android)
	g := testGatewayWith(t, func(o *gateway.Options) { o.AutoFlows = true; o.FlowPace = time.Millisecond }, dev)
	open := func() *PhoneTools {
		s := g.OpenSession(gateway.AgentInfo{Name: "Pinky"})
		t.Cleanup(s.Close)
		return &PhoneTools{Sess: s}
	}
	ctx := context.Background()
	p := open()
	p.Call(ctx, "phone_connect", map[string]any{"task": "Reorder lunch", "brief": "Put last week's lunch in the cart"})
	out := p.Call(ctx, "phone_launch", map[string]any{"app": "Dinely"})
	for _, label := range []string{`tab "Orders"`, `button "Reorder · Chicken rice bowl"`} {
		m := regexp.MustCompile(`\[(\d+)\] ` + regexp.QuoteMeta(label)).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no %s:\n%s", label, out)
		}
		ref, _ := strconv.Atoi(m[1])
		if out = p.Call(ctx, "phone_tap", map[string]any{"ref": float64(ref)}); !strings.HasPrefix(out, "settled") {
			t.Fatalf("tap: %s", out)
		}
		out = p.Call(ctx, "phone_observe", map[string]any{})
	}
	if out = p.Call(ctx, "phone_release", map[string]any{"outcome": "done"}); !strings.HasPrefix(out, "settled") {
		t.Fatalf("release: %s", out)
	}
	p.Call(ctx, "phone_connect", map[string]any{"task": "Home"})
	p.Call(ctx, "phone_key", map[string]any{"key": "home"})
	p.Call(ctx, "phone_release", map[string]any{"outcome": "done"})

	// Another request: the connect result lists the app's parts.
	p = open()
	out = p.Call(ctx, "phone_connect", map[string]any{"task": "Show orders", "brief": "Show me my orders in Dinely"})
	if !strings.Contains(out, "parts of com.dinely.app:") || !strings.Contains(out, "phone_flow_run { parts:") {
		t.Fatalf("connect lists no parts:\n%s", out)
	}
	id := func(text string) string {
		m := regexp.MustCompile(`(?m)^(p[0-9a-f]{6}) · ` + regexp.QuoteMeta(text)).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no part %q:\n%s", text, out)
		}
		return m[1]
	}
	launch, orders := id("open com.dinely.app"), id(`tap tab "Orders"`)
	out = p.Call(ctx, "phone_flow_run", map[string]any{"parts": []any{launch, orders}})
	if !strings.HasPrefix(out, "settled") || !strings.Contains(out, "The 2 parts ran") || !strings.Contains(out, "step 2: ok") || dev.Current() != "orders" {
		t.Fatalf("run (on %s): %s", dev.Current(), out)
	}
	if out = p.Call(ctx, "phone_flow_list", map[string]any{"app": "dinely"}); !strings.Contains(out, orders+" · tap tab") {
		t.Fatalf("list: %s", out)
	}
}
