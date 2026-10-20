package claude

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// fakeAPI answers like the Messages API with the given stop reason and text.
func fakeAPI(t *testing.T, stop, text string, check func(body map[string]any)) *Translator {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if check != nil {
			check(body)
		}
		resp := map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": Model,
			"stop_reason": stop, "content": []any{map[string]any{"type": "text", "text": text}},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
		}
		if stop == "refusal" {
			resp["stop_details"] = map[string]any{"type": "refusal", "category": "cyber"}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return New("test-key", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
}

func TestTranslate(t *testing.T) {
	tr := fakeAPI(t, "end_turn",
		// Out of order, to prove matching is by id.
		`{"translations":[{"id":1,"text":" 陶芸 "},{"id":0,"text":"ボウリング"}]}`,
		func(body map[string]any) {
			if body["model"] != Model {
				t.Errorf("model = %v", body["model"])
			}
			oc, _ := body["output_config"].(map[string]any)
			format, _ := oc["format"].(map[string]any)
			if format["type"] != "json_schema" || oc["effort"] != "low" {
				t.Errorf("output_config = %v", oc)
			}
			msgs, _ := json.Marshal(body["messages"])
			if !strings.Contains(string(msgs), "Target language: ja") || !strings.Contains(string(msgs), "Pottery") {
				t.Errorf("prompt is missing the language or items: %s", msgs)
			}
		})
	got, err := tr.Translate(context.Background(), "ja", []translate.Item{{Text: "Bowling"}, {Text: "Pottery"}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "ボウリング" || got[1] != "陶芸" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateMissingItemStaysEmpty(t *testing.T) {
	tr := fakeAPI(t, "end_turn", `{"translations":[{"id":0,"text":"Randonnée"}]}`, nil)
	got, err := tr.Translate(context.Background(), "fr", []translate.Item{{Text: "Hike"}, {Text: "Bowling"}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "Randonnée" || got[1] != "" {
		t.Errorf("got %q; a missing item must stay empty so it isn't cached", got)
	}
}

func TestTranslateFailures(t *testing.T) {
	for name, tc := range map[string]struct{ stop, text string }{
		"refusal":    {"refusal", ""},
		"max tokens": {"max_tokens", `{"translations":[`},
		"empty":      {"end_turn", `{"translations":[]}`},
	} {
		tr := fakeAPI(t, tc.stop, tc.text, nil)
		if _, err := tr.Translate(context.Background(), "fr", []translate.Item{{Text: "Hike"}}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
