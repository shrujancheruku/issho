package google

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/googleapi"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

func newTest(t *testing.T, h http.HandlerFunc) *Translator {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	tr := New("test-key")
	tr.URL = srv.URL
	tr.api.Backoff = time.Millisecond
	return tr
}

// echo "translates" by appending the target language, like the real API's shape.
func echo(t *testing.T, calls *atomic.Int32, maxQ *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != "test-key" || r.URL.Query().Get("key") != "" {
			t.Error("api key must be sent in a header only")
		}
		var req request
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Source != "en" || req.Format != "text" {
			t.Errorf("source/format = %q/%q", req.Source, req.Format)
		}
		calls.Add(1)
		if n := int32(len(req.Q)); n > maxQ.Load() {
			maxQ.Store(n)
		}
		var resp response
		for _, q := range req.Q {
			resp.Data.Translations = append(resp.Data.Translations, struct {
				TranslatedText string `json:"translatedText"`
			}{q + " [" + req.Target + "]"})
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func TestTranslate(t *testing.T) {
	var calls, maxQ atomic.Int32
	tr := newTest(t, echo(t, &calls, &maxQ))
	got, err := tr.Translate(context.Background(), "ja", []translate.Item{{Text: "Bowling"}, {Text: "Rock & roll"}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "Bowling [ja]" || got[1] != "Rock & roll [ja]" {
		t.Errorf("got %q", got)
	}
}

func TestBatchesBySegmentsAndSize(t *testing.T) {
	var calls, maxQ atomic.Int32
	tr := newTest(t, echo(t, &calls, &maxQ))
	items := make([]translate.Item, 300)
	for i := range items {
		items[i] = translate.Item{Text: "Hike"}
	}
	got, err := tr.Translate(context.Background(), "fr", items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 300 || calls.Load() != 3 || maxQ.Load() > maxSegments {
		t.Errorf("300 items: %d results, %d calls, max %d per call", len(got), calls.Load(), maxQ.Load())
	}

	calls.Store(0)
	long := strings.Repeat("x", 3000)
	if _, err := tr.Translate(context.Background(), "fr", []translate.Item{{Text: long}, {Text: long}}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("two 3,000-character items should be split into 2 calls, got %d", calls.Load())
	}
}

func TestUnescapesEntities(t *testing.T) {
	tr := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"translations":[{"translatedText":"Rock &amp; roll"}]}}`))
	})
	got, err := tr.Translate(context.Background(), "fr", []translate.Item{{Text: "Rock & roll"}})
	if err != nil || got[0] != "Rock & roll" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestErrors(t *testing.T) {
	tr := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Bad language pair: en|xx"}}`))
	})
	_, err := tr.Translate(context.Background(), "xx", []translate.Item{{Text: "Hike"}})
	var apiErr *googleapi.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Errorf("got %v, want a 400 APIError", err)
	}

	short := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"translations":[]}}`))
	})
	if _, err := short.Translate(context.Background(), "fr", []translate.Item{{Text: "Hike"}}); err == nil {
		t.Error("a response with the wrong number of translations must fail")
	}
}

func TestMatchCapital(t *testing.T) {
	for _, tc := range []struct{ src, in, want string }{
		{"Indie band concert", "concierto de banda indie", "Concierto de banda indie"},
		{"Golf driving range", "été", "Été"},            // non-ASCII letters too
		{"Pottery class", "陶芸教室", "陶芸教室"},               // no case: unchanged
		{"eBay meetup", "réunion eBay", "réunion eBay"}, // lower-case source: unchanged
		{"Karaoke", "", ""},
	} {
		if got := matchCapital(tc.src, tc.in); got != tc.want {
			t.Errorf("matchCapital(%q, %q) = %q, want %q", tc.src, tc.in, got, tc.want)
		}
	}
}
