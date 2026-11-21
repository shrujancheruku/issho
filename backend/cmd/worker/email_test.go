package main

import (
	"errors"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/email"
	"github.com/shrujancheruku/issho/backend/internal/email/resend"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestEmailSenderIsResendWhenConfigured(t *testing.T) {
	s, err := newEmailSender(env(map[string]string{"RESEND_API_KEY": "re_x", "EMAIL_FROM": "Issho <hello@issho.example>"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*resend.Sender); !ok {
		t.Errorf("sender is %T, want *resend.Sender", s)
	}
}

func TestEmailSenderIsNotConfiguredWithoutAKey(t *testing.T) {
	s, err := newEmailSender(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(email.NotConfigured); !ok {
		t.Errorf("sender is %T, want email.NotConfigured", s)
	}
}

// A key with no From address is a half-set deploy: starting would only move the
// failure to the first send.
func TestEmailSenderRefusesAKeyWithoutAFromAddress(t *testing.T) {
	_, err := newEmailSender(env(map[string]string{"RESEND_API_KEY": "re_x"}))
	if err == nil || errors.Is(err, email.ErrNotConfigured) {
		t.Errorf("err = %v, want a configuration error", err)
	}
}
