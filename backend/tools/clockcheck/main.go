// Command clockcheck fails when Go code under a directory reads the system
// clock with time.Now, time.Since or time.Until, or builds the real clock,
// clock.Real, outside cmd/. Only internal/clock may read the system clock and
// only the commands may wire it in: the rest of the backend takes an injected
// clock.Clock, so a test can set the time ("One city clock" in
// docs/spec/architecture.md). It runs from `make lint`.
//
// Usage, from backend/: go run ./tools/clockcheck
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The package-level functions of package time that read the system clock.
var banned = map[string]bool{"Now": true, "Since": true, "Until": true}

func main() {
	// internal/clock is the one package allowed to call time.Now, and cmd/ the
	// one tree allowed to build clock.Real.
	found, err := check(".", filepath.Join("internal", "clock"), []string{"cmd"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "clockcheck:", err)
		os.Exit(2)
	}
	for _, f := range found {
		fmt.Fprintln(os.Stderr, f)
	}
	if len(found) > 0 {
		fmt.Fprintln(os.Stderr, "clockcheck: take a clock.Clock instead; only internal/clock reads the system clock and only cmd/ builds clock.Real")
		os.Exit(1)
	}
}

// The import path of the clock package, whose Real only cmd/ may build.
const clockPath = "github.com/shrujancheruku/issho/backend/internal/clock"

// check parses every .go file under root (tests included: they take the fake
// clock too) and returns one "path:line: message" per read of the system clock
// and per clock.Real outside the wiring directories. A directory named testdata
// is not walked, since the go tool ignores it, and neither is clockDir, the
// clock's own. Paths are relative to the same base as root.
func check(root, clockDir string, wiring []string) ([]string, error) {
	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if clockDir != "" && filepath.Clean(path) == filepath.Clean(clockDir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		found = append(found, inFile(fset, file, !under(path, wiring))...)
		return nil
	})
	return found, err
}

func under(path string, dirs []string) bool {
	for _, d := range dirs {
		d = filepath.Clean(d)
		if p := filepath.Clean(path); p == d || strings.HasPrefix(p, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// importNames returns the names an import path goes by in a file: its last
// element unless aliased, and a file may import it more than once.
func importNames(file *ast.File, path string) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p != path {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}
	return names
}

// inFile finds the reads of the system clock in a file and, when flagReal is
// set, each use of clock.Real.
func inFile(fset *token.FileSet, file *ast.File, flagReal bool) []string {
	timeNames := importNames(file, "time")
	clockNames := map[string]bool{}
	if flagReal {
		clockNames = importNames(file, clockPath)
	}

	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// Obj is nil for a name that is not declared in the file's own scopes,
		// which is what an import is; a local variable called "time" has one.
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil {
			return true
		}
		pos := fset.Position(sel.Pos())
		at := fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line)
		switch {
		case timeNames[id.Name] && banned[sel.Sel.Name]:
			found = append(found, fmt.Sprintf("%s: time.%s reads the system clock", at, sel.Sel.Name))
		case clockNames[id.Name] && sel.Sel.Name == "Real":
			found = append(found, at+": clock.Real built outside cmd/; take a clock.Clock")
		}
		return true
	})
	return found
}
