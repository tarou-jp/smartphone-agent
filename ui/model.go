package ui

import (
	"context"
	"crypto/rand"
	hexenc "encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/cmd/pinky/locales"
	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/secret"
)

// modelState is Settings › Model: saved API key profiles and which one
// the chat uses. Secrets stay in the keychain.
type modelState struct {
	ready   bool
	has     map[string]bool // profile id → key present
	editing string          // profile id, or "new"
	draft   string
	label   string
	prov    inapp.Provider
	err         string
	busy        bool
	gen         int
	ollamaDraft string
	ollamaBusy  bool
	ollamaErr   string
	cachedKeys  map[string]string // keychain account → API key after one macOS prompt
}

func newModelKeyID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "k" + hexenc.EncodeToString(b[:])
}

func (a *app) ensureModelProfiles() {
	keys := a.settings.ModelKeys
	if len(keys) == 0 {
		store := a.secrets()
		for _, p := range inapp.Providers {
			if keychainSupported(store) && store.Has(inapp.LegacyKeyAccount(p)) {
				keys = append(keys, ModelKeyProfile{
					ID: newModelKeyID(), Provider: string(p), Label: p.Label(),
				})
			}
		}
		if len(keys) == 0 && a.settings.ModelProvider != "" && inapp.Valid(inapp.Provider(a.settings.ModelProvider)) {
			p := inapp.Provider(a.settings.ModelProvider)
			keys = append(keys, ModelKeyProfile{ID: newModelKeyID(), Provider: string(p), Label: p.Label()})
		}
		if len(keys) > 0 {
			a.settings.ModelKeys = keys
			a.changed()
		}
	}
	if a.settings.ModelKeyActive == "" && len(a.settings.ModelKeys) > 0 {
		a.settings.ModelKeyActive = a.settings.ModelKeys[0].ID
		a.changed()
	}
}

func (a *app) modelProfile(id string) *ModelKeyProfile {
	for i := range a.settings.ModelKeys {
		if a.settings.ModelKeys[i].ID == id {
			return &a.settings.ModelKeys[i]
		}
	}
	return nil
}

func (a *app) activeModelKey() *ModelKeyProfile {
	a.ensureModelProfiles()
	if p := a.modelProfile(a.settings.ModelKeyActive); p != nil {
		return p
	}
	if len(a.settings.ModelKeys) > 0 {
		return &a.settings.ModelKeys[0]
	}
	return nil
}

func (a *app) modelAnyKey() bool {
	if !a.model.ready {
		a.refreshModelKeys()
	}
	for _, ok := range a.model.has {
		if ok {
			return true
		}
	}
	return a.ollamaReady()
}

func (a *app) allChatModels() []inapp.ChatModel {
	out := append([]inapp.ChatModel{}, inapp.ChatModels...)
	out = append(out, inapp.OllamaChatModels(a.settings.OllamaModels)...)
	return out
}

func (a *app) lookupChatModel(id string) *inapp.ChatModel {
	for _, m := range a.allChatModels() {
		if m.ID == id {
			cp := m
			return &cp
		}
	}
	return nil
}

func (a *app) ollamaBaseURL() string {
	return inapp.NormalizeOllamaBaseURL(a.settings.OllamaBaseURL)
}

func (a *app) ollamaReady() bool {
	for _, n := range a.settings.OllamaModels {
		if strings.TrimSpace(n) != "" {
			return true
		}
	}
	return false
}

func (a *app) providerReady(prov inapp.Provider) bool {
	if prov == inapp.Ollama {
		return a.ollamaReady()
	}
	return a.providerHasKey(prov)
}

func (a *app) profileHasKey(p *ModelKeyProfile) bool {
	if p == nil || a.model.has == nil {
		return false
	}
	return a.model.has[p.ID]
}

func (a *app) providerHasKey(prov inapp.Provider) bool {
	if !a.model.ready {
		a.refreshModelKeys()
	}
	for i := range a.settings.ModelKeys {
		p := &a.settings.ModelKeys[i]
		if inapp.Provider(p.Provider) == prov && a.profileHasKey(p) {
			return true
		}
	}
	return false
}

