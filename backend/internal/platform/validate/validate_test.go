package validate_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/validate"
)

// call sends req through the same order handlerOptions builds: errs outside,
// validation inside it. It reports whether the handler was reached.
func call(t *testing.T, req connect.AnyRequest) (reached bool, err error) {
	t.Helper()
	v := validate.Interceptor()
	handler := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		reached = true
		return nil, nil
	}
	chain := errs.Interceptor(slog.New(slog.NewTextHandler(io.Discard, nil)))(v(handler))
	_, err = chain(context.Background(), req)
	return reached, err
}

func chatBody(s string) connect.AnyRequest {
	return connect.NewRequest(&isshov1.SendMessageRequest{
		Room: &isshov1.ChatRoom{Room: &isshov1.ChatRoom_EventId{EventId: "e"}}, Body: s,
	})
}

func profileName(s string) connect.AnyRequest {
	return connect.NewRequest(&isshov1.SaveProfileRequest{Profile: &isshov1.Profile{DisplayName: s}})
}

func TestLimitsAreEnforcedAsInvalidArgument(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name  string
		req   connect.AnyRequest
		field string // the field the message must name; "" when the request passes
	}{
		{"chat body at the limit", chatBody(long(1000)), ""},
		// 1000 characters of 3 bytes each: the limit counts characters, not bytes.
		{"chat body of 1000 multibyte characters", chatBody(strings.Repeat("あ", 1000)), ""},
		{"chat body too long", chatBody(long(1001)), "body"},
		{"chat body empty", chatBody(""), "body"},
		{"report details at the limit", connect.NewRequest(&isshov1.ReportPersonRequest{Details: long(1000)}), ""},
		{"report details too long", connect.NewRequest(&isshov1.ReportPersonRequest{Details: long(1001)}), "details"},
		{"crew report details too long", connect.NewRequest(&isshov1.ReportCrewMemberRequest{Details: long(1001)}), "details"},
		{"leave details too long", connect.NewRequest(&isshov1.LeaveCrewRequest{Details: long(1001)}), "details"},
		{"display name at the limit", profileName(long(40)), ""},
		{"display name too long", profileName(long(41)), "profile.display_name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reached, err := call(t, tc.req)
			if tc.field == "" {
				if err != nil || !reached {
					t.Fatalf("reached=%v err=%v, want the handler reached", reached, err)
				}
				return
			}
			if reached || connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("reached=%v code=%v, want InvalidArgument without reaching the handler", reached, connect.CodeOf(err))
			}
			if !strings.Contains(err.Error(), tc.field+": ") {
				t.Errorf("message %q should name the field %q", err, tc.field)
			}
		})
	}
}
