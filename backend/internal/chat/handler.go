package chat

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
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

func (h *Handler) ListMessages(ctx context.Context, req *isshov1.ListMessagesRequest) (*isshov1.ListMessagesResponse, error) {
	userID, room, err := call(ctx, req.Room)
	if err != nil {
		return nil, err
	}
	var msgs []Message
	var st State
	if req.AfterId == 0 {
		msgs, st, err = h.Svc.Latest(ctx, userID, room, h.now())
	} else {
		msgs, st, err = h.Svc.After(ctx, userID, room, req.AfterId, h.now())
	}
	if err != nil {
		return nil, roomError(err)
	}
	out := &isshov1.ListMessagesResponse{Open: st.Open, OpensAt: timestamp(st.OpensAt), ClosesAt: timestamp(st.ClosesAt)}
	for _, m := range msgs {
		out.Messages = append(out.Messages, ToProto(m))
	}
	return out, nil
}

func (h *Handler) SendMessage(ctx context.Context, req *isshov1.SendMessageRequest) (*isshov1.SendMessageResponse, error) {
	userID, room, err := call(ctx, req.Room)
	if err != nil {
		return nil, err
	}
	m, err := h.Svc.Send(ctx, userID, room, req.Body, h.now())
	if err != nil {
		return nil, roomError(err)
	}
	return &isshov1.SendMessageResponse{Message: ToProto(m)}, nil
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
		return uuid.Nil, Room{}, errs.Invalid("room: exactly one of event_id or crew_id, as an id")
	}
	return c.UserID, room, nil
}

func roomError(err error) error {
	switch {
	case errors.Is(err, ErrBadMessage):
		return errs.Invalid("%s", err)
	case errors.Is(err, ErrNoRoom):
		return errs.NotFound("%s", err)
	case errors.Is(err, ErrForbidden):
		return errs.Forbidden("%s", err)
	case errors.Is(err, ErrClosed):
		return errs.Refused("%s", err)
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
