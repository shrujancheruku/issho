package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProposeAndRead(t *testing.T) {
	var calls []string
	var put map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/o/r/contents/data/citypacks/tokyo.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"content": base64.StdEncoding.EncodeToString([]byte("{\"id\":\"tokyo\"}"))[:8] + "\n" + base64.StdEncoding.EncodeToString([]byte("{\"id\":\"tokyo\"}"))[8:], "encoding": "base64", "sha": "blob1"})
		case "GET /repos/o/r/contents/data/citypacks/nowhere.json":
			w.WriteHeader(http.StatusNotFound)
		case "GET /repos/o/r/contents/data/citypacks":
			_, _ = w.Write([]byte(`[{"name":"tokyo.json","type":"file"},{"name":"old","type":"dir"}]`))
		case "GET /repos/o/r/git/ref/heads/main":
			_, _ = w.Write([]byte(`{"object":{"sha":"c0ffee"}}`))
		case "POST /repos/o/r/git/refs":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["sha"] != "c0ffee" {
				t.Errorf("ref %v", b)
			}
			w.WriteHeader(http.StatusCreated)
		case "PUT /repos/o/r/contents/data/citypacks/tokyo.json":
			_ = json.NewDecoder(r.Body).Decode(&put)
			w.WriteHeader(http.StatusOK)
		case "POST /repos/o/r/pulls":
			_, _ = w.Write([]byte(`{"number":7,"html_url":"https://github.com/o/r/pull/7"}`))
		case "GET /repos/o/r/pulls":
			_, _ = w.Write([]byte(`[{"number":7,"title":"t","html_url":"u","head":{"ref":"citypack/tokyo-1"}},{"number":8,"title":"x","html_url":"v","head":{"ref":"feat/x"}}]`))
		default:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		}
	}))
	defer srv.Close()
	c := New("tok", "o/r")
	c.API = srv.URL
	ctx := context.Background()

	b, sha, err := c.File(ctx, "data/citypacks/tokyo.json", "main")
	if err != nil || string(b) != `{"id":"tokyo"}` || sha != "blob1" {
		t.Fatalf("File = %q %q %v", b, sha, err)
	}
	if _, _, err := c.File(ctx, "data/citypacks/nowhere.json", "main"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing file: %v", err)
	}
	if names, err := c.Dir(ctx, "data/citypacks", "main"); err != nil || len(names) != 1 || names[0] != "tokyo.json" {
		t.Errorf("Dir = %v %v", names, err)
	}
	pr, err := c.Propose(ctx, "citypack/tokyo-1", "data/citypacks/tokyo.json", []byte("new"), "blob1", "feat(data): x", "title", "body")
	if err != nil || pr.Number != 7 || pr.URL != "https://github.com/o/r/pull/7" {
		t.Fatalf("Propose = %+v %v", pr, err)
	}
	if put["branch"] != "citypack/tokyo-1" || put["sha"] != "blob1" || put["content"] != base64.StdEncoding.EncodeToString([]byte("new")) {
		t.Errorf("put = %v", put)
	}
	prs, err := c.OpenPRs(ctx, "citypack/")
	if err != nil || len(prs) != 1 || prs[0].Number != 7 {
		t.Errorf("OpenPRs = %v %v", prs, err)
	}
	var apiErr *APIError
	if _, err := c.Propose(ctx, "x", "other.json", nil, "", "m", "t", "b"); !errors.As(err, &apiErr) || apiErr.Message != "nope" {
		t.Errorf("a failing commit: %v", err)
	}
}

func TestCommitFiles(t *testing.T) {
	var tree struct {
		BaseTree string `json:"base_tree"`
		Tree     []struct {
			Path, Mode, Type, Content string
		} `json:"tree"`
	}
	var commit struct {
		Message string   `json:"message"`
		Tree    string   `json:"tree"`
		Parents []string `json:"parents"`
	}
	var ref map[string]any
	moved := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/o/r/git/ref/heads/translations/ja-1":
			_, _ = w.Write([]byte(`{"object":{"sha":"tip"}}`))
		case "GET /repos/o/r/git/commits/tip":
			_, _ = w.Write([]byte(`{"tree":{"sha":"tree0"}}`))
		case "POST /repos/o/r/git/trees":
			_ = json.NewDecoder(r.Body).Decode(&tree)
			_, _ = w.Write([]byte(`{"sha":"tree1"}`))
		case "POST /repos/o/r/git/commits":
			_ = json.NewDecoder(r.Body).Decode(&commit)
			_, _ = w.Write([]byte(`{"sha":"commit1"}`))
		case "PATCH /repos/o/r/git/refs/heads/translations/ja-1":
			_ = json.NewDecoder(r.Body).Decode(&ref)
			if moved {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message":"Update is not a fast forward"}`))
			}
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer srv.Close()
	c := New("tok", "o/r")
	c.API = srv.URL
	files := map[string][]byte{"b/ja.json": []byte("{}\n"), "a/ja.json": []byte("{}\n")}
	if err := c.CommitFiles(context.Background(), "translations/ja-1", files, "feat(data): x"); err != nil {
		t.Fatal(err)
	}
	if tree.BaseTree != "tree0" || len(tree.Tree) != 2 || tree.Tree[0].Path != "a/ja.json" || tree.Tree[0].Mode != "100644" || tree.Tree[1].Content != "{}\n" {
		t.Errorf("tree %+v", tree)
	}
	if commit.Tree != "tree1" || len(commit.Parents) != 1 || commit.Parents[0] != "tip" || commit.Message != "feat(data): x" {
		t.Errorf("commit %+v", commit)
	}
	if ref["sha"] != "commit1" || ref["force"] != false {
		t.Errorf("ref %+v", ref)
	}
	moved = true
	if err := c.CommitFiles(context.Background(), "translations/ja-1", files, "x"); !errors.Is(err, ErrConflict) {
		t.Errorf("moved branch: %v", err)
	}
}
