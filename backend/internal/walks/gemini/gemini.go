// Package gemini implements walks.Proposer with Gemini on Google Cloud's
// Agent Platform (internal/platform/gemini), from the model's own knowledge:
// no grounding. Grounding with Google Maps or Search would know more, but
// its answers may only be shown to the person who asked and not stored
// (Google Cloud service terms), and a plan is composed once for everyone.
package gemini

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/gemini"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

const (
	// Model knows neighbourhoods well enough to name real temples, cafés
	// and viewpoints: in the trial (2026-10-22) it named real, well-chosen
	// places in Tokyo and Austin, where 3.1 Pro (preview) made up a temple.
	// The cheapest models know too few small places.
	Model = "gemini-3.8-flash"
	// A walk is composed in the background, and medium thinking can take a
	// few minutes.
	timeout = 5 * time.Minute
)

// Proposer calls Gemini.
type Proposer struct {
	*gemini.Client
	Thinking string // LOW, MEDIUM (default) or HIGH
	// System replaces walks.SystemPrompt when set (prompt experiments).
	System string
}

// New uses a Google Cloud API key for Agent Platform, with Model.
func New(apiKey string) *Proposer { return NewModel(apiKey, Model) }

// NewModel uses another Gemini model (for comparisons).
func NewModel(apiKey, model string) *Proposer {
	c := gemini.New(apiKey, model, timeout)
	// One retry at most: each attempt can take minutes.
	c.API.MaxRetries = 1
	return &Proposer{Client: c, Thinking: "MEDIUM"}
}

var _ walks.Proposer = (*Proposer)(nil)

func (p *Proposer) Propose(ctx context.Context, req walks.Request, feedback []string) (walks.Proposal, error) {
	start := time.Now()
	answer, usage, err := p.GenerateWithUsage(ctx, gemini.Request{
		System: cmp.Or(p.System, walks.SystemPrompt),
		User:   walks.UserPrompt(req, feedback),
		Schema: walks.ProposalSchema(false),
		// The answer is ~700 tokens; medium thinking used 1,500 to 9,000 in
		// the trial.
		MaxOutputTokens: 20000,
		Thinking:        p.Thinking,
	})
	if err != nil {
		return walks.Proposal{}, err
	}
	w, err := walks.ParseProposal(answer)
	if err != nil {
		return walks.Proposal{}, fmt.Errorf("gemini: %w", err)
	}
	w.Meta = walks.Meta{
		Model: p.Model, Latency: time.Since(start),
		InputTokens: usage.PromptTokens, OutputTokens: usage.CandidatesTokens + usage.ThoughtsTokens,
	}
	return w, nil
}
