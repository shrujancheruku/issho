// Package chatws serves chat rooms over WebSocket (GET /ws/chat), an event
// group's or a crew's (internal/chat): the app opens a socket on a room and
// gets new messages as they're sent. Sending and reading history go through
// ChatService (SendMessage, ListMessages); the socket only pushes.
//
// React Native has WebSocket built in but no streaming fetch, so Connect's
// server streaming isn't an option on phones.
//
// Protocol (JSON text frames):
//
//	client → {"token": "<Supabase access token>", "room": {"eventId": "…"} or {"crewId": "…"}, "afterId": 123}
//	         first message, within 10 s; the token isn't in the URL so it
//	         never lands in logs. afterId 0 asks for the room's latest
//	         messages, anything else for all messages after it.
//	server → {"type": "ready", "typing": true}
//	         once the socket is subscribed (before any messages): the
//	         server takes typing notices, so the app waits for this before
//	         sending any
//	server → {"type": "messages", "messages": [{id, userId?, name, body, at, mine, planId?}, …]}
//	         the messages after afterId straight away, then each new
//	         message; at most chat.Page per frame, as many frames as it
//	         takes (userId: the sender; planId: a plan suggested in a
//	         crew's chat, with no sender)
//	client → {"type": "typing"}
//	         while the person types, at most every couple of seconds; passed
//	         on only while they could write a message (the room's access is
//	         checked, as for reading)
//	server → {"type": "typing", "userId": "…", "name": "Aiko"}
//	         someone else in the room is typing; the app shows it for a few
//	         seconds unless another notice keeps it up
//	server → close 4401 bad token or hello, 4403 not (or no longer) allowed
//	         in the room, e.g. not going, not a member, cancelled, 4404 no
//	         such room; the app shouldn't reconnect after these. Any other
//	         close (1011 after a server error, a dropped connection) is
//	         worth retrying.
//
// New messages reach every API instance through Postgres NOTIFY on the
// "chat" channel (payload: the room's topic, an event group's or a crew's id;
// both are uuids, so they never collide), sent in the transaction that stores
// the message, which also holds the room's lock so ids commit in order: after
// a signal a socket reads everything after the last id it sent, a page at a
// time until a page isn't full. Typing notices go the same way on
// "chat_typing" (payload: JSON, chat.Typing); they're never stored.
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

	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

const (
	channel       = "chat"
	typingChannel = "chat_typing"
	// The most often one socket passes on that its person is typing.
	typingEvery = 2 * time.Second
	readLimit   = 512 // client frames after the first are tiny
	authTimeout = 10 * time.Second
	writeWait   = 10 * time.Second
	pingEvery   = 30 * time.Second
	// Close codes in the 4000s are the application's own.
	closeUnauthorized = 4401
	closeForbidden    = 4403
	closeNotFound     = 4404
)

// Hub tells open sockets when their room has a new message or someone in it
// is typing. Run listens for the whole process; each socket subscribes to
// its room's topic.
type Hub struct {
	mu   sync.Mutex
	subs map[uuid.UUID]map[*sub]struct{}
}

// sub is one socket's subscription: a signal (coalesced: at most one
// pending) whenever the room has new messages, and typing notices (a few
// buffered; more are dropped, as the next one says the same).
type sub struct {
	signal chan struct{}
	typing chan chat.Typing
}

func NewHub() *Hub { return &Hub{subs: map[uuid.UUID]map[*sub]struct{}{}} }

