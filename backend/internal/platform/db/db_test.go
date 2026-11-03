package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

func TestOpenRejectsAnUnparsableURL(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user@host:notaport/db")
	if err == nil || !strings.Contains(err.Error(), "parse database url") {
		t.Fatalf("err = %v, want a parse error", err)
	}
}

// Without a database the pool must not be handed out: Open pings first, so
// the API and worker exit at startup instead of failing on the first request.
func TestOpenFailsWithoutADatabase(t *testing.T) {
	// Port 1 refuses connections at once; the short deadline bounds the test
	// if something answers anyway.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, "postgres://u:p@127.0.0.1:1/none?sslmode=disable&connect_timeout=2")
	if err == nil {
		pool.Close()
		t.Fatal("Open returned a pool for a database that is not there")
	}
	if pool != nil {
		t.Errorf("pool = %v with an error, want nil", pool)
	}
	if !strings.Contains(err.Error(), "ping database") {
		t.Errorf("err = %v, want it to say the ping failed", err)
	}
}

func TestIntegrationOpenConfiguresThePool(t *testing.T) {
	pool, err := Open(context.Background(), testfixture.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := pool.Config()
	if cfg.MinConns != minConns {
		t.Errorf("MinConns = %d, want %d", cfg.MinConns, minConns)
	}
	if _, ok := cfg.ConnConfig.Tracer.(sentry.DBTracer); !ok {
		t.Errorf("tracer = %T, want sentry.DBTracer", cfg.ConnConfig.Tracer)
	}
	// A connection used within the last minute is trusted; an older one is pinged.
	ping := func(idle time.Duration) bool {
		return cfg.ShouldPing(context.Background(), pgxpool.ShouldPingParams{IdleDuration: idle})
	}
	if ping(pingAfterIdle) || ping(5*time.Second) {
		t.Error("a recently used connection is pinged again")
	}
	if !ping(pingAfterIdle + time.Second) {
		t.Error("a connection idle for over a minute is not pinged")
	}
	var one int
	if err := pool.QueryRow(context.Background(), "select 1").Scan(&one); err != nil || one != 1 {
		t.Errorf("select 1 = %d, %v", one, err)
	}
}

func TestIntegrationMigrateRejectsAnUnknownCommand(t *testing.T) {
	if err := Migrate(context.Background(), testfixture.Pool(t), "no-such-command"); err == nil {
		t.Error("an unknown goose command succeeded")
	}
}
