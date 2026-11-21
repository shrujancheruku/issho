// Package resend is the real email.Sender: it posts to Resend's HTTP API
// (https://resend.com/docs/api-reference/emails/send-email). Only cmd/ imports
// it; everything else takes an email.Sender.
//
//boundaries:outside-service
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/email"
)

const endpoint = "https://api.resend.com/emails"

// Sender sends through Resend.
type Sender struct {
	apiKey string
	from   string
	client *http.Client
}

var _ email.Sender = (*Sender)(nil)

// Option changes how New builds a Sender.
type Option func(*Sender)

// WithHTTPClient replaces the HTTP client, which tests give a transport that
// serves a recorded response so nothing touches the network.
func WithHTTPClient(c *http.Client) Option { return func(s *Sender) { s.client = c } }

// New returns a Sender that sends as from (an address, or "Name <address>")
// with the given API key. The key is held only here and never put in an error.
func New(apiKey, from string, opts ...Option) *Sender {
	// A job that waits forever on Resend holds a River worker slot; the job's
	// own retry is the recovery for a slow response.
	s := &Sender{apiKey: apiKey, from: from, client: &http.Client{Timeout: 15 * time.Second}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// request is Resend's send-email body. Fields left empty are omitted, as the
// API treats an empty html as a message with no HTML part.
type request struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
}

// apiError is the body Resend returns with a 4xx or 5xx.
type apiError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Send posts m to Resend. It returns nil for a 2xx and otherwise an error that
// names the status and Resend's own reason.
func (s *Sender) Send(ctx context.Context, m email.Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(request{From: s.from, To: []string{m.To}, Subject: m.Subject, Text: m.Text, HTML: m.HTML})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		// url.Error carries the URL, never the headers, so the key stays out.
		return fmt.Errorf("resend: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	var ae apiError
	// A body that isn't Resend's JSON (a proxy's HTML page) leaves ae empty;
	// the status alone is still reported.
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&ae)
	return fmt.Errorf("resend: send: status %d: %s: %s", resp.StatusCode, ae.Name, ae.Message)
}
