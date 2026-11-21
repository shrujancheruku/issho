package resend_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/email"
	"github.com/shrujancheruku/issho/backend/internal/email/resend"
)

// transport serves a recorded response from testdata and keeps the request it
// was sent. No test here opens a socket: the client's transport is this.
type transport struct {
	status int
	file   string
	req    *http.Request
	body   []byte
}

func (tr *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.req = r
	tr.body, _ = io.ReadAll(r.Body)
	recorded, err := os.ReadFile("testdata/" + tr.file)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: tr.status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(recorded))),
		Request:    r,
	}, nil
}

func newSender(tr *transport) email.Sender {
	return resend.New("re_test_key", "Issho <hello@issho.example>", resend.WithHTTPClient(&http.Client{Transport: tr}))
}

func TestSendPostsResendsDocumentedRequest(t *testing.T) {
	tr := &transport{status: 200, file: "send_ok.json"}
	m := email.Message{To: "ada@example.com", Subject: "Your code", Text: "123456", HTML: "<p>123456</p>"}

	if err := newSender(tr).Send(context.Background(), m); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if tr.req.Method != http.MethodPost || tr.req.URL.String() != "https://api.resend.com/emails" {
		t.Errorf("request = %s %s, want POST https://api.resend.com/emails", tr.req.Method, tr.req.URL)
	}
	if got := tr.req.Header.Get("Authorization"); got != "Bearer re_test_key" {
		t.Errorf("Authorization = %q", got)
	}
	if got := tr.req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(tr.body, &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	want := map[string]any{
		"from":    "Issho <hello@issho.example>",
		"to":      []any{"ada@example.com"},
		"subject": "Your code",
		"text":    "123456",
		"html":    "<p>123456</p>",
	}
	if len(body) != len(want) {
		t.Errorf("body = %v, want exactly %v", body, want)
	}
	for k, w := range want {
		if g, _ := json.Marshal(body[k]); string(g) != mustJSON(w) {
			t.Errorf("body[%q] = %s, want %s", k, g, mustJSON(w))
		}
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSendOmitsAnEmptyHTMLBody(t *testing.T) {
	tr := &transport{status: 200, file: "send_ok.json"}
	if err := newSender(tr).Send(context.Background(), email.Message{To: "ada@example.com", Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tr.body), `"html"`) {
		t.Errorf("body has an html field for a text-only message: %s", tr.body)
	}
}

func TestSendReportsResendsRejection(t *testing.T) {
	tr := &transport{status: 422, file: "send_rejected.json"}

	err := newSender(tr).Send(context.Background(), email.Message{To: "ada@example.com", Subject: "s", Text: "t"})

	if err == nil {
		t.Fatal("Send succeeded on a 422")
	}
	// The status and Resend's own name for the problem, so a job's log says why.
	for _, s := range []string{"422", "validation_error"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not mention %q", err, s)
		}
	}
}

func TestSendRefusesAnInvalidMessageWithoutARequest(t *testing.T) {
	tr := &transport{status: 200, file: "send_ok.json"}

	err := newSender(tr).Send(context.Background(), email.Message{Subject: "s", Text: "t"})

	if !errors.Is(err, email.ErrInvalidMessage) {
		t.Fatalf("Send = %v, want ErrInvalidMessage", err)
	}
	if tr.req != nil {
		t.Error("a request was made for an invalid message")
	}
}

func TestSendDoesNotLeakTheKeyInAnError(t *testing.T) {
	tr := &transport{status: 401, file: "send_rejected.json"}
	err := newSender(tr).Send(context.Background(), email.Message{To: "ada@example.com", Subject: "s", Text: "t"})
	if err == nil || strings.Contains(err.Error(), "re_test_key") {
		t.Errorf("error = %v; want one that omits the key", err)
	}
}
