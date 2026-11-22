// Package testdb gives each test its own schema in a throwaway Postgres.
//
// The first New in a test binary starts one Postgres container through Docker;
// with Docker stopped it fails the test. There is no flag or environment
// variable that skips the database (docs/spec/quality.md#backend).
package testdb

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	once   sync.Once
	dsn    string
	dsnErr error
	seq    atomic.Int64
)

// start boots the shared container. Testcontainers' reaper removes it when the
// test binary exits, so a crashed run leaves nothing behind.
func start() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Testcontainers falls back to other Docker sockets when DOCKER_HOST is
	// unreachable; ping the daemon the environment names so a stopped or
	// unreachable Docker fails here rather than finding another one.
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("docker client: %w", err)
	}
	defer dc.Close()
	if _, err := dc.Ping(ctx, client.PingOptions{}); err != nil {
		return "", fmt.Errorf("Docker is not reachable (is it running?): %w", err)
	}

	c, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("issho_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		return "", fmt.Errorf("start Postgres (is Docker running?): %w", err)
	}
	return c.ConnectionString(ctx, "sslmode=disable")
}

// New returns a pool whose search_path is a fresh schema owned by t. The schema
// is dropped when the test ends.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	once.Do(func() { dsn, dsnErr = start() })
	if dsnErr != nil {
		t.Fatal(dsnErr)
	}
	ctx := context.Background()
	schema := fmt.Sprintf("t%d", seq.Add(1))

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "drop schema "+schema+" cascade")
		admin.Close()
	})
	return pool
}

// NewDatabase returns a pool on a database of its own, dropped when the test
// ends. A schema per test (New) can't hold what the migrations create, since
// they use fixed schema names (app, river, goose).
func NewDatabase(t testing.TB) *pgxpool.Pool {
	t.Helper()
	once.Do(func() { dsn, dsnErr = start() })
	if dsnErr != nil {
		t.Fatal(dsnErr)
	}
	ctx := context.Background()
	name := fmt.Sprintf("d%d", seq.Add(1))

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "create database "+name); err != nil {
		admin.Close()
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "drop database "+name+" with (force)")
		admin.Close()
	})
	return pool
}

// URL returns a connection string for the database of a pool from NewDatabase,
// for a test that starts a process which connects by itself.
func URL(pool *pgxpool.Pool) string {
	c := pool.Config().ConnConfig
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))),
		Path:     "/" + c.Database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
