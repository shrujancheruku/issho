package repoedit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
)

const secretText = "secret-github-text-do-not-leak"

// fakeGitHub is the part of GitHub's REST API the client uses, in memory:
// branches of files, commits, and pull requests.
type fakeGitHub struct {
	mu       sync.Mutex
	files    map[string]map[string]string // branch → path → content
	tips     map[string]string            // branch → commit sha
	commits  map[string]fakeCommit        // sha → commit
	trees    map[string]map[string]string // tree sha → files
	prs      []fakePR
	log      []string // "METHOD path" per request
	moved    int      // ref updates to refuse (422) before accepting one
	failPOST string   // a path suffix whose POST answers 422 with secretText
	extra    int      // pull requests to list first, other tools' (Renovate)
}

type fakeCommit struct {
	parent, tree, message string
}

type fakePR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"html_url"`
	Head   struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

func newFake(main map[string]string) *fakeGitHub {
	return &fakeGitHub{
		files: map[string]map[string]string{"main": main}, tips: map[string]string{"main": "c0"},
		commits: map[string]fakeCommit{"c0": {tree: "t0"}}, trees: map[string]map[string]string{"t0": main},
	}
}

func (f *fakeGitHub) count(prefix string) (n int) {
	for _, l := range f.log {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, r.Method+" "+r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/repos/o/r/")
	reply := func(code int, v any) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	var in map[string]any
	if r.Body != nil && r.Method != http.MethodGet {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	if r.Method == http.MethodPost && f.failPOST != "" && strings.HasSuffix(path, f.failPOST) {
		reply(http.StatusUnprocessableEntity, map[string]string{"message": secretText})
		return
	}
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(path, "contents/"):
		c, ok := f.files[r.URL.Query().Get("ref")][strings.TrimPrefix(path, "contents/")]
		if !ok {
			reply(http.StatusNotFound, map[string]string{"message": secretText})
			return
		}
		reply(http.StatusOK, map[string]string{"content": base64.StdEncoding.EncodeToString([]byte(c)), "encoding": "base64", "sha": "blob"})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "git/ref/heads/"):
		tip, ok := f.tips[strings.TrimPrefix(path, "git/ref/heads/")]
		if !ok {
			reply(http.StatusNotFound, nil)
			return
		}
		reply(http.StatusOK, map[string]any{"object": map[string]string{"sha": tip}})
	case r.Method == http.MethodPost && path == "git/refs":
		branch := strings.TrimPrefix(in["ref"].(string), "refs/heads/")
		src := ""
		for b, tip := range f.tips {
			if tip == in["sha"] {
				src = b
			}
		}
		f.tips[branch], f.files[branch] = in["sha"].(string), map[string]string{}
		for p, c := range f.files[src] {
			f.files[branch][p] = c
		}
		reply(http.StatusCreated, map[string]string{})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "git/commits/"):
		reply(http.StatusOK, map[string]any{"tree": map[string]string{"sha": f.commits[strings.TrimPrefix(path, "git/commits/")].tree}})
	case r.Method == http.MethodPost && path == "git/trees":
		tree := map[string]string{}
		for p, c := range f.trees[in["base_tree"].(string)] {
			tree[p] = c
		}
		for _, e := range in["tree"].([]any) {
			e := e.(map[string]any)
			tree[e["path"].(string)] = e["content"].(string)
		}
		sha := fmt.Sprintf("t%d", len(f.trees))
		f.trees[sha] = tree
		reply(http.StatusCreated, map[string]string{"sha": sha})
	case r.Method == http.MethodPost && path == "git/commits":
		sha := fmt.Sprintf("c%d", len(f.commits))
		f.commits[sha] = fakeCommit{parent: in["parents"].([]any)[0].(string), tree: in["tree"].(string), message: in["message"].(string)}
		reply(http.StatusCreated, map[string]string{"sha": sha})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "git/refs/heads/"):
		branch := strings.TrimPrefix(path, "git/refs/heads/")
		if f.moved > 0 {
			f.moved--
			reply(http.StatusUnprocessableEntity, map[string]string{"message": secretText})
			return
		}
		sha := in["sha"].(string)
		f.tips[branch], f.files[branch] = sha, f.trees[f.commits[sha].tree]
		reply(http.StatusOK, map[string]string{})
	case r.Method == http.MethodGet && path == "pulls":
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		all := make([]fakePR, 0, f.extra+len(f.prs))
		for i := range f.extra {
			p := fakePR{Number: 1000 + i, Title: "chore(deps): bump", URL: "u"}
			p.Head.Ref = fmt.Sprintf("renovate/dep-%d", i)
			all = append(all, p)
		}
		all = append(all, f.prs...)
		lo, hi := (page-1)*100, min(page*100, len(all))
		if lo > len(all) {
			lo = len(all)
		}
		reply(http.StatusOK, all[lo:hi])
	case r.Method == http.MethodPost && path == "pulls":
		p := fakePR{Number: 100 + len(f.prs), Title: in["title"].(string), Body: in["body"].(string), URL: fmt.Sprintf("https://github.test/pull/%d", 100+len(f.prs))}
		p.Head.Ref = in["head"].(string)
		f.prs = append(f.prs, p)
		reply(http.StatusCreated, p)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "pulls/"):
		n, _ := strconv.Atoi(strings.TrimPrefix(path, "pulls/"))
		for i := range f.prs {
			if f.prs[i].Number == n {
				f.prs[i].Body = in["body"].(string)
			}
		}
		reply(http.StatusOK, map[string]string{})
	default:
		reply(http.StatusTeapot, map[string]string{"message": "unexpected " + r.Method + " " + path})
	}
}

func setup(t *testing.T, main map[string]string) (*Repo, *fakeGitHub) {
	t.Helper()
	f := newFake(main)
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	gh := github.New("tok", "o/r")
	gh.API = srv.URL
	r := New(gh, "main")
	r.Now = func() time.Time { return time.Unix(1700000000, 0) }
	return r, f
}

var (
	ctx   = context.Background()
	aPath = "data/a.json"
	bPath = "data/b.json"
)

// set returns an edit that sets a file to content.
func set(path, content, body string) func(map[string][]byte) (bool, string, error) {
	return func(files map[string][]byte) (bool, string, error) {
		if string(files[path]) == content {
			return false, "", nil
		}
		files[path] = []byte(content)
		return true, body, nil
	}
}

func edit(key string) Edit {
	return Edit{Kind: "things", Key: key, Paths: []string{aPath, bPath}, Message: "feat(data): set " + key, Title: "feat(data): update " + key}
}

func TestChange(t *testing.T) {
	r, f := setup(t, map[string]string{aPath: "old"})

	// Nothing changed and no pull request: no branch, no commit, no PR.
	if _, ok, err := r.Change(ctx, edit("ja"), set(aPath, "old", "")); err != nil || ok {
		t.Fatalf("no-op: %v %v", ok, err)
	}
	if f.count("POST") != 0 {
		t.Fatalf("a no-op wrote: %v", f.log)
	}

	// The first change opens a branch off main and a pull request; every
	// changed file goes in one commit.
	edits := func(files map[string][]byte) (bool, string, error) {
		files[aPath], files[bPath] = []byte("new"), []byte("copy")
		return true, "body 1", nil
	}
	pr, ok, err := r.Change(ctx, edit("ja"), edits)
	if err != nil || !ok || pr.Number != 100 || pr.Title != "feat(data): update ja" || pr.URL != "https://github.test/pull/100" {
		t.Fatalf("first change: %+v %v %v", pr, ok, err)
	}
	if len(f.prs) != 1 || f.prs[0].Head.Ref != "things/ja-1700000000" || f.prs[0].Body != "body 1" {
		t.Fatalf("prs: %+v", f.prs)
	}
	br := f.files["things/ja-1700000000"]
	if br[aPath] != "new" || br[bPath] != "copy" || f.count("POST /repos/o/r/git/commits") != 1 {
		t.Fatalf("branch %v, log %v", br, f.log)
	}
	if f.files["main"][aPath] != "old" {
		t.Error("main moved")
	}

	// A second change commits on the same branch, rewriting the body.
	pr, ok, err = r.Change(ctx, edit("ja"), set(aPath, "newer", "body 2"))
	if err != nil || !ok || pr.Number != 100 {
		t.Fatalf("second change: %+v %v %v", pr, ok, err)
	}
	if len(f.prs) != 1 || f.count("POST /repos/o/r/pulls") != 1 || f.count("POST /repos/o/r/git/refs") != 1 {
		t.Errorf("a second pull request or branch: %v", f.log)
	}
	if f.files["things/ja-1700000000"][aPath] != "newer" || f.files["things/ja-1700000000"][bPath] != "copy" || f.prs[0].Body != "body 2" {
		t.Errorf("second commit: %v body %q", f.files["things/ja-1700000000"], f.prs[0].Body)
	}

	// Nothing changed: no commit, and the open pull request is returned.
	commits := f.count("POST /repos/o/r/git/commits")
	pr, ok, err = r.Change(ctx, edit("ja"), set(aPath, "newer", "body 3"))
	if err != nil || !ok || pr.Number != 100 || f.count("POST /repos/o/r/git/commits") != commits || f.prs[0].Body != "body 2" {
		t.Errorf("no-op on an open change: %+v %v %v", pr, ok, err)
	}

	// An edit that fails commits nothing.
	boom := errors.New("boom")
	if _, _, err := r.Change(ctx, edit("ja"), func(map[string][]byte) (bool, string, error) { return false, "", boom }); !errors.Is(err, boom) {
		t.Errorf("edit error: %v", err)
	}

	// Another key is another pull request.
	if pr, _, err := r.Change(ctx, edit("fr"), set(aPath, "fr", "b")); err != nil || pr.Number != 101 {
		t.Errorf("another key: %+v %v", pr, err)
	}

	// Files reads the open pull request's version, else main's.
	files, open, err := r.Files(ctx, "things", "ja", []string{aPath, "missing"})
	if err != nil || open == nil || open.Number != 100 || string(files[aPath]) != "newer" || len(files) != 1 {
		t.Errorf("Files with a pull request: %v %+v %v", files, open, err)
	}
	files, open, err = r.Files(ctx, "things", "de", []string{aPath})
	if err != nil || open != nil || string(files[aPath]) != "old" {
		t.Errorf("Files without: %v %+v %v", files, open, err)
	}
}

func TestBranchMoved(t *testing.T) {
	r, f := setup(t, map[string]string{aPath: "old"})
	if _, _, err := r.Change(ctx, edit("ja"), set(aPath, "one", "b")); err != nil {
		t.Fatal(err)
	}

	// Moved once: read again and commit.
	f.moved = 1
	edits := 0
	pr, _, err := r.Change(ctx, edit("ja"), func(files map[string][]byte) (bool, string, error) {
		edits++
		return set(aPath, "two", "b")(files)
	})
	if err != nil || pr.Number != 100 || edits != 2 || f.files["things/ja-1700000000"][aPath] != "two" {
		t.Fatalf("retry: %+v %v (edit ran %d times)", pr, err, edits)
	}

	// Moved twice: the retry fails too, and GitHub's words stay out.
	f.moved = 2
	_, _, err = r.Change(ctx, edit("ja"), set(aPath, "three", "b"))
	var e *errs.Error
	if !errors.As(err, &e) || e.Kind != errs.KindUpstream || strings.Contains(err.Error(), secretText) {
		t.Fatalf("a moved branch twice: %v", err)
	}
	if f.files["things/ja-1700000000"][aPath] != "two" {
		t.Error("a failed save changed the branch")
	}
}

func TestPendingPagesThroughEveryPullRequest(t *testing.T) {
	r, f := setup(t, map[string]string{aPath: "old"})
	// Other tools' pull requests first, then more than a page of them in all:
	// the editors' must still be found.
	f.extra = 230
	for _, b := range []string{"things/zh-Hant-5", "things/ja-3", "things/ja-9", "things/bad", "things/x-notanumber", "other/ja-1"} {
		p := fakePR{Number: 300 + len(f.prs), URL: "u"}
		p.Head.Ref = b
		if b == "things/ja-3" {
			p.Number = 7
		}
		f.prs = append(f.prs, p)
	}
	f.files["things/zh-Hant-5"], f.tips["things/zh-Hant-5"] = map[string]string{aPath: "old"}, "c0"
	got, err := r.Pending(ctx, "things")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["zh-Hant"].Number != 300 || got["ja"].Number != 302 {
		t.Errorf("Pending = %+v (the newest pull request wins; others' ignored)", got)
	}
	if f.count("GET /repos/o/r/pulls") != 3 {
		t.Errorf("pages read: %d, want 3", f.count("GET /repos/o/r/pulls"))
	}
	// And Change finds an existing pull request past the first page.
	if pr, ok, err := r.Change(ctx, edit("zh-Hant"), set(aPath, "x", "b")); err != nil || !ok || pr.Number != 300 || f.count("POST /repos/o/r/pulls") != 0 {
		t.Errorf("Change past page one: %+v %v %v", pr, ok, err)
	}
}

func TestGitHubTextNeverReachesTheCaller(t *testing.T) {
	r, f := setup(t, map[string]string{aPath: "old"})
	check := func(what string, err error) {
		t.Helper()
		var e *errs.Error
		if !errors.As(err, &e) || e.Kind != errs.KindUpstream {
			t.Errorf("%s: %v, want an upstream error", what, err)
			return
		}
		if strings.Contains(err.Error(), secretText) || strings.Contains(err.Error(), "422") {
			t.Errorf("%s leaked GitHub's text: %q", what, err)
		}
	}
	f.failPOST = "git/refs"
	_, _, err := r.Change(ctx, edit("ja"), set(aPath, "new", "b"))
	check("branching", err)
	f.failPOST = "pulls"
	_, _, err = r.Change(ctx, edit("ja"), set(aPath, "new", "b"))
	check("opening the pull request", err)
	f.failPOST = ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"` + secretText + `"}`))
	}))
	defer srv.Close()
	gh := github.New("bad", "o/r")
	gh.API = srv.URL
	bad := New(gh, "main")
	_, _, err = bad.Read(ctx, aPath)
	check("reading", err)
	if !TokenRefused(err) {
		t.Error("a 401 is a refused token")
	}
	_, err = bad.Pending(ctx, "things")
	check("listing", err)
}

