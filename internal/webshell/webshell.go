// Package webshell shows the web client (internal/deskapi) in a window.
// Builds with the wails tag (and cgo) get a native window through Wails;
// the others, which need no cgo, open the browser.
package webshell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/deskapi"
)

// Shell shows the web client's page.
type Shell interface {
	// Run shows url and returns when the app quits.
	Run(url string, opt Options) error
	// Show brings the page forward, opening it when it is not.
	Show()
	// Quit makes Run return.
	Quit()
}

// Options configure the window.
type Options struct {
	Theme  string // "light", "dark" or "" for the system's
	Hidden bool   // start without a window; Show opens it
	// OnReady runs once the page is up.
	OnReady func()
}

// HasWindow: this build shows the web client in a window of its own (the
// wails build tag), not the browser.
var HasWindow = false

// newShell is the browser unless the wails build tag replaces it.
var newShell = func() Shell { return &browserShell{done: make(chan struct{})} }

// New returns the window for this build.
func New() Shell { return newShell() }

// Run serves the client on the loopback and shows it until quit. A hidden
// start opens the window when the first Needs you item arrives.
func Run(shell Shell, srv *deskapi.Server, opt Options) error {
	url, err := srv.Listen("127.0.0.1:0")
	if err != nil {
		return err
	}
	if opt.Theme != "" {
		url += "&theme=" + opt.Theme
	}
	if opt.Hidden {
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-srv.Done():
					return
				case <-t.C:
					if srv.NeedCount() > 0 {
						shell.Show()
						return
					}
				}
			}
		}()
	}
	return shell.Run(url, opt)
}

// browserShell opens the page in the default browser and waits for Quit.
type browserShell struct {
	mu   sync.Mutex
	url  string
	once sync.Once
	done chan struct{}
}

func (b *browserShell) Run(url string, opt Options) error {
	b.mu.Lock()
	b.url = url
	b.mu.Unlock()
	fmt.Fprintf(os.Stderr, "Pinky is at %s\n", url)
	if !opt.Hidden {
		b.Show()
	}
	if opt.OnReady != nil {
		opt.OnReady()
	}
	<-b.done
	return nil
}

func (b *browserShell) Show() {
	b.mu.Lock()
	url := b.url
	b.mu.Unlock()
	if url == "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func (b *browserShell) Quit() { b.once.Do(func() { close(b.done) }) }
