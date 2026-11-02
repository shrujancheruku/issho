package crews

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// ChatAccess lets people into a crew's chat (chat.Access): the members of an
// active crew (Membership.Chat), to read and to write, with no schedule. The
// room id and the topic are both the crew's id.
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
	return chat.State{Topic: crewID, Open: true}, nil
}
