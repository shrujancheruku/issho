package plans

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// A crew's state (app.crews.status).
const (
	CrewOffered = "offered" // waiting for answers until offer_until
	CrewActive  = "active"
	CrewLapsed  = "lapsed" // too few accepted in time
	CrewClosed  = "closed" // fewer than 2 members left
)

// A member's answer (app.crew_members.response); "offered" until they answer.
const (
	CrewAccepted = "accepted"
	CrewDeclined = "declined"
	CrewLeft     = "left"
)

// Why someone left a crew (app.crew_leaves.reason).
const (
	LeaveUncomfortable   = "uncomfortable"
	LeaveNotInterested   = "not_interested"
	LeaveInactive        = "inactive"
	LeaveTooManyMessages = "too_many_messages"
	LeaveOther           = "other"
)

// Crew notifications (app.notifications.kind).
const (
	NotifyCrewOffered    = "crew_offered"    // you got on well: keep meeting as a crew?
	NotifyCrewFormed     = "crew_formed"     // enough people accepted
	NotifyCrewSuggestion = "crew_suggestion" // a plan suggested in the crew's chat
)

const (
	// CrewOfferFor is how long people have to accept a crew.
	CrewOfferFor = 7 * 24 * time.Hour
	// CrewSuggestEvery is how often a plan is suggested in a crew's chat at
	// most; the first comes a day after it forms.
	CrewSuggestEvery = 7 * 24 * time.Hour
	// crewSuggestMinNotice: a suggested plan's signups stay open at least
	// this long, so the crew has time to sign up together.
	crewSuggestMinNotice = 24 * time.Hour
)

var (
	// ErrNoCrew means no crew with that id that the caller was offered or is in.
	ErrNoCrew = errors.New("crew not found")
	// ErrNotInCrew means the caller isn't (or is no longer) a member.
	ErrNotInCrew = errors.New("only crew members can do that")
	// ErrCrewOver means the offer has closed, or the crew has.
	ErrCrewOver = errors.New("this crew offer or crew is over")
)

// CrewItem is a crew as one of its members (or someone offered it) sees it.
type CrewItem struct {
	ID       uuid.UUID
	Status   string
	Activity string // what it's named after: an activity ...
	Area     string // ... or an area (exactly one is set)
	// OfferUntil is when the offer closes (for an offer).
	OfferUntil time.Time
	Response   string // the caller's
	// Members: everyone who accepted, and while it's an offer, everyone
	// still deciding (first names). Accepted is set for those who accepted.
	Members []CrewMate
}

// CrewMate is someone in a crew.
type CrewMate struct {
	ID       uuid.UUID
	Name     string
	Accepted bool
	Me       bool
}

// CrewMessage is a message in a crew's chat: someone's text, or a plan
// suggested by Issho (PlanID set, Name and Body empty).
type CrewMessage struct {
	ChatMessage
	PlanID uuid.UUID
}

