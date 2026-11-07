// Event group chats: who may read and write one. Storage is internal/chat.

package plans

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// ErrNotGoing means only people who confirmed can use the group chat.
var ErrNotGoing = errors.New("only people going can use the chat")

// ChatAccess lets people into an event's group chat (chat.Access): those
// going (Membership.Chat), who may write between chat.OpensBefore the start
// and chat.ClosesAfter it. The id it is given is the event's; the room (the
// Topic) is the caller's group's.
type ChatAccess struct{}

func (ChatAccess) Access(ctx context.Context, q *dbgen.Queries, userID, eventID uuid.UUID, now time.Time) (chat.State, error) {
	groupID, m, err := loadMembership(ctx, q, userID, eventID)
	switch {
	case errors.Is(err, ErrNoEvent):
		return chat.State{}, chat.ErrNoRoom
	case err != nil:
		return chat.State{}, err
	}
	roomID, err := q.GetGroupRoom(ctx, pgtype.UUID{Bytes: groupID, Valid: true})
	if err != nil {
		return chat.State{}, err
	}
	st := chat.State{Topic: roomID}
	if st.OpensAt, st.ClosesAt, st.Open, err = m.Chat(now); err != nil {
		// Not going, or cancelled: the same to the person.
		return chat.State{}, chat.ErrForbidden
	}
	return st, nil
}
