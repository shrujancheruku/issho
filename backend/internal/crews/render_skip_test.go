package crews

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationRenderCrewGone: a notification about a crew that no longer
// exists is skipped (the notifier then finishes it with no push) instead of
// failing until its attempts run out.
func TestIntegrationRenderCrewGone(t *testing.T) {
	f := testfixture.New(t, "zz-crews-rendergone", "UTC", 1)
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	r := &NotificationRenderer{Plans: &plans.Service{DB: f.Pool}, Catalog: catalog}
	got := r.Render(context.Background(), []notifications.Pending{
		{ID: 1, UserID: f.Users[0], Kind: NotifyCrewOffered, Language: "en", Ref: notifications.Ref{CrewID: uuid.New()}},
	})
	if len(got) != 1 || !got[0].Skip || got[0].Err != nil {
		t.Errorf("result = %+v, want a skip", got)
	}
}
