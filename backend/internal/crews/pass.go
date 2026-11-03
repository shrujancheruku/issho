package crews

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// Runner runs the worker's crews pass for a city.
type Runner struct {
	DB  *pgxpool.Pool
	Log *slog.Logger
}

// PassReport counts what a crews pass did.
type PassReport struct {
	Offered, Formed, Lapsed, Suggested int
	Failed                             int // crews that could not be decided, suggested a plan or offered
}

// Pass decides offers that everyone answered or that ran out, suggests a
// plan to crews due one, and offers new crews.
//
// Failure policy (the same as plans.Runner.Match): a crew is the item. One
// that fails is logged with its id, reported to Sentry and counted in
// Failed, and the pass carries on. Each crew commits in its own
// transaction and the counts go up only after that commit, so a rollback
// never over-counts. Pass returns an error, so River retries the job, only
// for a setup failure (the crews or the city can't be loaded, the database
// is down) or when every crew of a stage failed. Some failing is success.
func (r *Runner) Pass(ctx context.Context, cityID string, now time.Time) (PassReport, error) {
	var rep PassReport
	var errs []error
	q := dbgen.New(r.DB)
	at := pgtype.Timestamptz{Time: now, Valid: true}

	offers, err := q.ListCrewOffersToDecide(ctx, dbgen.ListCrewOffersToDecideParams{CityID: cityID, Now: at})
	if err != nil {
		return rep, err
	}
	var stageErrs []error
	for _, id := range offers {
		// Cancelled (timeout, shutdown): stop, don't count the rest as failed.
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		var status string
		err := pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			var err error
			status, err = decide(ctx, dbgen.New(tx), id, now)
			return err
		})
		if err != nil {
			rep.Failed++
			stageErrs = append(stageErrs, r.itemFailed(ctx, "decide", cityID, id, err))
			continue
		}
		switch status {
		case Active:
			rep.Formed++
		case Lapsed:
			rep.Lapsed++
		}
	}
	if len(offers) > 0 && len(stageErrs) == len(offers) {
		errs = append(errs, allFailed("decide offers", stageErrs))
	}

	due, err := q.ListCrewsToSuggest(ctx, dbgen.ListCrewsToSuggestParams{CityID: cityID, Before: pgtype.Timestamptz{Time: now.Add(-SuggestEvery), Valid: true}})
	if err != nil {
		return rep, errors.Join(append(errs, err)...)
	}
	stageErrs = nil
	tried := 0
	for _, id := range due {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		plan, err := q.GetCrewPlanSuggestion(ctx, dbgen.GetCrewPlanSuggestionParams{CrewID: id, After: pgtype.Timestamptz{Time: now.Add(suggestMinNotice), Valid: true}})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // nothing fits this week; try again next pass
		}
		tried++
		if err == nil {
			err = pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
				q := dbgen.New(tx)
				// Posting takes the chat's lock, as a member's message does, so
				// the suggestion's id commits in order with theirs.
				if err := chat.PostPlan(ctx, q, id, plan, now); err != nil {
					return err
				}
				if err := q.SetCrewSuggested(ctx, dbgen.SetCrewSuggestedParams{ID: id, At: at}); err != nil {
					return err
				}
				members, err := q.ListCrewMemberIDs(ctx, dbgen.ListCrewMemberIDsParams{CrewID: id, Response: Accepted})
				if err != nil {
					return err
				}
				return notify(ctx, q, NotifyCrewSuggestion, id, members)
			})
		}
		if err != nil {
			rep.Failed++
			stageErrs = append(stageErrs, r.itemFailed(ctx, "suggest", cityID, id, err))
			continue
		}
		rep.Suggested++
	}
	if tried > 0 && len(stageErrs) == tried {
		errs = append(errs, allFailed("suggest plans", stageErrs))
	}

	if err := r.offer(ctx, q, cityID, now, &rep); err != nil {
		errs = append(errs, err)
	}
	return rep, errors.Join(errs...)
}

// itemFailed records one crew's failure (log, Sentry) and returns the error
// to keep for the stage.
func (r *Runner) itemFailed(ctx context.Context, stage, cityID string, crew uuid.UUID, err error) error {
	err = fmt.Errorf("crew %s: %w", crew, err)
	r.Log.Error("item failed", "stage", "crews "+stage, "city", cityID, "crew", crew, "err", err)
	sentry.Report(ctx, err, map[string]string{"stage": "crews " + stage, "city": cityID})
	return err
}

