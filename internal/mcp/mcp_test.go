package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/policy"
)

type harness struct {
	g   *gateway.Gateway
	dev *demo.Device
	srv *Server
}

func newHarness(t *testing.T, opts ...func(*gateway.Options)) *harness {
	t.Helper()
	dev := demo.NewDevice("pixel", "Pixel 9", driver.Android)
	pol := policy.Default()
	for cat, e := range demo.PolicyApps() {
		pol.AddApps(cat, e)
	}
	o := gateway.Options{Providers: []driver.Provider{demo.NewProvider(driver.Android, dev)}, Policy: pol,
		TrustNewAgents: true, NoScreenshots: true, SettleMax: 200 * time.Millisecond, PollInterval: 5 * time.Millisecond,
		MaxBlock: 2 * time.Second, DiscoverEvery: 20 * time.Millisecond}
	for _, f := range opts {
		f(&o)
	}
	g, err := gateway.New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return &harness{g: g, dev: dev, srv: New(g, "test")}
}

type client struct {
	cs       *sdk.ClientSession
	mu       sync.Mutex
	progress []string
	done     chan error
}

// connect starts an MCP session over the SDK's in-memory transport.
func (h *harness) connect(t *testing.T, name string) *client {
	t.Helper()
	st, ct := sdk.NewInMemoryTransports()
	c := &client{done: make(chan error, 1)}
	go func() { c.done <- h.srv.Serve(context.Background(), st) }()
	cl := sdk.NewClient(&sdk.Implementation{Name: name, Version: "1.0"}, &sdk.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *sdk.ProgressNotificationClientRequest) {
			c.mu.Lock()
			c.progress = append(c.progress, req.Params.Message)
			c.mu.Unlock()
		},
	})
	cs, err := cl.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.cs = cs
	t.Cleanup(func() { cs.Close() })
	return c
}

func (c *client) call(t *testing.T, tool string, args map[string]any) (string, bool) {
	t.Helper()
	return c.callWith(t, &sdk.CallToolParams{Name: tool, Arguments: args})
}

