package chat

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// Handler serves ChatService.
type Handler struct {
	Svc *Service
	Now func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ListMessages(ctx context.Context, req *connect.Request[isshov1.ListMessagesRequest]) (*connect.Response[isshov1.ListMessagesResponse], error) {
	userID, room, err := call(ctx, req.Msg.Room)
	if err != nil {
		return nil, err
	}
	var msgs []Message
	var st State
	if req.Msg.AfterId == 0 {
		msgs, st, err = h.Svc.Latest(ctx, userID, room, h.now())
	} else {
		msgs, st, err = h.Svc.After(ctx, userID, room, req.Msg.AfterId, h.now())
	}
	if err != nil {
		return nil, roomError(err)
	}
	out := &isshov1.ListMessagesResponse{Open: st.Open, OpensAt: timestamp(st.OpensAt), ClosesAt: timestamp(st.ClosesAt)}
	for _, m := range msgs {
		out.Messages = append(out.Messages, ToProto(m))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SendMessage(ctx context.Context, req *connect.Request[isshov1.SendMessageRequest]) (*connect.Response[isshov1.SendMessageResponse], error) {
	userID, room, err := call(ctx, req.Msg.Room)
	if err != nil {
		return nil, err
	}
	m, err := h.Svc.Send(ctx, userID, room, req.Msg.Body, h.now())
	if err != nil {
		return nil, roomError(err)
	}
	return connect.NewResponse(&isshov1.SendMessageResponse{Message: ToProto(m)}), nil
}

// ParseRoom reads a room from the API's ChatRoom, or the socket's hello
// (kind and id as strings). It fails with ErrNoRoom for an id that isn't one.
func ParseRoom(eventID, crewID string) (Room, error) {
	switch {
	case eventID != "" && crewID == "":
		id, err := uuid.Parse(eventID)
		return Room{Kind: EventRoom, ID: id}, noRoomIf(err)
	case crewID != "" && eventID == "":
		id, err := uuid.Parse(crewID)
		return Room{Kind: CrewRoom, ID: id}, noRoomIf(err)
	}
	return Room{}, ErrNoRoom
}

func noRoomIf(err error) error {
	if err != nil {
		return ErrNoRoom
	}
	return nil
}

func call(ctx context.Context, r *isshov1.ChatRoom) (uuid.UUID, Room, error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return uuid.Nil, Room{}, err
	}
	room, err := ParseRoom(r.GetEventId(), r.GetCrewId())
	if err != nil {
		return uuid.Nil, Room{}, connect.NewError(connect.CodeInvalidArgument, errors.New("room: exactly one of event_id or crew_id, as an id"))
	}
	return c.UserID, room, nil
}

func roomError(err error) error {
	switch {
	case errors.Is(err, ErrBadMessage):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNoRoom):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, ErrClosed):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return err
}

// ToProto is a message as the API sends it.
func ToProto(m Message) *isshov1.ChatMessage {
	out := &isshov1.ChatMessage{Id: m.ID, Name: m.Name, Body: m.Body, At: timestamppb.New(m.At), Mine: m.Mine}
	if m.UserID != uuid.Nil {
		out.UserId = m.UserID.String()
	}
	if m.PlanID != uuid.Nil {
		out.PlanId = m.PlanID.String()
	}
	return out
}

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