// allFailed is the error for a stage where every crew failed: more likely
// a shared fault than bad crews, so the job is retried.
func allFailed(stage string, errs []error) error {
	return fmt.Errorf("%s: every crew failed: %w", stage, errors.Join(errs...))
}

// offer offers crews to people who got on well across outings
// (matching.FindCrews), unless those people were all offered one before,
// or two or more of them already share a crew on the same basis. Failing to
// load what it needs is an error; a crew that fails to be offered is counted
// in rep.Failed (and is an error only if every one failed).
func (r *Runner) offer(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time, rep *PassReport) error {
	met, err := q.ListCityMetPairs(ctx, cityID)
	if err != nil || len(met) == 0 {
		return err
	}
	g := matching.CrewGraph{Met: make(map[matching.Pair]bool, len(met)), Averse: map[matching.Pair]bool{}}
	for _, m := range met {
		g.Met[matching.MakePair(matching.UserID(m.AID.String()), matching.UserID(m.BID.String()))] = true
	}
	averse, err := q.ListCityCrewAversions(ctx, cityID)
	if err != nil {
		return err
	}
	for _, a := range averse {
		g.Averse[matching.MakePair(matching.UserID(a.AverseID.String()), matching.UserID(a.OtherID.String()))] = true
	}
	// Seasons are the city's own, not UTC's: they differ near month ends.
	row, err := q.GetCityMatching(ctx, cityID)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return fmt.Errorf("city %s: %w", cityID, err)
	}
	// Crews are their own job (queued after Match), so this is the one load
	// of the city in this pass.
	city, err := plans.LoadCity(ctx, q, cityID, now.In(loc).Month())
	if err != nil {
		return err
	}
	offered := make([]matching.ActivityID, len(city.Activities))
	for i, a := range city.Activities {
		offered[i] = a.ID
	}
	at := pgtype.Timestamptz{Time: now, Valid: true}
	var errs []error
	tried := 0
	for _, c := range matching.FindCrews(city.Users, g, city.Relations, offered) {
		if err := ctx.Err(); err != nil {
			return err
		}
		members := make([]uuid.UUID, len(c.Members))
		for i, id := range c.Members {
			members[i] = uuid.MustParse(string(id))
		}
		activity, area := orNull(string(c.Basis.Activity)), orNull(string(c.Basis.Area))
		covered, err := q.CrewCoversMembers(ctx, dbgen.CrewCoversMembersParams{CityID: cityID, UserIds: members})
		var overlaps bool
		if err == nil {
			overlaps, err = q.CrewOverlaps(ctx, dbgen.CrewOverlapsParams{CityID: cityID, ActivityID: activity, AreaID: area, UserIds: members})
		}
		if err == nil && (covered || overlaps) {
			continue
		}
		tried++
		if err == nil {
			err = pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
				q := dbgen.New(tx)
				crewID, err := q.CreateCrew(ctx, dbgen.CreateCrewParams{
					CityID: cityID, ActivityID: activity, AreaID: area,
					OfferUntil: pgtype.Timestamptz{Time: now.Add(OfferFor), Valid: true}, At: at,
				})
				if err != nil {
					return err
				}
				if err := q.InsertCrewMembers(ctx, dbgen.InsertCrewMembersParams{CrewID: crewID, UserIds: members}); err != nil {
					return err
				}
				return notify(ctx, q, NotifyCrewOffered, crewID, members)
			})
		}
		if err != nil {
			// No crew id when the insert rolled back: say who was to be in it.
			rep.Failed++
			err = fmt.Errorf("offer crew to %d people (%s %s): %w", len(members), c.Basis.Activity, c.Basis.Area, err)
			r.Log.Error("item failed", "stage", "crews offer", "city", cityID, "members", members, "err", err)
			sentry.Report(ctx, err, map[string]string{"stage": "crews offer", "city": cityID})
			errs = append(errs, err)
			continue
		}
		rep.Offered++
		r.Log.Info("crew offered", "city", cityID, "members", len(members), "activity", c.Basis.Activity, "area", c.Basis.Area)
	}
	if tried > 0 && len(errs) == tried {
		return allFailed("offer crews", errs)
	}
	return nil
}

// orNull maps matching's "" (none) to SQL null.
func orNull(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
