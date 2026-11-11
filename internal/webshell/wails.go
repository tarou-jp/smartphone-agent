//go:build wails

// The native window for the web client, through Wails, which needs cgo:
// build with `make web-app` (go build -tags wails,desktop,production).
// The window's page only forwards to the client's loopback server, so the
// front end talks HTTP alike in this window and in a browser.

package webshell

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func init() {
	HasWindow = true
	newShell = func() Shell { return &wailsShell{} }
}

type wailsShell struct {
	mu      sync.Mutex
	ctx     context.Context
	quit    bool // Quit came before the window was up
	show    bool // Show came before the window was up
	readyFn sync.Once
}

func (w *wailsShell) Run(url string, opt Options) error {
	forward := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		u := html.EscapeString(url)
		fmt.Fprintf(rw, `<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=%s">`+
			`<script>location.replace(%q)</script>`, u, url)
	})
	theme := mac.DefaultAppearance
	switch opt.Theme {
	case "light":
		theme = mac.NSAppearanceNameAqua
	case "dark":
		theme = mac.NSAppearanceNameDarkAqua
	}
	appMenu := menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu())
	return wails.Run(&options.App{
		Title:       "Pinky",
		Width:       1320,
		Height:      860,
		MinWidth:    960,
		MinHeight:   600,
		StartHidden: opt.Hidden,
		Menu:        appMenu,
		AssetServer: &assetserver.Options{Handler: forward},
		Mac: &mac.Options{
			// No title bar of its own: the page runs to the window's top
			// edge and the traffic lights sit over the sidebar, which
			// leaves room for them (ui/frontend/src/lib/window.ts). The
			// page asks for a window drag from its top strip. The strip is
			// the compact one, 38 pt (titlebar_darwin.go).
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: theme,
			About:      &mac.AboutInfo{Title: "Pinky", Message: "Let agents use your phone, with you watching."},
		},
		OnStartup: func(ctx context.Context) {
			compactTitleBar()
			w.mu.Lock()
			w.ctx = ctx
			quit, show := w.quit, w.show
			w.mu.Unlock()
			if quit {
				wruntime.Quit(ctx)
			} else if show {
				wruntime.WindowShow(ctx)
			}
		},
		OnDomReady: func(context.Context) {
			compactTitleBar()
			if opt.OnReady != nil {
				w.readyFn.Do(opt.OnReady)
			}
		},
	})
}

func (w *wailsShell) Show() {
	w.mu.Lock()
	ctx := w.ctx
	w.show = ctx == nil
	w.mu.Unlock()
	if ctx != nil {
		wruntime.WindowShow(ctx)
		wruntime.WindowUnminimise(ctx)
	}
}

func (w *wailsShell) Quit() {
	w.mu.Lock()
	ctx := w.ctx
	w.quit = ctx == nil
	w.mu.Unlock()
	if ctx != nil {
		wruntime.Quit(ctx)
	}
}
