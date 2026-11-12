package deskapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"

	"github.com/DIYgod/pinky/internal/agents"
	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
)

// Methods the gateway has beyond gateway.API, as the MyGo client checks
// for them (ui/extras.go). The client is told which there are and leaves
// out what they back.
type (
	mediaPather interface {
		MediaPath(ctx context.Context, id string) (string, error)
	}
	agentLister interface {
		Agents(ctx context.Context) ([]gateway.AgentView, error)
	}
	agentScoper interface {
		SetAgentScope(name string, scope gateway.Scope)
	}
	historyWiper interface {
		WipeHistory(ctx context.Context) error
	}
	exemptionKeeper interface {
		Exemptions() []gateway.Exemption
		RemoveExemption(x gateway.Exemption) error
	}
	rulesEditor interface {
		Rules() gateway.RulesView
		SetRules(base, rules []gateway.RuleView) error
	}
	fileProblemLister interface {
		FileProblems() []gateway.FileProblem
	}
	pairer interface {
		StartPairing(ctx context.Context) (driver.Pairing, error)
		PairWithCode(ctx context.Context, addr, code string) (driver.Info, error)
	}
	wifiConnector interface {
		ConnectWiFi(ctx context.Context, addr string) (driver.Info, error)
	}
	companionInstaller interface {
		CompanionState(ctx context.Context, deviceID string) (driver.CompanionState, error)
		InstallCompanion(ctx context.Context, deviceID string) error
	}
	companionUpdater interface {
		UpdateCompanion(ctx context.Context, deviceID string) (driver.CompanionUpdate, error)
	}
	backgroundSetter interface {
		SetBackground(deviceID string, on bool) error
		Background(deviceID string) bool
	}
	connectionChooser interface {
		Connections(ctx context.Context, deviceID string) (driver.Connections, error)
		SetConnection(deviceID, way string) error
	}
	agentStreamer interface {
		AgentStream(ctx context.Context, deviceID string, opt driver.StreamOptions) (driver.Stream, error)
	}
	recordingFinder interface {
		Recording(ctx context.Context, taskID string) (gateway.RecordingView, error)
	}
	historyExporter interface {
		ExportHistory(ctx context.Context, taskIDs []string, dir string) (string, error)
	}
	hostChecker interface {
		HostChecks() map[driver.Platform]driver.HostCheck
	}
	diskImageDownloader interface {
		DiskImageMirror() (driver.DiskImageMirror, bool)
		DownloadDiskImage(ctx context.Context, progress func(done, total int64)) error
	}
	checkupRunner interface {
		Checkup(ctx context.Context, deviceID string) (gateway.CheckupView, error)
	}
	phoneRemover interface {
		RemoveDevice(ctx context.Context, deviceID string, opt gateway.RemoveOptions) error
	}
)

// capabilities says which extras the gateway has, by the names the client
// checks.
func (s *Server) capabilities() map[string]bool {
	a := s.api
	_, media := a.(mediaPather)
	_, agentList := a.(agentLister)
	_, agentScope := a.(agentScoper)
	_, wipe := a.(historyWiper)
	_, exemptions := a.(exemptionKeeper)
	_, rules := a.(rulesEditor)
	_, problems := a.(fileProblemLister)
	_, pairing := a.(pairer)
	_, wifi := a.(wifiConnector)
	_, companion := a.(companionInstaller)
	_, companionUpdate := a.(companionUpdater)
	_, background := a.(backgroundSetter)
	_, connections := a.(connectionChooser)
	_, agentStream := a.(agentStreamer)
	_, recordings := a.(recordingFinder)
	_, export := a.(historyExporter)
	_, hostChecks := a.(hostChecker)
	_, diskImage := a.(diskImageDownloader)
	_, checkup := a.(checkupRunner)
	_, remove := a.(phoneRemover)
	_, library := a.(libraryKeeper)
	return map[string]bool{
		"media": media, "agents": agentList, "agentScope": agentScope, "wipeHistory": wipe,
		"exemptions": exemptions, "rules": rules, "fileProblems": problems, "pairing": pairing,
		"wifi": wifi, "companion": companion, "companionUpdate": companionUpdate, "background": background,
		"connections": connections, "agentStream": agentStream, "recordings": recordings, "export": export,
		"hostChecks": hostChecks, "diskImage": diskImage, "checkup": checkup, "removePhone": remove,
		"agentSetup": s.opt.AgentSetup != nil,
		"library":    library,
	}
}

// pairState is the server's own state beside the settings: pairings in
// progress, mirror sizes and the files the client may open.
type pairState struct {
	seq      int
	pairings map[string]*pairing
	sizes    map[string][2]int
	exports  map[string]bool
	disk     *diskProgress
}

