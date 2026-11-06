// Package moderation serves ModerationService, the moderation queue in the
// admin SPA (M8): reports, flags, crew leave reasons, and people (looking
// someone up, suspending them).
// It also holds the API's check that refuses suspended people (Gate).
//
// Every call is for admins only. cmd/api enforces that for the whole
// service (auth.AdminOnly); each call checks again (auth.RequireAdmin) so a wiring mistake
// fails closed.
package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/paging"
	"github.com/shrujancheruku/issho/backend/internal/platform/pgconv"
)

// MaxNote matches the database check on notes.
const MaxNote = 2000

// Handler serves ModerationService.
type Handler struct {
	Q *dbgen.Queries
	// Svc suspends and lifts (and tells the API's check).
	Svc *Service
	Now func() time.Time // nil = time.Now
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

func (h *Handler) ListReports(ctx context.Context, req *isshov1.ListReportsRequest) (*isshov1.ListReportsResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	size, err := paging.Size(req.PageSize)
	if err != nil {
		return nil, err
	}
	arg := dbgen.ListReportsParams{MaxRows: size + 1}
	if req.Status != isshov1.ReportStatus_REPORT_STATUS_UNSPECIFIED {
		s, ok := reportStatuses[req.Status]
		if !ok {
			return nil, errs.Invalid("status: unknown")
		}
		arg.Status = s
	}
	if req.ReportedId != "" {
		id, err := uuid.Parse(req.ReportedId)
		if err != nil {
			return nil, errs.Invalid("reported_id: not a user id")
		}
		arg.ReportedID = id
	}
	// Open reports (oldest first) come before the rest (newest first), so the
	// cursor says which part it was in.
	scope := paging.Scope("ListReports", arg.Status, arg.ReportedID.String())
	k, err := paging.Open(req.PageToken, scope, 3)
	if err != nil {
		return nil, err
	}
	arg.HasAfter, arg.AfterOpen, arg.AfterTs, arg.AfterID = k.Present, k.Bool(), pgconv.Time(k.Time()), k.UUID()
	if err := k.Err(); err != nil {
		return nil, err
	}
	rows, err := h.Q.ListReports(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows, more := paging.Trim(rows, size)
	out := &isshov1.ListReportsResponse{Reports: make([]*isshov1.ModerationReport, len(rows))}
	if more {
		last := rows[len(rows)-1]
		out.NextPageToken = paging.Encode(scope, paging.Bool(last.Status == "open"), paging.Time(last.CreatedAt.Time), paging.UUID(last.ID))
	}
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
		// The outing may be gone (no block); a block we don't know is an error.
		if r.EventBlock != "" {
			if m.EventBlock, err = plans.BlockProto(r.EventBlock); err != nil {
				return nil, fmt.Errorf("report %s: %w", r.ID, err)
			}
		}
		if r.EventDate.Valid {
			m.EventDate = r.EventDate.Time.Format(time.DateOnly)
		}
		out.Reports[i] = m
	}
	return out, nil
}

func (h *Handler) ResolveReport(ctx context.Context, req *isshov1.ResolveReportRequest) (*isshov1.ResolveReportResponse, error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.ReportId)
	if err != nil {
		return nil, errs.Invalid("report_id: not a report id")
	}
	status, ok := reportStatuses[req.Status]
	if !ok {
		return nil, errs.Invalid("status: open, actioned or dismissed")
	}
	note, err := cleanNote(req.Note)
	if err != nil {
		return nil, err
	}
	n, err := h.Q.ResolveReport(ctx, dbgen.ResolveReportParams{
		ID: id, Status: status, Note: note, ReviewedBy: pgUUID(c.UserID), At: pgconv.Time(h.now()),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errs.NotFound("no such report")
	}
	return &isshov1.ResolveReportResponse{}, nil
}

func (h *Handler) ListFlags(ctx context.Context, req *isshov1.ListFlagsRequest) (*isshov1.ListFlagsResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	size, err := paging.Size(req.PageSize)
	if err != nil {
		return nil, err
	}
	scope := paging.Scope("ListFlags", paging.Bool(req.Resolved))
	k, err := paging.Open(req.PageToken, scope, 2)
	if err != nil {
		return nil, err
	}
	arg := dbgen.ListModerationFlagsParams{Open: !req.Resolved, MaxRows: size + 1}
	arg.HasAfter, arg.AfterTs, arg.AfterID = k.Present, pgconv.Time(k.Time()), k.Int()
	if err := k.Err(); err != nil {
		return nil, err
	}
	rows, err := h.Q.ListModerationFlags(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows, more := paging.Trim(rows, size)
	out := &isshov1.ListFlagsResponse{Flags: make([]*isshov1.ModerationFlag, len(rows))}
	if more {
		// Open flags are ordered by created_at, resolved ones by resolved_at.
		last := rows[len(rows)-1]
		at := last.CreatedAt
		if req.Resolved {
			at = last.ResolvedAt
		}
		out.NextPageToken = paging.Encode(scope, paging.Time(at.Time), paging.Int(last.ID))
	}
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
	return out, nil
}

func (h *Handler) ResolveFlag(ctx context.Context, req *isshov1.ResolveFlagRequest) (*isshov1.ResolveFlagResponse, error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	note, err := cleanNote(req.Note)
	if err != nil {
		return nil, err
	}
	n, err := h.Q.ResolveModerationFlag(ctx, dbgen.ResolveModerationFlagParams{
		ID: req.FlagId, Note: note, ResolvedBy: pgUUID(c.UserID), At: pgconv.Time(h.now()),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errs.Refused("no open flag with that id")
	}
	return &isshov1.ResolveFlagResponse{}, nil
}

func (h *Handler) ListCrewLeaves(ctx context.Context, req *isshov1.ListCrewLeavesRequest) (*isshov1.ListCrewLeavesResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	size, err := paging.Size(req.PageSize)
	if err != nil {
		return nil, err
	}
	arg := dbgen.ListCrewLeavesParams{MaxRows: size + 1}
	if req.Reason != isshov1.LeaveReason_LEAVE_REASON_UNSPECIFIED {
		r, ok := leaveReasons[req.Reason]
		if !ok {
			return nil, errs.Invalid("reason: unknown")
		}
		arg.Reason = r
	}
	scope := paging.Scope("ListCrewLeaves", arg.Reason)
	k, err := paging.Open(req.PageToken, scope, 2)
	if err != nil {
		return nil, err
	}
	arg.HasAfter, arg.AfterTs, arg.AfterID = k.Present, pgconv.Time(k.Time()), k.Int()
	if err := k.Err(); err != nil {
		return nil, err
	}
	rows, err := h.Q.ListCrewLeaves(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows, more := paging.Trim(rows, size)
	out := &isshov1.ListCrewLeavesResponse{Leaves: make([]*isshov1.CrewLeave, len(rows))}
	if more {
		last := rows[len(rows)-1]
		out.NextPageToken = paging.Encode(scope, paging.Time(last.CreatedAt.Time), paging.Int(last.ID))
	}
	for i, l := range rows {
		m := &isshov1.CrewLeave{
			Id:        l.ID,
			CrewId:    l.CrewID.String(),
			CrewName:  l.CrewName,
			UserId:    uuidString(l.UserID),
			UserName:  l.UserName,
			Details:   l.Details,
			CreatedAt: timestamp(l.CreatedAt),
		}
		if m.CrewStatus, err = crews.StatusProto(l.CrewStatus); err != nil {
			return nil, fmt.Errorf("crew leave %d: %w", l.ID, err)
		}
		for k, v := range leaveReasons {
			if v == l.Reason {
				m.Reason = k
			}
		}
		out.Leaves[i] = m
	}
	return out, nil
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
