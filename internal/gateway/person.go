package gateway

import (
	"context"
	"math"
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/flow"
	"github.com/DIYgod/pinky/internal/screen"
)

// This file lets a task learn from the person. When an agent cannot do a
// step and the person does it on the mirror in Pinky's window, the step
// is kept like one the agent made: it joins the task's trace, so it
// becomes a part of its app when the task ends (flowparts.go), and the
// next agent runs it without asking anyone.
//
// Only input that goes through Pinky can be learned. A finger on the
// phone itself is seen by no driver in a way that says what it touched,
// so such steps leave a gap in the trace instead. Text the person types
// is never kept, since it may be a password: what is learned is the field
// it went into.
//
// A step needs the screen before it and the screen it led to, and
// reading a screen takes up to seconds, during which the person may
// already have gone on. Every input of the person gets a number (pseq),
// and a read counts as the screen after input n only when no other input
// began before it ended. An input whose screens are not known that way
// is left out, and the trace is marked as having a gap: Pinky would
// rather not know a step than know it wrong.

// personTapSlop is how far, in pixels, a finger may move on the mirror
// between going down and lifting and still have tapped.
const personTapSlop = 24

// personEvent is an input of the person that waits to be recorded.
type personEvent struct {
	seq    int
	kind   string // "look", "tap", "key", "text"
	point  screen.Point
	long   bool
	key    string
	before *screen.Screen // nil when the screen before it is not known
}

// personLearnsLocked reports whether the person's input on d is worth
// recording: a task holds the phone, and it is not a test run.
func (g *Gateway) personLearnsLocked(d *device) bool {
	return g.opt.AutoFlows && d.lease != nil && !d.lease.task.test && d.lease.bg == nil
}

// personInputLocked numbers an input of the person and queues it for
// recording. kind "look" only reads the screen, for the inputs to come.
func (g *Gateway) personInputLocked(d *device, ev personEvent) {
	if !g.personLearnsLocked(d) {
		d.pview, d.pending = nil, nil
		return
	}
	if ev.kind != "look" {
		d.pseq++
		if d.pview != nil && d.pviewSeq == d.pseq-1 {
			ev.before = d.pview
		}
	}
	ev.seq = d.pseq
	d.pending = append(d.pending, ev)
	if !d.recording {
		d.recording = true
		go g.recordPerson(d)
	}
}

// personTouch follows the person's finger on the mirror and queues a tap
// when it lifts where it went down.
func (g *Gateway) personTouch(d *device, a driver.TouchAction, p screen.Point) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch a {
	case driver.TouchDown:
		d.pdown = &touchState{from: p, at: time.Now()}
	case driver.TouchUp:
		start := d.pdown
		d.pdown = nil
		if start == nil {
			return
		}
		if math.Hypot(float64(p.X-start.from.X), float64(p.Y-start.from.Y)) >= personTapSlop {
			// A drag: scrolling is no step to keep, but it changed the
			// screen the next step starts from.
			g.personInputLocked(d, personEvent{kind: "swipe"})
			return
		}
		g.personInputLocked(d, personEvent{kind: "tap", point: start.from, long: time.Since(start.at) > 500*time.Millisecond})
	}
}

