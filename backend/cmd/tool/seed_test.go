package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/cities"
)

func TestApplyPacks(t *testing.T) {
	packs := []cities.Pack{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	boom := errors.New("database down")

	run := func(failures map[string]error) ([]string, error) {
		var applied []string
		err := applyPacks(context.Background(), packs, false, func(_ context.Context, p cities.Pack) (cities.Diff, error) {
			applied = append(applied, p.ID)
			return cities.Diff{NewAreas: []string{p.ID + "-x"}}, failures[p.ID]
		})
		return applied, err
	}

	// A travel-time failure leaves the pack seeded: carry on with the rest.
	applied, err := run(map[string]error{"a": fmt.Errorf("%w: quota", cities.ErrTravelTimes)})
	if err != nil || len(applied) != 3 {
		t.Errorf("ErrTravelTimes: applied %v, err %v; want all 3 and no error", applied, err)
	}

	// Any other error stops the run and is returned.
	applied, err = run(map[string]error{"b": boom})
	if !errors.Is(err, boom) || len(applied) != 2 {
		t.Errorf("other error: applied %v, err %v; want 2 applied and the error", applied, err)
	}
}
