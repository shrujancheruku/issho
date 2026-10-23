package bench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// fakeCLI writes a stand-in for the claude binary that records how it was
// called (arguments, working directory, whether an API key leaked through,
// the prompt on stdin) and prints the given JSON.
func fakeCLI(t *testing.T, output string) (bin, record string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(dir, "record")
	bin = filepath.Join(dir, "claude")
	script := `#!/bin/sh
{ printf 'ARGS:'; for a in "$@"; do printf '[%s]' "$a"; done; echo
  echo "PWD:$(pwd)"; echo "KEY:${ANTHROPIC_API_KEY:-none}"; echo "STDIN:$(cat)"; } > ` + record + `
cat <<'JSON'
` + output + `
JSON
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, record
}

func TestClaudeCode(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-must-not-be-used")
	bin, record := fakeCLI(t, `{"type":"result","subtype":"success","is_error":false,"duration_api_ms":4200,"total_cost_usd":0.12,
"structured_output":{"title":"T","summary":"S","meet":{"name":"St","search":"St","kind":"station","minutes":10,"note":"n"},"stops":[],"finish":{"name":"C","search":"C","kind":"cafe","minutes":40,"note":"n"}},
"modelUsage":{"claude-haiku-5-5":{"inputTokens":10,"outputTokens":5},"claude-opus-5-5":{"inputTokens":3,"outputTokens":900,"cacheReadInputTokens":20000,"cacheCreationInputTokens":1000}}}`)
	c := &ClaudeCode{Model: "claude-opus-5-5", Effort: "high", Bin: bin}
	p, err := c.Propose(context.Background(), walks.Request{City: "Tokyo", Area: "Koenji", Theme: "temples"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "T" || p.Finish.Kind != "cafe" || p.Meta.Model != "claude-opus-5-5" || p.Meta.OutputTokens != 900 ||
		p.Meta.InputTokens != 21003 || p.Meta.CostUSD != 0.12 || p.Meta.Latency.Milliseconds() != 4200 {
		t.Errorf("proposal = %+v", p)
	}
	raw, _ := os.ReadFile(record)
	rec := string(raw)
	for _, want := range []string{
		"[-p][--model][claude-opus-5-5]", "[--tools][]", "[--setting-sources][]", "[--no-session-persistence]",
		"[--disable-slash-commands]", "[--strict-mcp-config]", "[--effort][high]", "[--system-prompt][You plan walking outings",
		"KEY:none", "STDIN:Theme:  (temples)", "Area: Koenji, Tokyo",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("call lacks %q:\n%s", want, rec)
		}
	}
	// An empty working directory, so no project files or CLAUDE.md load.
	pwd := strings.TrimPrefix(strings.SplitN(strings.SplitN(rec, "PWD:", 2)[1], "\n", 2)[0], "/private")
	if !strings.Contains(pwd, "walk-bench-claude-") {
		t.Errorf("ran in %s", pwd)
	}
	if _, err := os.Stat(pwd); !os.IsNotExist(err) {
		t.Errorf("the working directory %s wasn't removed", pwd)
	}

	// A failed call (a usage limit, say) is an error, not a proposal.
	bin, _ = fakeCLI(t, `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"Usage limit reached"}`)
	if _, err := (&ClaudeCode{Model: "m", Bin: bin}).Propose(context.Background(), walks.Request{}, nil); err == nil ||
		!strings.Contains(err.Error(), "Usage limit") {
		t.Errorf("err = %v", err)
	}
}
