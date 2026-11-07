package testfixture

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { Run(m) }

func TestWithDatabase(t *testing.T) {
	got, err := withDatabase("postgres://u:p%40ss@localhost:5432/issho_template?sslmode=disable", "x")
	if err != nil {
		t.Fatal(err)
	}
	if want := "postgres://u:p%40ss@localhost:5432/x?sslmode=disable"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// The package gets a database of its own, cloned from the template (so
// migrated and seeded), and never the template itself.
func TestIntegrationPackageDatabase(t *testing.T) {
	tmpl, _ := databaseName(os.Getenv("TEST_DATABASE_URL"))
	pool := Pool(t)
	var name string
	var migrated bool
	if err := pool.QueryRow(context.Background(),
		"select current_database(), to_regclass('app.cities') is not null").Scan(&name, &migrated); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, DatabasePrefix) {
		t.Errorf("database %q is not a per-package clone", name)
	}
	if name == tmpl {
		t.Error("connected to the template itself")
	}
	if !migrated {
		t.Error("the clone has no app schema")
	}
	first := DatabaseURL(t)
	if second := DatabaseURL(t); first != second {
		t.Error("two calls in one package gave different databases")
	}
}
