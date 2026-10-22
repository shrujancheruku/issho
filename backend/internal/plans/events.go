package plans

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// An invitation's answer (app.group_members.response).
const (
	Invited   = "invited"
	Confirmed = "confirmed"
	Declined  = "declined"
	Dropped   = "dropped" // confirmed, then cancelled
)

// A group's state (app.event_groups.status).
const (
	GroupInviting  = "inviting"
	GroupConfirmed = "confirmed"
	GroupCancelled = "cancelled"
)

// What someone can do with an invitation.
type Action int

const (
	Confirm Action = iota + 1 // invited or declined → confirmed, until confirm_by
	Decline                   // invited → declined, until confirm_by
	Cancel                    // confirmed → dropped, until the event starts
)

// Notification kinds (app.notifications.kind), sent by SendNotifications.
const (
	NotifyInvited   = "event_invited"   // a round you signed up for has a date: please answer
	NotifyConfirmed = "group_confirmed" // your group is going ahead
	NotifyCancelled = "group_cancelled" // your group is cancelled
)

// notify queues a notification for each person, in the caller's
// transaction (an outbox: nothing is sent for a change that rolls back).
func notify(ctx context.Context, q *dbgen.Queries, kind string, eventID uuid.UUID, users []uuid.UUID) error {
	if len(users) == 0 {
		return nil
	}
	return q.InsertNotifications(ctx, dbgen.InsertNotificationsParams{UserIds: users, Kind: kind, EventID: eventID})
}

// Check-in opens CheckInBefore the start and closes CheckInAfter it:
// arriving a little early, or late, still counts.
const (
	CheckInBefore = 30 * time.Minute
	CheckInAfter  = 3 * time.Hour
)

// ErrCheckInClosed means it's not the time to check in.
var ErrCheckInClosed = errors.New("check-in is open from 30 minutes before the start until 3 hours after")

// RevealBefore is how long before an event its meeting point is shown to
// the people going.
const RevealBefore = 24 * time.Hour

var (
	// ErrNoEvent means no event with that id that the caller is invited to.
	ErrNoEvent = errors.New("event not found")
	// ErrEventOver means the event or the caller's group was cancelled, or
	// it already happened.
	ErrEventOver = errors.New("this event is cancelled or over")
	// ErrTooLate means the deadline for this answer has passed.
	ErrTooLate = errors.New("too late to change your answer")
	// ErrNotNow means the answer doesn't apply to the current one (e.g.
	// cancelling an invitation that wasn't confirmed).
	ErrNotNow = errors.New("that answer doesn't apply now")
)

// EventItem is a scheduled event as one invitee sees it.
type EventItem struct {
	ID, PlanID, GroupID uuid.UUID
	matching.Plan
	Venue, FollowUpVenue  string
	Route                 *Route // the plan's; nil when a group meets elsewhere
	MeetingPlace          string // a walk's meeting place (Google place id), or ""
	Title, SecondaryTitle string
	Date                  time.Time // local date in the city, as midnight UTC
	Block                 matching.Block
	StartsAt, ConfirmBy   time.Time // StartsAt zero = not set
	Status, GroupStatus   string
	Response              string
	Confirmed             int // people who confirmed in the caller's group
	Quorum, MinAttendance int
	// MeetingPoint is the exact place to meet, in the caller's language,
	// once revealed (RevealAt) to someone who confirmed; "" otherwise.
	MeetingPoint string
	RevealAt     time.Time
	meetingRaw   []byte
	// CheckedIn is set once the caller checked in; check-in is open from
	// CheckInFrom to CheckInUntil (zero without a start time).
	CheckedIn                 bool
	CheckInFrom, CheckInUntil time.Time
}

// Events returns the caller's events from yesterday on, soonest first.
func (s *Service) Events(ctx context.Context, userID uuid.UUID, now time.Time) ([]EventItem, error) {
	items, caller, err := s.events(ctx, dbgen.New(s.DB), userID, now.UTC().AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].reveal(caller, now)
	}
	return items, nil
}

