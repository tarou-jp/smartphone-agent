package mcp

import (
	"strings"

	"github.com/DIYgod/pinky/internal/gateway"
)

// FormatResult is the text agents see from a tool call, same as MCP tool results.
func FormatResult(res *gateway.Result) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	writeResult(&b, res, "")
	return strings.TrimRight(b.String(), "\n")
}
