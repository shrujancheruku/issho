// Event group chats: who may read and write one. Storage is internal/chat.

package plans

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// ErrNotGoing means only people who confirmed can use the group chat.
var ErrNotGoing = errors.New("only people going can use the chat")

// ChatAccess lets people into an event's group chat (chat.Access): those
// going (Membership.Chat), who may write between chat.OpensBefore the start
// and chat.ClosesAfter it. The room id is the event's; its messages are the
// caller's group's.
type ChatAccess struct{}

func (ChatAccess) Access(ctx context.Context, q *dbgen.Queries, userID, eventID uuid.UUID, now time.Time) (chat.State, error) {
	groupID, m, err := loadMembership(ctx, q, userID, eventID)
	switch {
	case errors.Is(err, ErrNoEvent):
		return chat.State{}, chat.ErrNoRoom
	case err != nil:
		return chat.State{}, err
	}
	st := chat.State{Topic: groupID}
	if st.OpensAt, st.ClosesAt, st.Open, err = m.Chat(now); err != nil {
		// Not going, or cancelled: the same to the person.
		return chat.State{}, chat.ErrForbidden
	}
	return st, nil
}
