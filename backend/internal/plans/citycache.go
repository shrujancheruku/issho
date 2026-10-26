package plans

import (
	"context"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// cityTTL bounds how long the feed and plan titles can lag behind a change
// to a city's settings, areas, activities or venues (an admin's edit, a
// re-seeded city pack). They change rarely, and loading them is six round
// trips to a database that's a long way from the API.
const cityTTL = time.Minute

// cityCache keeps, per city, what loadPlaces reads (any month) and the
// activities' English names. Entries are shared between requests and must
// not be modified.
type cityCache struct {
	mu      sync.Mutex
	entries map[string]cityEntry
}

type cityEntry struct {
	city   *City             // nil until the feed needs it
	names  map[string]string // activity id -> English name; nil until titles need them
	loaded time.Time         // of the older part present
}

func (c *cityCache) get(cityID string) cityEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[cityID]
	if time.Since(e.loaded) >= cityTTL {
		return cityEntry{}
	}
	return e
}

// put stores what was just loaded, keeping the other part if it's still
// fresh. Loads happen outside the lock: two requests may load the same
// city at once, which is harmless.
func (c *cityCache) put(cityID string, update func(*cityEntry)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cityEntry{}
	}
	e := c.entries[cityID]
	now := time.Now()
	if now.Sub(e.loaded) >= cityTTL {
		e = cityEntry{loaded: now}
	}
	update(&e)
	c.entries[cityID] = e
}

// places is loadPlaces(cityID, any month), cached.
func (s *Service) places(ctx context.Context, q *dbgen.Queries, cityID string) (*City, error) {
	if c := s.cities.get(cityID).city; c != nil {
		return c, nil
	}
	c, err := loadPlaces(ctx, q, cityID, 0)
	if err != nil {
		return nil, err
	}
	s.cities.put(cityID, func(e *cityEntry) { e.city = c })
	return c, nil
}

// activityNames maps a city's activity ids to their English names, cached.
func (s *Service) activityNames(ctx context.Context, q *dbgen.Queries, cityID string) (map[string]string, error) {
	if n := s.cities.get(cityID).names; n != nil {
		return n, nil
	}
	rows, err := q.ListCityActivities(ctx, cityID)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(rows))
	for _, r := range rows {
		names[r.ID] = i18n.Pick(r.Name, i18n.Authoring)
	}
	s.cities.put(cityID, func(e *cityEntry) { e.names = names })
	return names, nil
}
