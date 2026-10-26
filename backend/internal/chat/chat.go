// Package chat stores and limits the app's two chats: an event group's and
// a crew's. Who may read or write each is decided by its owner
// (plans for an event's group, crews for a crew); this package only checks
// messages and reads and writes them. A message posted to either NOTIFYs
// the "chat" channel with the group or crew id, which internal/chatws
// relays to open chats.
package chat

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

const (
	// OpensBefore and ClosesAfter set when a group can write in its chat,
	// around the event's start: time to plan the meeting, and for "running
	// late" and "thanks" after. It stays readable afterwards until the
	// messages are deleted (Keep).
	OpensBefore = 48 * time.Hour
	ClosesAfter = 24 * time.Hour
	// Keep is how long after the event its group's messages are kept.
	Keep = 30 * 24 * time.Hour
	// MaxMessage is the longest message, in characters.
	MaxMessage = 1000
	// Page is how many messages one read returns at most.
	Page = 100
)

// Message is one message as the caller sees it.
type Message struct {
	ID   int64
	Name string // the sender's first name
	Body string
	At   time.Time
	Mine bool
}

// Clean trims a message and reports whether it's one that can be sent:
// 1 to MaxMessage characters of valid UTF-8.
func Clean(body string) (string, bool) {
	body = strings.TrimSpace(body)
	return body, body != "" && utf8.RuneCountInString(body) <= MaxMessage && utf8.ValidString(body)
}

// ListGroup returns an event group's messages after afterID (0 = the
// latest Page), oldest first.
func ListGroup(ctx context.Context, q *dbgen.Queries, groupID, userID uuid.UUID, afterID int64) ([]Message, error) {
	var rows []dbgen.ListChatMessagesRow
	if afterID == 0 {
		rs, err := q.ListLatestChatMessages(ctx, dbgen.ListLatestChatMessagesParams{GroupID: groupID, Max: Page})
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			rows = append(rows, dbgen.ListChatMessagesRow(r))
		}
	} else {
		var err error
		if rows, err = q.ListChatMessages(ctx, dbgen.ListChatMessagesParams{GroupID: groupID, AfterID: afterID, Max: Page}); err != nil {
			return nil, err
		}
	}
	out := make([]Message, len(rows))
	for i, r := range rows {
		out[i] = Message{ID: r.ID, Body: r.Body, At: r.CreatedAt.Time, Mine: r.UserID == userID}
		if r.DisplayName != nil {
			out[i].Name = *r.DisplayName
		}
	}
	return out, nil
}

// PostGroup stores a cleaned message in an event group's chat and tells
// every API instance. Run it in the transaction that checked access.
func PostGroup(ctx context.Context, q *dbgen.Queries, groupID, userID uuid.UUID, body string) (Message, error) {
	r, err := q.InsertChatMessage(ctx, dbgen.InsertChatMessageParams{GroupID: groupID, UserID: userID, Body: body})
	if err != nil {
		return Message{}, err
	}
	return Message{ID: r.ID, Body: body, At: r.CreatedAt.Time, Mine: true}, q.NotifyChat(ctx, groupID.String())
}

// DeleteOld deletes the group chats of a city's events that ended more
// than Keep ago. Run by the worker's settle pass.
func DeleteOld(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time) (int64, error) {
	return q.DeleteOldChats(ctx, dbgen.DeleteOldChatsParams{CityID: cityID, Before: pgtype.Date{Time: now.Add(-Keep), Valid: true}})
}
