// Package catalog serves reference data for onboarding and settings: the
// activity catalogue (localised per city), cities and areas, and languages.
package catalog

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/tagging"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

type Handler struct {
	Q *dbgen.Queries
	// Names are stored in English; other languages come from here.
	Translations *translate.Service
	// Suggestions maps free text to activities (SuggestActivities).
	Suggestions *tagging.Service
}

func (h *Handler) ListActivities(ctx context.Context, req *connect.Request[isshov1.ListActivitiesRequest]) (*connect.Response[isshov1.ListActivitiesResponse], error) {
	if req.Msg.CityId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("city_id is required"))
	}
	primary, secondary, err := displayLanguages(req.Msg.Locale, req.Msg.SecondaryLanguage)
	if err != nil {
		return nil, err
	}
	rows, err := h.Q.ListCityActivities(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	texts := make([]translate.Text, len(rows))
	for i, r := range rows {
		texts[i] = translate.Text{Raw: r.Name, Context: "name of a group activity, shown on a tile"}
	}
	primaryNames, secondaryNames := h.resolvePair(ctx, primary, secondary, texts)

	out := make([]*isshov1.Activity, 0, len(rows))
	for i, r := range rows {
		name, secondaryName := i18n.Pair(primaryNames[i], secondaryNames[i], i18n.Pick(r.Name, i18n.Authoring))
		out = append(out, &isshov1.Activity{
			Id:             r.ID,
			Name:           name,
			SecondaryName:  secondaryName,
			Category:       r.Category,
			Indoor:         r.Indoor,
			Energy:         int32(r.Energy),
			CostLevel:      int32(r.CostLevel),
			Conversational: r.Conversational,
			MinGroupSize:   int32(r.MinGroupSize),
			MaxGroupSize:   int32(r.MaxGroupSize),
		})
	}
	return connect.NewResponse(&isshov1.ListActivitiesResponse{Activities: out}), nil
}

// baseLang reduces a BCP 47 tag to a display language ("ja-JP" -> "ja",
// "zh-TW" -> "zh-Hant"): translations are per language, not per region.
func baseLang(tag string) string { return i18n.Display(tag) }
