package inapp

import "testing"

func TestDefaultChatModel(t *testing.T) {
	d := DefaultChatModel()
	if d.ID != "claude-sonnet-4-6" {
		t.Fatalf("default %q", d.ID)
	}
}

func TestChatModelMigration(t *testing.T) {
	m := ChatModelByID("claude-sonnet-4-5")
	if m == nil || m.ID != "claude-sonnet-4-6" {
		t.Fatalf("migrate sonnet 4.5 → 4.6, got %+v", m)
	}
}
