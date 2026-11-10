// Package deskapi is the desktop client's back end for the web front end
// (ui/frontend, React): an HTTP server on this computer's loopback that the
// client's window (Wails) or a browser loads. It serves the built front end
// and a JSON API over gateway.API and the extras the gateway offers, with
// Server-Sent Events for changes and chat, and MJPEG for the mirrors.
//
// Nothing here needs cgo; the native window lives behind a build tag in
// cmd/pinky.
package deskapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/secret"
	"github.com/DIYgod/pinky/internal/telemetry"
)

// Options configure the server.
type Options struct {
	// API is the gateway the client drives.
	API gateway.API
	// Agent is the core gateway for the in-app chat agent; nil leaves the
	// agent out (chat then only talks).
	Agent *gateway.Gateway
	// Version shows in Settings › Advanced.
	Version string
	// Settings are the settings to start with; the zero value is
	// DefaultSettings. OnSettings gets each change, with the settings as
	// they were before it.
	Settings   *Settings
	OnSettings func(old, s Settings)
	// GatewaySettings reads again what the gateway keeps of Settings ›
	// Safety, as in ui.Options.
	GatewaySettings func(since int) (s Settings, version int, ok bool)
	// RulesFile is the policy rules file Settings › Advanced opens.
	RulesFile string
	// AgentToken is the bearer token remote agents present.
	AgentToken string
	// Secrets keeps PINs and model API keys; nil is the system keychain.
	Secrets secret.Store
	// AgentSetup connects agents on this computer to Pinky; nil leaves it out.
	AgentSetup AgentSetup
	// Telemetry gathers usage statistics; nil leaves the switch out.
	Telemetry *telemetry.Client
	// Changelog is CHANGELOG.md, for What's new.
	Changelog string
	// Assets is the built front end (ui/frontend/dist); nil serves a page
	// that says how to build it.
	Assets fs.FS
	// StartPage opens "activity", "settings" or "add-phone" first.
	StartPage string
	// Hidden starts without a window, as `pinky -background` does.
	Hidden bool
	// Quit quits the app; nil leaves Quit out.
	Quit func()
	// Open opens a file or folder in its app; nil uses the system's opener.
	Open func(path string) error
	Log  *slog.Logger
}

// Server is the HTTP server. Create it with New, then Listen.
type Server struct {
	opt   Options
	api   gateway.API
	log   *slog.Logger
	token string
	mux   *http.ServeMux
	ctx   context.Context
	end   context.CancelFunc

	mu       sync.Mutex
	settings Settings
	policyV  int // the policy version GatewaySettings returned last

	hub   hub
	model modelState
	pair  pairState
	ln    net.Listener
	port  int
}

// New returns a server for opt.
func New(opt Options) *Server {
	s := &Server{opt: opt, api: opt.API, log: opt.Log, policyV: -1}
	if s.log == nil {
		s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opt.Settings != nil {
		s.settings = opt.Settings.Clone()
	} else {
		s.settings = DefaultSettings()
	}
	var b [24]byte
	_, _ = rand.Read(b[:])
	s.token = hex.EncodeToString(b[:])
	s.ctx, s.end = context.WithCancel(context.Background())
	s.hub.init()
	s.routes()
	go s.relay()
	return s
}

// SetToken replaces the token, before Listen; the demo fixes it for tests.
func (s *Server) SetToken(t string) { s.token = t }

// Token is the secret a client presents, as the pinky_token cookie, the
// X-Pinky-Token header or the token query parameter.
func (s *Server) Token() string { return s.token }

// Listen starts serving on addr ("127.0.0.1:0" picks a port) and returns
// the URL a window opens: it carries the token once, and the page trades
// it for a cookie.
func (s *Server) Listen(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-s.ctx.Done()
		_ = srv.Close()
	}()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("deskapi: serve", "err", err)
		}
	}()
	return s.URL(), nil
}

// URL is the address of the client's page, with the token.
func (s *Server) URL() string {
	page := ""
	if s.opt.StartPage != "" {
		page = "#/" + s.opt.StartPage
	}
	return fmt.Sprintf("http://127.0.0.1:%d/?token=%s%s", s.port, s.token, page)
}

// Close stops the server and everything it streams.
func (s *Server) Close() { s.end() }

// Done is closed when Close was called, or the client asked to quit and
// no Quit was given.
func (s *Server) Done() <-chan struct{} { return s.ctx.Done() }

// NeedCount is the number of Needs you items, for the window's shell.
func (s *Server) NeedCount() int { return len(s.api.Needs()) }

func (s *Server) cookieName() string { return fmt.Sprintf("pinky_token_%d", s.port) }

// ServeHTTP checks where a request comes from, then routes it.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A page on another site that resolves its own name to 127.0.0.1 (DNS
	// rebinding) sends its own Host: refuse any but ours.
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return
	}
	if t := r.URL.Query().Get("token"); t != "" && s.tokenOK(t) {
		http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	if strings.HasPrefix(r.URL.Path, "/api/") && !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) tokenOK(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func (s *Server) authorized(r *http.Request) bool {
	if t := r.Header.Get("X-Pinky-Token"); t != "" {
		return s.tokenOK(t)
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return s.tokenOK(t)
	}
	if c, err := r.Cookie(s.cookieName()); err == nil {
		return s.tokenOK(c.Value)
	}
	return false
}

// Handler serves the API without the host and token checks, for tests
// and for shells that serve it in process.
func (s *Server) Handler() http.Handler { return s.mux }

// route adds a JSON endpoint: fn returns the value to send, or an error.
func (s *Server) route(pattern string, fn func(r *http.Request) (any, error)) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
}

// httpError is an error with a status code of its own.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

func badRequest(format string, a ...any) error {
	return &httpError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}

func notFound(format string, a ...any) error {
	return &httpError{http.StatusNotFound, fmt.Sprintf(format, a...)}
}

// errUnsupported says the gateway lacks what a request needs.
var errUnsupported = &httpError{http.StatusNotImplemented, "not supported by this gateway"}

func writeError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	var he *httpError
	switch {
	case errors.As(err, &he):
		code = he.code
	case errors.Is(err, secret.ErrCancelled):
		code = http.StatusConflict
	case errors.Is(err, secret.ErrNotFound):
		code = http.StatusNotFound
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if v == nil {
		v = struct{}{}
	}
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads a JSON body into v.
func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return badRequest("bad JSON: %v", err)
	}
	return nil
}

// ok is the reply of actions that return nothing.
var ok = map[string]bool{"ok": true}

func (s *Server) open(path string) error {
	if s.opt.Open != nil {
		return s.opt.Open(path)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// serveAssets serves the front end, with index.html for client routes.
func (s *Server) serveAssets(w http.ResponseWriter, r *http.Request) {
	if s.opt.Assets == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, noAssetsPage)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "" {
		if f, err := fs.Stat(s.opt.Assets, name); err == nil && !f.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				// Vite names them by content.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, s.opt.Assets, name)
			return
		}
	}
	index, err := fs.ReadFile(s.opt.Assets, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, noAssetsPage)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}

const noAssetsPage = `<!doctype html><meta charset="utf-8"><title>Pinky</title>
<body style="font:15px system-ui;padding:40px">
<h1>Pinky's window is not built</h1>
<p>Build the front end, then build Pinky again: <code>make web</code>.</p>`
