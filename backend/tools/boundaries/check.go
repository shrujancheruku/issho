package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The kinds of finding, one per way a seam can be crossed.
const (
	KindInternal       = "internal"        // a package imports another's non-exported parts
	KindOutsideService = "outside-service" // something but cmd/ imports a real outside-service implementation
)

// outsideServiceMarker, on a line of its own in a package's source, declares the
// package a real implementation of an outside service (the Resend sender, a
// push provider, the Places client). Only cmd/ may import such a package.
const outsideServiceMarker = "//boundaries:outside-service"

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

// file is one parsed source file of the backend.
type file struct {
	rel     string // slash-separated path from the repo root
	pkgDir  string // its directory relative to the backend module, e.g. "internal/email"
	imports []imp
	marked  bool // carries outsideServiceMarker
}

type imp struct {
	path string
	line int
}

// Check lints the backend module under root: a package under internal/ may be
// imported from outside its own tree only at its top (internal/email, not
// internal/email/resend), and a package marked as a real outside-service
// implementation only from cmd/. cmd/ is where the process is wired, so it may
// import anything.
func Check(root string) ([]Finding, error) {
	module, err := modulePath(filepath.Join(root, "backend", "go.mod"))
	if err != nil {
		return nil, err
	}
	files, err := parseBackend(root)
	if err != nil {
		return nil, err
	}

	marked := map[string]bool{} // module-relative package dir -> real implementation
	for _, f := range files {
		if f.marked {
			marked[f.pkgDir] = true
		}
	}

	var findings []Finding
	for _, f := range files {
		if f.pkgDir == "cmd" || strings.HasPrefix(f.pkgDir, "cmd/") {
			continue
		}
		for _, im := range f.imports {
			target, ok := strings.CutPrefix(im.path, module+"/")
			if !ok || !strings.HasPrefix(target, "internal/") {
				continue
			}
			switch {
			case marked[target]:
				// Not even the seam's own package may import it: the interface
				// package would then depend on one provider.
				if f.pkgDir != target {
					findings = append(findings, Finding{KindOutsideService, f.rel, im.line, fmt.Sprintf(
						"%s is a real outside-service implementation; only cmd/ may import it (take the seam's interface instead)", target)})
				}
			case internalPart(target, f.pkgDir):
				findings = append(findings, Finding{KindInternal, f.rel, im.line, fmt.Sprintf(
					"%s is part of %s's internals; import %s and use its exported API", target, seamRoot(target), seamRoot(target))})
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

// seamRoot is the top package of a path under internal/: internal/email for
// internal/email/resend.
func seamRoot(target string) string {
	parts := strings.SplitN(target, "/", 3)
	return path.Join(parts[0], parts[1])
}

// internalPart reports whether target is below its seam's root and the
// importer is outside that root's tree.
func internalPart(target, importerDir string) bool {
	root := seamRoot(target)
	if target == root {
		return false
	}
	return importerDir != root && !strings.HasPrefix(importerDir, root+"/")
}

// modulePath reads the module line of a go.mod.
func modulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(m), nil
		}
	}
	return "", fmt.Errorf("no module line in %s", goMod)
}

// parseBackend reads the imports of every Go file in backend/, tests included:
// a test that reaches into another package's internals leaves the same mess.
// Generated code and testdata (which Go never builds) are left out.
func parseBackend(root string) ([]file, error) {
	var files []file
	base := filepath.Join(root, "backend")
	fset := token.NewFileSet()
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != base && (name == "testdata" || name == "gen" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		af, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		relBackend, _ := filepath.Rel(base, filepath.Dir(p))
		rel, _ := filepath.Rel(root, p)
		f := file{rel: filepath.ToSlash(rel), pkgDir: path.Clean(filepath.ToSlash(relBackend))}
		// A marker in a test file doesn't make its package a real implementation.
		if !strings.HasSuffix(name, "_test.go") {
			for _, cg := range af.Comments {
				for _, c := range cg.List {
					if strings.TrimSpace(c.Text) == outsideServiceMarker {
						f.marked = true
					}
				}
			}
		}
		for _, is := range af.Imports {
			ip, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			f.imports = append(f.imports, imp{ip, fset.Position(is.Pos()).Line})
		}
		files = append(files, f)
		return nil
	})
	return files, err
}
