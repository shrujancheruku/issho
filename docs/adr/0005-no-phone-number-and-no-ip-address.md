# No phone number and no IP address

Issho never collects or verifies a phone number, at any stage, and keeps no IP address anywhere: not in the sign-in log, the API's logs, Sentry or the proxy's logs. Sign-in is an email code, with Apple and Google from the soft launch. Vietnam's rules for social platforms expect accounts verified by a Vietnamese phone number and a log of sign-ins, which may be read to include the address. Both choices are deliberate, taken as accepted risks while the owner operates Issho as an individual ([ADR 0004](0004-the-owner-operates-issho-until-the-public-launch.md)). The rules are in [Privacy](../spec/privacy.md#no-phone-number-and-the-sign-in-log) and [Technical operations](../spec/operations.md#the-sign-in-log).

We chose this because of what each would cost against what it would protect:

- **A +84 phone code** needs a registered SMS sender ID or a Zalo Official Account, and both are likely to need a business registration the owner doesn't have ([What the v2 spec asks of the stack](https://github.com/shrujancheruku/issho/issues/571)). Nothing in the product uses a phone number, so it would be collected only for the rule.
- **IP addresses** are personal data with no use in the product. The sign-in log is Supabase Auth's own, which records the account and the time of each sign-in and sign-out and no address; keeping one would mean capture code and a table of our own for something only a lawful request would ever read.

The check on who joins is the chain of join links in the Pilot and the waitlist in the soft launch, not a phone number.

## Considered options

- **Verify a +84 number from the soft launch** (the spec's first answer). Dropped when the research showed the sender registration it needs.
- **Log IP addresses for 12 months, read by nobody.** Rejected: it is the most sensitive thing Issho would hold, kept only against a request that may never come.

## Consequences

- Don't add phone verification or IP logging as a fix. Either one changes the privacy notice and needs a fresh tick from every member.
- Two questions go to the lawyer before the public launch: whether email codes without phone verification are enough, and whether a sign-in log with no IP address is enough. If the answer is no, this ADR is reopened with the LLC in place.
- Cloudflare sees every request's IP address in transit. That is stated in the recipient list; Issho still keeps none.
- Keeping none depends on the zone's "Remove visitor IP headers" managed transform. Cloudflare otherwise sends the address to the origin in `X-Forwarded-For`, which kamal-proxy writes to its request log before the forwarders drop it. The setting is made by hand, and [Deploying dev](../deploy.md#a-required-cloudflare-zone-setting) says how to confirm it.
- Nothing is guessed from an IP address: a visitor's city comes from the link or a list.
