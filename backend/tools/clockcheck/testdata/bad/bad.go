// Package bad is a fixture for clockcheck: every function reads the system clock.
package bad

import (
	"time"
	t2 "time"
)

func Now() time.Time { return time.Now() }

func Since(t time.Time) time.Duration { return time.Since(t) }

func Until(t time.Time) time.Duration { return time.Until(t) }

// Aliased import, and the function taken as a value rather than called.
func Aliased() func() time.Time { return t2.Now }
