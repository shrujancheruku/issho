package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// fakeAccess is an owner's Access with the rules a test sets: who is in,
// and whether the room is open for writing.
type fakeAccess struct {
	mu     sync.Mutex
	topic  uuid.UUID
	in     map[uuid.UUID]bool
	open   bool
	opens  time.Time
	closes time.Time
}

func (f *fakeAccess) set(user uuid.UUID, in bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.in[user] = in
}

func (f *fakeAccess) setOpen(open bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open = open
}

func (f *fakeAccess) Access(_ context.Context, _ *dbgen.Queries, userID, roomID uuid.UUID, _ time.Time) (State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// The tests use the topic as the room id too.
	if roomID != f.topic {
		return State{}, ErrNoRoom
	}
	if !f.in[userID] {
		return State{}, ErrForbidden
	}
	return State{Topic: f.topic, Open: f.open, OpensAt: f.opens, ClosesAt: f.closes}, nil
}

// rooms are one room of each kind, with fake access, in a throwaway city.
type rooms struct {
	f      *testfixture.Fixture
	svc    *Service
	event  *fakeAccess
	crew   *fakeAccess
	plan   uuid.UUID
	groups map[Kind]uuid.UUID
}

func (r *rooms) room(k Kind) Room { return Room{Kind: k, ID: r.groups[k]} }

