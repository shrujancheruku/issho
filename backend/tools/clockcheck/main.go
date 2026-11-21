// Command clockcheck fails when Go code under a directory reads the system
// clock with time.Now, time.Since or time.Until. Only internal/clock may: the
// rest of the backend takes an injected clock.Clock, so a test can set the time
// ("One city clock" in docs/spec/architecture.md). It runs from `make lint`.
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
	// internal/clock is the one package allowed to call time.Now.
	found, err := check(".", []string{filepath.Join("internal", "clock")})
	if err != nil {
		fmt.Fprintln(os.Stderr, "clockcheck:", err)
		os.Exit(2)
	}
	for _, f := range found {
		fmt.Fprintln(os.Stderr, f)
	}
	if len(found) > 0 {
		fmt.Fprintln(os.Stderr, "clockcheck: take a clock.Clock instead; only internal/clock may read the system clock")
		os.Exit(1)
	}
}

// check parses every .go file under root (tests included: they take the fake
// clock too) and returns one "path:line: message" per read of the system clock.
// A directory named testdata is not walked, since the go tool ignores it, and
// neither is a directory in exempt (paths relative to the same base as root).
func check(root string, exempt []string) ([]string, error) {
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
			for _, e := range exempt {
				if filepath.Clean(path) == filepath.Clean(e) {
					return filepath.SkipDir
				}
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
		found = append(found, inFile(fset, file)...)
		return nil
	})
	return found, err
}

func inFile(fset *token.FileSet, file *ast.File) []string {
	// The names package time goes by in this file ("time" unless aliased; a
	// file may import it more than once).
	names := map[string]bool{}
	for _, imp := range file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p != "time" {
			continue
		}
		name := "time"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}
	if len(names) == 0 {
		return nil
	}

	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || !banned[sel.Sel.Name] {
			return true
		}
		// Obj is nil for a name that is not declared in the file's own scopes,
		// which is what an import is; a local variable called "time" has one.
		if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] && id.Obj == nil {
			pos := fset.Position(sel.Pos())
			found = append(found, fmt.Sprintf("%s:%d: time.%s reads the system clock", filepath.ToSlash(pos.Filename), pos.Line, sel.Sel.Name))
		}
		return true
	})
	return found
}
