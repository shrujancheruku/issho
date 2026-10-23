// Package gemini calls Gemini on Google Cloud's Agent Platform (formerly
// Vertex AI): generateContent over REST with a Google Cloud API key, through
// the same transport as the Maps and Translation clients
// (internal/platform/googleapi). Callers own their prompts and response
// schemas; this package sends them and returns the JSON answer.
//
// Agent Platform keys (they start "AQ.") work only on aiplatform.googleapis.com;
// the Gemini Developer API (generativelanguage.googleapis.com) rejects them
// with API_KEY_SERVICE_BLOCKED.
package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/googleapi"
)

// The global endpoint; the key's own project is billed, so no project id is
// needed in the path.
const defaultURL = "https://aiplatform.googleapis.com/v1/publishers/google/models/%s:generateContent"

// ErrDeclined means the model wouldn't answer: a blocked prompt, a safety
// stop or an empty answer.
var ErrDeclined = errors.New("gemini: no answer")

// Client calls one model.
type Client struct {
	API   *googleapi.Client
	URL   string // with %s for the model; overridable for tests
	Model string
}

// New uses a Google Cloud API key for Agent Platform, restricted to it.
// timeout bounds each attempt (the transport retries 429s and 5xxs).
func New(apiKey, model string, timeout time.Duration) *Client {
	api := googleapi.New(apiKey)
	api.SetTimeout(timeout)
	return &Client{API: api, URL: defaultURL, Model: model}
}

// Request is one generateContent call with a JSON answer.
type Request struct {
	System string
	User   string
	// Schema is the answer's JSON schema, in the OpenAPI subset Agent
	// Platform accepts (no additionalProperties).
	Schema          map[string]any
	MaxOutputTokens int
	Thinking        string // LOW, MEDIUM or HIGH
}

type part struct {
	Text string `json:"text"`
	// Thought parts carry the model's reasoning, not the answer.
	Thought bool `json:"thought,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type request struct {
	SystemInstruction content          `json:"systemInstruction"`
	Contents          []content        `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type generationConfig struct {
	MaxOutputTokens  int            `json:"maxOutputTokens"`
	ResponseMimeType string         `json:"responseMimeType"`
	ResponseSchema   map[string]any `json:"responseSchema"`
	ThinkingConfig   thinkingConfig `json:"thinkingConfig"`
}

type thinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel"`
}

type response struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata Usage `json:"usageMetadata"`
}

// Usage is a call's token counts, as Agent Platform reports them.
type Usage struct {
	PromptTokens     int `json:"promptTokenCount"`
	CandidatesTokens int `json:"candidatesTokenCount"`
	ThoughtsTokens   int `json:"thoughtsTokenCount"`
}

// Generate returns the model's answer: JSON matching the request's schema.
// The model declining is ErrDeclined; an answer cut off at
// MaxOutputTokens is an error of its own.
func (c *Client) Generate(ctx context.Context, r Request) (string, error) {
	answer, _, err := c.GenerateWithUsage(ctx, r)
	return answer, err
}

// GenerateWithUsage is Generate, also returning the call's token counts.
func (c *Client) GenerateWithUsage(ctx context.Context, r Request) (string, Usage, error) {
	req := request{
		SystemInstruction: content{Parts: []part{{Text: r.System}}},
		Contents:          []content{{Role: "user", Parts: []part{{Text: r.User}}}},
		GenerationConfig: generationConfig{
			MaxOutputTokens:  r.MaxOutputTokens,
			ResponseMimeType: "application/json",
			ResponseSchema:   r.Schema,
			ThinkingConfig:   thinkingConfig{ThinkingLevel: r.Thinking},
		},
	}
	var resp response
	if err := c.API.Post(ctx, fmt.Sprintf(c.URL, c.Model), nil, req, &resp); err != nil {
		return "", Usage{}, err
	}
	if resp.PromptFeedback.BlockReason != "" || len(resp.Candidates) == 0 {
		return "", resp.UsageMetadata, ErrDeclined
	}
	cand := resp.Candidates[0]
	switch cand.FinishReason {
	case "STOP", "":
	case "MAX_TOKENS":
		return "", resp.UsageMetadata, fmt.Errorf("gemini: answer cut off at maxOutputTokens")
	default: // SAFETY, PROHIBITED_CONTENT, BLOCKLIST, RECITATION, …
		return "", resp.UsageMetadata, ErrDeclined
	}
	var answer strings.Builder
	for _, p := range cand.Content.Parts {
		if !p.Thought {
			answer.WriteString(p.Text)
		}
	}
	if answer.Len() == 0 {
		return "", resp.UsageMetadata, ErrDeclined
	}
	return answer.String(), resp.UsageMetadata, nil
}
