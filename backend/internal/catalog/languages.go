package catalog

import (
	"context"

	"connectrpc.com/connect"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// ListLanguages names languages from CLDR data (golang.org/x/text), not
// machine translation: language names are exactly what CLDR exists for.
func (h *Handler) ListLanguages(_ context.Context, req *connect.Request[isshov1.ListLanguagesRequest]) (*connect.Response[isshov1.ListLanguagesResponse], error) {
	locale, _, err := displayLanguages(req.Msg.Locale, "")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.ListLanguagesResponse{Languages: Languages(locale)}), nil
}

// Languages lists every ISO 639-1 language, named in itself and in locale.
// CLDR has no own-language name for about a third of them (e.g. Latin,
// Xhosa); those show their English name, as does any name CLDR lacks in
// locale.
func Languages(locale string) []*isshov1.Language {
	in := display.Languages(language.Make(locale))
	english := display.Languages(language.English)
	out := make([]*isshov1.Language, 0, len(i18n.Codes()))
	for _, code := range i18n.Codes() {
		tag := language.Make(code)
		l := &isshov1.Language{Code: code, Autonym: display.Self.Name(tag), Name: in.Name(tag)}
		if l.Autonym == "" {
			l.Autonym = english.Name(tag)
		}
		if l.Name == "" {
			l.Name = english.Name(tag)
		}
		if l.Autonym == "" {
			l.Autonym = code
		}
		if l.Name == "" {
			l.Name = l.Autonym
		}
		out = append(out, l)
	}
	return out
}
