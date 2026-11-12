package gateway

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/appicon"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/screen"
)

// The Flows page shows each app with its icon. Phones that can read an
// installed app's icon (driver.AppIconer: Android with a recent companion
// app) are asked first; the App Store, by bundle id, after them. Icons are
// kept in DataDir/icons.
//
// A phone answers only while its companion app's service is on, that is
// while an agent works on it, so an app's icon is fetched in the
// background the first time a task shows the app (warmIcon), and the
// Flows page finds it in the cache later.

// iconsWarmed holds the apps warmIcon asked about since Pinky started.
var iconsWarmed sync.Map

func (g *Gateway) newIcons(dataDir string) *appicon.Cache {
	dir := ""
	if dataDir != "" {
		dir = filepath.Join(filepath.Clean(dataDir), "icons")
	}
	return appicon.New(dir, g.phoneIcon, appicon.AppStore(nil))
}

// AppIcon returns the icon of an app, as PNG or JPEG; an error wrapping
// appicon.ErrNotFound when no phone and no store has it.
func (g *Gateway) AppIcon(ctx context.Context, app string) ([]byte, error) {
	return g.icons.Icon(ctx, app)
}

// phoneIcon asks the open phones that read icons for the app's.
func (g *Gateway) phoneIcon(ctx context.Context, app string) ([]byte, error) {
	g.mu.Lock()
	var phones []driver.AppIconer
	for _, d := range g.devices {
		if ic, ok := d.dev.(driver.AppIconer); ok && d.dev != nil {
			phones = append(phones, ic)
		}
	}
	g.mu.Unlock()
	var other error
	for _, ic := range phones {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		b, err := ic.AppIcon(cctx, app)
		cancel()
		switch {
		case err == nil:
			return b, nil
		case errors.Is(err, driver.ErrNotFound), errors.Is(err, driver.ErrUnsupported):
		default:
			if other == nil {
				other = err
			}
		}
	}
	if other != nil {
		return nil, other
	}
	return nil, appicon.ErrNotFound
}

// warmIcon fetches the icon of the app on screen in the background, once,
// when the phone can read icons and the rules do not block the app.
func (g *Gateway) warmIcon(d *device, s *screen.Screen) {
	if s == nil || s.App.ID == "" {
		return
	}
	if _, blocked := g.policy.Blocked(s.App); blocked {
		return
	}
	g.mu.Lock()
	_, can := d.dev.(driver.AppIconer)
	g.mu.Unlock()
	if !can {
		return
	}
	if _, seen := iconsWarmed.LoadOrStore(s.App.ID, true); seen {
		return
	}
	app := s.App.ID
	go func() {
		ctx, cancel := context.WithTimeout(g.ctx, 30*time.Second)
		defer cancel()
		if _, err := g.icons.Icon(ctx, app); err != nil && !errors.Is(err, appicon.ErrNotFound) {
			iconsWarmed.Delete(app) // try again with a later task
		}
	}()
}