func (a *app) keyProfileForProvider(prov inapp.Provider) *ModelKeyProfile {
	if p := a.activeModelKey(); p != nil && inapp.Provider(p.Provider) == prov && a.profileHasKey(p) {
		return p
	}
	for i := range a.settings.ModelKeys {
		p := &a.settings.ModelKeys[i]
		if inapp.Provider(p.Provider) == prov && a.profileHasKey(p) {
			return p
		}
	}
	return nil
}

func (a *app) ensureChatModel() {
	id := a.settings.ChatModel
	if _, ollama := inapp.OllamaModelName(id); !ollama {
		if norm := inapp.NormalizeChatModelID(id); norm != id {
			a.settings.ChatModel = norm
			a.changed()
		}
	}
	cur := a.lookupChatModel(a.settings.ChatModel)
	if cur == nil {
		if inapp.ChatModelByID(a.settings.ChatModel) != nil {
			a.settings.ChatModel = inapp.NormalizeChatModelID(a.settings.ChatModel)
			a.changed()
			cur = a.lookupChatModel(a.settings.ChatModel)
		}
	}
	if cur == nil && a.settings.ChatModel != "" {
		if name, ok := inapp.OllamaModelName(a.settings.ChatModel); ok && ollamaModelListed(a.settings.OllamaModels, name) {
			return
		}
	}
	if cur == nil {
		if a.ollamaReady() && len(a.settings.OllamaModels) > 0 {
			a.settings.ChatModel = inapp.OllamaChatModelID(a.settings.OllamaModels[0])
		} else {
			a.settings.ChatModel = inapp.DefaultChatModel().ID
		}
		a.changed()
		cur = a.lookupChatModel(a.settings.ChatModel)
	}
	if cur != nil && a.providerReady(cur.Provider) {
		return
	}
	if a.ollamaReady() && len(a.settings.OllamaModels) > 0 {
		id := inapp.OllamaChatModelID(a.settings.OllamaModels[0])
		if a.settings.ChatModel != id {
			a.settings.ChatModel = id
			a.changed()
		}
		return
	}
	for _, m := range a.allChatModels() {
		if a.providerReady(m.Provider) {
			if a.settings.ChatModel != m.ID {
				a.settings.ChatModel = m.ID
				a.changed()
			}
			return
		}
	}
	if a.settings.ChatModel == "" {
		a.settings.ChatModel = inapp.DefaultChatModel().ID
		a.changed()
	}
}

func (a *app) activeChatModel() *inapp.ChatModel {
	a.ensureChatModel()
	return a.lookupChatModel(a.settings.ChatModel)
}

func (a *app) chatModelReady() bool {
	m := a.activeChatModel()
	return m != nil && a.providerReady(m.Provider)
}

func (a *app) ensureChatAgentModel() {
	id := inapp.NormalizeChatModelID(a.settings.ChatAgentModel)
	if id != a.settings.ChatAgentModel {
		a.settings.ChatAgentModel = id
		a.changed()
	}
	if m := a.lookupChatModel(a.settings.ChatAgentModel); m != nil && m.Provider != inapp.Ollama && a.providerReady(m.Provider) {
		return
	}
	for _, m := range inapp.ChatModels {
		if a.providerReady(m.Provider) {
			if a.settings.ChatAgentModel != m.ID {
				a.settings.ChatAgentModel = m.ID
				a.changed()
			}
			return
		}
	}
}

func (a *app) activeChatAgentModel() (*inapp.ChatModel, *ModelKeyProfile) {
	a.ensureChatAgentModel()
	m := a.lookupChatModel(a.settings.ChatAgentModel)
	if m == nil || m.Provider == inapp.Ollama {
		return nil, nil
	}
	prof := a.keyProfileForProvider(m.Provider)
	if prof == nil || !a.providerReady(m.Provider) {
		return nil, nil
	}
	return m, prof
}

func (a *app) refreshModelKeys() {
	st := &a.model
	store := a.secrets()
	a.ensureModelProfiles()
	if st.has == nil {
		st.has = map[string]bool{}
	}
	for id := range st.has {
		delete(st.has, id)
	}
	for _, p := range a.settings.ModelKeys {
		prov := inapp.Provider(p.Provider)
		if !inapp.Valid(prov) {
			continue
		}
		acct := inapp.KeyAccount(prov, p.ID)
		if !keychainSupported(store) {
			st.has[p.ID] = false
			continue
		}
		// First profile per provider may still use the legacy account name.
		if !store.Has(acct) && store.Has(inapp.LegacyKeyAccount(prov)) && a.isLegacyProfile(&p) {
			acct = inapp.LegacyKeyAccount(prov)
		}
		st.has[p.ID] = store.Has(acct)
	}
	st.ready = true
	a.ensureChatModel()
	a.ensureChatAgentModel()
}

