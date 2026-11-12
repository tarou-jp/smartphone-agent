package inapp

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
)

// The in-app agent is shown the screen when it asks to see it, as an MCP
// agent is: phone_observe with screenshot hands back the picture with the
// text.
func TestPhoneToolsScreenshot(t *testing.T) {
	dev := demo.NewDevice("pixel", "Pixel 9", driver.Android)
	g := testGatewayWith(t, func(o *gateway.Options) { o.NoScreenshots = false }, dev)
	s := g.OpenSession(gateway.AgentInfo{Name: "Pinky"})
	t.Cleanup(s.Close)
	p := &PhoneTools{Sess: s}
	ctx := context.Background()
	p.Call(ctx, "phone_connect", map[string]any{"task": "Look"})
	out, img := p.callImage(ctx, "phone_observe", map[string]any{"screenshot": true}, nil)
	if !strings.HasPrefix(out, "settled") || !strings.HasSuffix(out, "screenshot: attached") {
		t.Fatalf("observe: %s", out)
	}
	pic, err := jpeg.Decode(bytes.NewReader(img))
	if err != nil || pic.Bounds().Dx() == 0 {
		t.Fatalf("the picture: %d bytes, %v", len(img), err)
	}
	// Without asking, no picture: screens are text.
	if out, img = p.callImage(ctx, "phone_observe", map[string]any{}, nil); img != nil || strings.Contains(out, "screenshot") {
		t.Fatalf("a picture nobody asked for: %s", out)
	}
	if got := p.Call(ctx, "phone_observe", map[string]any{"screenshot": true}); !strings.Contains(got, "screenshot: attached") {
		t.Fatalf("call: %s", got)
	}
}

func resultWithImage(id string) item {
	return item{Kind: itemResult, CallID: id, Tool: "phone_observe", Output: "settled\nscreenshot: attached", Image: []byte{0xff, 0xd8, 0xff}}
}

func TestScreenshotsReachTheModels(t *testing.T) {
	items := []item{
		{Kind: itemUser, Text: "Check the stars"},
		{Kind: itemAssistant, Calls: []toolCall{{ID: "a", Name: "phone_observe"}, {ID: "b", Name: "phone_observe"}}},
		resultWithImage("a"),
		{Kind: itemResult, CallID: "b", Tool: "phone_observe", Output: "settled"},
		{Kind: itemAssistant, Text: "Five stars."},
	}
	text := func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}

	// Anthropic: the picture is part of the tool's answer.
	got := text((&anthropicClient{}).messages(items))
	if !strings.Contains(got, `"tool_use_id":"a"`) || !strings.Contains(got, `"media_type":"image/jpeg"`) || !strings.Contains(got, `"data":"/9j/"`) {
		t.Fatalf("anthropic: %s", got)
	}

	// OpenAI: answers are text and follow each other; the picture comes
	// in a user message after the last of them.
	msgs := (&openAIClient{images: true}).messages("sys", items)
	var roles []string
	for _, m := range msgs {
		roles = append(roles, m["role"].(string))
	}
	if strings.Join(roles, " ") != "system user assistant tool tool user assistant" {
		t.Fatalf("openai roles: %v", roles)
	}
	if got := text(msgs[5]); !strings.Contains(got, "data:image/jpeg;base64,/9j/") {
		t.Fatalf("openai picture message: %s", got)
	}
	// A picture that ends the thread is sent too.
	msgs = (&openAIClient{images: true}).messages("sys", items[:3])
	if last := msgs[len(msgs)-1]; last["role"] != "user" || !strings.Contains(text(last), "image_url") {
		t.Fatalf("openai, picture last: %s", text(msgs))
	}
	// A model that is not shown pictures is told so, and sent none.
	got = text((&openAIClient{}).messages("sys", items))
	if strings.Contains(got, "image_url") || !strings.Contains(got, "not shown pictures") {
		t.Fatalf("a model without pictures: %s", got)
	}
}

// Only the latest pictures stay in what the model sees: they are large,
// and show screens that are gone.
func TestOldScreenshotsAreLeftOut(t *testing.T) {
	var items []item
	for _, id := range []string{"a", "b", "c", "d"} {
		items = append(items, item{Kind: itemAssistant, Calls: []toolCall{{ID: id, Name: "phone_observe"}}}, resultWithImage(id))
	}
	seen := view(items)
	var with []string
	for _, it := range seen {
		if it.Image != nil {
			with = append(with, it.CallID)
		}
	}
	if strings.Join(with, " ") != "c d" {
		t.Fatalf("pictures kept: %v", with)
	}
	if !strings.Contains(seen[1].Output, "older screenshot left out") || items[1].Image == nil {
		t.Fatalf("the oldest answer: %q; the thread itself must keep its picture", seen[1].Output)
	}
}

// A model that sends the arguments it does not use as null still taps by
// ref: null is no coordinate.
func TestNullArgumentsAreAbsent(t *testing.T) {
	if _, _, ok := xy(map[string]any{"ref": float64(7), "x": nil, "y": nil}); ok {
		t.Fatal("null x and y made a point")
	}
	if x, y, ok := xy(map[string]any{"x": float64(116), "y": float64(2268), "ref": nil}); !ok || x != 116 || y != 2268 {
		t.Fatalf("a point: %d,%d %v", x, y, ok)
	}
	dev := demo.NewDevice("pixel", "Pixel 9", driver.Android)
	g := testGateway(t, dev)
	s := g.OpenSession(gateway.AgentInfo{Name: "Pinky"})
	t.Cleanup(s.Close)
	p := &PhoneTools{Sess: s}
	ctx := context.Background()
	p.Call(ctx, "phone_connect", map[string]any{"task": "Open", "apps": nil, "brief": nil, "read_only": nil})
	out := p.Call(ctx, "phone_launch", map[string]any{"app": "Dinely", "device": nil, "intent": nil, "approval_id": nil})
	m := regexp.MustCompile(`\[(\d+)\] tab "Orders"`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("launch: %s", out)
	}
	ref, _ := strconv.Atoi(m[1])
	out = p.Call(ctx, "phone_tap", map[string]any{"ref": float64(ref), "x": nil, "y": nil, "long_press": nil, "intent": nil, "device": nil, "approval_id": nil})
	if !strings.HasPrefix(out, "settled") || dev.Current() != "orders" {
		t.Fatalf("a tap by ref with nulls for the rest (on %s): %s", dev.Current(), out)
	}
}
