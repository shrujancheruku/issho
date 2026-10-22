package plans

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

const (
	// ChatOpensBefore and ChatClosesAfter set when a group can write in its
	// chat, around the event's start: time to plan the meeting, and for
	// "running late" and "thanks" after. It stays readable afterwards until
	// the messages are deleted (ChatKeep).
	ChatOpensBefore = 48 * time.Hour
	ChatClosesAfter = 24 * time.Hour
	// ChatKeep is how long after the event its messages are kept.
	ChatKeep = 30 * 24 * time.Hour
	// MaxChatMessage is the longest message, in characters.
	MaxChatMessage = 1000
	// chatPage is how many messages one read returns at most.
	chatPage = 100
)

var (
	// ErrNotGoing means only people who confirmed can use the group chat.
	ErrNotGoing = errors.New("only people going can use the chat")
	// ErrChatClosed means the chat isn't open for writing now.
	ErrChatClosed = errors.New("the chat is not open")
)

// ChatMessage is one message as the caller sees it.
type ChatMessage struct {
	ID   int64
	Name string // the sender's first name
	Body string
	At   time.Time
	Mine bool
}

// ChatState is when a group's chat is open for writing.
type ChatState struct {
	GroupID           uuid.UUID
	OpensAt, ClosesAt time.Time // zero when the event has no start time
	Open              bool      // writable now
}

// chatAccess checks the caller may read the event's group chat: they
// confirmed, and the group and event aren't cancelled.
func chatAccess(ctx context.Context, q *dbgen.Queries, userID, eventID uuid.UUID, now time.Time) (ChatState, error) {
	r, err := q.GetChatAccess(ctx, dbgen.GetChatAccessParams{EventID: eventID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatState{}, ErrNoEvent
	}
	if err != nil {
		return ChatState{}, err
	}
	if r.Response != Confirmed {
		return ChatState{}, ErrNotGoing
	}
	if r.GroupStatus == GroupCancelled || r.EventStatus == "cancelled" {
		return ChatState{}, ErrEventOver
	}
	st := ChatState{GroupID: r.GroupID}
	if r.StartsAt.Valid {
		st.OpensAt, st.ClosesAt = r.StartsAt.Time.Add(-ChatOpensBefore), r.StartsAt.Time.Add(ChatClosesAfter)
		st.Open = !now.Before(st.OpensAt) && now.Before(st.ClosesAt)
	}
	return st, nil
}

// ChatMessages returns the event's group chat after message afterID
// (0 = the latest messages), oldest first, and when it's open. Before it
// opens there's nothing to read yet.
func (s *Service) ChatMessages(ctx context.Context, userID, eventID uuid.UUID, afterID int64, now time.Time) ([]ChatMessage, ChatState, error) {
	q := dbgen.New(s.DB)
	st, err := chatAccess(ctx, q, userID, eventID, now)
	if err != nil {
		return nil, st, err
	}
	msgs, err := listChat(ctx, q, st.GroupID, userID, afterID)
	return msgs, st, err
}

func listChat(ctx context.Context, q *dbgen.Queries, groupID, userID uuid.UUID, afterID int64) ([]ChatMessage, error) {
	type row struct {
		id   int64
		user uuid.UUID
		name *string
		body string
		at   time.Time
	}
	var rows []row
	if afterID == 0 {
		rs, err := q.ListLatestChatMessages(ctx, dbgen.ListLatestChatMessagesParams{GroupID: groupID, Max: chatPage})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, row{r.ID, r.UserID, r.DisplayName, r.Body, r.CreatedAt.Time})
		}
	} else {
		rs, err := q.ListChatMessages(ctx, dbgen.ListChatMessagesParams{GroupID: groupID, AfterID: afterID, Max: chatPage})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, row{r.ID, r.UserID, r.DisplayName, r.Body, r.CreatedAt.Time})
		}
	}
	out := make([]ChatMessage, len(rows))
	for i, r := range rows {
		out[i] = ChatMessage{ID: r.id, Name: deref(r.name), Body: r.body, At: r.at, Mine: r.user == userID}
	}
	return out, nil
}

// SendChat posts a message to the caller's group chat while it's open, and
// tells every API instance (NOTIFY) so open chats update straight away.
func (s *Service) SendChat(ctx context.Context, userID, eventID uuid.UUID, body string, now time.Time) (ChatMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > MaxChatMessage || !utf8.ValidString(body) {
		return ChatMessage{}, invalid("message: 1 to %d characters", MaxChatMessage)
	}
	var msg ChatMessage
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		st, err := chatAccess(ctx, q, userID, eventID, now)
		if err != nil {
			return err
		}
		if !st.Open {
			return ErrChatClosed
		}
		r, err := q.InsertChatMessage(ctx, dbgen.InsertChatMessageParams{GroupID: st.GroupID, UserID: userID, Body: body})
		if err != nil {
			return err
		}
		msg = ChatMessage{ID: r.ID, Body: body, At: r.CreatedAt.Time, Mine: true}
		return q.NotifyChat(ctx, st.GroupID.String())
	})
	return msg, err
}

// DeleteOldChats deletes the chats of a city's events that ended more than
// ChatKeep ago. Run by the worker's settle pass.
func DeleteOldChats(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time) (int64, error) {
	return q.DeleteOldChats(ctx, dbgen.DeleteOldChatsParams{CityID: cityID, Before: date(now.Add(-ChatKeep))})
}
