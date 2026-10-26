// Package moderation serves ModerationService, the moderation queue in the
// admin SPA (M8): reports, flags, crew leave reasons, corrections to
// machine translations, and people (looking someone up, suspending them).
// It also holds the API's check that refuses suspended people (Gate).
//
// Every call is for admins only. cmd/api enforces that for the whole
// service (auth.AdminOnly); moderator() checks again so a wiring mistake
// fails closed.
package moderation

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

const (
	// maxRows bounds each list. The queue is small at launch; paging can
	// come when it isn't.
	maxRows = 200
	// MaxNote matches the database check on notes.
	MaxNote = 2000
)

// Handler serves ModerationService.
type Handler struct {
	Q *dbgen.Queries
	// Plans and Crews take a suspended person out of what they'd joined.
	Plans *plans.Service
	Crews *crews.Service
	// Suspensions is the API's check, told at once when one changes here.
	Suspensions *Suspensions
	Now         func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// Report statuses as stored and as sent.
var reportStatuses = map[isshov1.ReportStatus]string{
	isshov1.ReportStatus_REPORT_STATUS_OPEN:      "open",
	isshov1.ReportStatus_REPORT_STATUS_ACTIONED:  "actioned",
	isshov1.ReportStatus_REPORT_STATUS_DISMISSED: "dismissed",
}

// The reasons people give, as stored (plans.Report*, plans.Leave*).
var reportReasons = map[string]isshov1.ReportReason{
	"dating":     isshov1.ReportReason_REPORT_REASON_DATING,
	"harassment": isshov1.ReportReason_REPORT_REASON_HARASSMENT,
	"unsafe":     isshov1.ReportReason_REPORT_REASON_UNSAFE,
	"no_show":    isshov1.ReportReason_REPORT_REASON_NO_SHOW,
	"other":      isshov1.ReportReason_REPORT_REASON_OTHER,
}

var leaveReasons = map[isshov1.LeaveReason]string{
	isshov1.LeaveReason_LEAVE_REASON_UNCOMFORTABLE:     "uncomfortable",
	isshov1.LeaveReason_LEAVE_REASON_NOT_INTERESTED:    "not_interested",
	isshov1.LeaveReason_LEAVE_REASON_INACTIVE:          "inactive",
	isshov1.LeaveReason_LEAVE_REASON_TOO_MANY_MESSAGES: "too_many_messages",
	isshov1.LeaveReason_LEAVE_REASON_OTHER:             "other",
}

var flagKinds = map[string]isshov1.FlagKind{
	"pick_gender_lean": isshov1.FlagKind_FLAG_KIND_PICK_GENDER_LEAN,
}

func (h *Handler) ListReports(ctx context.Context, req *connect.Request[isshov1.ListReportsRequest]) (*connect.Response[isshov1.ListReportsResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	arg := dbgen.ListReportsParams{MaxRows: maxRows}
	if req.Msg.Status != isshov1.ReportStatus_REPORT_STATUS_UNSPECIFIED {
		s, ok := reportStatuses[req.Msg.Status]
		if !ok {
			return nil, errs.Invalid("status: unknown")
		}
		arg.Status = s
	}
	if req.Msg.ReportedId != "" {
		id, err := uuid.Parse(req.Msg.ReportedId)
		if err != nil {
			return nil, errs.Invalid("reported_id: not a user id")
		}
		arg.ReportedID = id
	}
	rows, err := h.Q.ListReports(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListReportsResponse{Reports: make([]*isshov1.ModerationReport, len(rows))}
	for i, r := range rows {
		m := &isshov1.ModerationReport{
			Id:                   r.ID.String(),
			ReporterId:           uuidString(r.ReporterID),
			ReporterName:         r.ReporterName,
			ReportedId:           r.ReportedID.String(),
			ReportedName:         r.ReportedName,
			ReportsAboutReported: r.ReportsAboutReported,
			Reason:               reportReasons[r.Reason],
			Details:              r.Details,
			Note:                 r.Note,
			EventId:              uuidString(r.EventID),
			EventTitle:           r.EventTitle,
			EventBlock:           r.EventBlock,
			CrewId:               uuidString(r.CrewID),
			CrewName:             r.CrewName,
			CreatedAt:            timestamp(r.CreatedAt),
			UpdatedAt:            timestamp(r.UpdatedAt),
			ReviewedAt:           timestamp(r.ReviewedAt),
			ReviewedBy:           uuidString(r.ReviewedBy),
		}
		for k, v := range reportStatuses {
			if v == r.Status {
				m.Status = k
			}
		}
		if r.EventDate.Valid {
			m.EventDate = r.EventDate.Time.Format(time.DateOnly)
		}
		out.Reports[i] = m
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ResolveReport(ctx context.Context, req *connect.Request[isshov1.ResolveReportRequest]) (*connect.Response[isshov1.ResolveReportResponse], error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.ReportId)
	if err != nil {
		return nil, errs.Invalid("report_id: not a report id")
	}
	status, ok := reportStatuses[req.Msg.Status]
	if !ok {
		return nil, errs.Invalid("status: open, actioned or dismissed")
	}
	note, err := cleanNote(req.Msg.Note)
	if err != nil {
		return nil, err
	}
	n, err := h.Q.ResolveReport(ctx, dbgen.ResolveReportParams{
		ID: id, Status: status, Note: note, ReviewedBy: pgUUID(c.UserID), At: pgTime(h.now()),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such report"))
	}
	return connect.NewResponse(&isshov1.ResolveReportResponse{}), nil
}

func (h *Handler) ListFlags(ctx context.Context, req *connect.Request[isshov1.ListFlagsRequest]) (*connect.Response[isshov1.ListFlagsResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := h.Q.ListModerationFlags(ctx, dbgen.ListModerationFlagsParams{Open: !req.Msg.Resolved, MaxRows: maxRows})
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListFlagsResponse{Flags: make([]*isshov1.ModerationFlag, len(rows))}
	for i, f := range rows {
		out.Flags[i] = &isshov1.ModerationFlag{
			Id:          f.ID,
			UserId:      f.UserID.String(),
			UserName:    f.UserName,
			Kind:        flagKinds[f.Kind],
			DetailsJson: string(f.Details),
			Note:        f.Note,
			CreatedAt:   timestamp(f.CreatedAt),
			UpdatedAt:   timestamp(f.UpdatedAt),
			ResolvedAt:  timestamp(f.ResolvedAt),
			ResolvedBy:  uuidString(f.ResolvedBy),
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ResolveFlag(ctx context.Context, req *connect.Request[isshov1.ResolveFlagRequest]) (*connect.Response[isshov1.ResolveFlagResponse], error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	note, err := cleanNote(req.Msg.Note)
	if err != nil {
		return nil, err
	}
	n, err := h.Q.ResolveModerationFlag(ctx, dbgen.ResolveModerationFlagParams{
		ID: req.Msg.FlagId, Note: note, ResolvedBy: pgUUID(c.UserID), At: pgTime(h.now()),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no open flag with that id"))
	}
	return connect.NewResponse(&isshov1.ResolveFlagResponse{}), nil
}

func (h *Handler) ListCrewLeaves(ctx context.Context, req *connect.Request[isshov1.ListCrewLeavesRequest]) (*connect.Response[isshov1.ListCrewLeavesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	arg := dbgen.ListCrewLeavesParams{MaxRows: maxRows}
	if req.Msg.Reason != isshov1.LeaveReason_LEAVE_REASON_UNSPECIFIED {
		r, ok := leaveReasons[req.Msg.Reason]
		if !ok {
			return nil, errs.Invalid("reason: unknown")
		}
		arg.Reason = r
	}
	rows, err := h.Q.ListCrewLeaves(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListCrewLeavesResponse{Leaves: make([]*isshov1.CrewLeave, len(rows))}
	for i, l := range rows {
		m := &isshov1.CrewLeave{
			Id:         l.ID,
			CrewId:     l.CrewID.String(),
			CrewName:   l.CrewName,
			CrewStatus: l.CrewStatus,
			UserId:     uuidString(l.UserID),
			UserName:   l.UserName,
			Details:    l.Details,
			CreatedAt:  timestamp(l.CreatedAt),
		}
		for k, v := range leaveReasons {
			if v == l.Reason {
				m.Reason = k
			}
		}
		out.Leaves[i] = m
	}
	return connect.NewResponse(out), nil
}

func cleanNote(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxNote {
		return "", errs.Invalid("note: at most 2000 characters")
	}
	return s, nil
}

func timestamp(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
