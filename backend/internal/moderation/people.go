package moderation

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

const (
	// searchRows and outingRows bound the person search and outing list.
	searchRows = 50
	outingRows = 50
)

func (h *Handler) SearchPeople(ctx context.Context, req *connect.Request[isshov1.SearchPeopleRequest]) (*connect.Response[isshov1.SearchPeopleResponse], error) {
	if _, err := moderator(ctx); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.Msg.Query)
	if utf8.RuneCountInString(query) < 2 {
		return nil, invalid("query: at least 2 characters")
	}
	rows, err := h.Q.SearchPeople(ctx, dbgen.SearchPeopleParams{Query: query, MaxRows: searchRows})
	if err != nil {
		return nil, err
	}
	out := &isshov1.SearchPeopleResponse{People: make([]*isshov1.PersonSummary, len(rows))}
	for i, r := range rows {
		out.People[i] = &isshov1.PersonSummary{UserId: r.ID.String(), Name: r.DisplayName, Email: r.Email, Suspended: r.Suspended}
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetPerson(ctx context.Context, req *connect.Request[isshov1.GetPersonRequest]) (*connect.Response[isshov1.GetPersonResponse], error) {
	if _, err := moderator(ctx); err != nil {
		return nil, err
	}
	id, err := parseUser(req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	p, err := h.Q.GetPerson(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such person"))
	}
	if err != nil {
		return nil, err
	}
	out := &isshov1.Person{
		UserId:           p.ID.String(),
		Name:             p.DisplayName,
		Email:            p.Email,
		JoinedAt:         timestamp(p.CreatedAt),
		Gender:           p.Gender,
		BirthYear:        p.BirthYear,
		HomeArea:         p.HomeArea,
		CityId:           p.CityID,
		ReportsAbout:     p.ReportsAbout,
		OpenReportsAbout: p.OpenReportsAbout,
		ReportsMade:      p.ReportsMade,
	}
	suspensions, err := h.Q.ListPersonSuspensions(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, s := range suspensions {
		out.Suspensions = append(out.Suspensions, &isshov1.Suspension{
			Id: s.ID, Reason: s.Reason, CreatedBy: s.CreatedBy.String(), CreatedAt: timestamp(s.CreatedAt),
			LiftedAt: timestamp(s.LiftedAt), LiftedBy: uuidString(s.LiftedBy), LiftNote: s.LiftNote,
		})
		if !s.LiftedAt.Valid {
			out.Suspended = true
		}
	}
	flags, err := h.Q.ListPersonFlags(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, f := range flags {
		out.Flags = append(out.Flags, &isshov1.ModerationFlag{
			Id: f.ID, UserId: out.UserId, UserName: out.Name, Kind: flagKinds[f.Kind], DetailsJson: string(f.Details),
			Note: f.Note, CreatedAt: timestamp(f.CreatedAt), UpdatedAt: timestamp(f.UpdatedAt),
			ResolvedAt: timestamp(f.ResolvedAt), ResolvedBy: uuidString(f.ResolvedBy),
		})
	}
	outings, err := h.Q.ListPersonOutings(ctx, dbgen.ListPersonOutingsParams{UserID: id, MaxRows: outingRows})
	if err != nil {
		return nil, err
	}
	for _, o := range outings {
		po := &isshov1.PersonOuting{
			EventId: o.EventID.String(), Title: o.Title, Block: o.Block, EventStatus: o.EventStatus,
			Response: o.Response, CheckedIn: o.CheckedIn,
		}
		if o.Date.Valid {
			po.Date = o.Date.Time.Format(time.DateOnly)
		}
		if o.SaidAttended != nil {
			po.Feedback = isshov1.OutingFeedback_OUTING_FEEDBACK_SAID_DIDNT_GO
			if *o.SaidAttended {
				po.Feedback = isshov1.OutingFeedback_OUTING_FEEDBACK_SAID_WENT
			}
		}
		out.Outings = append(out.Outings, po)
	}
	return connect.NewResponse(&isshov1.GetPersonResponse{Person: out}), nil
}

func (h *Handler) SuspendPerson(ctx context.Context, req *connect.Request[isshov1.SuspendPersonRequest]) (*connect.Response[isshov1.SuspendPersonResponse], error) {
	c, err := moderator(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseUser(req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if id == c.UserID {
		return nil, invalid("user_id: you can't suspend yourself")
	}
	reason := strings.TrimSpace(req.Msg.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > MaxNote {
		return nil, invalid("reason: 1-2000 characters")
	}
	if _, err := h.Q.GetPerson(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such person"))
	} else if err != nil {
		return nil, err
	}
	now := h.now()
	// The suspension first, so the API refuses them from now on; then
	// take them out of what they'd joined. If that fails part-way,
	// suspending again finishes it (the insert does nothing the second
	// time).
	if _, err := h.Q.InsertSuspension(ctx, dbgen.InsertSuspensionParams{
		UserID: id, Reason: reason, CreatedBy: c.UserID, At: pgTime(now),
	}); err != nil {
		return nil, err
	}
	h.Suspensions.Invalidate()
	if err := h.Plans.RemoveFromUpcoming(ctx, id, now); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SuspendPersonResponse{}), nil
}

func (h *Handler) LiftSuspension(ctx context.Context, req *connect.Request[isshov1.LiftSuspensionRequest]) (*connect.Response[isshov1.LiftSuspensionResponse], error) {
	c, err := moderator(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseUser(req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	note, err := cleanNote(req.Msg.Note)
	if err != nil {
		return nil, err
	}
	n, err := h.Q.LiftSuspension(ctx, dbgen.LiftSuspensionParams{UserID: id, LiftedBy: pgUUID(c.UserID), Note: note, At: pgTime(h.now())})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("not suspended"))
	}
	h.Suspensions.Invalidate()
	return connect.NewResponse(&isshov1.LiftSuspensionResponse{}), nil
}

func (h *Handler) ListSuspended(ctx context.Context, _ *connect.Request[isshov1.ListSuspendedRequest]) (*connect.Response[isshov1.ListSuspendedResponse], error) {
	if _, err := moderator(ctx); err != nil {
		return nil, err
	}
	rows, err := h.Q.ListOpenSuspensions(ctx, maxRows)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListSuspendedResponse{People: make([]*isshov1.SuspendedPerson, len(rows))}
	for i, r := range rows {
		out.People[i] = &isshov1.SuspendedPerson{
			UserId: r.UserID.String(), Name: r.DisplayName, Reason: r.Reason,
			Since: timestamp(r.CreatedAt), SuspendedBy: r.CreatedBy.String(),
		}
	}
	return connect.NewResponse(out), nil
}

func parseUser(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, invalid("user_id: not a user id")
	}
	return id, nil
}
