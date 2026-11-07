# Deleted suspended accounts leave a tombstone

Deleting an account erases everything about the person, including their suspension and the reports against them (`on delete cascade` on `app.suspensions` and `app.reports`). Before this decision, a suspended person could delete their account and sign up again with the same email as if nothing had happened. So when someone with an active suspension deletes their account, we keep a tombstone: an HMAC of their normalized email, plus the suspension and its reasons. Nothing else is kept, and nothing at all is kept for someone who isn't suspended. A later sign-up whose email matches a tombstone gets an account that is already suspended with the original reasons. They see the usual suspended screen, and a moderator can lift the suspension as usual. Standing ([#402](https://github.com/shrujancheruku/issho/issues/402)) does the check, and [#476](https://github.com/shrujancheruku/issho/issues/476) builds it.

This keeps a small amount of personal data after the person has asked for erasure, which is a trade-off under Vietnam's PDPL. We judged it justified for safety: one record per suspended person, holding no email in the clear. [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) confirmed it for the Pilot: the privacy policy says what is kept, why and for how long, and a lawyer checks it before the public launch.

## Considered options

- **Keep nothing**, and rely on invite-only sign-up to keep a suspended person out. Rejected: the invite list is keyed by email too, so the same person could be invited again by mistake with nothing to warn the owner.
- **Soft-delete the whole account.** Rejected: it keeps far more than safety needs, against the point of deletion.
- **Refuse the sign-up outright.** Rejected: it needs its own error path and screen, and a moderator could no longer lift a mistaken suspension the usual way.

## Consequences

- The HMAC key lives only in the deployment's secret store, never in the repo. It is never rotated: if it's lost or changed, existing tombstones stop matching, which we accept for the Pilot.
- A different email gets around it. Invite-only sign-up is the control for that, not this.
