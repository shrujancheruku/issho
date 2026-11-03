// Package repoedit owns pending changes (GLOSSARY.md): edits saved in the
// admin SPA that wait in one open pull request per kind and key, such as
// (translations, ja) or (citypack, tokyo). The city-pack editor and the
// translation editor both save through Change.
//
// The interface is five methods. Callers never see branches, SHAs or which
// GitHub API is used: Change locks per key, reads the files from the key's
// open pull request (or main without one), branches off only when something
// changed, commits every changed file in one commit, retries once if the
// branch moved, and opens the pull request or rewrites its body.
//
// Auth is the handlers' job: repoedit and the services above it trust their
// callers. Failures are errs kinds; GitHub's own text stays in the logs.
package repoedit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
)

// Host is the GitHub repository (*github.Client). It is repoedit's one
// adapter seam: a test can stand in a fake at this level, but repoedit's
// own tests run the real client against an httptest server instead.
type Host interface {
	File(ctx context.Context, path, branch string) ([]byte, string, error)
	Dir(ctx context.Context, path, branch string) ([]string, error)
	OpenPRs(ctx context.Context, prefix string) ([]github.PR, error)
	CreateBranch(ctx context.Context, branch string) error
	CommitFiles(ctx context.Context, branch string, files map[string][]byte, message string) error
	OpenPR(ctx context.Context, branch, title, body string) (github.PR, error)
	UpdatePR(ctx context.Context, number int, body string) error
}

// PR is a pending change's open pull request.
type PR struct {
	Number int
	Title  string
	URL    string
}

// Repo is the repository's pending changes. The nil *Repo is the editors
// without GITHUB_TOKEN: Pending is empty and everything else is refused.
type Repo struct {
	host Host
	base string
	// Now names new branches (<kind>/<key>-<unix time>); nil = time.Now.
	Now func() time.Time

	mu    sync.Mutex // guards locks
	locks map[string]*sync.Mutex
}

// New returns the pending changes of host, whose default branch is base.
func New(host Host, base string) *Repo {
	return &Repo{host: host, base: base, locks: map[string]*sync.Mutex{}}
}

// Edit says which pending change Change edits and how it is worded.
type Edit struct {
	// Kind and Key name the change: one open pull request each. Branches
	// are <Kind>/<Key>-<unix time>, so Kind has no "/" and Key may hold "-".
	Kind, Key string
	// Paths are the files edit reads and may write.
	Paths []string
	// Message is the commit message; Title the pull request's, when Change
	// opens it (an open one keeps its title).
	Message, Title string
}

func notConfigured() error {
	return errs.Refused("saving edits needs GITHUB_TOKEN on the API")
}

// upstream keeps GitHub's text off the wire (logs and Sentry only).
func upstream(err error) error { return errs.Upstream("github", err) }

// TokenRefused reports whether err means GitHub refused the token: a 401,
// or a 403 that is not a rate limit (GitHub answers its primary and
// secondary limits with 403 too, which says nothing about the token).
func TokenRefused(err error) bool {
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusUnauthorized ||
		(apiErr.StatusCode == http.StatusForbidden && !strings.Contains(strings.ToLower(apiErr.Message), "rate limit"))
}

func (r *Repo) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func prOf(p github.PR) PR { return PR{Number: p.Number, Title: p.Title, URL: p.URL} }

// Read returns a file on main; false when it isn't there.
func (r *Repo) Read(ctx context.Context, path string) ([]byte, bool, error) {
	if r == nil {
		return nil, false, notConfigured()
	}
	b, _, err := r.host.File(ctx, path, r.base)
	if errors.Is(err, github.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, upstream(err)
	}
	return b, true, nil
}

// List names the files in a directory on main; none when it isn't there.
// (Beyond the four methods of the design: city packs and the followed human
// versions both need to know which files exist.)
func (r *Repo) List(ctx context.Context, dir string) ([]string, error) {
	if r == nil {
		return nil, notConfigured()
	}
	names, err := r.host.Dir(ctx, dir, r.base)
	if errors.Is(err, github.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, upstream(err)
	}
	return names, nil
}

// keyOf splits a branch <kind>/<key>-<unix time>; the key may contain a dash
// (zh-Hant, zz-packs).
func keyOf(kind, branch string) (string, bool) {
	rest, ok := strings.CutPrefix(branch, kind+"/")
	if !ok {
		return "", false
	}
	i := strings.LastIndex(rest, "-")
	if i <= 0 {
		return "", false
	}
	if _, err := strconv.ParseInt(rest[i+1:], 10, 64); err != nil {
		return "", false
	}
	return rest[:i], true
}

// open is a key's open pull request with the branch it was opened from.
type open struct {
	PR
	branch, body string
}

