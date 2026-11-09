package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DIYgod/pinky/internal/mcp"
)

type agentTool struct {
	Name        string
	Description string
	Schema      map[string]any
}

var (
	loadToolsOnce sync.Once
	cachedTools   []agentTool
	loadToolsErr  error
)

func agentTools() ([]agentTool, error) {
	loadToolsOnce.Do(func() {
		list, err := mcp.Tools(context.Background(), "0")
		if err != nil {
			loadToolsErr = err
			return
		}
		for _, t := range list {
			if t == nil {
				continue
			}
			cachedTools = append(cachedTools, agentTool{
				Name:        t.Name,
				Description: t.Description,
				Schema:      toolSchemaMap(t),
			})
		}
	})
	return cachedTools, loadToolsErr
}

func toolSchemaMap(t *sdkmcp.Tool) map[string]any {
	if t.InputSchema == nil {
		return map[string]any{"type": "object", "additionalProperties": true}
	}
	raw, err := json.Marshal(t.InputSchema)
	if err != nil {
		return map[string]any{"type": "object"}
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return map[string]any{"type": "object"}
	}
	return m
}

func parseToolArgs(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func toolCallLabel(name string) string {
	return fmt.Sprintf("[%s]", name)
}
