package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

func (h *Handler) ListCities(ctx context.Context, req *connect.Request[isshov1.ListCitiesRequest]) (*connect.Response[isshov1.ListCitiesResponse], error) {
	primary, secondary, err := displayLanguages(req.Msg.Locale, req.Msg.SecondaryLanguage)
	if err != nil {
		return nil, err
	}
	cities, err := h.Q.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	areas, err := h.Q.ListAreas(ctx)
	if err != nil {
		return nil, err
	}

	// City names are translated (Tokyo → Tokio); a stored local name (東京)
	// wins over machine translation for its language.
	texts := make([]translate.Text, len(cities))
	for i, c := range cities {
		texts[i] = translate.Text{Raw: c.Name, Context: "name of a city"}
	}
	names, secondaryNames := h.resolvePair(ctx, primary, secondary, texts)

	out := make([]*isshov1.City, 0, len(cities))
	byID := map[string]*isshov1.City{}
	for i, c := range cities {
		name, sec := i18n.Pair(names[i], secondaryNames[i], i18n.Pick(c.Name, i18n.Authoring))
		pc := &isshov1.City{Id: c.ID, Name: name, SecondaryName: sec, Languages: c.Languages}
		out = append(out, pc)
		byID[c.ID] = pc
	}
	// Area names are proper nouns and are never machine-translated: it turned
	// "The Domain" into "El dominio". Only their stored names are shown (the
	// English name, plus a real local one such as 吉祥寺).
	for _, a := range areas {
		name, sec := i18n.Pair(stored(a.Name, primary), stored(a.Name, secondary), i18n.Pick(a.Name, i18n.Authoring))
		if c := byID[a.CityID]; c != nil {
			c.Areas = append(c.Areas, &isshov1.Area{Id: a.ID, Name: name, SecondaryName: sec})
		}
	}
	return connect.NewResponse(&isshov1.ListCitiesResponse{Cities: out}), nil
}

// displayLanguages validates a request's primary (required) and secondary
// (optional) display languages and applies the app's language rule.
func displayLanguages(locale, secondaryLanguage string) (string, string, error) {
	primary, secondary := baseLang(locale), baseLang(secondaryLanguage)
	if !i18n.Valid(primary) {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("locale must be a language code such as \"ja\""))
	}
	if secondary != "" && !i18n.Valid(secondary) {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("secondary_language must be a language code such as \"es\""))
	}
	primary, secondary = i18n.ForApp(primary, secondary)
	return primary, secondary, nil
}

// resolvePair resolves texts into both languages concurrently: on a cold
// cache each may wait for the translator.
func (h *Handler) resolvePair(ctx context.Context, primary, secondary string, texts []translate.Text) ([]string, []string) {
	var p, s []string
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p = h.Translations.Resolve(ctx, primary, texts) }()
	go func() { defer wg.Done(); s = h.Translations.Resolve(ctx, secondary, texts) }()
	wg.Wait()
	return p, s
}

// stored returns the text stored in exactly lang, or "".
func stored(raw []byte, lang string) string {
	var m map[string]string
	if lang == "" || json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m[lang]
}
