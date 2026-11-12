package deskapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/gateway"
)

// The Flows page: what Pinky keeps for replaying tasks (the notes on apps,
// the saved flows and the apps' parts), for the person to read and tidy.

// libraryKeeper is the extra a gateway that keeps flows has.
type libraryKeeper interface {
	Library() gateway.LibraryView
	SetNotes(app, text string) error
	RemoveFlow(name string) error
	RemovePart(id string) error
	PartShot(id string) ([]byte, error)
}

// appIconer is the extra a gateway that finds app icons has.
type appIconer interface {
	AppIcon(ctx context.Context, app string) ([]byte, error)
}

// Library is the Flows page's data.
type Library struct {
	// Dir is the folder the files are in, "" when there is none.
	Dir      string       `json:"dir,omitempty"`
	Apps     []LibraryApp `json:"apps"`
	Broken   []string     `json:"broken,omitempty"`
	MaxNotes int          `json:"maxNotes"`
}

// LibraryApp is what Pinky keeps of one app; App is "" for the flows of
// no app.
type LibraryApp struct {
	App   string        `json:"app"`
	Notes string        `json:"notes"`
	Flows []LibraryFlow `json:"flows"`
	Parts []LibraryPart `json:"parts"`
}

// LibraryFlow is a saved flow.
type LibraryFlow struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Task        string        `json:"task,omitempty"`
	Version     int           `json:"version"`
	File        string        `json:"file,omitempty"`
	Steps       []LibraryStep `json:"steps"`
}

// LibraryStep is a step of a flow.
type LibraryStep struct {
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Leads    string `json:"leads,omitempty"`
	Intent   string `json:"intent,omitempty"`
	Part     string `json:"part,omitempty"`
	Missing  bool   `json:"missing,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Shot     bool   `json:"shot,omitempty"`
}

// LibraryPart is a part of an app.
type LibraryPart struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Text   string   `json:"text"`
	Leads  string   `json:"leads,omitempty"`
	Intent string   `json:"intent,omitempty"`
	After  []string `json:"after,omitempty"`
	Seen   int      `json:"seen"`
	Last   string   `json:"last,omitempty"`
	Broken bool     `json:"broken,omitempty"`
	Flows  []string `json:"flows,omitempty"`
	Shot   bool     `json:"shot,omitempty"`
}

func libraryOf(v gateway.LibraryView) Library {
	out := Library{Dir: v.Dir, Apps: []LibraryApp{}, Broken: v.Broken, MaxNotes: flow.MaxNotes}
	for _, a := range v.Apps {
		la := LibraryApp{App: a.App, Notes: a.Notes, Flows: []LibraryFlow{}, Parts: []LibraryPart{}}
		for _, f := range a.Flows {
			lf := LibraryFlow{Name: f.Name, Description: f.Description, Task: f.Task, Version: f.Version, File: f.File, Steps: []LibraryStep{}}
			for _, st := range f.Steps {
				lf.Steps = append(lf.Steps, LibraryStep(st))
			}
			la.Flows = append(la.Flows, lf)
		}
		for _, p := range a.Parts {
			la.Parts = append(la.Parts, LibraryPart(p))
		}
		out.Apps = append(out.Apps, la)
	}
	return out
}

// libraryPaths lists the files and the folder of the library, which the
// client may open.
func (s *Server) libraryPaths() []string {
	k, ok := s.api.(libraryKeeper)
	if !ok {
		return nil
	}
	v := k.Library()
	var out []string
	if v.Dir != "" {
		out = append(out, v.Dir)
	}
	for _, a := range v.Apps {
		for _, f := range a.Flows {
			if f.File != "" {
				out = append(out, f.File)
			}
		}
	}
	return out
}

func libraryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, flow.ErrNotFound):
		return notFound("%v", err)
	}
	return badRequest("%v", err)
}

func (s *Server) libraryRoutes() {
	keeper := func() (libraryKeeper, error) {
		k, ok := s.api.(libraryKeeper)
		if !ok {
			return nil, errUnsupported
		}
		return k, nil
	}
	s.route("GET /api/library", func(*http.Request) (any, error) {
		k, err := keeper()
		if err != nil {
			return nil, err
		}
		return libraryOf(k.Library()), nil
	})
	s.route("PUT /api/library/notes", func(r *http.Request) (any, error) {
		k, err := keeper()
		if err != nil {
			return nil, err
		}
		var b struct{ App, Text string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		if err := k.SetNotes(b.App, b.Text); err != nil {
			return nil, libraryError(err)
		}
		return libraryOf(k.Library()), nil
	})
	s.route("DELETE /api/library/flows/{name}", func(r *http.Request) (any, error) {
		k, err := keeper()
		if err != nil {
			return nil, err
		}
		if err := k.RemoveFlow(r.PathValue("name")); err != nil {
			return nil, libraryError(err)
		}
		return libraryOf(k.Library()), nil
	})
	// The picture of the screen a part leads to.
	s.mux.HandleFunc("GET /api/library/parts/{id}/shot", func(w http.ResponseWriter, r *http.Request) {
		k, ok := s.api.(libraryKeeper)
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := k.PartShot(r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, max-age=300")
		_, _ = w.Write(b)
	})
	// An app's icon, PNG or JPEG.
	s.mux.HandleFunc("GET /api/apps/{app}/icon", func(w http.ResponseWriter, r *http.Request) {
		ic, ok := s.api.(appIconer)
		if !ok {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		b, err := ic.AppIcon(ctx, r.PathValue("app"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", http.DetectContentType(b))
		w.Header().Set("Cache-Control", "private, max-age=86400")
		_, _ = w.Write(b)
	})
	s.route("DELETE /api/library/parts/{id}", func(r *http.Request) (any, error) {
		k, err := keeper()
		if err != nil {
			return nil, err
		}
		if err := k.RemovePart(r.PathValue("id")); err != nil {
			return nil, libraryError(err)
		}
		return libraryOf(k.Library()), nil
	})
}
