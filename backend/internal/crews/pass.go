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
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// Runner runs the worker's crews pass for a city.
type Runner struct {
	DB  *pgxpool.Pool
	Log *slog.Logger
}

// PassReport counts what a crews pass did.
type PassReport struct {
	Offered, Formed, Lapsed, Suggested int
}

// Pass decides offers that everyone answered or that ran out, suggests a
// plan to crews due one, and offers new crews.
func (r *Runner) Pass(ctx context.Context, cityID string, now time.Time) (PassReport, error) {
	var rep PassReport
	q := dbgen.New(r.DB)
	at := pgtype.Timestamptz{Time: now, Valid: true}

	offers, err := q.ListCrewOffersToDecide(ctx, dbgen.ListCrewOffersToDecideParams{CityID: cityID, Now: at})
	if err != nil {
		return rep, err
	}
	for _, id := range offers {
		err := pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			status, err := decide(ctx, dbgen.New(tx), id, now)
			switch status {
			case Active:
				rep.Formed++
			case Lapsed:
				rep.Lapsed++
			}
			return err
		})
		if err != nil {
			return rep, fmt.Errorf("crew %s: %w", id, err)
		}
	}

	due, err := q.ListCrewsToSuggest(ctx, dbgen.ListCrewsToSuggestParams{CityID: cityID, Before: pgtype.Timestamptz{Time: now.Add(-SuggestEvery), Valid: true}})
	if err != nil {
		return rep, err
	}
	for _, id := range due {
		plan, err := q.GetCrewPlanSuggestion(ctx, dbgen.GetCrewPlanSuggestionParams{CrewID: id, After: pgtype.Timestamptz{Time: now.Add(suggestMinNotice), Valid: true}})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // nothing fits this week; try again next pass
		}
		if err != nil {
			return rep, err
		}
		err = pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			if err := q.InsertCrewPlanMessage(ctx, dbgen.InsertCrewPlanMessageParams{CrewID: id, PlanID: pgtype.UUID{Bytes: plan, Valid: true}, At: at}); err != nil {
				return err
			}
			if err := q.SetCrewSuggested(ctx, dbgen.SetCrewSuggestedParams{ID: id, At: at}); err != nil {
				return err
			}
			members, err := q.ListCrewMemberIDs(ctx, dbgen.ListCrewMemberIDsParams{CrewID: id, Response: Accepted})
			if err != nil {
				return err
			}
			if err := notify(ctx, q, plans.NotifyCrewSuggestion, id, members); err != nil {
				return err
			}
			return q.NotifyChat(ctx, id.String())
		})
		if err != nil {
			return rep, fmt.Errorf("crew %s: %w", id, err)
		}
		rep.Suggested++
	}

	return rep, r.offer(ctx, q, cityID, now, &rep)
}

// offer offers crews to people who got on well across outings
// (matching.FindCrews), unless those people were all offered one before,
// or two or more of them already share a crew on the same basis.
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
	city, err := plans.LoadCity(ctx, q, cityID, now.In(time.UTC).Month())
	if err != nil {
		return err
	}
	offered := make([]matching.ActivityID, len(city.Activities))
	for i, a := range city.Activities {
		offered[i] = a.ID
	}
	at := pgtype.Timestamptz{Time: now, Valid: true}
	for _, c := range matching.FindCrews(city.Users, g, city.Relations, offered) {
		members := make([]uuid.UUID, len(c.Members))
		for i, id := range c.Members {
			members[i] = uuid.MustParse(string(id))
		}
		activity, area := orNull(string(c.Basis.Activity)), orNull(string(c.Basis.Area))
		covered, err := q.CrewCoversMembers(ctx, dbgen.CrewCoversMembersParams{CityID: cityID, UserIds: members})
		if err != nil {
			return err
		}
		overlaps, err := q.CrewOverlaps(ctx, dbgen.CrewOverlapsParams{CityID: cityID, ActivityID: activity, AreaID: area, UserIds: members})
		if err != nil {
			return err
		}
		if covered || overlaps {
			continue
		}
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
			return notify(ctx, q, plans.NotifyCrewOffered, crewID, members)
		})
		if err != nil {
			return fmt.Errorf("offer crew: %w", err)
		}
		rep.Offered++
		r.Log.Info("crew offered", "city", cityID, "members", len(members), "activity", c.Basis.Activity, "area", c.Basis.Area)
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
