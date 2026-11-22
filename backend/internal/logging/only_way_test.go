package logging_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// forbidden maps an import path to the package-level names a package must not
// use to log or to write to the process's output: each is a way round the
// logger a process is handed, which a test could not capture. "*" is every name.
var forbidden = map[string]map[string]bool{
	"log": {"*": true},
	"fmt": {"Print": true, "Printf": true, "Println": true},
	"log/slog": {
		"New": true, "NewTextHandler": true, "NewJSONHandler": true, "NewLogLogger": true, "Default": true, "SetDefault": true,
		"Debug": true, "Info": true, "Warn": true, "Error": true, "Log": true, "LogAttrs": true,
		"DebugContext": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true,
	},
	"os": {"Stdout": true, "Stderr": true},
}

// testMayUse is what a test may name anyway: capturing the standard logger is
// how a test proves nothing reaches it.
var testMayUse = map[string]map[string]bool{
	"log": {"SetOutput": true, "Writer": true},
}

// A helper is the only way a package logs (docs/spec/operations.md, "Errors and
// logs"): the packages under internal/ take the logger they are handed, and the
// two long-running commands build theirs with logging.New. If one built its own
// or wrote to stdout, a test could not capture a whole run's output, and a line
// could carry what the helper keeps out.
func TestOnlyTheHelperMakesALoggerOrWritesToTheProcessOutput(t *testing.T) {
	root := filepath.Join("..", "..") // backend/
	// Where a command's main may name os.Stdout: it is what it hands logging.New.
	mainMayUseStdout := func(path string) bool {
		return strings.HasPrefix(filepath.ToSlash(path), "cmd/api/") || strings.HasPrefix(filepath.ToSlash(path), "cmd/worker/")
	}

	checked := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			// logging is the helper; the rest of cmd/ and tools/ are command-line
			// programs that print their own results.
			if rel == "internal/logging" || rel == "gen" || rel == "tools" || rel == "cmd/migrate" || rel == "cmd/rpcauth" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		inScope := strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/api/") || strings.HasPrefix(rel, "cmd/worker/")
		if !inScope || !strings.HasSuffix(rel, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		checked++
		names := map[string]string{} // local name -> import path
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			local := filepath.Base(path)
			if imp.Name != nil {
				local = imp.Name.Name
			}
			names[local] = path
		}
		isTest := strings.HasSuffix(rel, "_test.go")
		ast.Inspect(f, func(n ast.Node) bool {
			// The builtins print and println write to stderr.
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Obj == nil && (id.Name == "print" || id.Name == "println") {
					t.Errorf("%s: %s: log through the *slog.Logger you are handed", fset.Position(id.Pos()), id.Name)
				}
			}
			// An http.Server with no ErrorLog writes net/http's errors to the
			// standard logger, client addresses and all.
			if lit, ok := n.(*ast.CompositeLit); ok {
				if sel, ok := lit.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Server" {
					if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] == "net/http" && !setsField(lit, "ErrorLog") {
						t.Errorf("%s: http.Server without ErrorLog: set it to logging.ServerErrorLog", fset.Position(lit.Pos()))
					}
				}
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Obj != nil {
				return true // a local variable, not the package
			}
			bad := forbidden[names[id.Name]]
			if bad == nil || !(bad["*"] || bad[sel.Sel.Name]) {
				return true
			}
			if isTest && testMayUse[names[id.Name]][sel.Sel.Name] {
				return true
			}
			if names[id.Name] == "os" && mainMayUseStdout(rel) {
				return true
			}
			t.Errorf("%s: %s.%s: log through the *slog.Logger you are handed (logging.New builds it)", fset.Position(sel.Pos()), id.Name, sel.Sel.Name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("checked no files: the walk is looking in the wrong place")
	}
}

func setsField(lit *ast.CompositeLit, name string) bool {
	for _, e := range lit.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == name {
				return true
			}
		}
	}
	return false
}
