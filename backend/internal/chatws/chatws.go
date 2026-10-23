// Package chatws serves group chats over WebSocket (GET /ws/chat), an
// event group's or a crew's:
// the app opens a socket on a chat and gets new messages as they're sent.
// Sending and reading history go through EventService (SendChatMessage,
// ListChatMessages) or CrewService (SendCrewMessage, ListCrewMessages); the
// socket only pushes.
//
// React Native has WebSocket built in but no streaming fetch, so Connect's
// server streaming isn't an option on phones.
//
// Protocol (JSON text frames):
//
//	client → {"token": "<Supabase access token>", "eventId": "…", "afterId": 123}
//	         or {"token", "crewId", "afterId"} for a crew's chat
//	         first message, within 10 s; the token isn't in the URL so it
//	         never lands in logs
//	server → {"type": "messages", "messages": [{id, name, body, at, mine, planId?}, …]}
//	         everything after afterId straight away, then each new message
//	         (planId: a plan suggested in a crew's chat)
//	server → close 4401 bad token, 4403 not going / not a member /
//	         cancelled, 4404 no such event or crew; the app shouldn't
//	         reconnect after these
//
// New messages reach every API instance through Postgres NOTIFY on the
// "chat" channel (payload: the group or crew id; both are uuids, so they
// never collide), sent in the transaction that stores the message.
package chatws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

const (
	channel     = "chat"
	authTimeout = 10 * time.Second
	writeWait   = 10 * time.Second
	pingEvery   = 30 * time.Second
	// Close codes in the 4000s are the application's own.
	closeUnauthorized = 4401
	closeForbidden    = 4403
	closeNotFound     = 4404
)

// Hub tells open sockets when their group has a new message. Run listens
// for the whole process; each socket subscribes to its group.
type Hub struct {
	mu   sync.Mutex
	subs map[uuid.UUID]map[chan struct{}]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[uuid.UUID]map[chan struct{}]struct{}{}} }

