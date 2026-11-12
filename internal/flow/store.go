package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrNotFound is returned for a flow the store does not have.
var ErrNotFound = errors.New("flow not found")

// Store keeps flows as <name>.yaml files in one folder, which people may
// edit, copy and keep under version control, the apps' parts in its parts
// folder (parts.go) and the notes on apps in its notes folder (notes.go).
// Without a folder it keeps them in memory.
type Store struct {
	dir string
	mu  sync.Mutex
	mem map[string][]byte
	// memParts holds the apps' parts (parts.go) of a store in memory.
	memParts map[string][]byte
	// memNotes holds the notes on apps (notes.go) of a store in memory.
	memNotes map[string]string
	// memShots holds the parts' pictures (shots.go) of a store in memory.
	memShots map[string][]byte
}

// NewStore returns a store for the folder, made when the first flow is
// saved. An empty dir keeps flows in memory only.
func NewStore(dir string) *Store { return &Store{dir: dir, mem: map[string][]byte{}} }

// Dir is the store's folder, "" for a store in memory.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name+".yaml") }

// File is the file a flow of that name is kept in, "" for a store in
// memory.
func (s *Store) File(name string) string {
	if s.dir == "" || !ValidName(name) {
		return ""
	}
	return s.path(name)
}

func (s *Store) read(name string) ([]byte, error) {
	if s.dir == "" {
		data, ok := s.mem[name]
		if !ok {
			return nil, ErrNotFound
		}
		return data, nil
	}
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

// Load returns the flow of that name.
func (s *Store) Load(name string) (*Flow, error) {
	if !ValidName(name) {
		return nil, fmt.Errorf("%q is not a flow name: %w", name, ErrNotFound)
	}
	s.mu.Lock()
	data, err := s.read(name)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if f.Name != name {
		return nil, fmt.Errorf("flow: %s.yaml holds the flow %q; the file and the flow must have the same name", name, f.Name)
	}
	return f, nil
}

// Save writes the flow under its name and returns it as saved. Its version
// is one more than the version it replaces, 1 for a new flow.
func (s *Store) Save(f *Flow) (*Flow, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := f.clone()
	out.Version = 1
	if old, err := s.read(f.Name); err == nil {
		// A file somebody broke by hand is replaced like any other.
		if prev, err := Parse(old); err == nil && prev.Version >= 1 {
			out.Version = prev.Version + 1
		}
	}
	data, err := out.Marshal()
	if err != nil {
		return nil, err
	}
	if s.dir == "" {
		s.mem[f.Name] = data
		return out, nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(s.dir, ".flow-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), s.path(f.Name)); err != nil {
		return nil, err
	}
	return out, nil
}

// List returns every flow that parses, sorted by name, and the names of
// the files that do not.
func (s *Store) List() (flows []*Flow, broken []string) {
	s.mu.Lock()
	var names []string
	if s.dir == "" {
		for n := range s.mem {
			names = append(names, n)
		}
	} else if entries, err := os.ReadDir(s.dir); err == nil {
		for _, e := range entries {
			if n, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && !e.IsDir() && ValidName(n) {
				names = append(names, n)
			}
		}
	}
	s.mu.Unlock()
	sort.Strings(names)
	for _, n := range names {
		if f, err := s.Load(n); err == nil {
			flows = append(flows, f)
		} else {
			broken = append(broken, n)
		}
	}
	return flows, broken
}
