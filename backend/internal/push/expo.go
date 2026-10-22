// Package push sends push notifications through Expo's push service, which
// forwards them to Apple (APNs) and Google (FCM) with the credentials
// uploaded to the Expo project. It also serves PushService, where the app
// registers its device's Expo push token.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultURL         = "https://exp.host/--/api/v2/push/send"
	defaultReceiptsURL = "https://exp.host/--/api/v2/push/getReceipts"
	// Expo accepts at most 100 messages, or 1,000 receipt ids, per request.
	batchSize        = 100
	receiptBatchSize = 1000
)

// Message is one notification to one device.
type Message struct {
	To    string            `json:"to"`
	Title string            `json:"title,omitempty"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
	Sound string            `json:"sound,omitempty"`
	// ChannelID is the Android notification channel the app created.
	ChannelID string `json:"channelId,omitempty"`
}

// Ticket is Expo's answer for one message, in the same order as sent. OK
// means Expo accepted it, not that it reached the phone: that's in the
// receipt for ID, ready some minutes later.
type Ticket struct {
	OK bool
	ID string // the receipt id, when OK
	// Error is Expo's error code when !OK, e.g. "DeviceNotRegistered" (the
	// app was uninstalled: stop sending to that token).
	Error   string
	Message string
}

// Receipt says whether Apple or Google accepted a message Expo forwarded.
// Error is e.g. "DeviceNotRegistered" (the app is gone: stop sending to
// that token) or "MessageRateExceeded".
type Receipt struct {
	OK      bool
	Error   string
	Message string
}

// Sender sends messages and returns a ticket for each, and fetches the
// receipts for tickets. Receipts missing from the answer aren't ready yet
// (or have expired after 24 hours).
type Sender interface {
	Send(ctx context.Context, msgs []Message) ([]Ticket, error)
	Receipts(ctx context.Context, ids []string) (map[string]Receipt, error)
}

// Expo is the Sender for Expo's push service.
type Expo struct {
	// AccessToken is needed only when the Expo project requires it
	// ("enhanced push security"); "" sends without one.
	AccessToken string
	URL         string // overridable for tests
	ReceiptsURL string
	HTTP        *http.Client
}

func NewExpo(accessToken string) *Expo {
	return &Expo{AccessToken: accessToken, URL: defaultURL, ReceiptsURL: defaultReceiptsURL, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (e *Expo) Send(ctx context.Context, msgs []Message) ([]Ticket, error) {
	out := make([]Ticket, 0, len(msgs))
	for start := 0; start < len(msgs); start += batchSize {
		batch := msgs[start:min(start+batchSize, len(msgs))]
		tickets, err := e.send(ctx, batch)
		if err != nil {
			return out, err
		}
		out = append(out, tickets...)
	}
	return out, nil
}

func (e *Expo) send(ctx context.Context, msgs []Message) ([]Ticket, error) {
	var parsed struct {
		Data []struct {
			Status  string `json:"status"`
			ID      string `json:"id"`
			Message string `json:"message"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := e.post(ctx, e.URL, msgs, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(msgs) {
		return nil, fmt.Errorf("expo push: %d tickets for %d messages", len(parsed.Data), len(msgs))
	}
	tickets := make([]Ticket, len(msgs))
	for i, d := range parsed.Data {
		tickets[i] = Ticket{OK: d.Status == "ok", ID: d.ID, Error: d.Details.Error, Message: d.Message}
	}
	return tickets, nil
}

func (e *Expo) Receipts(ctx context.Context, ids []string) (map[string]Receipt, error) {
	out := map[string]Receipt{}
	for start := 0; start < len(ids); start += receiptBatchSize {
		var parsed struct {
			Data map[string]struct {
				Status  string `json:"status"`
				Message string `json:"message"`
				Details struct {
					Error string `json:"error"`
				} `json:"details"`
			} `json:"data"`
		}
		body := map[string][]string{"ids": ids[start:min(start+receiptBatchSize, len(ids))]}
		if err := e.post(ctx, e.ReceiptsURL, body, &parsed); err != nil {
			return out, err
		}
		for id, d := range parsed.Data {
			out[id] = Receipt{OK: d.Status == "ok", Error: d.Details.Error, Message: d.Message}
		}
	}
	return out, nil
}

// post sends v as JSON to url and decodes the answer into out.
func (e *Expo) post(ctx context.Context, url string, v, out any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("expo push: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("expo push: %w", err)
	}
	return nil
}
