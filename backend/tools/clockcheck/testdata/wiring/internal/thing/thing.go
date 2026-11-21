// Package thing builds the real clock itself instead of taking one.
package thing

import (
	"github.com/shrujancheruku/issho/backend/internal/clock"
	c2 "github.com/shrujancheruku/issho/backend/internal/clock"
)

func New() clock.Clock { return clock.Real{} }

func Aliased() c2.Clock { return c2.Real{} }

// Taking a clock.Clock and using the fake are fine.
func Uses(c clock.Clock) { _ = c.Now(); _ = clock.NewFake(c.Now()) }
