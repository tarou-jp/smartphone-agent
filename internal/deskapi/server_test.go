package deskapi_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/deskapi"
	"github.com/DIYgod/pinky/internal/secret"
	"github.com/DIYgod/pinky/ui/fakeapi"
)

type harness struct {
	t    *testing.T
	base string
	tok  string
	api  *fakeapi.API
	srv  *deskapi.Server
	got  []deskapi.Settings // what OnSettings reported
}

func start(t *testing.T) *harness {
	t.Helper()
	api := fakeapi.New(fakeapi.Options{Speed: 20, ApprovalTimeout: time.Minute})
	h := &harness{t: t, api: api}
	settings := deskapi.DefaultSettings()
	h.srv = deskapi.New(deskapi.Options{
		API: api, Version: "test", Settings: &settings, Secrets: &secret.Memory{},
		OnSettings: func(_, s deskapi.Settings) { h.got = append(h.got, s) },
	})
	t.Cleanup(h.srv.Close)
	url, err := h.srv.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.base = url[:strings.Index(url, "/?")]
	h.tok = h.srv.Token()
	return h
}

func (h *harness) do(method, path, body string, out any) int {
	h.t.Helper()
	req, err := http.NewRequest(method, h.base+path, strings.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("X-Pinky-Token", h.tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if out != nil && res.StatusCode < 300 {
		if err := json.Unmarshal(data, out); err != nil {
			h.t.Fatalf("%s %s: %v: %s", method, path, err, data)
		}
	}
	return res.StatusCode
}

func TestAuth(t *testing.T) {
	h := start(t)
	res, err := http.Get(h.base + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("without a token: %d, want 401", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", h.base+"/api/state", nil)
	req.Header.Set("X-Pinky-Token", h.tok)
	req.Host = "attacker.example"
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("another host: %d, want 403", res.StatusCode)
	}
	// The page's address trades the token for a cookie.
	res, err = http.Get(h.base + "/?token=" + h.tok)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if strings.HasPrefix(c.Name, "pinky_token_") {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %+v, want an HttpOnly, SameSite=Strict token cookie", cookie)
	}
	req, _ = http.NewRequest("GET", h.base+"/api/state", nil)
	req.AddCookie(cookie)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("with the cookie: %d, want 200", res.StatusCode)
	}
}

func TestStateAndActions(t *testing.T) {
	h := start(t)
	var st deskapi.State
	if code := h.do("GET", "/api/state", "", &st); code != 200 {
		t.Fatalf("state: %d", code)
	}
	if len(st.Devices) == 0 {
		t.Fatal("no devices")
	}
	id := st.Devices[0].ID
	if code := h.do("POST", "/api/devices/"+id+"/takeover", "", nil); code != 200 {
		t.Errorf("take over: %d", code)
	}
	if code := h.do("POST", "/api/devices/"+id+"/handback", `{"note":"done"}`, nil); code != 200 {
		t.Errorf("hand back: %d", code)
	}
	if code := h.do("POST", "/api/devices/"+id+"/touch", `{"action":"tap"}`, nil); code != 400 {
		t.Errorf("bad touch: %d, want 400", code)
	}
	// Touches as fractions need the screen size, which a mirror reports.
	if code := h.do("POST", "/api/devices/"+id+"/touch", `{"action":"down","nx":0.5,"ny":0.5}`, nil); code != 400 {
		t.Errorf("touch before any picture: %d, want 400", code)
	}
	if code := h.do("POST", "/api/devices/"+id+"/touch", `{"action":"down","x":10,"y":10}`, nil); code != 200 {
		t.Errorf("touch in pixels: %d", code)
	}
	if code := h.do("POST", "/api/devices/"+id+"/touch", `{"action":"up","x":10,"y":10}`, nil); code != 200 {
		t.Errorf("touch up: %d", code)
	}
	var page deskapi.HistoryPage
	if code := h.do("GET", "/api/history?limit=2", "", &page); code != 200 {
		t.Fatalf("history: %d", code)
	}
	if len(page.Tasks) != 2 || !page.More {
		t.Errorf("history page: %d tasks, more %v; want 2 and more", len(page.Tasks), page.More)
	}
}

func TestSettingsPatch(t *testing.T) {
	h := start(t)
	var v deskapi.SettingsView
	if code := h.do("PUT", "/api/settings", `{"keepDays":90,"developerMode":true}`, &v); code != 200 {
		t.Fatalf("patch: %d", code)
	}
	if v.KeepDays != 90 || !v.DeveloperMode || v.Preset != 2 {
		t.Errorf("after patch: %+v", v)
	}
	if len(h.got) != 1 || h.got[0].KeepDays != 90 || !h.got[0].DeveloperMode {
		t.Errorf("OnSettings got %+v", h.got)
	}
	if code := h.do("PUT", "/api/settings", `{"preset":7}`, nil); code != 400 {
		t.Errorf("bad preset: %d, want 400", code)
	}
}

func TestPINNeverReturned(t *testing.T) {
	h := start(t)
	var st deskapi.PINState
	if code := h.do("PUT", "/api/devices/pixel-9/pin", `{"pin":"1234"}`, &st); code != 200 || !st.Saved {
		t.Fatalf("save PIN: %d %+v", code, st)
	}
	req, _ := http.NewRequest("GET", h.base+"/api/devices/pixel-9/pin", nil)
	req.Header.Set("X-Pinky-Token", h.tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.Contains(string(data), "1234") {
		t.Errorf("the PIN came back: %s", data)
	}
	if code := h.do("DELETE", "/api/devices/pixel-9/pin", "", &st); code != 200 || st.Saved {
		t.Errorf("remove PIN: %d %+v", code, st)
	}
}

func TestModelKeys(t *testing.T) {
	h := start(t)
	var m deskapi.ModelView
	if code := h.do("POST", "/api/model/keys", `{"provider":"anthropic","label":"Work","key":"sk-secret"}`, &m); code != 200 {
		t.Fatalf("add key: %d", code)
	}
	if len(m.Profiles) != 1 || !m.Profiles[0].HasKey || m.Active != m.Profiles[0].ID {
		t.Fatalf("profiles = %+v", m)
	}
	if m.ChatModel == "" {
		t.Error("no chat model chosen with a key")
	}
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "sk-secret") {
		t.Errorf("the key came back: %s", raw)
	}
	if code := h.do("DELETE", "/api/model/keys/"+m.Profiles[0].ID, "", &m); code != 200 || len(m.Profiles) != 0 {
		t.Errorf("remove key: %d %+v", code, m)
	}
}

func TestChatWithoutKey(t *testing.T) {
	h := start(t)
	req, _ := http.NewRequest("POST", h.base+"/api/chat", strings.NewReader(`{"prompt":"hi","locale":"en"}`))
	req.Header.Set("X-Pinky-Token", h.tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(data), "event: error") || !strings.Contains(string(data), `"code":"no_key"`) {
		t.Errorf("chat without a key: %s", data)
	}
}

func TestEvents(t *testing.T) {
	h := start(t)
	req, _ := http.NewRequest("GET", h.base+"/api/events", nil)
	req.Header.Set("X-Pinky-Token", h.tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	// The fake's agent works, so changes come soon; a settings change is
	// sent too.
	go func() {
		time.Sleep(100 * time.Millisecond)
		h.do("PUT", "/api/settings", `{"record":false}`, nil)
	}()
	seen := map[string]bool{}
	timeout := time.After(10 * time.Second)
	for !(seen["hello"] && seen["settings"] && seen["change"]) {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("stream ended; saw %v", seen)
			}
			if name, ok := strings.CutPrefix(l, "event: "); ok {
				seen[name] = true
			}
		case <-timeout:
			t.Fatalf("saw %v; want hello, settings and change", seen)
		}
	}
}