// subscribe returns a channel that receives a signal (coalesced: at most
// one pending) whenever the group has new messages.
func (h *Hub) subscribe(group uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subs[group] == nil {
		h.subs[group] = map[chan struct{}]struct{}{}
	}
	h.subs[group][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[group], ch)
		if len(h.subs[group]) == 0 {
			delete(h.subs, group)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) signal(group uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[group] {
		select {
		case ch <- struct{}{}:
		default: // already signalled; the socket reads everything new at once
		}
	}
}

// Run holds one connection listening on the chat channel until ctx ends,
// reconnecting after errors. A gap while reconnecting is harmless: sockets
// also re-read on every signal and the app re-reads when it reconnects.
func (h *Hub) Run(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	for ctx.Err() == nil {
		err := h.listen(ctx, pool)
		if ctx.Err() != nil {
			return
		}
		log.Warn("chat listener stopped; retrying", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (h *Hub) listen(ctx context.Context, pool *pgxpool.Pool) error {
	pooled, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// A connection that has LISTENed must not go back to the pool, so take
	// it out (Hijack) before using it. (Not `defer pooled.Hijack().Close()`:
	// that would hijack it right here, before the LISTEN below.)
	conn := pooled.Hijack()
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "listen "+channel); err != nil {
		return err
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if group, err := uuid.Parse(n.Payload); err == nil {
			h.signal(group)
		}
	}
}

// Handler serves GET /ws/chat.
type Handler struct {
	Hub      *Hub
	Svc      *plans.Service
	Verifier *auth.Verifier
	// OriginPatterns are the browser origins allowed to open sockets (the
	// API's ALLOWED_ORIGINS); apps send no Origin and aren't affected.
	OriginPatterns []string
	Log            *slog.Logger
	Now            func() time.Time // nil = time.Now
}

type hello struct {
	Token   string `json:"token"`
	EventID string `json:"eventId"`
	CrewID  string `json:"crewId"`
	AfterID int64  `json:"afterId"`
}

// Message is one chat message as sent to the app.
type Message struct {
	ID     int64     `json:"id"`
	Name   string    `json:"name"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
	Mine   bool      `json:"mine"`
	PlanID string    `json:"planId,omitempty"`
}

type batch struct {
	Type     string    `json:"type"`
	Messages []Message `json:"messages"`
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return // Accept has written the HTTP error
	}
	defer ws.CloseNow()
	ctx := r.Context()

	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	var first hello
	_, raw, err := ws.Read(authCtx)
	cancel()
	if err != nil || json.Unmarshal(raw, &first) != nil {
		ws.Close(closeUnauthorized, "send {token, eventId} first")
		return
	}
	caller, err := h.Verifier.Verify(first.Token)
	if err != nil {
		ws.Close(closeUnauthorized, "invalid token")
		return
	}
	// fetch reads the chat after a message; topic is what NOTIFY names.
	var fetch func(after int64) ([]Message, error)
	var topic uuid.UUID
	if first.CrewID != "" {
		crewID, err := uuid.Parse(first.CrewID)
		if err != nil {
			ws.Close(closeNotFound, "no such crew")
			return
		}
		topic = crewID
		fetch = func(after int64) ([]Message, error) {
			ms, err := h.Svc.CrewMessages(ctx, caller.UserID, crewID, after)
			out := make([]Message, len(ms))
			for i, m := range ms {
				out[i] = Message{ID: m.ID, Name: m.Name, Body: m.Body, At: m.At, Mine: m.Mine}
				if m.PlanID != uuid.Nil {
					out[i].PlanID = m.PlanID.String()
				}
			}
			return out, err
		}
	} else {
		eventID, err := uuid.Parse(first.EventID)
		if err != nil {
			ws.Close(closeNotFound, "no such event")
			return
		}
		fetch = func(after int64) ([]Message, error) {
			ms, st, err := h.Svc.ChatMessages(ctx, caller.UserID, eventID, after, h.now())
			topic = st.GroupID
			out := make([]Message, len(ms))
			for i, m := range ms {
				out[i] = Message{ID: m.ID, Name: m.Name, Body: m.Body, At: m.At, Mine: m.Mine}
			}
			return out, err
		}
	}
	msgs, err := fetch(first.AfterID)
	switch {
	case errors.Is(err, plans.ErrNoEvent), errors.Is(err, plans.ErrNoCrew):
		ws.Close(closeNotFound, "no such chat")
		return
	case errors.Is(err, plans.ErrNotGoing), errors.Is(err, plans.ErrEventOver), errors.Is(err, plans.ErrNoProfile),
		errors.Is(err, plans.ErrNotInCrew), errors.Is(err, plans.ErrCrewOver):
		ws.Close(closeForbidden, "not going")
		return
	case err != nil:
		h.Log.Error("chat socket", "err", err)
		ws.Close(websocket.StatusInternalError, "internal error")
		return
	}

	signals, unsubscribe := h.Hub.subscribe(topic)
	defer unsubscribe()
	// The client never sends anything else; reading handles its pings and
	// notices it closing.
	ctx = ws.CloseRead(ctx)
	last := first.AfterID
	send := func(ms []Message) error {
		if len(ms) == 0 {
			return nil
		}
		last = ms[len(ms)-1].ID
		b, _ := json.Marshal(batch{Type: "messages", Messages: ms})
		wctx, cancel := context.WithTimeout(ctx, writeWait)
		defer cancel()
		return ws.Write(wctx, websocket.MessageText, b)
	}
	if err := send(msgs); err != nil {
		return
	}
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			// Keeps proxies from closing an idle socket.
			pctx, cancel := context.WithTimeout(ctx, writeWait)
			err := ws.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		case <-signals:
			ms, err := fetch(last)
			if err != nil {
				// Cancelled or no longer going: close so the app stops listening.
				ws.Close(closeForbidden, "chat no longer available")
				return
			}
			if err := send(ms); err != nil {
				return
			}
		}
	}
}
