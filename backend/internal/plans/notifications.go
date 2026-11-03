package plans

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// notificationText is the UI catalogue key for each kind's message. The
// plan's title is the notification's title; dates are left to the app,
// which formats them in the person's language (Go can't).
var notificationText = map[string]string{
	NotifyInvited:   "push.invited",
	NotifyConfirmed: "push.confirmed",
	NotifyCancelled: "push.cancelled",
	NotifyNoGroup:   "push.noGroup",
	NotifyFeedback:  "push.feedback",
}

// NotificationRenderer writes the notifications about events and plans
// (notifications.Notifier.Register it for NotificationKinds).
type NotificationRenderer struct {
	Plans   *Service // the database, and titles in the person's language
	Catalog *uistrings.Catalog
}

// NotificationKinds are the kinds this package queues.
func NotificationKinds() []string {
	return []string{NotifyInvited, NotifyConfirmed, NotifyCancelled, NotifyNoGroup, NotifyFeedback}
}

// Render writes each item on its own: one whose plan is gone is skipped,
// and one that errors is its own failure, not the batch's.
func (r *NotificationRenderer) Render(ctx context.Context, items []notifications.Pending) []notifications.Result {
	q := dbgen.New(r.Plans.DB)
	out := make([]notifications.Result, len(items))
	// Items about the same plan, in the same language, share a title.
	type key struct {
		ref  notifications.Ref
		lang string
	}
	type titleResult struct {
		title string
		gone  bool
		err   error
	}
	titles := map[key]titleResult{}
	for i, p := range items {
		k := key{p.Ref, p.Language}
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

// title is the plan's title in the person's language; gone is true if the
// plan was deleted (nothing to say, so the notification is skipped).
func (r *NotificationRenderer) title(ctx context.Context, q *dbgen.Queries, p notifications.Pending) (title string, gone bool, err error) {
	pl, err := q.GetNotificationPlan(ctx, dbgen.GetNotificationPlanParams{EventID: nullable(p.Ref.EventID), PlanID: nullable(p.Ref.PlanID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	plan := titled{matching.Plan{Activity: matching.ActivityID(pl.ActivityID), FollowUp: matching.ActivityID(deref(pl.FollowUpID))}, pl.Title}
	t, _, err := r.Plans.titles(ctx, q, pl.CityID, Caller{Primary: p.Language}, []titled{plan})
	if err != nil {
		return "", false, err
	}
	return t[0], false, nil
}

func nullable(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil} }

// ActivityTitle is an activity's name in a language, translated like plan
// titles; other modules (crews) title their notifications with it.
func (s *Service) ActivityTitle(ctx context.Context, cityID, language string, activity matching.ActivityID) (string, error) {
	t, _, err := s.titles(ctx, dbgen.New(s.DB), cityID, Caller{Primary: language}, []titled{{plan: matching.Plan{Activity: activity}}})
	if err != nil {
		return "", err
	}
	return t[0], nil
}
