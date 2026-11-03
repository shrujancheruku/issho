package plans

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationRenderPlanGone: a notification about a plan or event that
// no longer exists is skipped (the notifier then finishes it with no push),
// rather than sent with an empty title.
func TestIntegrationRenderPlanGone(t *testing.T) {
	f := newFixture(t, "zz-plans-rendergone", "UTC", 1)
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	r := &NotificationRenderer{Plans: &Service{DB: f.pool}, Catalog: catalog}
	got := r.Render(context.Background(), []notifications.Pending{
		{ID: 1, UserID: f.users[0], Kind: NotifyInvited, Language: "en", Ref: notifications.Ref{EventID: uuid.New()}},
		{ID: 2, UserID: f.users[0], Kind: NotifyNoGroup, Language: "en", Ref: notifications.Ref{PlanID: uuid.New()}},
	})
	for i, res := range got {
		if !res.Skip || res.Err != nil {
			t.Errorf("item %d = %+v, want a skip", i, res)
		}
	}
}