// events loads the caller's events on or after since, with titles.
func (s *Service) events(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, since time.Time) ([]EventItem, Caller, error) {
	_, caller, err := LoadUser(ctx, q, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, caller, ErrNoProfile
	}
	if err != nil {
		return nil, caller, err
	}
	rows, err := q.ListUserEvents(ctx, dbgen.ListUserEventsParams{UserID: userID, Since: date(since)})
	if err != nil {
		return nil, caller, err
	}
	items := make([]EventItem, len(rows))
	plans := make([]matching.OpenPlan, len(rows))
	for i, r := range rows {
		items[i] = eventFromRow(r)
		plans[i] = matching.OpenPlan{Plan: items[i].Plan}
	}
	// Titles come from the caller's city: someone who moved still sees their
	// old events, so use each event's own city.
	byCity := map[string][]int{}
	for i, r := range rows {
		byCity[r.CityID] = append(byCity[r.CityID], i)
	}
	for city, idx := range byCity {
		ps := make([]matching.OpenPlan, len(idx))
		for j, i := range idx {
			ps[j] = plans[i]
		}
		titles, secondary, err := s.titles(ctx, q, city, caller, ps)
		if err != nil {
			return nil, caller, err
		}
		for j, i := range idx {
			items[i].Title, items[i].SecondaryTitle = titles[j], secondary[j]
		}
	}
	return items, caller, nil
}

// Event returns one of the caller's events with its stops and, if the
// caller confirmed, the first names of everyone confirmed in their group.
func (s *Service) Event(ctx context.Context, userID, eventID uuid.UUID, now time.Time) (EventItem, []Step, []string, error) {
	q := dbgen.New(s.DB)
	// Any date: a link to a past event still opens it.
	items, caller, err := s.events(ctx, q, userID, time.Time{})
	if err != nil {
		return EventItem{}, nil, nil, err
	}
	for _, it := range items {
		if it.ID != eventID {
			continue
		}
		it.reveal(caller, now)
		steps := s.steps(ctx, it.Plan, it.Venue, it.FollowUpVenue, it.MeetingPlace, it.Route, caller)
		var names []string
		// Only people who are going see who else is (no profile browsing).
		if it.Response == Confirmed {
			if names, err = q.ListConfirmedNames(ctx, it.GroupID); err != nil {
				return EventItem{}, nil, nil, err
			}
		}
		return it, steps, names, nil
	}
	return EventItem{}, nil, nil, ErrNoEvent
}

func eventFromRow(r dbgen.ListUserEventsRow) EventItem {
	it := EventItem{
		ID: r.ID, PlanID: r.PlanID, GroupID: r.GroupID,
		Plan: matching.Plan{
			Area: matching.AreaID(r.AreaID), Activity: matching.ActivityID(r.ActivityID),
			FollowUp: matching.ActivityID(deref(r.FollowUpID)), Primary: r.PrimaryLanguage, Secondary: deref(r.SecondaryLanguage),
		},
		Venue: deref(r.VenuePlaceID), FollowUpVenue: deref(r.FollowUpVenuePlaceID),
		Route: routeFrom(r.RoutePolyline, r.RouteSeconds), MeetingPlace: deref(r.MeetingPlaceID),
		Date: r.Date.Time, Block: matching.Block(r.Block),
		StartsAt: r.StartsAt.Time, ConfirmBy: r.ConfirmBy.Time,
		Status: r.Status, GroupStatus: r.GroupStatus, Response: r.Response,
		Confirmed: int(r.Confirmed), Quorum: int(r.Quorum), MinAttendance: int(r.MinAttendance),
		meetingRaw: r.MeetingPoint,
	}
	// A group sent somewhere else (a full venue, an admin) meets there; the
	// plan's route doesn't start there.
	if v := deref(r.GroupVenuePlaceID); v != "" && v != it.Venue {
		it.Venue, it.Route = v, nil
	}
	if !it.StartsAt.IsZero() {
		it.RevealAt = it.StartsAt.Add(-RevealBefore)
		it.CheckInFrom, it.CheckInUntil = it.StartsAt.Add(-CheckInBefore), it.StartsAt.Add(CheckInAfter)
	}
	it.CheckedIn = r.CheckedInAt.Valid
	return it
}

