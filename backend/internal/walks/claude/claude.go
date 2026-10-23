// Package claude implements walks.Proposer with Claude (the Anthropic API),
// from the model's own knowledge with the same prompt and schema as the
// Gemini proposer (walks.SystemPrompt, walks.ProposalSchema), so the two
// can be compared fairly (internal/walks/bench). No tools: like Gemini's
// grounding, web search would know more, but the walk must come from the
// model, and Maps content may not be given to it.
package claude

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// ErrDeclined means the model declined (a refusal stop).
var ErrDeclined = errors.New("claude: declined")

// Proposer calls one Claude model.
type Proposer struct {
	Client anthropic.Client
	Model  string
	// Effort: low, medium, high, xhigh or max; "" = the model's default.
	Effort string
	// System replaces walks.SystemPrompt when set (prompt experiments).
	System string
}

// New uses an Anthropic API key. A walk is composed in the background, so
// each attempt may take minutes; the SDK retries 429s and 5xxs.
func New(apiKey, model, effort string) *Proposer {
	c := anthropic.NewClient(option.WithAPIKey(apiKey), option.WithRequestTimeout(5*time.Minute), option.WithMaxRetries(2))
	return &Proposer{Client: c, Model: model, Effort: effort}
}

var _ walks.Proposer = (*Proposer)(nil)

func (p *Proposer) Propose(ctx context.Context, req walks.Request, feedback []string) (walks.Proposal, error) {
	params := anthropic.MessageNewParams{
		Model: anthropic.Model(p.Model),
		// The answer is ~1,000 tokens; the rest is room for thinking.
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: cmp.Or(p.System, walks.SystemPrompt)}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(walks.UserPrompt(req, feedback)))},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: walks.ProposalSchema(true)},
			Effort: anthropic.OutputConfigEffort(p.Effort),
		},
	}
	start := time.Now()
	resp, err := p.Client.Messages.New(ctx, params)
	if err != nil {
		return walks.Proposal{}, fmt.Errorf("claude: %w", err)
	}
	latency := time.Since(start)
	// No refusal fallback on purpose: a benchmark must measure the model it
	// names, and a walk the model declines simply isn't opened.
	switch resp.StopReason {
	case anthropic.StopReasonRefusal:
		return walks.Proposal{}, ErrDeclined
	case anthropic.StopReasonMaxTokens:
		return walks.Proposal{}, errors.New("claude: answer cut off at max_tokens")
	}
	var answer strings.Builder
	for _, b := range resp.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			answer.WriteString(t.Text)
		}
	}
	w, err := walks.ParseProposal(answer.String())
	if err != nil {
		return walks.Proposal{}, fmt.Errorf("claude: %w", err)
	}
	w.Meta = walks.Meta{
		Model: string(resp.Model), Latency: latency,
		InputTokens: int(resp.Usage.InputTokens), OutputTokens: int(resp.Usage.OutputTokens),
	}
	return w, nil
}
