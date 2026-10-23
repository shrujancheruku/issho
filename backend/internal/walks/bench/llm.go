package bench

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/shrujancheruku/issho/backend/internal/platform/gemini"
)

// LLM answers a prompt with JSON matching a schema: the judges.
type LLM interface {
	Name() string
	JSON(ctx context.Context, system, user string, schema map[string]any) (string, Call, error)
}

// ClaudeLLM is a judge on the Anthropic API.
type ClaudeLLM struct {
	Client        anthropic.Client
	Model, Effort string
}

// NewClaudeLLM uses an Anthropic API key.
func NewClaudeLLM(apiKey, model, effort string) *ClaudeLLM {
	return &ClaudeLLM{
		Client: anthropic.NewClient(option.WithAPIKey(apiKey), option.WithRequestTimeout(5*time.Minute), option.WithMaxRetries(3)),
		Model:  model, Effort: effort,
	}
}

func (c *ClaudeLLM) Name() string { return "claude:" + c.Model + ":" + c.Effort }

func (c *ClaudeLLM) JSON(ctx context.Context, system, user string, schema map[string]any) (string, Call, error) {
	start := time.Now()
	resp, err := c.Client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.Model),
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
			Effort: anthropic.OutputConfigEffort(c.Effort),
		},
	})
	if err != nil {
		return "", Call{}, fmt.Errorf("claude judge: %w", err)
	}
	call := Call{
		Model: string(resp.Model), InputTokens: int(resp.Usage.InputTokens), OutputTokens: int(resp.Usage.OutputTokens),
		LatencyMs: time.Since(start).Milliseconds(),
	}
	call.CostUSD = cost(call.Model, call.InputTokens, call.OutputTokens)
	if resp.StopReason == anthropic.StopReasonRefusal || resp.StopReason == anthropic.StopReasonMaxTokens {
		return "", call, fmt.Errorf("claude judge: stopped (%s)", resp.StopReason)
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String(), call, nil
}

// GeminiLLM is a judge on Agent Platform.
type GeminiLLM struct {
	Client   *gemini.Client
	Thinking string
}

// NewGeminiLLM uses a Google Cloud API key for Agent Platform.
func NewGeminiLLM(apiKey, model, thinking string) *GeminiLLM {
	return &GeminiLLM{Client: gemini.New(apiKey, model, 5*time.Minute), Thinking: thinking}
}

func (g *GeminiLLM) Name() string { return "gemini:" + g.Client.Model + ":" + g.Thinking }

func (g *GeminiLLM) JSON(ctx context.Context, system, user string, schema map[string]any) (string, Call, error) {
	start := time.Now()
	answer, u, err := g.Client.GenerateWithUsage(ctx, gemini.Request{
		System: system, User: user, Schema: withoutAdditional(schema), MaxOutputTokens: 20000, Thinking: g.Thinking,
	})
	call := Call{
		Model: g.Client.Model, InputTokens: u.PromptTokens, OutputTokens: u.CandidatesTokens + u.ThoughtsTokens,
		LatencyMs: time.Since(start).Milliseconds(),
	}
	call.CostUSD = cost(call.Model, call.InputTokens, call.OutputTokens)
	if errors.Is(err, gemini.ErrDeclined) {
		return "", call, fmt.Errorf("gemini judge: declined")
	}
	return answer, call, err
}

// withoutAdditional strips "additionalProperties" (which Claude's
// structured outputs need and Agent Platform rejects) from a schema.
func withoutAdditional(v any) map[string]any {
	var strip func(any) any
	strip = func(v any) any {
		switch t := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, x := range t {
				if k != "additionalProperties" {
					out[k] = strip(x)
				}
			}
			return out
		case []any:
			out := make([]any, len(t))
			for i, x := range t {
				out[i] = strip(x)
			}
			return out
		}
		return v
	}
	return strip(v).(map[string]any)
}
