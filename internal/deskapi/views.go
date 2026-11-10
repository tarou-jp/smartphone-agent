package deskapi

import (
	"time"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/screen"
)

// The JSON the web client reads. gateway's views have no JSON tags and
// hold images, so the client gets these copies, in camelCase, instead.

// Rect is a screen rectangle in device pixels.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Point is a screen point in device pixels.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func rectOf(r screen.Rect) *Rect {
	if r.Empty() {
		return nil
	}
	return &Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

func pointOf(p screen.Point) *Point {
	if p == (screen.Point{}) {
		return nil
	}
	return &Point{X: p.X, Y: p.Y}
}

// Device is one row of the sidebar.
type Device struct {
	ID            string     `json:"id"`
	Platform      string     `json:"platform"`
	Name          string     `json:"name"`
	Model         string     `json:"model"`
	OSVersion     string     `json:"osVersion"`
	Connection    string     `json:"connection"`
	Backend       string     `json:"backend"`
	State         string     `json:"state"`
	Detail        string     `json:"detail,omitempty"`
	Setup         string     `json:"setup,omitempty"`
	HelperExpires *time.Time `json:"helperExpires,omitempty"`
	Status        string     `json:"status"`
	StatusLine    string     `json:"statusLine"`
	TaskID        string     `json:"taskId,omitempty"`
	Features      Features   `json:"features"`
}

// Features are the driver's optional capabilities.
type Features struct {
	Unlock     bool `json:"unlock"`
	Background bool `json:"background"`
	Files      bool `json:"files"`
	Companion  bool `json:"companion"`
}

func deviceOf(d gateway.DeviceView) Device {
	v := Device{
		ID: d.Info.ID, Platform: string(d.Info.Platform), Name: d.Info.Name, Model: d.Info.Model,
		OSVersion: d.Info.OSVersion, Connection: d.Info.Connection, Backend: d.Info.Backend,
		State: string(d.Info.State), Detail: d.Info.Detail, Setup: d.Info.Setup,
		Status: string(d.Status), StatusLine: d.StatusLine, TaskID: d.TaskID,
		Features: Features(d.Features),
	}
	if v.Name == "" {
		v.Name = d.Info.ID
	}
	if !d.Info.HelperExpires.IsZero() {
		t := d.Info.HelperExpires
		v.HelperExpires = &t
	}
	return v
}

