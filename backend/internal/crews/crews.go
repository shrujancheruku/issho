// Package crews keeps people who got on well meeting: the worker offers a
// crew to people who picked each other across outings (matching.FindCrews),
// it forms when enough accept, and then has its own chat, with an open
// plan suggested in it weekly. Built on internal/plans (outings, feedback,
// reports and notifications); plans doesn't know about crews.
package crews

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// A crew's state (app.crews.status).
const (
	Offered = "offered" // waiting for answers until offer_until
	Active  = "active"
	Lapsed  = "lapsed" // too few accepted in time
	Closed  = "closed" // fewer than 2 members left
)

// A member's answer (app.crew_members.response).
const (
	Undecided = "offered" // no answer yet
	Accepted  = "accepted"
	Declined  = "declined"
	Left      = "left"
)

// Why someone left a crew (app.crew_leaves.reason).
const (
	LeaveUncomfortable   = "uncomfortable"
	LeaveNotInterested   = "not_interested"
	LeaveInactive        = "inactive"
	LeaveTooManyMessages = "too_many_messages"
	LeaveOther           = "other"
)

const (
	// OfferFor is how long people have to accept a crew.
	OfferFor = 7 * 24 * time.Hour
	// SuggestEvery is how often a plan is suggested in a crew's chat at
	// most; the first comes a day after it forms.
	SuggestEvery = 7 * 24 * time.Hour
	// suggestMinNotice: a suggested plan's signups stay open at least this
	// long, so the crew has time to sign up together.
	suggestMinNotice = 24 * time.Hour
)

var (
	// ErrNoCrew means no crew with that id that the caller was offered or is in.
	ErrNoCrew = errors.New("crew not found")
	// ErrNotMember means the caller isn't (or is no longer) a member.
	ErrNotMember = errors.New("only crew members can do that")
	// ErrOver means the offer has closed, or the crew has.
	ErrOver = errors.New("this crew offer or crew is over")
)

// Service is what CrewService does for one caller.
type Service struct {
	DB *pgxpool.Pool
}

// Item is a crew as one of its members (or someone offered it) sees it.
type Item struct {
	ID       uuid.UUID
	Status   string
	Activity string // what it's named after: an activity ...
	Area     string // ... or an area (exactly one is set)
	// OfferUntil is when the offer closes (for an offer).
	OfferUntil time.Time
	Response   string // the caller's
	// Members: everyone who accepted, and while it's an offer, everyone
	// still deciding (first names). Accepted is set for those who accepted.
	Members []Mate
}

// Mate is someone in a crew.
type Mate struct {
	ID       uuid.UUID
	Name     string
	Accepted bool
	Me       bool
}

// Message is a message in a crew's chat: someone's text, or a plan
// suggested by Issho (PlanID set, Name and Body empty).
type Message struct {
	chat.Message
	PlanID uuid.UUID
}

