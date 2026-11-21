package db_test

import (
	"context"
	"testing"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// The sample query in db/queries/sample.sql is compiled by `make gen`; calling
// the generated method on the migrated database proves sqlc's output builds
// and runs against the real schema.
func TestGeneratedQueryRuns(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	got, err := dbgen.New(pool).CountJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("jobs on a fresh database = %d, want 0", got)
	}
}
