package db_test

import (
	"context"
	"testing"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// River's tables are written out in a goose migration, not run by River. This
// fails when the River in go.mod ships a migration the baseline doesn't have
// (the fix is a new goose migration with its SQL), or when the baseline's
// bookkeeping drifts from River's.
func TestBaselineMatchesRiversOwnMigrations(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	m, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.Validate(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("river schema is not at River's latest version: %v", res.Messages)
	}
}
