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

// chatState is the phone page's right-column chat (fork experiment).
type chatState struct {
	draft   string
	msgs    []chatMsg
	busy    bool
	targets map[string]bool // empty or all true → every connected phone
}

func (a *app) chatPanel(c *mg.Context) {
	p := pal(c)
	if !a.model.ready {
		a.refreshModelKeys()
	}
	mg.Column(c).Gap(10).MinHeight(0).Grow(1).Label(locales.ChatSection()).Children(func() {
		mg.Row(c).AlignItems(mg.Center).Gap(8).Children(func() {
			mg.Icon(c, icChat).FontSize(15).TextColor(p.ink2)
			mg.Text(c, locales.ChatSection()).FontSize(13).FontWeight(600).TextColor(p.ink3)
		})
		a.chatPhoneChips(c)
		mg.Column(c).Grow(1).MinHeight(160).Radius(14).Background(p.bg2).Border(1, p.line).
			Padding(12, 12, 10, 12).Gap(10).Children(func() {
			mg.Scroll(c).Grow(1).MinHeight(0).Gap(10).Children(func() {
				if len(a.chat.msgs) == 0 {
					mg.Text(c, locales.ChatEmpty()).FontSize(13).TextColor(p.ink2)
					prov := inapp.Provider(a.settings.ModelProvider)
					if !a.model.has[prov] {
						if linkButton(c, locales.ChatNeedKeyLink()).AlignSelf(mg.Start).Clicked() {
							a.page, a.set.sec = pageSettings, secModel
						}
					}
					return
				}
				for i, m := range a.chat.msgs {
					a.chatBubble(c.Key(fmt.Sprintf("chat-%s-%d", m.Role, i)), m)
				}
				if a.chat.busy {
					mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
						spinner(c)
						mg.Text(c, locales.ChatThinking()).FontSize(12.5).TextColor(p.ink3)
					})
				}
			})
			a.chatComposer(c)
		})
	})
}

func (a *app) chatPhoneChips(c *mg.Context) {
	p := pal(c)
	online := 0
	for _, d := range a.devices {
		if d.Status != gateway.DeviceOffline {
			online++
		}
	}
	if online == 0 {
		mg.Text(c, locales.ChatNoPhones()).FontSize(12).TextColor(p.ink3)
		return
	}
	mg.Column(c).Gap(6).Children(func() {
		mg.Text(c, locales.ChatOnPhones()).FontSize(12).FontWeight(600).TextColor(p.ink3)
		mg.Row(c).Gap(6).Wrap().Children(func() {
			allOn := a.chatAllSelected()
			if a.chatChip(c.Key("all"), locales.ChatAllPhones(), allOn).Clicked() {
				a.chatSelectAll(!allOn)
			}
			for _, d := range a.devices {
				if d.Status == gateway.DeviceOffline {
					continue
				}
				d := d
				on := a.chat.targets[d.Info.ID]
				if a.chat.targets == nil {
					on = true
				}
				if a.chatChip(c.Key(d.Info.ID), d.Info.Name, on).Clicked() {
					a.chatToggle(d.Info.ID)
				}
			}
		})
	})
}

func (a *app) chatAllSelected() bool {
	if a.chat.targets == nil {
		return true
	}
	n := 0
	for _, d := range a.devices {
		if d.Status == gateway.DeviceOffline {
			continue
		}
		if !a.chat.targets[d.Info.ID] {
			return false
		}
		n++
	}
	return n > 0
}

func (a *app) chatSelectAll(on bool) {
	if on {
		a.chat.targets = nil
		return
	}
	a.chat.targets = map[string]bool{}
}

func (a *app) chatToggle(id string) {
	if a.chat.targets == nil {
		a.chat.targets = map[string]bool{}
		for _, d := range a.devices {
			if d.Status != gateway.DeviceOffline {
				a.chat.targets[d.Info.ID] = true
			}
		}
	}
	a.chat.targets[id] = !a.chat.targets[id]
}

func (a *app) chatChip(c *mg.Context, label string, on bool) mg.Element {
	p := pal(c)
	b := mg.ButtonBase(c).Height(26).Padding(0, 10).Radius(999).Label(label)
	bg, fg, border := p.bg3, p.ink2, p.line2
	if on {
		bg, fg, border = p.sel, p.ink, p.accentBtn.Alpha(0.35)
	} else if b.Hovered() {
		bg, fg = p.bg2, p.ink
	}
	b.Background(bg).Border(1, border).TextColor(fg).Children(func() {
		mg.Text(c, label).FontSize(12).FontWeight(500).NoWrap()
	})
	return b
}

func (a *app) chatBubble(c *mg.Context, m chatMsg) {
	p := pal(c)
	mine := m.Role == chatUser
	err := m.Role == chatError
	align := mg.Start
	if mine {
		align = mg.End
	}
	label := locales.ChatPinky()
	bg, fg := p.panel, p.ink
	border := p.line
	switch {
	case mine:
		label, bg, border = locales.ChatYou(), p.sel, p.accentBtn.Alpha(0.25)
	case err:
		label, bg, fg, border = locales.ChatPinky(), p.redTint, p.red, p.redLine
	}
	mg.Column(c).Gap(4).AlignItems(align).MaxWidth(280).Children(func() {
		mg.Text(c, label).FontSize(11).FontWeight(600).TextColor(p.ink3)
		mg.Column(c).Padding(10, 12).Radius(12).Background(bg).Border(1, border).Children(func() {
			mg.Text(c, m.Text).FontSize(13.5).TextColor(fg)
		})
	})
}

func (a *app) chatComposer(c *mg.Context) {
	p := pal(c)
	st := &a.chat
	mg.Row(c).Gap(8).AlignItems(mg.End).Children(func() {
		in := mg.TextArea(c.Key("chat-draft"), &st.draft).
			Placeholder(locales.ChatPlaceholder()).Label(locales.ChatPlaceholder()).
			Lines(2, 5).Grow(1).MinWidth(0)
		send := button(c, "", btnOpt{kind: btnPrimary, icon: icSend, label: locales.ChatSend(), small: true}).
			Width(34).Padding(0).Disabled(st.busy || strings.TrimSpace(st.draft) == "")
		if (send.Clicked() || in.Submitted()) && !st.busy && strings.TrimSpace(st.draft) != "" {
			a.chatSend()
		}
		_ = p
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
	phones := a.chatTargetNames()
	system := "You are Pinky's in-app chat (a fork experiment). Be concise. The person is looking at connected phones: " +
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
				// person dismissed the prompt; drop the pending feel quietly
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

func (a *app) chatTargetNames() []string {
	var names []string
	for _, d := range a.devices {
		if d.Status == gateway.DeviceOffline {
			continue
		}
		if a.chat.targets != nil && !a.chat.targets[d.Info.ID] {
			continue
		}
		names = append(names, d.Info.Name)
	}
	if len(names) == 0 {
		return []string{"(none)"}
	}
	return names
}
