package adminjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type fakeJobs map[int64]*rivertype.JobRow

func (f fakeJobs) JobGet(_ context.Context, id int64) (*rivertype.JobRow, error) {
	if r, ok := f[id]; ok {
		return r, nil
	}
	return nil, river.ErrNotFound
}

func row(t *testing.T, kind string, state rivertype.JobState, out *Output) *rivertype.JobRow {
	t.Helper()
	at := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	r := &rivertype.JobRow{ID: 7, Kind: kind, State: state, CreatedAt: at}
	if state != rivertype.JobStateAvailable {
		started := at.Add(time.Second)
		r.AttemptedAt = &started
	}
	if state == rivertype.JobStateCompleted || state == rivertype.JobStateDiscarded || state == rivertype.JobStateCancelled {
		done := at.Add(time.Minute)
		r.FinalizedAt = &done
	}
	if out != nil {
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		r.Metadata = []byte(fmt.Sprintf(`{"output":%s}`, raw))
	}
	return r
}

func TestDescribe(t *testing.T) {
	apply := &Output{ApplyCityPack: &cities.Diff{
		NewAreas: []string{"x-a"}, TravelTimesRebuilt: true, TravelTimes: 12,
		ReplacedAreas: []cities.ReplacedArea{{Old: "x-old", New: "x-a", People: 4, Crews: 2, Plans: 7}},
	}}
	cases := []struct {
		name  string
		row   *rivertype.JobRow
		state isshov1.AdminJobState
		check func(t *testing.T, r *isshov1.GetAdminJobResponse)
	}{
		{"available", row(t, KindApplyCityPack, rivertype.JobStateAvailable, nil), isshov1.AdminJobState_ADMIN_JOB_STATE_QUEUED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if r.StartedAt != nil || r.FinishedAt != nil || r.QueuedAt == nil {
					t.Errorf("times = %v", r)
				}
			}},
		{"scheduled", row(t, KindApplyCityPack, rivertype.JobStateScheduled, nil), isshov1.AdminJobState_ADMIN_JOB_STATE_QUEUED, nil},
		{"running", row(t, KindApplyCityPack, rivertype.JobStateRunning, nil), isshov1.AdminJobState_ADMIN_JOB_STATE_RUNNING,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if r.StartedAt == nil || r.FinishedAt != nil {
					t.Errorf("times = %v", r)
				}
			}},
		// A failed first attempt that will be retried is still queued, and
		// its recorded failure is not the job's result.
		{"retryable", row(t, KindDiscoverAreaVenues, rivertype.JobStateRetryable, &Output{Failure: &Failure{Message: "x"}}),
			isshov1.AdminJobState_ADMIN_JOB_STATE_QUEUED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if r.Failure != nil {
					t.Errorf("failure shown while retrying: %v", r.Failure)
				}
			}},
		{"succeeded apply", row(t, KindApplyCityPack, rivertype.JobStateCompleted, apply), isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				a := r.GetApplyCityPack()
				if a == nil || len(a.NewAreas) != 1 || !a.TravelTimesRebuilt || a.TravelTimes != 12 || len(a.ReplacedAreas) != 1 ||
					a.ReplacedAreas[0].OldAreaId != "x-old" || a.ReplacedAreas[0].Plans != 7 || r.FinishedAt == nil || r.Failure != nil {
					t.Errorf("result = %v", r)
				}
			}},
		{"succeeded discover", row(t, KindDiscoverAreaVenues, rivertype.JobStateCompleted, &Output{DiscoverAreaVenues: &VenuesFound{Searches: 9, Venues: 31}}),
			isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if d := r.GetDiscoverAreaVenues(); d == nil || d.Searches != 9 || d.Venues != 31 {
					t.Errorf("result = %v", r)
				}
			}},
		{"cancelled with a reason", row(t, KindDiscoverAreaVenues, rivertype.JobStateCancelled,
			&Output{Failure: &Failure{Message: "needs a key", Reason: isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE}}),
			isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if r.Failure == nil || r.Failure.Message != "needs a key" || r.Failure.Reason != isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE || r.Result != nil {
					t.Errorf("failure = %v", r)
				}
			}},
		// River discarded it without the worker recording anything.
		{"discarded without output", row(t, KindApplyCityPack, rivertype.JobStateDiscarded, nil), isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED,
			func(t *testing.T, r *isshov1.GetAdminJobResponse) {
				if r.Failure == nil || r.Failure.Message == "" {
					t.Errorf("failure = %v", r)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Describe(c.row)
			if err != nil || got.State != c.state || got.JobId != 7 {
				t.Fatalf("Describe = %v, %v; want state %v", got, err, c.state)
			}
			if c.check != nil {
				c.check(t, got)
			}
		})
	}
}

