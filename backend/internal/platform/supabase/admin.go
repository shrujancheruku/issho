// Package supabase calls Supabase's server-side admin APIs.
//
// It needs the project's secret key (sb_secret_…), which bypasses all access
// rules. The key is only ever read from the environment (SUPABASE_SECRET_KEY)
// and never logged; see docs/secrets.md.
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Admin struct {
	baseURL   string
	secretKey string
	http      *http.Client
}

func NewAdmin(supabaseURL, secretKey string) *Admin {
	return &Admin{baseURL: supabaseURL, secretKey: secretKey, http: &http.Client{Timeout: 10 * time.Second}}
}

// DeleteUser removes the auth user. On Supabase, app.users references
// auth.users with ON DELETE CASCADE, so this also deletes the user's app data.
// Deleting an already-deleted user is treated as success, so retries are safe.
func (a *Admin) DeleteUser(ctx context.Context, id uuid.UUID) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, a.baseURL+"/auth/v1/admin/users/"+id.String(), nil)
	if err != nil {
		return err
	}
	// New-style secret keys go in the apikey header only; they are not JWTs.
	req.Header.Set("apikey", a.secretKey)
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	// The response body is safe to surface: it never echoes the key.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("delete auth user: status %d: %s", resp.StatusCode, body)
}

// SetAdmin grants or removes the admin role (app_metadata.admin), which
// the API reads from access tokens (auth.Caller.Admin). Supabase merges
// app_metadata keys and deletes those set to null, so the user's other
// metadata (sign-in provider) is kept. The change reaches the admin SPA
// when its token is next refreshed.
func (a *Admin) SetAdmin(ctx context.Context, id uuid.UUID, admin bool) error {
	var value any // null removes the key
	if admin {
		value = true
	}
	body, err := json.Marshal(map[string]any{"app_metadata": map[string]any{"admin": value}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, a.baseURL+"/auth/v1/admin/users/"+id.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", a.secretKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("update auth user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("update auth user: status %d: %s", resp.StatusCode, msg)
}
