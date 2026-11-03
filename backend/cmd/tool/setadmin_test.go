package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// addAuthUser inserts a Supabase account (set-admin and set-translator find
// people in auth.users by email) and removes it when the test ends.
func addAuthUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := pool.Exec(ctx, "insert into auth.users (id, email) values ($1, $2)", id, email); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "delete from auth.users where id = $1", id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return id
}

func TestIntegrationSetAdmin(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	sb := newFakeSupabase(t)
	// A second account that must never be touched.
	other := addAuthUser(t, pool, "zz-tool-other@example.com")
	id := addAuthUser(t, pool, "Zz-Tool-Admin@Example.com")

	want := func(t *testing.T, n int, admin any) {
		t.Helper()
		calls := sb.Calls()
		if len(calls) != n {
			t.Fatalf("%d Supabase calls, want %d: %+v", len(calls), n, calls)
		}
		c := calls[n-1]
		if c.Method != http.MethodPut || c.Path != "/auth/v1/admin/users/"+id.String() || c.APIKey != "sb_secret_test" {
			t.Errorf("call = %+v; want a PUT for the matched user with the secret key", c)
		}
		if got, ok := c.Body["app_metadata"]["admin"]; !ok || got != admin || len(c.Body["app_metadata"]) != 1 {
			t.Errorf("app_metadata = %v; want only admin = %v", c.Body["app_metadata"], admin)
		}
		if strings.Contains(c.Path, other.String()) {
			t.Errorf("touched the other account")
		}
	}

	// The email matches whatever its case, and surrounding space is ignored.
	if err := setAdmin(ctx, pool, []string{" zz-tool-admin@EXAMPLE.com "}); err != nil {
		t.Fatal(err)
	}
	want(t, 1, true)
	// -revoke sends null, which deletes the key and keeps other metadata.
	if err := setAdmin(ctx, pool, []string{"-revoke", "zz-tool-admin@example.com"}); err != nil {
		t.Fatal(err)
	}
	want(t, 2, nil)

	// Refusals write nothing.
	for name, args := range map[string][]string{
		"unknown email": {"zz-tool-nobody@example.com"},
		"no email":      {},
		"two emails":    {"a@example.com", "b@example.com"},
	} {
		if err := setAdmin(ctx, pool, args); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := setAdmin(ctx, pool, []string{"zz-tool-nobody@example.com"}); err == nil || !strings.Contains(err.Error(), "sign in once") {
		t.Errorf("unknown email: err = %v; want the sign-in-once hint", err)
	}
	toolCfg.SupabaseSecretKey = "" // restored by newFakeSupabase's cleanup
	if err := setAdmin(ctx, pool, []string{"zz-tool-admin@example.com"}); err == nil {
		t.Error("no secret key: want an error")
	}
	if n := len(sb.Calls()); n != 2 {
		t.Errorf("%d Supabase calls after the refusals, want still 2", n)
	}
}

func TestIntegrationSetAdminSupabaseFailure(t *testing.T) {
	pool := testPool(t)
	sb := newFakeSupabase(t)
	sb.Status = http.StatusInternalServerError
	addAuthUser(t, pool, "zz-tool-fail@example.com")
	if err := setAdmin(context.Background(), pool, []string{"zz-tool-fail@example.com"}); err == nil {
		t.Error("Supabase 500: want an error, not a silent success")
	}
	// The client retries a 5xx (the PUT is idempotent): 3 attempts, pinned here.
	if n := len(sb.Calls()); n != 3 {
		t.Errorf("%d Supabase attempts, want 3", n)
	}
}

func TestIntegrationSetTranslator(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	sb := newFakeSupabase(t)
	other := addAuthUser(t, pool, "zz-tool-other-tr@example.com")
	id := addAuthUser(t, pool, "Zz-Tool-Translator@Example.com")

	last := func(t *testing.T, n int) supabaseCall {
		t.Helper()
		calls := sb.Calls()
		if len(calls) != n {
			t.Fatalf("%d Supabase calls, want %d: %+v", len(calls), n, calls)
		}
		c := calls[n-1]
		if c.Method != http.MethodPut || c.Path != "/auth/v1/admin/users/"+id.String() || strings.Contains(c.Path, other.String()) {
			t.Errorf("call = %+v; want a PUT for the matched user only", c)
		}
		return c
	}

	// Case-insensitive email; languages are normalised (ja-JP -> ja, zh-TW ->
	// zh-Hant) and listed once, in order.
	if err := setTranslator(ctx, pool, []string{"zz-tool-translator@example.com", "JA, es,ja-JP,zh-TW"}); err != nil {
		t.Fatal(err)
	}
	got := last(t, 1).Body["app_metadata"]["translator_langs"]
	langs, _ := got.([]any)
	if len(langs) != 3 || langs[0] != "ja" || langs[1] != "es" || langs[2] != "zh-Hant" {
		t.Errorf("translator_langs = %v; want [ja es zh-Hant]", got)
	}

	if err := setTranslator(ctx, pool, []string{"-revoke", "zz-tool-translator@example.com"}); err != nil {
		t.Fatal(err)
	}
	if v, ok := last(t, 2).Body["app_metadata"]["translator_langs"]; !ok || v != nil {
		t.Errorf("revoke sent %v (present %v); want null", v, ok)
	}

	// Refusals write nothing: an unknown language, an unknown email, bad usage.
	for name, args := range map[string][]string{
		"unknown language": {"zz-tool-translator@example.com", "ja,xx"},
		"not a language":   {"zz-tool-translator@example.com", "klingon"},
		"unknown email":    {"zz-tool-nobody@example.com", "ja"},
		"no languages":     {"zz-tool-translator@example.com"},
		"revoke + langs":   {"-revoke", "zz-tool-translator@example.com", "ja"},
	} {
		if err := setTranslator(ctx, pool, args); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if n := len(sb.Calls()); n != 2 {
		t.Errorf("%d Supabase calls after the refusals, want still 2", n)
	}
}

// An unknown language is refused before anything is looked up or written, so
// this needs no database.
func TestSetTranslatorRefusesUnknownLanguage(t *testing.T) {
	sb := newFakeSupabase(t)
	err := setTranslator(context.Background(), nil, []string{"a@example.com", "ja,xx"})
	if err == nil || !strings.Contains(err.Error(), "xx") {
		t.Errorf("err = %v; want it to name xx", err)
	}
	if n := len(sb.Calls()); n != 0 {
		t.Errorf("%d Supabase calls, want none", n)
	}
}
