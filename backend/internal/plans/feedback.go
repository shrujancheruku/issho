package plans

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Feedback opens when check-in closes (the outing is over, or nearly) and
// stays open for two weeks; reports are taken for 30 days after the start.
const (
	FeedbackAfter = CheckInAfter
	FeedbackFor   = 14 * 24 * time.Hour
	ReportFor     = 30 * 24 * time.Hour
	// MaxReportDetails is the longest report text, in characters.
	MaxReportDetails = 1000
)

// ErrFeedbackClosed means it's not the time to give feedback.
var ErrFeedbackClosed = errors.New("feedback is open from 3 hours after the start for 14 days")

// Report reasons (app.reports.reason).
const (
	ReportDating     = "dating"
	ReportHarassment = "harassment"
	ReportUnsafe     = "unsafe"
	ReportNoShow     = "no_show"
	ReportOther      = "other"
)

// FlagPickLean is the moderation flag for picks that lean towards one
// gender (matching.PickLean).
const FlagPickLean = "pick_gender_lean"

// Mate is someone else in the caller's group.
type Mate struct {
	ID        uuid.UUID
	Name      string // first name
	Cancelled bool   // dropped their place: can be reported, not picked
}

// Feedback is one person's answer about an outing.
type Feedback struct {
	Attended                bool
	PlanRating, GroupRating int // 1-5; 0 when they didn't go
	// Picks: people they'd meet again. Avoid: people they'd rather not be
	// in a group with again. Both private; nobody in neither.
	Picks, Avoid []uuid.UUID
}

// Pick kinds (app.mate_picks.pick).
const (
	PickAgain = "again"
	PickAvoid = "avoid"
)

// FeedbackForm is what the caller sees when giving feedback.
type FeedbackForm struct {
	Mates []Mate
	Given bool // Feedback holds their earlier answer
	Feedback
	OpensAt, ClosesAt time.Time
	Open              bool
}

// feedbackWindow says when the caller can give feedback: only someone who
// went ahead with their group (confirmed, in a group that wasn't
// cancelled), from FeedbackAfter to FeedbackFor after the start. err is
// why they can't at all.
func feedbackWindow(r dbgen.GetChatAccessRow, now time.Time) (FeedbackForm, error) {
	var f FeedbackForm
	switch {
	case r.Response != Confirmed:
		return f, ErrNotGoing
	case r.GroupStatus != GroupConfirmed || r.EventStatus == "cancelled":
		return f, ErrEventOver
	case !r.StartsAt.Valid:
		return f, ErrFeedbackClosed
	}
	f.OpensAt, f.ClosesAt = r.StartsAt.Time.Add(FeedbackAfter), r.StartsAt.Time.Add(FeedbackFor)
	f.Open = !now.Before(f.OpensAt) && now.Before(f.ClosesAt)
	return f, nil
}

