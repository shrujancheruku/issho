// Package catalog serves the activity catalogue, localised per city.
package catalog

import (
	"context"
	"errors"

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
	rows, err := h.Q.ListCityActivities(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	out := make([]*isshov1.Activity, 0, len(rows))
	for _, r := range rows {
		out = append(out, &isshov1.Activity{
			Id:             r.ID,
			Name:           i18n.Pick(r.Name, req.Msg.Locale),
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
