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

// chatState is the far-right Agent-style column. Scope follows the sidebar.
type chatState struct {
	draft string
	msgs  []chatMsg
	busy  bool
}

func (a *app) chatColumn(c *mg.Context) {
	p := pal(c)
	if !a.model.ready {
		a.refreshModelKeys()
	}
	mg.Column(c).Width(380).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(p.line).Background(p.bg).
		MinHeight(0).Label(locales.ChatSection()).Children(func() {
		a.chatHeader(c)
		a.chatThread(c)
		a.chatComposer(c)
	})
}

func (a *app) chatHeader(c *mg.Context) {
	p := pal(c)
	mg.Row(c).Height(48).Shrink(0).Padding(0, 12, 0, 16).AlignItems(mg.Center).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(p.line).Children(func() {
		mg.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			mg.Text(c, locales.ChatSection()).FontSize(13).FontWeight(650).SingleLine()
			mg.Text(c, a.chatScopeLabel()).FontSize(11.5).TextColor(p.ink3).SingleLine()
		})
		empty := len(a.chat.msgs) == 0 && strings.TrimSpace(a.chat.draft) == ""
		off := a.chat.busy || empty
		b := mg.ButtonBase(c).Size(30, 30).Radius(8).Center().
			Label(locales.ChatNew()).Tooltip(locales.ChatNew()).Disabled(off)
		col := p.ink2
		if off {
			col = p.ink3
		} else if b.Hovered() {
			b.Background(p.bg3)
			col = p.ink
		}
		if b.Clicked() && !off {
			a.chatNew()
		}
		b.Children(func() {
			mg.Icon(c, icPlus).FontSize(16).TextColor(col)
		})
	})
}

func (a *app) chatScopeLabel() string {
	if a.sel == allPhones {
		return locales.ChatScopeAll()
	}
	if d := a.device(a.sel); d != nil {
		return locales.ChatScopeOne(d.Info.Name)
	}
	return locales.ChatScopeNone()
}

func (a *app) chatThread(c *mg.Context) {
	p := pal(c)
	mg.Scroll(c).Grow(1).MinHeight(0).Padding(18, 16).Gap(16).Children(func() {
		if len(a.chat.msgs) == 0 && !a.chat.busy {
			a.chatEmpty(c)
			return
		}
		for i, m := range a.chat.msgs {
			a.chatBubble(c.Key(fmt.Sprintf("chat-%s-%d", m.Role, i)), m)
		}
		if a.chat.busy {
			mg.Row(c).Gap(8).AlignItems(mg.Center).Padding(2, 4).Children(func() {
				mg.Box(c).Size(22, 22).Radius(11).Background(p.accentTint).Center().Children(func() {
					mg.Icon(c, icChat).FontSize(12).TextColor(p.accentInk)
				})
				mg.Text(c, locales.ChatThinking()).FontSize(13).TextColor(p.ink3)
			})
		}
	})
}

func (a *app) chatEmpty(c *mg.Context) {
	p := pal(c)
	mg.Column(c).Grow(1).MinHeight(240).Center().Gap(14).Padding(12, 8).Children(func() {
		mg.Box(c).Size(44, 44).Radius(14).Background(p.bg2).Border(1, p.line).Center().Children(func() {
			mg.Icon(c, icChat).FontSize(20).TextColor(p.ink2)
		})
		mg.Column(c).Gap(6).AlignItems(mg.Center).MaxWidth(280).Children(func() {
			mg.Text(c, locales.ChatEmptyTitle()).FontSize(15).FontWeight(650).TextAlign(mg.Center)
			mg.Text(c, locales.ChatEmpty()).FontSize(13).TextColor(p.ink2).TextAlign(mg.Center)
		})
		prov := inapp.Provider(a.settings.ModelProvider)
		if !a.model.has[prov] {
			if linkButton(c, locales.ChatNeedKeyLink()).Clicked() {
				a.page, a.set.sec = pageSettings, secModel
			}
		}
	})
}

func (a *app) chatComposer(c *mg.Context) {
	p := pal(c)
	st := &a.chat
	canSend := !st.busy && strings.TrimSpace(st.draft) != ""

	mg.Column(c).Shrink(0).Padding(10, 12, 14, 12).Background(p.bg).Children(func() {
		// One input shell — Cursor-like, not a nested attachment card.
		mg.Row(c).Gap(8).Padding(8, 10).Radius(14).AlignItems(mg.End).
			Background(p.bg2).Border(1, p.line2).Children(func() {
			in := mg.TextArea(c.Key("chat-draft"), &st.draft).
				Placeholder(locales.ChatPlaceholder()).Label(locales.ChatPlaceholder()).
				Lines(1, 5).Grow(1).MinWidth(0)
			send := mg.ButtonBase(c).Size(32, 32).Radius(10).Center().
				Label(locales.ChatSend()).Disabled(!canSend)
			bg, fg := p.bg3, p.ink3
			if canSend {
				bg, fg = p.accentBtn, p.white
			}
			if send.Hovered() && canSend {
				bg = bg.Mix(mg.RGB(0, 0, 0), 0.1)
			}
			send.Background(bg).Children(func() {
				mg.Icon(c, icSend).FontSize(15).TextColor(fg)
			})
			if (send.Clicked() || in.Submitted()) && canSend {
				a.chatSend()
			}
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
	switch m.Role {
	case chatUser:
		mg.Row(c).Justify(mg.End).Children(func() {
			mg.Column(c).MaxWidth(300).Padding(10, 12).Radius(14).Background(p.sel).Children(func() {
				mg.Text(c, m.Text).FontSize(13.5).TextColor(p.ink)
			})
		})
	case chatError:
		mg.Row(c).Gap(10).AlignItems(mg.Start).Children(func() {
			mg.Box(c).Size(22, 22).Radius(11).Background(p.redTint).Border(1, p.redLine).Center().Shrink(0).Children(func() {
				mg.Icon(c, icAlert).FontSize(12).TextColor(p.red)
			})
			mg.Text(c, m.Text).FontSize(13.5).TextColor(p.red).Grow(1).MinWidth(0)
		})
	default:
		mg.Row(c).Gap(10).AlignItems(mg.Start).Children(func() {
			mg.Box(c).Size(22, 22).Radius(11).Background(p.accentTint).Center().Shrink(0).Children(func() {
				mg.Icon(c, icChat).FontSize(12).TextColor(p.accentInk)
			})
			mg.Text(c, m.Text).FontSize(13.5).TextColor(p.ink).Grow(1).MinWidth(0)
		})
	}
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
	system := "You are Pinky's in-app assistant. Be concise and practical. The person selected: " +
		strings.Join(phones, ", ") +
		". You cannot control phones yet; if asked to act, briefly say what you would do."

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
