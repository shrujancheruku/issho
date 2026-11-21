package testdb_test

import (
	"context"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// Two tests writing the same key both pass in parallel only if each has its own
// schema; a shared one would hit the primary key.
func TestSchemaIsolation(t *testing.T) {
	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			pool := testdb.New(t)
			if _, err := pool.Exec(ctx, `create table sample (id int primary key)`); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `insert into sample values (1)`); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := pool.QueryRow(ctx, `select count(*) from sample`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("rows = %d, want 1", n)
			}
		})
	}
}