// Task is a task card or a row of Activity.
type Task struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"deviceId"`
	DeviceName string     `json:"deviceName"`
	Agent      string     `json:"agent"`
	Title      string     `json:"title"`
	State      string     `json:"state"`
	Started    time.Time  `json:"started"`
	Ended      *time.Time `json:"ended,omitempty"`
	Steps      []Step     `json:"steps"`
	ReadOnly   bool       `json:"readOnly,omitempty"`
}

// Step is one step of a task.
type Step struct {
	Index     int       `json:"index"`
	Text      string    `json:"text"`
	Tool      string    `json:"tool,omitempty"`
	Args      string    `json:"args,omitempty"`
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
	ElapsedMs int64     `json:"elapsedMs"`
	Before    string    `json:"before,omitempty"`
	After     string    `json:"after,omitempty"`
	Target    *Rect     `json:"target,omitempty"`
	Point     *Point    `json:"point,omitempty"`
}

func taskOf(t gateway.TaskView, dev bool) Task {
	v := Task{
		ID: t.ID, DeviceID: t.DeviceID, DeviceName: t.DeviceName, Agent: t.Agent, Title: t.Title,
		State: string(t.State), Started: t.Started, ReadOnly: t.ReadOnly, Steps: make([]Step, 0, len(t.Steps)),
	}
	if !t.Ended.IsZero() {
		e := t.Ended
		v.Ended = &e
	}
	for _, s := range t.Steps {
		st := Step{
			Index: s.Index, Text: s.Text, Status: s.Status, At: s.At, ElapsedMs: s.Elapsed.Milliseconds(),
			Before: s.Before, After: s.After, Target: rectOf(s.Target), Point: pointOf(s.Point),
		}
		// Tools and their arguments are for Developer mode only.
		if dev {
			st.Tool, st.Args = s.Tool, s.Args
		}
		v.Steps = append(v.Steps, st)
	}
	return v
}

// Need is a Needs you card.
type Need struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	DeviceID      string     `json:"deviceId"`
	DeviceName    string     `json:"deviceName"`
	App           string     `json:"app,omitempty"`
	Agent         string     `json:"agent,omitempty"`
	TaskID        string     `json:"taskId,omitempty"`
	Title         string     `json:"title"`
	Detail        string     `json:"detail,omitempty"`
	Rule          string     `json:"rule,omitempty"`
	Created       time.Time  `json:"created"`
	Deadline      *time.Time `json:"deadline,omitempty"`
	HasScreenshot bool       `json:"hasScreenshot"`
	ShotW         int        `json:"shotW,omitempty"`
	ShotH         int        `json:"shotH,omitempty"`
	Target        *Rect      `json:"target,omitempty"`
	Point         *Point     `json:"point,omitempty"`
	To            *Point     `json:"to,omitempty"`
	AlwaysAllow   bool       `json:"alwaysAllow,omitempty"`
	AllowTask     bool       `json:"allowTask,omitempty"`
	EndsOnUnlock  bool       `json:"endsOnUnlock,omitempty"`
	Unblock       bool       `json:"unblock,omitempty"`
	ReadOnly      bool       `json:"readOnly,omitempty"`
}

func needOf(n gateway.NeedView) Need {
	v := Need{
		ID: n.ID, Kind: string(n.Kind), DeviceID: n.DeviceID, DeviceName: n.DeviceName, App: n.App,
		Agent: n.Agent, TaskID: n.TaskID, Title: n.Title, Detail: n.Detail, Rule: n.Rule, Created: n.Created,
		Target: rectOf(n.Target), Point: pointOf(n.Point), To: pointOf(n.To),
		AlwaysAllow: n.AlwaysAllow, AllowTask: n.AllowTask, EndsOnUnlock: n.EndsOnUnlock,
		Unblock: n.Unblock, ReadOnly: n.ReadOnly,
	}
	if !n.Deadline.IsZero() {
		d := n.Deadline
		v.Deadline = &d
	}
	if n.Screenshot != nil {
		b := n.Screenshot.Bounds()
		v.HasScreenshot, v.ShotW, v.ShotH = true, b.Dx(), b.Dy()
	}
	return v
}

// State is everything the main window shows.
type State struct {
	Devices []Device `json:"devices"`
	Tasks   []Task   `json:"tasks"`
	Needs   []Need   `json:"needs"`
}

// Check is one item of a phone's checkup.
type Check struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Fix    string `json:"fix,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Checkup is a phone's checkup.
type Checkup struct {
	DeviceID string    `json:"deviceId"`
	Platform string    `json:"platform"`
	Family   string    `json:"family,omitempty"`
	System   string    `json:"system,omitempty"`
	Checks   []Check   `json:"checks"`
	At       time.Time `json:"at"`
}

func checkupOf(c gateway.CheckupView) Checkup {
	v := Checkup{DeviceID: c.DeviceID, Platform: string(c.Platform), Family: c.Family, System: c.System, At: c.At, Checks: []Check{}}
	for _, ch := range c.Checks {
		v.Checks = append(v.Checks, Check{ID: ch.ID, Status: string(ch.Status), Fix: ch.Fix, Detail: ch.Detail})
	}
	return v
}

// Connections are the ways an iPhone can be reached.
type Connections struct {
	Chosen  string          `json:"chosen"`
	Current string          `json:"current"`
	Ways    []ConnectionWay `json:"ways"`
}

// ConnectionWay is one way to reach a phone.
type ConnectionWay struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

func connectionsOf(c driver.Connections) Connections {
	v := Connections{Chosen: c.Chosen, Current: c.Current, Ways: []ConnectionWay{}}
	for _, w := range c.Ways {
		v.Ways = append(v.Ways, ConnectionWay(w))
	}
	return v
}
