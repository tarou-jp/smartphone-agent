package deskapi

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/screen"
	"github.com/DIYgod/pinky/internal/secret"
)

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	m := s.mux

	m.HandleFunc("GET /", s.serveAssets)
	s.route("GET /api/bootstrap", s.bootstrap)
	s.route("GET /api/state", func(*http.Request) (any, error) { return s.state(), nil })
	m.HandleFunc("GET /api/events", s.events)
	s.route("POST /api/quit", s.quit)
	s.route("POST /api/open", func(r *http.Request) (any, error) {
		var b struct{ Path string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, s.openKnown(b.Path)
	})

	// Phones.
	s.route("POST /api/stop-all", func(*http.Request) (any, error) { return ok, s.api.StopAll() })
	s.route("POST /api/devices/{id}/stop", s.deviceAction(s.api.Stop))
	s.route("POST /api/devices/{id}/unlock", s.deviceAction(s.api.Unlock))
	s.route("POST /api/devices/{id}/takeover", s.deviceAction(s.api.TakeOver))
	s.route("POST /api/devices/{id}/handback", func(r *http.Request) (any, error) {
		var b struct{ Note string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, s.api.HandBack(r.PathValue("id"), b.Note)
	})
	s.route("POST /api/devices/{id}/touch", s.touch)
	s.route("POST /api/devices/{id}/key", func(r *http.Request) (any, error) {
		var b struct{ Key string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, s.api.Key(r.PathValue("id"), driver.Key(b.Key))
	})
	s.route("POST /api/devices/{id}/text", func(r *http.Request) (any, error) {
		var b struct{ Text string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, s.api.Text(r.PathValue("id"), b.Text)
	})
	m.HandleFunc("GET /api/devices/{id}/mirror", s.mirror)
	s.route("GET /api/devices/{id}/size", func(r *http.Request) (any, error) {
		id := r.PathValue("id")
		w, h, _ := s.mirrorSize(id)
		return MirrorSize{DeviceID: id, W: w, H: h}, nil
	})
	s.route("GET /api/devices/{id}/pin", s.pinState)
	s.route("PUT /api/devices/{id}/pin", s.savePIN)
	s.route("DELETE /api/devices/{id}/pin", s.removePIN)

	// Needs you.
	s.route("POST /api/needs/{id}/decide", func(r *http.Request) (any, error) {
		var b struct{ Decision, Note string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, s.api.Decide(r.PathValue("id"), gateway.Decision(b.Decision), b.Note)
	})
	m.HandleFunc("GET /api/needs/{id}/screenshot", s.needShot)

	// Activity.
	s.route("GET /api/history", s.history)
	m.HandleFunc("GET /api/media/{id}", s.media)

	// Settings.
	s.route("GET /api/settings", s.getSettings)
	s.route("PUT /api/settings", s.putSettings)

	s.extraRoutes()
	s.modelRoutes()
}

// Bootstrap is what the client needs once, at start.
type Bootstrap struct {
	Version    string            `json:"version"`
	Platform   string            `json:"platform"`
	StartPage  string            `json:"startPage,omitempty"`
	AgentToken string            `json:"agentToken,omitempty"`
	RulesFile  string            `json:"rulesFile,omitempty"`
	Changelog  string            `json:"changelog,omitempty"`
	InAppAgent bool              `json:"inAppAgent"`
	CanQuit    bool              `json:"canQuit"`
	Keychain   bool              `json:"keychain"`
	Telemetry  bool              `json:"telemetry"`
	Has        map[string]bool   `json:"has"`
	Extra      map[string]string `json:"extra,omitempty"`
}

func (s *Server) bootstrap(*http.Request) (any, error) {
	return Bootstrap{
		Version: s.opt.Version, Platform: runtime.GOOS, StartPage: s.opt.StartPage,
		AgentToken: s.opt.AgentToken, RulesFile: s.opt.RulesFile, Changelog: s.opt.Changelog,
		InAppAgent: s.opt.Agent != nil, CanQuit: s.opt.Quit != nil,
		Keychain: secret.Supported(s.secrets()), Telemetry: s.opt.Telemetry != nil,
		Has: s.capabilities(),
	}, nil
}

func (s *Server) secrets() secret.Store {
	if s.opt.Secrets != nil {
		return s.opt.Secrets
	}
	return secret.System()
}

func (s *Server) state() State {
	dev := s.currentSettings().DeveloperMode
	st := State{Devices: []Device{}, Tasks: []Task{}, Needs: []Need{}}
	for _, d := range s.api.Devices() {
		st.Devices = append(st.Devices, deviceOf(d))
	}
	for _, t := range s.api.Tasks() {
		st.Tasks = append(st.Tasks, taskOf(t, dev))
	}
	for _, n := range s.api.Needs() {
		st.Needs = append(st.Needs, needOf(n))
	}
	return st
}

func (s *Server) quit(*http.Request) (any, error) {
	if s.opt.Quit == nil {
		return nil, errUnsupported
	}
	// Answer first: quitting ends this server.
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.opt.Quit()
	}()
	return ok, nil
}

// openKnown opens only paths Pinky itself shows: the rules files and
// exports. The client cannot open anything else on the computer.
func (s *Server) openKnown(path string) error {
	allowed := map[string]bool{}
	if s.opt.RulesFile != "" {
		allowed[s.opt.RulesFile] = true
	}
	if fl, ok := s.api.(fileProblemLister); ok {
		for _, p := range fl.FileProblems() {
			allowed[p.Path] = true
		}
	}
	s.mu.Lock()
	for p := range s.pair.exports {
		allowed[p] = true
	}
	s.mu.Unlock()
	if !allowed[path] {
		return badRequest("not a path Pinky shows")
	}
	return s.open(path)
}

func (s *Server) deviceAction(fn func(id string) error) func(r *http.Request) (any, error) {
	return func(r *http.Request) (any, error) { return ok, fn(r.PathValue("id")) }
}

func (s *Server) touch(r *http.Request) (any, error) {
	// Nx and Ny are where on the screen, as fractions of its width and
	// height; X and Y are device pixels, for clients that know the size.
	var b struct {
		Action string
		X, Y   *int
		Nx, Ny *float64
	}
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	switch driver.TouchAction(b.Action) {
	case driver.TouchDown, driver.TouchMove, driver.TouchUp:
	default:
		return nil, badRequest("unknown touch action %q", b.Action)
	}
	id := r.PathValue("id")
	var p screen.Point
	switch {
	case b.Nx != nil && b.Ny != nil:
		var err error
		if p, err = s.touchPoint(id, *b.Nx, *b.Ny); err != nil {
			return nil, err
		}
	case b.X != nil && b.Y != nil:
		p = screen.Point{X: *b.X, Y: *b.Y}
	default:
		return nil, badRequest("touch needs nx and ny, or x and y")
	}
	return ok, s.api.Touch(id, driver.TouchAction(b.Action), p)
}

// PINState says whether a phone has a PIN saved, without revealing it.
type PINState struct {
	Saved     bool `json:"saved"`
	Supported bool `json:"supported"`
}

func (s *Server) pinState(r *http.Request) (any, error) {
	st := s.secrets()
	if !secret.Supported(st) {
		return PINState{}, nil
	}
	return PINState{Saved: st.Has(secret.PINAccount(r.PathValue("id"))), Supported: true}, nil
}

func (s *Server) savePIN(r *http.Request) (any, error) {
	var b struct{ PIN string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	pin := strings.TrimSpace(b.PIN)
	if pin == "" {
		return nil, badRequest("empty PIN")
	}
	if err := s.secrets().Save(secret.PINAccount(r.PathValue("id")), pin); err != nil {
		return nil, err
	}
	return PINState{Saved: true, Supported: true}, nil
}

func (s *Server) removePIN(r *http.Request) (any, error) {
	if err := s.secrets().Delete(secret.PINAccount(r.PathValue("id"))); err != nil && err != secret.ErrNotFound {
		return nil, err
	}
	return PINState{Supported: true}, nil
}

func (s *Server) needShot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var img image.Image
	for _, n := range s.api.Needs() {
		if n.ID == id {
			img = n.Screenshot
		}
	}
	if img == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_ = jpeg.Encode(w, img, &jpeg.Options{Quality: 85})
}

// HistoryPage is a page of Activity.
type HistoryPage struct {
	Tasks []Task `json:"tasks"`
	More  bool   `json:"more"`
}

func (s *Server) history(r *http.Request) (any, error) {
	q := r.URL.Query()
	hq := gateway.HistoryQuery{Search: q.Get("search"), Limit: 50}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && n <= 500 {
		hq.Limit = n
	}
	if b := q.Get("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			return nil, badRequest("before: %v", err)
		}
		hq.Before = t
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	// One more than asked says whether there are more.
	hq.Limit++
	list, err := s.api.History(ctx, hq)
	if err != nil {
		return nil, err
	}
	page := HistoryPage{Tasks: []Task{}}
	if len(list) == hq.Limit {
		page.More = true
		list = list[:hq.Limit-1]
	}
	dev := s.currentSettings().DeveloperMode
	for _, t := range list {
		page.Tasks = append(page.Tasks, taskOf(t, dev))
	}
	return page, nil
}

func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	mp, ok := s.api.(mediaPather)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	path, err := mp.MediaPath(ctx, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// hub fans gateway events and the server's own out to every SSE client.
type hub struct {
	mu   sync.Mutex
	subs map[chan sseEvent]struct{}
}

type sseEvent struct {
	name string
	data any
}

func (h *hub) init() { h.subs = map[chan sseEvent]struct{}{} }

func (h *hub) subscribe() chan sseEvent {
	c := make(chan sseEvent, 64)
	h.mu.Lock()
	h.subs[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *hub) unsubscribe(c chan sseEvent) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
}

// publish sends to every client, dropping it for one that lags: clients
// read the state again on any change, so a dropped change is caught up by
// the next.
func (h *hub) publish(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- sseEvent{name, data}:
		default:
		}
	}
}

// relay passes the gateway's events on until the server closes.
func (s *Server) relay() {
	for {
		ch := s.api.Subscribe(s.ctx)
		for ev := range ch {
			s.hub.publish("change", map[string]string{
				"kind": string(ev.Kind), "deviceId": ev.DeviceID, "taskId": ev.TaskID, "needId": ev.NeedID,
			})
		}
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// events streams Server-Sent Events: "change" when the state changed,
// "settings" when the settings did, and a comment every 15 s to keep the
// connection open.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)
	c := s.hub.subscribe()
	defer s.hub.unsubscribe(c)
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	_, _ = fmt.Fprint(w, "event: hello\ndata: {}\n\n")
	flush(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.ctx.Done():
			return
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flush(w)
		case ev := <-c:
			if err := writeSSE(w, ev.name, ev.data); err != nil {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, name string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b); err != nil {
		return err
	}
	flush(w)
	return nil
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
