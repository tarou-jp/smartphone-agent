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

type chatSession struct {
	ID      string
	Title   string
	Msgs    []chatMsg
	Updated time.Time
}

// chatState mirrors Cursor's agent sidebar: tabs of chats, history, composer.
type chatState struct {
	sessions []chatSession
	active   string
	draft    string
	busy     bool
	history  bool // history list is open
	seq      int
}

func (a *app) chatEnsure() *chatSession {
	st := &a.chat
	if st.active != "" {
		for i := range st.sessions {
			if st.sessions[i].ID == st.active {
				return &st.sessions[i]
			}
		}
	}
	st.seq++
	id := fmt.Sprintf("c%d", st.seq)
	st.sessions = append([]chatSession{{
		ID: id, Title: locales.ChatUntitled(), Updated: time.Now(),
	}}, st.sessions...)
	st.active = id
	return &st.sessions[0]
}

func (a *app) chatColumn(c *mg.Context) {
	if !a.model.ready {
		a.refreshModelKeys()
	}
	a.chatEnsure()
	p := pal(c)
	mg.Column(c).Width(400).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(p.line).Background(p.bg).
		MinHeight(0).Label(locales.ChatSection()).Children(func() {
		a.chatHeader(c)
		if a.chat.history {
			a.chatHistory(c)
		} else {
			a.chatThread(c)
			a.chatComposer(c)
		}
	})
}

func (a *app) chatHeader(c *mg.Context) {
	p := pal(c)
	s := a.chatEnsure()
	mg.Row(c).Height(44).Shrink(0).Padding(0, 10, 0, 10).AlignItems(mg.Center).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(p.line).Children(func() {
		// Active chat as a tab (Cursor-style)
		tab := mg.Row(c).Height(28).Padding(0, 10).Radius(8).Gap(8).AlignItems(mg.Center).
			Background(p.bg3).MaxWidth(220)
		tab.Children(func() {
			mg.Text(c, s.Title).FontSize(12.5).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
		})

		mg.Spacer(c)

		a.chatIconBtn(c, "new", icPlus, locales.ChatNew(), !a.chat.busy, func() {
			a.chatNew()
		})
		a.chatIconBtn(c, "history", icHistory, locales.ChatHistory(), true, func() {
			a.chat.history = !a.chat.history
		})
	})
}

func (a *app) chatIconBtn(c *mg.Context, key string, ic *mg.SVG, tip string, enabled bool, fn func()) {
	p := pal(c)
	b := mg.ButtonBase(c.Key(key)).Size(28, 28).Radius(7).Center().Label(tip).Tooltip(tip).Disabled(!enabled)
	col := p.ink2
	on := false
	if key == "history" && a.chat.history {
		on = true
	}
	switch {
	case !enabled:
		col = p.ink3
	case on:
		b.Background(p.sel)
		col = p.ink
	case b.Hovered():
		b.Background(p.bg3)
		col = p.ink
	}
	if b.Clicked() && enabled {
		fn()
	}
	b.Children(func() {
		mg.Icon(c, ic).FontSize(15).TextColor(col)
	})
}

func (a *app) chatHistory(c *mg.Context) {
	p := pal(c)
	mg.Column(c).Grow(1).MinHeight(0).Padding(12, 10).Gap(6).Children(func() {
		mg.Text(c, locales.ChatHistory()).FontSize(12).FontWeight(650).TextColor(p.ink3).Padding(4, 6, 8, 6)
		if len(a.chat.sessions) == 0 {
			mg.Text(c, locales.ChatHistoryEmpty()).FontSize(13).TextColor(p.ink2).Padding(8, 6)
			return
		}
		mg.Scroll(c).Grow(1).MinHeight(0).Gap(2).Children(func() {
			for i := range a.chat.sessions {
				s := a.chat.sessions[i]
				on := s.ID == a.chat.active
				row := mg.ButtonBase(c.Key(s.ID)).Height(40).Padding(0, 10).Radius(8).
					AlignItems(mg.Center).Justify(mg.Start).Label(s.Title)
				if on {
					row.Background(p.sel)
				} else if row.Hovered() {
					row.Background(p.bg3)
				}
				if row.Clicked() {
					a.chat.active = s.ID
					a.chat.draft = ""
					a.chat.history = false
				}
				row.Children(func() {
					mg.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
						mg.Text(c, s.Title).FontSize(13).FontWeight(500).SingleLine()
						mg.Text(c, chatWhen(s.Updated)).FontSize(11).TextColor(p.ink3).SingleLine()
					})
				})
			}
		})
	})
}

func chatWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return t.Format("1/2 15:04")
	}
}

func (a *app) chatThread(c *mg.Context) {
	p := pal(c)
	s := a.chatEnsure()
	mg.Scroll(c).Grow(1).MinHeight(0).Padding(16, 14).Gap(14).Children(func() {
		if len(s.Msgs) == 0 && !a.chat.busy {
			a.chatEmpty(c)
			return
		}
		for i, m := range s.Msgs {
			a.chatBubble(c.Key(fmt.Sprintf("%s-%s-%d", s.ID, m.Role, i)), m)
		}
		if a.chat.busy {
			mg.Text(c, locales.ChatThinking()).FontSize(13).TextColor(p.ink3).Padding(4, 2)
		}
	})
}

