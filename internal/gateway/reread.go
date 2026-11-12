package gateway

import (
	"context"
	"strings"
	"time"

	"github.com/DIYgod/pinky/internal/screen"
)

// This file reads a screen again when its list of elements looks
// unfinished. A phone draws a new screen before its accessibility tree
// has all of it: on an iPhone, a form that had stopped moving once came
// back with its title and nothing else, and the agent took the form for
// one without fields. The pixels knew better: OCR read text where the
// tree had no element.
//
// That is the sign used here, for a tree with few elements. Text that
// only OCR reads, in a place where the tree has nothing, means either
// that the tree is behind or that the app draws the text itself (a game,
// a chart, a custom control). One more
// read tells them apart: a tree that grew was behind, and is read once
// more; a tree that did not is as it is, and the text is remembered as
// drawn, so that the same screen does not cost a read again.

const (
	// rereadWait is how long the tree gets before it is read again.
	rereadWait = 400 * time.Millisecond
	// rereadTries bounds the extra reads of one screen.
	rereadTries = 2
	// orphanLines is how many lines of text without an element make a
	// read look unfinished. One may be a logo.
	orphanLines = 2
	// sparseTree is how many elements a tree may have and still look
	// unfinished. A feed full of posts has text in its pictures that no
	// element holds, and OCR reads its icons as letters: with a tree that
	// size, such text says nothing, and reading again only costs time (a
	// swipe in such a feed took 15 s on an iPhone when it did).
	sparseTree = 24
	// drawnMax bounds the lines remembered as drawn for one task.
	drawnMax = 400
)

// orphans lists the text on a screen that only OCR reads and that lies in
// no element of the tree: neither is it the text of something the tree
// has, nor does it sit on something the tree has (a picture, a button).
// Elements that span a quarter of the screen or more do not count, being
// the page rather than a thing on it.
func orphans(sc *screen.Screen) []string {
	if sc == nil {
		return nil
	}
	quarter := sc.Width * sc.Height / 4
	var out []string
	for _, o := range sc.Elements {
		label := strings.TrimSpace(o.Label)
		if o.Source != screen.SourceOCR || len([]rune(label)) < 2 {
			continue
		}
		c := o.Bounds.Center()
		covered := false
		for _, e := range sc.Elements {
			b := e.Bounds
			if e.Source == screen.SourceOCR || b.Empty() || (quarter > 0 && b.W*b.H >= quarter) {
				continue
			}
			if c.X >= b.X && c.Y >= b.Y && c.X < b.X+b.W && c.Y < b.Y+b.H {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, label)
		}
	}
	return out
}

// treeSize counts the elements the accessibility tree gave.
func treeSize(sc *screen.Screen) int {
	n := 0
	for _, e := range sc.Elements {
		if e.Source != screen.SourceOCR {
			n++
		}
	}
	return n
}

// finishRead returns cur, or a later read of the same screen when cur's
// list of elements looks unfinished. known holds the text already known
// to be drawn by the app; drawn is what this call learned to be. read
// reads the screen again.
func finishRead(ctx context.Context, cur *screen.Screen, known map[string]bool, wait time.Duration, read func() (*screen.Screen, error)) (out *screen.Screen, drawn []string) {
	for try := 0; try < rereadTries; try++ {
		if treeSize(cur) >= sparseTree {
			return cur, nil
		}
		fresh := 0
		for _, t := range orphans(cur) {
			if !known[t] {
				fresh++
			}
		}
		if fresh < orphanLines {
			return cur, nil
		}
		if !sleepCtx(ctx, wait, nil) && ctx.Err() != nil {
			return cur, nil
		}
		again, err := read()
		if err != nil || again == nil {
			return cur, nil
		}
		grew := treeSize(again) > treeSize(cur) || appKey(again) != appKey(cur)
		cur = again
		if !grew {
			// Nothing more came: the app draws this text itself.
			return cur, orphans(cur)
		}
	}
	return cur, nil
}

// finishRead is the read an agent gets after a step or when it looks:
// read again if its elements look unfinished.
func (g *Gateway) finishRead(ctx context.Context, l *lease, cur *screen.Screen) *screen.Screen {
	if cur == nil || cur.Locked {
		return cur
	}
	app := appKey(cur)
	g.mu.Lock()
	known := map[string]bool{}
	for t := range l.drawn[app] {
		known[t] = true
	}
	g.mu.Unlock()
	out, drawn := finishRead(ctx, cur, known, rereadWait, func() (*screen.Screen, error) { return g.observe(ctx, l.dev, true) })
	if len(drawn) > 0 {
		g.mu.Lock()
		app = appKey(out)
		if l.drawn == nil {
			l.drawn = map[string]map[string]bool{}
		}
		if l.drawn[app] == nil || len(l.drawn[app])+len(drawn) > drawnMax {
			l.drawn[app] = map[string]bool{}
		}
		for _, t := range drawn {
			l.drawn[app][t] = true
		}
		g.mu.Unlock()
	}
	return out
}
