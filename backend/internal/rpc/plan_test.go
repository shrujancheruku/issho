package rpc_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

const (
	lanternPlan = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001"
	sunrisePlan = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0002"
)

// planClient is the public plan RPCs on the fixture city, with no
// authenticator at all: nothing here sends a token.
func planClient(t *testing.T) isshov1connect.PlanServiceClient {
	t.Helper()
	c, _ := planClientAndPool(t)
	return c
}

// planClientAndPool is planClient and the database behind it, for a test that
// changes the city's state after its plans exist.
func planClientAndPool(t *testing.T) (isshov1connect.PlanServiceClient, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{
		Version: "test",
		Cities:  cities.New(pool),
		Plans:   plans.New(pool),
	}))
	t.Cleanup(srv.Close)
	return isshov1connect.NewPlanServiceClient(srv.Client(), srv.URL), pool
}

// Anyone can list a city's plans: no token, no account.
func TestAVisitorWithNoTokenListsTheCitysPlans(t *testing.T) {
	c := planClient(t)
	res, err := c.ListPlans(context.Background(), connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil {
		t.Fatal(err)
	}
	got := res.Msg.GetPlans()
	if len(got) != 2 || got[0].GetId() != sunrisePlan || got[1].GetId() != lanternPlan {
		t.Fatalf("plans = %v", got)
	}
	// An empty language reads in English.
	if got[1].GetTitle() != "Hoi An old town at lantern time" || got[1].GetLanguage() != "en" {
		t.Fatalf("lantern plan = %v", got[1])
	}
}

// One plan carries every field a plan page shows.
func TestAVisitorWithNoTokenReadsOnePlan(t *testing.T) {
	c := planClient(t)
	res, err := c.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: sunrisePlan, DisplayLanguage: "en"}))
	if err != nil {
		t.Fatal(err)
	}
	p := res.Msg.GetPlan()
	if p.GetCity() != "da-nang-hoi-an" || p.GetPrice() != 120000 || p.GetCurrency() != "VND" || p.GetTimeZone() != "Asia/Ho_Chi_Minh" {
		t.Fatalf("plan = %v", p)
	}
	if p.GetStartsAt() != "06:30" || p.GetEndsAt() != "10:00" || len(p.GetPartsOfDay()) != 1 || p.GetPartsOfDay()[0] != "morning" {
		t.Fatalf("times = %v", p)
	}
	if p.GetMeetingAddress() == "" || p.GetMeetingPoint() == "" || len(p.GetWhatToBring()) != 3 {
		t.Fatalf("meeting point or what to bring missing: %v", p)
	}
	stops := p.GetStops()
	if len(stops) != 3 || stops[0].GetPosition() != 1 || stops[0].GetStayMinutes() != 60 {
		t.Fatalf("stops = %v", stops)
	}
	if leg := stops[0].GetLeg(); leg.GetMode() != "ride" || leg.GetMinutes() != 10 {
		t.Fatalf("first leg = %v", leg)
	}
	if stops[2].GetLeg() != nil {
		t.Fatalf("the last stop has a leg: %v", stops[2].GetLeg())
	}
}

// The text is in the caller's display language, in the list and in one plan.
func TestAPlansTextComesBackInTheDisplayLanguage(t *testing.T) {
	c := planClient(t)
	ctx := context.Background()
	list, err := c.ListPlans(ctx, connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an", DisplayLanguage: "vi"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := list.Msg.GetPlans()[0]; got.GetTitle() != "Bình minh ở Mỹ Khê, rồi bánh mì và cà phê" || got.GetLanguage() != "vi" {
		t.Fatalf("listed plan = %v", got)
	}
	one, err := c.GetPlan(ctx, connect.NewRequest(&isshov1.GetPlanRequest{Id: lanternPlan, DisplayLanguage: "vi"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := one.Msg.GetPlan().GetStops()[0].GetName(); got != "Chùa Cầu" {
		t.Fatalf("vi stop name = %q", got)
	}
}

// A plan that doesn't exist is NotFound, not an empty plan; an id that
// couldn't be a plan's is the same.
func TestAPlanThatDoesNotExistIsNotFoundOverTheWire(t *testing.T) {
	c := planClient(t)
	for _, id := range []string{"6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff", "", "not-an-id"} {
		res, err := c.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: id}))
		wantCode(t, err, connect.CodeNotFound)
		if res != nil {
			t.Errorf("GetPlan(%q) returned %v with the error", id, res.Msg)
		}
	}
}

// A closed city's plans are kept but not shown (docs/spec/cities.md): the list
// is empty, and one of its plans is NotFound, exactly as for a plan that
// doesn't exist. The city closes after its plans exist.
func TestAClosedCitysPlansAreNotShown(t *testing.T) {
	c, pool := planClientAndPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `update app.cities set state = 'closed' where slug = 'da-nang-hoi-an'`); err != nil {
		t.Fatal(err)
	}
	list, err := c.ListPlans(ctx, connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(list.Msg.GetPlans()); n != 0 {
		t.Fatalf("a closed city listed %d plans", n)
	}
	_, missing := c.GetPlan(ctx, connect.NewRequest(&isshov1.GetPlanRequest{Id: "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"}))
	_, closed := c.GetPlan(ctx, connect.NewRequest(&isshov1.GetPlanRequest{Id: lanternPlan}))
	wantCode(t, closed, connect.CodeNotFound)
	if connect.CodeOf(closed) != connect.CodeOf(missing) || closed.Error() != missing.Error() {
		t.Fatalf("a closed city's plan answers %q, a missing plan %q", closed, missing)
	}
	// The plans are kept: the city opening again shows them.
	if _, err := pool.Exec(ctx, `update app.cities set state = 'open' where slug = 'da-nang-hoi-an'`); err != nil {
		t.Fatal(err)
	}
	list, err = c.ListPlans(ctx, connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil || len(list.Msg.GetPlans()) != 2 {
		t.Fatalf("reopened city lists %v, %v", list, err)
	}
}

// A city that isn't listed is NotFound, and a language that isn't offered is
// InvalidArgument, as it is for an account.
func TestAnUnlistedCityAndAnUnofferedLanguageAreRefused(t *testing.T) {
	c := planClient(t)
	_, err := c.ListPlans(context.Background(), connect.NewRequest(&isshov1.ListPlansRequest{City: "atlantis"}))
	wantCode(t, err, connect.CodeNotFound)
	_, err = c.ListPlans(context.Background(), connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an", DisplayLanguage: "fr"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = c.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: lanternPlan, DisplayLanguage: "fr"}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

// A process with no database serves no plans: the service isn't mounted.
func TestWithoutPlansTheServiceIsNotServed(t *testing.T) {
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "test"}))
	t.Cleanup(srv.Close)
	c := isshov1connect.NewPlanServiceClient(srv.Client(), srv.URL)
	_, err := c.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: lanternPlan}))
	wantCode(t, err, connect.CodeUnimplemented)
}
