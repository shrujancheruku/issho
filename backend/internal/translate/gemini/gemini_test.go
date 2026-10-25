package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// fake answers each generateContent call with every item "translated" as
// "[lang] text", except ids in drop, and records the system prompts.
func fake(t *testing.T, drop map[int]bool) (*Translator, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var systems []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SystemInstruction struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"systemInstruction"`
			Contents []struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		systems = append(systems, req.SystemInstruction.Parts[0].Text)
		mu.Unlock()
		var items []item
		_ = json.Unmarshal([]byte(req.Contents[0].Parts[0].Text), &items)
		var a answer
		for _, it := range items {
			if !drop[it.ID] {
				a.Translations = append(a.Translations, struct {
					ID   int    `json:"id"`
					Text string `json:"text"`
				}{it.ID, "[ja] " + it.Text})
			}
		}
		text, _ := json.Marshal(a)
		body, _ := json.Marshal(map[string]any{"candidates": []map[string]any{{
			"content":      map[string]any{"role": "model", "parts": []map[string]any{{"text": string(text)}}},
			"finishReason": "STOP",
		}}})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	tr := New("test-key")
	tr.Client.URL = srv.URL + "/v1/publishers/google/models/%s:generateContent"
	tr.Client.API.MaxRetries = 0
	tr.Client.API.Backoff = time.Millisecond
	return tr, &systems
}

func TestTranslate(t *testing.T) {
	tr, systems := fake(t, nil)
	items := make([]translate.Item, 90) // three chunks
	for i := range items {
		items[i] = translate.Item{Text: fmt.Sprintf("Text %d", i), Context: "a button"}
	}
	got, err := tr.Translate(context.Background(), "ja", items)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range got {
		if s != fmt.Sprintf("[ja] Text %d", i) {
			t.Fatalf("item %d: %q (order lost?)", i, s)
		}
	}
	if len(*systems) != 3 {
		t.Errorf("%d calls, want 3", len(*systems))
	}
	// The prompt names the language and carries the guide's rules for it.
	sys := (*systems)[0]
	for _, want := range []string{"into Japanese", `"Issho"`, "です/ます", "{placeholder}"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	if tr.Name() != Model {
		t.Errorf("Name() = %q", tr.Name())
	}
}

func TestTranslateMissingItem(t *testing.T) {
	tr, _ := fake(t, map[int]bool{1: true})
	if _, err := tr.Translate(context.Background(), "ja", []translate.Item{{Text: "Save"}, {Text: "Back"}}); err == nil {
		t.Error("a missing translation must fail the call")
	}
}

func TestGuide(t *testing.T) {
	ja := translate.Guide("ja")
	if !strings.Contains(ja, "plan") || !strings.Contains(ja, "です/ます") || strings.Contains(ja, "Du, not Sie") {
		t.Errorf("ja guide:\n%s", ja)
	}
	// A language without its own section gets the shared rules only.
	if sw := translate.Guide("sw"); !strings.Contains(sw, "Issho") || strings.Contains(sw, "For this language") {
		t.Errorf("sw guide:\n%s", sw)
	}
}