func (h *Hub) subscribe(topic uuid.UUID) (*sub, func()) {
	s := &sub{signal: make(chan struct{}, 1), typing: make(chan chat.Typing, 4)}
	h.mu.Lock()
	if h.subs[topic] == nil {
		h.subs[topic] = map[*sub]struct{}{}
	}
	h.subs[topic][s] = struct{}{}
	h.mu.Unlock()
	return s, func() {
		h.mu.Lock()
		delete(h.subs[topic], s)
		if len(h.subs[topic]) == 0 {
			delete(h.subs, topic)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) signal(topic uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[topic] {
		select {
		case s.signal <- struct{}{}:
		default: // already signalled; the socket reads everything new at once
		}
	}
}

func (h *Hub) typed(t chat.Typing) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[t.Topic] {
		select {
		case s.typing <- t:
		default: // the socket is behind; the next notice says the same
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
	// Close even though ctx has ended (that is why Run called us back): keep
	// ctx's values but drop its cancellation.
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	for _, ch := range []string{channel, typingChannel} {
		if _, err := conn.Exec(ctx, "listen "+ch); err != nil {
			return err
		}
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		switch n.Channel {
		case channel:
			if topic, err := uuid.Parse(n.Payload); err == nil {
				h.signal(topic)
			}
		case typingChannel:
			var t chat.Typing
			if json.Unmarshal([]byte(n.Payload), &t) == nil {
				h.typed(t)
			}
		}
	}
}

// errGone means the socket can't be written to.
var errGone = errors.New("socket gone")

// Handler serves GET /ws/chat.
type Handler struct {
	Hub      *Hub
	Chat     *chat.Service
	Verifier *auth.Verifier
	// Suspended refuses suspended people, as the API's Gate does (nil =
	// nobody is).
	Suspended func(ctx context.Context, userID uuid.UUID) (bool, error)
	// OriginPatterns are the browser origins allowed to open sockets (the
	// API's ALLOWED_ORIGINS); apps send no Origin and aren't affected.
	OriginPatterns []string
	Log            *slog.Logger
	Now            func() time.Time // nil = time.Now
}

type hello struct {
	Token string `json:"token"`
	Room  struct {
		EventID string `json:"eventId"`
		CrewID  string `json:"crewId"`
	} `json:"room"`
	AfterID int64 `json:"afterId"`
}

// Message is one chat message as sent to the app.
type Message struct {
	ID     int64     `json:"id"`
	UserID string    `json:"userId,omitempty"`
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

func toMessage(m chat.Message) Message {
	out := Message{ID: m.ID, Name: m.Name, Body: m.Body, At: m.At, Mine: m.Mine}
	if m.UserID != uuid.Nil {
		out.UserID = m.UserID.String()
	}
	if m.PlanID != uuid.Nil {
		out.PlanID = m.PlanID.String()
	}
	return out
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// closeFor ends the socket for an access error: the app is told not to
// come back after the permanent ones; anything else is ours to log, and the
// app retries.
func (h *Handler) closeFor(ws *websocket.Conn, err error) {
	switch {
	case errors.Is(err, chat.ErrNoRoom):
		_ = ws.Close(closeNotFound, "no such chat")
	case errors.Is(err, chat.ErrForbidden):
		_ = ws.Close(closeForbidden, "not allowed in this chat")
	default:
		h.Log.Error("chat socket", "err", err)
		_ = ws.Close(websocket.StatusInternalError, "internal error")
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.OriginPatterns})
	if err != nil {
		return // Accept has written the HTTP error
	}
	defer func() { _ = ws.CloseNow() }()
	ctx := r.Context()

	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	var first hello
	_, raw, err := ws.Read(authCtx)
	cancel()
	if err != nil || json.Unmarshal(raw, &first) != nil {
		_ = ws.Close(closeUnauthorized, `send {token, room: {eventId|crewId}, afterId} first`)
		return
	}
	caller, err := h.Verifier.Verify(first.Token)
	if err != nil {
		_ = ws.Close(closeUnauthorized, "invalid token")
		return
	}
	// A socket already open when someone is suspended ends when they're
	// taken out of the group or crew (the access check below, on each read).
	if h.Suspended != nil {
		if suspended, err := h.Suspended(ctx, caller.UserID); err != nil || suspended {
			_ = ws.Close(closeForbidden, "account suspended")
			return
		}
	}
	room, err := chat.ParseRoom(first.Room.EventID, first.Room.CrewID)
	if err != nil {
		_ = ws.Close(closeNotFound, "no such chat")
		return
	}
	// Subscribe before reading, so a message committed in between still
	// signals the loop below.
	st, err := h.Chat.State(ctx, caller.UserID, room, h.now())
	if err != nil {
		h.closeFor(ws, err)
		return
	}
	subscription, unsubscribe := h.Hub.subscribe(st.Topic)
	defer unsubscribe()

	// After the first message the client only sends typing notices. The
	// reader also handles its pings and notices it closing, which ends ctx.
	ws.SetReadLimit(readLimit)
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	typed := make(chan struct{}, 1)
	// suspended is signalled when a send finds the person suspended since
	// they connected; the loop below closes the socket.
	suspended := make(chan struct{}, 1)
	go func() {
		defer stop()
		for {
			_, raw, err := ws.Read(ctx)
			if err != nil {
				return
			}
			var f struct {
				Type string `json:"type"`
			}
			// Every send (a typing notice is all a socket sends) is checked
			// against the suspended list the API gate uses, which is cached in
			// memory, so this costs no query. The hub never pushes a close.
			// A cache error fails open here on purpose, unlike the connect-time
			// check: a database blip shouldn't drop live sockets, and the API gate
			// still refuses the person.
			if h.Suspended != nil {
				if sus, err := h.Suspended(ctx, caller.UserID); err == nil && sus {
					select {
					case suspended <- struct{}{}:
					default:
					}
					continue // the loop below closes the socket
				}
			}
			if json.Unmarshal(raw, &f) == nil && f.Type == "typing" {
				select {
				case typed <- struct{}{}:
				default:
				}
			}
		}
	}()

	write := func(v any) error {
		b, _ := json.Marshal(v)
		wctx, cancel := context.WithTimeout(ctx, writeWait)
		defer cancel()
		return ws.Write(wctx, websocket.MessageText, b)
	}
	last := first.AfterID
	send := func(ms []chat.Message) error {
		if len(ms) == 0 {
			return nil
		}
		last = ms[len(ms)-1].ID
		out := make([]Message, len(ms))
		for i, m := range ms {
			out[i] = toMessage(m)
		}
		if err := write(batch{Type: "messages", Messages: out}); err != nil {
			return errGone
		}
		return nil
	}
	// drain sends everything after the last message sent, a page at a time:
	// more than a page can pile up between signals (they coalesce) or while
	// the app was away. It fails with the access check's error if the person
	// is no longer allowed, or errGone if the socket is.
	drain := func() error {
		for {
			ms, _, err := h.Chat.After(ctx, caller.UserID, room, last, h.now())
			if err != nil {
				return err
			}
			if err := send(ms); err != nil {
				return err
			}
			if len(ms) < chat.Page {
				return nil
			}
		}
	}
	// end closes the socket for an error from reading or sending.
	end := func(err error) {
		if !errors.Is(err, errGone) {
			h.closeFor(ws, err)
		}
	}

	if err := write(map[string]any{"type": "ready", "typing": true}); err != nil {
		return
	}
	if first.AfterID == 0 {
		ms, _, err := h.Chat.Latest(ctx, caller.UserID, room, h.now())
		if err == nil {
			err = send(ms)
		}
		if err != nil {
			end(err)
			return
		}
	}
	// With afterId 0 and an empty room, last stays 0 and the next drain
	// reads from the beginning, which is everything.
	if err := drain(); err != nil {
		end(err)
		return
	}

	var lastTyped time.Time
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
		case <-subscription.signal:
			if err := drain(); err != nil {
				// Cancelled or no longer going: the app stops listening.
				end(err)
				return
			}
		case <-suspended:
			_ = ws.Close(closeForbidden, "account suspended")
			return
		case <-typed:
			now := h.now()
			if now.Sub(lastTyped) < typingEvery {
				continue
			}
			lastTyped = now
			switch err := h.Chat.Typed(ctx, caller.UserID, room, now); {
			case err == nil, errors.Is(err, chat.ErrClosed):
			case ctx.Err() != nil:
				return
			case errors.Is(err, chat.ErrNoRoom), errors.Is(err, chat.ErrForbidden):
				h.closeFor(ws, err)
				return
			default:
				h.Log.Warn("chat typing", "err", err)
			}
		case t := <-subscription.typing:
			if t.User == caller.UserID {
				continue // their own (another device, or this socket's echo)
			}
			if err := write(map[string]string{"type": "typing", "userId": t.User.String(), "name": t.Name}); err != nil {
				return
			}
		}
	}
}
