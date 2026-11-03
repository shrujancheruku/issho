// Package supabase calls Supabase's server-side admin APIs.
//
// It needs the project's secret key (sb_secret_…), which bypasses all access
// rules. The key is only ever read from the environment (SUPABASE_SECRET_KEY)
// and never logged; see docs/secrets.md.
package supabase

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
)

type Admin struct {
	baseURL   string
	secretKey string
	api       *httpjson.Client
}

func NewAdmin(supabaseURL, secretKey string) *Admin {
	return &Admin{baseURL: supabaseURL, secretKey: secretKey, api: httpjson.New("supabase", 10*time.Second)}
}

// userRoute is the trace route for the admin users endpoints (the id is not
// part of it).
const userRoute = "/auth/v1/admin/users/{id}"

// call sends one admin request. New-style secret keys go in the apikey header
// only; they are not JWTs.
func (a *Admin) call(ctx context.Context, method string, id uuid.UUID, body any) error {
	return a.api.Do(ctx, httpjson.Request{
		Method: method, URL: a.baseURL + "/auth/v1/admin/users/" + id.String(), Route: userRoute,
		Header: map[string]string{"apikey": a.secretKey}, Body: body,
	})
}

// DeleteUser removes the auth user. On Supabase, app.users references
// auth.users with ON DELETE CASCADE, so this also deletes the user's app data.
// Deleting an already-deleted user is treated as success, so retries are safe
// (and the client retries a DELETE on a reset or 5xx).
func (a *Admin) DeleteUser(ctx context.Context, id uuid.UUID) error {
	err := a.call(ctx, http.MethodDelete, id, nil)
	if se, ok := httpjson.Status(err); ok && se.StatusCode == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	return nil
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
	return a.setAppMetadata(ctx, id, "admin", value)
}

// SetTranslatorLangs sets the languages a translator edits in the admin
// SPA's translation editor (app_metadata.translator_langs, read as
// auth.Caller.TranslatorLangs); none removes the role.
func (a *Admin) SetTranslatorLangs(ctx context.Context, id uuid.UUID, langs []string) error {
	var value any // null removes the key
	if len(langs) > 0 {
		value = langs
	}
	return a.setAppMetadata(ctx, id, "translator_langs", value)
}

// setAppMetadata is a PUT of the same value: repeating it is harmless, so the
// client may retry it.
func (a *Admin) setAppMetadata(ctx context.Context, id uuid.UUID, key string, value any) error {
	if err := a.call(ctx, http.MethodPut, id, map[string]any{"app_metadata": map[string]any{key: value}}); err != nil {
		return fmt.Errorf("update auth user: %w", err)
	}
	return nil
}
