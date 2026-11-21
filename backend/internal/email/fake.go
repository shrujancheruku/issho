package email

import (
	"context"
	"slices"
	"sync"
)

// Fake is the one in-memory Sender every test uses. It records what was sent,
// in order, so a test asserts on email through the same interface the code
// under test used. It is safe for concurrent use: jobs send from goroutines.
type Fake struct {
	mu   sync.Mutex
	sent []Message
	err  error
}

// NewFake returns a Fake that has sent nothing and succeeds.
func NewFake() *Fake { return &Fake{} }

// Send records m, unless it is invalid or FailWith has been called, in which
// case it records nothing and returns the error.
func (f *Fake) Send(_ context.Context, m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	f.sent = append(f.sent, m)
	return nil
}

// Sent returns a copy of the messages sent so far, oldest first.
func (f *Fake) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

// FailWith makes every later Send return err, to test a caller's handling of a
// provider that is down. FailWith(nil) restores success.
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}
