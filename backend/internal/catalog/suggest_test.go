package catalog

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/tagging"
)

// Without a model key the RPC keeps FailedPrecondition (installed builds read
// the code) and adds the reason newer builds word it by.
func TestSuggestActivitiesUnavailableNamesItsReason(t *testing.T) {
	h := &Handler{Suggestions: &tagging.Service{}}
	_, err := h.SuggestActivities(context.Background(), &isshov1.SuggestActivitiesRequest{CityId: "tokyo", Text: "hiking"})
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	if ce.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code = %v, want failed_precondition", ce.Code())
	}
	var got isshov1.ErrorReason
	for _, d := range ce.Details() {
		m, verr := connectproto.UnmarshalErrorDetail(d)
		if verr != nil {
			t.Fatal(verr)
		}
		if rd, ok := m.(*isshov1.ErrorReasonDetail); ok {
			got = rd.Reason
		}
	}
	if got != isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE {
		t.Errorf("reason = %v, want FEATURE_UNAVAILABLE", got)
	}
}