func TestRead(t *testing.T) {
	r, _ := setup(t, map[string]string{aPath: "old"})
	if b, ok, err := r.Read(ctx, aPath); err != nil || !ok || string(b) != "old" {
		t.Errorf("Read = %q %v %v", b, ok, err)
	}
	if _, ok, err := r.Read(ctx, "nope.json"); err != nil || ok {
		t.Errorf("Read of a missing file = %v %v", ok, err)
	}
}

func TestTokenRefused(t *testing.T) {
	for _, tc := range []struct {
		code int
		msg  string
		want bool
	}{
		{http.StatusUnauthorized, "Bad credentials", true},
		{http.StatusForbidden, "Resource not accessible by personal access token", true},
		{http.StatusForbidden, "API rate limit exceeded", false},
		{http.StatusForbidden, "You have exceeded a secondary rate limit", false},
		{http.StatusNotFound, "x", false},
	} {
		err := fmt.Errorf("list: %w", &github.APIError{StatusCode: tc.code, Message: tc.msg})
		if got := TokenRefused(err); got != tc.want {
			t.Errorf("%d %q: %v", tc.code, tc.msg, got)
		}
	}
	if TokenRefused(errors.New("down")) {
		t.Error("an outage is not a bad token")
	}
}

// Without GITHUB_TOKEN there is no Repo: nothing is pending and every
// edit is refused.
func TestNotConfigured(t *testing.T) {
	var r *Repo
	if got, err := r.Pending(ctx, "things"); err != nil || len(got) != 0 {
		t.Errorf("Pending = %v %v", got, err)
	}
	refused := func(err error) bool {
		var e *errs.Error
		return errors.As(err, &e) && e.Kind == errs.KindPrecondition && strings.Contains(err.Error(), "GITHUB_TOKEN")
	}
	if _, _, err := r.Change(ctx, edit("ja"), set(aPath, "x", "")); !refused(err) {
		t.Errorf("Change: %v", err)
	}
	if _, _, err := r.Files(ctx, "things", "ja", nil); !refused(err) {
		t.Errorf("Files: %v", err)
	}
	if _, _, err := r.Read(ctx, aPath); !refused(err) {
		t.Errorf("Read: %v", err)
	}
	if _, err := r.List(ctx, "data"); !refused(err) {
		t.Errorf("List: %v", err)
	}
}
