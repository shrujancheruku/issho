package crews

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationListCap: ListMyCrews returns at most Cap.Max crews, newest
// first, and warns when it hits the cap.
func TestIntegrationListCap(t *testing.T) {
	f := testfixture.New(t, "zz-crews-cap", "UTC", 2)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "ja")
	}
	act := f.Act
	for i := 1; i <= 3; i++ {
		id, err := f.Q.CreateCrew(ctx, dbgen.CreateCrewParams{
			CityID: f.City, ActivityID: &act,
			OfferUntil: pgtype.Timestamptz{Time: day(20), Valid: true}, At: pgtype.Timestamptz{Time: day(i), Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Q.InsertCrewMembers(ctx, dbgen.InsertCrewMembersParams{CrewID: id, UserIds: f.Users}); err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{DB: f.Pool}

	if got, err := svc.List(ctx, f.Users[0]); err != nil || len(got) != 3 {
		t.Fatalf("under the cap: %d crews, %v; want 3", len(got), err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	defer slog.SetDefault(prev)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer func(m int32) { Cap.Max = m }(Cap.Max)
	Cap.Max = 2
	got, err := svc.List(ctx, f.Users[0])
	if err != nil || len(got) != 2 {
		t.Fatalf("at the cap: %d crews, %v; want 2", len(got), err)
	}
	if !strings.Contains(buf.String(), "rpc=ListMyCrews") || !strings.Contains(buf.String(), "cap=2") {
		t.Errorf("no cap warning logged:\n%s", buf.String())
	}
}
