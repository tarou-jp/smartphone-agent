package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/cmd/pinky/locales"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/inapp"
	"github.com/DIYgod/pinky/internal/secret"
)

const chatPanelWidth = 380

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
	queued  []string
}

// chatState is the Agent column on the phone page (Cursor-style sidebar).
type chatState struct {
	sessions  []chatSession
	active    string
	draft     string
	busy      bool
	history   bool
	collapsed bool
	seq       int
	scrollTab bool // one frame: ScrollIntoView on the active tab
	genCancel context.CancelFunc
	interruptNext string // Enter while busy: stop and run this message next
	queueOpen     bool   // composer queue list expanded
	threadScroll  mg.ScrollState
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
	if len(st.sessions) > 0 {
		st.active = st.sessions[0].ID
		return &st.sessions[0]
	}
	st.seq++
	id := fmt.Sprintf("c%d", st.seq)
	s := chatSession{ID: id, Title: locales.ChatUntitled(), Updated: time.Now()}
	st.sessions = append(st.sessions, s)
	st.active = id
	return &st.sessions[len(st.sessions)-1]
}

func (a *app) chatColumn(c *mg.Context) {
	if !a.model.ready {
		a.refreshModelKeys()
	}
	p := pal(c)
	if a.chat.collapsed {
		a.chatRail(c)
		return
	}
	a.chatEnsure()
	mg.Column(c).Width(chatPanelWidth).Shrink(0).MinHeight(0).
		Background(p.bg).BorderWidth(0, 0, 0, 1).BorderColor(p.line).
		Label(locales.ChatSection()).Children(func() {
		a.chatHeader(c)
		if a.chat.history {
			a.chatHistory(c)
		} else {
			a.chatThread(c)
		}
		a.chatComposer(c)
	})
}

// chatRail is the narrow strip when the Agent panel is hidden (reopen here).
func (a *app) chatRail(c *mg.Context) {
	p := pal(c)
	mg.Column(c).Width(48).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(p.line).Background(p.bg).
		Padding(8, 6, 8, 6).Gap(6).Label(locales.ChatSection()).Children(func() {
		a.chatIconBtn(c, "expand", icChevL, locales.ChatExpand(), true, func() {
			a.chat.collapsed = false
		})
		mg.Spacer(c)
		open := mg.ButtonBase(c.Key("rail-chat")).Size(36, 36).Radius(8).Center().
			Label(locales.ChatSection()).Tooltip(locales.ChatExpand())
		if open.Hovered() {
			open.Background(p.bg3)
		}
		if open.Clicked() {
			a.chat.collapsed = false
		}
		open.Children(func() {
			mg.Icon(c, icChat).FontSize(18).TextColor(p.ink2)
		})
	})
}

func (a *app) chatHeader(c *mg.Context) {
	p := pal(c)
	a.chatEnsure()
	reveal := a.chat.scrollTab
	if reveal {
		a.chat.scrollTab = false
	}
	tabs := append([]chatSession(nil), a.chat.sessions...)
	mg.Row(c).Height(36).Shrink(0).AlignItems(mg.Stretch).Gap(0).
		Background(p.bg2).BorderWidth(0, 0, 1, 0).BorderColor(p.line).Children(func() {
		mg.ScrollHorizontal(c.Key("tabs")).Grow(1).MinWidth(0).Height(36).Scrollbars(true).
			BorderWidth(0, 1, 0, 0).BorderColor(p.line).Children(func() {
			for _, s := range tabs {
				a.chatTab(c.Key(s.ID), s, reveal && s.ID == a.chat.active)
			}
		})
		mg.Row(c).Height(36).Shrink(0).Padding(0, 6).Gap(2).AlignItems(mg.Center).Children(func() {
			a.chatIconBtn(c, "new", icPlus, locales.ChatNew(), !a.chat.busy, func() {
				a.chatNew()
			})
			a.chatIconBtn(c, "history", icHistory, locales.ChatHistory(), true, func() {
				a.chat.history = !a.chat.history
			})
			a.chatIconBtn(c, "close", icPanelRight, locales.ChatCollapse(), true, func() {
				a.chat.collapsed = true
			})
		})
	})
}