// reveal fills MeetingPoint when the caller may see it: they confirmed, the
// group is going ahead, and it's at most RevealBefore until the start.
func (it *EventItem) reveal(caller Caller, now time.Time) {
	it.MeetingPoint = ""
	if it.Response != Confirmed || it.GroupStatus == GroupCancelled || it.RevealAt.IsZero() || now.Before(it.RevealAt) {
		return
	}
	if len(it.meetingRaw) > 0 {
		it.MeetingPoint = i18n.Pick(it.meetingRaw, caller.Primary)
	}
}

// Respond records the caller's answer to an event invitation and updates
// their group: confirmed as soon as `quorum` people confirm; back to
// collecting answers, or cancelled after the deadline, when a cancellation
// leaves it below `min_attendance`.
func (s *Service) Respond(ctx context.Context, userID, eventID uuid.UUID, action Action, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		m, err := q.LockMembership(ctx, dbgen.LockMembershipParams{EventID: eventID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoEvent
		}
		if err != nil {
			return err
		}
		if m.EventStatus != "scheduled" || m.GroupStatus == GroupCancelled {
			return ErrEventOver
		}
		var next string
		switch action {
		case Confirm, Decline:
			if !now.Before(m.ConfirmBy.Time) {
				return ErrTooLate
			}
			if action == Confirm && (m.Response == Invited || m.Response == Declined) {
				next = Confirmed
			} else if action == Decline && m.Response == Invited {
				next = Declined
			} else {
				return ErrNotNow
			}
		case Cancel:
			if m.StartsAt.Valid && !now.Before(m.StartsAt.Time) {
				return ErrTooLate
			}
			if m.Response != Confirmed {
				return ErrNotNow
			}
			next = Dropped
		default:
			return fmt.Errorf("unknown action %d", action)
		}
		if err := q.SetResponse(ctx, dbgen.SetResponseParams{
			GroupID: m.GroupID, UserID: userID, Response: next, At: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return err
		}
		confirmed, err := q.CountConfirmed(ctx, m.GroupID)
		if err != nil {
			return err
		}
		switch {
		case next == Confirmed && m.GroupStatus == GroupInviting && confirmed >= int32(m.Quorum):
			if err := q.SetGroupStatus(ctx, dbgen.SetGroupStatusParams{ID: m.GroupID, Status: GroupConfirmed}); err != nil {
				return err
			}
			// Everyone who confirmed earlier hears it's on; the person who
			// just confirmed sees it on screen.
			users, err := q.ListConfirmedUsers(ctx, m.GroupID)
			if err != nil {
				return err
			}
			return notify(ctx, q, NotifyConfirmed, eventID, slices.DeleteFunc(users, func(u uuid.UUID) bool { return u == userID }))
		case next == Dropped && m.GroupStatus == GroupConfirmed && confirmed < int32(m.MinAttendance):
			if now.Before(m.ConfirmBy.Time) {
				// Still time for others to answer.
				return q.SetGroupStatus(ctx, dbgen.SetGroupStatusParams{ID: m.GroupID, Status: GroupInviting})
			}
			return cancelGroup(ctx, q, m.GroupID, m.PlanID, eventID)
		}
		return nil
	})
}

