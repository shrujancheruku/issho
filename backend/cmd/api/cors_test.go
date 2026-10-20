package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})
	tests := []struct {
		name    string
		origins []string
		origin  string
		allowed bool
	}{
		{"no origins configured denies all", nil, "https://evil.example", false},
		{"listed origin allowed", []string{"https://admin.issho.app"}, "https://admin.issho.app", true},
		{"unlisted origin denied", []string{"https://admin.issho.app"}, "https://evil.example", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/issho.v1.HealthService/Check", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			rec := httptest.NewRecorder()
			withCORS(ok, tt.origins).ServeHTTP(rec, req)
			got := rec.Header().Get("Access-Control-Allow-Origin") == tt.origin
			if got != tt.allowed {
				t.Fatalf("allowed = %v, want %v (headers %v)", got, tt.allowed, rec.Header())
			}
		})
	}
}
