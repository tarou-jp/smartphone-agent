package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/cmd/pinky/locales"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/secret"
)

type chatRole string

const (
	chatUser      chatRole = "user"
	chatAssistant chatRole = "assistant"
	chatError     chatRole = "error"
)

type chatMsg struct {
	Role chatRole
	Text string
	At   time.Time
}

// chatState is the far-right chat column (fork experiment). Phone targets
// follow the sidebar selection (one phone or All phones).
type chatState struct {
	draft string
	msgs  []chatMsg
	busy  bool
}

// chatColumn is Cursor-like: header with New chat, a clean thread, composer.
func (a *app) chatColumn(c *mg.Context) {
	p := pal(c)
	if !a.model.ready {
		a.refreshModelKeys()
	}
	mg.Column(c).Width(340).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(p.line).Background(p.bg).
		MinHeight(0).Label(locales.ChatSection()).Children(func() {
		// Header
		mg.Row(c).Height(52).Shrink(0).Padding(0, 14).AlignItems(mg.Center).Gap(10).
			BorderWidth(0, 0, 1, 0).BorderColor(p.line).Children(func() {
			mg.Text(c, locales.ChatSection()).FontSize(14).FontWeight(650).Grow(1)
			newChat := button(c, locales.ChatNew(), btnOpt{small: true}).Disabled(a.chat.busy || (len(a.chat.msgs) == 0 && a.chat.draft == ""))
			if newChat.Clicked() && !a.chat.busy {
				a.chatNew()
			}
		})

		// Thread
		mg.Scroll(c).Grow(1).MinHeight(0).Padding(16, 16).Gap(14).Children(func() {
			if len(a.chat.msgs) == 0 && !a.chat.busy {
				mg.Column(c).Grow(1).MinHeight(200).Center().Gap(10).Padding(8, 12).Children(func() {
					mg.Icon(c, icChat).FontSize(28).TextColor(p.ink3)
					mg.Text(c, locales.ChatEmpty()).FontSize(14).TextColor(p.ink2).TextAlign(mg.Center)
					prov := inapp.Provider(a.settings.ModelProvider)
					if !a.model.has[prov] {
						if linkButton(c, locales.ChatNeedKeyLink()).Clicked() {
							a.page, a.set.sec = pageSettings, secModel
						}
					}
				})
				return
			}
			for i, m := range a.chat.msgs {
				a.chatBubble(c.Key(fmt.Sprintf("chat-%s-%d", m.Role, i)), m)
			}
			if a.chat.busy {
				mg.Row(c).Gap(8).AlignItems(mg.Center).Padding(4, 0).Children(func() {
					spinner(c)
					mg.Text(c, locales.ChatThinking()).FontSize(13).TextColor(p.ink3)
				})
			}
		})

		// Composer — flat bar, not a nested “attachment” card
		mg.Column(c).Shrink(0).Padding(12, 14, 14, 14).Gap(8).
			BorderWidth(1, 0, 0, 0).BorderColor(p.line).Background(p.bg2).Children(func() {
			st := &a.chat
			mg.Column(c).Radius(12).Background(p.panel).Border(1, p.line2).Padding(10, 12, 10, 12).Gap(8).Children(func() {
				in := mg.TextArea(c.Key("chat-draft"), &st.draft).
					Placeholder(locales.ChatPlaceholder()).Label(locales.ChatPlaceholder()).
					Lines(2, 6).FillWidth()
				mg.Row(c).AlignItems(mg.Center).Children(func() {
					mg.Spacer(c)
					send := button(c, locales.ChatSend(), btnOpt{kind: btnPrimary, small: true, icon: icSend}).
						Disabled(st.busy || strings.TrimSpace(st.draft) == "")
					if (send.Clicked() || in.Submitted()) && !st.busy && strings.TrimSpace(st.draft) != "" {
						a.chatSend()
					}
				})
			})
		})
	})
}

func (a *app) chatNew() {
	a.chat.draft = ""
	a.chat.msgs = nil
	a.chat.busy = false
}

func (a *app) chatBubble(c *mg.Context, m chatMsg) {
	p := pal(c)
	mine := m.Role == chatUser
	err := m.Role == chatError

	name := locales.ChatPinky()
	nameCol := p.ink3
	body := p.ink
	switch {
	case mine:
		name = locales.ChatYou()
	case err:
		body = p.red
	}

	mg.Column(c).Gap(6).FillWidth().Children(func() {
		mg.Text(c, name).FontSize(12).FontWeight(650).TextColor(nameCol)
		if mine {
			mg.Column(c).AlignSelf(mg.Stretch).Padding(10, 12).Radius(10).Background(p.bg3).Children(func() {
				mg.Text(c, m.Text).FontSize(14).TextColor(body)
			})
		} else {
			mg.Text(c, m.Text).FontSize(14).TextColor(body)
		}
	})
}

func (a *app) chatSend() {
	st := &a.chat
	text := strings.TrimSpace(st.draft)
	if text == "" || st.busy {
		return
	}
	prov := inapp.Provider(a.settings.ModelProvider)
	if !inapp.Valid(prov) {
		prov = inapp.Anthropic
	}
	if !a.model.has[prov] {
		a.refreshModelKeys()
	}
	if !a.model.has[prov] {
		st.msgs = append(st.msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
		return
	}
	st.draft = ""
	st.msgs = append(st.msgs, chatMsg{Role: chatUser, Text: text, At: time.Now()})
	st.busy = true

	history := make([]inapp.Message, 0, len(st.msgs))
	for _, m := range st.msgs {
		switch m.Role {
		case chatUser:
			history = append(history, inapp.Message{Role: "user", Text: m.Text})
		case chatAssistant:
			history = append(history, inapp.Message{Role: "assistant", Text: m.Text})
		}
	}
	phones := a.chatScopeNames()
	system := "You are Pinky's in-app chat (a fork experiment). Be concise. The sidebar selection is in scope: " +
		strings.Join(phones, ", ") +
		". You cannot operate phones yet; say what you would do when asked to act on them."

	store := a.secrets()
	account := inapp.KeyAccount(prov)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		key, err := store.Reveal(ctx, account, locales.SettingsSectionModel())
		var reply string
		if err == nil {
			reply, err = inapp.Complete(ctx, prov, key, system, history)
		}
		a.post(func() {
			st.busy = false
			switch {
			case errors.Is(err, secret.ErrCancelled):
			case errors.Is(err, secret.ErrNotFound):
				a.refreshModelKeys()
				st.msgs = append(st.msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
			case err != nil:
				st.msgs = append(st.msgs, chatMsg{Role: chatError, Text: locales.ChatErrReply(err.Error()), At: time.Now()})
			default:
				st.msgs = append(st.msgs, chatMsg{Role: chatAssistant, Text: reply, At: time.Now()})
			}
		})
	}()
}

// chatScopeNames follows the left sidebar: All phones, or the one selected.
func (a *app) chatScopeNames() []string {
	if a.sel == allPhones {
		var names []string
		for _, d := range a.devices {
			if d.Status != gateway.DeviceOffline {
				names = append(names, d.Info.Name)
			}
		}
		if len(names) == 0 {
			return []string{"(none)"}
		}
		return names
	}
	if d := a.device(a.sel); d != nil {
		return []string{d.Info.Name}
	}
	return []string{"(none)"}
}
