package staff

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/staff/store"
)

// State is where an item stands.
type State string

const (
	// StateOpen: waiting for a person.
	StateOpen State = "open"
	// StateResolved: a person acted, and gave a reason.
	StateResolved State = "resolved"
	// StateDefaulted: the item fell due and its kind's written default was applied.
	StateDefaulted State = "defaulted"
)

// Item is one by-hand step in the queue.
type Item struct {
	ID int64
	// Kind names the step; the package that opens it registers it (Kinds).
	Kind string
	// Subject is what the step is about, in the opening package's terms. staff
	// never reads it.
	Subject string
	DueAt   time.Time
	State   State
	// Reason is why a person resolved the item; empty unless StateResolved.
	Reason    string
	CreatedAt time.Time
	// ClosedAt is when it was resolved or defaulted; nil while open.
	ClosedAt *time.Time
}

func itemOf(r store.AppQueueItem) Item {
	it := Item{
		ID: r.ID, Kind: r.Kind, Subject: r.Subject, DueAt: r.DueAt,
		State: State(r.State), CreatedAt: r.CreatedAt, ClosedAt: r.ClosedAt,
	}
	if r.Reason != nil {
		it.Reason = *r.Reason
	}
	return it
}

// Default applies a kind's written default to an item that fell due. It runs in
// the transaction that marks the item defaulted: if it returns an error both
// roll back, the item stays open, and River retries the job.
type Default func(ctx context.Context, tx pgx.Tx, item Item) error

// Kinds is the registry of the kinds of item and the default each applies.
// A package that opens items registers its kinds at startup, so staff imports
// no domain package. It is safe for concurrent use.
type Kinds struct {
	mu       sync.RWMutex
	defaults map[string]Default
}

func NewKinds() *Kinds { return &Kinds{defaults: map[string]Default{}} }

// Register names a kind and the function that applies its default. Registering
// a kind twice, with no name or with no default is a bug in the caller's
// wiring, so it panics at startup rather than failing when an item falls due.
func (k *Kinds) Register(kind string, apply Default) {
	if kind == "" || apply == nil {
		panic("staff: a kind needs a name and a default")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, dup := k.defaults[kind]; dup {
		panic(fmt.Sprintf("staff: kind %q is registered twice", kind))
	}
	k.defaults[kind] = apply
}

func (k *Kinds) lookup(kind string) (Default, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	apply, ok := k.defaults[kind]
	return apply, ok
}

// Inserter enqueues a River job in a transaction. *river.Client[pgx.Tx]
// satisfies it, and an insert-only client (no workers) is enough, which is what
// the API process has.
type Inserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Queue is the exported API of the queue: packages open items on it, staff
// resolves them, and the due job (AddWorker) applies the defaults.
type Queue struct {
	clock    clock.Clock
	kinds    *Kinds
	inserter Inserter
}

// NewQueue returns a Queue that stamps times from clk, accepts only the kinds
// registered on kinds, and enqueues each item's due job through inserter.
func NewQueue(clk clock.Clock, kinds *Kinds, inserter Inserter) *Queue {
	return &Queue{clock: clk, kinds: kinds, inserter: inserter}
}

// ErrUnknownKind is returned by Open for a kind no package registered: its
// default could never be applied, so the item would sit past due.
var ErrUnknownKind = errors.New("staff: kind is not registered")

// OpenParams is what a package gives the queue to open an item.
type OpenParams struct {
	Kind    string
	Subject string
	DueAt   time.Time
}

// Open adds an item to the queue and enqueues the job that applies its default
// at p.DueAt, both in tx: if the caller's transaction rolls back, neither
// exists.
func (q *Queue) Open(ctx context.Context, tx pgx.Tx, p OpenParams) (Item, error) {
	if _, ok := q.kinds.lookup(p.Kind); !ok {
		return Item{}, fmt.Errorf("%w: %q", ErrUnknownKind, p.Kind)
	}
	row, err := store.New(tx).InsertQueueItem(ctx, store.InsertQueueItemParams{
		Kind: p.Kind, Subject: p.Subject, DueAt: p.DueAt, CreatedAt: q.clock.Now(),
	})
	if err != nil {
		return Item{}, fmt.Errorf("insert queue item: %w", err)
	}
	if _, err := q.inserter.InsertTx(ctx, tx, DueArgs{ItemID: row.ID}, &river.InsertOpts{ScheduledAt: p.DueAt}); err != nil {
		return Item{}, fmt.Errorf("enqueue due job: %w", err)
	}
	return itemOf(row), nil
}

// ListOpen returns the open items, soonest due first. db is a pool or a transaction.
func (q *Queue) ListOpen(ctx context.Context, db store.DBTX) ([]Item, error) {
	rows, err := store.New(db).ListOpenQueueItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("list open queue items: %w", err)
	}
	items := make([]Item, len(rows))
	for i, r := range rows {
		items[i] = itemOf(r)
	}
	return items, nil
}

var (
	// ErrReasonRequired is returned by Resolve when the reason is empty.
	ErrReasonRequired = errors.New("staff: a resolution needs a reason")
	// ErrNotOpen is returned when the item doesn't exist or is already resolved
	// or defaulted.
	ErrNotOpen = errors.New("staff: no such open item")
)

// Resolve closes an open item with the reason a person gives, in tx. The item's
// due job stays enqueued and finds it closed when it runs.
func (q *Queue) Resolve(ctx context.Context, tx pgx.Tx, id int64, reason string) (Item, error) {
	if reason == "" {
		return Item{}, ErrReasonRequired
	}
	now := q.clock.Now()
	row, err := store.New(tx).ResolveQueueItem(ctx, store.ResolveQueueItemParams{ID: id, Reason: &reason, ClosedAt: &now})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotOpen
	}
	if err != nil {
		return Item{}, fmt.Errorf("resolve queue item: %w", err)
	}
	return itemOf(row), nil
}

// DueArgs is the due job's arguments: the item it applies the default to.
type DueArgs struct {
	ItemID int64 `json:"item_id"`
}

func (DueArgs) Kind() string { return "staff_queue_item_due" }
