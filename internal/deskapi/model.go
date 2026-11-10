package deskapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/secret"
)

// modelState is Settings › Model's: which profiles have a key, and the
// keys the keychain gave once this run (go run builds are signed ad hoc, so
// macOS asks for each Reveal). Keys never leave the server.
type modelState struct {
	mu       sync.Mutex
	migrated bool
	cached   map[string]string // keychain account → API key
	running  map[string]context.CancelFunc
}

func newModelKeyID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "k" + hex.EncodeToString(b[:])
}

// ModelView is Settings › Model and the composer's model menu.
type ModelView struct {
	Keychain       bool             `json:"keychain"`
	Profiles       []ProfileView    `json:"profiles"`
	Active         string           `json:"active"`
	ChatModel      string           `json:"chatModel"`
	ChatAgentModel string           `json:"chatAgentModel"`
	Models         []ChatModelView  `json:"models"`
	OllamaBaseURL  string           `json:"ollamaBaseURL"`
	OllamaModels   []string         `json:"ollamaModels"`
	Providers      []ProviderOption `json:"providers"`
}

// ProfileView is a saved API key, without the key.
type ProfileView struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Label    string `json:"label"`
	HasKey   bool   `json:"hasKey"`
}

// ChatModelView is a model the chat can use.
type ChatModelView struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Label    string `json:"label"`
	Ready    bool   `json:"ready"`
}

