// Package github is the small part of GitHub's REST API internal/repoedit
// needs: read files on a branch, commit files to a branch, open pull
// requests, rewrite a pull request's body and list the open ones. Nothing
// but repoedit and the wiring in cmd/ imports it.
// The token is a fine-grained personal access token limited to the one
// repository, with Contents and Pull requests write access.
package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
)

const defaultAPI = "https://api.github.com"

type Client struct {
	token string
	// Repo is "owner/name"; Base the branch pull requests target.
	Repo, Base string
	API        string // overridable for tests
	api        *httpjson.Client
}

func New(token, repo string) *Client {
	return &Client{token: token, Repo: repo, Base: "main", API: defaultAPI, api: httpjson.New("github", 30*time.Second)}
}

// ErrNotFound is a 404: no such file, branch or repository.
var ErrNotFound = errors.New("not found on GitHub")

// APIError is any other non-2xx response; Message is GitHub's, which never
// contains the token.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string { return fmt.Sprintf("GitHub: %d %s", e.StatusCode, e.Message) }

// templateWords are the path segments of GitHub's REST API that are part of
// the route; every other segment (repository, branch, file path, sha) is an
// id and becomes "{}" in traces and logs.
var templateWords = map[string]bool{
	"repos": true, "contents": true, "git": true, "ref": true, "refs": true,
	"heads": true, "trees": true, "commits": true, "pulls": true,
}

func routeOf(path string) string {
	path, _, _ = strings.Cut(path, "?")
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, s := range segs {
		if !templateWords[s] {
			segs[i] = "{}"
		}
	}
	return "/" + strings.Join(segs, "/")
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	// Trees and commits are content-addressed objects: a second one is
	// harmless, so those POSTs may be resent. A second ref, pull request, file
	// commit or ref update would report a conflict for work that already
	// happened, so those are sent once.
	objectPost := method == http.MethodPost && (strings.HasSuffix(path, "/git/trees") || strings.HasSuffix(path, "/git/commits"))
	expect := []int{http.StatusNotFound}
	if method == http.MethodPatch {
		expect = append(expect, http.StatusUnprocessableEntity)
	}
	err := c.api.Do(ctx, httpjson.Request{
		Method: method, URL: c.API + path, Route: routeOf(path), Body: body, Out: out,
		Header: map[string]string{
			"Authorization":        "Bearer " + c.token,
			"Accept":               "application/vnd.github+json",
			"X-GitHub-Api-Version": "2022-11-28",
		},
		Idempotent: objectPost,
		NoRetry:    method != http.MethodGet && !objectPost,
		// A missing file or branch is an answer callers probe for; a 422 on the
		// ref update means the branch moved (ErrConflict).
		Expect: expect,
	})
	if err == nil {
		return nil
	}
	se, ok := httpjson.Status(err)
	if !ok {
		return err
	}
	if se.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal([]byte(se.Body), &e)
	if slices.Contains(expect, se.StatusCode) { // not wrapped: the caller reads it
		return &APIError{StatusCode: se.StatusCode, Message: e.Message}
	}
	// Keeps errs.Upstream's fixed client message; the cause is GitHub's.
	return errs.WithCause(err, &APIError{StatusCode: se.StatusCode, Message: e.Message})
}

func (c *Client) repoPath(format string, a ...any) string {
	return "/repos/" + c.Repo + fmt.Sprintf(format, a...)
}

// File returns a file's content and blob sha on a branch; ErrNotFound when
// it isn't there.
func (c *Client) File(ctx context.Context, path, branch string) ([]byte, string, error) {
	var res struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/contents/%s?ref=%s", path, url.QueryEscape(branch)), nil, &res); err != nil {
		return nil, "", err
	}
	if res.Encoding != "base64" {
		return nil, "", fmt.Errorf("GitHub: %s: unexpected encoding %q", path, res.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(res.Content, "\n", ""))
	return b, res.SHA, err
}

// Dir lists the names of the files in a directory on a branch.
func (c *Client) Dir(ctx context.Context, path, branch string) ([]string, error) {
	var res []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/contents/%s?ref=%s", path, url.QueryEscape(branch)), nil, &res); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range res {
		if f.Type == "file" {
			out = append(out, f.Name)
		}
	}
	return out, nil
}

