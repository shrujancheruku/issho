package catalog

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/tagging"
)

// Interest levels by preference weight (app.user_activity_prefs.weight).
var interestByWeight = []isshov1.Interest{
	isshov1.Interest_INTEREST_OPEN_TO_TRY,
	isshov1.Interest_INTEREST_LIKE,
	isshov1.Interest_INTEREST_LOVE,
}

func (h *Handler) SuggestActivities(ctx context.Context, req *connect.Request[isshov1.SuggestActivitiesRequest]) (*connect.Response[isshov1.SuggestActivitiesResponse], error) {
	tags, err := h.Suggestions.Suggest(ctx, req.Msg.CityId, req.Msg.Text)
	var invalid *tagging.InvalidError
	switch {
	case errors.As(err, &invalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, tagging.ErrUnavailable):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, tagging.ErrNoSuggestion):
		// The model declined; to the person that's just "nothing matched".
		tags = nil
	case err != nil:
		return nil, err
	}
	out := make([]*isshov1.ActivityPreference, len(tags))
	for i, t := range tags {
		out[i] = &isshov1.ActivityPreference{ActivityId: t.ActivityID, Interest: interestByWeight[t.Weight]}
	}
	return connect.NewResponse(&isshov1.SuggestActivitiesResponse{Suggestions: out}), nil
}