// Crews returns the caller's crews and the crew offers still open to them,
// newest first.
func (s *Service) Crews(ctx context.Context, userID uuid.UUID) ([]CrewItem, error) {
	q := dbgen.New(s.DB)
	rows, err := q.ListUserCrews(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]CrewItem, len(rows))
	for i, r := range rows {
		items[i] = CrewItem{ID: r.ID, Status: r.Status, Activity: deref(r.ActivityID), Area: deref(r.AreaID), OfferUntil: r.OfferUntil.Time, Response: r.Response}
		if items[i].Members, err = crewMates(ctx, q, r.ID, userID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// Crew returns one of the caller's crews or crew offers.
func (s *Service) Crew(ctx context.Context, userID, crewID uuid.UUID) (CrewItem, error) {
	q := dbgen.New(s.DB)
	r, err := q.GetCrewMembership(ctx, dbgen.GetCrewMembershipParams{CrewID: crewID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CrewItem{}, ErrNoCrew
	}
	if err != nil {
		return CrewItem{}, err
	}
	visible := (r.Status == CrewOffered && (r.Response == "offered" || r.Response == CrewAccepted)) ||
		(r.Status == CrewActive && r.Response == CrewAccepted)
	if !visible {
		return CrewItem{}, ErrNoCrew
	}
	it := CrewItem{ID: crewID, Status: r.Status, Activity: deref(r.ActivityID), Area: deref(r.AreaID), OfferUntil: r.OfferUntil.Time, Response: r.Response}
	it.Members, err = crewMates(ctx, q, crewID, userID)
	return it, err
}

func crewMates(ctx context.Context, q *dbgen.Queries, crewID, userID uuid.UUID) ([]CrewMate, error) {
	rows, err := q.ListCrewMembers(ctx, crewID)
	if err != nil {
		return nil, err
	}
	out := make([]CrewMate, len(rows))
	for i, r := range rows {
		out[i] = CrewMate{ID: r.UserID, Name: r.DisplayName, Accepted: r.Response == CrewAccepted, Me: r.UserID == userID}
	}
	return out, nil
}

// RespondToCrew accepts or declines a crew offer while it's open; the
// answer can be changed until then. Once everyone has answered, the crew
// is decided straight away rather than at the deadline.
func (s *Service) RespondToCrew(ctx context.Context, userID, crewID uuid.UUID, accept bool, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		m, err := q.LockCrewMembership(ctx, dbgen.LockCrewMembershipParams{CrewID: crewID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCrew
		}
		if err != nil {
			return err
		}
		if m.Status != CrewOffered || !now.Before(m.OfferUntil.Time) {
			return ErrCrewOver
		}
		response := CrewDeclined
		if accept {
			response = CrewAccepted
		}
		if err := q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{
			CrewID: crewID, UserID: userID, Response: response, At: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return err
		}
		waiting, err := q.CountCrewMembers(ctx, dbgen.CountCrewMembersParams{CrewID: crewID, Response: "offered"})
		if err != nil || waiting > 0 {
			return err
		}
		_, err = decideCrew(ctx, q, crewID, now)
		return err
	})
}

// decideCrew forms an offered crew with the people who accepted, if there
// are at least matching.CrewMinSize of them, and tells them; otherwise the
// offer lapses quietly. It reports the new status ("" if not an offer).
func decideCrew(ctx context.Context, q *dbgen.Queries, crewID uuid.UUID, now time.Time) (string, error) {
	c, err := q.LockCrew(ctx, crewID)
	if err != nil {
		return "", err
	}
	if c.Status != CrewOffered {
		return "", nil
	}
	accepted, err := q.ListCrewMemberIDs(ctx, dbgen.ListCrewMemberIDsParams{CrewID: crewID, Response: CrewAccepted})
	if err != nil {
		return "", err
	}
	status := CrewLapsed
	if len(accepted) >= matching.CrewMinSize {
		status = CrewActive
	}
	if err := q.SetCrewStatus(ctx, dbgen.SetCrewStatusParams{ID: crewID, Status: status, At: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
		return "", err
	}
	if status == CrewActive {
		err = notifyCrew(ctx, q, NotifyCrewFormed, crewID, accepted)
	}
	return status, err
}

func notifyCrew(ctx context.Context, q *dbgen.Queries, kind string, crewID uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	return q.InsertCrewNotifications(ctx, dbgen.InsertCrewNotificationsParams{UserIds: users, Kind: kind, CrewID: pgtype.UUID{Bytes: crewID, Valid: true}})
}

// LeaveCrew takes the caller out of a crew, with why (private: the crew
// isn't told). A crew left with fewer than 2 members closes.
func (s *Service) LeaveCrew(ctx context.Context, userID, crewID uuid.UUID, reason, details string, now time.Time) error {
	switch reason {
	case LeaveUncomfortable, LeaveNotInterested, LeaveInactive, LeaveTooManyMessages, LeaveOther:
	default:
		return invalid("reason: uncomfortable, not_interested, inactive, too_many_messages or other")
	}
	details = strings.TrimSpace(details)
	if utf8.RuneCountInString(details) > MaxReportDetails {
		return invalid("details: at most %d characters", MaxReportDetails)
	}
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		m, err := q.LockCrewMembership(ctx, dbgen.LockCrewMembershipParams{CrewID: crewID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCrew
		}
		if err != nil {
			return err
		}
		if m.Response != CrewAccepted || (m.Status != CrewActive && m.Status != CrewOffered) {
			return ErrNotInCrew
		}
		at := pgtype.Timestamptz{Time: now, Valid: true}
		if err := q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{CrewID: crewID, UserID: userID, Response: CrewLeft, At: at}); err != nil {
			return err
		}
		if err := q.InsertCrewLeave(ctx, dbgen.InsertCrewLeaveParams{
			CrewID: crewID, UserID: pgtype.UUID{Bytes: userID, Valid: true}, Reason: reason, Details: details, At: at,
		}); err != nil {
			return err
		}
		if m.Status != CrewActive {
			return nil // an offer is decided when it closes
		}
		left, err := q.CountCrewMembers(ctx, dbgen.CountCrewMembersParams{CrewID: crewID, Response: CrewAccepted})
		if err != nil || left >= 2 {
			return err
		}
		return q.SetCrewStatus(ctx, dbgen.SetCrewStatusParams{ID: crewID, Status: CrewClosed, At: at})
	})
}

// crewChatAccess checks the caller is in an active crew.
func crewChatAccess(ctx context.Context, q *dbgen.Queries, userID, crewID uuid.UUID) error {
	r, err := q.GetCrewMembership(ctx, dbgen.GetCrewMembershipParams{CrewID: crewID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoCrew
	}
	if err != nil {
		return err
	}
	if r.Response != CrewAccepted {
		return ErrNotInCrew
	}
	if r.Status != CrewActive {
		return ErrCrewOver
	}
	return nil
}

// CrewMessages returns a crew's chat after message afterID (0 = the latest
// messages), oldest first.
func (s *Service) CrewMessages(ctx context.Context, userID, crewID uuid.UUID, afterID int64) ([]CrewMessage, error) {
	q := dbgen.New(s.DB)
	if err := crewChatAccess(ctx, q, userID, crewID); err != nil {
		return nil, err
	}
	var rows []dbgen.ListCrewMessagesRow
	if afterID == 0 {
		rs, err := q.ListLatestCrewMessages(ctx, dbgen.ListLatestCrewMessagesParams{CrewID: crewID, Max: chatPage})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, dbgen.ListCrewMessagesRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListCrewMessages(ctx, dbgen.ListCrewMessagesParams{CrewID: crewID, AfterID: afterID, Max: chatPage}); err != nil {
			return nil, err
		}
	}
	out := make([]CrewMessage, len(rows))
	for i, r := range rows {
		out[i] = CrewMessage{
			ChatMessage: ChatMessage{ID: r.ID, Name: deref(r.DisplayName), Body: r.Body, At: r.CreatedAt.Time, Mine: r.UserID.Valid && r.UserID.Bytes == userID},
			PlanID:      r.PlanID.Bytes,
		}
	}
	return out, nil
}

// SendCrewMessage posts to the caller's crew chat and tells every API
// instance (NOTIFY chat, as for event chats), so open chats update.
func (s *Service) SendCrewMessage(ctx context.Context, userID, crewID uuid.UUID, body string) (CrewMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > MaxChatMessage || !utf8.ValidString(body) {
		return CrewMessage{}, invalid("message: 1 to %d characters", MaxChatMessage)
	}
	var msg CrewMessage
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := crewChatAccess(ctx, q, userID, crewID); err != nil {
			return err
		}
		r, err := q.InsertCrewMessage(ctx, dbgen.InsertCrewMessageParams{CrewID: crewID, UserID: pgtype.UUID{Bytes: userID, Valid: true}, Body: body})
		if err != nil {
			return err
		}
		msg = CrewMessage{ChatMessage: ChatMessage{ID: r.ID, Body: body, At: r.CreatedAt.Time, Mine: true}}
		return q.NotifyChat(ctx, crewID.String())
	})
	return msg, err
}

