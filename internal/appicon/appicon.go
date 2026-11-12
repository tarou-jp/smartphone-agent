// Package appicon finds the icons of the apps Pinky shows by id on the
// Flows page, and keeps them on disk.
//
// An icon comes from the first source that has it: the cache, a phone
// that has the app installed (an Android phone's companion app), or the
// App Store, which Apple's lookup service answers by bundle id. The App
// Store lookup sends the app's id to Apple, and nothing else. Apps no
// source knows are remembered for a while, so the page does not ask again
// every time it draws.
package appicon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned for an app no source has an icon of.
var ErrNotFound = errors.New("appicon: no icon")

// maxIcon bounds an icon, in bytes.
const maxIcon = 512 << 10

// missFor is how long an app no source knew is not asked about again.
const missFor = 6 * time.Hour

var appRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,150}$`)

// Valid reports whether app can name an icon: an app id.
func Valid(app string) bool { return appRE.MatchString(app) && !strings.Contains(app, "..") }

// Source finds an app's icon, as PNG or JPEG. It returns an error wrapping
// ErrNotFound when it does not have the app.
type Source func(ctx context.Context, app string) ([]byte, error)

// Cache finds and keeps icons.
type Cache struct {
	dir     string // "" keeps them in memory
	sources []Source

	mu   sync.Mutex
	mem  map[string][]byte
	miss map[string]time.Time
	// one lookup per app at a time
	busy map[string]chan struct{}
}

// New returns a cache that keeps icons in dir ("" for memory) and asks the
// sources in order.
func New(dir string, sources ...Source) *Cache {
	return &Cache{dir: dir, sources: sources, mem: map[string][]byte{}, miss: map[string]time.Time{}, busy: map[string]chan struct{}{}}
}

func (c *Cache) path(app string) string { return filepath.Join(c.dir, app+".icon") }

func (c *Cache) cached(app string) ([]byte, bool) {
	if b, ok := c.mem[app]; ok {
		return b, true
	}
	if c.dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(c.path(app))
	if err != nil || len(b) == 0 {
		return nil, false
	}
	c.mem[app] = b
	return b, true
}

// Icon returns the icon of app, as PNG or JPEG.
func (c *Cache) Icon(ctx context.Context, app string) ([]byte, error) {
	if !Valid(app) {
		return nil, ErrNotFound
	}
	for {
		c.mu.Lock()
		if b, ok := c.cached(app); ok {
			c.mu.Unlock()
			return b, nil
		}
		if t, ok := c.miss[app]; ok && time.Since(t) < missFor {
			c.mu.Unlock()
			return nil, ErrNotFound
		}
		wait, busy := c.busy[app]
		if !busy {
			c.busy[app] = make(chan struct{})
			c.mu.Unlock()
			break
		}
		c.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b, err := c.find(ctx, app)
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.busy[app])
	delete(c.busy, app)
	switch {
	case err == nil:
		c.mem[app] = b
		delete(c.miss, app)
		if c.dir != "" {
			if os.MkdirAll(c.dir, 0o700) == nil {
				tmp := c.path(app) + ".tmp"
				if os.WriteFile(tmp, b, 0o600) == nil {
					os.Rename(tmp, c.path(app))
				}
			}
		}
		return b, nil
	case errors.Is(err, ErrNotFound):
		c.miss[app] = time.Now()
		return nil, ErrNotFound
	}
	return nil, err
}

// find asks the sources in order. Errors other than ErrNotFound (a phone
// that went away, no network) are reported only when no source has it, so
// the app is asked about again next time.
func (c *Cache) find(ctx context.Context, app string) ([]byte, error) {
	var other error
	for _, s := range c.sources {
		b, err := s(ctx, app)
		if err == nil && isImage(b) {
			return b, nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) && other == nil {
			other = err
		}
	}
	if other != nil {
		return nil, other
	}
	return nil, ErrNotFound
}

func isImage(b []byte) bool {
	return len(b) > 0 && len(b) <= maxIcon &&
		(bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) || bytes.HasPrefix(b, []byte("\xff\xd8\xff")))
}

// AppStore is a Source that looks an app up in the App Store by bundle id,
// in the stores of the computer's region and the US.
func AppStore(client *http.Client) Source {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	countries := storeCountries()
	return func(ctx context.Context, app string) ([]byte, error) {
		var other error
		for _, cc := range countries {
			b, err := appStoreIcon(ctx, client, app, cc)
			if err == nil {
				return b, nil
			}
			if !errors.Is(err, ErrNotFound) && other == nil {
				other = err
			}
		}
		if other != nil {
			return nil, other
		}
		return nil, ErrNotFound
	}
}

// lookupURL is Apple's lookup service; tests point it elsewhere.
var lookupURL = "https://itunes.apple.com/lookup"

func appStoreIcon(ctx context.Context, client *http.Client, app, country string) ([]byte, error) {
	q := url.Values{"bundleId": {app}, "country": {country}, "entity": {"software"}}
	var res struct {
		Results []struct {
			BundleID string `json:"bundleId"`
			Art512   string `json:"artworkUrl512"`
			Art100   string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if err := getJSON(ctx, client, lookupURL+"?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	for _, r := range res.Results {
		if !strings.EqualFold(r.BundleID, app) {
			continue
		}
		art := r.Art100
		if art == "" {
			art = r.Art512
		}
		// The artwork comes at any size the name asks for.
		art = strings.Replace(art, "/100x100bb.", "/192x192bb.", 1)
		if u, err := url.Parse(art); err != nil || u.Scheme != "https" {
			continue
		}
		b, err := get(ctx, client, art)
		if err != nil {
			return nil, err
		}
		if isImage(b) {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

func get(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("appicon: %s: %s", req.URL.Host, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxIcon+1))
}

func getJSON(ctx context.Context, client *http.Client, u string, v any) error {
	b, err := get(ctx, client, u)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// storeCountries are the App Store regions to look in: the computer's,
// then the US store, which has most apps.
func storeCountries() []string {
	var out []string
	if cc := systemRegion(); cc != "" && cc != "us" {
		out = append(out, cc)
	}
	return append(out, "us")
}

var regionRE = regexp.MustCompile(`^[a-z]{2,3}[_-]([A-Za-z]{2})\b`)

func systemRegion() string {
	for _, v := range []string{os.Getenv("LC_ALL"), os.Getenv("LANG")} {
		if m := regionRE.FindStringSubmatch(v); m != nil {
			return strings.ToLower(m[1])
		}
	}
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleLocale").Output(); err == nil {
			if m := regionRE.FindStringSubmatch(strings.TrimSpace(string(out))); m != nil {
				return strings.ToLower(m[1])
			}
		}
	}
	return ""
}
