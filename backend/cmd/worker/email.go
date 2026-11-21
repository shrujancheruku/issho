package main

import (
	"errors"

	"github.com/shrujancheruku/issho/backend/internal/email"
	"github.com/shrujancheruku/issho/backend/internal/email/resend"
)

// newEmailSender is where the real email provider is wired: cmd/ is the only
// place allowed to name resend (make boundaries). Without RESEND_API_KEY the
// worker still runs, for local work that sends no email, and a send fails
// loudly with email.ErrNotConfigured.
func newEmailSender(getenv func(string) string) (email.Sender, error) {
	key := getenv("RESEND_API_KEY")
	if key == "" {
		return email.NotConfigured{}, nil
	}
	from := getenv("EMAIL_FROM")
	if from == "" {
		return nil, errors.New("EMAIL_FROM is not set (RESEND_API_KEY is)")
	}
	return resend.New(key, from), nil
}
