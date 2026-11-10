package deskapi

import (
	"net/http"
	"slices"
	"strings"
)

// SettingsView is Settings as the web client reads and patches them.
// API key profiles are changed through /api/model, not here.
type SettingsView struct {
	Preset             int           `json:"preset"`
	BlockedApps        []BlockedView `json:"blockedApps"`
	ReadOnly           bool          `json:"readOnly"`
	UnlockFromComputer bool          `json:"unlockFromComputer"`
	HideCodes          bool          `json:"hideCodes"`
	KeepDays           int           `json:"keepDays"`
	Record             bool          `json:"record"`
	DeveloperMode      bool          `json:"developerMode"`
	RemoteAgents       bool          `json:"remoteAgents"`
	RemoteAddr         string        `json:"remoteAddr"`
	UsageStats         bool          `json:"usageStats"`
	AutoUpdate         bool          `json:"autoUpdate"`
	ChatModel          string        `json:"chatModel"`
	ChatAgentModel     string        `json:"chatAgentModel"`
	OllamaBaseURL      string        `json:"ollamaBaseURL"`
	OllamaModels       []string      `json:"ollamaModels"`
}

// BlockedView is a blocked app.
type BlockedView struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

func settingsView(s Settings) SettingsView {
	v := SettingsView{
		Preset: s.Preset, ReadOnly: s.ReadOnly, UnlockFromComputer: s.UnlockFromComputer, HideCodes: s.HideCodes,
		KeepDays: s.KeepDays, Record: s.Record, DeveloperMode: s.DeveloperMode, RemoteAgents: s.RemoteAgents,
		RemoteAddr: s.RemoteAddr, UsageStats: s.UsageStats, AutoUpdate: s.AutoUpdate, ChatModel: s.ChatModel,
		ChatAgentModel: s.ChatAgentModel, OllamaBaseURL: s.OllamaBaseURL,
		OllamaModels: append([]string{}, s.OllamaModels...), BlockedApps: []BlockedView{},
	}
	if v.RemoteAddr == "" {
		v.RemoteAddr = DefaultRemoteAddr
	}
	for _, b := range s.BlockedApps {
		v.BlockedApps = append(v.BlockedApps, BlockedView{Name: b.Name, Kind: b.Kind, ID: b.ID})
	}
	return v
}

// settingsPatch is a change to Settings: only the fields sent change.
type settingsPatch struct {
	Preset             *int           `json:"preset"`
	BlockedApps        *[]BlockedView `json:"blockedApps"`
	ReadOnly           *bool          `json:"readOnly"`
	UnlockFromComputer *bool          `json:"unlockFromComputer"`
	HideCodes          *bool          `json:"hideCodes"`
	KeepDays           *int           `json:"keepDays"`
	Record             *bool          `json:"record"`
	DeveloperMode      *bool          `json:"developerMode"`
	RemoteAgents       *bool          `json:"remoteAgents"`
	RemoteAddr         *string        `json:"remoteAddr"`
	UsageStats         *bool          `json:"usageStats"`
	AutoUpdate         *bool          `json:"autoUpdate"`
	ChatModel          *string        `json:"chatModel"`
	ChatAgentModel     *string        `json:"chatAgentModel"`
	OllamaBaseURL      *string        `json:"ollamaBaseURL"`
	OllamaModels       *[]string      `json:"ollamaModels"`
}

func (p settingsPatch) apply(s *Settings) error {
	set := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	if p.Preset != nil {
		if *p.Preset < 1 || *p.Preset > 3 {
			return badRequest("preset must be 1, 2 or 3")
		}
		s.Preset = *p.Preset
	}
	if p.KeepDays != nil {
		if *p.KeepDays < 0 || *p.KeepDays > 3650 {
			return badRequest("keepDays out of range")
		}
		s.KeepDays = *p.KeepDays
	}
	if p.BlockedApps != nil {
		var list []BlockedApp
		seen := map[string]bool{}
		for _, b := range *p.BlockedApps {
			name := strings.TrimSpace(b.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			list = append(list, BlockedApp{Name: name, Kind: b.Kind, ID: b.ID})
		}
		s.BlockedApps = list
	}
	set(&s.ReadOnly, p.ReadOnly)
	set(&s.UnlockFromComputer, p.UnlockFromComputer)
	set(&s.HideCodes, p.HideCodes)
	set(&s.Record, p.Record)
	set(&s.DeveloperMode, p.DeveloperMode)
	set(&s.RemoteAgents, p.RemoteAgents)
	set(&s.UsageStats, p.UsageStats)
	set(&s.AutoUpdate, p.AutoUpdate)
	if p.RemoteAddr != nil {
		s.RemoteAddr = strings.TrimSpace(*p.RemoteAddr)
	}
	if p.ChatModel != nil {
		s.ChatModel = *p.ChatModel
	}
	if p.ChatAgentModel != nil {
		s.ChatAgentModel = *p.ChatAgentModel
	}
	if p.OllamaBaseURL != nil {
		s.OllamaBaseURL = strings.TrimSpace(*p.OllamaBaseURL)
	}
	if p.OllamaModels != nil {
		var list []string
		for _, m := range *p.OllamaModels {
			if m = strings.TrimSpace(m); m != "" && !slices.Contains(list, m) {
				list = append(list, m)
			}
		}
		s.OllamaModels = list
	}
	return nil
}

func (s *Server) currentSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Clone()
}

// refreshGatewaySettings takes in hand edits of rules.yaml and apps.yaml.
func (s *Server) refreshGatewaySettings() {
	if s.opt.GatewaySettings == nil {
		return
	}
	s.mu.Lock()
	since := s.policyV
	s.mu.Unlock()
	gs, v, ok := s.opt.GatewaySettings(since)
	s.mu.Lock()
	s.policyV = v
	if !ok {
		s.mu.Unlock()
		return
	}
	s.settings.Preset = gs.Preset
	s.settings.HideCodes = gs.HideCodes
	s.settings.ReadOnly = gs.ReadOnly
	s.settings.BlockedApps = append([]BlockedApp(nil), gs.BlockedApps...)
	next := s.settings.Clone()
	s.mu.Unlock()
	// Every window shows the hand edit, not only the one that asked.
	s.hub.publish("settings", settingsView(next))
}

func (s *Server) getSettings(*http.Request) (any, error) {
	s.refreshGatewaySettings()
	return settingsView(s.currentSettings()), nil
}

func (s *Server) putSettings(r *http.Request) (any, error) {
	var p settingsPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	next, err := s.updateSettings(p.apply)
	if err != nil {
		return nil, err
	}
	return settingsView(next), nil
}

// updateSettings changes the settings with fn and reports the change.
func (s *Server) updateSettings(fn func(*Settings) error) (Settings, error) {
	s.mu.Lock()
	old := s.settings.Clone()
	next := s.settings.Clone()
	if err := fn(&next); err != nil {
		s.mu.Unlock()
		return old, err
	}
	s.settings = next.Clone()
	s.mu.Unlock()
	if s.opt.OnSettings != nil {
		s.opt.OnSettings(old, next.Clone())
	}
	s.hub.publish("settings", settingsView(next))
	return next, nil
}