func (a *app) isLegacyProfile(p *ModelKeyProfile) bool {
	prov := inapp.Provider(p.Provider)
	for _, q := range a.settings.ModelKeys {
		if q.Provider == string(prov) {
			return q.ID == p.ID
		}
	}
	return false
}

// revealModelAPIKey returns a provider API key, asking the keychain once per
// account per Pinky run (go run is ad hoc signed, so macOS prompts each Reveal).
func (a *app) revealModelAPIKey(ctx context.Context, account string) (string, error) {
	st := &a.model
	if k, ok := st.cachedKeys[account]; ok {
		return k, nil
	}
	key, err := a.secrets().Reveal(ctx, account, locales.SettingsSectionModel())
	if err != nil {
		return "", err
	}
	if st.cachedKeys == nil {
		st.cachedKeys = map[string]string{}
	}
	st.cachedKeys[account] = key
	return key, nil
}

func (a *app) forgetModelAPIKeyCache(accounts ...string) {
	st := &a.model
	for _, acct := range accounts {
		delete(st.cachedKeys, acct)
	}
}

func (a *app) keyAccountFor(p *ModelKeyProfile) string {
	prov := inapp.Provider(p.Provider)
	store := a.secrets()
	acct := inapp.KeyAccount(prov, p.ID)
	if store.Has(acct) {
		return acct
	}
	if a.isLegacyProfile(p) && store.Has(inapp.LegacyKeyAccount(prov)) {
		return inapp.LegacyKeyAccount(prov)
	}
	return acct
}

func (a *app) settingsModel(c *mg.Context) {
	p := pal(c)
	if !a.model.ready {
		a.refreshModelKeys()
	}
	settingsTitle(c, locales.SettingsSectionModel(), locales.SettingsModelIntro())

	storeOK := keychainSupported(a.secrets())
	mg.Column(c).Gap(14).Children(func() {
		if !storeOK {
			mg.Text(c, locales.UnlockNotAvailable()).FontSize(13).TextColor(p.ink2)
		}
		for i := range a.settings.ModelKeys {
			prof := &a.settings.ModelKeys[i]
			a.modelProfileRow(c.Key(prof.ID), prof)
		}
		if a.model.editing == "new" {
			a.modelNewForm(c)
		} else if storeOK && button(c, locales.SettingsModelKeyAdd(), btnOpt{small: true}).Clicked() {
			a.model.editing = "new"
			a.model.label = ""
			a.model.prov = inapp.Anthropic
			a.model.draft, a.model.err = "", ""
			a.model.gen++
		}
		a.settingsOllama(c)
		a.settingsChatAgent(c)
	})
}

func (a *app) settingsChatAgent(c *mg.Context) {
	p := pal(c)
	a.ensureChatAgentModel()
	mg.Column(c).Gap(8).Padding(10, 12).Radius(12).Background(p.bg2).Border(1, p.line).Children(func() {
		mg.Text(c, locales.SettingsChatAgentTitle()).FontSize(14).FontWeight(600)
		mg.Text(c, locales.SettingsChatAgentIntro()).FontSize(13).TextColor(p.ink2)
		label := locales.ChatModelNone()
		if m := a.lookupChatModel(a.settings.ChatAgentModel); m != nil {
			label = m.Label
		}
		btn := mg.ButtonBase(c.Key("chat-agent-model")).Height(28).Padding(0, 10).Radius(8).
			AlignItems(mg.Center).Label(label).Background(p.bg3)
		btn.PopoverMenuButton(func(menu *mg.Menu) {
			for _, model := range inapp.ChatModels {
				if !a.providerReady(model.Provider) {
					continue
				}
				if menu.Item(model.Label).Checked(a.settings.ChatAgentModel == model.ID).Chosen() {
					a.settings.ChatAgentModel = model.ID
					a.changed()
				}
			}
		})
		btn.Children(func() {
			mg.Text(c, label).FontSize(13).FontWeight(500)
			mg.Icon(c, icChevD).FontSize(12).TextColor(p.ink3)
		})
	})
}

