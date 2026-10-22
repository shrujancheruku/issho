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
	defaultURL = "https://exp.host/--/api/v2/push/send"
	// Expo accepts at most 100 messages per request.
	batchSize = 100
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

// Ticket is Expo's answer for one message, in the same order as sent.
type Ticket struct {
	OK bool
	// Error is Expo's error code when !OK, e.g. "DeviceNotRegistered" (the
	// app was uninstalled: stop sending to that token).
	Error   string
	Message string
}

// Sender sends messages and returns a ticket for each.
type Sender interface {
	Send(ctx context.Context, msgs []Message) ([]Ticket, error)
}

// Expo is the Sender for Expo's push service.
type Expo struct {
	// AccessToken is needed only when the Expo project requires it
	// ("enhanced push security"); "" sends without one.
	AccessToken string
	URL         string // overridable for tests
	HTTP        *http.Client
}

func NewExpo(accessToken string) *Expo {
	return &Expo{AccessToken: accessToken, URL: defaultURL, HTTP: &http.Client{Timeout: 30 * time.Second}}
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
	body, err := json.Marshal(msgs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("expo push: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Data []struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	if len(parsed.Data) != len(msgs) {
		return nil, fmt.Errorf("expo push: %d tickets for %d messages", len(parsed.Data), len(msgs))
	}
	tickets := make([]Ticket, len(msgs))
	for i, d := range parsed.Data {
		tickets[i] = Ticket{OK: d.Status == "ok", Error: d.Details.Error, Message: d.Message}
	}
	return tickets, nil
}