type pairing struct {
	p      driver.Pairing
	cancel context.CancelFunc
	once   sync.Once
}

func (p *pairing) close() {
	p.once.Do(func() {
		p.cancel()
		_ = p.p.Close()
	})
}

// DiskProgress is the developer disk image's download.
type diskProgress struct {
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
	Ok      bool   `json:"ok"`
}

func (s *Server) extraRoutes() {
	// Agents.
	s.route("GET /api/agents", func(r *http.Request) (any, error) {
		al, ok := s.api.(agentLister)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		list, err := al.Agents(ctx)
		if err != nil {
			return nil, err
		}
		out := []AgentView{}
		for _, a := range list {
			out = append(out, AgentView{Name: a.Name, Approved: a.Approved, Scope: ScopeView{Mode: string(a.Scope.Mode), Apps: append([]string{}, a.Scope.Apps...)}})
		}
		return out, nil
	})
	s.route("PUT /api/agents/{name}/scope", func(r *http.Request) (any, error) {
		sc, ok := s.api.(agentScoper)
		if !ok {
			return nil, errUnsupported
		}
		var b ScopeView
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		switch gateway.ScopeMode(b.Mode) {
		case gateway.ScopeEverything, gateway.ScopeReadOnly, gateway.ScopeWatchOnly, gateway.ScopeApps:
		default:
			return nil, badRequest("unknown scope %q", b.Mode)
		}
		sc.SetAgentScope(r.PathValue("name"), gateway.Scope{Mode: gateway.ScopeMode(b.Mode), Apps: b.Apps})
		return ok, nil
	})
	s.route("GET /api/agent-setup", func(r *http.Request) (any, error) {
		if s.opt.AgentSetup == nil {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		out := []AgentSetupView{}
		for _, st := range s.opt.AgentSetup.Find(ctx) {
			out = append(out, agentSetupOf(st))
		}
		return out, nil
	})
	s.route("POST /api/agent-setup/{id}/{op}", func(r *http.Request) (any, error) {
		if s.opt.AgentSetup == nil {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		var res agents.Result
		var err error
		switch r.PathValue("op") {
		case "connect":
			res, err = s.opt.AgentSetup.Connect(ctx, r.PathValue("id"))
		case "remove":
			res, err = s.opt.AgentSetup.Remove(ctx, r.PathValue("id"))
		default:
			return nil, notFound("unknown operation")
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"backups": res.Backups, "restart": restartName(res.Restart), "opened": res.Opened}, nil
	})

	// Safety.
	s.route("GET /api/exemptions", func(*http.Request) (any, error) {
		k, ok := s.api.(exemptionKeeper)
		if !ok {
			return nil, errUnsupported
		}
		out := []ExemptionView{}
		for _, x := range k.Exemptions() {
			out = append(out, ExemptionView(x))
		}
		return out, nil
	})
	s.route("POST /api/exemptions/remove", func(r *http.Request) (any, error) {
		k, ok := s.api.(exemptionKeeper)
		if !ok {
			return nil, errUnsupported
		}
		var b ExemptionView
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, k.RemoveExemption(gateway.Exemption(b))
	})
	s.route("GET /api/rules", func(*http.Request) (any, error) {
		ed, ok := s.api.(rulesEditor)
		if !ok {
			return nil, errUnsupported
		}
		v := ed.Rules()
		return RulesView{File: rulesOf(v.File), BuiltIn: rulesOf(v.BuiltIn)}, nil
	})
	s.route("PUT /api/rules", func(r *http.Request) (any, error) {
		ed, ok := s.api.(rulesEditor)
		if !ok {
			return nil, errUnsupported
		}
		var b struct {
			Base  []RuleView `json:"base"`
			Rules []RuleView `json:"rules"`
		}
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		if err := ed.SetRules(rulesTo(b.Base), rulesTo(b.Rules)); err != nil {
			if err == gateway.ErrRulesChanged {
				return nil, &httpError{http.StatusConflict, err.Error()}
			}
			return nil, err
		}
		v := ed.Rules()
		return RulesView{File: rulesOf(v.File), BuiltIn: rulesOf(v.BuiltIn)}, nil
	})
	s.route("GET /api/file-problems", func(*http.Request) (any, error) {
		out := []FileProblemView{}
		if fl, ok := s.api.(fileProblemLister); ok {
			for _, p := range fl.FileProblems() {
				out = append(out, FileProblemView(p))
			}
		}
		return out, nil
	})

	// Activity.
	s.route("DELETE /api/history", func(r *http.Request) (any, error) {
		w, ok := s.api.(historyWiper)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		return ok, w.WipeHistory(ctx)
	})
	s.route("POST /api/history/export", func(r *http.Request) (any, error) {
		exp, ok := s.api.(historyExporter)
		if !ok {
			return nil, errUnsupported
		}
		var b struct {
			TaskIDs []string `json:"taskIds"`
		}
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		dir := downloadsDir()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		path, err := exp.ExportHistory(ctx, b.TaskIDs, dir)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		if s.pair.exports == nil {
			s.pair.exports = map[string]bool{}
		}
		s.pair.exports[path] = true
		s.mu.Unlock()
		return map[string]string{"path": path}, nil
	})
	s.route("GET /api/recordings/{task}", func(r *http.Request) (any, error) {
		rf, ok := s.api.(recordingFinder)
		if !ok {
			return nil, errUnsupported
		}
		rec, err := rf.Recording(r.Context(), r.PathValue("task"))
		if err != nil {
			return nil, notFound("%v", err)
		}
		v := RecordingInfo{TaskID: rec.TaskID, Start: rec.Start, LengthMs: rec.Length.Milliseconds(), HasFile: rec.Path != "", Marks: []RecordingMarkView{}}
		for _, m := range rec.Marks {
			v.Marks = append(v.Marks, RecordingMarkView{AtMs: m.At.Milliseconds(), Step: m.Step, Text: m.Text})
		}
		return v, nil
	})
	s.mux.HandleFunc("GET /api/recordings/{task}/file", func(w http.ResponseWriter, r *http.Request) {
		rf, ok := s.api.(recordingFinder)
		if !ok {
			http.NotFound(w, r)
			return
		}
		rec, err := rf.Recording(r.Context(), r.PathValue("task"))
		if err != nil || rec.Path == "" {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(rec.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	})

	// Phones.
	s.route("GET /api/host-checks", func(*http.Request) (any, error) {
		out := map[string]HostCheckView{}
		if hc, ok := s.api.(hostChecker); ok {
			for p, c := range hc.HostChecks() {
				out[string(p)] = HostCheckView{Missing: c.Missing, Detail: c.Detail}
			}
		}
		return out, nil
	})
	s.route("GET /api/devices/{id}/companion", func(r *http.Request) (any, error) {
		ci, ok := s.api.(companionInstaller)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		st, err := ci.CompanionState(ctx, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return map[string]string{"state": string(st)}, nil
	})
	s.route("POST /api/devices/{id}/companion/install", func(r *http.Request) (any, error) {
		ci, ok := s.api.(companionInstaller)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		return ok, ci.InstallCompanion(ctx, r.PathValue("id"))
	})
	s.route("POST /api/devices/{id}/companion/update", func(r *http.Request) (any, error) {
		cu, ok := s.api.(companionUpdater)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		u, err := cu.UpdateCompanion(ctx, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"from": u.From, "to": u.To, "updated": u.Updated, "replaced": u.Replaced, "serviceOn": u.ServiceOn}, nil
	})
	s.route("GET /api/devices/{id}/background", func(r *http.Request) (any, error) {
		bs, ok := s.api.(backgroundSetter)
		if !ok {
			return nil, errUnsupported
		}
		return map[string]bool{"on": bs.Background(r.PathValue("id"))}, nil
	})
	s.route("PUT /api/devices/{id}/background", func(r *http.Request) (any, error) {
		bs, ok := s.api.(backgroundSetter)
		if !ok {
			return nil, errUnsupported
		}
		var b struct{ On bool }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		if err := bs.SetBackground(r.PathValue("id"), b.On); err != nil {
			return nil, err
		}
		return map[string]bool{"on": bs.Background(r.PathValue("id"))}, nil
	})
	s.route("GET /api/devices/{id}/connections", func(r *http.Request) (any, error) {
		cc, ok := s.api.(connectionChooser)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		c, err := cc.Connections(ctx, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return connectionsOf(c), nil
	})
	s.route("PUT /api/devices/{id}/connection", func(r *http.Request) (any, error) {
		cc, ok := s.api.(connectionChooser)
		if !ok {
			return nil, errUnsupported
		}
		var b struct{ Way string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		return ok, cc.SetConnection(r.PathValue("id"), b.Way)
	})
	s.route("POST /api/devices/{id}/checkup", func(r *http.Request) (any, error) {
		cr, ok := s.api.(checkupRunner)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		c, err := cr.Checkup(ctx, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return checkupOf(c), nil
	})
	s.route("DELETE /api/devices/{id}", func(r *http.Request) (any, error) {
		pr, ok := s.api.(phoneRemover)
		if !ok {
			return nil, errUnsupported
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()
		return ok, pr.RemoveDevice(ctx, r.PathValue("id"), gateway.RemoveOptions{History: r.URL.Query().Get("history") == "1"})
	})

	// Adding a phone.
	s.route("POST /api/pairing", s.startPairing)
	s.route("GET /api/pairing/{id}/wait", s.waitPairing)
	s.route("DELETE /api/pairing/{id}", func(r *http.Request) (any, error) {
		s.mu.Lock()
		p := s.pair.pairings[r.PathValue("id")]
		delete(s.pair.pairings, r.PathValue("id"))
		s.mu.Unlock()
		if p != nil {
			p.close()
		}
		return ok, nil
	})
	s.mux.HandleFunc("GET /api/pairing/{id}/qr.png", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := s.pair.pairings[r.PathValue("id")]
		s.mu.Unlock()
		if p == nil {
			http.NotFound(w, r)
			return
		}
		code, err := qr.Encode(p.p.QR(), qr.M)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		code.Scale = 8
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(code.PNG())
	})
	s.route("POST /api/pairing/code", func(r *http.Request) (any, error) {
		pr, ok := s.api.(pairer)
		if !ok {
			return nil, errUnsupported
		}
		var b struct{ Addr, Code string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()
		info, err := pr.PairWithCode(ctx, strings.TrimSpace(b.Addr), strings.TrimSpace(b.Code))
		if err != nil {
			return nil, err
		}
		return deviceOf(gateway.DeviceView{Info: info}), nil
	})
	s.route("POST /api/pairing/wifi", func(r *http.Request) (any, error) {
		wc, ok := s.api.(wifiConnector)
		if !ok {
			return nil, errUnsupported
		}
		var b struct{ Addr string }
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()
		info, err := wc.ConnectWiFi(ctx, strings.TrimSpace(b.Addr))
		if err != nil {
			return nil, err
		}
		return deviceOf(gateway.DeviceView{Info: info}), nil
	})
	s.route("GET /api/disk-image", func(*http.Request) (any, error) {
		dl, ok := s.api.(diskImageDownloader)
		if !ok {
			return nil, errUnsupported
		}
		m, need := dl.DiskImageMirror()
		s.mu.Lock()
		var prog diskProgress
		if s.pair.disk != nil {
			prog = *s.pair.disk
		}
		s.mu.Unlock()
		return map[string]any{"needed": need, "name": m.Name, "image": m.Image, "size": m.Size, "progress": prog}, nil
	})
	s.route("POST /api/disk-image/download", func(*http.Request) (any, error) {
		dl, ok := s.api.(diskImageDownloader)
		if !ok {
			return nil, errUnsupported
		}
		s.mu.Lock()
		if s.pair.disk != nil && s.pair.disk.Running {
			s.mu.Unlock()
			return ok, nil
		}
		s.pair.disk = &diskProgress{Running: true}
		s.mu.Unlock()
		go func() {
			last := time.Time{}
			err := dl.DownloadDiskImage(s.ctx, func(done, total int64) {
				s.mu.Lock()
				s.pair.disk.Done, s.pair.disk.Total = done, total
				p := *s.pair.disk
				s.mu.Unlock()
				if time.Since(last) > 200*time.Millisecond {
					last = time.Now()
					s.hub.publish("diskImage", p)
				}
			})
			s.mu.Lock()
			s.pair.disk.Running = false
			s.pair.disk.Ok = err == nil
			if err != nil {
				s.pair.disk.Error = err.Error()
			}
			p := *s.pair.disk
			s.mu.Unlock()
			s.hub.publish("diskImage", p)
		}()
		return ok, nil
	})
}

func (s *Server) startPairing(r *http.Request) (any, error) {
	pr, ok := s.api.(pairer)
	if !ok {
		return nil, errUnsupported
	}
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	pg, err := pr.StartPairing(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	s.mu.Lock()
	s.pair.seq++
	id := "p" + time.Now().Format("150405") + "-" + itoa(s.pair.seq)
	if s.pair.pairings == nil {
		s.pair.pairings = map[string]*pairing{}
	}
	p := &pairing{p: pg, cancel: cancel}
	s.pair.pairings[id] = p
	s.mu.Unlock()
	go func() {
		<-ctx.Done()
		p.close()
	}()
	return map[string]string{"id": id, "qr": pg.QR()}, nil
}

// waitPairing waits up to 25 s for the phone to pair: the client asks
// again while it shows the QR code.
func (s *Server) waitPairing(r *http.Request) (any, error) {
	s.mu.Lock()
	p := s.pair.pairings[r.PathValue("id")]
	s.mu.Unlock()
	if p == nil {
		return nil, notFound("no such pairing")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	info, err := p.p.Wait(ctx)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() == nil {
			return map[string]any{"paired": false}, nil
		}
		return nil, err
	}
	s.mu.Lock()
	delete(s.pair.pairings, r.PathValue("id"))
	s.mu.Unlock()
	p.close()
	return map[string]any{"paired": true, "device": deviceOf(gateway.DeviceView{Info: info})}, nil
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

func downloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	d := filepath.Join(home, "Downloads")
	if fi, err := os.Stat(d); err == nil && fi.IsDir() {
		return d
	}
	return home
}

// AgentView is a row of Settings › Agents.
type AgentView struct {
	Name     string    `json:"name"`
	Approved bool      `json:"approved"`
	Scope    ScopeView `json:"scope"`
}

// ScopeView is an agent's scope.
type ScopeView struct {
	Mode string   `json:"mode"`
	Apps []string `json:"apps"`
}

// AgentSetupView is an agent on this computer.
type AgentSetupView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	Found     bool   `json:"found"`
	State     string `json:"state"`
	Command   string `json:"command,omitempty"`
	Settings  string `json:"settings,omitempty"`
	Restart   string `json:"restart"`
	Problem   string `json:"problem,omitempty"`
	Manual    bool   `json:"manual,omitempty"`
	Extension bool   `json:"extension,omitempty"`
	Opens     bool   `json:"opens,omitempty"`
}

func agentSetupOf(st agents.Status) AgentSetupView {
	state := "not_connected"
	switch st.State {
	case agents.Connected:
		state = "connected"
	case agents.Outdated:
		state = "outdated"
	}
	return AgentSetupView{
		ID: st.ID, Name: st.Name, URL: st.URL, Found: st.Found, State: state, Command: st.Command,
		Settings: st.Settings, Restart: restartName(st.Restart), Problem: st.Problem, Manual: st.Manual,
		Extension: st.Extension, Opens: st.Opens,
	}
}

func restartName(k agents.RestartKind) string {
	switch k {
	case agents.RestartSession:
		return "session"
	case agents.RestartApp:
		return "app"
	}
	return "none"
}

// ExemptionView is an Always allow answer.
type ExemptionView struct {
	Rule    string   `json:"rule"`
	With    []string `json:"with"`
	AppID   string   `json:"appId"`
	AppName string   `json:"appName"`
}

// RuleView is a policy rule.
type RuleView struct {
	Name       string   `json:"name"`
	Apps       []string `json:"apps"`
	Categories []string `json:"categories"`
	Actions    []string `json:"actions"`
	Labels     []string `json:"labels"`
	Decision   string   `json:"decision"`
}

// RulesView is the rules file's rules and Pinky's own.
type RulesView struct {
	File    []RuleView `json:"file"`
	BuiltIn []RuleView `json:"builtIn"`
}

func rulesOf(list []gateway.RuleView) []RuleView {
	out := []RuleView{}
	for _, r := range list {
		out = append(out, RuleView{
			Name: r.Name, Apps: nonNil(r.Apps), Categories: nonNil(r.Categories), Actions: nonNil(r.Actions),
			Labels: nonNil(r.Labels), Decision: r.Decision,
		})
	}
	return out
}

func rulesTo(list []RuleView) []gateway.RuleView {
	var out []gateway.RuleView
	for _, r := range list {
		out = append(out, gateway.RuleView{
			Name: r.Name, Apps: emptyNil(r.Apps), Categories: emptyNil(r.Categories), Actions: emptyNil(r.Actions),
			Labels: emptyNil(r.Labels), Decision: r.Decision,
		})
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func emptyNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// FileProblemView is a rules file Pinky cannot use.
type FileProblemView struct {
	File  string `json:"file"`
	Path  string `json:"path"`
	Error string `json:"error"`
}

// HostCheckView is what the computer lacks for a platform's phones.
type HostCheckView struct {
	Missing string `json:"missing,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// RecordingInfo is a task's recording.
type RecordingInfo struct {
	TaskID   string              `json:"taskId"`
	Start    time.Time           `json:"start"`
	LengthMs int64               `json:"lengthMs"`
	HasFile  bool                `json:"hasFile"`
	Marks    []RecordingMarkView `json:"marks"`
}

// RecordingMarkView is a step's place in a recording.
type RecordingMarkView struct {
	AtMs int64  `json:"atMs"`
	Step int    `json:"step"`
	Text string `json:"text"`
}