func (a *app) chatTab(c *mg.Context, s chatSession, reveal bool) {
	p := pal(c)
	on := s.ID == a.chat.active
	tab := mg.ButtonBase(c).Height(36).Padding(0, 10).Gap(6).AlignItems(mg.Center).
		Label(s.Title).MaxWidth(168).Shrink(0).
		BorderWidth(0, 1, 0, 0).BorderColor(p.line)
	if on {
		tab.Background(p.bg).Margin(0, 0, -1, 0)
	} else if tab.Hovered() {
		tab.Background(p.bg3)
	}
	if reveal {
		tab.ScrollIntoView()
	}
	if tab.Clicked() {
		a.chat.active = s.ID
		a.chat.draft = ""
		a.chat.scrollTab = true
	}
	tab.Children(func() {
		iconCol, textCol := p.ink3, p.ink3
		if on {
			iconCol, textCol = p.ink, p.ink
		}
		mg.Icon(c, icChat).FontSize(14).TextColor(iconCol)
		mg.Text(c, s.Title).FontSize(12.5).FontWeight(500).TextColor(textCol).SingleLine().Grow(1).MinWidth(0)
		if on && len(a.chat.sessions) > 1 {
			x := mg.ButtonBase(c.Key("x")).Size(20, 20).Radius(5).Center().
				Label(locales.ChatCloseTab()).Tooltip(locales.ChatCloseTab())
			if x.Hovered() {
				x.Background(p.bg3)
			}
			if x.Clicked() {
				a.chatCloseSession(s.ID)
				a.chat.scrollTab = true
			}
			x.Children(func() {
				mg.Icon(c, icX).FontSize(12).TextColor(p.ink2)
			})
		}
	})
}

func (a *app) chatCloseSession(id string) {
	st := &a.chat
	if st.busy || len(st.sessions) <= 1 {
		return
	}
	out := make([]chatSession, 0, len(st.sessions)-1)
	for _, s := range st.sessions {
		if s.ID != id {
			out = append(out, s)
		}
	}
	st.sessions = out
	if st.active == id {
		st.active = st.sessions[0].ID
		st.draft = ""
	}
}

