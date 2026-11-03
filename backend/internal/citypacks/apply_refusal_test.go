package citypacks

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationApplyRefusesInUseArea: applying a pack that drops an area
// people still use is failed_precondition naming it, and writes nothing.
func TestIntegrationApplyRefusesInUseArea(t *testing.T) {
	f := testfixture.New(t, "zz-packs-rm", "Asia/Tokyo", 1)
	ctx := context.Background()
	area := func(suffix string, lat float64) cities.Area {
		return cities.Area{ID: f.City + "-" + suffix, Name: cities.Text{"en": suffix}, Lat: lat, Lng: 139.6}
	}
	pack := cities.Pack{
		ID: f.City, Name: cities.Text{"en": "Rm"}, Timezone: "Asia/Tokyo", Currency: "JPY", Languages: []string{"en"},
		TravelModes: []string{"walk"}, Areas: []cities.Area{area("a", 35.70), area("b", 35.71)},
		ActivityOverrides: []cities.ActivityOverride{{ActivityID: f.Act, Disabled: true}},
	}
	repo := &fakeRepo{files: map[string][]byte{}}
	publish := func() {
		t.Helper()
		b, err := cities.EncodePack(pack)
		if err != nil {
			t.Fatal(err)
		}
		repo.files["data/citypacks/"+f.City+".json"] = b
	}
	h := &Handler{DB: f.Pool, GitHub: repo, Base: "main"}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Email: "admin@example.com", Admin: true})
	apply := func() error {
		_, err := h.ApplyCityPack(adminCtx, connect.NewRequest(&isshov1.ApplyCityPackRequest{CityId: f.City}))
		return err
	}
	publish()
	if err := apply(); err != nil {
		t.Fatal(err)
	}
	f.AddProfile(t, f.Users[0], "en")
	if _, err := f.Pool.Exec(ctx, "update app.profiles set home_area_id = $1 where user_id = $2", f.City+"-b", f.Users[0]); err != nil {
		t.Fatal(err)
	}

	// Drops the in-use area b and the override.
	pack.Areas, pack.ActivityOverrides = pack.Areas[:1], nil
	publish()
	err := apply()
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), f.City+"-b: 1 people") {
		t.Fatalf("apply = %v", err)
	}
	var n int
	if err := f.Pool.QueryRow(ctx, `select (select count(*) from app.areas where id = $1)
	  + (select count(*) from app.city_activity_overrides where city_id = $2)`, f.City+"-b", f.City).Scan(&n); err != nil || n != 2 {
		t.Errorf("rows after a refused apply = %d, %v; want the area and the override kept", n, err)
	}
}
