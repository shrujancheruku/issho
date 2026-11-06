package uistrings

import (
	"context"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// Handler serves UIService: the app's UI text in the reader's language.
type Handler struct {
	Catalog      *Catalog
	Translations *translate.Service
}

func (h *Handler) GetStrings(ctx context.Context, req *isshov1.GetStringsRequest) (*isshov1.GetStringsResponse, error) {
	lang := i18n.Display(req.Language)
	if !i18n.ValidDisplay(lang) {
		return nil, errs.Invalid("language must be a language code such as \"ja\"")
	}
	strs, complete := h.Catalog.In(ctx, h.Translations, lang)
	version := Version(strs)
	// The app checks a complete copy every so often; most checks find it
	// current, and then there's no need to send ~300 strings again.
	if complete && req.KnownVersion == version {
		return &isshov1.GetStringsResponse{Complete: true, CatalogVersion: version, Unchanged: true}, nil
	}
	return &isshov1.GetStringsResponse{
		Strings:        strs,
		Complete:       complete,
		CatalogVersion: version,
	}, nil
}