func TestGetAdminJob(t *testing.T) {
	h := &Handler{Jobs: fakeJobs{
		7: row(t, KindApplyCityPack, rivertype.JobStateRunning, nil),
		8: row(t, "match_city", rivertype.JobStateRunning, nil),
	}}
	call := func(ctx context.Context, id int64) error {
		_, err := h.GetAdminJob(ctx, connect.NewRequest(&isshov1.GetAdminJobRequest{JobId: id}))
		return err
	}
	admin := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New(), Email: "a@example.com", Admin: true})
	if err := call(admin, 7); err != nil {
		t.Errorf("an admin job: %v", err)
	}
	if err := call(admin, 99); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown job: %v", err)
	}
	if err := call(admin, 8); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a job that isn't an admin operation: %v", err)
	}
	member := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New(), Email: "m@example.com"})
	if err := call(member, 7); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a member: %v", err)
	}
}

// classify decides what the admin is told and whether River retries.
func TestClassify(t *testing.T) {
	cancelled := func(err error) bool { var c *river.JobCancelError; return errors.As(err, &c) }
	cases := []struct {
		err       error
		message   string
		reason    isshov1.ErrorReason
		noRetry   bool
		secretOut string
	}{
		{fmt.Errorf("%w: area x-b: 1 people", cities.ErrRefused), "pack not applied: area x-b: 1 people", 0, true, ""},
		{fmt.Errorf("city: %w", geo.ErrCityNotFound), "no such city", 0, true, ""},
		{cities.ErrAreaNotInCity, "area_id: not in that city", 0, true, ""},
		{fmt.Errorf("%w: quota for key AIza-secret", cities.ErrTravelTimes), msgTravel, 0, false, "AIza"},
		{fmt.Errorf("%w: quota for key AIza-secret", cities.ErrSearch), msgSearch, 0, false, "AIza"},
		{fmt.Errorf("get: %w", context.DeadlineExceeded), msgTimedOut, isshov1.ErrorReason_ERROR_REASON_TIMED_OUT, false, ""},
		{errors.New(`relation "app.cities" does not exist (SQLSTATE 42P01)`), msgInternal, 0, false, "SQLSTATE"},
	}
	for _, c := range cases {
		f, err := classify(c.err)
		if f.Message != c.message {
			t.Errorf("%v: message %q, want %q", c.err, f.Message, c.message)
		}
		if f.Reason != c.reason || cancelled(err) != c.noRetry {
			t.Errorf("%v: reason %v, cancelled %v", c.err, f.Reason, cancelled(err))
		}
		if c.secretOut != "" && strings.Contains(f.Message, c.secretOut) {
			t.Errorf("%v: message leaks %q: %q", c.err, c.secretOut, f.Message)
		}
	}
}

// Without a Maps key on the worker, finding venues fails for good with the
// feature-unavailable reason (a retry can't conjure a key).
func TestDiscoverWithoutPlacesKey(t *testing.T) {
	out, err := DiscoverAreaVenues(context.Background(), nil, nil, DiscoverAreaVenuesArgs{City: "x", Area: "x-a"})
	var c *river.JobCancelError
	if !errors.As(err, &c) || out.Failure == nil || out.Failure.Reason != isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
}

// Both kinds go to the maintenance queue with explicit attempts and are unique
// while live (the queue itself is tested in internal/worker).
func TestInsertOpts(t *testing.T) {
	a := ApplyCityPackArgs{City: "x", Pack: cities.Pack{ID: "x"}}
	if o := a.InsertOpts(); o.Queue != Queue || o.MaxAttempts != ApplyCityPackMaxAttempts || !o.UniqueOpts.ByArgs {
		t.Errorf("apply opts = %+v", o)
	}
	if o := (DiscoverAreaVenuesArgs{}).InsertOpts(); o.Queue != Queue || o.MaxAttempts != DiscoverAreaVenuesMaxAttempts || !o.UniqueOpts.ByArgs {
		t.Errorf("discover opts = %+v", o)
	}
}
