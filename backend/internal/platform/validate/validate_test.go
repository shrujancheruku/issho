package validate_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/validate"
)

// call sends req through the same order newRPCServer builds: errs outside,
// validation inside it. Like a generated handler, ours reads the request
// first. It reports whether the handler was reached.
func call(t *testing.T, req proto.Message) (reached bool, err error) {
	t.Helper()
	v := validate.Interceptor()
	handler := func(_ context.Context, _ connect.Spec, stream connect.ServerStream) error {
		msg := req.ProtoReflect().New().Interface()
		if err := stream.Receive(msg); err != nil {
			return err
		}
		reached = true
		return nil
	}
	chain := errs.Interceptor(slog.New(slog.NewTextHandler(io.Discard, nil)))(v(handler))
	err = chain(context.Background(), connect.Spec{}, oneRequest{req})
	return reached, err
}

// oneRequest is a stream that delivers a single request message.
type oneRequest struct{ req proto.Message }

func (s oneRequest) Receive(msg any) error {
	proto.Merge(msg.(proto.Message), s.req)
	return nil
}
func (oneRequest) SendHeaders() error { return nil }
func (oneRequest) Send(any) error     { return nil }

func chatBody(s string) proto.Message {
	return &isshov1.SendMessageRequest{
		Room: &isshov1.ChatRoom{Room: &isshov1.ChatRoom_EventId{EventId: "e"}}, Body: s,
	}
}

func profileName(s string) proto.Message {
	return &isshov1.SaveProfileRequest{Profile: &isshov1.Profile{DisplayName: s}}
}

// packMonths is a city-pack proposal whose first override is in season in months.
func packMonths(months ...int32) proto.Message {
	return &isshov1.ProposeCityPackRequest{Pack: &isshov1.CityPack{
		ActivityOverrides: []*isshov1.PackOverride{{ActivityId: "a", Months: months}},
	}}
}

func TestLimitsAreEnforcedAsInvalidArgument(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name  string
		req   proto.Message
		field string // the field the message must name; "" when the request passes
	}{
		{"chat body at the limit", chatBody(long(1000)), ""},
		// 1000 characters of 3 bytes each: the limit counts characters, not bytes.
		{"chat body of 1000 multibyte characters", chatBody(strings.Repeat("あ", 1000)), ""},
		{"chat body too long", chatBody(long(1001)), "body"},
		{"chat body empty", chatBody(""), "body"},
		{"report details at the limit", &isshov1.ReportPersonRequest{Details: long(1000)}, ""},
		{"report details too long", &isshov1.ReportPersonRequest{Details: long(1001)}, "details"},
		{"crew report details too long", &isshov1.ReportCrewMemberRequest{Details: long(1001)}, "details"},
		{"leave details too long", &isshov1.LeaveCrewRequest{Details: long(1001)}, "details"},
		{"display name at the limit", profileName(long(40)), ""},
		{"display name too long", profileName(long(41)), "profile.display_name"},
		{"pack months 1 to 12", packMonths(1, 12), ""},
		{"pack month 0", packMonths(0), "pack.activity_overrides[0].months[0]"},
		{"pack month 13", packMonths(13), "pack.activity_overrides[0].months[0]"},
		// 65537 wraps to 1 in int16, so the handler's own check would have passed it.
		{"pack month 65537", packMonths(65537), "pack.activity_overrides[0].months[0]"},
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
