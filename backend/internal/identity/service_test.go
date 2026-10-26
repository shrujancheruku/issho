package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type fakeStore struct {
	users     map[uuid.UUID]dbgen.AppUser
	profiles  map[uuid.UUID]dbgen.GetDisplayLanguagesRow
	suspended map[uuid.UUID]bool
	deleteErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[uuid.UUID]dbgen.AppUser{}, profiles: map[uuid.UUID]dbgen.GetDisplayLanguagesRow{}}
}

func (f *fakeStore) UpsertUser(_ context.Context, arg dbgen.UpsertUserParams) (dbgen.AppUser, error) {
	u, ok := f.users[arg.ID]
	if !ok {
		u = dbgen.AppUser{ID: arg.ID}
	}
	if arg.Email != nil { // mirrors the SQL coalesce: never clear a stored email
		u.Email = arg.Email
	}
	f.users[arg.ID] = u
	return u, nil
}

func (f *fakeStore) GetDisplayLanguages(_ context.Context, id uuid.UUID) (dbgen.GetDisplayLanguagesRow, error) {
	p, ok := f.profiles[id]
	if !ok {
		return p, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeStore) IsSuspended(_ context.Context, id uuid.UUID) (bool, error) {
	return f.suspended[id], nil
}

func (f *fakeStore) DeleteUser(_ context.Context, id uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.users, id)
	return nil
}

type fakeAdmin struct {
	err     error
	deleted []uuid.UUID
}

func (f *fakeAdmin) DeleteUser(_ context.Context, id uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func TestMe(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := &Service{Store: store}
	c := auth.Caller{UserID: uuid.New(), Email: "a@example.com"}

	acct, err := svc.Me(ctx, c)
	if err != nil || acct.User.ID != c.UserID || acct.Onboarded {
		t.Fatalf("first call: %+v, %v", acct, err)
	}
	// A later token without an email must not erase the stored one.
	acct, _ = svc.Me(ctx, auth.Caller{UserID: c.UserID})
	if acct.User.Email == nil || *acct.User.Email != "a@example.com" {
		t.Fatalf("email lost: %+v", acct.User)
	}
	es := "es"
	store.profiles[c.UserID] = dbgen.GetDisplayLanguagesRow{PrimaryLanguage: "ja", SecondaryLanguage: &es, CityID: "tokyo"}
	if acct, _ = svc.Me(ctx, c); !acct.Onboarded || acct.PrimaryLanguage != "ja" || acct.SecondaryLanguage != "es" || acct.CityID != "tokyo" {
		t.Fatalf("expected onboarded with languages and city once a profile exists: %+v", acct)
	}
	if acct.Suspended {
		t.Fatal("suspended without a suspension")
	}
	store.suspended = map[uuid.UUID]bool{c.UserID: true}
	if acct, err = svc.Me(ctx, c); err != nil || !acct.Suspended || !acct.Onboarded {
		t.Fatalf("a suspended account still loads, saying so: %+v, %v", acct, err)
	}
}

func TestDeleteAccount(t *testing.T) {
	ctx := context.Background()
	c := auth.Caller{UserID: uuid.New()}

	t.Run("not configured", func(t *testing.T) {
		svc := &Service{Store: newFakeStore()}
		if err := svc.DeleteAccount(ctx, c); !errors.Is(err, ErrDeletionUnavailable) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("auth deletion fails: app data kept", func(t *testing.T) {
		store := newFakeStore()
		store.users[c.UserID] = dbgen.AppUser{ID: c.UserID}
		svc := &Service{Store: store, Admin: &fakeAdmin{err: errors.New("supabase down")}}
		if err := svc.DeleteAccount(ctx, c); err == nil {
			t.Fatal("expected error")
		}
		if _, ok := store.users[c.UserID]; !ok {
			t.Fatal("app user deleted although auth user still exists")
		}
	})

	t.Run("success deletes auth then app user", func(t *testing.T) {
		store := newFakeStore()
		store.users[c.UserID] = dbgen.AppUser{ID: c.UserID}
		admin := &fakeAdmin{}
		svc := &Service{Store: store, Admin: admin}
		if err := svc.DeleteAccount(ctx, c); err != nil {
			t.Fatal(err)
		}
		if len(admin.deleted) != 1 || admin.deleted[0] != c.UserID {
			t.Fatalf("auth deletions = %v", admin.deleted)
		}
		if _, ok := store.users[c.UserID]; ok {
			t.Fatal("app user not deleted")
		}
	})
}
