package translate

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// langTranslator is a Translator that fails for some languages, and can hold
// every call until `want` calls are in flight (a barrier).
type langTranslator struct {
	failFor map[string]bool
	want    int32
	arrived atomic.Int32
	all     chan struct{}
	once    sync.Once
}

func (l *langTranslator) Name() string { return "lang-fake" }

func (l *langTranslator) Translate(_ context.Context, lang string, items []Item) ([]string, error) {
	if l.want > 0 {
		if l.arrived.Add(1) >= l.want {
			l.once.Do(func() { close(l.all) })
		}
		select {
		case <-l.all:
		case <-time.After(2 * time.Second):
			return nil, errors.New("the other language was not looked up at the same time")
		}
	}
	if l.failFor[lang] {
		return nil, errors.New("down")
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = lang + ":" + it.Text
	}
	return out, nil
}

func TestResolvePair(t *testing.T) {
	const karaoke = `{"en":"Karaoke","ja":"カラオケ"}`
	tests := []struct {
		name               string
		translator         Translator
		primary, secondary string
		raw                string
		want               [2]string
	}{
		{"both available", &langTranslator{}, "ja", "es", karaoke, [2]string{"カラオケ", "es:Karaoke"}},
		{"primary missing: the secondary alone", &langTranslator{failFor: map[string]bool{"fr": true}}, "fr", "es", karaoke, [2]string{"es:Karaoke", ""}},
		{"neither available: English", nil, "fr", "es", `{"en":"Karaoke"}`, [2]string{"Karaoke", ""}},
		{"no secondary language", &langTranslator{}, "es", "", karaoke, [2]string{"es:Karaoke", ""}},
		{"secondary the same as primary", &langTranslator{}, "es", "es", karaoke, [2]string{"es:Karaoke", ""}},
		{"same words in both languages are shown once", &langTranslator{}, "ja", "en", `{"en":"カラオケ","ja":"カラオケ"}`, [2]string{"カラオケ", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{Store: &memStore{}}
			if tt.translator != nil {
				s.Translator = tt.translator
			}
			got := s.ResolvePair(context.Background(), tt.primary, tt.secondary, []Text{{Raw: []byte(tt.raw), Context: "test"}})
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePairNilService(t *testing.T) {
	var s *Service
	got := s.ResolvePair(context.Background(), "ja", "es", []Text{{Raw: []byte(`{"en":"Karaoke"}`)}})
	if got[0] != [2]string{"Karaoke", ""} {
		t.Errorf("got %q, want English alone", got)
	}
}

func TestResolvePairLooksUpBothLanguagesAtOnce(t *testing.T) {
	// Each translator call waits for the other language's call, so this only
	// gets both translations if the two lookups run concurrently.
	tr := &langTranslator{want: 2, all: make(chan struct{})}
	s := &Service{Store: &memStore{}, Translator: tr}
	got := s.ResolvePair(context.Background(), "ja", "es", []Text{{Raw: []byte(`{"en":"Pottery"}`), Context: "test"}})
	if want := ([2]string{"ja:Pottery", "es:Pottery"}); got[0] != want {
		t.Errorf("got %q, want %q (looked up one after the other?)", got[0], want)
	}
}
