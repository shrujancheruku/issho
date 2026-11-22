package people_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// supabaseConfig is the part of supabase/config.toml that makes sign-in an
// email code. The stack itself isn't started in tests, so this reads the file
// the local stack is started from.
type supabaseConfig struct {
	API struct {
		Enabled bool `toml:"enabled"`
	} `toml:"api"`
	LocalSMTP struct {
		Enabled bool `toml:"enabled"`
	} `toml:"local_smtp"`
	Auth struct {
		EnableAnonymousSignIns bool `toml:"enable_anonymous_sign_ins"`
		Email                  struct {
			EnableConfirmations bool `toml:"enable_confirmations"`
			OTPLength           int  `toml:"otp_length"`
			OTPExpiry           int  `toml:"otp_expiry"`
			Template            map[string]struct {
				Subject     string `toml:"subject"`
				ContentPath string `toml:"content_path"`
			} `toml:"template"`
			SMTP *struct{} `toml:"smtp"`
		} `toml:"email"`
	} `toml:"auth"`
}

const repoRoot = "../../.."

func loadSupabaseConfig(t *testing.T) supabaseConfig {
	t.Helper()
	var c supabaseConfig
	if _, err := toml.DecodeFile(filepath.Join(repoRoot, "supabase", "config.toml"), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// Sign-in is a 6-digit code that Auth emails through the local mail catcher
// (the local stand-in for Resend's SMTP, docs/spec/architecture.md, "The seams").
func TestSupabaseAuthSendsASixDigitCodeToTheLocalMailCatcher(t *testing.T) {
	c := loadSupabaseConfig(t)
	if c.Auth.Email.OTPLength != 6 {
		t.Errorf("otp_length = %d, want 6", c.Auth.Email.OTPLength)
	}
	if c.Auth.Email.OTPExpiry <= 0 || c.Auth.Email.OTPExpiry > 3600 {
		t.Errorf("otp_expiry = %d seconds, want one hour or less", c.Auth.Email.OTPExpiry)
	}
	if !c.LocalSMTP.Enabled {
		t.Error("[local_smtp] is off: the code's email has nowhere to land locally")
	}
	if c.Auth.Email.SMTP != nil {
		t.Error("[auth.email.smtp] is set: the local stack must catch its mail, not send it; a hosted project's SMTP belongs in its [remotes.<project>] override")
	}
	if c.Auth.EnableAnonymousSignIns {
		t.Error("anonymous sign-ins are on: every account is an email's")
	}
	if c.API.Enabled {
		t.Error("the Data API is on: clients reach the database only through the Go API")
	}
}

// Auth's own password sign-up stays callable by anyone who reaches its URL
// (the app never calls it). Without confirmation, someone could register a
// password for an address that hasn't joined; with it, the address is not a
// user until its owner confirms it with the emailed code. A code sign-in is
// that confirmation (a new person gets the "confirmation" email, whose code
// TestBothSignInEmailsShowTheCode checks), so the code flow works with this on.
func TestAnAddressIsConfirmedByItsCodeBeforeItIsAUser(t *testing.T) {
	c := loadSupabaseConfig(t)
	if !c.Auth.Email.EnableConfirmations {
		t.Error("[auth.email] enable_confirmations is off: a password can be registered for an address that hasn't joined")
	}
}

// A new person's code arrives in the "confirm signup" email and an existing
// person's in the "magic link" one, so both must show the code, not a link.
func TestBothSignInEmailsShowTheCode(t *testing.T) {
	c := loadSupabaseConfig(t)
	for _, name := range []string{"confirmation", "magic_link"} {
		t.Run(name, func(t *testing.T) {
			tmpl, ok := c.Auth.Email.Template[name]
			if !ok || tmpl.ContentPath == "" || tmpl.Subject == "" {
				t.Fatalf("[auth.email.template.%s] needs a subject and a content_path", name)
			}
			// content_path is relative to the repo root, where the CLI runs.
			body, err := os.ReadFile(filepath.Join(repoRoot, tmpl.ContentPath))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "{{ .Token }}") {
				t.Errorf("%s does not show {{ .Token }}, the 6-digit code", tmpl.ContentPath)
			}
			if strings.Contains(string(body), "{{ .ConfirmationURL }}") {
				t.Errorf("%s offers a link: sign-in is by code only", tmpl.ContentPath)
			}
		})
	}
}