// PR is an open pull request.
type PR struct {
	Number int
	Title  string
	URL    string
	Branch string
	Body   string
}

// maxPRPages bounds OpenPRs: 5,000 open pull requests is a runaway, not a
// repository this editor serves.
const maxPRPages = 50

// OpenPRs lists every open pull request whose branch starts with prefix,
// paging through all of them: other tools (Renovate) open pull requests
// too, and a single page of 100 would let the editors' fall off it.
func (c *Client) OpenPRs(ctx context.Context, prefix string) ([]PR, error) {
	var out []PR
	for page := 1; page <= maxPRPages; page++ {
		var res []struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
			Body    string `json:"body"`
			Head    struct {
				Ref string `json:"ref"`
			} `json:"head"`
		}
		if err := c.do(ctx, http.MethodGet, c.repoPath("/pulls?state=open&per_page=100&page=%d", page), nil, &res); err != nil {
			return nil, err
		}
		for _, p := range res {
			if strings.HasPrefix(p.Head.Ref, prefix) {
				out = append(out, PR{Number: p.Number, Title: p.Title, URL: p.HTMLURL, Branch: p.Head.Ref, Body: p.Body})
			}
		}
		if len(res) < 100 {
			return out, nil
		}
	}
	return out, nil
}

// UpdatePR rewrites a pull request's body.
func (c *Client) UpdatePR(ctx context.Context, number int, body string) error {
	return c.do(ctx, http.MethodPatch, c.repoPath("/pulls/%d", number), map[string]string{"body": body}, nil)
}

// ErrConflict means the branch moved while committing (someone else
// committed first); read again and retry.
var ErrConflict = errors.New("GitHub: the branch changed meanwhile")

func (c *Client) head(ctx context.Context, branch string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/git/ref/heads/%s", branch), nil, &ref); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

// CreateBranch makes branch from the current tip of Base.
func (c *Client) CreateBranch(ctx context.Context, branch string) error {
	sha, err := c.head(ctx, c.Base)
	if err != nil {
		return fmt.Errorf("base branch: %w", err)
	}
	return c.do(ctx, http.MethodPost, c.repoPath("/git/refs"), map[string]string{"ref": "refs/heads/" + branch, "sha": sha}, nil)
}

// CommitFiles commits several files to branch in one commit (the contents
// API takes one file per commit), through the Git data API: a tree on top
// of the branch's, a commit, then a fast-forward of the branch. ErrConflict
// if the branch moved in between.
func (c *Client) CommitFiles(ctx context.Context, branch string, files map[string][]byte, message string) error {
	parent, err := c.head(ctx, branch)
	if err != nil {
		return fmt.Errorf("branch: %w", err)
	}
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/git/commits/%s", parent), nil, &commit); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	type entry struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	tree := struct {
		BaseTree string  `json:"base_tree"`
		Tree     []entry `json:"tree"`
	}{BaseTree: commit.Tree.SHA}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		tree.Tree = append(tree.Tree, entry{Path: p, Mode: "100644", Type: "blob", Content: string(files[p])})
	}
	var created struct {
		SHA string `json:"sha"`
	}
	if err := c.do(ctx, http.MethodPost, c.repoPath("/git/trees"), tree, &created); err != nil {
		return fmt.Errorf("tree: %w", err)
	}
	newTree := created.SHA
	if err := c.do(ctx, http.MethodPost, c.repoPath("/git/commits"), map[string]any{
		"message": message, "tree": newTree, "parents": []string{parent},
	}, &created); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	err = c.do(ctx, http.MethodPatch, c.repoPath("/git/refs/heads/%s", branch), map[string]any{"sha": created.SHA, "force": false}, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity {
		return ErrConflict
	}
	return err
}

// OpenPR opens a pull request from branch into Base.
func (c *Client) OpenPR(ctx context.Context, branch, title, body string) (PR, error) {
	var pr struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := c.do(ctx, http.MethodPost, c.repoPath("/pulls"), map[string]string{
		"title": title, "head": branch, "base": c.Base, "body": body,
	}, &pr); err != nil {
		return PR{}, fmt.Errorf("pull request: %w", err)
	}
	return PR{Number: pr.Number, Title: title, URL: pr.HTMLURL, Branch: branch, Body: body}, nil
}
