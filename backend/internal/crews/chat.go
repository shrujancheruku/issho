package crews

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// ChatAccess lets people into a crew's chat (chat.Access): the members of an
// active crew (Membership.Chat), to read and to write, with no schedule. The
// id it is given is the crew's; the room (the Topic) is the crew's room.
type ChatAccess struct{}

func (ChatAccess) Access(ctx context.Context, q *dbgen.Queries, userID, crewID uuid.UUID, _ time.Time) (chat.State, error) {
	m, err := loadMembership(ctx, q, userID, crewID)
	switch {
	case errors.Is(err, ErrNoCrew):
		return chat.State{}, chat.ErrNoRoom
	case err != nil:
		return chat.State{}, err
	}
	if err := m.Chat(); err != nil {
		// Not a member, or the crew isn't active (an offer, or over).
		return chat.State{}, chat.ErrForbidden
	}
	roomID, err := q.GetCrewRoom(ctx, pgtype.UUID{Bytes: crewID, Valid: true})
	if err != nil {
		return chat.State{}, err
	}
	return chat.State{Topic: roomID, Open: true}, nil
}