func (a *app) chatEmpty(c *mg.Context) {
	p := pal(c)
	mg.Column(c).Grow(1).MinHeight(220).Center().Gap(10).Padding(20, 16).Children(func() {
		mg.Text(c, locales.ChatEmptyTitle()).FontSize(16).FontWeight(650).TextAlign(mg.Center)
		mg.Text(c, locales.ChatEmpty()).FontSize(13).TextColor(p.ink2).TextAlign(mg.Center).MaxWidth(300)
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

	mg.Column(c).Shrink(0).Padding(10, 12, 12, 12).Gap(8).Background(p.bg).Children(func() {
		// Context strip (Cursor's “N Files” row → selected phones)
		mg.Row(c).Height(28).Padding(0, 2).AlignItems(mg.Center).Gap(8).Children(func() {
			chip := mg.Row(c).Height(24).Padding(0, 8).Radius(6).Gap(6).AlignItems(mg.Center).
				Background(p.bg2).Border(1, p.line)
			chip.Children(func() {
				mg.Text(c, "›").FontSize(12).TextColor(p.ink3)
				mg.Text(c, a.chatScopeLabel()).FontSize(12).FontWeight(500).TextColor(p.ink2).SingleLine()
			})
		})

		// Composer card: textarea on top, toolbar inside bottom (Cursor layout)
		mg.Column(c).Radius(16).Background(p.bg2).Border(1, p.line2).Padding(12, 12, 10, 12).Gap(10).Children(func() {
			in := mg.TextArea(c.Key("chat-draft"), &st.draft).
				Placeholder(locales.ChatPlaceholder()).Label(locales.ChatPlaceholder()).
				Lines(3, 8).FillWidth()

			mg.Row(c).AlignItems(mg.Center).Gap(8).Children(func() {
				// Mode pill
				pill := mg.Row(c).Height(26).Padding(0, 10).Radius(999).Gap(6).AlignItems(mg.Center).
					Background(p.bg3).Border(1, p.line)
				pill.Children(func() {
					mg.Text(c, locales.ChatAgentMode()).FontSize(12).FontWeight(600).TextColor(p.ink)
				})
				mg.Spacer(c)
				// Send — round up-arrow like Cursor
				send := mg.ButtonBase(c).Size(28, 28).Radius(14).Center().
					Label(locales.ChatSend()).Disabled(!canSend)
				bg, fg := p.line2, p.ink3
				if canSend {
					if p.dark {
						bg, fg = p.white, hex("#1c1c1f")
					} else {
						bg, fg = p.ink, p.white
					}
				}
				send.Background(bg).Children(func() {
					mg.Icon(c, icSend).FontSize(14).TextColor(fg)
				})
				if (send.Clicked() || in.Submitted()) && canSend {
					a.chatSend()
				}
			})
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

func (a *app) chatNew() {
	if a.chat.busy {
		return
	}
	// Keep current session in history; start a fresh empty one at the front.
	a.chat.seq++
	id := fmt.Sprintf("c%d", a.chat.seq)
	a.chat.sessions = append([]chatSession{{
		ID: id, Title: locales.ChatUntitled(), Updated: time.Now(),
	}}, a.chat.sessions...)
	a.chat.active = id
	a.chat.draft = ""
	a.chat.history = false
}

func (a *app) chatBubble(c *mg.Context, m chatMsg) {
	p := pal(c)
	switch m.Role {
	case chatUser:
		mg.Row(c).Justify(mg.End).Children(func() {
			bg := p.bg3
			if p.dark {
				bg = hex("#2f2f35")
			}
			mg.Column(c).MaxWidth(320).Padding(10, 12).Radius(12).Background(bg).Children(func() {
				mg.Text(c, m.Text).FontSize(13.5).TextColor(p.ink)
			})
		})
	case chatError:
		mg.Text(c, m.Text).FontSize(13.5).TextColor(p.red)
	default:
		mg.Text(c, m.Text).FontSize(13.5).TextColor(p.ink)
	}
}

func (a *app) chatSend() {
	st := &a.chat
	s := a.chatEnsure()
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
		s.Msgs = append(s.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
		s.Updated = time.Now()
		return
	}
	st.draft = ""
	s.Msgs = append(s.Msgs, chatMsg{Role: chatUser, Text: text, At: time.Now()})
	if s.Title == locales.ChatUntitled() || s.Title == "" {
		s.Title = chatTitleFrom(text)
	}
	s.Updated = time.Now()
	st.busy = true
	// Move active session to front of history.
	a.chatBumpActive()

	history := make([]inapp.Message, 0, len(s.Msgs))
	for _, m := range s.Msgs {
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
	sessionID := s.ID

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
			sess := a.chatSession(sessionID)
			if sess == nil {
				return
			}
			switch {
			case errors.Is(err, secret.ErrCancelled):
			case errors.Is(err, secret.ErrNotFound):
				a.refreshModelKeys()
				sess.Msgs = append(sess.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
			case err != nil:
				sess.Msgs = append(sess.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrReply(err.Error()), At: time.Now()})
			default:
				sess.Msgs = append(sess.Msgs, chatMsg{Role: chatAssistant, Text: reply, At: time.Now()})
			}
			sess.Updated = time.Now()
		})
	}()
}

func (a *app) chatSession(id string) *chatSession {
	for i := range a.chat.sessions {
		if a.chat.sessions[i].ID == id {
			return &a.chat.sessions[i]
		}
	}
	return nil
}

func (a *app) chatBumpActive() {
	st := &a.chat
	var cur *chatSession
	rest := make([]chatSession, 0, len(st.sessions))
	for i := range st.sessions {
		if st.sessions[i].ID == st.active {
			c := st.sessions[i]
			cur = &c
			continue
		}
		rest = append(rest, st.sessions[i])
	}
	if cur == nil {
		return
	}
	st.sessions = append([]chatSession{*cur}, rest...)
}

func chatTitleFrom(prompt string) string {
	p := strings.Join(strings.Fields(prompt), " ")
	r := []rune(p)
	if len(r) > 28 {
		return string(r[:27]) + "…"
	}
	if p == "" {
		return "Chat"
	}
	return p
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
