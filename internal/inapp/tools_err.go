package inapp

import (
	"strings"
)

// ErrToolsNotSupported reports whether err is an Ollama (or API) refusal to use tools.
func ErrToolsNotSupported(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "does not support tools") ||
		strings.Contains(s, "tool use") && strings.Contains(s, "not support")
}
