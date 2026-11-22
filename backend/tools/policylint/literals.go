package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// value is a constant the lint can compute without type-checking: a whole
// number, or a duration in nanoseconds (an expression that uses the time
// package's units). An int 48 and a duration of 48ns are different numbers.
type value struct {
	n        int64
	duration bool
}

// numbers maps each value in the policy package to the constants that hold it,
// in source order. Two constants may hold the same value (a round and the
// feedback window are both 7 days), and a literal repeats all of them.
type numbers map[value][]string

var timeUnits = map[string]int64{
	"Nanosecond": 1, "Microsecond": 1e3, "Millisecond": 1e6,
	"Second": 1e9, "Minute": 60e9, "Hour": 3600e9,
}

// eval computes a constant expression of integer literals, the time package's
// units and the operators + - * /. lookup resolves a bare name (a constant in
// the policy package defined from another); it may be nil. ok is false for
// anything else.
func eval(e ast.Expr, lookup func(string) (value, bool)) (v value, ok bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return value{}, false
		}
		n, err := strconv.ParseInt(strings.ReplaceAll(e.Value, "_", ""), 0, 64)
		return value{n: n}, err == nil
	case *ast.ParenExpr:
		return eval(e.X, lookup)
	case *ast.Ident:
		if lookup == nil {
			return value{}, false
		}
		return lookup(e.Name)
	case *ast.SelectorExpr: // time.Hour
		if id, isIdent := e.X.(*ast.Ident); isIdent && id.Name == "time" {
			if n, isUnit := timeUnits[e.Sel.Name]; isUnit {
				return value{n: n, duration: true}, true
			}
		}
	case *ast.CallExpr: // time.Duration(48)
		if sel, isSel := e.Fun.(*ast.SelectorExpr); isSel && len(e.Args) == 1 {
			if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "time" && sel.Sel.Name == "Duration" {
				return eval(e.Args[0], lookup)
			}
		}
	case *ast.BinaryExpr:
		a, okA := eval(e.X, lookup)
		b, okB := eval(e.Y, lookup)
		if !okA || !okB {
			return value{}, false
		}
		r := value{duration: a.duration || b.duration}
		switch e.Op {
		case token.ADD:
			r.n = a.n + b.n
		case token.SUB:
			r.n = a.n - b.n
		case token.MUL:
			r.n = a.n * b.n
		case token.QUO:
			if b.n == 0 {
				return value{}, false
			}
			r.n = a.n / b.n
		default:
			return value{}, false
		}
		return r, true
	}
	return value{}, false
}

// collectNumbers evaluates the policy package's constants. A constant it can't
// compute (a string, an iota block) isn't a number the literal check can find.
// 0 and 1 are left out: they are written everywhere for reasons that have
// nothing to do with a rule, so flagging them would drown the real findings.
func collectNumbers(pkgs map[string]*ast.Package) numbers {
	type def struct {
		name string
		expr ast.Expr
	}
	var defs []def
	byName := map[string]ast.Expr{}
	var paths []string
	files := map[string]*ast.File{}
	for _, pkg := range pkgs {
		for path, f := range pkg.Files {
			paths = append(paths, path)
			files[path] = f
		}
	}
	sort.Strings(paths) // a stable order for the names a finding lists
	for _, path := range paths {
		for _, decl := range files[path].Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				if len(vs.Values) != len(vs.Names) {
					continue
				}
				for i, name := range vs.Names {
					defs = append(defs, def{name.Name, vs.Values[i]})
					byName[name.Name] = vs.Values[i]
				}
			}
		}
	}

	var lookup func(string) (value, bool)
	seen := map[string]bool{} // guards a constant that (illegally) refers to itself
	lookup = func(name string) (value, bool) {
		e, ok := byName[name]
		if !ok || seen[name] {
			return value{}, false
		}
		seen[name] = true
		defer delete(seen, name)
		return eval(e, lookup)
	}

	nums := numbers{}
	for _, d := range defs {
		v, ok := lookup(d.name)
		if !ok || (v.n >= -1 && v.n <= 1) {
			continue
		}
		nums[v] = append(nums[v], "policy."+d.name)
	}
	return nums
}

var generated = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// checkLiterals reports a policy number written out in any backend package but
// policy. It leaves out tests (a test states the spec's number on its own, so
// it can disagree with the constant), generated code, and testdata.
func checkLiterals(root string, nums numbers) ([]Finding, error) {
	backend := filepath.Join(root, "backend")
	var findings []Finding
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel == policyPath || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if generated.Match(src) {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		findings = append(findings, literalsIn(fset, file, rel, nums)...)
		return nil
	})
	return findings, err
}

// ignore is the escape hatch for a number that only looks like a rule's: a
// comment on the same line or the line above, with a reason.
var ignore = regexp.MustCompile(`^//\s*policylint:ignore\s+\S`)

func literalsIn(fset *token.FileSet, file *ast.File, rel string, nums numbers) []Finding {
	ignored := map[int]bool{}
	for _, g := range file.Comments {
		for _, c := range g.List {
			if ignore.MatchString(c.Text) {
				line := fset.Position(c.Pos()).Line
				ignored[line], ignored[line+1] = true, true
			}
		}
	}

	var findings []Finding
	ast.Inspect(file, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BasicLit, *ast.BinaryExpr, *ast.CallExpr:
		default:
			return true
		}
		v, ok := eval(n.(ast.Expr), nil)
		if !ok {
			// Not a number we can read (a call, a comparison): look inside it.
			return true
		}
		if names, match := nums[v]; match {
			line := fset.Position(n.Pos()).Line
			if !ignored[line] {
				findings = append(findings, Finding{KindLiteral, rel, line, fmt.Sprintf(
					"%s repeats %s; use the constant (or //policylint:ignore <why> if it is not that rule)",
					types.ExprString(n.(ast.Expr)), strings.Join(names, ", "))})
			}
			return false
		}
		// A sum of durations that is no rule's is not made of rules either, so
		// don't flag the 5 inside `5 * time.Minute`. Plain arithmetic on
		// integers is still searched: `2 * 3` holds two literals.
		return !v.duration
	})
	return findings
}
