// Package email is the seam to the outside service that sends the product's
// own email (docs/spec/architecture.md, "The seams"). A domain package takes an
// email.Sender and never names a provider; the real one, internal/email/resend,
// is wired only in cmd/, and every test uses the Fake in this package.
package email

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
)

// Sender sends one email. A nil error means the provider accepted it, not that
// it was delivered.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Message is one email to one person. The sender's address belongs to the
// implementation, not the message, so no code can send as someone else.
type Message struct {
	To      string // a single address, e.g. "ada@example.com"
	Subject string
	Text    string // the plain-text body
	HTML    string // optional; the same content as Text, formatted
}

// ErrInvalidMessage is wrapped by Send when the message is malformed. Every
// Sender refuses the same messages, so a test that passes against the Fake
// can't hand production one it would reject.
var ErrInvalidMessage = errors.New("email: invalid message")

// ErrNotConfigured is what NotConfigured returns from every Send.
var ErrNotConfigured = errors.New("email: no sender is configured")

// Validate reports why the message can't be sent, wrapping ErrInvalidMessage.
func (m Message) Validate() error {
	// ParseAddress would accept "Ada <a@x.com>" and lists; the recipient is one
	// bare address, because the fallback rules in the spec decide per person.
	addr, err := mail.ParseAddress(m.To)
	if err != nil || addr.Address != m.To {
		return fmt.Errorf("%w: To must be one bare address", ErrInvalidMessage)
	}
	if m.Subject == "" {
		return fmt.Errorf("%w: no subject", ErrInvalidMessage)
	}
	if m.Text == "" && m.HTML == "" {
		return fmt.Errorf("%w: no body", ErrInvalidMessage)
	}
	return nil
}

// NotConfigured is the Sender for a process started without a provider key: it
// lets the process run (a local worker needs no email) and fails loudly the
// moment something tries to send.
type NotConfigured struct{}

// Send always returns ErrNotConfigured.
func (NotConfigured) Send(context.Context, Message) error { return ErrNotConfigured }