// ReportCrewMember reports someone met in a crew, as ReportPerson does for
// an outing: both must be or have been members (accepted, or left since).
func (s *Service) ReportCrewMember(ctx context.Context, userID, crewID, mateID uuid.UUID, reason, details string, now time.Time) error {
	switch reason {
	case ReportDating, ReportHarassment, ReportUnsafe, ReportNoShow, ReportOther:
	default:
		return invalid("reason: dating, harassment, unsafe, no_show or other")
	}
	details = strings.TrimSpace(details)
	if utf8.RuneCountInString(details) > MaxReportDetails {
		return invalid("details: at most %d characters", MaxReportDetails)
	}
	q := dbgen.New(s.DB)
	in, err := q.IsCrewMember(ctx, dbgen.IsCrewMemberParams{CrewID: crewID, UserID: userID})
	if err != nil {
		return err
	}
	if !in {
		return ErrNotInCrew
	}
	if mateID == userID {
		return invalid("mate_id: not someone else in your crew")
	}
	if in, err = q.IsCrewMember(ctx, dbgen.IsCrewMemberParams{CrewID: crewID, UserID: mateID}); err != nil {
		return err
	}
	if !in {
		return invalid("mate_id: not someone else in your crew")
	}
	return q.UpsertCrewReport(ctx, dbgen.UpsertCrewReportParams{
		ReporterID: pgtype.UUID{Bytes: userID, Valid: true}, ReportedID: mateID,
		CrewID: pgtype.UUID{Bytes: crewID, Valid: true}, Reason: reason, Details: details,
		At: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

// CrewReport counts what a crews pass did.
type CrewReport struct {
	Offered, Formed, Lapsed, Suggested int
}

// Crews runs a city's crew pass: decides offers that everyone answered or
// that ran out, suggests a plan to crews due one, and offers new crews.
func (r *Runner) Crews(ctx context.Context, cityID string, now time.Time) (CrewReport, error) {
	var rep CrewReport
	q := dbgen.New(r.DB)
	at := pgtype.Timestamptz{Time: now, Valid: true}

	offers, err := q.ListCrewOffersToDecide(ctx, dbgen.ListCrewOffersToDecideParams{CityID: cityID, Now: at})
	if err != nil {
		return rep, err
	}
	for _, id := range offers {
		err := pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			status, err := decideCrew(ctx, dbgen.New(tx), id, now)
			switch status {
			case CrewActive:
				rep.Formed++
			case CrewLapsed:
				rep.Lapsed++
			}
			return err
		})
		if err != nil {
			return rep, fmt.Errorf("crew %s: %w", id, err)
		}
	}

	due, err := q.ListCrewsToSuggest(ctx, dbgen.ListCrewsToSuggestParams{CityID: cityID, Before: pgtype.Timestamptz{Time: now.Add(-CrewSuggestEvery), Valid: true}})
	if err != nil {
		return rep, err
	}
	for _, id := range due {
		plan, err := q.GetCrewPlanSuggestion(ctx, dbgen.GetCrewPlanSuggestionParams{CrewID: id, After: pgtype.Timestamptz{Time: now.Add(crewSuggestMinNotice), Valid: true}})
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
			members, err := q.ListCrewMemberIDs(ctx, dbgen.ListCrewMemberIDsParams{CrewID: id, Response: CrewAccepted})
			if err != nil {
				return err
			}
			if err := notifyCrew(ctx, q, NotifyCrewSuggestion, id, members); err != nil {
				return err
			}
			return q.NotifyChat(ctx, id.String())
		})
		if err != nil {
			return rep, fmt.Errorf("crew %s: %w", id, err)
		}
		rep.Suggested++
	}

	return rep, r.offerCrews(ctx, q, cityID, now, &rep)
}

// offerCrews offers crews to people who got on well across outings
// (matching.FindCrews), unless those people were all offered one before,
// or two or more of them already share a crew on the same basis.
func (r *Runner) offerCrews(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time, rep *CrewReport) error {
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
	city, err := LoadCity(ctx, q, cityID, now.In(time.UTC).Month())
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
		activity, area := ptr(string(c.Basis.Activity)), ptr(string(c.Basis.Area))
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
				OfferUntil: pgtype.Timestamptz{Time: now.Add(CrewOfferFor), Valid: true}, At: at,
			})
			if err != nil {
				return err
			}
			if err := q.InsertCrewMembers(ctx, dbgen.InsertCrewMembersParams{CrewID: crewID, UserIds: members}); err != nil {
				return err
			}
			return notifyCrew(ctx, q, NotifyCrewOffered, crewID, members)
		})
		if err != nil {
			return fmt.Errorf("offer crew: %w", err)
		}
		rep.Offered++
		r.Log.Info("crew offered", "city", cityID, "members", len(members), "activity", c.Basis.Activity, "area", c.Basis.Area)
	}
	return nil
}
