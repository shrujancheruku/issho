package moderation

import (
	"context"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// suspensionsTTL bounds how long another API instance can take to notice a
// suspension (the instance that made it notices at once).
const suspensionsTTL = 15 * time.Second

// Suspensions answers "is this person suspended?" for every API call
// without a query each time: it keeps everyone suspended now (a short
// list) in memory, reloaded when older than 15 s.
type Suspensions struct {
	Q interface {
		ListSuspendedIDs(ctx context.Context) ([]uuid.UUID, error)
	}
	Now func() time.Time // nil = time.Now

	mu     sync.Mutex
	ids    map[uuid.UUID]bool
	loaded time.Time
}

// Suspended reports whether the person is suspended now.
func (s *Suspensions) Suspended(ctx context.Context, id uuid.UUID) (bool, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil || now.Sub(s.loaded) >= suspensionsTTL {
		list, err := s.Q.ListSuspendedIDs(ctx)
		if err != nil {
			return false, err
		}
		s.ids = make(map[uuid.UUID]bool, len(list))
		for _, u := range list {
			s.ids[u] = true
		}
		s.loaded = now
	}
	return s.ids[id], nil
}

// Invalidate makes the next check reload, after a suspension changes.
func (s *Suspensions) Invalidate() {
	s.mu.Lock()
	s.ids = nil
	s.mu.Unlock()
}

// ErrSuspended is what a suspended person's calls get.
var ErrSuspended = connect.NewError(connect.CodePermissionDenied, errors.New("account suspended"))

// Gate refuses calls from suspended people, except the procedures in
// allowed (GetMe, so the app can say so; signing out; deleting the
// account). It runs after auth.Interceptor; public procedures have no
// caller and pass.
func Gate(s *Suspensions, allowed ...string) connect.UnaryInterceptorFunc {
	ok := make(map[string]bool, len(allowed))
	for _, p := range allowed {
		ok[p] = true
	}
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			c, signedIn := auth.FromContext(ctx)
			if !signedIn || ok[req.Spec().Procedure] {
				return next(ctx, req)
			}
			suspended, err := s.Suspended(ctx, c.UserID)
			if err != nil {
				return nil, err
			}
			if suspended {
				return nil, ErrSuspended
			}
			return next(ctx, req)
		}
	}
}
