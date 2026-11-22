package main

import (
	"strings"
	"testing"
)

const devRef = "abcdefghijklmnopqrst"

// The opt-in reaches a hosted database only when its target is the dev
// project the operator named; anything else, production included, is refused.
func TestCheckTarget(t *testing.T) {
	direct := "postgresql://postgres:hunter2@db." + devRef + ".supabase.co:5432/postgres"
	pooler := "postgresql://postgres." + devRef + ":hunter2@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres"
	prodRef := "zzzzzzzzzzzzzzzzzzzz"
	tests := []struct {
		name    string
		url     string
		dev     bool
		ref     string
		wantErr bool
	}{
		{"loopback needs no opt-in", "postgresql://postgres:x@127.0.0.1:54322/postgres", false, "", false},
		{"hosted without opt-in", direct, false, devRef, true},
		{"hosted with opt-in but no ref", direct, true, "", true},
		{"dev direct host", direct, true, devRef, false},
		{"dev pooler user", pooler, true, devRef, false},
		{"production direct host", "postgresql://postgres:hunter2@db." + prodRef + ".supabase.co:5432/postgres", true, devRef, true},
		{"production pooler user", "postgresql://postgres." + prodRef + ":hunter2@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres", true, devRef, true},
		{"ref only as a prefix of the host label", "postgresql://postgres:x@db." + devRef + "zz.supabase.co/postgres", true, devRef, true},
		{"ref in someone else's domain", "postgresql://postgres:x@db." + devRef + ".supabase.co.evil.example/postgres", true, devRef, true},
		{"host override in the query", direct + "?host=db." + prodRef + ".supabase.co", true, devRef, true},
		{"service file in the query", direct + "?service=prod", true, devRef, true},
		{"user override in the query names production", pooler + "?user=postgres." + prodRef, true, devRef, true},
		{"user override in the query on a url with no user", "postgresql://aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres?user=postgres." + prodRef + "&password=hunter2", true, devRef, true},
		{"dev user only by query override", "postgresql://postgres." + prodRef + ":hunter2@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres?user=postgres." + devRef, true, devRef, false},
		{"malformed ref", direct, true, "dev", true},
		{"not a url", "not a url", true, devRef, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkTarget(tc.url, tc.dev, tc.ref)
			if (err != nil) != tc.wantErr {
				t.Fatalf("checkTarget error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "hunter2") {
				t.Fatalf("error printed the password: %v", err)
			}
		})
	}
}

// The check is on what pgx will connect with, not on the URL's text: the
// environment fills in whatever the URL leaves out, so a pooler URL with no
// user, run under PGUSER naming production, is production's.
func TestCheckTargetReadsTheConnectionPgxWouldMake(t *testing.T) {
	prodRef := "zzzzzzzzzzzzzzzzzzzz"
	noUser := "postgresql://aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres"
	t.Run("PGUSER names production", func(t *testing.T) {
		t.Setenv("PGUSER", "postgres."+prodRef)
		if err := checkTarget(noUser, true, devRef); err == nil {
			t.Fatal("checkTarget accepted a pooler URL whose user comes from PGUSER (production)")
		}
	})
	t.Run("PGUSER names dev", func(t *testing.T) {
		t.Setenv("PGUSER", "postgres."+devRef)
		if err := checkTarget(noUser, true, devRef); err != nil {
			t.Fatalf("checkTarget refused a pooler URL whose user comes from PGUSER (dev): %v", err)
		}
	})
}