func (c *client) callWith(t *testing.T, p *sdk.CallToolParams) (string, bool) {
	t.Helper()
	res, err := c.cs.CallTool(context.Background(), p)
	if err != nil {
		t.Fatalf("%s: %v", p.Name, err)
	}
	var texts []string
	for _, ct := range res.Content {
		if tc, ok := ct.(*sdk.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	return strings.Join(texts, "\n"), res.IsError
}

var refRe = regexp.MustCompile(`\[(\d+)\] \w+ "([^"]*)"`)

// refOf finds the ref of a label in a result's text.
func refOf(t *testing.T, text, label string) int {
	t.Helper()
	for _, m := range refRe.FindAllStringSubmatch(text, -1) {
		if m[2] == label {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	t.Fatalf("%q not in:\n%s", label, text)
	return 0
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

func TestToolList(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	res, err := c.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"phone_connect", "phone_release", "phone_observe", "phone_tap", "phone_type", "phone_swipe", "phone_key",
		"phone_launch", "phone_apps", "phone_batch", "phone_wait_for", "phone_files", "phone_request_approval", "phone_handoff",
		"phone_flow_run", "phone_flow_save", "phone_flow_list", "phone_note"}
	got := map[string]*sdk.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	if len(got) != len(want) {
		t.Fatalf("%d tools, want %d", len(got), len(want))
	}
	for _, name := range want {
		tool := got[name]
		if tool == nil || tool.Description == "" {
			t.Fatalf("tool %s missing or undocumented", name)
		}
	}
	schema, _ := json.Marshal(got["phone_key"].InputSchema)
	for _, s := range []string{`"enum"`, `"recents"`, `"approval_id"`, `"intent"`, `"device"`} {
		if !strings.Contains(string(schema), s) {
			t.Fatalf("phone_key schema misses %s: %s", s, schema)
		}
	}
	schema, _ = json.Marshal(got["phone_tap"].InputSchema)
	if strings.Contains(string(schema), `"null"`) {
		t.Fatalf("phone_tap schema advertises null types: %s", schema)
	}
	schema, _ = json.Marshal(got["phone_batch"].InputSchema)
	if !strings.Contains(string(schema), `"maxItems":50`) || !strings.Contains(string(schema), `"launch"`) {
		t.Fatalf("phone_batch schema: %s", schema)
	}
	if c.cs.InitializeResult().Instructions == "" {
		t.Fatal("no server instructions")
	}
}

func TestFlowWithApproval(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	text, isErr := c.call(t, "phone_connect", map[string]any{"task": "Reorder last week's lunch"})
	if isErr || !strings.HasPrefix(text, "settled · Pixel 9") || !strings.Contains(text, "screen 1080x2400") {
		t.Fatalf("connect:\n%s", text)
	}
	if tasks := h.g.Tasks(); len(tasks) != 1 || tasks[0].Agent != "Claude Code" {
		t.Fatalf("agent name: %+v", tasks)
	}
	text, _ = c.call(t, "phone_launch", map[string]any{"app": "Dinely"})
	text, _ = c.call(t, "phone_tap", map[string]any{"ref": refOf(t, text, "Orders")})
	text, _ = c.call(t, "phone_tap", map[string]any{"ref": refOf(t, text, "Reorder · Chicken rice bowl")})
	if !strings.Contains(text, `toast: "Added to cart"`) || !strings.Contains(text, `+ [`) {
		t.Fatalf("diff after tap:\n%s", text)
	}
	text, _ = c.call(t, "phone_tap", map[string]any{"ref": refOf(t, text, "Cart · 1")})
	place := refOf(t, text, "Place order · $12.80")

	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if needs := h.g.Needs(); len(needs) > 0 {
				h.g.Decide(needs[0].ID, gateway.AllowOnce, "ok")
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	p := &sdk.CallToolParams{Name: "phone_tap", Arguments: map[string]any{"ref": place, "intent": "Pay $12.80 for lunch"}}
	p.SetProgressToken("tap-1")
	text, isErr = c.callWith(t, p)
	if isErr || !strings.HasPrefix(text, "settled") || !strings.Contains(text, `note: "ok"`) || !strings.Contains(text, "Order placed") {
		t.Fatalf("approved tap:\n%s", text)
	}
	// The client handles notifications on a goroutine of their own, which
	// may run after the call's result came in.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		c.mu.Lock()
		progress := append([]string(nil), c.progress...)
		c.mu.Unlock()
		if len(progress) > 0 && strings.Contains(progress[0], "Waiting for the person") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no progress notification: %q", progress)
		}
	}

	text, _ = c.call(t, "phone_release", map[string]any{"outcome": "done", "summary": "Ordered lunch."})
	if !strings.HasPrefix(text, "settled") {
		t.Fatalf("release:\n%s", text)
	}
}

func TestPendingAndErrors(t *testing.T) {
	h := newHarness(t, func(o *gateway.Options) { o.MaxBlock = 30 * time.Millisecond })
	c := h.connect(t, "codex")
	text, _ := c.call(t, "phone_connect", map[string]any{"task": "Lunch"})
	h.dev.Show("cart")
	text, _ = c.call(t, "phone_observe", nil)
	place := refOf(t, text, "Place order · $12.80")
	text, isErr := c.call(t, "phone_tap", map[string]any{"ref": place})
	if isErr || !strings.HasPrefix(text, "approval_pending · Pixel 9 · approval_id: ap_") || !strings.Contains(text, "rule: Payments") {
		t.Fatalf("pending:\n%s", text)
	}
	id := regexp.MustCompile(`approval_id: (ap_\w+)`).FindStringSubmatch(text)[1]
	h.g.Decide(id, gateway.Deny, "not today")
	text, _ = c.call(t, "phone_tap", map[string]any{"ref": place, "approval_id": id})
	if !strings.HasPrefix(text, "approval_denied") || !strings.Contains(text, `note: "not today"`) {
		t.Fatalf("denied:\n%s", text)
	}

	// A second agent gets lease_held, as an error result.
	other := h.connect(t, "cursor")
	text, isErr = other.call(t, "phone_connect", map[string]any{"task": "Something"})
	if !isErr || !strings.HasPrefix(text, "error: lease_held") || !strings.Contains(text, "Codex · Lunch") {
		t.Fatalf("lease_held:\n%s", text)
	}
	// Bad arguments are caught.
	if text, isErr = c.call(t, "phone_tap", map[string]any{"x": 3}); !isErr || !strings.Contains(text, "x and y") {
		t.Fatalf("x without y:\n%s", text)
	}
	if text, isErr = c.call(t, "phone_key", map[string]any{"key": "power"}); !isErr {
		t.Fatalf("power key accepted:\n%s", text)
	}
	if text, isErr = c.call(t, "phone_files", nil); !isErr || !strings.HasPrefix(text, "error: unsupported") {
		t.Fatalf("phone_files:\n%s", text)
	}
	// Typing into a password field is refused.
	h.dev.Show("account")
	text, _ = c.call(t, "phone_observe", nil)
	text, _ = c.call(t, "phone_type", map[string]any{"ref": refOf(t, text, "Password"), "text": "hunter2"})
	if !strings.HasPrefix(text, "blocked_by_policy") || !strings.Contains(text, "rule: credentials") {
		t.Fatalf("password:\n%s", text)
	}
}

func TestSessionCloseReleasesLease(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	c.call(t, "phone_connect", map[string]any{"task": "Lunch"})
	c.cs.Close()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after the client closed")
	}
	other := h.connect(t, "codex")
	if text, isErr := other.call(t, "phone_connect", map[string]any{"task": "Next"}); isErr {
		t.Fatalf("lease not released:\n%s", text)
	}
}

func TestBatchAndScreenshot(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	text, _ := c.call(t, "phone_connect", map[string]any{"task": "Calc"})
	text, _ = c.call(t, "phone_batch", map[string]any{"steps": []map[string]any{
		{"tool": "launch", "app": "Dinely"},
		{"tool": "phone_key", "key": "back"},
		{"tool": "key", "key": "home"},
	}})
	if !strings.HasPrefix(text, "settled") || !strings.Contains(text, "step 3: settled") {
		t.Fatalf("batch:\n%s", text)
	}
	res, err := c.cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "phone_observe", Arguments: map[string]any{"screenshot": true}})
	if err != nil {
		t.Fatal(err)
	}
	var img *sdk.ImageContent
	for _, ct := range res.Content {
		if ic, ok := ct.(*sdk.ImageContent); ok {
			img = ic
		}
	}
	if img == nil || img.MIMEType != "image/jpeg" || len(img.Data) < 100 {
		t.Fatalf("no screenshot: %+v", res.Content)
	}
}

