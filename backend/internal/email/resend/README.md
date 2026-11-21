# resend

The real `email.Sender`: one POST to Resend's [send-email API](https://resend.com/docs/api-reference/emails/send-email) with `Authorization: Bearer <key>`, a JSON body of `from`, `to` (one address), `subject`, `text` and `html` (omitted when empty), and an error carrying the status and Resend's own `name` and `message` for any non-2xx. The API key is never put in an error.

`resend.New(apiKey, from, opts...)` is called only in `cmd/` (`cmd/worker/email.go`); `make boundaries` fails any other importer, because this package carries the `//boundaries:outside-service` marker.

## Tests

`resend_test.go` gives the client a transport that serves a file from `testdata/` and keeps the request, so no test opens a socket. `testdata/send_ok.json` and `send_rejected.json` are the response shapes from Resend's API reference, written out by hand: no key was created or used. When the first real send is made by the founder, replace them with the recorded bodies.
