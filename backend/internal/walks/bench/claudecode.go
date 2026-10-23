package bench

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// ClaudeCode runs Claude through the Claude Code CLI in headless mode
// (`claude -p`), signed in with the developer's Claude subscription, so a
// benchmark can include Claude without an API key. Benchmark only: the
// worker calls the API (internal/walks/claude).
//
// It isn't quite the API call production would make: Claude Code adds a
// short environment note (date, platform) to every call and returns
// structured output through a tool, over a few turns. Each call runs in an
// empty directory with tools, settings, skills and MCP servers off, so no
// project files, CLAUDE.md or memory reach the model. Confirm a winner on
// the API (claude: variants) before switching production to it.
type ClaudeCode struct {
	Model  string
	Effort string // low, medium, high, xhigh or max; "" = default
	System string // for proposals: replaces walks.SystemPrompt when set
	Bin    string // "" = claude on PATH
}

var (
	_ walks.Proposer = (*ClaudeCode)(nil)
	_ LLM            = (*ClaudeCode)(nil)
)

func (c *ClaudeCode) Name() string { return "claudecode:" + c.Model + ":" + c.Effort }

func (c *ClaudeCode) Propose(ctx context.Context, req walks.Request, feedback []string) (walks.Proposal, error) {
	answer, call, err := c.JSON(ctx, cmp.Or(c.System, walks.SystemPrompt), walks.UserPrompt(req, feedback), walks.ProposalSchema(true))
	if err != nil {
		return walks.Proposal{}, err
	}
	p, err := walks.ParseProposal(answer)
	if err != nil {
		return walks.Proposal{}, fmt.Errorf("claude code: %w", err)
	}
	p.Meta = walks.Meta{
		Model: call.Model, InputTokens: call.InputTokens, OutputTokens: call.OutputTokens,
		Latency: time.Duration(call.LatencyMs) * time.Millisecond, CostUSD: call.CostUSD,
	}
	return p, nil
}

// cliResult is the part of `claude -p --output-format json` used here.
type cliResult struct {
	IsError          bool            `json:"is_error"`
	Subtype          string          `json:"subtype"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	DurationAPIMs    int64           `json:"duration_api_ms"`
	ModelUsage       map[string]struct {
		InputTokens         int `json:"inputTokens"`
		OutputTokens        int `json:"outputTokens"`
		CacheReadTokens     int `json:"cacheReadInputTokens"`
		CacheCreationTokens int `json:"cacheCreationInputTokens"`
	} `json:"modelUsage"`
}

func (c *ClaudeCode) JSON(ctx context.Context, system, user string, schema map[string]any) (string, Call, error) {
	dir, err := os.MkdirTemp("", "walk-bench-claude-")
	if err != nil {
		return "", Call{}, err
	}
	defer os.RemoveAll(dir)
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", Call{}, err
	}
	args := []string{
		"-p", "--model", c.Model, "--output-format", "json", "--json-schema", string(schemaJSON),
		"--system-prompt", system, "--tools", "", "--no-session-persistence",
		"--setting-sources", "", "--disable-slash-commands", "--strict-mcp-config",
	}
	if c.Effort != "" {
		args = append(args, "--effort", c.Effort)
	}
	cmd := exec.CommandContext(ctx, cmp.Or(c.Bin, "claude"), args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(user) // the prompt, so it can't be read as a flag
	// The subscription login, never an API key that happens to be set.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=")
	})
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	runErr := cmd.Run()
	var res cliResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		if runErr != nil {
			return "", Call{}, fmt.Errorf("claude code: %w: %s", runErr, firstLine(stderr.String()))
		}
		return "", Call{}, fmt.Errorf("claude code: unreadable output: %w", err)
	}
	call := Call{LatencyMs: cmp.Or(res.DurationAPIMs, time.Since(start).Milliseconds()), CostUSD: res.TotalCostUSD}
	// The model that did the most work answered (Claude Code may use a
	// small model for housekeeping).
	for m, u := range res.ModelUsage {
		if u.OutputTokens >= call.OutputTokens {
			call.Model = m
			call.OutputTokens = u.OutputTokens
			call.InputTokens = u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
		}
	}
	if res.IsError || runErr != nil {
		return "", call, fmt.Errorf("claude code: %s: %s", res.Subtype, firstLine(cmp.Or(res.Result, stderr.String())))
	}
	if len(res.StructuredOutput) == 0 || string(res.StructuredOutput) == "null" {
		return "", call, errors.New("claude code: no structured output")
	}
	return string(res.StructuredOutput), call, nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
