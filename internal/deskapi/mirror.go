package deskapi

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"net/http"
	"strconv"
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/screen"
	"github.com/DIYgod/pinky/internal/video"
)

// MirrorSize is a phone's screen in device pixels, as its stream last said.
type MirrorSize struct {
	DeviceID string `json:"deviceId"`
	W        int    `json:"w"`
	H        int    `json:"h"`
}

func (s *Server) setMirrorSize(id string, w, h int) {
	s.mu.Lock()
	if s.pair.sizes == nil {
		s.pair.sizes = map[string][2]int{}
	}
	changed := s.pair.sizes[id] != [2]int{w, h}
	s.pair.sizes[id] = [2]int{w, h}
	s.mu.Unlock()
	if changed {
		s.hub.publish("mirrorSize", MirrorSize{DeviceID: id, W: w, H: h})
	}
}

func (s *Server) mirrorSize(id string) (w, h int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sz, ok := s.pair.sizes[id]
	return sz[0], sz[1], ok
}

// mirror streams a phone's screen as MJPEG (multipart/x-mixed-replace),
// which an <img> shows as it arrives. Query: agent=1 for the background
// display agents work on, w and h for the box it shows in (pixels), fps
// for the most pictures a second (default 30), and read=1 for the same
// parts as application/octet-stream: the web client reads the stream with
// fetch, which WebKit refuses for multipart/x-mixed-replace, so that it
// can close the connection (an <img> in WebKit keeps it open).
func (s *Server) mirror(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	boxW, _ := strconv.Atoi(q.Get("w"))
	boxH, _ := strconv.Atoi(q.Get("h"))
	fps, _ := strconv.Atoi(q.Get("fps"))
	if fps <= 0 || fps > 60 {
		fps = 30
	}
	quality, _ := strconv.Atoi(q.Get("q"))
	if quality <= 0 || quality > 95 {
		quality = 78
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-s.ctx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	// The stream comes at the phone's own size, which touches and the
	// overlay are in; the player scales its pictures down to the box.
	var opt driver.StreamOptions
	var stream driver.Stream
	var err error
	if q.Get("agent") == "1" {
		as, ok := s.api.(agentStreamer)
		if !ok {
			http.Error(w, "no background display", http.StatusNotImplemented)
			return
		}
		stream, err = as.AgentStream(ctx, id, opt)
	} else {
		stream, err = s.api.Stream(ctx, id, opt)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	wake := make(chan struct{}, 1)
	player, err := video.NewPlayer(stream, video.PlayerOptions{OnFrame: func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		_ = stream.Close()
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer player.Close()
	if boxW > 0 && boxH > 0 {
		player.SetSize(boxW, boxH)
	}

	const boundary = "pinkyframe"
	if q.Get("read") == "1" {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flush(w)

	var buf bytes.Buffer
	every := time.Second / time.Duration(fps)
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-player.Done():
			return
		case <-wake:
		}
		if wait := every - time.Since(last); wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
		f := player.Take()
		if f == nil {
			continue
		}
		last = time.Now()
		s.setMirrorSize(id, f.Width, f.Height)
		buf.Reset()
		if err := jpeg.Encode(&buf, f.Image, &jpeg.Options{Quality: quality}); err != nil {
			continue
		}
		if _, err := fmt.Fprintf(w, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, buf.Len()); err != nil {
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			return
		}
		if _, err := w.Write([]byte("\r\n")); err != nil {
			return
		}
		flush(w)
	}
}

// touchPoint turns a point given as a fraction of the screen into device
// pixels, from the size the phone's stream last had.
func (s *Server) touchPoint(id string, nx, ny float64) (screen.Point, error) {
	w, h, ok := s.mirrorSize(id)
	if !ok || w == 0 || h == 0 {
		return screen.Point{}, badRequest("the phone's screen size is not known yet")
	}
	px := int(nx * float64(w))
	py := int(ny * float64(h))
	return screen.Point{X: min(max(px, 0), w-1), Y: min(max(py, 0), h-1)}, nil
}
