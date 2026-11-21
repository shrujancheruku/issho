package retention_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/retention"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// migrated returns the tables of a database every migration has run on.
func migrated(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tables, err := retention.Tables(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

// The registry in the repo covers every table the migrations create. This is
// the `retention` check: a migration that adds a table fails here until the
// table has an entry saying what personal data it holds and for how long.
func TestRegistryCoversEveryMigratedTable(t *testing.T) {
	reg, err := retention.Load()
	if err != nil {
		t.Fatal(err)
	}
	if problems := reg.Check(migrated(t)); len(problems) != 0 {
		t.Fatalf("the registry (registry.json) and the schema disagree:\n%s", strings.Join(problems, "\n"))
	}
}

// A table holding personal data with no entry fails the check, and the message
// names the table. The fixture is a migration-shaped file that adds one.
func TestTableWithPersonalDataAndNoEntryFails(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("testdata/unregistered_table.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(fixture)); err != nil {
		t.Fatal(err)
	}
	tables, err := retention.Tables(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := retention.Load()
	if err != nil {
		t.Fatal(err)
	}

	problems := reg.Check(tables)

	if len(problems) != 1 || !strings.Contains(problems[0], "app.fixture_people") || !strings.Contains(problems[0], "no entry") {
		t.Fatalf("want one problem naming app.fixture_people, got %q", problems)
	}
}

func TestEntryForATableThatIsGoneFails(t *testing.T) {
	reg, err := retention.Parse(strings.NewReader(`[
		{"table": "app.gone", "personal_data": "none"}
	]`))
	if err != nil {
		t.Fatal(err)
	}

	problems := reg.Check(nil)

	if len(problems) != 1 || !strings.Contains(problems[0], "app.gone") {
		t.Fatalf("want one problem naming app.gone, got %q", problems)
	}
}

func TestParseRejectsAnEntryThatSaysNothingUseful(t *testing.T) {
	for name, entry := range map[string]string{
		"personal data with no lifetime": `{"table": "app.a", "personal_data": "email address"}`,
		"no table":                       `{"personal_data": "none"}`,
		"no personal_data":               `{"table": "app.a"}`,
		"a table outside a schema":       `{"table": "a", "personal_data": "none"}`,
		"unknown field":                  `{"table": "app.a", "personal_data": "none", "lifetime": "x", "kept": 1}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := retention.Parse(strings.NewReader("[" + entry + "]")); err == nil {
				t.Fatalf("Parse accepted %s", entry)
			}
		})
	}
}

func TestParseRejectsTheSameTableTwice(t *testing.T) {
	_, err := retention.Parse(strings.NewReader(`[
		{"table": "app.a", "personal_data": "none"},
		{"table": "app.a", "personal_data": "none"}
	]`))
	if err == nil {
		t.Fatal("Parse accepted a table listed twice")
	}
}