func TestHandoffOverMCP(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	c.call(t, "phone_connect", map[string]any{"task": "Sign in"})
	go func() {
		for i := 0; i < 1000; i++ {
			for _, n := range h.g.Needs() {
				if n.Kind == gateway.NeedHandoff {
					h.g.TakeOver("pixel")
					h.g.HandBack("pixel", "signed in for you")
					return
				}
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	text, isErr := c.call(t, "phone_handoff", map[string]any{"reason": "Please sign in"})
	if isErr || !strings.HasPrefix(text, "settled") || !strings.Contains(text, "signed in for you") {
		t.Fatalf("handoff:\n%s", text)
	}
}

func TestDisplayName(t *testing.T) {
	for in, want := range map[string]string{"claude-code": "Claude Code", "codex-mcp-client": "Codex", "my-agent": "My Agent", "": "Agent"} {
		if got := DisplayName(in, ""); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := DisplayName("x-client", "X Client Pro"); got != "X Client Pro" {
		t.Errorf("title not used: %q", got)
	}
}

func TestStatusLine(t *testing.T) {
	m := true
	got := StatusLine(&gateway.Result{Status: gateway.StatusPending, DeviceName: "Pixel 9", ApprovalID: "ap_1", Rule: "Payments", Matched: &m})
	if got != "approval_pending · Pixel 9 · approval_id: ap_1 · rule: Payments · matched: true" {
		t.Fatal(got)
	}
	if got := firstLine(StatusLine(&gateway.Result{Error: gateway.ErrLeaseHeld, Holder: "Codex · Lunch"})); got != "error: lease_held · holder: Codex · Lunch" {
		t.Fatal(got)
	}
	if got := StatusLine(&gateway.Result{Status: gateway.StatusNeedsSetup, DeviceName: "iPhone 16", Setup: "trust"}); got != "device_needs_setup · iPhone 16 · setup: trust" {
		t.Fatal(got)
	}
}

// An agent does a routine once, saves it, and replays it in one call.
func TestFlowTools(t *testing.T) {
	h := newHarness(t, func(o *gateway.Options) { o.FlowPace = time.Millisecond })
	c := h.connect(t, "claude-code")
	text, _ := c.call(t, "phone_connect", map[string]any{"task": "Reorder lunch"})
	text, _ = c.call(t, "phone_launch", map[string]any{"app": "Dinely"})
	text, _ = c.call(t, "phone_tap", map[string]any{"ref": refOf(t, text, "Orders")})
	if firstLine(text) != "settled · Pixel 9" {
		t.Fatalf("tap: %s", text)
	}
	text, isErr := c.call(t, "phone_flow_save", map[string]any{"name": "dinely.orders", "description": "Open Dinely's orders"})
	if isErr || !strings.Contains(text, "flow: dinely.orders") || !strings.Contains(text, "id: dinely:tab:orders") {
		t.Fatalf("save: %s", text)
	}
	text, _ = c.call(t, "phone_flow_list", nil)
	if !strings.Contains(text, "dinely.orders · version 1 · 2 steps · com.dinely.app · Open Dinely's orders") {
		t.Fatalf("list: %s", text)
	}
	text, _ = c.call(t, "phone_flow_list", map[string]any{"name": "dinely.orders"})
	if !strings.Contains(text, "steps:") {
		t.Fatalf("show: %s", text)
	}
	c.call(t, "phone_key", map[string]any{"key": "home"})

	res, err := c.cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "phone_flow_run", Arguments: map[string]any{"flow": "dinely.orders"}})
	if err != nil {
		t.Fatal(err)
	}
	text = res.Content[0].(*sdk.TextContent).Text
	for _, want := range []string{"settled · Pixel 9", "The flow “dinely.orders” (version 1) ran: 2 steps", "step 1: ok · open com.dinely.app", "step 2: ok · tap #dinely:tab:orders", `"Reorder · Chicken rice bowl"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("run misses %q:\n%s", want, text)
		}
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out FlowOutput
	if err := json.Unmarshal(raw, &out); err != nil || out.Status != "settled" || out.Flow != "dinely.orders" || len(out.Steps) != 2 || out.Steps[1].Status != "ok" {
		t.Fatalf("structured content: %s (%v)", raw, err)
	}

	text, isErr = c.call(t, "phone_flow_run", map[string]any{"yaml": "flow: x\ntimeout_ms: 50\nsteps:\n  - tap: Checkout\n"})
	if isErr || !strings.HasPrefix(text, "mismatch · Pixel 9") || !strings.Contains(text, "step 1: mismatch · tap \"Checkout\"") || !strings.Contains(text, "from_step 1") {
		t.Fatalf("mismatch: %s", text)
	}
	if text, isErr = c.call(t, "phone_flow_run", map[string]any{"flow": "nope"}); !isErr || !strings.HasPrefix(text, "error: not_found") {
		t.Fatalf("unknown flow: %s", text)
	}
}

// The parts of an app reach an agent as lines it can pick ids from, and
// phone_flow_run takes the ids.
func TestPartsTools(t *testing.T) {
	h := newHarness(t, func(o *gateway.Options) { o.FlowPace = time.Millisecond; o.AutoFlows = true })
	c := h.connect(t, "claude-code")
	c.call(t, "phone_connect", map[string]any{"task": "Reorder lunch"})
	text, _ := c.call(t, "phone_launch", map[string]any{"app": "Dinely"})
	c.call(t, "phone_tap", map[string]any{"ref": refOf(t, text, "Orders")})
	c.call(t, "phone_key", map[string]any{"key": "home"})
	c.call(t, "phone_release", map[string]any{"outcome": "failed"})

	text, _ = c.call(t, "phone_connect", map[string]any{"task": "Show my orders", "apps": []string{"Dinely"}})
	re := regexp.MustCompile(`(?m)^(p[0-9a-f]{6}) · (open com\.dinely\.app → app com\.dinely\.app|tap tab "Orders" #dinely:tab:orders → .* · after p[0-9a-f]{6})$`)
	ids := re.FindAllStringSubmatch(text, -1)
	if len(ids) != 2 || !strings.Contains(text, "parts of com.dinely.app:") || strings.Contains(text, "press home") {
		t.Fatalf("connect: %v\n%s", ids, text)
	}
	text, isErr := c.call(t, "phone_flow_run", map[string]any{"parts": []string{ids[0][1], ids[1][1]}})
	if isErr || !strings.HasPrefix(text, "settled · Pixel 9") || !strings.Contains(text, "The plan ran") || !strings.Contains(text, "step 2: ok · tap #dinely:tab:orders") {
		t.Fatalf("run: %s", text)
	}
	if text, isErr = c.call(t, "phone_flow_run", map[string]any{"parts": []string{"pffffff"}}); !isErr || !strings.Contains(text, "pffffff") {
		t.Fatalf("an unknown part: %s", text)
	}
	text, _ = c.call(t, "phone_flow_list", map[string]any{"app": "com.dinely.app"})
	if !strings.Contains(text, "2 parts of com.dinely.app") || !strings.Contains(text, ids[1][1]) {
		t.Fatalf("list: %s", text)
	}
}

// A note an agent keeps on an app is shown to the next task in the app.
func TestNoteTool(t *testing.T) {
	h := newHarness(t)
	c := h.connect(t, "claude-code")
	c.call(t, "phone_connect", map[string]any{"task": "Look around"})
	c.call(t, "phone_launch", map[string]any{"app": "Dinely"})
	text, isErr := c.call(t, "phone_note", map[string]any{"note": "The row of tabs hides when the page scrolls."})
	if isErr || !strings.HasPrefix(text, "settled") || !strings.Contains(text, "com.dinely.app") {
		t.Fatalf("note: %s", text)
	}
	if text, isErr = c.call(t, "phone_note", map[string]any{"note": ""}); !isErr {
		t.Fatalf("an empty note: %s", text)
	}
	c.call(t, "phone_release", map[string]any{"outcome": "done"})

	text, _ = c.call(t, "phone_connect", map[string]any{"task": "Order lunch", "apps": []string{"Dinely"}})
	if !strings.Contains(text, "notes on com.dinely.app:\n  - The row of tabs hides when the page scrolls.") {
		t.Fatalf("connect: %s", text)
	}
}
