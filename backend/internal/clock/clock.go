// Package clock is the one place the system's time is read. Code that needs
// the time takes a Clock; only cmd/ builds a Real, and tests build a Fake.
// backend/tools/clockcheck fails make lint if any other package calls
// time.Now, time.Since or time.Until.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current instant.
type Clock interface {
	Now() time.Time
}

// Real reads the system clock. It is wired only in cmd/.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fake stands still until a test moves it. It is safe for concurrent use,
// because the code under test may read it from other goroutines.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(now time.Time) *Fake { return &Fake{now: now} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set jumps to t, forwards or back.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

// Advance moves the clock on by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