func (a *app) settingsOllama(c *mg.Context) {
	p := pal(c)
	st := &a.model
	mg.Column(c).Gap(8).Padding(10, 12).Radius(12).Background(p.bg2).Border(1, p.line).Children(func() {
		mg.Text(c, locales.SettingsOllamaTitle()).FontSize(14).FontWeight(600)
		mg.Text(c, locales.SettingsOllamaIntro()).FontSize(13).TextColor(p.ink2)
		urlIn := mg.TextInput(c.Key("ollama-url"), &a.settings.OllamaBaseURL).
			Placeholder(locales.SettingsOllamaURLPlaceholder()).Label(locales.SettingsOllamaURL()).MaxWidth(360)
		if urlIn.Changed() {
			a.changed()
		}
		mg.Text(c, locales.SettingsOllamaModels()).FontSize(13).FontWeight(600).Padding(4, 0, 0, 0)
		for i, name := range a.settings.OllamaModels {
			n := name
			idx := i
			mg.Row(c.Key(fmt.Sprintf("ollama-%d", idx))).AlignItems(mg.Center).Gap(8).Children(func() {
				mg.Text(c, n).FontSize(13).Grow(1).MinWidth(0).SingleLine()
				if linkButton(c, locales.SettingsOllamaModelRemove()).Clicked() {
					a.settings.OllamaModels = removeOllamaModel(a.settings.OllamaModels, idx)
					a.changed()
					a.ensureChatModel()
				}
			})
		}
		mg.Row(c).Gap(8).AlignItems(mg.Center).Wrap().Children(func() {
			draftIn := mg.TextInput(c.Key("ollama-model-draft"), &st.ollamaDraft).
				Placeholder(locales.SettingsOllamaModelPlaceholder()).Label(locales.SettingsOllamaModelAdd()).MaxWidth(220)
			if button(c, locales.SettingsOllamaModelAdd(), btnOpt{small: true}).Clicked() ||
				(draftIn.Submitted() && strings.TrimSpace(st.ollamaDraft) != "") {
				name := strings.TrimSpace(st.ollamaDraft)
				if name != "" && !ollamaModelListed(a.settings.OllamaModels, name) {
					a.settings.OllamaModels = append(a.settings.OllamaModels, name)
					a.changed()
					a.ensureChatModel()
				}
				st.ollamaDraft = ""
			}
			refresh := button(c, locales.SettingsOllamaRefresh(), btnOpt{small: true}).Disabled(st.ollamaBusy)
			if refresh.Clicked() && !st.ollamaBusy {
				a.refreshOllamaModels()
			}
			if st.ollamaBusy {
				spinner(c)
			}
		})
		if st.ollamaErr != "" {
			mg.Text(c, st.ollamaErr).FontSize(13).TextColor(p.red).Role(mg.RoleStatus)
		}
	})
}

func ollamaModelListed(list []string, name string) bool {
	name = strings.TrimSpace(name)
	for _, n := range list {
		if strings.TrimSpace(n) == name {
			return true
		}
	}
	return false
}

func removeOllamaModel(list []string, i int) []string {
	if i < 0 || i >= len(list) {
		return list
	}
	out := append([]string{}, list[:i]...)
	return append(out, list[i+1:]...)
}

func (a *app) refreshOllamaModels() {
	st := &a.model
	st.ollamaBusy, st.ollamaErr = true, ""
	base := a.ollamaBaseURL()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		names, err := inapp.ListOllamaModels(ctx, base)
		a.post(func() {
			st.ollamaBusy = false
			if err != nil {
				st.ollamaErr = locales.SettingsOllamaRefreshFail(err.Error())
				return
			}
			st.ollamaErr = ""
			merged := append([]string{}, a.settings.OllamaModels...)
			for _, n := range names {
				if !ollamaModelListed(merged, n) {
					merged = append(merged, n)
				}
			}
			if len(merged) > 0 {
				a.settings.OllamaModels = merged
				a.changed()
				a.ensureChatModel()
			}
		})
	}()
}

