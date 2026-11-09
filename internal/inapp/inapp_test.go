package inapp

import "testing"

func TestKeyAccount(t *testing.T) {
	if got := KeyAccount(Anthropic, "default"); got != "llm:anthropic" {
		t.Fatalf("Anthropic account %q", got)
	}
	if got := KeyAccount(OpenAI, "work"); got != "llm:openai:work" {
		t.Fatalf("OpenAI account %q", got)
	}
}

func TestFanOutNormalize(t *testing.T) {
	f, err := (FanOut{
		Prompt:    "  Walk through onboarding  ",
		DeviceIDs: []string{" pixel ", "pixel", "iphone", ""},
		Provider:  Anthropic,
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if f.Prompt != "Walk through onboarding" || f.Agent != "Pinky" {
		t.Fatalf("prompt/agent %#v", f)
	}
	if len(f.DeviceIDs) != 2 || f.DeviceIDs[0] != "pixel" || f.DeviceIDs[1] != "iphone" {
		t.Fatalf("devices %#v", f.DeviceIDs)
	}
	if f.Title != "Walk through onboarding" {
		t.Fatalf("title %q", f.Title)
	}
}

func TestFanOutNormalizeErrors(t *testing.T) {
	if _, err := (FanOut{Provider: Anthropic, DeviceIDs: []string{"a"}}).Normalize(); err == nil {
		t.Fatal("empty prompt")
	}
	if _, err := (FanOut{Prompt: "x", Provider: Anthropic}).Normalize(); err == nil {
		t.Fatal("no devices")
	}
	if _, err := (FanOut{Prompt: "x", DeviceIDs: []string{"a"}, Provider: "nope"}).Normalize(); err == nil {
		t.Fatal("bad provider")
	}
}

func TestTaskTitle(t *testing.T) {
	if got := TaskTitle("Onboarding", "Pixel 9"); got != "Onboarding · Pixel 9" {
		t.Fatal(got)
	}
}
