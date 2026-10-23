package claude

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/shrujancheruku/issho/backend/internal/walks"
)

const walkJSON = `{"title":"Temple lanes","summary":"Quiet.","meet":{"name":"Station","search":"Station, A","kind":"station","minutes":10,"note":"Gather."},
"stops":[{"name":"Temple","search":"Temple, A","kind":"temple","minutes":20,"note":"Look."}],"finish":{"name":"Cafe","search":"Cafe, A","kind":"cafe","minutes":40,"note":"Sit."}}`

// fake answers every request with one message and records the request.
func fake(t *testing.T, stop, text string, got *map[string]any) *Proposer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" || r.URL.Path != "/v1/messages" {
			t.Errorf("auth %q, path %s", r.Header.Get("X-Api-Key"), r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(got)
		body, _ := json.Marshal(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
			"content":     []map[string]any{{"type": "thinking", "thinking": "", "signature": "s"}, {"type": "text", "text": text}},
			"stop_reason": stop, "usage": map[string]any{"input_tokens": 1200, "output_tokens": 3400},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return &Proposer{
		Client: anthropic.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0)),
		Model:  "claude-opus-5-5", Effort: "high",
	}
}

func TestPropose(t *testing.T) {
	var req map[string]any
	p := fake(t, "end_turn", walkJSON, &req)
	w, err := p.Propose(context.Background(), walks.Request{City: "Tokyo", Area: "Koenji", Theme: "temples"}, []string{"Hase-ji: not found on Google Maps"})
	if err != nil {
		t.Fatal(err)
	}
	if w.Title != "Temple lanes" || len(w.Stops) != 1 || w.Finish.Kind != "cafe" ||
		w.Meta.Model != "claude-opus-5-5" || w.Meta.InputTokens != 1200 || w.Meta.OutputTokens != 3400 {
		t.Errorf("proposal = %+v", w)
	}
	// Same prompt as every proposer, the strict schema, the effort asked for.
	if sys := req["system"].([]any)[0].(map[string]any)["text"]; sys != walks.SystemPrompt {
		t.Errorf("system = %v", sys)
	}
	oc := req["output_config"].(map[string]any)
	format := oc["format"].(map[string]any)
	if oc["effort"] != "high" || format["type"] != "json_schema" || format["schema"].(map[string]any)["additionalProperties"] != false {
		t.Errorf("output_config = %v", oc)
	}
	user := req["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(user, "Area: Koenji, Tokyo") || !strings.Contains(user, "Hase-ji: not found") {
		t.Errorf("user turn = %q", user)
	}
}

func TestProposeFailures(t *testing.T) {
	var req map[string]any
	ctx := context.Background()
	if _, err := fake(t, "refusal", "", &req).Propose(ctx, walks.Request{}, nil); !errors.Is(err, ErrDeclined) {
		t.Errorf("refusal: %v", err)
	}
	if _, err := fake(t, "max_tokens", `{"title":`, &req).Propose(ctx, walks.Request{}, nil); err == nil {
		t.Error("a cut-off answer must fail")
	}
	if _, err := fake(t, "end_turn", "not json", &req).Propose(ctx, walks.Request{}, nil); err == nil {
		t.Error("an undecodable answer must fail")
	}
}