func (a *app) modelProfileRow(c *mg.Context, prof *ModelKeyProfile) {
	p := pal(c)
	st := &a.model
	prov := inapp.Provider(prof.Provider)
	title := prof.Label
	if title == "" {
		title = prov.Label()
	}
	has := st.has[prof.ID]
	active := prof.ID == a.settings.ModelKeyActive

	mg.Column(c).Gap(6).Padding(10, 12).Radius(12).Background(p.bg2).Border(1, p.line).Children(func() {
		mg.Row(c).AlignItems(mg.Center).Gap(10).Children(func() {
			mg.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				mg.Text(c, title).FontSize(14).FontWeight(600).SingleLine()
				mg.Text(c, prov.Label()).FontSize(12).TextColor(p.ink2)
			})
			if active {
				mg.Text(c, locales.SettingsModelKeyInUse()).FontSize(12).FontWeight(600).TextColor(p.accent)
			} else if has && linkButton(c, locales.SettingsModelKeyUse()).Clicked() {
				a.settings.ModelKeyActive = prof.ID
				a.changed()
			}
		})
		switch {
		case st.editing == prof.ID:
			a.modelKeyInput(c, prof.ID, prov)
		case has:
			mg.Row(c).Gap(12).AlignItems(mg.Center).Wrap().Children(func() {
				mg.Row(c).Gap(6).AlignItems(mg.Center).Children(func() {
					mg.Icon(c, icLock).FontSize(14).TextColor(p.ink2)
					mg.Text(c, locales.SettingsModelKeySaved()).FontSize(13).TextColor(p.ink2)
				})
				if linkButton(c, locales.SettingsModelKeyChange()).Clicked() {
					st.editing = prof.ID
					st.draft, st.err = "", ""
					st.gen++
				}
				if linkButton(c, locales.SettingsModelKeyRemove()).Clicked() {
					a.removeModelKey(prof)
				}
			})
		default:
			mg.Text(c, locales.SettingsModelKeyMissing()).FontSize(13).TextColor(p.ink2)
			if linkButton(c, locales.SettingsModelKeySave()).Clicked() {
				st.editing = prof.ID
				st.draft, st.err = "", ""
				st.gen++
			}
		}
	})
}

func (a *app) modelNewForm(c *mg.Context) {
	p := pal(c)
	st := &a.model
	mg.Column(c).Gap(8).Padding(10, 12).Radius(12).Background(p.bg2).Border(1, p.line).Children(func() {
		mg.Text(c, locales.SettingsModelKeyAdd()).FontSize(14).FontWeight(600)
		labels := make([]string, len(inapp.Providers))
		choice := inapp.Anthropic.Label()
		for i, pr := range inapp.Providers {
			labels[i] = pr.Label()
			if pr == st.prov {
				choice = pr.Label()
			}
		}
		mg.Row(c).Gap(10).AlignItems(mg.Center).Wrap().Children(func() {
			if mg.Select(c.Key("model-new-prov"), &choice, labels).Label(locales.SettingsModelProvider()).MaxWidth(180).Changed() {
				for _, pr := range inapp.Providers {
					if pr.Label() == choice {
						st.prov = pr
					}
				}
			}
			mg.TextInput(c.Key(fmt.Sprintf("model-label-%d", st.gen)), &st.label).
				Placeholder(locales.SettingsModelKeyLabelPlaceholder()).Label(locales.SettingsModelKeyLabel()).MaxWidth(220)
		})
		a.modelKeyInput(c, "new", st.prov)
		if st.err != "" {
			mg.Text(c, st.err).FontSize(13).TextColor(p.red).Role(mg.RoleStatus)
		}
		if linkButton(c, locales.PinCancel()).Clicked() {
			st.editing = ""
			st.draft, st.label, st.err = "", "", ""
			st.gen++
		}
	})
}

