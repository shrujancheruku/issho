package db_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

func schemas(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`select nspname from pg_namespace where nspname in ('app', 'river', 'goose', 'public') order by 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// objectsIn lists the tables, views, sequences, types and functions in a
// schema, so "nothing in public" covers everything a migration could create.
func objectsIn(t *testing.T, pool *pgxpool.Pool, schema string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select c.relname from pg_class c join pg_namespace n on n.oid = c.relnamespace where n.nspname = $1
		union all
		select t.typname from pg_type t join pg_namespace n on n.oid = t.typnamespace
		  where n.nspname = $1 and t.typtype in ('e', 'd', 'c') and t.typrelid = 0
		union all
		select p.proname from pg_proc p join pg_namespace n on n.oid = p.pronamespace where n.nspname = $1
		order by 1`, schema)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestUpCreatesAppAndRiverAndKeepsPublicEmpty(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	got := schemas(t, pool)
	for _, want := range []string{"app", "river", "goose"} {
		if !slices.Contains(got, want) {
			t.Errorf("schema %q missing after up; have %v", want, got)
		}
	}
	if got := objectsIn(t, pool, "public"); len(got) != 0 {
		t.Errorf("public holds %v, want nothing", got)
	}
	if !slices.Contains(objectsIn(t, pool, "goose"), "goose_db_version") {
		t.Errorf("goose's version table is not in the goose schema: %v", objectsIn(t, pool, "goose"))
	}
	if !slices.Contains(objectsIn(t, pool, "river"), "river_job") {
		t.Errorf("river_job is not in the river schema: %v", objectsIn(t, pool, "river"))
	}
}

// Down must undo the whole baseline so that CI's up -> down -> up (and a
// developer's local reset) works.
func TestDownThenUpAgain(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := db.DownAll(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got := schemas(t, pool)
	for _, gone := range []string{"app", "river"} {
		if slices.Contains(got, gone) {
			t.Errorf("schema %q still there after down; have %v", gone, got)
		}
	}
	if got := objectsIn(t, pool, "public"); len(got) != 0 {
		t.Errorf("public holds %v after down, want nothing", got)
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

// The queue's migration (00002) is undone by one step of Down, which leaves the
// baseline in place, and goes up again.
func TestQueueMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "queue_items") {
		t.Fatalf("app.queue_items missing after up: %v", objectsIn(t, pool, "app"))
	}

	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if got := objectsIn(t, pool, "app"); len(got) != 0 {
		t.Errorf("app holds %v after one down, want nothing (queue_items and its index gone)", got)
	}
	if !slices.Contains(schemas(t, pool), "app") {
		t.Error("one down removed the app schema, which is the baseline's")
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "queue_items") {
		t.Error("app.queue_items missing after up, down, up")
	}
}
