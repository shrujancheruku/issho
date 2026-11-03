package plans

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// Installed builds match these codes and messages; reasons are additive.
func TestEventErrorKeepsCodesAndMessages(t *testing.T) {
	reasonOf := func(ce *connect.Error) isshov1.ErrorReason {
		for _, d := range ce.Details() {
			if m, err := d.Value(); err == nil {
				if rd, ok := m.(*isshov1.ErrorReasonDetail); ok {
					return rd.Reason
				}
			}
		}
		return isshov1.ErrorReason_ERROR_REASON_UNSPECIFIED
	}
	for _, tt := range []struct {
		err    error
		code   connect.Code
		reason isshov1.ErrorReason
	}{
		{Invalid("start: bad"), connect.CodeInvalidArgument, 0},
		{ErrNoEvent, connect.CodeNotFound, 0},
		{ErrNotGoing, connect.CodePermissionDenied, 0},
		{ErrCheckInClosed, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_CHECK_IN_CLOSED},
		{ErrFeedbackClosed, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_FEEDBACK_CLOSED},
		{ErrNoProfile, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_NO_PROFILE},
		{ErrEventOver, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_EVENT_OVER},
		{ErrTooLate, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_TOO_LATE},
		{ErrNotNow, connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_NOT_NOW},
		{fmt.Errorf("wrapped: %w", ErrTooLate), connect.CodeFailedPrecondition, isshov1.ErrorReason_ERROR_REASON_TOO_LATE},
	} {
		got := eventError(tt.err)
		var ce *connect.Error
		if !errors.As(got, &ce) {
			t.Fatalf("%v: not a connect error", tt.err)
		}
		if ce.Code() != tt.code || ce.Message() != tt.err.Error() || reasonOf(ce) != tt.reason {
			t.Errorf("%v: code=%v msg=%q reason=%v", tt.err, ce.Code(), ce.Message(), reasonOf(ce))
		}
		if !errors.Is(got, tt.err) && !errors.As(tt.err, new(*InvalidError)) {
			t.Errorf("%v: sentinel not reachable", tt.err)
		}
	}
	other := errors.New("db down")
	if eventError(other) != other {
		t.Error("unknown errors must pass through")
	}
}
