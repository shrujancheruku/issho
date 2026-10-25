// Package github is the small part of GitHub's REST API the admin SPA's
// editors need: read files on a branch, commit files to a branch (the
// city-pack editor's new branch per proposal; the translation editor's
// rolling branch per language), open pull requests and list open ones.
// The token is a fine-grained personal access token limited to the one
// repository, with Contents and Pull requests write access.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const defaultAPI = "https://api.github.com"

type Client struct {
	token string
	// Repo is "owner/name"; Base the branch pull requests target.
	Repo, Base string
	API        string // overridable for tests
	http       *http.Client
}

func New(token, repo string) *Client {
	return &Client{token: token, Repo: repo, Base: "main", API: defaultAPI, http: &http.Client{Timeout: 30 * time.Second}}
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

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.API+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		return &APIError{StatusCode: res.StatusCode, Message: e.Message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
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
}

// Propose commits one file to a new branch made from Base and opens a pull
// request for it. sha is the file's blob sha on Base ("" for a new file).
func (c *Client) Propose(ctx context.Context, branch, path string, content []byte, sha, message, title, body string) (PR, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/git/ref/heads/%s", c.Base), nil, &ref); err != nil {
		return PR{}, fmt.Errorf("base branch: %w", err)
	}
	if err := c.do(ctx, http.MethodPost, c.repoPath("/git/refs"), map[string]string{
		"ref": "refs/heads/" + branch, "sha": ref.Object.SHA,
	}, nil); err != nil {
		return PR{}, fmt.Errorf("branch: %w", err)
	}
	put := map[string]string{"message": message, "content": base64.StdEncoding.EncodeToString(content), "branch": branch}
	if sha != "" {
		put["sha"] = sha
	}
	if err := c.do(ctx, http.MethodPut, c.repoPath("/contents/%s", path), put, nil); err != nil {
		return PR{}, fmt.Errorf("commit: %w", err)
	}
	var pr struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := c.do(ctx, http.MethodPost, c.repoPath("/pulls"), map[string]string{
		"title": title, "head": branch, "base": c.Base, "body": body,
	}, &pr); err != nil {
		return PR{}, fmt.Errorf("pull request: %w", err)
	}
	return PR{Number: pr.Number, Title: title, URL: pr.HTMLURL, Branch: branch}, nil
}

// OpenPRs lists open pull requests whose branch starts with prefix.
func (c *Client) OpenPRs(ctx context.Context, prefix string) ([]PR, error) {
	var res []struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
	}
	if err := c.do(ctx, http.MethodGet, c.repoPath("/pulls?state=open&per_page=100"), nil, &res); err != nil {
		return nil, err
	}
	var out []PR
	for _, p := range res {
		if strings.HasPrefix(p.Head.Ref, prefix) {
			out = append(out, PR{Number: p.Number, Title: p.Title, URL: p.HTMLURL, Branch: p.Head.Ref})
		}
	}
	return out, nil
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
	return PR{Number: pr.Number, Title: title, URL: pr.HTMLURL, Branch: branch}, nil
}
