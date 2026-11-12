package deskapi_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/demo"
	"github.com/DIYgod/pinky/internal/deskapi"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/policy"
	"github.com/DIYgod/pinky/internal/secret"
)

// The Flows page reads what a task left behind, and the person tidies it.
func TestLibrary(t *testing.T) {
	dir := t.TempDir()
	g, err := gateway.New(gateway.Options{
		Providers: []driver.Provider{demo.NewProvider(driver.Android, demo.NewDevice("a", "A", driver.Android))},
		Policy:    policy.Default(), TrustNewAgents: true, DataDir: dir, NoScreenshots: true, AutoFlows: true,
		SettleMax: 100 * time.Millisecond, PollInterval: 5 * time.Millisecond, DiscoverEvery: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	for len(g.Devices()) < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	settings := deskapi.DefaultSettings()
	h := &harness{t: t}
	var opened []string
	h.srv = deskapi.New(deskapi.Options{API: g, Version: "test", Settings: &settings, Secrets: &secret.Memory{},
		Open: func(path string) error { opened = append(opened, path); return nil }})
	t.Cleanup(h.srv.Close)
	url, err := h.srv.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.base, h.tok = url[:strings.Index(url, "/?")], h.srv.Token()

	var boot deskapi.Bootstrap
	h.do("GET", "/api/bootstrap", "", &boot)
	if !boot.Has["library"] {
		t.Fatal("the gateway keeps flows, and the client is not told")
	}

	// A task leaves a flow, parts and a note.
	ctx := context.Background()
	s := g.OpenSession(gateway.AgentInfo{Name: "test"})
	t.Cleanup(s.Close)
	if res := s.Connect(ctx, gateway.ConnectRequest{Task: "Open the settings"}); res.IsError() {
		t.Fatalf("connect: %s", res.Message)
	}
	res := s.Act(ctx, gateway.Call{}, gateway.Action{Tool: "launch", App: "Settings"})
	if res.IsError() || res.Screen == nil {
		t.Fatalf("launch: %+v", res)
	}
	app := res.Screen.App.ID
	if res := s.Act(ctx, gateway.Call{}, gateway.Action{Tool: "key", Key: "back"}); res.IsError() {
		t.Fatalf("back: %s", res.Message)
	}
	if res := s.Note(ctx, gateway.NoteRequest{App: app, Note: "The list scrolls back to its top when a row closes."}); res.IsError() {
		t.Fatalf("note: %s", res.Message)
	}
	if res := s.Release(ctx, gateway.ReleaseRequest{Outcome: "done"}); res.IsError() {
		t.Fatalf("release: %s", res.Message)
	}

	var lib deskapi.Library
	if code := h.do("GET", "/api/library", "", &lib); code != http.StatusOK {
		t.Fatalf("GET /api/library: %d", code)
	}
	if lib.Dir != filepath.Join(dir, "flows") || len(lib.Apps) != 1 || lib.MaxNotes == 0 {
		t.Fatalf("library: %+v", lib)
	}
	a := lib.Apps[0]
	if a.App != app || !strings.Contains(a.Notes, "scrolls back to its top") || len(a.Flows) != 1 || len(a.Parts) == 0 {
		t.Fatalf("app: %+v", a)
	}
	f := a.Flows[0]
	if len(f.Steps) != 2 || f.Steps[0].Kind != "launch" || f.Steps[0].Part == "" || f.File == "" {
		t.Fatalf("flow: %+v", f)
	}
	var part deskapi.LibraryPart
	for _, p := range a.Parts {
		if p.ID == f.Steps[0].Part {
			part = p
		}
	}
	if part.Kind != "launch" || len(part.Flows) != 1 || part.Flows[0] != f.Name {
		t.Fatalf("the part of the flow's first step: %+v", part)
	}
	parts := len(a.Parts)

	// The client may open the folder and the flow's file, and nothing else.
	if code := h.do("POST", "/api/open", `{"path":"`+filepath.Join(dir, "pinky.db")+`"}`, nil); code != http.StatusBadRequest {
		t.Fatalf("opening a file that is not the library's: %d", code)
	}
	h.do("POST", "/api/open", `{"path":"`+lib.Dir+`"}`, nil)
	h.do("POST", "/api/open", `{"path":"`+f.File+`"}`, nil)
	if len(opened) != 2 || opened[0] != lib.Dir || opened[1] != f.File {
		t.Fatalf("opened: %v", opened)
	}

	// The person rewrites the notes, then removes everything.
	if code := h.do("PUT", "/api/library/notes", `{"app":"`+app+`","text":"Rows open slowly.\n"}`, &lib); code != http.StatusOK || lib.Apps[0].Notes != "Rows open slowly." {
		t.Fatalf("PUT notes: %d %q", code, lib.Apps[0].Notes)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "flows", "notes", app+".md")); err != nil || string(data) != "Rows open slowly.\n" {
		t.Fatalf("the notes file: %q %v", data, err)
	}
	if code := h.do("PUT", "/api/library/notes", `{"app":"../x","text":"y"}`, nil); code != http.StatusBadRequest {
		t.Fatalf("notes for a name that is no app: %d", code)
	}
	if code := h.do("DELETE", "/api/library/parts/"+part.ID, "", &lib); code != http.StatusOK || len(lib.Apps[0].Parts) != parts-1 || !lib.Apps[0].Flows[0].Steps[0].Missing {
		t.Fatalf("DELETE part: %d %+v", code, lib.Apps[0])
	}
	if code := h.do("DELETE", "/api/library/parts/"+part.ID, "", nil); code != http.StatusNotFound {
		t.Fatalf("DELETE a part twice: %d", code)
	}
	if code := h.do("DELETE", "/api/library/flows/"+f.Name, "", &lib); code != http.StatusOK || len(lib.Apps[0].Flows) != 0 {
		t.Fatalf("DELETE flow: %d %+v", code, lib.Apps)
	}
	if code := h.do("PUT", "/api/library/notes", `{"app":"`+app+`","text":""}`, &lib); code != http.StatusOK {
		t.Fatalf("removing the notes: %d", code)
	}
	for _, a := range lib.Apps {
		if a.Notes != "" || len(a.Flows) != 0 {
			t.Fatalf("after removing the flow and the notes: %+v", a)
		}
	}
}