func (a *app) chatIconBtn(c *mg.Context, key string, ic *mg.SVG, tip string, enabled bool, fn func()) {
	p := pal(c)
	b := mg.ButtonBase(c.Key(key)).Size(28, 28).Radius(6).Center().Label(tip).Tooltip(tip).Disabled(!enabled)
	col := p.ink2
	on := key == "history" && a.chat.history
	switch {
	case !enabled:
		col = p.ink3
	case on:
		b.Background(p.bg3)
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
					a.chat.scrollTab = true
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
	scroll := &a.chat.threadScroll
	a.chatFollowThread(scroll)
	mg.Scroll(c.Key("chat-thread")).Grow(1).MinHeight(0).TrackScroll(scroll).
		Padding(16, 14).Gap(14).Children(func() {
		if len(s.Msgs) == 0 && !a.chat.busy {
			a.chatEmpty(c)
			return
		}
		for i, m := range s.Msgs {
			a.chatBubble(c.Key(fmt.Sprintf("%s-%s-%d", s.ID, m.Role, i)), m)
		}
		if a.chat.busy && !chatHasStreamingTail(s) {
			mg.Text(c.Key("chat-thinking"), locales.ChatThinking()).FontSize(13).TextColor(p.ink3).Padding(4, 2)
		}
	})
}

// chatFollowThread keeps the thread pinned to the bottom while the person is
// already at the bottom (Cursor-style), so streaming does not fight manual scroll-up.
func (a *app) chatFollowThread(scroll *mg.ScrollState) {
	const slack float32 = 28
	atBottom := scroll.MaxY == 0 || scroll.Y >= scroll.MaxY-slack
	if atBottom {
		scroll.Y = math.MaxFloat32
	}
}

func chatHasStreamingTail(s *chatSession) bool {
	if s == nil || len(s.Msgs) == 0 {
		return false
	}
	return s.Msgs[len(s.Msgs)-1].Role == chatAssistant
}

func (a *app) chatAppendStream(sessionID, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	sess := a.chatSession(sessionID)
	if sess == nil {
		return
	}
	if n := len(sess.Msgs); n > 0 && sess.Msgs[n-1].Role == chatAssistant {
		prev := strings.TrimSpace(sess.Msgs[n-1].Text)
		if prev == "" {
			sess.Msgs[n-1].Text = line
		} else {
			sess.Msgs[n-1].Text = prev + "\n" + line
		}
	} else {
		sess.Msgs = append(sess.Msgs, chatMsg{Role: chatAssistant, Text: line, At: time.Now()})
	}
	sess.Updated = time.Now()
}

func (a *app) chatFinishStream(sessionID, reply string) {
	sess := a.chatSession(sessionID)
	if sess == nil {
		return
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}
	if n := len(sess.Msgs); n > 0 && sess.Msgs[n-1].Role == chatAssistant {
		body := strings.TrimSpace(sess.Msgs[n-1].Text)
		if body == "" {
			sess.Msgs[n-1].Text = reply
			return
		}
		if body == reply || strings.Contains(body, reply) {
			return
		}
		sess.Msgs[n-1].Text = body + "\n\n" + reply
		return
	}
	sess.Msgs = append(sess.Msgs, chatMsg{Role: chatAssistant, Text: reply, At: time.Now()})
}

func (a *app) chatEmpty(c *mg.Context) {
	p := pal(c)
	mg.Column(c).Grow(1).MinHeight(200).Center().Gap(8).Padding(20, 12).Children(func() {
		mg.Text(c, locales.ChatEmpty()).FontSize(13).TextColor(p.ink2).TextAlign(mg.Center).MaxWidth(300)
		if !a.modelAnyKey() {
			if linkButton(c, locales.ChatNeedKeyLink()).Clicked() {
				a.page, a.set.sec = pageSettings, secModel
			}
		}
	})
}

func (a *app) chatComposer(c *mg.Context) {
	p := pal(c)
	st := &a.chat
	a.ensureChatModel()
	draftOK := strings.TrimSpace(st.draft) != "" && a.chatModelReady()
	canSend := !st.busy && draftOK
	canQueue := st.busy && draftOK
	fill := p.bg2
	if p.dark {
		fill = hex("#252528")
	}

	mg.Column(c).Shrink(0).Padding(10, 12, 14, 12).Children(func() {
		s := a.chatEnsure()
		hasToolbar := len(s.queued) > 0 || st.busy
		ph := locales.ChatPlaceholder()
		if st.busy {
			ph = locales.ChatPlaceholderFollowUp()
		}
		// One bordered region; composer toolbar (queue + Stop) like Cursor, then the input.
		box := mg.Column(c).Radius(12).Border(1, p.line2).Background(fill).Gap(0)
		box.Children(func() {
			if hasToolbar {
				a.chatComposerToolbar(c, p)
				mg.Box(c).Height(1).FillWidth().Background(p.line2)
			}
			mg.Column(c).Padding(14, 14, 10, 14).Gap(4).Children(func() {
			_ = mg.TextAreaBase(c.Key("chat-draft"), &st.draft).
				Placeholder(ph).Label(ph).
				Lines(2, 12).FillWidth().Background(fill).MinHeight(44).
				OnTextKey(func(k mg.TextKey) bool {
					if k.Key != mg.KeyEnter {
						return false
					}
					if k.Mods&mg.Shift != 0 {
						return false // Shift+Enter: newline.
					}
					text := strings.TrimSpace(k.Text())
					if text == "" || !a.chatModelReady() {
						return false
					}
					k.SetText("")
					st.draft = ""
					a.chatComposerEnter(text, chatEnterQueues(k.Mods))
					return true
				})
			mg.Row(c).AlignItems(mg.Center).Gap(6).Children(func() {
				a.chatModelPicker(c, fill)
				mg.Spacer(c)
				// One FAB slot: stop while generating, send otherwise (queue when busy + draft).
				showStop := st.busy && !canQueue
				fabActive := showStop || canSend || canQueue
				bg, fg := chatComposerFabColors(p, fabActive)
				if showStop {
					stop := mg.ButtonBase(c.Key("chat-stop")).Size(26, 26).Radius(13).Center().
						Label(locales.ChatStop()).Background(bg)
					stop.Children(func() {
						mg.Icon(c, icStopSq).FontSize(16).TextColor(fg)
					})
					if stop.Clicked() {
						a.chatStop()
					}
				} else {
					send := mg.ButtonBase(c.Key("chat-send")).Size(26, 26).Radius(13).Center().
						Label(locales.ChatSend()).Background(bg).Disabled(!canSend && !canQueue)
					send.Children(func() {
						mg.Icon(c, icArrowUp).FontSize(14).TextColor(fg)
					})
					if send.Clicked() && (canSend || canQueue) {
						text := strings.TrimSpace(st.draft)
						st.draft = ""
						a.chatComposerEnter(text, false)
					}
				}
			})
			})
		})
	})
}

// chatComposerFabColors styles the round send/stop FAB like Cursor's composer.
func chatComposerFabColors(p *palette, active bool) (bg, fg mg.Color) {
	if !active {
		return p.line2, p.ink3
	}
	if p.dark {
		return p.white, hex("#1c1c1f")
	}
	return p.ink, p.white
}

// chatModelPicker is a text + ▾ menu of LLM models (keys missing → item disabled).
func (a *app) chatModelPicker(c *mg.Context, fill mg.Color) {
	p := pal(c)
	label := a.chatModelPickerLabel()
	pick := mg.ButtonBase(c.Key("chat-model")).Height(26).Padding(0, 6).Radius(8).
		Gap(4).AlignItems(mg.Center).Label(label)
	if pick.Hovered() {
		pick.Background(p.bg3)
	} else {
		pick.Background(fill)
	}
	pick.PopoverMenuButton(func(menu *mg.Menu) {
		for _, model := range a.allChatModels() {
			if !a.providerReady(model.Provider) {
				continue
			}
			if menu.Item(model.Label).Checked(a.settings.ChatModel == model.ID).Chosen() {
				a.settings.ChatModel = model.ID
				a.changed()
			}
		}
	})
	textCol := p.ink
	if !a.chatModelReady() {
		textCol = p.ink3
	}
	pick.Children(func() {
		mg.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(textCol)
		mg.Icon(c, icChevD).FontSize(12).TextColor(p.ink3)
	})
}

func (a *app) chatModelPickerLabel() string {
	if m := a.activeChatModel(); m != nil {
		return m.Label
	}
	return locales.ChatModelNone()
}

func (a *app) chatNew() {
	if a.chat.busy {
		return
	}
	a.chat.seq++
	id := fmt.Sprintf("c%d", a.chat.seq)
	a.chat.sessions = append(a.chat.sessions, chatSession{
		ID: id, Title: locales.ChatUntitled(), Updated: time.Now(),
	})
	a.chat.active = id
	a.chat.draft = ""
	a.chat.history = false
	a.chat.scrollTab = true
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

func (a *app) chatStop() {
	st := &a.chat
	if !st.busy || st.genCancel == nil {
		return
	}
	st.genCancel()
}

// chatEnterQueues is Cmd/Ctrl+Enter (queue); plain Enter interrupts when busy.
func chatEnterQueues(mods mg.Modifiers) bool {
	return mods&(mg.Cmd|mg.Ctrl) != 0
}

func (a *app) chatComposerEnter(text string, queue bool) {
	text = strings.TrimSpace(text)
	if text == "" || !a.chatModelReady() {
		return
	}
	st := &a.chat
	if st.busy {
		if queue {
			a.chatEnqueue(text)
		} else {
			a.chatInterruptWith(text)
		}
		return
	}
	a.chatSendNow(text, true)
}

func (a *app) chatEnqueue(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s := a.chatEnsure()
	s.queued = append(s.queued, text)
	s.Updated = time.Now()
	a.chat.queueOpen = true
}

func (a *app) chatInterruptWith(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	st := &a.chat
	st.interruptNext = text
	if st.busy && st.genCancel != nil {
		st.genCancel()
		return
	}
	st.interruptNext = ""
	a.chatSendNow(text, true)
}

func (a *app) chatComposerToolbar(c *mg.Context, p *palette) {
	st := &a.chat
	s := a.chatEnsure()
	labelCol := p.ink2
	if p.dark {
		labelCol = hex("#9d9da6")
	}
	previewCol := p.ink2
	indBorder := p.ink.Alpha(0.4)
	mg.Column(c).Gap(1).Padding(6, 4, 4, 4).Children(func() {
		if len(s.queued) > 0 {
			if !st.queueOpen {
				st.queueOpen = true
			}
			head := mg.ButtonBase(c.Key("chat-queue-toggle")).FillWidth().Gap(4).AlignItems(mg.Center).
				Padding(2, 4).Label(locales.ChatQueueToggle())
			if head.Clicked() {
				st.queueOpen = !st.queueOpen
			}
			head.Children(func() {
				chev := icChevR
				if st.queueOpen {
					chev = icChevD
				}
				mg.Icon(c, chev).FontSize(12).TextColor(labelCol)
				mg.Text(c, locales.ChatQueueHeading(len(s.queued))).FontSize(12).TextColor(labelCol)
			})
			if st.queueOpen {
				mg.Column(c).Gap(1).Padding(0, 4).Children(func() {
					for i, line := range s.queued {
						a.chatQueueRow(c.Key(fmt.Sprintf("chat-q-%d", i)), p, i, line, indBorder, previewCol, labelCol)
					}
				})
			}
		}
		if st.busy {
			mg.Row(c).AlignItems(mg.Center).Padding(4, 8, 2, 8).Children(func() {
				mg.Spacer(c)
				stop := mg.ButtonBase(c.Key("chat-toolbar-stop")).Gap(6).AlignItems(mg.Center).
					Padding(4, 8).Radius(6).Label(locales.ChatStop())
				if stop.Hovered() {
					stop.Background(p.bg3)
				}
				if stop.Clicked() {
					a.chatStop()
				}
				stop.Children(func() {
					mg.Text(c, locales.ChatStop()).FontSize(12).TextColor(labelCol)
					mg.Text(c, locales.ChatStopKeyHint()).FontSize(12).TextColor(p.ink3)
				})
			})
		}
	})
}

func (a *app) chatQueueRow(c *mg.Context, p *palette, i int, line string, indBorder, previewCol, actionCol mg.Color) {
	row := mg.Row(c).Gap(8).AlignItems(mg.Start).Padding(2, 4, 2, 8).Radius(4).Grow(1)
	if row.Hovered() {
		row.Background(p.bg3)
	}
	row.Children(func() {
		mg.Box(c).Size(10, 10).Radius(5).Border(1, indBorder).Margin(4, 0, 0, 0).Shrink(0)
		mg.Text(c, chatQueuePreview(line)).FontSize(12).TextColor(previewCol).
			SingleLine().Grow(1).MinWidth(0)
		mg.Row(c).Gap(4).AlignItems(mg.Center).Shrink(0).Children(func() {
			steer := mg.ButtonBase(c.Key(fmt.Sprintf("chat-q-steer-%d", i))).
				Padding(2, 6).Radius(4).Label(locales.ChatQueueSteer())
			if steer.Hovered() {
				steer.Background(p.line2)
			}
			if steer.Clicked() {
				a.chatQueueSteer(i)
			}
			steer.Children(func() {
				mg.Text(c, locales.ChatQueueSteer()).FontSize(12).TextColor(actionCol)
				mg.Text(c, chatQueueSteerKeys()).FontSize(11).TextColor(p.ink3)
			})
			rm := mg.ButtonBase(c.Key(fmt.Sprintf("chat-q-rm-%d", i))).Size(24, 24).Center().
				Label(locales.ChatQueueRemove())
			if rm.Hovered() {
				rm.Background(p.line2)
			}
			if rm.Clicked() {
				a.chatQueueRemove(i)
			}
			rm.Children(func() {
				mg.Icon(c, icX).FontSize(12).TextColor(p.ink3)
			})
		})
	})
}

func chatQueueSteerKeys() string {
	if runtime.GOOS == "darwin" {
		return "⌘↩"
	}
	return "Ctrl+↩"
}

func (a *app) chatQueueSteer(i int) {
	s := a.chatEnsure()
	if i < 0 || i >= len(s.queued) {
		return
	}
	text := s.queued[i]
	s.queued = append(s.queued[:i], s.queued[i+1:]...)
	a.chatInterruptWith(text)
}

func (a *app) chatQueueRemove(i int) {
	s := a.chatEnsure()
	if i < 0 || i >= len(s.queued) {
		return
	}
	s.queued = append(s.queued[:i], s.queued[i+1:]...)
}

func chatQueuePreview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 120 {
		return string(r[:119]) + "…"
	}
	return s
}

func (a *app) chatFlushQueue() {
	st := &a.chat
	if strings.TrimSpace(st.interruptNext) != "" {
		return
	}
	s := a.chatSession(st.active)
	if st.busy || s == nil || len(s.queued) == 0 {
		return
	}
	next := s.queued[0]
	s.queued = s.queued[1:]
	a.chatSendNow(next, true)
}

func (a *app) chatAfterGeneration() {
	st := &a.chat
	if next := strings.TrimSpace(st.interruptNext); next != "" {
		st.interruptNext = ""
		a.chatSendNow(next, true)
		return
	}
	a.chatFlushQueue()
}

func (a *app) chatSendNow(text string, showUser bool) {
	st := &a.chat
	s := a.chatEnsure()
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if st.busy {
		return
	}
	if !a.chatModelReady() {
		a.refreshModelKeys()
	}
	m := a.activeChatModel()
	if m == nil || !a.providerReady(m.Provider) {
		s.Msgs = append(s.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
		s.Updated = time.Now()
		return
	}
	prov := m.Provider
	var prof *ModelKeyProfile
	if prov != inapp.Ollama {
		prof = a.keyProfileForProvider(prov)
		if prof == nil {
			s.Msgs = append(s.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
			s.Updated = time.Now()
			return
		}
	}
	st.draft = ""
	if showUser {
		s.Msgs = append(s.Msgs, chatMsg{Role: chatUser, Text: text, At: time.Now()})
		a.chatFollowThread(&st.threadScroll)
	}
	if s.Title == locales.ChatUntitled() || s.Title == "" {
		s.Title = chatTitleFrom(text)
	}
	s.Updated = time.Now()
	st.busy = true
	sessionID := s.ID
	devID, devName := a.chatScopeDevice()
	useAgent := a.opt.AgentGateway != nil && devID != ""

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	st.genCancel = cancel
	go func() {
		defer cancel()
		var reply string
		var err error
		if useAgent {
			emit := func(line string) {
				a.post(func() { a.chatAppendStream(sessionID, line) })
			}
			reply, err = a.chatRunAgent(ctx, m, prof, devID, devName, s.Title, text, emit)
		} else {
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
				strings.Join(phones, ", ") + "."
			switch prov {
			case inapp.Ollama:
				name, ok := inapp.OllamaModelName(m.ID)
				if !ok {
					err = fmt.Errorf("unknown Ollama model")
				} else {
					reply, err = inapp.CompleteOllama(ctx, a.ollamaBaseURL(), name, system, history)
				}
			default:
				account := a.keyAccountFor(prof)
				var key string
				key, err = a.revealModelAPIKey(ctx, account)
				if err == nil {
					reply, err = inapp.Complete(ctx, prov, m.ID, key, system, history)
				}
			}
		}
		a.post(func() {
			st.busy = false
			st.genCancel = nil
			sess := a.chatSession(sessionID)
			if sess == nil {
				return
			}
			switch {
			case errors.Is(err, context.Canceled):
			case errors.Is(err, secret.ErrCancelled):
			case errors.Is(err, secret.ErrNotFound):
				a.refreshModelKeys()
				sess.Msgs = append(sess.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrNoKey(), At: time.Now()})
			case err != nil:
				sess.Msgs = append(sess.Msgs, chatMsg{Role: chatError, Text: locales.ChatErrReply(err.Error()), At: time.Now()})
			default:
				a.chatFinishStream(sessionID, reply)
			}
			sess.Updated = time.Now()
			a.chatAfterGeneration()
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

func (a *app) chatRunAgent(ctx context.Context, m *inapp.ChatModel, prof *ModelKeyProfile, devID, devName, title, prompt string, emit func(string)) (string, error) {
	if m == nil {
		return "", fmt.Errorf("inapp: no model")
	}
	run := func(model *inapp.ChatModel, profile *ModelKeyProfile) inapp.AgentOutcome {
		var apiKey string
		var err error
		if model.Provider != inapp.Ollama {
			account := a.keyAccountFor(profile)
			apiKey, err = a.revealModelAPIKey(ctx, account)
			if err != nil {
				return inapp.AgentOutcome{Err: err}
			}
		}
		return inapp.RunAgent(ctx, inapp.AgentConfig{
			Gateway:    a.opt.AgentGateway,
			Provider:   model.Provider,
			ModelID:    model.ID,
			APIKey:     apiKey,
			OllamaBase: a.ollamaBaseURL(),
			DeviceID:   devID,
			DeviceName: devName,
			TaskTitle:  title,
			Prompt:     prompt,
			LocaleIsJA: strings.HasPrefix(i18n.Locale(), "ja"),
		}, func(ev inapp.AgentEvent) { emit(ev.Text) })
	}
	out := run(m, prof)
	if out.Err == nil || m.Provider != inapp.Ollama || !inapp.ErrToolsNotSupported(out.Err) {
		return out.Reply, out.Err
	}
	fb, fprof := a.activeChatAgentModel()
	if fb == nil || fprof == nil {
		return "", fmt.Errorf("%s", locales.ChatAgentNoFallback())
	}
	emit(locales.ChatAgentFallback(fb.Label))
	out = run(fb, fprof)
	return out.Reply, out.Err
}

func (a *app) chatScopeDevice() (id, name string) {
	if a.sel == allPhones {
		for _, d := range a.devices {
			if d.Status != gateway.DeviceOffline {
				return d.Info.ID, d.Info.Name
			}
		}
		return "", ""
	}
	if d := a.device(a.sel); d != nil && d.Status != gateway.DeviceOffline {
		return d.Info.ID, d.Info.Name
	}
	return "", ""
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
