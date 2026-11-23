package testdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Barrier bounds. A barrier gives up after polls×pollEvery (10s) in all, and
// each poll's query, connection acquire included, after pollTimeout: a pool
// with no connection to spare fails the test in seconds, naming what it waited
// for, instead of hanging until go test's own timeout. The bounds are counted
// polls and a context deadline because only internal/clock reads the wall clock.
const (
	polls       = 1000
	pollEvery   = 10 * time.Millisecond
	pollTimeout = 2 * time.Second
)

// WaitUntil polls query, which selects one boolean, on pool until it is true,
// so a concurrency test knows its calls overlap. what names the wait in a
// failure.
func WaitUntil(t testing.TB, pool *pgxpool.Pool, what, query string, args ...any) {
	t.Helper()
	for i := 0; ; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
		var ok bool
		err := pool.QueryRow(ctx, query, args...).Scan(&ok)
		cancel()
		if err != nil {
			st := pool.Stat()
			t.Fatalf("waiting for %s: poll failed after up to %s (pool %d/%d connections in use): %v",
				what, pollTimeout, st.AcquiredConns(), st.MaxConns(), err)
		}
		if ok {
			return
		}
		if i >= polls {
			t.Fatalf("waited %s for %s; it never happened", polls*pollEvery, what)
		}
		time.Sleep(pollEvery)
	}
}

// WaitForLockWaiters waits until n sessions on pool's database are blocked on
// a lock while running the named sqlc query.
func WaitForLockWaiters(t testing.TB, pool *pgxpool.Pool, query string, n int) {
	t.Helper()
	WaitUntil(t, pool, fmt.Sprintf("%d calls to wait on a lock in %s", n, query),
		`select count(*) >= $2 from pg_stat_activity
		 where datname = current_database() and wait_event_type = 'Lock' and query like '-- name: ' || $1 || ' %'`,
		query, n)
}
