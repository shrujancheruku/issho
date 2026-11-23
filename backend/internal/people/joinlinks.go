package people

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shrujancheruku/issho/backend/internal/people/store"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// Join links and admission (docs/spec/people.md, "Join links"; docs/spec/launch.md,
// "The Pilot"). Admission is a fact about an account that this package owns: a
// row in app.admissions. In the Pilot a signup needs it, and a join link, or the
// seed command for the owner, is the only way to get it. Other packages ask
// Admitted or RequireAdmitted, never the table.

var (
	// ErrNotAdmitted: the account hasn't been admitted.
	ErrNotAdmitted = errors.New("people: account is not admitted")
	// ErrAlreadyAdmitted: the account is already admitted, so a link it opens is
	// left unused for the friend it was made for.
	ErrAlreadyAdmitted = errors.New("people: account is already admitted")
	// ErrNoJoinLinksLeft: the member already has policy.JoinLinksPerMember links
	// that haven't returned (used, or outstanding and not yet expired).
	ErrNoJoinLinksLeft = errors.New("people: no join links left")
	// ErrJoinLinkNotFound: the member has no such link, or it can no longer be
	// cancelled (it was used, expired or cancelled already).
	ErrJoinLinkNotFound = errors.New("people: no such join link")
	// ErrJoinLinkUnusable covers every reason a code admits nobody: it is no
	// link's, or the link was used, cancelled or has expired. One error, so a
	// caller can't tell a made-up code from a used one.
	ErrJoinLinkUnusable = errors.New("people: join link cannot be used")
)

// JoinLinkState is where a link stands.
type JoinLinkState string

const (
	JoinLinkOutstanding JoinLinkState = "outstanding" // can still admit someone
	JoinLinkUsed        JoinLinkState = "used"        // admitted someone; does not return to the member's links
	JoinLinkExpired     JoinLinkState = "expired"     // older than policy.JoinLinkLife; returned
)

// JoinLink is a member's link. Code is the secret the member shares, so a link
// is only ever shown to the member who made it. It never says who used it.
type JoinLink struct {
	ID        string
	Code      string
	State     JoinLinkState
	CreatedAt time.Time
	ExpiresAt time.Time
}

// JoinLinks is a member's own links, and how many more they can make.
type JoinLinks struct {
	// Links are the member's links that haven't been cancelled, newest first.
	Links []JoinLink
	// Remaining is how many more links the member can make.
	Remaining int
}

// codeBytes is how many random bytes make a link's code: 128 bits, so a code
// can't be guessed, and the 22 characters it encodes to fit in a URL.
const codeBytes = 16

