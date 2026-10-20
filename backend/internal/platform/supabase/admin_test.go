package supabase

import (
	"context"
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
