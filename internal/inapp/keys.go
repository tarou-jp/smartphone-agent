package inapp

import "fmt"

// KeyAccount is the keychain account for a saved API key. id is unique per
// provider; "default" keeps the first key at llm:provider for compatibility.
func KeyAccount(p Provider, id string) string {
	if id == "" {
		id = "default"
	}
	if id == "default" {
		return "llm:" + string(p)
	}
	return fmt.Sprintf("llm:%s:%s", p, id)
}

// LegacyKeyAccount is the original single-key account name.
func LegacyKeyAccount(p Provider) string { return "llm:" + string(p) }