func newCode() (string, error) {
	b := make([]byte, codeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// notExpiredAfter is the oldest created_at a link can have and still be valid
// at now: a link expires policy.JoinLinkLife after it was made, so it is
// still valid strictly before that instant.
func notExpiredAfter(now time.Time) time.Time { return now.Add(-policy.JoinLinkLife) }

func joinLinkOf(r store.AppJoinLink, now time.Time) JoinLink {
	l := JoinLink{ID: r.ID, Code: r.Code, State: JoinLinkOutstanding, CreatedAt: r.CreatedAt.UTC(), ExpiresAt: r.CreatedAt.Add(policy.JoinLinkLife).UTC()}
	switch {
	case r.RedeemedAt != nil:
		l.State = JoinLinkUsed
	case !now.Before(l.ExpiresAt):
		l.State = JoinLinkExpired
	}
	return l
}

// Admitted is whether the account has been admitted.
func (s *Service) Admitted(ctx context.Context, accountID string) (bool, error) {
	ok, err := store.New(s.pool).IsAdmitted(ctx, lowerASCII(accountID))
	if err != nil {
		if isBadID(err) {
			return false, nil
		}
		return false, fmt.Errorf("people: admitted: %w", err)
	}
	return ok, nil
}

// RequireAdmitted is nil for an admitted account, ErrNotAdmitted for one that
// isn't, and ErrNotFound for an id with no account: what a signup asks first.
func (s *Service) RequireAdmitted(ctx context.Context, accountID string) error {
	ok, err := s.Admitted(ctx, accountID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if _, err := s.Account(ctx, accountID); err != nil {
		return err
	}
	return ErrNotAdmitted
}

// MakeJoinLink makes a link for an admitted member while they have fewer than
// policy.JoinLinksPerMember that haven't returned, else ErrNoJoinLinksLeft. An
// account that isn't admitted has none to give (ErrNotAdmitted).
func (s *Service) MakeJoinLink(ctx context.Context, memberID string) (JoinLink, error) {
	memberID = lowerASCII(memberID)
	now := s.clock.Now()
	code, err := newCode()
	if err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)

	// The member's account row is locked until commit, so two calls at once
	// count one another instead of both seeing a free place.
	if _, err := q.LockAccount(ctx, memberID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isBadID(err) {
			return JoinLink{}, ErrNotFound
		}
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	if ok, err := q.IsAdmitted(ctx, memberID); err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	} else if !ok {
		return JoinLink{}, ErrNotAdmitted
	}
	held, err := q.CountHeldJoinLinks(ctx, store.CountHeldJoinLinksParams{MemberID: memberID, CreatedAt: notExpiredAfter(now)})
	if err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	if held >= policy.JoinLinksPerMember {
		return JoinLink{}, ErrNoJoinLinksLeft
	}
	row, err := q.InsertJoinLink(ctx, store.InsertJoinLinkParams{MemberID: memberID, Code: code, CreatedAt: now})
	if err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return JoinLink{}, fmt.Errorf("people: make join link: %w", err)
	}
	return joinLinkOf(row, now), nil
}

// CancelJoinLink cancels the member's own link while it is outstanding, and it
// returns to their links. A link that is another member's, was used, has
// expired or is no link is ErrJoinLinkNotFound.
func (s *Service) CancelJoinLink(ctx context.Context, memberID, linkID string) error {
	now := s.clock.Now()
	_, err := store.New(s.pool).CancelJoinLink(ctx, store.CancelJoinLinkParams{
		ID: lowerASCII(linkID), MemberID: lowerASCII(memberID), CancelledAt: &now, CreatedAt: notExpiredAfter(now),
	})
	if errors.Is(err, pgx.ErrNoRows) || isBadID(err) {
		return ErrJoinLinkNotFound
	}
	if err != nil {
		return fmt.Errorf("people: cancel join link: %w", err)
	}
	return nil
}

// MyJoinLinks is the member's own links and how many more they can make. Only
// an admitted member has links; for anyone else it is empty with none left.
func (s *Service) MyJoinLinks(ctx context.Context, memberID string) (JoinLinks, error) {
	memberID = lowerASCII(memberID)
	now := s.clock.Now()
	q := store.New(s.pool)
	rows, err := q.ListJoinLinks(ctx, memberID)
	if err != nil {
		if isBadID(err) {
			return JoinLinks{}, nil
		}
		return JoinLinks{}, fmt.Errorf("people: join links: %w", err)
	}
	out := JoinLinks{Links: make([]JoinLink, len(rows))}
	held := 0
	for i, r := range rows {
		out.Links[i] = joinLinkOf(r, now)
		if out.Links[i].State != JoinLinkExpired {
			held++
		}
	}
	out.Remaining = max(0, policy.JoinLinksPerMember-held)
	return out, nil
}

// RedeemJoinLink admits the signed-in account with the link's code and uses the
// link, in one transaction: if the account can't be admitted the link stays
// unused. The link's member is recorded as who brought the account in.
// ErrJoinLinkUnusable for a code that is no link's or a link that was used,
// cancelled or has expired; ErrAlreadyAdmitted for an account that is already
// admitted (the link is left alone); ErrNotFound for no such account.
func (s *Service) RedeemJoinLink(ctx context.Context, accountID, code string) error {
	accountID = lowerASCII(accountID)
	now := s.clock.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("people: redeem join link: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)

	if _, err := q.LockAccount(ctx, accountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isBadID(err) {
			return ErrNotFound
		}
		return fmt.Errorf("people: redeem join link: %w", err)
	}
	if ok, err := q.IsAdmitted(ctx, accountID); err != nil {
		return fmt.Errorf("people: redeem join link: %w", err)
	} else if ok {
		return ErrAlreadyAdmitted
	}
	used, err := q.UseJoinLink(ctx, store.UseJoinLinkParams{Code: code, RedeemedAt: &now, CreatedAt: notExpiredAfter(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrJoinLinkUnusable
	}
	if err != nil {
		return fmt.Errorf("people: redeem join link: %w", err)
	}
	if err := q.AdmitByJoinLink(ctx, store.AdmitByJoinLinkParams{AccountID: accountID, AdmittedAt: now, BroughtBy: &used.MemberID, LinkID: &used.ID}); err != nil {
		return fmt.Errorf("people: redeem join link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("people: redeem join link: %w", err)
	}
	return nil
}

// BroughtInBy is the account of the member whose link admitted the account, and
// false for one that was admitted by the seed or isn't admitted. It is for the
// moderator's view of "who brought in whom" (docs/spec/safety.md, "Join links")
// and nothing a member can call returns it.
func (s *Service) BroughtInBy(ctx context.Context, accountID string) (string, bool, error) {
	by, err := store.New(s.pool).GetBroughtInBy(ctx, lowerASCII(accountID))
	if errors.Is(err, pgx.ErrNoRows) || isBadID(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("people: brought in by: %w", err)
	}
	if by == nil {
		return "", false, nil
	}
	return *by, true, nil
}

// DBTX is a pool or a transaction, whichever the caller writes through.
type DBTX = store.DBTX

// AdmitByEmail admits the account that signed in with the email, with no link
// and nobody who brought it in: the way the Pilot's first member, the owner, is
// admitted so there is someone to make the first links (cmd/seed). Running it
// again changes nothing. The account must already exist (it is made by its
// first sign-in): ErrNotFound otherwise. It is a plain function over a DBTX so
// the seed can run it in the transaction it checks the environment in.
func AdmitByEmail(ctx context.Context, db DBTX, email string, at time.Time) error {
	q := store.New(db)
	id, err := q.GetAccountIDByEmail(ctx, lowerASCII(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("people: admit: %w", err)
	}
	if err := q.AdmitBySeed(ctx, store.AdmitBySeedParams{AccountID: id, AdmittedAt: at}); err != nil {
		return fmt.Errorf("people: admit: %w", err)
	}
	return nil
}

// isBadID is whether the error is Postgres refusing a malformed uuid, which
// names no account or link.
func isBadID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
