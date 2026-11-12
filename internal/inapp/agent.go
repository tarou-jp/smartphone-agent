package inapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/mcp"
)

// The in-app agent, after Codex's harness: a thread of items the model
// works through turn by turn; each turn streams the model's words and
// runs the tool calls it asks for, side by side when they act on
// different phones; old screens are cut from what the model reads and a
// long thread is summed up; model calls try again after errors the server
// may get over; a call that waits for the person's approval waits as long
// as the approval does. It speaks to the gateway only through an agent
// session, the surface MCP agents use, so the person approves, stops and
// takes over as with any agent.

const agentMaxTurns = 80

// Device is a phone the agent works on.
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AgentConfig is one in-app agent run (same tools as MCP / Codex).
type AgentConfig struct {
	Gateway    *gateway.Gateway
	Provider   Provider
	ModelID    string
	APIKey     string
	OllamaBase string
	// DeviceID is the one phone the agent works on, when Devices is empty.
	DeviceID   string
	DeviceName string
	// Devices are the phones one agent works on together: it sees them
	// all and acts on each with the tools' device argument.
	Devices   []Device
	TaskTitle string
	Prompt    string
	// History is the chat so far, oldest first, so a follow-up keeps its
	// context. The agent sees it before Prompt.
	History    []Message
	LocaleIsJA bool
}

