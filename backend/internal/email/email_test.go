package email_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/email"
)

// A domain package's tests hand the fake to the code under test as a Sender and
// then read back what was sent.
func TestFakeRecordsWhatWasSent(t *testing.T) {
	fake := email.NewFake()
	var sender email.Sender = fake

	first := email.Message{To: "a@example.com", Subject: "Your code", Text: "123456"}
	second := email.Message{To: "b@example.com", Subject: "Your place is open", Text: "Hi", HTML: "<p>Hi</p>"}
	for _, m := range []email.Message{first, second} {
		if err := sender.Send(context.Background(), m); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	got := fake.Sent()
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("Sent() = %+v, want [%+v %+v] in order", got, first, second)
	}
}

func TestFakeSentIsACopy(t *testing.T) {
	fake := email.NewFake()
	_ = fake.Send(context.Background(), email.Message{To: "a@example.com", Subject: "s", Text: "t"})

	fake.Sent()[0].Subject = "changed"

	if got := fake.Sent()[0].Subject; got != "s" {
		t.Errorf("a caller changed the record: subject = %q", got)
	}
}

// A message the real sender would refuse is refused by the fake too, so a test
// can't pass on a message production would reject.
func TestFakeRefusesAnInvalidMessage(t *testing.T) {
	cases := map[string]email.Message{
		"no recipient": {Subject: "s", Text: "t"},
		"no subject":   {To: "a@example.com", Text: "t"},
		"no body":      {To: "a@example.com", Subject: "s"},
		"two people":   {To: "a@example.com, b@example.com", Subject: "s", Text: "t"},
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			fake := email.NewFake()
			err := fake.Send(context.Background(), m)
			if !errors.Is(err, email.ErrInvalidMessage) {
				t.Fatalf("Send = %v, want ErrInvalidMessage", err)
			}
			if n := len(fake.Sent()); n != 0 {
				t.Errorf("recorded %d messages for an invalid one", n)
			}
		})
	}
}

func TestFakeFailsWhenToldTo(t *testing.T) {
	fake := email.NewFake()
	boom := errors.New("provider down")
	fake.FailWith(boom)

	err := fake.Send(context.Background(), email.Message{To: "a@example.com", Subject: "s", Text: "t"})

	if !errors.Is(err, boom) {
		t.Fatalf("Send = %v, want the injected error", err)
	}
	if n := len(fake.Sent()); n != 0 {
		t.Errorf("recorded %d messages for a failed send", n)
	}
}

func TestNotConfiguredRefusesEverySend(t *testing.T) {
	var sender email.Sender = email.NotConfigured{}
	err := sender.Send(context.Background(), email.Message{To: "a@example.com", Subject: "s", Text: "t"})
	if !errors.Is(err, email.ErrNotConfigured) {
		t.Fatalf("Send = %v, want ErrNotConfigured", err)
	}
}
