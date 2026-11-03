package crews

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// Crew notifications (app.notifications.kind).
const (
	NotifyCrewOffered    = "crew_offered"    // you got on well: keep meeting as a crew?
	NotifyCrewFormed     = "crew_formed"     // enough people accepted
	NotifyCrewSuggestion = "crew_suggestion" // a plan suggested in the crew's chat
)

// notify queues a crew notification for each person, in the caller's
// transaction.
func notify(ctx context.Context, q *dbgen.Queries, kind string, crewID uuid.UUID, users []uuid.UUID) error {
	return notifications.Enqueue(ctx, q, kind, users, notifications.Ref{CrewID: crewID})
}

// notificationText is the UI catalogue key for each kind's message. The
// crew's name is the title ("crew.name").
var notificationText = map[string]string{
	NotifyCrewOffered:    "push.crewOffered",
	NotifyCrewFormed:     "push.crewFormed",
	NotifyCrewSuggestion: "push.crewSuggestion",
}

// NotificationKinds are the kinds this package queues.
func NotificationKinds() []string {
	return []string{NotifyCrewOffered, NotifyCrewFormed, NotifyCrewSuggestion}
}

// NotificationRenderer writes crew notifications
// (notifications.Notifier.Register it for NotificationKinds).
type NotificationRenderer struct {
	Plans   *plans.Service // the database, and activity names in the person's language
	Catalog *uistrings.Catalog
}

// Render writes each item on its own: one whose crew is gone is skipped,
// and one that errors is its own failure, not the batch's.
func (r *NotificationRenderer) Render(ctx context.Context, items []notifications.Pending) []notifications.Result {
	q := dbgen.New(r.Plans.DB)
	out := make([]notifications.Result, len(items))
	// Items about the same crew, in the same language, share a title.
	type key struct {
		crew uuid.UUID
		lang string
	}
	type titleResult struct {
		title string
		gone  bool
		err   error
	}
	titles := map[key]titleResult{}
	for i, p := range items {
		k := key{p.Ref.CrewID, p.Language}
		t, ok := titles[k]
		if !ok {
			t.title, t.gone, t.err = r.title(ctx, q, p)
			titles[k] = t
		}
		switch {
		case t.err != nil:
			out[i].Err = t.err
		case t.gone:
			out[i].Skip = true
		default:
			out[i].Content = notifications.Content{
				Title: t.title,
				Body:  r.Catalog.Text(ctx, r.Plans.Translations, p.Language, notificationText[p.Kind]),
			}
		}
	}
	return out
}

// title is a crew's name in the person's language: "{name} crew" from the
// UI catalogue, with the activity's name (translated like plan titles) or
// the area's stored name (never machine-translated). gone is true if the
// crew was deleted (the notification is skipped).
func (r *NotificationRenderer) title(ctx context.Context, q *dbgen.Queries, p notifications.Pending) (title string, gone bool, err error) {
	c, err := q.GetCrewForNotification(ctx, p.Ref.CrewID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	var name string
	if c.AreaID != nil && *c.AreaID != "" {
		raw, err := q.GetAreaName(ctx, *c.AreaID)
		if err != nil {
			return "", false, err
		}
		name = i18n.Pick(raw, p.Language)
	} else {
		activity := ""
		if c.ActivityID != nil {
			activity = *c.ActivityID
		}
		if name, err = r.Plans.ActivityTitle(ctx, c.CityID, p.Language, matching.ActivityID(activity)); err != nil {
			return "", false, err
		}
	}
	return strings.ReplaceAll(r.Catalog.Text(ctx, r.Plans.Translations, p.Language, "crew.name"), "{name}", name), false, nil
}
