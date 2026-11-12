package appicon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

var png = []byte("\x89PNG\r\n\x1a\n rest")

func TestCacheOrderAndMisses(t *testing.T) {
	dir := t.TempDir()
	var phone, store atomic.Int32
	fromPhone := func(ctx context.Context, app string) ([]byte, error) {
		phone.Add(1)
		if app == "com.android.only" {
			return png, nil
		}
		return nil, fmt.Errorf("phone: %w", ErrNotFound)
	}
	fromStore := func(ctx context.Context, app string) ([]byte, error) {
		store.Add(1)
		if app == "com.ios.only" {
			return png, nil
		}
		return nil, ErrNotFound
	}
	c := New(dir, fromPhone, fromStore)
	ctx := context.Background()
	if b, err := c.Icon(ctx, "com.android.only"); err != nil || string(b) != string(png) {
		t.Fatalf("android: %q %v", b, err)
	}
	if store.Load() != 0 {
		t.Error("the store was asked for an app a phone had")
	}
	if _, err := c.Icon(ctx, "com.ios.only"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "com.ios.only.icon")); err != nil {
		t.Errorf("not kept on disk: %v", err)
	}
	// Kept: a new cache on the same folder asks nobody.
	phone.Store(0)
	c2 := New(dir, fromPhone, fromStore)
	if _, err := c2.Icon(ctx, "com.ios.only"); err != nil || phone.Load() != 0 {
		t.Fatalf("from disk: %v, phone asked %d times", err, phone.Load())
	}
	// A miss is remembered.
	store.Store(0)
	for range 3 {
		if _, err := c.Icon(ctx, "com.nobody"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("nobody: %v", err)
		}
	}
	if store.Load() != 1 {
		t.Errorf("store asked %d times for a missing app", store.Load())
	}
	if _, err := c.Icon(ctx, "../etc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad id: %v", err)
	}
}

func TestAppStoreLookup(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/lookup" && r.URL.Query().Get("bundleId") == "com.example.app":
			fmt.Fprintf(w, `{"resultCount":1,"results":[{"bundleId":"com.example.app","artworkUrl100":"%s/art/100x100bb.png"}]}`, srv.URL)
		case r.URL.Path == "/lookup":
			fmt.Fprint(w, `{"resultCount":0,"results":[]}`)
		case strings.HasPrefix(r.URL.Path, "/art/192x192bb."):
			w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := lookupURL
	lookupURL = srv.URL + "/lookup"
	defer func() { lookupURL = old }()
	src := AppStore(srv.Client())
	if b, err := src(context.Background(), "com.example.app"); err != nil || string(b) != string(png) {
		t.Fatalf("lookup: %q %v", b, err)
	}
	if _, err := src(context.Background(), "com.unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}