// recordPerson records the person's queued inputs, one read of the
// screen at a time, until none are left.
func (g *Gateway) recordPerson(d *device) {
	for {
		g.mu.Lock()
		batch := d.pending
		d.pending = nil
		l := d.lease
		if len(batch) == 0 || l == nil {
			d.recording = false
			g.mu.Unlock()
			return
		}
		g.mu.Unlock()
		last := batch[len(batch)-1]

		ctx, cancel := context.WithTimeout(g.ctx, 20*time.Second)
		g.settle(ctx, d)
		next, err := g.observe(ctx, d, false)
		cancel()

		g.mu.Lock()
		same := d.lease == l
		clean := err == nil && next != nil && d.pseq == last.seq && same
		if clean {
			d.pview, d.pviewSeq = next, last.seq
		} else {
			d.pview = nil
		}
		g.mu.Unlock()
		if !same {
			continue
		}
		// One input, with the screen before it and the screen after it
		// known: a step. Several inputs between two reads cannot be told
		// apart, and leave a gap. Typing is the exception: the field the
		// text went into is known from the screen before the first key.
		var steps []personEvent
		for _, ev := range batch {
			if ev.kind != "look" {
				steps = append(steps, ev)
			}
		}
		traced := false
		if len(steps) == 1 && clean && steps[0].before != nil {
			traced = g.tracePerson(l, steps[0], next)
		}
		for _, ev := range steps {
			if ev.kind == "text" && ev.before != nil {
				g.learnTyping(ev.before)
				break
			}
		}
		if len(steps) > 0 && !traced {
			g.mu.Lock()
			l.task.traceGap = true
			g.mu.Unlock()
		}
	}
}

// tracePerson keeps a tap or a key of the person as a step of the task,
// and reports whether it did.
func (g *Gateway) tracePerson(l *lease, ev personEvent, after *screen.Screen) bool {
	before := ev.before
	switch ev.kind {
	case "tap":
		a := Action{Tool: "tap", LongPress: ev.long}
		p := &prep{}
		if e, ok := elementAt(before, ev.point); ok {
			if a.Ref = e.Ref; a.Ref == 0 {
				a.Ref = 1 // the step is written from the element, not from the ref
			}
			p.target = &e
		} else {
			pt := ev.point
			a.Point = &pt
		}
		g.traceStep(l, Call{}, a, p, before, after)
		return true
	case "key":
		g.traceStep(l, Call{}, Action{Tool: "key", Key: ev.key}, &prep{}, before, after)
		return true
	}
	return false
}

// learnTyping keeps the field the person typed into as a part of its app,
// and never the text, which may be a password: the next agent types its
// own text into the field. The task's trace has a gap where the text
// was, so no flow is kept of the task.
func (g *Gateway) learnTyping(before *screen.Screen) {
	f, ok := before.Focused()
	if !ok || f.Role != screen.RoleInput {
		return
	}
	if _, hidden := g.policy.Blocked(before.App); hidden {
		return
	}
	ref := f.Ref
	if ref == 0 {
		ref = 1
	}
	st, ok := flowStepOf(Call{}, Action{Tool: "type", Ref: ref}, &prep{target: &f}, before, before)
	if !ok || st.Into == nil {
		return
	}
	if _, _, err := g.flows.LearnParts([]flow.Ran{{Step: st, App: appKey(before)}}, time.Now()); err != nil {
		g.log.Warn("flows: keeping the field the person typed into", "err", err)
	}
}

// elementAt finds the element the person touched. A finger lands on
// pixels, where an agent names an element: of the elements at the point,
// the smallest control or element with an id is what was pressed (a
// button, not the text inside it), unless it is a large part of the
// screen, and else the smallest element. Text that only OCR reads is not
// something a finger presses.
func elementAt(sc *screen.Screen, p screen.Point) (screen.Element, bool) {
	var best, control screen.Element
	found, foundControl := false, false
	quarter := sc.Width * sc.Height / 4
	for _, e := range sc.Elements {
		b := e.Bounds
		if e.Source == screen.SourceOCR || b.Empty() || p.X < b.X || p.Y < b.Y || p.X >= b.X+b.W || p.Y >= b.Y+b.H {
			continue
		}
		area := b.W * b.H
		if !found || area < best.Bounds.W*best.Bounds.H {
			best, found = e, true
		}
		if (flow.Control(e.Role) || e.ID != "") && area <= quarter && (!foundControl || area < control.Bounds.W*control.Bounds.H) {
			control, foundControl = e, true
		}
	}
	if foundControl {
		return control, true
	}
	return best, found
}
