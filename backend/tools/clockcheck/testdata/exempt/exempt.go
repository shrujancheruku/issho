// Package exempt stands for internal/clock: the check skips the directory it
// is told is the clock's own.
package exempt

import "time"

func Now() time.Time { return time.Now() }
