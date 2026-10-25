package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/platform/supabase"
)

// setTranslator sets the languages someone edits in the admin SPA's
// translation editor (or with -revoke removes the role). Like admins,
// translators are ordinary Supabase users; the role is
// app_metadata.translator_langs, carried in their access tokens. It grants
// the translation editor only, for those languages: none of the admin
// pages. The person must have signed in once.
func setTranslator(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("set-translator", flag.ExitOnError)
	revoke := fs.Bool("revoke", false, "remove the translator role instead")
	_ = fs.Parse(args)
	if (*revoke && fs.NArg() != 1) || (!*revoke && fs.NArg() != 2) {
		return errors.New("usage: tool set-translator <email> <ja,es,...> | tool set-translator -revoke <email>")
	}
	email := strings.TrimSpace(fs.Arg(0))
	var langs []string
	if !*revoke {
		for _, l := range strings.Split(fs.Arg(1), ",") {
			l = strings.ToLower(strings.TrimSpace(l))
			if !i18n.Valid(l) {
				return fmt.Errorf("%q isn't an ISO 639-1 language code", l)
			}
			if !slices.Contains(langs, l) {
				langs = append(langs, l)
			}
		}
	}
	url, key := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"), os.Getenv("SUPABASE_SECRET_KEY")
	if url == "" || key == "" {
		return errors.New("SUPABASE_URL and SUPABASE_SECRET_KEY are required")
	}
	var id uuid.UUID
	err := pool.QueryRow(ctx, "select id from auth.users where lower(email) = lower($1)", email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no Supabase account for %s: sign in once first", email)
	}
	if err != nil {
		return err
	}
	if err := supabase.NewAdmin(url, key).SetTranslatorLangs(ctx, id, langs); err != nil {
		return err
	}
	slog.Info("translator role updated; takes effect when their token next refreshes (sign out and in to apply now)",
		"user_id", id, "languages", langs)
	return nil
}
