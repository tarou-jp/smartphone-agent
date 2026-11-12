package flow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// This file keeps a small picture of the screen each part leads to, so
// the person can see on the Flows page what a part does without reading
// its checkpoint. The gateway takes it from the task's own screenshots
// when the task ends (they are never taken of an app the rules block)
// and passes it here as JPEG; the store keeps the newest one per part in
// shots/<part id>.jpg beside the parts. Agents are never shown them.

const shotsDir = "shots"

// MaxShot bounds a part's picture, in bytes.
const MaxShot = 256 << 10

func (s *Store) shotPath(id string) string {
	return filepath.Join(s.dir, shotsDir, id+".jpg")
}

// SaveShot keeps jpeg as the picture of the part with that id, replacing
// the one it had.
func (s *Store) SaveShot(id string, jpeg []byte) error {
	if !ValidPartID(id) {
		return ErrNotFound
	}
	if len(jpeg) == 0 || len(jpeg) > MaxShot {
		return errors.New("shots: the picture is empty or too large")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		if s.memShots == nil {
			s.memShots = map[string][]byte{}
		}
		s.memShots[id] = append([]byte(nil), jpeg...)
		return nil
	}
	dir := filepath.Join(s.dir, shotsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".shot-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(jpeg); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.shotPath(id))
}

// Shot returns the picture of a part as JPEG, ErrNotFound when it has none.
func (s *Store) Shot(id string) ([]byte, error) {
	if !ValidPartID(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		if b, ok := s.memShots[id]; ok {
			return b, nil
		}
		return nil, ErrNotFound
	}
	b, err := os.ReadFile(s.shotPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

// ShotIDs lists the parts that have a picture.
func (s *Store) ShotIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shotIDsLocked()
}

func (s *Store) shotIDsLocked() map[string]bool {
	out := map[string]bool{}
	if s.dir == "" {
		for id := range s.memShots {
			out[id] = true
		}
		return out
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, shotsDir))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), ".jpg"); ok && !e.IsDir() && ValidPartID(id) {
			out[id] = true
		}
	}
	return out
}

// PruneShots removes the pictures of parts that are gone: removed by the
// person, or dropped when an app's parts grew past their bound.
func (s *Store) PruneShots() {
	keep := map[string]bool{}
	for _, a := range s.PartApps() {
		if ps, err := s.Parts(a); err == nil {
			for _, p := range ps.Parts {
				keep[p.ID] = true
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.shotIDsLocked() {
		if keep[id] {
			continue
		}
		if s.dir == "" {
			delete(s.memShots, id)
		} else {
			os.Remove(s.shotPath(id))
		}
	}
}