func newRooms(t *testing.T, city string) *rooms {
	t.Helper()
	f := testfixture.New(t, city, "UTC", 3)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "en")
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	var planID, eventID, groupID, crewID uuid.UUID
	if err := f.Pool.QueryRow(ctx, `insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		values ($1, $2, $3, 'en', 6, 1, current_date, 14, now() + interval '7 days') returning id`, f.City, f.Area, f.Act).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := f.Pool.QueryRow(ctx, `insert into app.events (plan_id, round, date, block, starts_at, confirm_by, forced)
		values ($1, 1, current_date, 'evening', now() + interval '1 hour', now() - interval '1 hour', false) returning id`, planID).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if err := f.Pool.QueryRow(ctx, `insert into app.event_groups (event_id, quorum, min_attendance, score, status) values ($1, 2, 2, 1, 'confirmed') returning id`, eventID).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	for _, u := range f.Users {
		exec(`insert into app.group_members (group_id, event_id, user_id, response, responded_at) values ($1, $2, $3, 'confirmed', now())`, groupID, eventID, u)
	}
	if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until, status) values ($1, $2, now(), 'active') returning id`, f.City, f.Act).Scan(&crewID); err != nil {
		t.Fatal(err)
	}
	a, b := f.Users[0], f.Users[1] // the third user is in neither room
	in := func() map[uuid.UUID]bool { return map[uuid.UUID]bool{a: true, b: true} }
	r := &rooms{
		f:      f,
		event:  &fakeAccess{topic: groupID, in: in(), open: true, opens: time.Unix(1000, 0), closes: time.Unix(2000, 0)},
		crew:   &fakeAccess{topic: crewID, in: in(), open: true},
		plan:   planID,
		groups: map[Kind]uuid.UUID{EventRoom: groupID, CrewRoom: crewID},
	}
	r.svc = &Service{DB: f.Pool, Event: r.event, Crew: r.crew}
	return r
}

func (r *rooms) access(k Kind) *fakeAccess {
	if k == EventRoom {
		return r.event
	}
	return r.crew
}

// TestIntegrationRooms: both kinds of room read, write and refuse the same
// way: only people the owner lets in, writing only while it's open, a
// cursor that returns what's newer, trimmed and limited messages.
func TestIntegrationRooms(t *testing.T) {
	r := newRooms(t, "zz-chat-rooms")
	ctx := context.Background()
	a, b, c := r.f.Users[0], r.f.Users[1], r.f.Users[2]
	now := time.Now()
	for name, kind := range map[string]Kind{"event": EventRoom, "crew": CrewRoom} {
		t.Run(name, func(t *testing.T) {
			room, acc := r.room(kind), r.access(kind)
			sent, err := r.svc.Send(ctx, a, room, "  hello  ", now)
			if err != nil || sent.Body != "hello" || !sent.Mine || sent.UserID != a || sent.ID == 0 {
				t.Fatalf("sent = %+v, %v", sent, err)
			}
			msgs, st, err := r.svc.Latest(ctx, b, room, now)
			if err != nil || len(msgs) != 1 || msgs[0].Mine || msgs[0].Name != "Test" || msgs[0].Body != "hello" || msgs[0].UserID != a || !st.Open {
				t.Fatalf("b reads %+v, %+v, %v", msgs, st, err)
			}
			if kind == EventRoom && (!st.OpensAt.Equal(acc.opens) || !st.ClosesAt.Equal(acc.closes)) {
				t.Errorf("state = %+v, want the owner's times", st)
			}
			second, err := r.svc.Send(ctx, b, room, "second", now)
			if err != nil {
				t.Fatal(err)
			}
			if newer, _, err := r.svc.After(ctx, a, room, sent.ID, now); err != nil || len(newer) != 1 || newer[0].ID != second.ID {
				t.Errorf("after the first = %+v, %v", newer, err)
			}
			for what, body := range map[string]string{"empty": "  ", "too long": strings.Repeat("あ", MaxMessage+1)} {
				if _, err := r.svc.Send(ctx, a, room, body, now); !errors.Is(err, ErrBadMessage) {
					t.Errorf("%s message: %v, want ErrBadMessage", what, err)
				}
			}
			if _, err := r.svc.Send(ctx, a, room, strings.Repeat("あ", MaxMessage), now); err != nil {
				t.Errorf("a message of exactly %d characters: %v", MaxMessage, err)
			}

			// Someone the owner doesn't let in can't read, write or type.
			if _, _, err := r.svc.Latest(ctx, c, room, now); !errors.Is(err, ErrForbidden) {
				t.Errorf("outsider reads: %v, want ErrForbidden", err)
			}
			if _, err := r.svc.Send(ctx, c, room, "hi", now); !errors.Is(err, ErrForbidden) {
				t.Errorf("outsider sends: %v, want ErrForbidden", err)
			}
			if err := r.svc.Typed(ctx, c, room, now); !errors.Is(err, ErrForbidden) {
				t.Errorf("outsider types: %v, want ErrForbidden", err)
			}
			if _, _, err := r.svc.Latest(ctx, a, Room{Kind: kind, ID: uuid.New()}, now); !errors.Is(err, ErrNoRoom) {
				t.Errorf("unknown room: %v, want ErrNoRoom", err)
			}

			// Closed: readable, not writable, and no typing notices.
			acc.setOpen(false)
			if _, err := r.svc.Send(ctx, a, room, "late", now); !errors.Is(err, ErrClosed) {
				t.Errorf("closed room: %v, want ErrClosed", err)
			}
			if err := r.svc.Typed(ctx, a, room, now); !errors.Is(err, ErrClosed) {
				t.Errorf("typing in a closed room: %v, want ErrClosed", err)
			}
			if msgs, st, err := r.svc.Latest(ctx, a, room, now); err != nil || st.Open || len(msgs) != 3 {
				t.Errorf("reading a closed room: %d messages, %+v, %v", len(msgs), st, err)
			}
			acc.setOpen(true)

			// Taken out mid-conversation: the next read or write fails.
			acc.set(b, false)
			if _, _, err := r.svc.After(ctx, b, room, second.ID, now); !errors.Is(err, ErrForbidden) {
				t.Errorf("after being removed: %v, want ErrForbidden", err)
			}
			if _, err := r.svc.Send(ctx, b, room, "still here?", now); !errors.Is(err, ErrForbidden) {
				t.Errorf("sending after being removed: %v, want ErrForbidden", err)
			}
			acc.set(b, true)
		})
	}
}

// TestIntegrationDrain: a room with more than a page of new messages is read
// a page at a time from the cursor, in order, with none repeated or skipped.
func TestIntegrationDrain(t *testing.T) {
	r := newRooms(t, "zz-chat-drain")
	ctx := context.Background()
	a := r.f.Users[0]
	now := time.Now()
	const n = 2*Page + 37
	if _, err := r.f.Pool.Exec(ctx, `insert into app.chat_messages (group_id, user_id, body) select $1, $2, 'm' || g from generate_series(1, $3) g`,
		r.groups[EventRoom], a, n); err != nil {
		t.Fatal(err)
	}
	if _, err := r.f.Pool.Exec(ctx, `insert into app.crew_messages (crew_id, user_id, body) select $1, $2, 'm' || g from generate_series(1, $3) g`,
		r.groups[CrewRoom], a, n); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []Kind{EventRoom, CrewRoom} {
		room := r.room(kind)
		latest, _, err := r.svc.Latest(ctx, a, room, now)
		if err != nil || len(latest) != Page || latest[Page-1].Body != "m"+itoa(n) {
			t.Fatalf("kind %d: latest = %d messages, %v", kind, len(latest), err)
		}
		var got []Message
		var last int64
		var pages []int
		for {
			ms, _, err := r.svc.After(ctx, a, room, last, now)
			if err != nil {
				t.Fatal(err)
			}
			got, pages = append(got, ms...), append(pages, len(ms))
			if len(ms) < Page {
				break
			}
			last = ms[len(ms)-1].ID
		}
		if len(got) != n || len(pages) != 3 || pages[2] != 37 {
			t.Fatalf("kind %d: drained %d messages in pages %v", kind, len(got), pages)
		}
		for i, m := range got {
			if m.Body != "m"+itoa(i+1) {
				t.Fatalf("kind %d: message %d is %q", kind, i, m.Body)
			}
		}
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// TestIntegrationPostsWaitForTheRoomLock: a poster holding a room's lock
// keeps another from inserting until it commits, so message ids commit in
// order. Without the lock, the second insert would take a higher id and
// could commit first (the first one is still open), and a reader that had
// moved its cursor past the higher id would never see the lower one.
func TestIntegrationPostsWaitForTheRoomLock(t *testing.T) {
	r := newRooms(t, "zz-chat-lock")
	ctx := context.Background()
	a := r.f.Users[0]
	now := time.Now()
	for _, kind := range []Kind{EventRoom, CrewRoom} {
		room := r.room(kind)
		// A slow poster: it has the lock and has inserted, but not committed.
		tx, err := r.f.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op once committed; frees the lock if the test fails
		q := dbgen.New(tx)
		if err := lock(ctx, q, kind, room.ID); err != nil {
			t.Fatal(err)
		}
		var firstID int64
		if kind == EventRoom {
			row, err := q.InsertChatMessage(ctx, dbgen.InsertChatMessageParams{GroupID: room.ID, UserID: a, Body: "first"})
			if err != nil {
				t.Fatal(err)
			}
			firstID = row.ID
		} else {
			row, err := q.InsertCrewMessage(ctx, dbgen.InsertCrewMessageParams{CrewID: room.ID, UserID: pgtype.UUID{Bytes: a, Valid: true}, Body: "first"})
			if err != nil {
				t.Fatal(err)
			}
			firstID = row.ID
		}

		type result struct {
			m   Message
			err error
		}
		done := make(chan result, 1)
		go func() {
			m, err := r.svc.Send(ctx, a, room, "second", now)
			done <- result{m, err}
		}()
		select {
		case res := <-done:
			t.Fatalf("kind %d: a second post finished while the first held the room's lock: %+v", kind, res)
		case <-time.After(300 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		res := <-done
		if res.err != nil || res.m.ID <= firstID {
			t.Fatalf("kind %d: second post = %+v, %v; want an id after %d", kind, res.m, res.err, firstID)
		}
		if ms, _, err := r.svc.After(ctx, a, room, firstID-1, now); err != nil || len(ms) != 2 || ms[0].ID != firstID || ms[1].ID != res.m.ID {
			t.Errorf("kind %d: messages from the first = %+v, %v", kind, ms, err)
		}
	}

	// A plan suggested in a crew's chat waits for the same lock.
	tx, err := r.f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // as above
	if err := lock(ctx, dbgen.New(tx), CrewRoom, r.groups[CrewRoom]); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		ptx, err := r.f.Pool.Begin(ctx)
		if err == nil {
			err = PostPlan(ctx, dbgen.New(ptx), r.groups[CrewRoom], r.plan, now)
		}
		if err == nil {
			err = ptx.Commit(ctx)
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a suggested plan was stored while a poster held the lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ms, _, err := r.svc.Latest(ctx, a, r.room(CrewRoom), now)
	if err != nil || len(ms) == 0 || ms[len(ms)-1].PlanID != r.plan || ms[len(ms)-1].UserID != uuid.Nil || ms[len(ms)-1].Mine {
		t.Errorf("crew chat after the suggestion = %+v, %v", ms, err)
	}
}

// TestIntegrationConcurrentPostsAreAllRead: many people posting at once
// while a reader follows its cursor never misses one. (Without the room
// lock this fails only now and then; TestIntegrationPostsWaitForTheRoomLock
// is the deterministic check.)
func TestIntegrationConcurrentPostsAreAllRead(t *testing.T) {
	r := newRooms(t, "zz-chat-race")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, b := r.f.Users[0], r.f.Users[1]
	now := time.Now()
	const posters, each = 6, 25
	for _, kind := range []Kind{EventRoom, CrewRoom} {
		room := r.room(kind)
		var wg sync.WaitGroup
		for p := range posters {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range each {
					who := []uuid.UUID{a, b}[p%2]
					if _, err := r.svc.Send(ctx, who, room, "p"+itoa(p)+"-"+itoa(i), now); err != nil {
						t.Error(err)
						return
					}
				}
			}()
		}
		seen := map[int64]bool{}
		var last int64
		finished := make(chan struct{})
		go func() { wg.Wait(); close(finished) }()
		drained := false
		for len(seen) < posters*each && ctx.Err() == nil {
			ms, _, err := r.svc.After(ctx, a, room, last, now)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range ms {
				seen[m.ID] = true
				last = m.ID
			}
			if len(ms) == 0 {
				select {
				case <-finished:
					if drained {
						t.Fatalf("kind %d: read %d of %d messages after every post finished", kind, len(seen), posters*each)
					}
					drained = true
				default:
				}
			}
		}
		wg.Wait()
		if len(seen) != posters*each {
			t.Errorf("kind %d: read %d of %d messages", kind, len(seen), posters*each)
		}
	}
}

// TestIntegrationNotifies: posting and typing reach the listeners of other
// API instances.
func TestIntegrationNotifies(t *testing.T) {
	r := newRooms(t, "zz-chat-notify")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	a := r.f.Users[0]
	now := time.Now()
	conn, err := r.f.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, ch := range []string{"chat", "chat_typing"} {
		if _, err := conn.Exec(ctx, "listen "+ch); err != nil {
			t.Fatal(err)
		}
	}
	defer conn.Exec(context.Background(), "unlisten *") //nolint:errcheck // the connection goes back to the pool
	for _, kind := range []Kind{EventRoom, CrewRoom} {
		room := r.room(kind)
		if _, err := r.svc.Send(ctx, a, room, "hi", now); err != nil {
			t.Fatal(err)
		}
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil || n.Channel != "chat" || n.Payload != room.ID.String() {
			t.Fatalf("kind %d: notification = %+v, %v; want the room's topic", kind, n, err)
		}
		if err := r.svc.Typed(ctx, a, room, now); err != nil {
			t.Fatal(err)
		}
		n, err = conn.Conn().WaitForNotification(ctx)
		var typed Typing
		if err != nil || n.Channel != "chat_typing" || json.Unmarshal([]byte(n.Payload), &typed) != nil ||
			typed.Topic != room.ID || typed.User != a || typed.Name != "Test" {
			t.Fatalf("kind %d: typing notification = %+v, %v", kind, n, err)
		}
	}
}
