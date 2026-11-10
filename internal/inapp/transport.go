package inapp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// The model APIs' base URLs. The environment overrides them, for a proxy
// or a test server.
func anthropicBase() string { return envBase("PINKY_ANTHROPIC_BASE_URL", "https://api.anthropic.com") }
func openAIBase() string    { return envBase("PINKY_OPENAI_BASE_URL", "https://api.openai.com") }

func envBase(name, def string) string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv(name)), "/"); v != "" {
		return v
	}
	return def
}

// retryPolicy is how a model call tries again after an error the server
// may get over: rate limits, overload, a dropped connection. As Codex
// does, it waits longer each time and honours Retry-After.
type retryPolicy struct {
	attempts int           // tries in all
	base     time.Duration // the first wait
	max      time.Duration // the longest wait
}

var defaultRetry = retryPolicy{attempts: 5, base: time.Second, max: 30 * time.Second}

// httpError is a model API's answer that was not a success.
type httpError struct {
	status     int
	body       string
	retryAfter time.Duration
}

func (e *httpError) Error() string {
	msg := strings.TrimSpace(e.body)
	// The APIs wrap the reason in {"error":{"message":…}}; say only that.
	var w struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(msg), &w) == nil && w.Error.Message != "" {
		msg = w.Error.Message
	}
	if len(msg) > 280 {
		msg = msg[:280] + "…"
	}
	return fmt.Sprintf("HTTP %d: %s", e.status, msg)
}

// retryable reports whether trying again may succeed.
func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var he *httpError
	if errors.As(err, &he) {
		switch he.status {
		case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests,
			http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable,
			http.StatusGatewayTimeout, 529: // Anthropic: overloaded
			return true
		}
		return false
	}
	// Anything else is the connection: refused, reset, cut mid-stream
	// before anything was read.
	return true
}

// wait is how long to wait before try n (from 1).
func (p retryPolicy) wait(n int, err error) time.Duration {
	var he *httpError
	if errors.As(err, &he) && he.retryAfter > 0 {
		return min(he.retryAfter, p.max)
	}
	d := p.base << (n - 1)
	d += time.Duration(rand.Int64N(int64(d/4) + 1)) // jitter
	return min(d, p.max)
}

// postStream posts body and returns the response once its status says it
// worked, trying again as the policy says. onRetry hears each wait.
func postStream(ctx context.Context, url string, headers map[string]string, body any, pol retryPolicy, onRetry func(n int, wait time.Duration, err error)) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var last error
	for n := 1; n <= pol.attempts; n++ {
		res, err := postOnce(ctx, url, headers, payload)
		if err == nil {
			return res, nil
		}
		last = err
		if n == pol.attempts || !retryable(err) {
			break
		}
		d := pol.wait(n, err)
		if onRetry != nil {
			onRetry(n, d, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
	return nil, last
}

// client has no overall timeout: a stream lasts as long as the model
// writes. The context ends it; idle streams are cut by streamIdle.
var client = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	ResponseHeaderTimeout: 120 * time.Second,
	IdleConnTimeout:       90 * time.Second,
	MaxIdleConnsPerHost:   4,
}}

func postOnce(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		res.Body.Close()
		he := &httpError{status: res.StatusCode, body: string(raw)}
		if s := res.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				he.retryAfter = time.Duration(secs) * time.Second
			} else if t, err := http.ParseTime(s); err == nil {
				he.retryAfter = time.Until(t)
			}
		}
		return nil, he
	}
	return res, nil
}

// streamIdle is how long a stream may stay silent before it counts as
// dropped.
const streamIdle = 90 * time.Second

// readSSE calls fn for each Server-Sent Event of r until r ends, fn
// returns errStop or an error, or nothing arrives for streamIdle.
func readSSE(parent context.Context, r io.ReadCloser, fn func(event, data string) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	// Closing the body is how a stalled read ends.
	alive := make(chan struct{}, 1)
	go func() {
		t := time.NewTimer(streamIdle)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				r.Close()
				return
			case <-alive:
				if !t.Stop() {
					<-t.C
				}
				t.Reset(streamIdle)
			case <-t.C:
				r.Close()
				return
			}
		}
	}()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	var event string
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			event = ""
			return nil
		}
		d := strings.TrimSuffix(data.String(), "\n")
		data.Reset()
		e := event
		event = ""
		return fn(e, d)
	}
	for sc.Scan() {
		select {
		case alive <- struct{}{}:
		default:
		}
		line := sc.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				if errors.Is(err, errStop) {
					return nil
				}
				return err
			}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimPrefix(line[len("data:"):], " "))
			data.WriteByte('\n')
		}
	}
	if err := sc.Err(); err != nil {
		if parent.Err() != nil {
			return parent.Err()
		}
		return &streamError{err}
	}
	if err := flush(); err != nil && !errors.Is(err, errStop) {
		return err
	}
	return nil
}

// streamError is a stream that broke off: worth another try when nothing
// of it was used yet.
type streamError struct{ err error }

func (e *streamError) Error() string { return "the model's stream broke off: " + e.err.Error() }
func (e *streamError) Unwrap() error { return e.err }

// errStop ends readSSE without an error.
var errStop = errors.New("stop")
