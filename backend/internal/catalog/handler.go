// Package catalog serves the activity catalogue, localised per city.
package catalog

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

type Handler struct {
	Q *dbgen.Queries
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
	out := make([]*isshov1.Activity, 0, len(rows))
	for _, r := range rows {
		name, secondaryName := i18n.Pair(r.Name, primary, secondary)
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
// the catalogue is translated per language, not per region.
func baseLang(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	return base
}