// AgentEvent reports progress for the UI. Text, when set, is one line for
// clients that show a plain log. Kind says what happened, for clients that
// draw the run:
//
//	tool_start, tool_progress, tool_end  a tool call (ID), its waiting
//	                                     (Text) and its result (Status, Detail)
//	text_delta                           the model's words as they stream (Delta)
//	text                                 the model's words between tool calls, whole
//	plan                                 the agent's plan (Plan)
//	retry                                a model call failed and is tried again (Text)
//	compact                              the thread was summed up (Text)
//	usage                                the run's tokens so far (Usage)
type AgentEvent struct {
	Text string `json:"text"`

	Kind   string         `json:"kind,omitempty"`
	ID     string         `json:"id,omitempty"`
	Tool   string         `json:"tool,omitempty"`
	Intent string         `json:"intent,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Status string         `json:"status,omitempty"` // tool_end: the result's status, or "error: <code>"
	Detail string         `json:"detail,omitempty"` // tool_end: the result's first sentence
	// Device is the phone a tool call acts on, when the agent has several.
	Device string     `json:"device,omitempty"`
	Delta  string     `json:"delta,omitempty"`
	Plan   []PlanStep `json:"plan,omitempty"`
	Usage  *Usage     `json:"usage,omitempty"`
}

// PlanStep is one step of the agent's plan.
type PlanStep struct {
	Step   string `json:"step"`
	Status string `json:"status"` // pending, in_progress or completed
}

// Usage counts a run's tokens.
type Usage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
}

// events turns a run's progress into AgentEvents for onEvent; it may be
// called from tool calls running side by side.
type events struct {
	mu     sync.Mutex
	on     func(AgentEvent)
	seq    int
	device map[string]string // tool call id → its phone
}

func (e *events) send(ev AgentEvent) {
	if e == nil || e.on == nil || (ev.Kind == "" && strings.TrimSpace(ev.Text) == "") {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.on(ev)
}

// text reports what the model said between tool calls.
func (e *events) text(s string) {
	if s = strings.TrimSpace(s); s != "" {
		e.send(AgentEvent{Kind: "text", Text: s})
	}
}

// start reports a tool call about to run and returns its id for end.
func (e *events) start(id, tool string, args map[string]any) string {
	if id == "" {
		e.mu.Lock()
		e.seq++
		id = fmt.Sprintf("t%d", e.seq)
		e.mu.Unlock()
	}
	shown := make(map[string]any, len(args))
	for k, v := range args {
		if k != "device" && k != "intent" && k != "approval_id" {
			shown[k] = v
		}
	}
	dev := strings.TrimSpace(str(args, "device"))
	e.mu.Lock()
	if e.device == nil {
		e.device = map[string]string{}
	}
	e.device[id] = dev
	e.mu.Unlock()
	e.send(AgentEvent{Kind: "tool_start", ID: id, Tool: tool, Intent: strings.TrimSpace(str(args, "intent")),
		Device: dev, Args: shown, Text: toolCallLabel(tool)})
	return id
}

// end reports a tool call's result, as FormatResult wrote it.
func (e *events) end(id, tool, result string) {
	first, rest, _ := strings.Cut(result, "\n")
	status, _, _ := strings.Cut(first, " · ")
	detail, _, _ := strings.Cut(strings.TrimSpace(rest), "\n")
	e.mu.Lock()
	dev := e.device[id]
	e.mu.Unlock()
	e.send(AgentEvent{Kind: "tool_end", ID: id, Tool: tool, Device: dev, Status: strings.TrimSpace(status), Detail: detail, Text: shortStatus(result)})
}

// AgentOutcome is the final assistant text or error.
type AgentOutcome struct {
	Reply string
	Err   error
	Usage Usage
}

// planTool is the agent's own tool for its plan, as Codex's update_plan.
var planTool = agentTool{
	Name: "update_plan",
	Description: "Keep a short plan the person sees as a checklist. Call it first for a task of more than two or three steps, " +
		"and again whenever a step starts or is done. Each step is a few words; exactly one step is in_progress at a time.",
	Schema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"plan": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"step":   map[string]any{"type": "string"},
						"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
					},
					"required": []string{"step", "status"},
				},
			},
		},
		"required": []string{"plan"},
	},
}

// phoneCaller runs one phone tool; PhoneTools is the gateway's, tests
// give their own.
type phoneCaller interface {
	call(ctx context.Context, tool string, args map[string]any, progress func(string)) string
}

// imageCaller is a phoneCaller that also hands back the picture of the
// screen a call took (phone_observe with screenshot), as JPEG.
type imageCaller interface {
	callImage(ctx context.Context, tool string, args map[string]any, progress func(string)) (string, []byte)
}

// RunAgent drives the phones through gateway tools until the model stops,
// releases every phone, or ctx ends.
func RunAgent(ctx context.Context, cfg AgentConfig, onEvent func(AgentEvent)) AgentOutcome {
	if cfg.Gateway == nil {
		return AgentOutcome{Err: fmt.Errorf("inapp: no gateway")}
	}
	devs := cfg.Devices
	if len(devs) == 0 && strings.TrimSpace(cfg.DeviceID) != "" {
		devs = []Device{{ID: strings.TrimSpace(cfg.DeviceID), Name: cfg.DeviceName}}
	}
	if len(devs) == 0 {
		return AgentOutcome{Err: fmt.Errorf("inapp: no phone selected")}
	}
	tools, err := agentTools()
	if err != nil {
		return AgentOutcome{Err: err}
	}
	mc, err := newClient(cfg.Provider, cfg.ModelID, cfg.APIKey, cfg.OllamaBase)
	if err != nil {
		return AgentOutcome{Err: err}
	}
	sess := cfg.Gateway.OpenSession(gateway.AgentInfo{Name: "Pinky", Client: "in-app-chat"})
	defer sess.Close()
	return runLoop(ctx, loopConfig{AgentConfig: cfg, devices: devs, tools: tools, model: mc, phones: &PhoneTools{Sess: sess}}, onEvent)
}

type loopConfig struct {
	AgentConfig
	devices []Device
	tools   []agentTool
	model   modelClient
	phones  phoneCaller
}

func systemPrompt(cfg loopConfig, task string) string {
	var b strings.Builder
	b.WriteString(mcp.Instructions)
	b.WriteString("\n\nYou are Pinky's built-in agent, working in Pinky's own window while the person watches the phones' screens beside the chat. ")
	if len(cfg.devices) == 1 {
		d := cfg.devices[0]
		fmt.Fprintf(&b, "You work on one phone, already connected for the task %q: %s (device %q). Pass device %q on every phone_* call.", task, d.Name, d.ID, d.ID)
	} else {
		fmt.Fprintf(&b, "You work on %d phones at once, all already connected for the task %q:\n", len(cfg.devices), task)
		for _, d := range cfg.devices {
			fmt.Fprintf(&b, "- %s: device %q\n", d.Name, d.ID)
		}
		b.WriteString("Every phone_* call must name its device. Calls for different phones in one reply run at the same time, so give each phone its next step in the same reply instead of finishing one phone before the next. A phone_release releases only its device.")
	}
	b.WriteString("\n\nWork in small steps and look at the screen before you act on it. For a task of more than a few steps, start with update_plan and keep it current. ")
	b.WriteString("When the task is done, release each phone with phone_release and finish with a short summary for the person: what you did and what you found. If you need the person, say so plainly and stop.")
	if cfg.LocaleIsJA {
		b.WriteString("\nWrite to the person in Japanese.")
	}
	return b.String()
}

func runLoop(ctx context.Context, cfg loopConfig, onEvent func(AgentEvent)) AgentOutcome {
	ev := &events{on: onEvent}
	var total usage
	report := func() {
		u := Usage(total)
		ev.send(AgentEvent{Kind: "usage", Usage: &u})
	}
	outcome := func(reply string, err error) AgentOutcome {
		report()
		return AgentOutcome{Reply: reply, Err: err, Usage: Usage(total)}
	}

	task := strings.TrimSpace(cfg.TaskTitle)
	if task == "" {
		task = taskFromPrompt(cfg.Prompt)
	}
	multi := len(cfg.devices) > 1

	// Connect every phone first, side by side.
	var wg sync.WaitGroup
	connected := make([]string, len(cfg.devices))
	for i, d := range cfg.devices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			args := map[string]any{"device": d.ID, "task": task, "brief": cfg.Prompt}
			id := ev.start("", "phone_connect", args)
			out := cfg.phones.call(ctx, "phone_connect", args, func(s string) { ev.send(AgentEvent{Kind: "tool_progress", ID: id, Device: d.ID, Text: s}) })
			ev.end(id, "phone_connect", out)
			connected[i] = out
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return outcome("", ctx.Err())
	}
	var failed []string
	for i, out := range connected {
		if strings.HasPrefix(out, "error:") {
			failed = append(failed, fmt.Sprintf("%s: %s", cfg.devices[i].Name, shortStatus(out)))
		}
	}
	if len(failed) == len(cfg.devices) {
		return outcome(strings.Join(failed, "\n"), nil)
	}

	// The thread: the chat so far, then the prompt with what connecting
	// showed, so the first turn needs no phone_observe.
	var items []item
	for _, h := range historyTurns(cfg.History) {
		k := itemUser
		if h.Role == "assistant" {
			k = itemAssistant
		}
		items = append(items, item{Kind: k, Text: h.Text})
	}
	var first strings.Builder
	first.WriteString(cfg.Prompt)
	for i, out := range connected {
		fmt.Fprintf(&first, "\n\n[phone_connect on %s]\n%s", cfg.devices[i].Name, clip(out, maxOutput))
	}
	items = append(items, item{Kind: itemUser, Text: first.String()})

	tools := append(append([]agentTool{}, cfg.tools...), planTool)
	system := systemPrompt(cfg, task)
	released := map[string]bool{}
	for i, out := range connected {
		if strings.HasPrefix(out, "error:") {
			released[cfg.devices[i].ID] = true
		}
	}
	var said []string

	for turn := 0; turn < agentMaxTurns; turn++ {
		if ctx.Err() != nil {
			return outcome("", ctx.Err())
		}
		// Keep the thread inside the model's window.
		if size(view(items)) > compactAt {
			if next, err := compact(ctx, cfg.model, system, items, ev); err == nil {
				items = next
			} else if ctx.Err() != nil {
				return outcome("", ctx.Err())
			}
		}
		res, err := cfg.model.turn(ctx, turnRequest{System: system, Tools: tools, Items: view(items)}, turnHandlers{
			text: func(d string) { ev.send(AgentEvent{Kind: "text_delta", Delta: d}) },
			retry: func(n int, wait time.Duration, err error) {
				ev.send(AgentEvent{Kind: "retry", Text: fmt.Sprintf("%v; trying again in %s", err, wait.Round(time.Second))})
			},
		})
		total.add(res.Usage)
		if err != nil {
			return outcome("", err)
		}
		items = append(items, item{Kind: itemAssistant, Text: res.Text, Calls: res.Calls, Raw: res.Raw})
		if len(res.Calls) == 0 {
			if res.Truncated && strings.TrimSpace(res.Text) == "" {
				return outcome("", errors.New("inapp: the model's reply was cut off"))
			}
			reply := strings.TrimSpace(res.Text)
			if reply == "" {
				reply = strings.Join(said, "\n\n")
			}
			if reply == "" {
				reply = "Done."
			}
			return outcome(reply, nil)
		}
		if t := strings.TrimSpace(res.Text); t != "" {
			said = append(said, t)
			ev.send(AgentEvent{Kind: "text", Text: t})
		}

		results, images := runCalls(ctx, cfg, ev, res.Calls, released)
		for i, c := range res.Calls {
			items = append(items, item{Kind: itemResult, CallID: c.ID, Tool: c.Name, Output: results[i], Image: images[i]})
		}
		if ctx.Err() != nil {
			return outcome("", ctx.Err())
		}
		if all(cfg.devices, released) {
			// Every phone is back: one more turn for the summary, without
			// tools.
			res, err := cfg.model.turn(ctx, turnRequest{System: system, Items: view(append(items, item{Kind: itemUser,
				Text: "Every phone is released. Write the person a short summary: what you did and what you found."}))}, turnHandlers{
				text: func(d string) { ev.send(AgentEvent{Kind: "text_delta", Delta: d}) },
			})
			total.add(res.Usage)
			if err == nil && strings.TrimSpace(res.Text) != "" {
				return outcome(strings.TrimSpace(res.Text), nil)
			}
			if ctx.Err() != nil {
				return outcome("", ctx.Err())
			}
			return outcome(releaseSummary(res.Calls, said, multi), nil)
		}
	}
	return outcome("Stopped after many steps. Say if you want me to continue.", nil)
}

func all(devs []Device, released map[string]bool) bool {
	for _, d := range devs {
		if !released[d.ID] {
			return false
		}
	}
	return true
}

func releaseSummary(_ []toolCall, said []string, _ bool) string {
	if len(said) > 0 {
		return said[len(said)-1]
	}
	return "Task finished."
}

// runCalls runs one turn's tool calls: calls for different phones side by
// side, calls for one phone in order. Results come back in the calls'
// order, each with the picture of the screen it took, if it took one.
func runCalls(ctx context.Context, cfg loopConfig, ev *events, calls []toolCall, released map[string]bool) ([]string, [][]byte) {
	results := make([]string, len(calls))
	images := make([][]byte, len(calls))
	byDevice := map[string][]int{}
	var order []string
	var mu sync.Mutex
	for i, c := range calls {
		if c.Args == nil {
			calls[i].Args = map[string]any{}
		}
		if c.Name == planTool.Name {
			results[i] = updatePlan(ev, calls[i])
			continue
		}
		d := strings.TrimSpace(str(calls[i].Args, "device"))
		if d == "" && len(cfg.devices) == 1 {
			d = cfg.devices[0].ID
		}
		// A phone named by its name is the same phone: one queue, one id
		// in the events.
		if d != "" {
			d = deviceID(cfg.devices, d)
			calls[i].Args["device"] = d
		}
		if _, ok := byDevice[d]; !ok {
			order = append(order, d)
		}
		byDevice[d] = append(byDevice[d], i)
	}
	var wg sync.WaitGroup
	for _, d := range order {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, i := range byDevice[d] {
				c := calls[i]
				if ctx.Err() != nil {
					results[i] = "error: cancelled · the person stopped the run"
					continue
				}
				id := ev.start(c.ID, c.Name, c.Args)
				progress := func(s string) { ev.send(AgentEvent{Kind: "tool_progress", ID: id, Device: d, Text: s}) }
				var out string
				if ic, ok := cfg.phones.(imageCaller); ok {
					out, images[i] = ic.callImage(ctx, c.Name, c.Args, progress)
				} else {
					out = cfg.phones.call(ctx, c.Name, c.Args, progress)
				}
				ev.end(id, c.Name, out)
				results[i] = out
				if c.Name == "phone_release" && !strings.HasPrefix(out, "error:") {
					mu.Lock()
					released[d] = true
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return results, images
}

// deviceID finds a phone named by id or by name.
func deviceID(devs []Device, ref string) string {
	for _, d := range devs {
		if d.ID == ref || strings.EqualFold(d.Name, ref) {
			return d.ID
		}
	}
	return ref
}

func updatePlan(ev *events, c toolCall) string {
	raw, _ := c.Args["plan"].([]any)
	var plan []PlanStep
	for _, r := range raw {
		m, _ := r.(map[string]any)
		step := strings.TrimSpace(str(m, "step"))
		if step == "" {
			continue
		}
		status := str(m, "status")
		switch status {
		case "pending", "in_progress", "completed":
		default:
			status = "pending"
		}
		plan = append(plan, PlanStep{Step: step, Status: status})
	}
	if len(plan) == 0 {
		return "error: invalid_argument · the plan has no steps"
	}
	ev.send(AgentEvent{Kind: "plan", Plan: plan})
	return "Plan updated."
}

// compact sums up the thread's older items with the model and returns the
// thread that starts from the summary.
func compact(ctx context.Context, mc modelClient, system string, items []item, ev *events) ([]item, error) {
	cut := compactCut(items)
	if cut == 0 {
		return items, errors.New("nothing to sum up")
	}
	res, err := mc.turn(ctx, turnRequest{
		System:    system,
		MaxTokens: 2000,
		Items:     []item{{Kind: itemUser, Text: compactPrompt + "\n\n" + transcript(items[:cut])}},
	}, turnHandlers{})
	if err != nil || strings.TrimSpace(res.Text) == "" {
		return items, fmt.Errorf("compact: %v", err)
	}
	ev.send(AgentEvent{Kind: "compact", Text: "Summed up the earlier steps to keep going."})
	next := []item{{Kind: itemUser, Text: "Summary of the work so far, by you:\n" + strings.TrimSpace(res.Text)}}
	// A thread goes on with the person's turn after a summary only when
	// the kept part starts with the model's.
	if items[cut].Kind == itemUser {
		next = append(next, item{Kind: itemAssistant, Text: "Understood."})
	}
	return append(next, items[cut:]...), nil
}

// historyTurns keeps the user and assistant turns of a chat with text,
// and drops a leading assistant turn, since a conversation must start with
// the person.
func historyTurns(h []Message) []Message {
	out := make([]Message, 0, len(h))
	for _, m := range h {
		if (m.Role != "user" && m.Role != "assistant") || strings.TrimSpace(m.Text) == "" {
			continue
		}
		if len(out) == 0 && m.Role != "user" {
			continue
		}
		// Two turns of one role in a row (a run that failed) merge.
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Text += "\n\n" + m.Text
			continue
		}
		out = append(out, m)
	}
	// The prompt follows as a user turn: end on the assistant's.
	if n := len(out); n > 0 && out[n-1].Role == "user" {
		out = append(out, Message{Role: "assistant", Text: "(No reply.)"})
	}
	return out
}

func taskFromPrompt(p string) string {
	return chatTitleFromPrompt(p)
}

func chatTitleFromPrompt(p string) string {
	p = strings.Join(strings.Fields(p), " ")
	r := []rune(p)
	if len(r) > 48 {
		return string(r[:47]) + "…"
	}
	if p == "" {
		return "In-app task"
	}
	return p
}

func shortStatus(toolOut string) string {
	line, _, _ := strings.Cut(toolOut, "\n")
	if len(line) > 120 {
		return line[:119] + "…"
	}
	return line
}

// TalkConfig is one reply of the chat that only talks (Ask).
type TalkConfig struct {
	Provider   Provider
	ModelID    string
	APIKey     string
	OllamaBase string
	System     string
	History    []Message
	Prompt     string
}

// Talk streams one reply without tools: text_delta events as it comes,
// retry when a call is tried again, usage at the end.
func Talk(ctx context.Context, cfg TalkConfig, onEvent func(AgentEvent)) (string, error) {
	mc, err := newClient(cfg.Provider, cfg.ModelID, cfg.APIKey, cfg.OllamaBase)
	if err != nil {
		return "", err
	}
	return talk(ctx, mc, cfg, onEvent)
}

func talk(ctx context.Context, mc modelClient, cfg TalkConfig, onEvent func(AgentEvent)) (string, error) {
	ev := &events{on: onEvent}
	var items []item
	for _, h := range historyTurns(cfg.History) {
		k := itemUser
		if h.Role == "assistant" {
			k = itemAssistant
		}
		items = append(items, item{Kind: k, Text: h.Text})
	}
	items = append(items, item{Kind: itemUser, Text: cfg.Prompt})
	res, err := mc.turn(ctx, turnRequest{System: cfg.System, Items: items}, turnHandlers{
		text: func(d string) { ev.send(AgentEvent{Kind: "text_delta", Delta: d}) },
		retry: func(n int, wait time.Duration, err error) {
			ev.send(AgentEvent{Kind: "retry", Text: fmt.Sprintf("%v; trying again in %s", err, wait.Round(time.Second))})
		},
	})
	u := Usage(res.Usage)
	ev.send(AgentEvent{Kind: "usage", Usage: &u})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res.Text) == "" {
		return "", errors.New("inapp: the model's reply was empty")
	}
	return strings.TrimSpace(res.Text), nil
}
