// Package chat is the app's chat rooms: an event group's and a crew's. A
// room is a kind plus an id (Room); this package owns listing, posting
// (cleaning, limits, the NOTIFY) and the cursor for both, and each owner
// decides who may use its rooms by plugging in an Access (plans for event
// rooms, crews for crew rooms). chat imports neither; cmd/api wires them.
//
// A message posted to either kind NOTIFYs the "chat" channel with the
// room's Topic (the event group's or the crew's id), which internal/chatws
// relays to open sockets.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

const (
	// OpensBefore and ClosesAfter set when a group can write in its chat,
	// around the event's start: time to plan the meeting, and for "running
	// late" and "thanks" after. It stays readable afterwards until the
	// messages are deleted (Keep).
	OpensBefore = 48 * time.Hour
	ClosesAfter = 24 * time.Hour
	// Keep is how long after the event its group's messages are kept, and
	// after a crew closes its chat.
	Keep = 30 * 24 * time.Hour
	// DeleteBatch is how many rows one retention delete removes at most.
	DeleteBatch = 1000
	// Page is how many messages one read returns at most.
	Page = 100
)

var (
	// ErrNoRoom means there's no such room (for this caller).
	ErrNoRoom = errors.New("chat not found")
	// ErrForbidden means the caller isn't, or is no longer, allowed in the room.
	ErrForbidden = errors.New("only people in this chat can use it")
	// ErrClosed means the room isn't open for writing now.
	ErrClosed = errors.New("the chat is not open")
	// ErrBadMessage means the message is empty once trimmed, or not valid UTF-8.
	// (Its upper length is the body's max_len rule in chat.proto.)
	ErrBadMessage = errors.New("message: must not be empty")
)

// Kind is what owns a room.
type Kind int

const (
	// EventRoom is an event's group chat; the room id is the event's.
	EventRoom Kind = iota + 1
	// CrewRoom is a crew's chat; the room id is the crew's.
	CrewRoom
)

// Room is one chat room.
type Room struct {
	Kind Kind
	ID   uuid.UUID
}

// State is what a room's owner says about the caller's use of it right now.
type State struct {
	// Topic is what the room's messages are stored under and NOTIFY names:
	// the event's group id, or the crew's id. Both are uuids, so they never
	// collide.
	Topic uuid.UUID
	// Open is whether the caller may write now. Reading is allowed whenever
	// Access returns no error.
	Open bool
	// OpensAt and ClosesAt are when the room is open for writing; zero when
	// it has no schedule.
	OpensAt, ClosesAt time.Time
}

// Access is how a room's owner lets people in. It returns ErrNoRoom,
// ErrForbidden or another error when the caller can't read the room.
type Access interface {
	Access(ctx context.Context, q *dbgen.Queries, userID, roomID uuid.UUID, now time.Time) (State, error)
}

// Message is one message as the caller sees it: someone's text, or (in a
// crew's room) a plan suggested by Issho, with no sender.
type Message struct {
	ID     int64
	UserID uuid.UUID // the sender; uuid.Nil for a suggested plan or a deleted account
	Name   string    // the sender's first name
	Body   string
	At     time.Time
	Mine   bool
	PlanID uuid.UUID
}

// Typing is someone typing in a room, as passed between API instances on
// the "chat_typing" channel.
type Typing struct {
	Topic uuid.UUID `json:"t"`
	User  uuid.UUID `json:"u"`
	Name  string    `json:"n"`
}

// Service reads and writes rooms for one caller at a time.
type Service struct {
	DB          *pgxpool.Pool
	Event, Crew Access
}

func (s *Service) access(kind Kind) (Access, error) {
	switch kind {
	case EventRoom:
		return s.Event, nil
	case CrewRoom:
		return s.Crew, nil
	}
	return nil, ErrNoRoom
}

// Clean trims a message and reports whether it's one that can be sent: not
// empty, valid UTF-8. The length limit is not here: it is the max_len rule on
// SendMessageRequest.body, enforced by the validate interceptor.
func Clean(body string) (string, bool) {
	body = strings.TrimSpace(body)
	return body, body != "" && utf8.ValidString(body)
}

