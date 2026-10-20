package translate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

type memStore struct {
	mu   sync.Mutex
	rows map[string]string // lang/hash -> text
}

func (m *memStore) GetTranslations(_ context.Context, arg dbgen.GetTranslationsParams) ([]dbgen.GetTranslationsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []dbgen.GetTranslationsRow
	for _, h := range arg.SourceHashes {
		if t, ok := m.rows[arg.Lang+"/"+h]; ok {
			out = append(out, dbgen.GetTranslationsRow{SourceHash: h, Text: t})
		}
	}
	return out, nil
}

func (m *memStore) InsertTranslation(_ context.Context, arg dbgen.InsertTranslationParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows == nil {
		m.rows = map[string]string{}
	}
	if _, ok := m.rows[arg.Lang+"/"+arg.SourceHash]; !ok {
		m.rows[arg.Lang+"/"+arg.SourceHash] = arg.Text
	}
	return nil
}

func (m *memStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rows)
}

// fakeTranslator returns "<lang>:<text>", optionally after a delay or with an error.
type fakeTranslator struct {
	calls, items atomic.Int32
	delay        time.Duration
	err          error
}

func (f *fakeTranslator) Name() string { return "fake" }

func (f *fakeTranslator) Translate(ctx context.Context, lang string, items []Item) ([]string, error) {
	f.calls.Add(1)
	f.items.Add(int32(len(items)))
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = lang + ":" + it.Text
	}
	return out, nil
}

func texts(names ...string) []Text {
	out := make([]Text, len(names))
	for i, n := range names {
		out[i] = Text{Raw: []byte(n), Context: "test"}
	}
	return out
}

func TestResolveOrder(t *testing.T) {
	store, tr := &memStore{}, &fakeTranslator{}
	s := &Service{Store: store, Translator: tr}
	// Pre-cache "Bowling" in ja.
	_ = store.InsertTranslation(context.Background(), dbgen.InsertTranslationParams{SourceHash: Key("Bowling", "test"), Lang: "ja", Text: "ボウリング"})

	got := s.Resolve(context.Background(), "ja", texts(
		`{"en":"Kichijoji","ja":"吉祥寺"}`, // stored in ja: used as is
		`{"en":"Bowling"}`,               // cached
		`{"en":"Pottery"}`,               // translated now
		`{"en":"Pottery"}`,               // same text twice: translated once
	))
	want := []string{"吉祥寺", "ボウリング", "ja:Pottery", "ja:Pottery"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
	if tr.items.Load() != 1 {
		t.Errorf("translator got %d items, want 1 (only the uncached one, once)", tr.items.Load())
	}

	// Second request: everything cached, no translator call.
	s.Resolve(context.Background(), "ja", texts(`{"en":"Pottery"}`))
	if tr.calls.Load() != 1 {
		t.Errorf("cached text was translated again: %d calls", tr.calls.Load())
	}
}

func TestResolveEnglishAndEmpty(t *testing.T) {
	tr := &fakeTranslator{}
	s := &Service{Store: &memStore{}, Translator: tr}
	if got := s.Resolve(context.Background(), "en", texts(`{"en":"Hike"}`)); got[0] != "Hike" {
		t.Errorf("English must come from storage, got %q", got[0])
	}
	if got := s.Resolve(context.Background(), "", texts(`{"en":"Hike"}`)); got[0] != "" {
		t.Errorf("no language must resolve to empty, got %q", got[0])
	}
	if tr.calls.Load() != 0 {
		t.Error("translator must not be called for English or no language")
	}
}

func TestResolveWithoutTranslator(t *testing.T) {
	s := &Service{Store: &memStore{}}
	if got := s.Resolve(context.Background(), "fr", texts(`{"en":"Hike"}`)); got[0] != "" {
		t.Errorf("got %q, want empty (caller falls back)", got[0])
	}
}

func TestResolveFailureIsNotCached(t *testing.T) {
	store := &memStore{}
	s := &Service{Store: store, Translator: &fakeTranslator{err: errors.New("boom")}}
	if got := s.Resolve(context.Background(), "fr", texts(`{"en":"Hike"}`)); got[0] != "" {
		t.Errorf("got %q, want empty", got[0])
	}
	if store.count() != 0 {
		t.Error("a failed translation must not be cached")
	}
}

func TestSlowTranslationFinishesInBackground(t *testing.T) {
	store := &memStore{}
	tr := &fakeTranslator{delay: 100 * time.Millisecond}
	s := &Service{Store: store, Translator: tr, Wait: 10 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	if got := s.Resolve(ctx, "fr", texts(`{"en":"Hike"}`)); got[0] != "" {
		t.Errorf("got %q before the translator finished", got[0])
	}
	cancel() // the request is gone; the translation must still complete

	deadline := time.Now().Add(2 * time.Second)
	for store.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := s.Resolve(context.Background(), "fr", texts(`{"en":"Hike"}`)); got[0] != "fr:Hike" {
		t.Errorf("after background completion got %q", got[0])
	}
	if tr.calls.Load() != 1 {
		t.Errorf("got %d translator calls, want 1", tr.calls.Load())
	}
}

func TestConcurrentRequestsTranslateOnce(t *testing.T) {
	tr := &fakeTranslator{delay: 50 * time.Millisecond}
	s := &Service{Store: &memStore{}, Translator: tr, Wait: time.Second}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := s.Resolve(context.Background(), "ko", texts(`{"en":"Hike"}`)); got[0] != "ko:Hike" {
				t.Errorf("got %q", got[0])
			}
		}()
	}
	wg.Wait()
	if tr.calls.Load() != 1 {
		t.Errorf("10 concurrent requests made %d translator calls, want 1", tr.calls.Load())
	}
}

func TestBatches(t *testing.T) {
	tr := &fakeTranslator{}
	s := &Service{Store: &memStore{}, Translator: tr}
	var names []string
	for i := range 230 {
		names = append(names, fmt.Sprintf(`{"en":"Activity %d"}`, i))
	}
	got := s.Resolve(context.Background(), "es", texts(names...))
	if tr.calls.Load() != 3 {
		t.Errorf("230 items made %d calls, want 3 batches", tr.calls.Load())
	}
	if got[229] != "es:Activity 229" {
		t.Errorf("last item %q: batches must keep positions", got[229])
	}
}

func TestPrefill(t *testing.T) {
	s := &Service{Store: &memStore{}, Translator: &fakeTranslator{}, Wait: time.Millisecond}
	raw, err := s.Prefill(context.Background(), "Language exchange", "event title", []string{"ja", "es", "en"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"en":"Language exchange"`, `"ja":"ja:Language exchange"`, `"es":"es:Language exchange"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("prefill %s missing %s", raw, want)
		}
	}

	failing := &Service{Store: &memStore{}, Translator: &fakeTranslator{err: errors.New("down")}}
	if _, err := failing.Prefill(context.Background(), "Hike", "event title", []string{"ja"}); err == nil {
		t.Error("prefill must fail when a required language can't be produced")
	}
}
