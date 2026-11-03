package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
	"github.com/shrujancheruku/issho/backend/internal/worker"
)

// definedKinds lists every job kind the code defines: each `Kind()` method
// on a type named ...Args that returns a string literal or a string constant
// of its package, in non-test files under backend/internal. A new job type is
// found here without anyone remembering to list it, so a kind the client does
// not register fails the test below.
func definedKinds(t *testing.T) []string {
	t.Helper()
	type returned struct{ dir, lit, ident string }
	var found []returned
	consts := map[string]string{} // "dir.Name" -> value
	err := filepath.WalkDir(filepath.Join("..", "..", "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		dir := filepath.Dir(path)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || d.Tok != token.CONST || len(vs.Names) != len(vs.Values) {
						continue
					}
					for i, v := range vs.Values {
						if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if s, err := strconv.Unquote(lit.Value); err == nil {
								consts[dir+"."+vs.Names[i].Name] = s
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.Name != "Kind" || d.Recv == nil || len(d.Recv.List) != 1 || d.Body == nil || len(d.Body.List) != 1 {
					continue
				}
				recv, ok := d.Recv.List[0].Type.(*ast.Ident)
				if !ok || !strings.HasSuffix(recv.Name, "Args") {
					continue
				}
				ret, ok := d.Body.List[0].(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 1 {
					continue
				}
				switch r := ret.Results[0].(type) {
				case *ast.BasicLit:
					if s, err := strconv.Unquote(r.Value); err == nil {
						found = append(found, returned{dir: dir, lit: s})
					}
				case *ast.Ident:
					found = append(found, returned{dir: dir, ident: r.Name})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, f := range found {
		if f.ident == "" {
			kinds = append(kinds, f.lit)
		} else if v, ok := consts[f.dir+"."+f.ident]; ok {
			kinds = append(kinds, v)
		} else {
			t.Fatalf("%s: Kind() returns %s, which is not a string constant the scan can read", f.dir, f.ident)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// anyKind is a job of a given kind with no arguments: River looks a worker up
// by kind alone, so inserting one shows whether the kind is registered.
type anyKind string

func (k anyKind) Kind() string { return string(k) }

func TestIntegrationEveryDefinedJobKindIsRegistered(t *testing.T) {
	kinds := definedKinds(t)
	// Guard the scan itself: a refactor that hides the kinds from it must not
	// turn this test into a pass over an empty list.
	if len(kinds) < 10 {
		t.Fatalf("found only %d job kinds in the code: %v", len(kinds), kinds)
	}
	ctx := context.Background()
	pool := testfixture.Pool(t)
	client, err := worker.NewClient(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, worker.Maps{})
	if err != nil {
		t.Fatal(err)
	}
	// Roll back: nothing is left in the shared database's job table.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, kind := range kinds {
		if _, err := client.InsertTx(ctx, tx, anyKind(kind), nil); err != nil {
			t.Errorf("%s: not registered with the worker client: %v", kind, err)
		}
	}
	// The probe is meaningful: a kind nobody registered is refused.
	if _, err := client.InsertTx(ctx, tx, anyKind("not_a_job"), nil); err == nil {
		t.Error("an unregistered kind was accepted, so this test proves nothing")
	}
	var _ river.JobArgs = anyKind("")
}

// run fails at the first missing piece, before it opens anything.
func TestRunFailsWithoutConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("WORKER_DATABASE_URL", "")
	t.Setenv("ENV_FILE", "")
	err := run()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want the missing database URL named", err)
	}
}

func TestRunFailsWithoutADatabase(t *testing.T) {
	t.Setenv("WORKER_DATABASE_URL", "postgres://u:p@127.0.0.1:1/none?sslmode=disable&connect_timeout=2")
	t.Setenv("ENV_FILE", "")
	err := run()
	if err == nil || !strings.Contains(err.Error(), "ping database") {
		t.Fatalf("err = %v, want the failed ping", err)
	}
}
