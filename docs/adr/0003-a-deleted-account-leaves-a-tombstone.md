# A deleted account leaves a tombstone

Deleting an account erases the person at once: the account, the profile, signups and picks. Without something more, a person who was suspended, warned, reported or kept apart from someone could delete their account and join again with the same email as if nothing had happened, and the people who reported them would be grouped with them again. So a deleted account leaves a **tombstone**: a keyed hash (HMAC) of the normalized email, holding only the safety records below. A later account whose email matches inherits them. An account with none of these records leaves nothing.

| Part | Kept |
|---|---|
| A warning, or a suspension that was lifted | 12 months from the act |
| A suspension still in force | While it is in force |
| A report about them (an open one is still reviewed) | 12 months after it was closed |
| A kept-apart pair where they were the one reported or not wanted | While the other person has an account |

The tombstone disappears when it holds nothing. The rules are in [Safety](../spec/safety.md#deleting-an-account) and [Privacy](../spec/privacy.md#the-tombstone).

This keeps a small amount of personal data after the person has asked for erasure, which is a trade-off under Vietnam's PDPL. We judged it justified for safety: no email in the clear, only records another person's safety depends on, and each with a lifetime. The privacy policy says what is kept, why and for how long, and whether it stands as an exception to erasure is a question for the lawyer before the public launch ([ADR 0004](0004-the-owner-operates-issho-until-the-public-launch.md)).

## Considered options

- **Keep nothing**, and rely on join links and the waitlist to keep a suspended person out. Rejected: both are keyed by email too, so the same person could be let in again with nothing to warn the moderator, and from the public launch there is no gate at all.
- **Keep only a suspension in force** (v1's version). Rejected: a kept-apart pair exists to protect the other person, and it must not end because the person it guards against deleted and came back.
- **Soft-delete the whole account.** Rejected: it keeps far more than safety needs, against the point of deletion.
- **Refuse the sign-up outright.** Rejected: it needs its own error path and screen, and a moderator could no longer lift a mistaken suspension the usual way.

## Consequences

- The HMAC key lives only in the deployment's secret store and is never rotated: if it is lost or changed, existing tombstones stop matching.
- A different email gets around it. Issho verifies no identity before the public launch, so this is accepted until then.
- A restore from backup must not bring a deleted account back: the **deletion ledger** is replayed after a restore ([Technical operations](../spec/operations.md#backups-and-the-restore)).