func (a *app) modelKeyInput(c *mg.Context, editID string, prov inapp.Provider) {
	st := &a.model
	in := mg.TextInput(c.Key(fmt.Sprintf("model-key-%s-%d", editID, st.gen)), &st.draft).Password().
		Placeholder(locales.SettingsModelKeyPlaceholder()).Label(locales.SettingsModelKey()).AutoFocus().MaxWidth(360)
	mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
		save := button(c, locales.SettingsModelKeySave(), btnOpt{kind: btnPrimary}).Disabled(st.busy || strings.TrimSpace(st.draft) == "")
		if (save.Clicked() || in.Submitted()) && !st.busy && strings.TrimSpace(st.draft) != "" {
			key := strings.TrimSpace(st.draft)
			if editID == "new" {
				a.saveNewModelKey(key)
			} else if p := a.modelProfile(editID); p != nil {
				a.saveModelKey(p, key)
			}
		}
		if st.busy {
			spinner(c)
		}
	})
}

func (a *app) saveNewModelKey(key string) {
	st := &a.model
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	label := strings.TrimSpace(st.label)
	if label == "" {
		label = st.prov.Label()
	}
	prof := ModelKeyProfile{ID: newModelKeyID(), Provider: string(st.prov), Label: label}
	st.draft, st.err, st.busy = "", "", true
	st.gen++
	a.saveModelKey(&prof, key)
}

func (a *app) saveModelKey(prof *ModelKeyProfile, key string) {
	st := &a.model
	key = strings.TrimSpace(key)
	if key == "" {
		st.busy = false
		return
	}
	st.draft, st.err, st.busy = "", "", true
	st.gen++
	prov := inapp.Provider(prof.Provider)
	isNew := a.modelProfile(prof.ID) == nil
	store := a.secrets()
	acct := inapp.KeyAccount(prov, prof.ID)
	if isNew && a.isLegacyProfile(prof) {
		// won't happen for new ids
	}
	go func() {
		err := store.Save(acct, key)
		a.post(func() {
			st.busy = false
			switch {
			case errors.Is(err, secret.ErrUnsupported):
				st.editing = ""
				st.err = locales.UnlockNotAvailable()
			case err != nil:
				st.err = locales.SettingsModelKeyErrSave(err.Error())
			default:
				if isNew {
					a.settings.ModelKeys = append(a.settings.ModelKeys, *prof)
					if a.settings.ModelKeyActive == "" {
						a.settings.ModelKeyActive = prof.ID
					}
				}
				st.has[prof.ID] = true
				st.editing = ""
				if st.cachedKeys == nil {
					st.cachedKeys = map[string]string{}
				}
				st.cachedKeys[acct] = key
				a.changed()
				a.toasts = append(a.toasts, locales.SettingsModelKeySavedToast(prov.Label()))
			}
		})
	}()
}

func (a *app) removeModelKey(prof *ModelKeyProfile) {
	prov := inapp.Provider(prof.Provider)
	store := a.secrets()
	acct := a.keyAccountFor(prof)
	go func() {
		err := store.Delete(acct)
		a.post(func() {
			if err != nil && !errors.Is(err, secret.ErrNotFound) {
				a.toasts = append(a.toasts, locales.SettingsModelKeyErrRemove(err.Error()))
				return
			}
			a.settings.ModelKeys = removeModelProfile(a.settings.ModelKeys, prof.ID)
			if a.settings.ModelKeyActive == prof.ID {
				a.settings.ModelKeyActive = ""
				if len(a.settings.ModelKeys) > 0 {
					a.settings.ModelKeyActive = a.settings.ModelKeys[0].ID
				}
			}
			if a.model.has != nil {
				delete(a.model.has, prof.ID)
			}
			a.model.editing = ""
			a.forgetModelAPIKeyCache(acct)
			if store.Has(inapp.LegacyKeyAccount(prov)) {
				a.forgetModelAPIKeyCache(inapp.LegacyKeyAccount(prov))
			}
			a.changed()
			a.refreshModelKeys()
			a.toasts = append(a.toasts, locales.SettingsModelKeyRemovedToast(prov.Label()))
		})
	}()
}

func removeModelProfile(list []ModelKeyProfile, id string) []ModelKeyProfile {
	out := make([]ModelKeyProfile, 0, len(list))
	for _, p := range list {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return out
}

func (a *app) modelPickerLabel(p *ModelKeyProfile) string {
	if p == nil {
		return locales.ChatModelNone()
	}
	label := strings.TrimSpace(p.Label)
	prov := inapp.Provider(p.Provider)
	if label == "" {
		return prov.Label()
	}
	return label + " · " + prov.Label()
}