// State returns what the caller may do in a room, or why they can't read it.
func (s *Service) State(ctx context.Context, userID uuid.UUID, room Room, now time.Time) (State, error) {
	return s.state(ctx, dbgen.New(s.DB), userID, room, now)
}

func (s *Service) state(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, room Room, now time.Time) (State, error) {
	a, err := s.access(room.Kind)
	if err != nil {
		return State{}, err
	}
	return a.Access(ctx, q, userID, room.ID, now)
}

// Latest returns the room's latest Page messages, oldest first, and its state.
func (s *Service) Latest(ctx context.Context, userID uuid.UUID, room Room, now time.Time) ([]Message, State, error) {
	return s.list(ctx, userID, room, 0, true, now)
}

// After returns up to Page of the room's messages after message afterID,
// oldest first, and its state. Reading until fewer than Page come back
// drains the room. Since posting locks the room (see Post), messages
// commit in id order, so what's after the last id seen is all that's new.
func (s *Service) After(ctx context.Context, userID uuid.UUID, room Room, afterID int64, now time.Time) ([]Message, State, error) {
	return s.list(ctx, userID, room, afterID, false, now)
}

func (s *Service) list(ctx context.Context, userID uuid.UUID, room Room, afterID int64, latest bool, now time.Time) ([]Message, State, error) {
	q := dbgen.New(s.DB)
	st, err := s.state(ctx, q, userID, room, now)
	if err != nil {
		return nil, st, err
	}
	var msgs []Message
	switch room.Kind {
	case EventRoom:
		msgs, err = listEvent(ctx, q, st.Topic, userID, afterID, latest)
	case CrewRoom:
		msgs, err = listCrew(ctx, q, st.Topic, userID, afterID, latest)
	}
	return msgs, st, err
}

func listEvent(ctx context.Context, q *dbgen.Queries, groupID, userID uuid.UUID, afterID int64, latest bool) ([]Message, error) {
	var rows []dbgen.ListChatMessagesRow
	if latest {
		rs, err := q.ListLatestChatMessages(ctx, dbgen.ListLatestChatMessagesParams{GroupID: groupID, Max: Page})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, dbgen.ListChatMessagesRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListChatMessages(ctx, dbgen.ListChatMessagesParams{GroupID: groupID, AfterID: afterID, Max: Page}); err != nil {
			return nil, err
		}
	}
	out := make([]Message, len(rows))
	for i, r := range rows {
		out[i] = Message{ID: r.ID, UserID: r.UserID, Name: deref(r.DisplayName), Body: r.Body, At: r.CreatedAt.Time, Mine: r.UserID == userID}
	}
	return out, nil
}

func listCrew(ctx context.Context, q *dbgen.Queries, crewID, userID uuid.UUID, afterID int64, latest bool) ([]Message, error) {
	var rows []dbgen.ListCrewMessagesRow
	if latest {
		rs, err := q.ListLatestCrewMessages(ctx, dbgen.ListLatestCrewMessagesParams{CrewID: crewID, Max: Page})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, dbgen.ListCrewMessagesRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListCrewMessages(ctx, dbgen.ListCrewMessagesParams{CrewID: crewID, AfterID: afterID, Max: Page}); err != nil {
			return nil, err
		}
	}
	out := make([]Message, len(rows))
	for i, r := range rows {
		out[i] = Message{
			ID: r.ID, UserID: r.UserID.Bytes, Name: deref(r.DisplayName), Body: r.Body, At: r.CreatedAt.Time,
			Mine: r.UserID.Valid && r.UserID.Bytes == userID, PlanID: r.PlanID.Bytes,
		}
	}
	return out, nil
}

