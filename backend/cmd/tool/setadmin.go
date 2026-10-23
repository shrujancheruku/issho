package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/platform/supabase"
)

// setAdmin grants (or with -revoke removes) the admin role for the admin
// SPA. Admins are ordinary Supabase users (owner's decision): the role is
// app_metadata.admin on their account, set through Supabase's admin API
// like any other account change, and carried in their access tokens. The
// person must have signed in once (the account must exist).
func setAdmin(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("set-admin", flag.ExitOnError)
	revoke := fs.Bool("revoke", false, "remove the admin role instead")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: tool set-admin [-revoke] <email>")
	}
	email := strings.TrimSpace(fs.Arg(0))
	url, key := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"), os.Getenv("SUPABASE_SECRET_KEY")
	if url == "" || key == "" {
		return errors.New("SUPABASE_URL and SUPABASE_SECRET_KEY are required")
	}
	// Supabase's admin API has no lookup by email; its users table does.
	var id uuid.UUID
	err := pool.QueryRow(ctx, "select id from auth.users where lower(email) = lower($1)", email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no Supabase account for %s: sign in once first", email)
	}
	if err != nil {
		return err
	}
	if err := supabase.NewAdmin(url, key).SetAdmin(ctx, id, !*revoke); err != nil {
		return err
	}
	slog.Info("admin role updated; takes effect when their token next refreshes (sign out and in to apply now)",
		"user_id", id, "admin", !*revoke)
	return nil
}
