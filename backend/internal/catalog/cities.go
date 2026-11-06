package catalog

import (
	"context"
	"encoding/json"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

func (h *Handler) ListCities(ctx context.Context, req *isshov1.ListCitiesRequest) (*isshov1.ListCitiesResponse, error) {
	primary, secondary, err := displayLanguages(req.Locale, req.SecondaryLanguage)
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
	pairs := h.Translations.ResolvePair(ctx, primary, secondary, texts)

	out := make([]*isshov1.City, 0, len(cities))
	byID := map[string]*isshov1.City{}
	for i, c := range cities {
		pc := &isshov1.City{Id: c.ID, Name: pairs[i][0], SecondaryName: pairs[i][1], Languages: c.Languages}
		out = append(out, pc)
		byID[c.ID] = pc
	}
	// Area names are proper nouns and are never machine-translated: it turned
	// "The Domain" into "El dominio". Only their stored names are shown (the
	// English name, plus a real local one such as 吉祥寺).
	for _, a := range areas {
		name, sec := i18n.Pair(stored(a.Name, primary), stored(a.Name, secondary), i18n.Pick(a.Name, i18n.Authoring))
		if c := byID[a.CityID]; c != nil {
			c.Areas = append(c.Areas, &isshov1.Area{Id: a.ID, Name: name, SecondaryName: sec, Lat: a.Lat, Lng: a.Lng})
		}
	}
	return &isshov1.ListCitiesResponse{Cities: out}, nil
}

// displayLanguages validates a request's primary (required) and secondary
// (optional) display languages and applies the app's language rule.
func displayLanguages(locale, secondaryLanguage string) (string, string, error) {
	primary, secondary := baseLang(locale), baseLang(secondaryLanguage)
	if !i18n.ValidDisplay(primary) {
		return "", "", errs.Invalid("locale must be a language code such as \"ja\"")
	}
	if secondary != "" && !i18n.ValidDisplay(secondary) {
		return "", "", errs.Invalid("secondary_language must be a language code such as \"es\"")
	}
	return primary, secondary, nil
}

// stored returns the text stored in exactly lang, or "".
func stored(raw []byte, lang string) string {
	var m map[string]string
	if lang == "" || json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m[lang]
}
