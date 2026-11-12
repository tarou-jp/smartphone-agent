package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/policy"
)

// testGateway is a gateway with demo phones, as the gateway's own tests
// make one.
func testGateway(t *testing.T, devs ...*demo.Device) *gateway.Gateway {
	t.Helper()
	return testGatewayWith(t, nil, devs...)
}

// testGatewayWith is testGateway with its options changed by opt.
func testGatewayWith(t *testing.T, opt func(*gateway.Options), devs ...*demo.Device) *gateway.Gateway {
	t.Helper()
	pol := policy.Default()
	for cat, e := range demo.PolicyApps() {
		pol.AddApps(cat, e)
	}
	var providers []driver.Provider
	for _, d := range devs {
		providers = append(providers, demo.NewProvider(driver.Android, d))
	}
	o := gateway.Options{
		Providers: providers, Policy: pol, TrustNewAgents: true, DataDir: t.TempDir(),
		// A short block per call: the loop must wait for the person over
		// several calls.
		ApprovalTimeout: 10 * time.Second, MaxBlock: 300 * time.Millisecond,
		SettleMax: 200 * time.Millisecond, PollInterval: 5 * time.Millisecond, DiscoverEvery: 20 * time.Millisecond, NoScreenshots: true,
	}
	if opt != nil {
		opt(&o)
	}
	g, err := gateway.New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	deadline := time.Now().Add(3 * time.Second)
	for len(g.Devices()) < len(devs) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return g
}

// dinelyAgent is a scripted model on the Messages API: it reorders lunch
// in Dinely on every phone it was given, reading refs off the screens the
// tools return, as a model would.
type dinelyAgent struct {
	mu    sync.Mutex
	steps map[string]int // device → next step
	reqs  int
}

var refRE = regexp.MustCompile(`\[(\d+)\] \w+ "([^"]*)"`)

func refOf(screen, label string) int {
	for _, m := range refRE.FindAllStringSubmatch(screen, -1) {
		if m[2] == label {
			var n int
			fmt.Sscan(m[1], &n)
			return n
		}
	}
	return 0
}

func (a *dinelyAgent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		System   []map[string]any `json:"system"`
		Tools    []any            `json:"tools"`
		Messages []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reqs++
	if len(body.Tools) == 0 {
		sse(w, anthReply("Reordered lunch on every phone.")...)
		return
	}
	// The phones, from the system prompt: "- A: device "a"".
	sys := fmt.Sprint(body.System[0]["text"])
	devs := regexp.MustCompile(`- (\S+): device "([^"]+)"`).FindAllStringSubmatch(sys, -1)
	name := map[string]string{}
	for _, d := range devs {
		name[d[1]] = d[2]
	}
	// The latest screen of each phone: its last tool result, or its part
	// of the first prompt.
	last := map[string]string{}
	for _, m := range body.Messages {
		for _, c := range m.Content {
			text := fmt.Sprint(c["text"]) + fmt.Sprint(c["content"])
			if strings.Contains(text, "[phone_connect on ") {
				for _, part := range strings.Split(text, "[phone_connect on ")[1:] {
					n, rest, _ := strings.Cut(part, "]")
					last[name[n]] = rest
				}
				continue
			}
			first, _, _ := strings.Cut(text, "\n")
			for n, id := range name {
				if strings.HasSuffix(first, "· "+n) || strings.Contains(first, "· "+n+" ·") {
					last[id] = text
				}
			}
		}
	}
	ids := make([]string, 0, len(devs))
	for _, d := range devs {
		ids = append(ids, d[2])
	}
	var calls []toolCall
	for _, id := range ids {
		screen := last[id]
		step := a.steps[id]
		a.steps[id]++
		call := func(name string, args map[string]any) {
			args["device"] = id
			calls = append(calls, toolCall{ID: fmt.Sprintf("%s-%d", id, step), Name: name, Args: args})
		}
		switch step {
		case 0:
			call("phone_launch", map[string]any{"app": "Dinely"})
		case 1:
			call("phone_tap", map[string]any{"ref": refOf(screen, "Orders")})
		case 2:
			call("phone_tap", map[string]any{"ref": refOf(screen, "Reorder · Chicken rice bowl")})
		case 3:
			call("phone_tap", map[string]any{"ref": refOf(screen, "Cart · 1")})
		case 4:
			call("phone_tap", map[string]any{"ref": refOf(screen, "Place order · $12.80"), "intent": "Pay $12.80"})
		case 5:
			call("phone_release", map[string]any{"outcome": "done", "summary": "Ordered."})
		}
	}
	sse(w, anthReply("", calls...)...)
}

func TestAgentReordersLunchOnTwoPhones(t *testing.T) {
	g := testGateway(t, demo.NewDevice("a", "A", driver.Android), demo.NewDevice("b", "B", driver.Android))
	srv := httptest.NewServer(&dinelyAgent{steps: map[string]int{}})
	defer srv.Close()
	t.Setenv("PINKY_ANTHROPIC_BASE_URL", srv.URL)

	// The person allows each payment after a moment, longer than one
	// call's block.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		seen := map[string]bool{}
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
			for _, n := range g.Needs() {
				if n.Kind == gateway.NeedApproval && !seen[n.ID] {
					seen[n.ID] = true
					go func(id string) {
						time.Sleep(700 * time.Millisecond)
						_ = g.Decide(id, gateway.AllowOnce, "")
					}(n.ID)
				}
			}
		}
	}()

	var mu sync.Mutex
	var evs []AgentEvent
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := RunAgent(ctx, AgentConfig{
		Gateway: g, Provider: Anthropic, ModelID: "m", APIKey: "k",
		Devices: []Device{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}},
		Prompt:  "Reorder last week's lunch",
	}, func(e AgentEvent) {
		mu.Lock()
		evs = append(evs, e)
		mu.Unlock()
	})
	if out.Err != nil || out.Reply != "Reordered lunch on every phone." {
		t.Fatalf("outcome: %+v", out)
	}
	ends := map[string][]string{}
	waited := 0
	for _, e := range evs {
		switch e.Kind {
		case "tool_end":
			ends[e.Tool] = append(ends[e.Tool], e.Status)
		case "tool_progress":
			waited++
		}
	}
	// Each phone: connect, launch, four taps (the last one paid, after
	// the person allowed it), release.
	if len(ends["phone_tap"]) != 8 || len(ends["phone_release"]) != 2 || len(ends["phone_connect"]) != 2 {
		t.Fatalf("tool ends: %v", ends)
	}
	for _, s := range ends["phone_tap"] {
		if s != "settled" {
			t.Errorf("a tap ended %q: %v", s, ends["phone_tap"])
		}
	}
	if waited == 0 {
		t.Error("no progress while the payments waited for the person")
	}
	if len(g.Needs()) != 0 {
		t.Errorf("needs left: %+v", g.Needs())
	}
}
