package plans

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// fixture is testfixture.Fixture with the short names this package's tests
// use.
type fixture struct {
	pool            *pgxpool.Pool
	q               *dbgen.Queries
	city, area, act string
	users           []uuid.UUID
	tf              *testfixture.Fixture
}

func newFixture(t *testing.T, id, timezone string, n int) *fixture {
	t.Helper()
	tf := testfixture.New(t, id, timezone, n)
	return &fixture{pool: tf.Pool, q: tf.Q, city: tf.City, area: tf.Area, act: tf.Act, users: tf.Users, tf: tf}
}

func (f *fixture) addProfile(t *testing.T, id uuid.UUID, lang string) {
	t.Helper()
	f.tf.AddProfile(t, id, lang)
}