// ProviderOption is a provider a key can be added for.
type ProviderOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func (s *Server) modelRoutes() {
	s.route("GET /api/model", func(*http.Request) (any, error) { return s.modelView(), nil })
	s.route("POST /api/model/keys", s.addModelKey)
	s.route("PUT /api/model/keys/{id}", s.editModelKey)
	s.route("DELETE /api/model/keys/{id}", s.removeModelKey)
	s.route("PUT /api/model/active", func(r *http.Request) (any, error) {
		var b struct {
			ID string `json:"id"`
		}
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		_, err := s.updateSettings(func(st *Settings) error {
			for _, p := range st.ModelKeys {
				if p.ID == b.ID {
					st.ModelKeyActive = b.ID
					return nil
				}
			}
			return notFound("no such key")
		})
		if err != nil {
			return nil, err
		}
		return s.modelView(), nil
	})
	s.route("POST /api/model/ollama/refresh", func(r *http.Request) (any, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		base := inapp.NormalizeOllamaBaseURL(s.currentSettings().OllamaBaseURL)
		names, err := inapp.ListOllamaModels(ctx, base)
		if err != nil {
			return nil, &httpError{http.StatusBadGateway, err.Error()}
		}
		if _, err := s.updateSettings(func(st *Settings) error {
			for _, n := range names {
				if !containsTrim(st.OllamaModels, n) {
					st.OllamaModels = append(st.OllamaModels, n)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
		return s.modelView(), nil
	})
	s.route("POST /api/chat/cancel", func(r *http.Request) (any, error) {
		var b struct {
			RunID string `json:"runId"`
		}
		if err := decode(r, &b); err != nil {
			return nil, err
		}
		s.model.mu.Lock()
		cancel := s.model.running[b.RunID]
		s.model.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return ok, nil
	})
	s.mux.HandleFunc("POST /api/chat", s.chat)
}

func containsTrim(list []string, name string) bool {
	name = strings.TrimSpace(name)
	for _, n := range list {
		if strings.TrimSpace(n) == name {
			return true
		}
	}
	return false
}

// migrateProfiles makes profiles for keys saved before profiles existed,
// once a run.
func (s *Server) migrateProfiles() {
	s.model.mu.Lock()
	done := s.model.migrated
	s.model.migrated = true
	s.model.mu.Unlock()
	if done {
		return
	}
	store := s.secrets()
	cur := s.currentSettings()
	if len(cur.ModelKeys) > 0 && cur.ModelKeyActive != "" {
		return
	}
	var add []ModelKeyProfile
	if len(cur.ModelKeys) == 0 {
		for _, p := range inapp.Providers {
			if secret.Supported(store) && store.Has(inapp.LegacyKeyAccount(p)) {
				add = append(add, ModelKeyProfile{ID: newModelKeyID(), Provider: string(p), Label: p.Label()})
			}
		}
		if len(add) == 0 && cur.ModelProvider != "" && inapp.Valid(inapp.Provider(cur.ModelProvider)) {
			p := inapp.Provider(cur.ModelProvider)
			add = append(add, ModelKeyProfile{ID: newModelKeyID(), Provider: string(p), Label: p.Label()})
		}
	}
	if len(add) == 0 && (len(cur.ModelKeys) == 0 || cur.ModelKeyActive != "") {
		return
	}
	_, _ = s.updateSettings(func(st *Settings) error {
		st.ModelKeys = append(st.ModelKeys, add...)
		if st.ModelKeyActive == "" && len(st.ModelKeys) > 0 {
			st.ModelKeyActive = st.ModelKeys[0].ID
		}
		return nil
	})
}

// isLegacyProfile: the first profile of a provider may use the account
// name from before profiles.
func isLegacyProfile(st Settings, p ModelKeyProfile) bool {
	for _, q := range st.ModelKeys {
		if q.Provider == p.Provider {
			return q.ID == p.ID
		}
	}
	return false
}

func (s *Server) keyAccount(st Settings, p ModelKeyProfile) string {
	prov := inapp.Provider(p.Provider)
	store := s.secrets()
	acct := inapp.KeyAccount(prov, p.ID)
	if store.Has(acct) {
		return acct
	}
	if isLegacyProfile(st, p) && store.Has(inapp.LegacyKeyAccount(prov)) {
		return inapp.LegacyKeyAccount(prov)
	}
	return acct
}

// keyStatus says which profiles have a key saved.
func (s *Server) keyStatus(st Settings) map[string]bool {
	store := s.secrets()
	has := map[string]bool{}
	if !secret.Supported(store) {
		return has
	}
	for _, p := range st.ModelKeys {
		if !inapp.Valid(inapp.Provider(p.Provider)) {
			continue
		}
		has[p.ID] = store.Has(s.keyAccount(st, p))
	}
	return has
}

func allChatModels(st Settings) []inapp.ChatModel {
	out := append([]inapp.ChatModel{}, inapp.ChatModels...)
	return append(out, inapp.OllamaChatModels(st.OllamaModels)...)
}

func lookupChatModel(st Settings, id string) *inapp.ChatModel {
	for _, m := range allChatModels(st) {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

func providerReady(st Settings, has map[string]bool, prov inapp.Provider) bool {
	if prov == inapp.Ollama {
		for _, n := range st.OllamaModels {
			if strings.TrimSpace(n) != "" {
				return true
			}
		}
		return false
	}
	for _, p := range st.ModelKeys {
		if inapp.Provider(p.Provider) == prov && has[p.ID] {
			return true
		}
	}
	return false
}

// keyProfileFor is the profile whose key the chat uses for prov: the
// active one when it is prov's, else prov's first with a key.
func keyProfileFor(st Settings, has map[string]bool, prov inapp.Provider) *ModelKeyProfile {
	for _, p := range st.ModelKeys {
		if p.ID == st.ModelKeyActive && inapp.Provider(p.Provider) == prov && has[p.ID] {
			return &p
		}
	}
	for _, p := range st.ModelKeys {
		if inapp.Provider(p.Provider) == prov && has[p.ID] {
			return &p
		}
	}
	return nil
}

// resolveChatModel is the model the chat uses: the chosen one when it is
// ready, else the first that is (Ollama first), as ui.ensureChatModel.
func resolveChatModel(st Settings, has map[string]bool) *inapp.ChatModel {
	id := st.ChatModel
	if _, ollama := inapp.OllamaModelName(id); !ollama {
		id = inapp.NormalizeChatModelID(id)
	}
	if m := lookupChatModel(st, id); m != nil && providerReady(st, has, m.Provider) {
		return m
	}
	if providerReady(st, has, inapp.Ollama) {
		for _, n := range st.OllamaModels {
			if strings.TrimSpace(n) != "" {
				return lookupChatModel(st, inapp.OllamaChatModelID(n))
			}
		}
	}
	for _, m := range allChatModels(st) {
		if providerReady(st, has, m.Provider) {
			return &m
		}
	}
	return lookupChatModel(st, id)
}

// resolveAgentModel is the cloud model that drives the phone when the chat
// model is an Ollama model without tools.
func resolveAgentModel(st Settings, has map[string]bool) *inapp.ChatModel {
	id := inapp.NormalizeChatModelID(st.ChatAgentModel)
	if m := lookupChatModel(st, id); m != nil && m.Provider != inapp.Ollama && providerReady(st, has, m.Provider) {
		return m
	}
	for _, m := range inapp.ChatModels {
		if providerReady(st, has, m.Provider) {
			return &m
		}
	}
	return nil
}

func (s *Server) modelView() ModelView {
	s.migrateProfiles()
	st := s.currentSettings()
	has := s.keyStatus(st)
	v := ModelView{
		Keychain: secret.Supported(s.secrets()), Active: st.ModelKeyActive,
		OllamaBaseURL: inapp.NormalizeOllamaBaseURL(st.OllamaBaseURL), OllamaModels: append([]string{}, st.OllamaModels...),
		Profiles: []ProfileView{}, Models: []ChatModelView{}, Providers: []ProviderOption{},
	}
	for _, p := range inapp.Providers {
		v.Providers = append(v.Providers, ProviderOption{ID: string(p), Label: p.Label()})
	}
	for _, p := range st.ModelKeys {
		v.Profiles = append(v.Profiles, ProfileView{ID: p.ID, Provider: p.Provider, Label: p.Label, HasKey: has[p.ID]})
	}
	for _, m := range allChatModels(st) {
		v.Models = append(v.Models, ChatModelView{ID: m.ID, Provider: string(m.Provider), Label: m.Label, Ready: providerReady(st, has, m.Provider)})
	}
	if m := resolveChatModel(st, has); m != nil {
		v.ChatModel = m.ID
	}
	if m := resolveAgentModel(st, has); m != nil {
		v.ChatAgentModel = m.ID
	}
	return v
}

func (s *Server) addModelKey(r *http.Request) (any, error) {
	var b struct {
		Provider string `json:"provider"`
		Label    string `json:"label"`
		Key      string `json:"key"`
	}
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	prov := inapp.Provider(b.Provider)
	if !inapp.Valid(prov) || prov == inapp.Ollama {
		return nil, badRequest("unknown provider %q", b.Provider)
	}
	key := strings.TrimSpace(b.Key)
	if key == "" {
		return nil, badRequest("empty key")
	}
	label := strings.TrimSpace(b.Label)
	if label == "" {
		label = prov.Label()
	}
	prof := ModelKeyProfile{ID: newModelKeyID(), Provider: string(prov), Label: label}
	acct := inapp.KeyAccount(prov, prof.ID)
	if err := s.secrets().Save(acct, key); err != nil {
		return nil, err
	}
	s.cacheKey(acct, key)
	if _, err := s.updateSettings(func(st *Settings) error {
		st.ModelKeys = append(st.ModelKeys, prof)
		if st.ModelKeyActive == "" {
			st.ModelKeyActive = prof.ID
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.modelView(), nil
}

func (s *Server) editModelKey(r *http.Request) (any, error) {
	var b struct {
		Label *string `json:"label"`
		Key   *string `json:"key"`
	}
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	st := s.currentSettings()
	var prof *ModelKeyProfile
	for _, p := range st.ModelKeys {
		if p.ID == id {
			prof = &p
		}
	}
	if prof == nil {
		return nil, notFound("no such key")
	}
	if b.Key != nil {
		key := strings.TrimSpace(*b.Key)
		if key == "" {
			return nil, badRequest("empty key")
		}
		acct := inapp.KeyAccount(inapp.Provider(prof.Provider), prof.ID)
		if err := s.secrets().Save(acct, key); err != nil {
			return nil, err
		}
		s.forgetKeys(acct, inapp.LegacyKeyAccount(inapp.Provider(prof.Provider)))
		s.cacheKey(acct, key)
	}
	if b.Label != nil {
		label := strings.TrimSpace(*b.Label)
		if _, err := s.updateSettings(func(st *Settings) error {
			for i := range st.ModelKeys {
				if st.ModelKeys[i].ID == id && label != "" {
					st.ModelKeys[i].Label = label
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return s.modelView(), nil
}

func (s *Server) removeModelKey(r *http.Request) (any, error) {
	id := r.PathValue("id")
	st := s.currentSettings()
	var prof *ModelKeyProfile
	for _, p := range st.ModelKeys {
		if p.ID == id {
			prof = &p
		}
	}
	if prof == nil {
		return nil, notFound("no such key")
	}
	acct := s.keyAccount(st, *prof)
	if err := s.secrets().Delete(acct); err != nil && !errors.Is(err, secret.ErrNotFound) {
		return nil, err
	}
	s.forgetKeys(acct)
	if _, err := s.updateSettings(func(st *Settings) error {
		out := st.ModelKeys[:0:0]
		for _, p := range st.ModelKeys {
			if p.ID != id {
				out = append(out, p)
			}
		}
		st.ModelKeys = out
		if st.ModelKeyActive == id {
			st.ModelKeyActive = ""
			if len(out) > 0 {
				st.ModelKeyActive = out[0].ID
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.modelView(), nil
}

func (s *Server) cacheKey(acct, key string) {
	s.model.mu.Lock()
	defer s.model.mu.Unlock()
	if s.model.cached == nil {
		s.model.cached = map[string]string{}
	}
	s.model.cached[acct] = key
}

func (s *Server) forgetKeys(accts ...string) {
	s.model.mu.Lock()
	defer s.model.mu.Unlock()
	for _, a := range accts {
		delete(s.model.cached, a)
	}
}

// revealKey asks the keychain for a key once a run.
func (s *Server) revealKey(ctx context.Context, acct string) (string, error) {
	s.model.mu.Lock()
	k, ok := s.model.cached[acct]
	s.model.mu.Unlock()
	if ok {
		return k, nil
	}
	key, err := s.secrets().Reveal(ctx, acct, "use your model API key in Pinky's chat")
	if err != nil {
		return "", err
	}
	s.cacheKey(acct, key)
	return key, nil
}

// chatRequest is one message of the in-app chat.
type chatRequest struct {
	RunID      string        `json:"runId"`
	History    []chatMessage `json:"history"` // earlier messages of the session
	Prompt     string        `json:"prompt"`
	Title      string        `json:"title"`
	DeviceID   string        `json:"deviceId"`   // the phone the agent works on; "" only talks
	DeviceName string        `json:"deviceName"` // for the agent's instructions
	Phones     []string      `json:"phones"`     // the phones in scope, by name, for talk
	Locale     string        `json:"locale"`
}

type chatMessage struct {
	Role string `json:"role"` // user or assistant
	Text string `json:"text"`
}

// chat runs the in-app chat and streams Server-Sent Events: "progress"
// with each line of the agent's work, then "done" with the reply or
// "error" with code no_key, cancelled or failed and a message. Closing
// the request, or /api/chat/cancel with its runId, stops it.
func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		writeError(w, badRequest("empty message"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	go func() {
		select {
		case <-s.ctx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	if req.RunID != "" {
		s.model.mu.Lock()
		if s.model.running == nil {
			s.model.running = map[string]context.CancelFunc{}
		}
		s.model.running[req.RunID] = cancel
		s.model.mu.Unlock()
		defer func() {
			s.model.mu.Lock()
			delete(s.model.running, req.RunID)
			s.model.mu.Unlock()
		}()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flush(w)
	var wmu sync.Mutex
	send := func(name string, data any) {
		wmu.Lock()
		defer wmu.Unlock()
		_ = writeSSE(w, name, data)
	}

	reply, err := s.runChat(ctx, req, func(line string) { send("progress", map[string]string{"text": line}) })
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, secret.ErrCancelled), ctx.Err() != nil && err != nil:
		send("error", map[string]string{"code": "cancelled", "message": "stopped"})
	case errors.Is(err, errNoKey), errors.Is(err, secret.ErrNotFound):
		send("error", map[string]string{"code": "no_key", "message": "no API key"})
	case errors.Is(err, errNoFallback):
		send("error", map[string]string{"code": "no_fallback", "message": err.Error()})
	case err != nil:
		send("error", map[string]string{"code": "failed", "message": err.Error()})
	default:
		send("done", map[string]string{"reply": reply})
	}
}

var (
	errNoKey      = errors.New("no API key for the chat's model")
	errNoFallback = errors.New("this Ollama model cannot use tools, and no cloud model has a key to drive the phone")
)

func (s *Server) runChat(ctx context.Context, req chatRequest, emit func(string)) (string, error) {
	s.migrateProfiles()
	st := s.currentSettings()
	has := s.keyStatus(st)
	m := resolveChatModel(st, has)
	if m == nil || !providerReady(st, has, m.Provider) {
		return "", errNoKey
	}
	var prof *ModelKeyProfile
	if m.Provider != inapp.Ollama {
		if prof = keyProfileFor(st, has, m.Provider); prof == nil {
			return "", errNoKey
		}
	}
	ollamaBase := inapp.NormalizeOllamaBaseURL(st.OllamaBaseURL)

	if s.opt.Agent != nil && req.DeviceID != "" {
		run := func(model *inapp.ChatModel, profile *ModelKeyProfile) inapp.AgentOutcome {
			var apiKey string
			if model.Provider != inapp.Ollama {
				var err error
				if apiKey, err = s.revealKey(ctx, s.keyAccount(st, *profile)); err != nil {
					return inapp.AgentOutcome{Err: err}
				}
			}
			return inapp.RunAgent(ctx, inapp.AgentConfig{
				Gateway: s.opt.Agent, Provider: model.Provider, ModelID: model.ID, APIKey: apiKey,
				OllamaBase: ollamaBase, DeviceID: req.DeviceID, DeviceName: req.DeviceName,
				TaskTitle: req.Title, Prompt: req.Prompt, LocaleIsJA: strings.HasPrefix(req.Locale, "ja"),
			}, func(ev inapp.AgentEvent) { emit(ev.Text) })
		}
		out := run(m, prof)
		if out.Err == nil || m.Provider != inapp.Ollama || !inapp.ErrToolsNotSupported(out.Err) {
			return out.Reply, out.Err
		}
		fb := resolveAgentModel(st, has)
		if fb == nil {
			return "", errNoFallback
		}
		fprof := keyProfileFor(st, has, fb.Provider)
		if fprof == nil {
			return "", errNoFallback
		}
		emit(fmt.Sprintf("→ %s", fb.Label))
		out = run(fb, fprof)
		return out.Reply, out.Err
	}

	history := make([]inapp.Message, 0, len(req.History)+1)
	for _, h := range req.History {
		if h.Role == "user" || h.Role == "assistant" {
			history = append(history, inapp.Message{Role: h.Role, Text: h.Text})
		}
	}
	history = append(history, inapp.Message{Role: "user", Text: req.Prompt})
	phones := req.Phones
	if len(phones) == 0 {
		phones = []string{"(none)"}
	}
	system := "You are Pinky's in-app assistant. Be concise and practical. The person selected: " + strings.Join(phones, ", ") + "."
	if m.Provider == inapp.Ollama {
		name, ok := inapp.OllamaModelName(m.ID)
		if !ok {
			return "", fmt.Errorf("unknown Ollama model")
		}
		return inapp.CompleteOllama(ctx, ollamaBase, name, system, history)
	}
	key, err := s.revealKey(ctx, s.keyAccount(st, *prof))
	if err != nil {
		return "", err
	}
	return inapp.Complete(ctx, m.Provider, m.ID, key, system, history)
}
