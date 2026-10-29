// Event group chats: who may read and write one. Storage is internal/chat.

package plans

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
)

var (
	// ErrNotGoing means only people who confirmed can use the group chat.
	ErrNotGoing = errors.New("only people going can use the chat")
	// ErrChatClosed means the chat isn't open for writing now.
	ErrChatClosed = errors.New("the chat is not open")
)

// ChatState is when a group's chat is open for writing.
type ChatState struct {
	GroupID           uuid.UUID
	OpensAt, ClosesAt time.Time // zero when the event has no start time
	Open              bool      // writable now
}

// chatAccess checks the caller may read the event's group chat
// (Membership.Chat) and says when it's open for writing.
func chatAccess(ctx context.Context, q *dbgen.Queries, userID, eventID uuid.UUID, now time.Time) (ChatState, error) {
	groupID, m, err := loadMembership(ctx, q, userID, eventID)
	if err != nil {
		return ChatState{}, err
	}
	st := ChatState{GroupID: groupID}
	if st.OpensAt, st.ClosesAt, st.Open, err = m.Chat(now); err != nil {
		return ChatState{}, err
	}
	return st, nil
}

// ChatMessages returns the event's group chat after message afterID
// (0 = the latest messages), oldest first, and when it's open. Before it
// opens there's nothing to read yet.
func (s *Service) ChatMessages(ctx context.Context, userID, eventID uuid.UUID, afterID int64, now time.Time) ([]chat.Message, ChatState, error) {
	q := dbgen.New(s.DB)
	st, err := chatAccess(ctx, q, userID, eventID, now)
	if err != nil {
		return nil, st, err
	}
	msgs, err := chat.ListGroup(ctx, q, st.GroupID, userID, afterID)
	return msgs, st, err
}

// SendChat posts a message to the caller's group chat while it's open, and
// tells every API instance (NOTIFY) so open chats update straight away.
func (s *Service) SendChat(ctx context.Context, userID, eventID uuid.UUID, body string, now time.Time) (chat.Message, error) {
	body, ok := chat.Clean(body)
	if !ok {
		return chat.Message{}, invalid("message: 1 to %d characters", chat.MaxMessage)
	}
	var msg chat.Message
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		st, err := chatAccess(ctx, q, userID, eventID, now)
		if err != nil {
			return err
		}
		if !st.Open {
			return ErrChatClosed
		}
		msg, err = chat.PostGroup(ctx, q, st.GroupID, userID, body)
		return err
	})
	return msg, err
}
