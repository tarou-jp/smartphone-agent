package inapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// modelClient asks one provider for the model's next turn, streaming its
// text as it comes.
type modelClient interface {
	turn(ctx context.Context, req turnRequest, h turnHandlers) (turnResult, error)
}

type turnRequest struct {
	System string
	Tools  []agentTool // none: the model only writes
	Items  []item
	// MaxTokens caps the reply.
	MaxTokens int
}

type turnHandlers struct {
	// text hears the model's words as they stream.
	text func(delta string)
	// retry hears each wait before another try.
	retry func(n int, wait time.Duration, err error)
}

type usage struct {
	Input, Output, CacheRead, CacheWrite int
}

func (u *usage) add(o usage) {
	u.Input += o.Input
	u.Output += o.Output
	u.CacheRead += o.CacheRead
	u.CacheWrite += o.CacheWrite
}

type turnResult struct {
	Text  string
	Calls []toolCall
	// Truncated is set when the reply hit MaxTokens.
	Truncated bool
	Usage     usage
}

// newClient is the client for a provider and model.
func newClient(p Provider, model, apiKey, ollamaBase string) (modelClient, error) {
	switch p {
	case Anthropic:
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("inapp: empty API key")
		}
		return &anthropicClient{model: model, key: apiKey, base: anthropicBase()}, nil
	case OpenAI:
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("inapp: empty API key")
		}
		return &openAIClient{url: openAIBase() + "/v1/chat/completions", key: apiKey, model: model, reasoningOff: true}, nil
	case Ollama:
		name := strings.TrimPrefix(model, ollamaModelPrefix)
		if name == "" {
			return nil, errors.New("inapp: empty Ollama model")
		}
		return &openAIClient{url: NormalizeOllamaBaseURL(ollamaBase) + "/v1/chat/completions", model: name}, nil
	default:
		return nil, fmt.Errorf("inapp: unknown provider %q", p)
	}
}

// streamTurn runs one streamed request with retries: a request that fails,
// or a stream that breaks before the model said anything, is tried again.
// A stream that breaks after it said something is an error: its words are
// already on the person's screen.
func streamTurn(ctx context.Context, h turnHandlers, once func(said *bool) (turnResult, error)) (turnResult, error) {
	pol := defaultRetry
	for n := 1; ; n++ {
		said := false
		res, err := once(&said)
		if err == nil {
			return res, nil
		}
		var se *streamError
		if said || n >= pol.attempts || ctx.Err() != nil || (!errors.As(err, &se) && !retryable(err)) {
			return res, err
		}
		d := pol.wait(n, err)
		if h.retry != nil {
			h.retry(n, d, err)
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(d):
		}
	}
}
