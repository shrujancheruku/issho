package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The kinds of finding, one per way the policy rules can be broken.
const (
	KindNoLink   = "no-link"   // a constant in policy names no file under docs/spec/
	KindDeadLink = "dead-link" // a constant's link names a file that doesn't exist
	KindLiteral  = "literal"   // a policy number written out in another package
)

// Finding is one broken rule, positioned like a compiler error.
type Finding struct {
	Kind string
	File string // slash-separated, relative to the repo root
	Line int
	Msg  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Kind, f.Msg)
}

// specLink matches a path to a spec document as a comment writes it. The
// anchor after a '#' is not checked: the lint holds each number to its document,
// and a heading can be reworded without moving the rule.
var specLink = regexp.MustCompile(`docs/spec/[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*\.md`)

// policyPath is the package the numbers live in, relative to the repo root.
const policyPath = "backend/internal/policy"

// Check lints the repo at root: every constant in the policy package must link
// a document that exists, and no other backend package may write a policy
// number as a literal.
func Check(root string) ([]Finding, error) {
	nums, findings, err := checkPolicy(root)
	if err != nil {
		return nil, err
	}
	literals, err := checkLiterals(root, nums)
	if err != nil {
		return nil, err
	}
	findings = append(findings, literals...)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

// checkPolicy reads the policy package, reports its unlinked and dead-linked
// constants, and returns the numbers the rest of the backend must not repeat.
func checkPolicy(root string) (numbers, []Finding, error) {
	dir := filepath.Join(root, filepath.FromSlash(policyPath))
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", policyPath, err)
	}
	if len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("no Go package in %s", policyPath)
	}

	var findings []Finding
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			rel := relSlash(root, path)
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					links := specLink.FindAllString(commentsOf(gd, vs), -1)
					for _, name := range vs.Names {
						if name.Name == "_" {
							continue
						}
						line := fset.Position(name.Pos()).Line
						if len(links) == 0 {
							findings = append(findings, Finding{KindNoLink, rel, line,
								name.Name + " has no comment linking a file under docs/spec/"})
							continue
						}
						for _, l := range links {
							if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(l))); err != nil || !fi.Mode().IsRegular() {
								findings = append(findings, Finding{KindDeadLink, rel, line,
									name.Name + " links " + l + ", which doesn't exist"})
							}
						}
					}
				}
			}
		}
	}
	return collectNumbers(pkgs), findings, nil
}

// commentsOf is the text of every comment that documents a constant: its own
// doc and trailing comments, and the declaration's doc when the declaration is
// a lone `const X = ...`, where the parser attaches the doc to the declaration.
// A comment above a parenthesised block describes the block, so it doesn't
// count for each constant inside: each one links its own document.
func commentsOf(gd *ast.GenDecl, vs *ast.ValueSpec) string {
	var b strings.Builder
	for _, g := range []*ast.CommentGroup{vs.Doc, vs.Comment} {
		if g != nil {
			b.WriteString(g.Text())
		}
	}
	if !gd.Lparen.IsValid() && gd.Doc != nil {
		b.WriteString(gd.Doc.Text())
	}
	return b.String()
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return filepath.ToSlash(rel)
}
