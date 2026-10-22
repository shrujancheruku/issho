package push

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpoSendBatchesAndParsesTickets(t *testing.T) {
	var requests [][]Message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization header = %q", r.Header.Get("Authorization"))
		}
		var msgs []Message
		if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, msgs)
		var data []map[string]any
		for _, m := range msgs {
			if m.To == "ExponentPushToken[gone]" {
				data = append(data, map[string]any{"status": "error", "message": "not registered", "details": map[string]string{"error": "DeviceNotRegistered"}})
			} else {
				data = append(data, map[string]any{"status": "ok", "id": "x"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	e := NewExpo("secret")
	e.URL = srv.URL

	var msgs []Message
	for i := range 150 {
		msgs = append(msgs, Message{To: fmt.Sprintf("ExponentPushToken[%d]", i), Body: "hi"})
	}
	msgs[120].To = "ExponentPushToken[gone]"
	tickets, err := e.Send(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || len(requests[0]) != 100 || len(requests[1]) != 50 {
		t.Errorf("batches: %d requests", len(requests))
	}
	if len(tickets) != 150 || !tickets[0].OK || tickets[120].OK || tickets[120].Error != "DeviceNotRegistered" {
		t.Errorf("tickets: %d, [0]=%+v, [120]=%+v", len(tickets), tickets[0], tickets[120])
	}
}

func TestExpoSendFailsOnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":[{"code":"TOO_MANY_REQUESTS"}]}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()
	e := NewExpo("")
	e.URL = srv.URL
	if _, err := e.Send(context.Background(), []Message{{To: "ExponentPushToken[a]", Body: "hi"}}); err == nil {
		t.Error("no error for HTTP 429")
	}
}
