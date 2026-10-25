package supabase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteUser(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"deleted", http.StatusOK, false},
		{"already gone is success", http.StatusNotFound, false},
		{"forbidden surfaces error", http.StatusForbidden, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/auth/v1/admin/users/"+id.String() {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("apikey") != "sb_secret_test" {
					t.Errorf("apikey header not sent")
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			err := NewAdmin(srv.URL, "sb_secret_test").DeleteUser(context.Background(), id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetAdmin(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		admin bool
		want  string
	}{
		{true, `{"app_metadata":{"admin":true}}`},
		// null deletes the key in Supabase, keeping the other metadata.
		{false, `{"app_metadata":{"admin":null}}`},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut || r.URL.Path != "/auth/v1/admin/users/"+id.String() {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("apikey") != "sb_secret_test" {
				t.Errorf("apikey header not sent")
			}
			if body, _ := io.ReadAll(r.Body); string(body) != tc.want {
				t.Errorf("body = %s, want %s", body, tc.want)
			}
		}))
		if err := NewAdmin(srv.URL, "sb_secret_test").SetAdmin(context.Background(), id, tc.admin); err != nil {
			t.Fatal(err)
		}
		srv.Close()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer srv.Close()
	if err := NewAdmin(srv.URL, "sb_secret_test").SetAdmin(context.Background(), id, true); err == nil {
		t.Fatal("missing user: want an error")
	}
}

func TestSetTranslatorLangs(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		langs []string
		want  string
	}{
		{[]string{"ja", "es"}, `{"app_metadata":{"translator_langs":["ja","es"]}}`},
		{nil, `{"app_metadata":{"translator_langs":null}}`},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if body, _ := io.ReadAll(r.Body); string(body) != tc.want {
				t.Errorf("body = %s, want %s", body, tc.want)
			}
		}))
		if err := NewAdmin(srv.URL, "sb_secret_test").SetTranslatorLangs(context.Background(), id, tc.langs); err != nil {
			t.Fatal(err)
		}
		srv.Close()
	}
}
