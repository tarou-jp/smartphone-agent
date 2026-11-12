package mcp

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DIYgod/pinky/internal/gateway"
)

// render turns a gateway result into an MCP tool result. The text is for
// models: the first line is the status (or "error: <code>") with
// "key: value" details separated by " · ", then the message, then the screen
// in full or the lines that changed.
func render(res *gateway.Result) *sdk.CallToolResult {
	var b strings.Builder
	writeResult(&b, res, "")
	out := &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: strings.TrimRight(b.String(), "\n")}},
		IsError: res.IsError()}
	if res.Screenshot != nil {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, res.Screenshot, &jpeg.Options{Quality: 80}); err == nil {
			out.Content = append(out.Content, &sdk.ImageContent{Data: buf.Bytes(), MIMEType: "image/jpeg"})
		}
	}
	return out
}

// StatusLine is the first line of a result.
func StatusLine(res *gateway.Result) string {
	var parts []string
	if res.IsError() {
		parts = append(parts, "error: "+string(res.Error))
	} else {
		parts = append(parts, string(res.Status))
	}
	if res.DeviceName != "" && !res.IsError() {
		parts = append(parts, res.DeviceName)
	}
	if res.ApprovalID != "" {
		parts = append(parts, "approval_id: "+res.ApprovalID)
	}
	if res.Rule != "" {
		parts = append(parts, "rule: "+res.Rule)
	}
	if res.Decision != "" {
		parts = append(parts, "decision: "+res.Decision)
	}
	if res.Settle != "" {
		parts = append(parts, "settle: "+res.Settle)
	}
	if res.Matched != nil {
		parts = append(parts, fmt.Sprintf("matched: %v", *res.Matched))
	}
	if res.Holder != "" {
		parts = append(parts, "holder: "+res.Holder)
	}
	if res.Setup != "" {
		parts = append(parts, "setup: "+res.Setup)
	}
	if !res.Full && res.Screen != nil && len(res.Diff) == 0 && len(res.Steps) == 0 {
		parts = append(parts, "no change")
	}
	return strings.Join(parts, " · ")
}

func writeResult(b *strings.Builder, res *gateway.Result, indent string) {
	line := func(s string) {
		for _, l := range strings.Split(s, "\n") {
			b.WriteString(indent)
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	line(StatusLine(res))
	if res.Note != "" {
		line(fmt.Sprintf("note: %q", res.Note))
	}
	if res.Message != "" {
		line(res.Message)
	}
	if res.Task != "" {
		line("device: " + res.Device)
	}
	for i, st := range res.Steps {
		b.WriteString(indent)
		fmt.Fprintf(b, "step %d: ", i+1)
		var sb strings.Builder
		writeResult(&sb, st, "")
		text := strings.TrimRight(sb.String(), "\n")
		first, rest, _ := strings.Cut(text, "\n")
		b.WriteString(first)
		b.WriteByte('\n')
		if rest != "" {
			line(indentLines(rest, "  "))
		}
	}
	if len(res.Steps) > 0 {
		return
	}
	if res.Flow != nil {
		for _, st := range res.Flow.Steps {
			l := fmt.Sprintf("step %d: %s · %s · %d ms", st.Index, st.Status, st.Text, st.Elapsed.Milliseconds())
			if st.Detail != "" && st.Status == gateway.FlowStepOK {
				l += " · " + st.Detail
			}
			line(l)
		}
	}
	for _, f := range res.Flows {
		l := fmt.Sprintf("%s · version %d · %d steps", f.Name, f.Version, f.Steps)
		if f.App != "" {
			l += " · " + f.App
		}
		if len(f.Params) > 0 {
			l += " · params: " + strings.Join(f.Params, ", ")
		}
		if f.Description != "" {
			l += " · " + f.Description
		}
		line(l)
	}
	app := ""
	for _, p := range res.Parts {
		if p.App != app {
			app = p.App
			line("parts of " + app + ":")
		}
		l := p.ID + " · " + p.Text
		if len(p.After) > 0 {
			l += " · after " + strings.Join(p.After, ", ")
		}
		line(l)
	}
	if res.FlowYAML != "" {
		line(strings.TrimRight(res.FlowYAML, "\n"))
	}
	if res.Folder != "" {
		line("folder: " + res.Folder)
		for _, f := range res.Files {
			line(fmt.Sprintf("%s · %s · %s · %s", f.Path, size(f.Size), f.ModTime.Format("2006-01-02 15:04"), f.Where))
		}
	}
	if len(res.Apps) > 0 {
		for _, a := range res.Apps {
			l := a.Name + " (" + a.ID + ")"
			if a.System {
				l += " · system"
			}
			line(l)
		}
	}
	switch {
	case res.Screen == nil:
	case res.Full:
		line(res.Screen.Render())
	case len(res.Diff) > 0:
		line(strings.Join(res.Diff, "\n"))
	}
}

// size writes a file size for people and models alike: 812 B, 14 KB, 3.2 MB.
func size(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return fmt.Sprintf("%d KB", (n+500)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.1f GB", float64(n)/1e9)
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
