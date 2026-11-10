package inapp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// A thread is the agent's conversation as a list of items, after Codex's:
// what the person said, what the model said and asked for, and what each
// tool answered. Providers turn items into their own message format; the
// loop decides which items the model sees, in what detail (see context).
type itemKind int

const (
	itemUser      itemKind = iota // the person, or the loop speaking for them
	itemAssistant                 // the model: text and tool calls
	itemResult                    // a tool's answer to one call
)

type toolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type item struct {
	Kind  itemKind
	Text  string     // user and assistant text
	Calls []toolCall // assistant
	// A result answers the call with this ID.
	CallID string
	Tool   string
	Output string
}

// argsJSON is a call's arguments as the APIs want them back.
func (c toolCall) argsJSON() string {
	if c.Args == nil {
		return "{}"
	}
	b, err := json.Marshal(c.Args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Context management. A phone's screen is most of what an agent reads,
// and only the latest one matters: older screens are long and stale. So,
// as Codex keeps a thread inside the model's window, the model sees the
// last few tool answers whole and older ones cut to their first lines; any
// answer is cut to maxOutput; and a thread that still grows past
// compactAt is summed up by the model and starts again from the summary.
const (
	keepWhole   = 3       // tool answers, from the latest, the model sees whole
	elidedLines = 3       // lines an older answer keeps
	maxOutput   = 24_000  // characters of any one answer
	compactAt   = 240_000 // characters of a thread (about 60k tokens)
	keepRecent  = 6       // items a compaction keeps as they are
)

// view is the thread as the model should see it now: old tool answers cut
// short, long ones cut in the middle.
func view(items []item) []item {
	out := make([]item, len(items))
	copy(out, items)
	whole := 0
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Kind != itemResult {
			continue
		}
		o := out[i].Output
		if whole < keepWhole {
			whole++
			out[i].Output = clip(o, maxOutput)
			continue
		}
		out[i].Output = elide(o)
	}
	return out
}

// elide keeps an answer's first lines (its status and message) and says
// what was left out.
func elide(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= elidedLines+1 && len(s) <= 600 {
		return s
	}
	head := lines
	if len(head) > elidedLines {
		head = head[:elidedLines]
	}
	h := clip(strings.Join(head, "\n"), 600)
	return h + "\n[older screen left out; phone_observe shows the current one]"
}

// clip cuts s to about n characters, keeping its start and its end.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	head, tail := n*3/4, n/4
	// Cut on line ends where there are any.
	h := s[:head]
	if i := strings.LastIndexByte(h, '\n'); i > head/2 {
		h = h[:i]
	}
	t := s[len(s)-tail:]
	if i := strings.IndexByte(t, '\n'); i >= 0 && i < tail/2 {
		t = t[i+1:]
	}
	return fmt.Sprintf("%s\n[… %d characters left out …]\n%s", h, len(s)-len(h)-len(t), t)
}

// size is about how many characters the model reads for items.
func size(items []item) int {
	n := 0
	for _, it := range items {
		n += len(it.Text) + len(it.Output)
		for _, c := range it.Calls {
			n += len(c.Name) + len(c.argsJSON())
		}
	}
	return n
}

// compactCut is where a compaction may split items: everything before it
// is summed up, everything from it on is kept as it is. It keeps the last
// few items, at most about a quarter of compactAt, and never splits a call
// from its answers. 0 means there is nothing worth summing up.
func compactCut(items []item) int {
	cut, kept := len(items), 0
	for cut > 0 && len(items)-cut < keepRecent {
		n := size(view(items[cut-1 : cut]))
		if len(items)-cut >= 2 && kept+n > compactAt/4 {
			break
		}
		kept += n
		cut--
	}
	for cut > 0 && cut < len(items) && items[cut].Kind == itemResult {
		cut--
	}
	if cut < 2 {
		return 0
	}
	return cut
}

// transcript writes items as plain text for the model that sums them up.
func transcript(items []item) string {
	var b strings.Builder
	for _, it := range items {
		switch it.Kind {
		case itemUser:
			fmt.Fprintf(&b, "PERSON: %s\n\n", it.Text)
		case itemAssistant:
			if strings.TrimSpace(it.Text) != "" {
				fmt.Fprintf(&b, "AGENT: %s\n", it.Text)
			}
			for _, c := range it.Calls {
				fmt.Fprintf(&b, "AGENT CALLED %s %s\n", c.Name, c.argsJSON())
			}
			b.WriteString("\n")
		case itemResult:
			fmt.Fprintf(&b, "RESULT of %s:\n%s\n\n", it.Tool, elide(it.Output))
		}
	}
	return b.String()
}

const compactPrompt = `You are handing this task over to yourself with a fresh memory. Sum up the work so far for the agent that continues it: the goal, what was done on each phone and what it showed, what worked and what did not, approvals the person gave or refused, and what is left to do. Be concrete (app names, screens, values) and brief. Do not call tools.`
