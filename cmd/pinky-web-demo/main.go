// Command pinky-web-demo serves the web client (ui/frontend) against the
// scripted fake gateway, at a fixed address, for working on the front end
// and for its tests. Nothing here touches a real device.
//
//	go run ./cmd/pinky-web-demo                  # one phone, at http://127.0.0.1:7466
//	go run ./cmd/pinky-web-demo -scenario three  # three phones
//	go run ./cmd/pinky-web-demo -library <dir>   # with the Flows page, from <dir>/flows
//	cd ui/frontend && npm run dev                # Vite, proxied to the demo
//	go run -tags wails,desktop,production ./cmd/pinky-web-demo -window  # in Pinky's own window (cgo)
//
// POST /demo/play?scenario=<name>, /demo/plug-android, /demo/plug-iphone
// and /demo/scan drive the fake as the MyGo demo's Demo menu does.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/deskapi"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/policy"
	"github.com/DIYgod/pinky/internal/secret"
	"github.com/DIYgod/pinky/internal/webshell"
	"github.com/DIYgod/pinky/ui/fakeapi"
	"github.com/DIYgod/pinky/ui/frontend"
)

func main() {
	var names []string
	for _, s := range fakeapi.Scenarios {
		names = append(names, string(s))
	}
	addr := flag.String("addr", "127.0.0.1:7466", "where to serve")
	scenario := flag.String("scenario", "one", "what to start with: "+strings.Join(names, ", "))
	speed := flag.Float64("speed", 1, "how fast the agents work; 2 is twice as fast")
	timeout := flag.Duration("approval-timeout", 60*time.Second, "how long an approval waits before it denies")
	dev := flag.Bool("dev", false, "start with Developer mode on")
	page := flag.String("page", "", `open "activity", "settings" or "add-phone" at start`)
	token := flag.String("token", "", "the client token; empty makes one. Tests set it to call the API")
	library := flag.String("library", "", "a Pinky data folder whose flows, parts and notes the Flows page shows; without it the demo has no Flows page")
	window := flag.Bool("window", false, "show the client in a window, as pinky does: Pinky's own in builds with the wails tag, else the browser")
	flag.Parse()

	fake := fakeapi.New(fakeapi.Options{Scenario: fakeapi.Scenario(*scenario), Speed: *speed, ApprovalTimeout: *timeout})
	var api gateway.API = fake
	if *library != "" {
		// A gateway without phones, for the flows it keeps in the folder.
		g, err := gateway.New(gateway.Options{DataDir: *library, Policy: policy.Default(), NoScreenshots: true})
		if err != nil {
			log.Fatal(err)
		}
		defer g.Close()
		api = withLibrary{API: fake, g: g}
	}
	settings := deskapi.DefaultSettings()
	settings.DeveloperMode = *dev
	srv := deskapi.New(deskapi.Options{
		API: api, Version: "0.0.0-demo", Settings: &settings, StartPage: *page,
		Secrets: &secret.Memory{}, AgentToken: "demo-0f3c9a7e51b24d6c8e2a", AgentSetup: newDemoAgents(),
		Assets: frontend.Assets(), Log: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Changelog: "## 0.0.0-demo\n\n- The web client.\n",
	})
	if *token != "" {
		srv.SetToken(*token)
	}
	if *window {
		shell := webshell.New()
		go serveDemo(fake, demoAddr(*addr))
		if err := webshell.Run(shell, srv, webshell.Options{}); err != nil {
			log.Fatal(err)
		}
		return
	}
	url, err := srv.Listen(*addr)
	if err != nil {
		log.Fatal(err)
	}
	go serveDemo(fake, demoAddr(*addr))
	fmt.Printf("Pinky web demo: %s\nDemo controls: http://%s/demo/...\n", url, demoAddr(*addr))
	<-srv.Done()
}

// withLibrary is the fake gateway with the flows a real one keeps.
type withLibrary struct {
	*fakeapi.API
	g *gateway.Gateway
}

func (w withLibrary) Library() gateway.LibraryView       { return w.g.Library() }
func (w withLibrary) SetNotes(app, text string) error    { return w.g.SetNotes(app, text) }
func (w withLibrary) RemoveFlow(name string) error       { return w.g.RemoveFlow(name) }
func (w withLibrary) RemovePart(id string) error         { return w.g.RemovePart(id) }
func (w withLibrary) PartShot(id string) ([]byte, error) { return w.g.PartShot(id) }
func (w withLibrary) AppIcon(ctx context.Context, app string) ([]byte, error) {
	return w.g.AppIcon(ctx, app)
}

// serveDemo serves the demo's controls.
func serveDemo(api *fakeapi.API, addr string) {
	demo := http.NewServeMux()
	demo.HandleFunc("POST /demo/play", func(w http.ResponseWriter, r *http.Request) {
		s := fakeapi.Scenario(r.URL.Query().Get("scenario"))
		if s == "" {
			s = api.Scenario()
		}
		api.Play(s)
	})
	demo.HandleFunc("POST /demo/plug-android", func(http.ResponseWriter, *http.Request) { api.ConnectPhone() })
	demo.HandleFunc("POST /demo/plug-iphone", func(http.ResponseWriter, *http.Request) { api.ConnectIPhone() })
	demo.HandleFunc("POST /demo/scan", func(http.ResponseWriter, *http.Request) { api.ScanPairing() })
	if err := http.ListenAndServe(addr, demo); err != nil {
		log.Print("demo controls: ", err)
	}
}

// demoAddr is the port after the client's.
func demoAddr(addr string) string {
	host, port, ok := strings.Cut(addr, ":")
	if !ok {
		return "127.0.0.1:7467"
	}
	var n int
	fmt.Sscanf(port, "%d", &n)
	return fmt.Sprintf("%s:%d", host, n+1)
}
