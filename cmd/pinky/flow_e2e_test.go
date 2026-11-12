//go:build !windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// `pinkyctl flow` replays a flow file on a fake phone through a real
// daemon, reports step by step, writes JUnit XML and fails on a mismatch.
func TestFlowCommand(t *testing.T) {
	bin := buildPinky(t)
	home := pinkyHome(t)
	d := dirsFor(t, home)

	daemon := exec.Command(bin, "daemon", "--fake")
	daemon.Env = env(home)
	var logs bytes.Buffer
	daemon.Stderr = &logs
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- daemon.Wait() }()
	defer func() {
		daemon.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			daemon.Process.Kill()
		}
		if t.Failed() {
			t.Logf("daemon log:\n%s", logs.String())
		}
	}()
	waitSocket(t, d)

	dir := t.TempDir()
	good := filepath.Join(dir, "orders.yaml")
	bad := filepath.Join(dir, "checkout.yaml")
	broken := filepath.Join(dir, "broken.yaml")
	write := func(path, src string) {
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(good, `
flow: dinely.reorder
params:
  dish: { default: Chicken rice bowl }
steps:
  - launch: Dinely
    expect: { app: Dinely, visible: { id: "dinely:title" } }
  - tap: { id: "dinely:tab:orders", or: [{ role: tab, label: Orders }] }
    expect: { visible: { id: "dinely:last" } }
  - tap: { role: button, contains: "Reorder · ${dish}" }
    expect: { visible: { id: "dinely:cart" } }
  - tap: Got it
    optional: true
`)
	write(bad, "flow: dinely.checkout\ntimeout_ms: 300\nsteps:\n  - launch: Dinely\n  - tap: Checkout\n  - key: back\n")
	write(broken, "flow: x\nsteps:\n  - tapp: Orders\n")

	out := run(t, bin, home, "flow", "check", good, bad)
	if strings.Count(out, ": ok") != 2 {
		t.Fatalf("check:\n%s", out)
	}
	cmd := exec.Command(bin, "flow", "check", broken)
	cmd.Env = env(home)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "tapp") {
		t.Fatalf("check of a broken flow: %v\n%s", err, out)
	}

	junit := filepath.Join(dir, "report.xml")
	out = run(t, bin, home, "flow", "run", "--device", "Pixel 9 (fake)", "--param", "dish=Beef noodles", "--junit", junit, good)
	for _, want := range []string{"ok    dinely.reorder  step 1: open Dinely", "step 3: tap button “…Reorder · Beef noodles…”", "skip  dinely.reorder  step 4", "dinely.reorder: 4 steps in"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run misses %q:\n%s", want, out)
		}
	}
	xml, err := os.ReadFile(junit)
	if err != nil || !strings.Contains(string(xml), `<testsuite name="dinely.reorder" tests="4" failures="0" skipped="1"`) {
		t.Fatalf("junit: %v\n%s", err, xml)
	}

	// A mismatch fails the command and the report, and the steps after it
	// are marked as not run.
	cmd = exec.Command(bin, "flow", "run", "--device", "Pixel 9 (fake)", "--junit", junit, bad, good)
	cmd.Env = env(home)
	raw, err := cmd.CombinedOutput()
	out = string(raw)
	if err == nil || !strings.Contains(out, "FAIL  dinely.checkout  step 2: tap \"Checkout\"") || !strings.Contains(out, "dinely.checkout: mismatch") ||
		strings.Contains(out, "dinely.reorder") {
		t.Fatalf("mismatch run: %v\n%s", err, out)
	}
	xml, _ = os.ReadFile(junit)
	if !strings.Contains(string(xml), `tests="3" failures="1" skipped="1"`) || !strings.Contains(string(xml), `<failure message="mismatch">`) {
		t.Fatalf("junit after a mismatch:\n%s", xml)
	}

	// The phone is free again, and nothing was saved by running files.
	out = run(t, bin, home, "devices")
	if !strings.Contains(out, "Pixel 9 (fake)") || strings.Contains(out, "Pinky Flow") {
		t.Fatalf("devices after the runs:\n%s", out)
	}
	if out = run(t, bin, home, "flow", "list", "--fake"); !strings.Contains(out, "No flows") {
		t.Fatalf("list:\n%s", out)
	}
	// The runs were test runs: they taught Pinky no parts either.
	if out = run(t, bin, home, "flow", "parts", "--fake"); !strings.Contains(out, "No parts") {
		t.Fatalf("parts:\n%s", out)
	}
	cmd = exec.Command(bin, "flow", "parts", "--fake", "dinely")
	cmd.Env = env(home)
	if _, err := cmd.CombinedOutput(); err == nil {
		t.Fatal("parts of an app that has none: no error")
	}
}