// CheckIn records that the caller has arrived: only people who confirmed,
// in a group going ahead, while check-in is open. Checking in again is
// fine and keeps the first time. Check-ins count as attending (newcomer
// status) and towards reliability.
func (s *Service) CheckIn(ctx context.Context, userID, eventID uuid.UUID, now time.Time) error {
	items, _, err := s.events(ctx, dbgen.New(s.DB), userID, time.Time{})
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.ID != eventID {
			continue
		}
		switch {
		case it.Response != Confirmed:
			return ErrNotGoing
		case it.Status == "cancelled" || it.GroupStatus == GroupCancelled:
			return ErrEventOver
		case it.CheckInFrom.IsZero() || now.Before(it.CheckInFrom) || now.After(it.CheckInUntil):
			return ErrCheckInClosed
		}
		_, err := dbgen.New(s.DB).CheckIn(ctx, dbgen.CheckInParams{EventID: eventID, UserID: userID, At: pgtype.Timestamptz{Time: now, Valid: true}})
		return err
	}
	return ErrNoEvent
}

// cancelGroup cancels a group, puts the people who had confirmed back on
// the plan's waiting list (if it's still open), and cancels the event if no
// group is left.
func cancelGroup(ctx context.Context, q *dbgen.Queries, groupID, planID, eventID uuid.UUID) error {
	if err := q.SetGroupStatus(ctx, dbgen.SetGroupStatusParams{ID: groupID, Status: GroupCancelled}); err != nil {
		return err
	}
	users, err := q.ListConfirmedUsers(ctx, groupID)
	if err != nil {
		return err
	}
	if err := notify(ctx, q, NotifyCancelled, eventID, users); err != nil {
		return err
	}
	if len(users) > 0 {
		if err := q.ReturnToSignups(ctx, dbgen.ReturnToSignupsParams{PlanID: planID, UserIds: users}); err != nil {
			return err
		}
	}
	return q.CancelEventIfNoGroups(ctx, eventID)
}

// SettleReport counts what a settling pass did.
type SettleReport struct {
	Confirmed, Cancelled, Finished int
	ChatMessagesDeleted            int
}

// Settle decides every group in a city whose confirmation deadline has
// passed: it goes ahead with at least `min_attendance` confirmations, and
// is cancelled otherwise. Events that have happened are marked done.
func (r *Runner) Settle(ctx context.Context, cityID string, now time.Time) (SettleReport, error) {
	var rep SettleReport
	q := dbgen.New(r.DB)
	loc, err := location(ctx, q, cityID)
	if err != nil {
		return rep, err
	}
	groups, err := q.ListGroupsToSettle(ctx, dbgen.ListGroupsToSettleParams{CityID: cityID, Now: pgtype.Timestamptz{Time: now, Valid: true}})
	if err != nil {
		return rep, err
	}
	for _, g := range groups {
		err := pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			locked, err := q.LockGroup(ctx, g.ID)
			if err != nil {
				return err
			}
			if locked.Status != GroupInviting {
				return nil // decided by an answer since the list was read
			}
			confirmed, err := q.CountConfirmed(ctx, g.ID)
			if err != nil {
				return err
			}
			if confirmed >= int32(locked.MinAttendance) {
				rep.Confirmed++
				if err := q.SetGroupStatus(ctx, dbgen.SetGroupStatusParams{ID: g.ID, Status: GroupConfirmed}); err != nil {
					return err
				}
				users, err := q.ListConfirmedUsers(ctx, g.ID)
				if err != nil {
					return err
				}
				return notify(ctx, q, NotifyConfirmed, locked.EventID, users)
			}
			rep.Cancelled++
			r.Log.Info("group cancelled at its deadline", "city", cityID, "event", locked.EventID, "group", g.ID, "confirmed", confirmed)
			return cancelGroup(ctx, q, g.ID, locked.PlanID, locked.EventID)
		})
		if err != nil {
			return rep, fmt.Errorf("group %s: %w", g.ID, err)
		}
	}
	finished, err := q.FinishPastEvents(ctx, dbgen.FinishPastEventsParams{CityID: cityID, Today: date(today(now, loc))})
	if err != nil {
		return rep, err
	}
	rep.Finished = int(finished)
	chats, err := DeleteOldChats(ctx, q, cityID, now)
	if err != nil {
		return rep, err
	}
	rep.ChatMessagesDeleted = int(chats)
	return rep, nil
}