// List returns the caller's crews and the crew offers still open to them,
// newest first.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Item, error) {
	q := dbgen.New(s.DB)
	rows, err := q.ListUserCrews(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]Item, len(rows))
	for i, r := range rows {
		items[i] = Item{ID: r.ID, Status: r.Status, Activity: deref(r.ActivityID), Area: deref(r.AreaID), OfferUntil: r.OfferUntil.Time, Response: r.Response}
		if items[i].Members, err = mates(ctx, q, r.ID, userID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// Get returns one of the caller's crews or crew offers.
func (s *Service) Get(ctx context.Context, userID, crewID uuid.UUID) (Item, error) {
	q := dbgen.New(s.DB)
	r, err := q.GetCrewMembership(ctx, dbgen.GetCrewMembershipParams{CrewID: crewID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNoCrew
	}
	if err != nil {
		return Item{}, err
	}
	visible := (r.Status == Offered && (r.Response == Undecided || r.Response == Accepted)) ||
		(r.Status == Active && r.Response == Accepted)
	if !visible {
		return Item{}, ErrNoCrew
	}
	it := Item{ID: crewID, Status: r.Status, Activity: deref(r.ActivityID), Area: deref(r.AreaID), OfferUntil: r.OfferUntil.Time, Response: r.Response}
	it.Members, err = mates(ctx, q, crewID, userID)
	return it, err
}

func mates(ctx context.Context, q *dbgen.Queries, crewID, userID uuid.UUID) ([]Mate, error) {
	rows, err := q.ListCrewMembers(ctx, crewID)
	if err != nil {
		return nil, err
	}
	out := make([]Mate, len(rows))
	for i, r := range rows {
		out[i] = Mate{ID: r.UserID, Name: r.DisplayName, Accepted: r.Response == Accepted, Me: r.UserID == userID}
	}
	return out, nil
}

// Respond accepts or declines a crew offer while it's open; the answer can
// be changed until then. Once everyone has answered, the crew is decided
// straight away rather than at the deadline.
func (s *Service) Respond(ctx context.Context, userID, crewID uuid.UUID, accept bool, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		m, err := q.LockCrewMembership(ctx, dbgen.LockCrewMembershipParams{CrewID: crewID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCrew
		}
		if err != nil {
			return err
		}
		if m.Status != Offered || !now.Before(m.OfferUntil.Time) {
			return ErrOver
		}
		response := Declined
		if accept {
			response = Accepted
		}
		if err := q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{
			CrewID: crewID, UserID: userID, Response: response, At: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return err
		}
		waiting, err := q.CountCrewMembers(ctx, dbgen.CountCrewMembersParams{CrewID: crewID, Response: Undecided})
		if err != nil || waiting > 0 {
			return err
		}
		_, err = decide(ctx, q, crewID, now)
		return err
	})
}

// decide forms an offered crew with the people who accepted, if there are
// at least matching.CrewMinSize of them, and tells them; otherwise the offer
// lapses quietly. It reports the new status ("" if not an offer).
func decide(ctx context.Context, q *dbgen.Queries, crewID uuid.UUID, now time.Time) (string, error) {
	c, err := q.LockCrew(ctx, crewID)
	if err != nil {
		return "", err
	}
	if c.Status != Offered {
		return "", nil
	}
	accepted, err := q.ListCrewMemberIDs(ctx, dbgen.ListCrewMemberIDsParams{CrewID: crewID, Response: Accepted})
	if err != nil {
		return "", err
	}
	status := Lapsed
	if len(accepted) >= matching.CrewMinSize {
		status = Active
	}
	if err := q.SetCrewStatus(ctx, dbgen.SetCrewStatusParams{ID: crewID, Status: status, At: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
		return "", err
	}
	if status == Active {
		err = notify(ctx, q, plans.NotifyCrewFormed, crewID, accepted)
	}
	return status, err
}

func notify(ctx context.Context, q *dbgen.Queries, kind string, crewID uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	return q.InsertCrewNotifications(ctx, dbgen.InsertCrewNotificationsParams{UserIds: users, Kind: kind, CrewID: pgtype.UUID{Bytes: crewID, Valid: true}})
}

// Leave takes the caller out of a crew, with why (private: the crew isn't
// told). A crew left with fewer than 2 members closes.
func (s *Service) Leave(ctx context.Context, userID, crewID uuid.UUID, reason, details string, now time.Time) error {
	switch reason {
	case LeaveUncomfortable, LeaveNotInterested, LeaveInactive, LeaveTooManyMessages, LeaveOther:
	default:
		return plans.Invalid("reason: uncomfortable, not_interested, inactive, too_many_messages or other")
	}
	// The same limit as a report: both are a few sentences for moderators.
	details, err := plans.CheckReport(plans.ReportOther, details)
	if err != nil {
		return err
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
		if m.Response != Accepted || (m.Status != Active && m.Status != Offered) {
			return ErrNotMember
		}
		at := pgtype.Timestamptz{Time: now, Valid: true}
		if err := q.InsertCrewLeave(ctx, dbgen.InsertCrewLeaveParams{
			CrewID: crewID, UserID: pgtype.UUID{Bytes: userID, Valid: true}, Reason: reason, Details: details, At: at,
		}); err != nil {
			return err
		}
		return leave(ctx, q, crewID, userID, m.Status, at)
	})
}

// leave marks a member who accepted as gone and closes an active crew left
// with fewer than 2. An offer is decided when it closes, with whoever
// accepted.
func leave(ctx context.Context, q *dbgen.Queries, crewID, userID uuid.UUID, status string, at pgtype.Timestamptz) error {
	if err := q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{CrewID: crewID, UserID: userID, Response: Left, At: at}); err != nil {
		return err
	}
	if status != Active {
		return nil
	}
	left, err := q.CountCrewMembers(ctx, dbgen.CountCrewMembersParams{CrewID: crewID, Response: Accepted})
	if err != nil || left >= 2 {
		return err
	}
	return q.SetCrewStatus(ctx, dbgen.SetCrewStatusParams{ID: crewID, Status: Closed, At: at})
}

// RemoveFromAll takes a suspended person out of their crews
// (ModerationService.SuspendPerson): they leave active ones, which close if
// one member is left, and decline open offers. No leave reason is stored:
// app.crew_leaves is why people chose to leave. Each crew is its own
// transaction, so a retry finishes what a failure left.
func (s *Service) RemoveFromAll(ctx context.Context, userID uuid.UUID, now time.Time) error {
	crews, err := dbgen.New(s.DB).ListUserOpenCrewMemberships(ctx, userID)
	if err != nil {
		return err
	}
	at := pgtype.Timestamptz{Time: now, Valid: true}
	for _, crewID := range crews {
		if err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			m, err := q.LockCrewMembership(ctx, dbgen.LockCrewMembershipParams{CrewID: crewID, UserID: userID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			switch {
			case m.Response == Undecided && m.Status == Offered:
				return q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{CrewID: crewID, UserID: userID, Response: Declined, At: at})
			case m.Response == Accepted && (m.Status == Offered || m.Status == Active):
				return leave(ctx, q, crewID, userID, m.Status, at)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// chatAccess checks the caller is in an active crew.
func chatAccess(ctx context.Context, q *dbgen.Queries, userID, crewID uuid.UUID) error {
	r, err := q.GetCrewMembership(ctx, dbgen.GetCrewMembershipParams{CrewID: crewID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoCrew
	}
	if err != nil {
		return err
	}
	if r.Response != Accepted {
		return ErrNotMember
	}
	if r.Status != Active {
		return ErrOver
	}
	return nil
}

// Messages returns a crew's chat after message afterID (0 = the latest
// messages), oldest first.
func (s *Service) Messages(ctx context.Context, userID, crewID uuid.UUID, afterID int64) ([]Message, error) {
	q := dbgen.New(s.DB)
	if err := chatAccess(ctx, q, userID, crewID); err != nil {
		return nil, err
	}
	var rows []dbgen.ListCrewMessagesRow
	if afterID == 0 {
		rs, err := q.ListLatestCrewMessages(ctx, dbgen.ListLatestCrewMessagesParams{CrewID: crewID, Max: chat.Page})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, dbgen.ListCrewMessagesRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListCrewMessages(ctx, dbgen.ListCrewMessagesParams{CrewID: crewID, AfterID: afterID, Max: chat.Page}); err != nil {
			return nil, err
		}
	}
	out := make([]Message, len(rows))
	for i, r := range rows {
		out[i] = Message{
			Message: chat.Message{ID: r.ID, Name: deref(r.DisplayName), Body: r.Body, At: r.CreatedAt.Time, Mine: r.UserID.Valid && r.UserID.Bytes == userID},
			PlanID:  r.PlanID.Bytes,
		}
	}
	return out, nil
}

// Send posts to the caller's crew chat and tells every API instance (NOTIFY
// chat, as for event chats), so open chats update.
func (s *Service) Send(ctx context.Context, userID, crewID uuid.UUID, body string) (Message, error) {
	body, ok := chat.Clean(body)
	if !ok {
		return Message{}, plans.Invalid("message: 1 to %d characters", chat.MaxMessage)
	}
	var msg Message
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := chatAccess(ctx, q, userID, crewID); err != nil {
			return err
		}
		r, err := q.InsertCrewMessage(ctx, dbgen.InsertCrewMessageParams{CrewID: crewID, UserID: pgtype.UUID{Bytes: userID, Valid: true}, Body: body})
		if err != nil {
			return err
		}
		msg = Message{Message: chat.Message{ID: r.ID, Body: body, At: r.CreatedAt.Time, Mine: true}}
		return q.NotifyChat(ctx, crewID.String())
	})
	return msg, err
}

// ReportMember reports someone met in a crew, as plans.Service.Report does
// for an outing: both must be or have been members (accepted, or left
// since).
func (s *Service) ReportMember(ctx context.Context, userID, crewID, mateID uuid.UUID, reason, details string, now time.Time) error {
	details, err := plans.CheckReport(reason, details)
	if err != nil {
		return err
	}
	q := dbgen.New(s.DB)
	in, err := q.IsCrewMember(ctx, dbgen.IsCrewMemberParams{CrewID: crewID, UserID: userID})
	if err != nil {
		return err
	}
	if !in {
		return ErrNotMember
	}
	if mateID == userID {
		return plans.Invalid("mate_id: not someone else in your crew")
	}
	if in, err = q.IsCrewMember(ctx, dbgen.IsCrewMemberParams{CrewID: crewID, UserID: mateID}); err != nil {
		return err
	}
	if !in {
		return plans.Invalid("mate_id: not someone else in your crew")
	}
	return q.UpsertCrewReport(ctx, dbgen.UpsertCrewReportParams{
		ReporterID: pgtype.UUID{Bytes: userID, Valid: true}, ReportedID: mateID,
		CrewID: pgtype.UUID{Bytes: crewID, Valid: true}, Reason: reason, Details: details,
		At: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