// membership reads the caller's place in an event.
func membership(ctx context.Context, q *dbgen.Queries, userID, eventID uuid.UUID) (dbgen.GetChatAccessRow, error) {
	r, err := q.GetChatAccess(ctx, dbgen.GetChatAccessParams{EventID: eventID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNoEvent
	}
	return r, err
}

func listMates(ctx context.Context, q *dbgen.Queries, groupID, userID uuid.UUID) ([]Mate, error) {
	rows, err := q.ListGroupMates(ctx, dbgen.ListGroupMatesParams{GroupID: groupID, UserID: userID})
	if err != nil {
		return nil, err
	}
	mates := make([]Mate, len(rows))
	for i, r := range rows {
		mates[i] = Mate{ID: r.UserID, Name: r.DisplayName, Cancelled: r.Response == Dropped}
	}
	return mates, nil
}

// FeedbackForm returns the caller's feedback form for an event: the others
// in their group, what they said before, and when it's open. Anyone who
// confirmed, even if they cancelled since, gets the form (to report
// someone); it's only open to those who went ahead with the group.
func (s *Service) FeedbackForm(ctx context.Context, userID, eventID uuid.UUID, now time.Time) (FeedbackForm, error) {
	q := dbgen.New(s.DB)
	r, err := membership(ctx, q, userID, eventID)
	if err != nil {
		return FeedbackForm{}, err
	}
	if r.Response != Confirmed && r.Response != Dropped {
		return FeedbackForm{}, ErrNotGoing
	}
	f, err := feedbackWindow(r, now)
	if err != nil {
		f = FeedbackForm{}
	}
	if f.Mates, err = listMates(ctx, q, r.GroupID, userID); err != nil {
		return f, err
	}
	fb, err := q.GetFeedback(ctx, dbgen.GetFeedbackParams{GroupID: r.GroupID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Given, f.Attended = true, fb.Attended
	if fb.PlanRating != nil && fb.GroupRating != nil {
		f.PlanRating, f.GroupRating = int(*fb.PlanRating), int(*fb.GroupRating)
	}
	picks, err := q.ListPicks(ctx, dbgen.ListPicksParams{GroupID: r.GroupID, PickerID: userID})
	for _, p := range picks {
		if p.Pick == PickAvoid {
			f.Avoid = append(f.Avoid, p.PickedID)
		} else {
			f.Picks = append(f.Picks, p.PickedID)
		}
	}
	return f, err
}

// SubmitFeedback records the caller's feedback, replacing any earlier
// answer. Someone who didn't go gives no ratings or picks (any sent are
// ignored). "Rather not" keeps the two out of each other's groups and
// crews for good, like a report, but without involving a moderator. Afterwards their picks so far are checked for a lean towards
// one gender, which flags them for a moderator.
func (s *Service) SubmitFeedback(ctx context.Context, userID, eventID uuid.UUID, fb Feedback, now time.Time) error {
	if fb.Attended {
		if fb.PlanRating < 1 || fb.PlanRating > 5 || fb.GroupRating < 1 || fb.GroupRating > 5 {
			return invalid("ratings: 1 to 5 each")
		}
	} else {
		fb = Feedback{}
	}
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		r, err := membership(ctx, q, userID, eventID)
		if err != nil {
			return err
		}
		f, err := feedbackWindow(r, now)
		if err != nil {
			return err
		}
		if !f.Open {
			return ErrFeedbackClosed
		}
		groupID := r.GroupID
		mates, err := listMates(ctx, q, groupID, userID)
		if err != nil {
			return err
		}
		// Each person at most once, either way.
		seen := map[uuid.UUID]bool{}
		check := func(field string, ids []uuid.UUID) ([]uuid.UUID, error) {
			out := []uuid.UUID{}
			for _, id := range ids {
				i := slices.IndexFunc(mates, func(m Mate) bool { return m.ID == id })
				if i < 0 || mates[i].Cancelled {
					return nil, invalid("%s: only people who went with your group", field)
				}
				if seen[id] && !slices.Contains(out, id) {
					return nil, invalid("%s: someone is in both picks and avoid", field)
				}
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
			}
			return out, nil
		}
		picks, err := check("picks", fb.Picks)
		if err != nil {
			return err
		}
		avoid, err := check("avoid", fb.Avoid)
		if err != nil {
			return err
		}
		at := pgtype.Timestamptz{Time: now, Valid: true}
		var plan, group *int16
		if fb.Attended {
			p, g := int16(fb.PlanRating), int16(fb.GroupRating)
			plan, group = &p, &g
		}
		if err := q.UpsertFeedback(ctx, dbgen.UpsertFeedbackParams{
			GroupID: groupID, UserID: userID, Attended: fb.Attended, PlanRating: plan, GroupRating: group, At: at,
		}); err != nil {
			return err
		}
		if err := q.DeletePicks(ctx, dbgen.DeletePicksParams{GroupID: groupID, PickerID: userID}); err != nil {
			return err
		}
		for kind, ids := range map[string][]uuid.UUID{PickAgain: picks, PickAvoid: avoid} {
			if len(ids) == 0 {
				continue
			}
			if err := q.InsertPicks(ctx, dbgen.InsertPicksParams{GroupID: groupID, PickerID: userID, PickedIds: ids, Pick: kind, At: at}); err != nil {
				return err
			}
		}
		return checkPickLean(ctx, q, userID, now)
	})
}

// checkPickLean flags the person for a moderator if their picks lean
// heavily towards one gender. An open flag is updated, and stays until a
// moderator resolves it even if later picks even out.
func checkPickLean(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, now time.Time) error {
	rows, err := q.ListMetForPicks(ctx, userID)
	if err != nil {
		return err
	}
	met := make([]matching.Met, len(rows))
	for i, r := range rows {
		met[i] = matching.Met{Gender: matching.Gender(r.Gender), Picked: r.Picked}
	}
	lean, ok := matching.PickLean(met)
	if !ok {
		return nil
	}
	details, err := json.Marshal(lean)
	if err != nil {
		return err
	}
	return q.UpsertModerationFlag(ctx, dbgen.UpsertModerationFlagParams{
		UserID: userID, Kind: FlagPickLean, Details: details, At: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

// CheckReport checks a report's reason and returns its details trimmed,
// for reports about an outing (Report) and about a crew (crews).
func CheckReport(reason, details string) (string, error) {
	switch reason {
	case ReportDating, ReportHarassment, ReportUnsafe, ReportNoShow, ReportOther:
	default:
		return "", invalid("reason: dating, harassment, unsafe, no_show or other")
	}
	details = strings.TrimSpace(details)
	if utf8.RuneCountInString(details) > MaxReportDetails {
		return "", invalid("details: at most %d characters", MaxReportDetails)
	}
	return details, nil
}

// Report records the caller's report about someone in their group, for the
// moderators. Anyone who confirmed (even if they later cancelled) can
// report someone else who did, until ReportFor after the start; reporting
// the same person about the same outing again replaces the report. The two
// are never grouped together again (matching's Blocked).
func (s *Service) Report(ctx context.Context, userID, eventID, mateID uuid.UUID, reason, details string, now time.Time) error {
	details, err := CheckReport(reason, details)
	if err != nil {
		return err
	}
	q := dbgen.New(s.DB)
	r, err := membership(ctx, q, userID, eventID)
	if err != nil {
		return err
	}
	if r.Response != Confirmed && r.Response != Dropped {
		return ErrNotGoing
	}
	if r.StartsAt.Valid && now.After(r.StartsAt.Time.Add(ReportFor)) {
		return ErrTooLate
	}
	mates, err := listMates(ctx, q, r.GroupID, userID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(mates, func(m Mate) bool { return m.ID == mateID }) {
		return invalid("mate_id: not someone in your group")
	}
	return q.UpsertReport(ctx, dbgen.UpsertReportParams{
		ReporterID: pgtype.UUID{Bytes: userID, Valid: true}, ReportedID: mateID,
		EventID: pgtype.UUID{Bytes: eventID, Valid: true}, Reason: reason, Details: details,
		At: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

// askFeedback queues "How was it?" for the people who went ahead with a
// group, once per event, as soon as feedback opens.
func askFeedback(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time) (int64, error) {
	return q.QueueFeedbackRequests(ctx, dbgen.QueueFeedbackRequestsParams{
		CityID: cityID,
		Opened: pgtype.Timestamptz{Time: now.Add(-FeedbackAfter), Valid: true},
		Since:  pgtype.Timestamptz{Time: now.Add(-FeedbackFor), Valid: true},
	})
}

// loadRelations reads a city's mutual picks and blocks for matching.
func loadRelations(ctx context.Context, q *dbgen.Queries, c *City) error {
	picks, err := q.ListCityPicks(ctx, c.ID)
	if err != nil {
		return err
	}
	ps := make([]matching.Pick, len(picks))
	for i, p := range picks {
		ps[i] = matching.Pick{From: matching.UserID(p.PickerID.String()), To: matching.UserID(p.PickedID.String())}
	}
	c.Relations.Mutual = matching.MutualLinks(ps)
	blocks, err := q.ListCityBlocks(ctx, c.ID)
	if err != nil {
		return err
	}
	c.Relations.Blocked = make(map[matching.Pair]bool, len(blocks))
	for _, b := range blocks {
		c.Relations.Blocked[matching.MakePair(matching.UserID(b.ReporterID.String()), matching.UserID(b.OtherID.String()))] = true
	}
	return nil
}
