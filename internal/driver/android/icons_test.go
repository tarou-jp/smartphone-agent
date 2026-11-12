package android

import (
	"context"
	"errors"
	"testing"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/driver/android/companion"
)

func TestAppIconPackageSyntax(t *testing.T) {
	for _, pkg := range []string{"com.example.app", "a.b", "com.example_1.App2", "_x.y"} {
		if !packageRE.MatchString(pkg) {
			t.Errorf("%q rejected", pkg)
		}
	}
	for _, pkg := range []string{"", "app", ".a.b", "a.b.", "a..b", "1a.b", "a.1b", "a.b c", "a.b/c", "a.b;rm", "微信.app"} {
		if packageRE.MatchString(pkg) {
			t.Errorf("%q accepted", pkg)
		}
	}
}

func TestAppIconWithoutCompanion(t *testing.T) {
	d, _ := testDevice(&fakeRunner{})
	ctx := context.Background()
	if _, err := d.AppIcon(ctx, "not a package"); !errors.Is(err, driver.ErrNotFound) {
		t.Errorf("malformed package: err = %v", err)
	}
	// The service is off: the driver does not turn it on for an icon.
	if _, err := d.AppIcon(ctx, "com.example.app"); !errors.Is(err, driver.ErrUnsupported) {
		t.Errorf("service off: err = %v", err)
	}
	// On, but an app from before app_icon.
	d.comp.on, d.comp.info = true, &companion.Info{Features: []string{companion.FeatureDisplays}}
	if _, err := d.AppIcon(ctx, "com.example.app"); !errors.Is(err, driver.ErrUnsupported) {
		t.Errorf("old app: err = %v", err)
	}
}

func TestAppIconWhileAgentWorks(t *testing.T) {
	p := newQuietPhone(t)
	d, _ := quietDevice(t, p, "")
	ctx := context.Background()
	d.AgentWorking(true)
	waitFor(t, "the service is on for the task", func() bool { return d.companionInUse() && p.listed() })
	png, err := d.AppIcon(ctx, "com.example.app")
	if err != nil || string(png) != "\x89PNG icon" {
		t.Fatalf("icon = %q, %v", png, err)
	}
	if _, err := d.AppIcon(ctx, "com.example.missing"); !errors.Is(err, driver.ErrNotFound) {
		t.Errorf("missing app: err = %v", err)
	}
	// A failed icon leaves the companion on.
	if !d.companionInUse() {
		t.Error("the companion went off after an icon")
	}
}
