// Package catalog serves the activity catalogue, localised per city.
package catalog

import (
	"context"
	"errors"
	"strings"
	"sync"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

type Handler struct {
	Q *dbgen.Queries
	// Names are stored in English; other languages come from here.
	Translations *translate.Service
}

func (h *Handler) ListActivities(ctx context.Context, req *connect.Request[isshov1.ListActivitiesRequest]) (*connect.Response[isshov1.ListActivitiesResponse], error) {
	if req.Msg.CityId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("city_id is required"))
	}
	primary, secondary := baseLang(req.Msg.Locale), baseLang(req.Msg.SecondaryLanguage)
	if !i18n.Valid(primary) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("locale must be a language code such as \"ja\""))
	}
	if secondary != "" && !i18n.Valid(secondary) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("secondary_language must be a language code such as \"es\""))
	}
	primary, secondary = i18n.ForApp(primary, secondary)
	rows, err := h.Q.ListCityActivities(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	texts := make([]translate.Text, len(rows))
	for i, r := range rows {
		texts[i] = translate.Text{Raw: r.Name, Context: "name of a group activity, shown on a tile"}
	}
	// Both languages concurrently: on a cold cache each may wait for the translator.
	var primaryNames, secondaryNames []string
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); primaryNames = h.Translations.Resolve(ctx, primary, texts) }()
	go func() { defer wg.Done(); secondaryNames = h.Translations.Resolve(ctx, secondary, texts) }()
	wg.Wait()

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

// baseLang reduces a BCP 47 tag to its lowercase base language ("ja-JP" -> "ja"):
// translations are per language, not per region.
func baseLang(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	return base
}
