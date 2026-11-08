package ui

import (
	"errors"
	"fmt"
	"strings"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/cmd/pinky/locales"
	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/secret"
)

// modelState is Settings › Model: which provider is chosen and whether
// its API key is in the keychain. The key itself never sits in Settings.
type modelState struct {
	ready   bool
	has     map[inapp.Provider]bool
	editing bool
	draft   string
	err     string
	busy    bool
	gen     int
}

func (a *app) refreshModelKeys() {
	st := &a.model
	store := a.secrets()
	if st.has == nil {
		st.has = map[inapp.Provider]bool{}
	}
	for _, p := range inapp.Providers {
		st.has[p] = keychainSupported(store) && store.Has(inapp.KeyAccount(p))
	}
	st.ready = true
	if a.settings.ModelProvider == "" || !inapp.Valid(inapp.Provider(a.settings.ModelProvider)) {
		a.settings.ModelProvider = string(inapp.Anthropic)
	}
}

func (a *app) settingsModel(c *mg.Context) {
	p := pal(c)
	if !a.model.ready {
		a.refreshModelKeys()
	}
	settingsTitle(c, locales.SettingsSectionModel(), locales.SettingsModelIntro())
	mg.Text(c, locales.SettingsModelExperiment()).FontSize(12.5).TextColor(p.ink3)

	prov := inapp.Provider(a.settings.ModelProvider)
	if !inapp.Valid(prov) {
		prov = inapp.Anthropic
		a.settings.ModelProvider = string(prov)
	}

	mg.Column(c).Gap(4).Children(func() {
		labels := make([]string, len(inapp.Providers))
		choice := inapp.Providers[0].Label()
		for i, pr := range inapp.Providers {
			labels[i] = pr.Label()
			if pr == prov {
				choice = pr.Label()
			}
		}
		settingRow(c, locales.SettingsModelProvider(), "", false, func() {
			if mg.Select(c.Key("model-provider"), &choice, labels).Label(locales.SettingsModelProvider()).MaxWidth(220).Changed() {
				for _, pr := range inapp.Providers {
					if pr.Label() == choice {
						a.settings.ModelProvider = string(pr)
					}
				}
				a.model.editing, a.model.draft, a.model.err = false, "", ""
				a.model.gen++
				a.changed()
				a.refreshModelKeys()
			}
		})

		storeOK := keychainSupported(a.secrets())
		mg.Column(c).Gap(8).Padding(10, 0).Children(func() {
			mg.Text(c, locales.SettingsModelKey()).FontWeight(600)
			if !storeOK {
				mg.Text(c, locales.UnlockNotAvailable()).FontSize(13).TextColor(p.ink2)
				return
			}
			a.modelKeyForm(c, prov)
		})
	})
}

func (a *app) modelKeyForm(c *mg.Context, prov inapp.Provider) {
	p := pal(c)
	st := &a.model
	has := st.has[prov]
	switch {
	case st.editing:
		in := mg.TextInput(c.Key(fmt.Sprintf("model-key-%d", st.gen)), &st.draft).Password().
			Placeholder(locales.SettingsModelKeyPlaceholder()).Label(locales.SettingsModelKey()).AutoFocus().MaxWidth(360)
		mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
			save := button(c, locales.SettingsModelKeySave(), btnOpt{kind: btnPrimary}).Disabled(st.busy || strings.TrimSpace(st.draft) == "")
			if (save.Clicked() || in.Submitted()) && !st.busy && strings.TrimSpace(st.draft) != "" {
				a.saveModelKey(prov)
			}
			if button(c, locales.PinCancel(), btnOpt{}).Clicked() {
				st.editing, st.draft, st.err = false, "", ""
				st.gen++
			}
			if st.busy {
				spinner(c)
			}
		})
		if st.err != "" {
			mg.Text(c, st.err).FontSize(13).TextColor(p.red).Role(mg.RoleStatus)
		}
	case has:
		mg.Row(c).Gap(12).AlignItems(mg.Center).Wrap().Children(func() {
			mg.Row(c).Gap(6).AlignItems(mg.Center).Children(func() {
				mg.Icon(c, icLock).FontSize(14).TextColor(p.ink2)
				mg.Text(c, locales.SettingsModelKeySaved()).FontSize(13.5).TextColor(p.ink2)
			})
			if linkButton(c, locales.SettingsModelKeyChange()).Clicked() {
				st.editing, st.draft, st.err = true, "", ""
				st.gen++
			}
			if linkButton(c, locales.SettingsModelKeyRemove()).Clicked() {
				a.removeModelKey(prov)
			}
		})
	default:
		mg.Text(c, locales.SettingsModelKeyMissing()).FontSize(13.5).TextColor(p.ink2)
		if button(c, locales.SettingsModelKeySave(), btnOpt{small: true, kind: btnPrimary}).Clicked() {
			st.editing, st.draft, st.err = true, "", ""
			st.gen++
		}
	}
}

func (a *app) saveModelKey(prov inapp.Provider) {
	st := &a.model
	key := strings.TrimSpace(st.draft)
	if key == "" {
		return
	}
	st.draft, st.err, st.busy = "", "", true
	st.gen++
	store := a.secrets()
	go func() {
		err := store.Save(inapp.KeyAccount(prov), key)
		a.post(func() {
			st.busy = false
			switch {
			case errors.Is(err, secret.ErrUnsupported):
				st.editing = false
				st.err = locales.UnlockNotAvailable()
			case err != nil:
				st.err = locales.SettingsModelKeyErrSave(err.Error())
			default:
				st.has[prov] = true
				st.editing = false
				a.toasts = append(a.toasts, locales.SettingsModelKeySavedToast(prov.Label()))
			}
		})
	}()
}

func (a *app) removeModelKey(prov inapp.Provider) {
	store := a.secrets()
	go func() {
		err := store.Delete(inapp.KeyAccount(prov))
		a.post(func() {
			if err != nil && !errors.Is(err, secret.ErrNotFound) {
				a.toasts = append(a.toasts, locales.SettingsModelKeyErrRemove(err.Error()))
				return
			}
			if a.model.has != nil {
				a.model.has[prov] = false
			}
			a.model.editing = false
			a.toasts = append(a.toasts, locales.SettingsModelKeyRemovedToast(prov.Label()))
		})
	}()
}
