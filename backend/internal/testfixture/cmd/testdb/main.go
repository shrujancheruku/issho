// Command testdb manages the template database for the integration tests, for
// scripts/db-checks.sh (which has no psql to count on):
//
//	testdb create <url> <source-url>  make an empty database named in <url>
//	                                  (replacing any), with the auth.users that
//	                                  the one <source-url> names has, if any
//	testdb sweep <url>                drop the database named in <url> and every
//	                                  per-package database a killed run left behind
//
// <url> is the template's connection string. The statements run from
// template1, never the database they act on.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: testdb create <template-url> <source-url> | testdb sweep <template-url>")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := run(ctx, os.Args[1], os.Args[2], os.Args[3:])
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd, rawURL string, rest []string) error {
	cfg, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return fmt.Errorf("not a valid connection string: %w", err)
	}
	name := cfg.Database
	if strings.HasPrefix(name, testfixture.DatabasePrefix) || name == "" || name == "postgres" || strings.HasPrefix(name, "template") {
		return fmt.Errorf("refusing to manage database %q: the template needs a name of its own", name)
	}
	admin := cfg.Copy()
	admin.Database = "template1"
	conn, err := pgx.ConnectConfig(ctx, admin)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	drop := func(db string) error {
		_, err := conn.Exec(ctx, "drop database if exists "+pgx.Identifier{db}.Sanitize()+" with (force)")
		return err
	}
	switch cmd {
	case "create":
		if len(rest) != 1 {
			return errors.New("create needs the source database's URL")
		}
		if err := drop(name); err != nil {
			return err
		}
		// The role needs CREATEDB (Supabase's `postgres` has it).
		if _, err := conn.Exec(ctx, "create database "+pgx.Identifier{name}.Sanitize()); err != nil {
			return err
		}
		return copyAuthUsers(ctx, rest[0], cfg)
	case "sweep":
		rows, err := conn.Query(ctx, "select datname from pg_database where datname like $1", testfixture.DatabasePrefix+"%")
		if err != nil {
			return err
		}
		stale, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, db := range append(stale, name) {
			if err := drop(db); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown command %q", cmd)
}

// copyAuthUsers gives the template an auth.users like the source's, when the
// source has one. The Supabase image creates the auth schema in its default
// database only, and that database can't be cloned (pg_cron and pg_net keep
// sessions on it), so the template is built empty and migrated; app.users'
// foreign key to auth.users needs the table to exist first, and the tests
// insert into it. Only the columns and the keys the tests rely on are copied.
func copyAuthUsers(ctx context.Context, srcURL string, tmpl *pgx.ConnConfig) error {
	src, err := pgx.Connect(ctx, srcURL)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	defer func() { _ = src.Close(ctx) }()
	rows, err := src.Query(ctx, `select attname::text, format_type(atttypid, atttypmod)
		from pg_attribute where attrelid = to_regclass('auth.users') and attnum > 0 and not attisdropped
		order by attnum`)
	if err != nil {
		return err
	}
	type col struct{ name, typ string }
	cols, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (col, error) {
		var c col
		return c, r.Scan(&c.name, &c.typ)
	})
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil // plain Postgres: no auth schema, and the tests know
	}
	defs := make([]string, 0, len(cols))
	for _, c := range cols {
		def := pgx.Identifier{c.name}.Sanitize() + " " + c.typ
		switch c.name {
		case "id":
			def += " primary key"
		case "email":
			def += " unique"
		}
		defs = append(defs, def)
	}
	dst, err := pgx.ConnectConfig(ctx, tmpl)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close(ctx) }()
	_, err = dst.Exec(ctx, "create schema if not exists auth; create table auth.users ("+strings.Join(defs, ", ")+")")
	return err
}
