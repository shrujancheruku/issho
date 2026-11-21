# email

The seam to the service that sends the product's own email ([Architecture and stack](../../../docs/spec/architecture.md#the-seams)). It is the first outside service, and the pattern for the rest: see "Outside services" in [`backend/README.md`](../../README.md).

| Name | What |
|---|---|
| `Sender` | The interface: `Send(ctx, Message) error`. Domain packages take this and nothing else |
| `Message` | One email to one person: `To` (one bare address), `Subject`, `Text`, optional `HTML`. The From address belongs to the implementation |
| `Fake` | The in-memory Sender every test uses: `NewFake()`, `Sent()` (a copy, oldest first), `FailWith(err)` |
| `NotConfigured` | The Sender a process gets when no provider key is set: every `Send` returns `ErrNotConfigured` |
| `ErrInvalidMessage` | What every Sender returns, wrapped, for a message with no valid `To`, no subject or no body. The Fake checks the same rules as the real sender |

[`resend`](resend/README.md) is the real implementation, and only `cmd/` imports it.

## In a test

```go
fake := email.NewFake()
svc := NewService(fake)              // any code that takes an email.Sender
// ... act ...
sent := fake.Sent()                  // assert on what was sent
```

A job's failure path: `fake.FailWith(errors.New("provider down"))`.