func (r *Repo) opens(ctx context.Context, kind string) (map[string]open, error) {
	prs, err := r.host.OpenPRs(ctx, kind+"/")
	if err != nil {
		return nil, upstream(err)
	}
	out := map[string]open{}
	for _, p := range prs {
		key, ok := keyOf(kind, p.Branch)
		if !ok {
			continue
		}
		// Older pull requests of a key (from before there was one rolling
		// pull request per key) lose to the newest.
		if cur, seen := out[key]; !seen || p.Number > cur.Number {
			out[key] = open{PR: prOf(p), branch: p.Branch, body: p.Body}
		}
	}
	return out, nil
}

// Pending maps each key of kind to its open pull request.
func (r *Repo) Pending(ctx context.Context, kind string) (map[string]PR, error) {
	out := map[string]PR{}
	if r == nil {
		return out, nil
	}
	opens, err := r.opens(ctx, kind)
	if err != nil {
		return nil, err
	}
	for k, o := range opens {
		out[k] = o.PR
	}
	return out, nil
}

// read gets paths from ref; files that aren't there are left out.
func (r *Repo) read(ctx context.Context, paths []string, ref string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, p := range paths {
		b, _, err := r.host.File(ctx, p, ref)
		if errors.Is(err, github.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, upstream(err)
		}
		files[p] = b
	}
	return files, nil
}

// Files returns paths as they stand in key's pending change, else on main,
// and that pull request (nil without one). Files that don't exist are left
// out of the map.
func (r *Repo) Files(ctx context.Context, kind, key string, paths []string) (map[string][]byte, *PR, error) {
	if r == nil {
		return nil, nil, notConfigured()
	}
	opens, err := r.opens(ctx, kind)
	if err != nil {
		return nil, nil, err
	}
	o, ok := opens[key]
	ref := r.base
	if ok {
		ref = o.branch
	}
	files, err := r.read(ctx, paths, ref)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return files, nil, nil
	}
	return files, &o.PR, nil
}

func (r *Repo) lock(kind, key string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := kind + "/" + key
	if r.locks[id] == nil {
		r.locks[id] = &sync.Mutex{}
	}
	return r.locks[id]
}

// Change applies edit to the files of key's pending change. edit gets the
// files as they stand in the open pull request (main without one) and sets
// the new content of any it changes in the map. It returns changed=false for
// nothing to do, else the pull request's body. Change commits the files that
// differ, opens the pull request when there was none, and rewrites an open
// one's body when it differs.
//
// The result is the pending change afterwards and whether there is one:
// false only when nothing changed and none is open. edit may call Files,
// Read and List but must not call Change for the same key: Change holds the
// key's lock, so that would deadlock. A branch that moves
// between the read and the commit is read again once; a second move fails.
func (r *Repo) Change(ctx context.Context, e Edit, edit func(files map[string][]byte) (changed bool, body string, err error)) (PR, bool, error) {
	if r == nil {
		return PR{}, false, notConfigured()
	}
	// One save at a time per key: two would race to open its pull request,
	// or to commit on top of the same tip.
	mu := r.lock(e.Kind, e.Key)
	mu.Lock()
	defer mu.Unlock()
	for attempt := 0; ; attempt++ {
		opens, err := r.opens(ctx, e.Kind)
		if err != nil {
			return PR{}, false, err
		}
		o, isOpen := opens[e.Key]
		// Without a pull request, start from main and branch off only once
		// there is something to commit.
		ref := r.base
		if isOpen {
			ref = o.branch
		}
		files, err := r.read(ctx, e.Paths, ref)
		if err != nil {
			return PR{}, false, err
		}
		before := map[string][]byte{}
		for p, b := range files {
			before[p] = bytes.Clone(b)
		}
		changed, body, err := edit(files)
		if err != nil {
			return PR{}, false, err
		}
		var out []string
		for _, p := range e.Paths {
			if b, ok := files[p]; ok && !bytes.Equal(b, before[p]) {
				out = append(out, p)
			}
		}
		if !changed || len(out) == 0 {
			return o.PR, isOpen, nil
		}
		branch := o.branch
		if !isOpen {
			branch = fmt.Sprintf("%s/%s-%d", e.Kind, e.Key, r.now().Unix())
			if err := r.host.CreateBranch(ctx, branch); err != nil {
				return PR{}, false, upstream(err)
			}
		}
		commit := map[string][]byte{}
		for _, p := range slices.Sorted(slices.Values(out)) {
			commit[p] = files[p]
		}
		err = r.host.CommitFiles(ctx, branch, commit, e.Message)
		if errors.Is(err, github.ErrConflict) {
			if attempt == 0 {
				continue
			}
			return PR{}, false, errs.UpstreamMsg("github", "the pull request changed while saving; try again", err)
		}
		if err != nil {
			return PR{}, false, upstream(err)
		}
		if !isOpen {
			pr, err := r.host.OpenPR(ctx, branch, e.Title, body)
			if err != nil {
				return PR{}, false, upstream(err)
			}
			return prOf(pr), true, nil
		}
		if body != "" && body != o.body {
			if err := r.host.UpdatePR(ctx, o.Number, body); err != nil {
				return PR{}, false, upstream(err)
			}
		}
		return o.PR, true, nil
	}
}