// Send posts a message to a room the caller may write in, and tells every
// API instance (NOTIFY) so open chats update straight away.
func (s *Service) Send(ctx context.Context, userID uuid.UUID, room Room, body string, now time.Time) (Message, error) {
	body, ok := Clean(body)
	if !ok {
		return Message{}, ErrBadMessage
	}
	var msg Message
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		st, err := s.state(ctx, q, userID, room, now)
		if err != nil {
			return err
		}
		if !st.Open {
			return ErrClosed
		}
		if err := lock(ctx, q, room.Kind, st.Topic); err != nil {
			return err
		}
		switch room.Kind {
		case EventRoom:
			r, err := q.InsertChatMessage(ctx, dbgen.InsertChatMessageParams{GroupID: st.Topic, UserID: userID, Body: body})
			if err != nil {
				return err
			}
			msg = Message{ID: r.ID, At: r.CreatedAt.Time}
		case CrewRoom:
			r, err := q.InsertCrewMessage(ctx, dbgen.InsertCrewMessageParams{CrewID: st.Topic, UserID: pgtype.UUID{Bytes: userID, Valid: true}, Body: body})
			if err != nil {
				return err
			}
			msg = Message{ID: r.ID, At: r.CreatedAt.Time}
		}
		msg.UserID, msg.Body, msg.Mine = userID, body, true
		return q.NotifyChat(ctx, st.Topic.String())
	})
	return msg, err
}

// lock takes the room's row for the rest of the transaction. Ids are
// assigned when a message is inserted but visible when its transaction
// commits, so two posters could commit out of order, and a reader that
// had already moved its cursor past the higher id would never see the lower
// one. One poster at a time per room makes ids commit in order.
func lock(ctx context.Context, q *dbgen.Queries, kind Kind, topic uuid.UUID) error {
	var err error
	switch kind {
	case EventRoom:
		_, err = q.LockEventRoom(ctx, topic)
	case CrewRoom:
		_, err = q.LockCrewRoom(ctx, topic)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRoom
	}
	return err
}

// PostPlan stores a plan suggested in a crew's chat (Issho is the sender)
// and tells every API instance. Run it in the transaction that decided to
// suggest it; it takes the same lock as Send.
func PostPlan(ctx context.Context, q *dbgen.Queries, crewID, planID uuid.UUID, at time.Time) error {
	if err := lock(ctx, q, CrewRoom, crewID); err != nil {
		return err
	}
	if err := q.InsertCrewPlanMessage(ctx, dbgen.InsertCrewPlanMessageParams{
		CrewID: crewID, PlanID: pgtype.UUID{Bytes: planID, Valid: true}, At: pgtype.Timestamptz{Time: at, Valid: true},
	}); err != nil {
		return err
	}
	return q.NotifyChat(ctx, crewID.String())
}

// Typed tells the room's other sockets that the caller is typing. It
// refuses (with the access error, or ErrClosed) unless they could write a
// message now, so a typing notice is held to the rules a message is.
func (s *Service) Typed(ctx context.Context, userID uuid.UUID, room Room, now time.Time) error {
	q := dbgen.New(s.DB)
	st, err := s.state(ctx, q, userID, room, now)
	if err != nil {
		return err
	}
	if !st.Open {
		return ErrClosed
	}
	name, err := q.GetDisplayName(ctx, userID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(Typing{Topic: st.Topic, User: userID, Name: name})
	if err != nil {
		return err
	}
	return q.NotifyTyping(ctx, string(payload))
}

// DeleteOld deletes the group chats of a city's events that ended more
// than Keep ago. Run by the worker's settle pass.
func DeleteOld(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time) (int64, error) {
	return q.DeleteOldChats(ctx, dbgen.DeleteOldChatsParams{CityID: cityID, Before: pgtype.Date{Time: now.Add(-Keep), Valid: true}})
}

// DeleteOldCrewChats deletes the chats of crews that closed or lapsed
// (closed_at) more than Keep ago, for privacy. It deletes in batches of
// DeleteBatch, so a pass never holds a long lock, and returns the total.
// An active crew's chat is kept, whoever has left it.
func DeleteOldCrewChats(ctx context.Context, q *dbgen.Queries, now time.Time) (int64, error) {
	var total int64
	for {
		n, err := q.DeleteOldCrewChats(ctx, dbgen.DeleteOldCrewChatsParams{
			Before: pgtype.Timestamptz{Time: now.Add(-Keep), Valid: true}, Max: DeleteBatch,
		})
		total += n
		if err != nil || n < DeleteBatch {
			return total, err
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
