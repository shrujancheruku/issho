package crews

import (
	"context"
	"strings"

	"github.com/google/uuid"

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

func (r *NotificationRenderer) Render(ctx context.Context, items []notifications.Pending) ([]notifications.Content, error) {
	q := dbgen.New(r.Plans.DB)
	out := make([]notifications.Content, len(items))
	type key struct {
		crew uuid.UUID
		lang string
	}
	titles := map[key]string{}
	for i, p := range items {
		k := key{p.Ref.CrewID, p.Language}
		title, ok := titles[k]
		if !ok {
			var err error
			if title, err = r.title(ctx, q, p); err != nil {
				return nil, err
			}
			titles[k] = title
		}
		out[i] = notifications.Content{
			Title: title,
			Body:  r.Catalog.Text(ctx, r.Plans.Translations, p.Language, notificationText[p.Kind]),
		}
	}
	return out, nil
}

// title is a crew's name in the person's language: "{name} crew" from the
// UI catalogue, with the activity's name (translated like plan titles) or
// the area's stored name (never machine-translated).
func (r *NotificationRenderer) title(ctx context.Context, q *dbgen.Queries, p notifications.Pending) (string, error) {
	c, err := q.GetCrewForNotification(ctx, p.Ref.CrewID)
	if err != nil {
		return "", err
	}
	var name string
	if c.AreaID != nil && *c.AreaID != "" {
		raw, err := q.GetAreaName(ctx, *c.AreaID)
		if err != nil {
			return "", err
		}
		name = i18n.Pick(raw, p.Language)
	} else {
		activity := ""
		if c.ActivityID != nil {
			activity = *c.ActivityID
		}
		if name, err = r.Plans.ActivityTitle(ctx, c.CityID, p.Language, matching.ActivityID(activity)); err != nil {
			return "", err
		}
	}
	return strings.ReplaceAll(r.Catalog.Text(ctx, r.Plans.Translations, p.Language, "crew.name"), "{name}", name), nil
}
