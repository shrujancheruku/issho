# people

Owns accounts: who a person is to Issho from the first signed-in call ([People](../../../docs/spec/people.md#sign-in-and-the-account), [Architecture and stack](../../../docs/spec/architecture.md#the-seams)). An **account** is an email, a city and a display language, nothing more. The table is `app.accounts` (`db/migrations/00003_accounts.sql`); no other package reads it.

The profile, standing, join links and consent are this package's too but arrive with their own tickets, each in a table keyed by the account's id.

## Sign-in

Sign-in is Supabase Auth's: a 6-digit code emailed to the person (`supabase/README.md`). This package never sees a code or a password. The app gets an access token from Auth and sends it as `Authorization: Bearer`; the API checks it with a `Verifier` and makes the account the first time it is used.

## Using it

| Function | What |
|---|---|
| `NewVerifier(clk, VerifierConfig)` | Checks Supabase Auth's access tokens against the project's JWKS. Accepts ES256 and RS256 and nothing else (not HS256, which would need the JWT secret the API never holds, and not `none`). Checks expiry and not-before on the injected clock, the issuer, the `authenticated` audience and role, and a subject. `Verify` returns `Claims` (subject, email, second step: the token's `aal2`) or an error |
| `New(clk, pool, Config)` | The account service. `Config.DefaultCity` is the only listed city: the city of a first call that names none, and the only city a call may name |
| `Service.Join(ctx, Claims, Preferences)` | The caller's account, made if this is the first call: the token's email (lowercased), the city and display language in `Preferences` or their defaults (`DefaultCity`, English). Once made it is returned as it is and `Preferences` are not applied (they are checked on every call): changing a city or language is a settings call. `ErrInvalid` for malformed input, a city that isn't listed or a display language other than `en` or `vi` (nothing is made), `ErrEmailTaken` for an email another account holds |
| `Service.Account(ctx, id)` | An account by id, or `ErrNotFound` |
| `NewFakeIssuer()` | Stands in for Supabase Auth in tests: `Verifier(clk)` trusts its key (the key set is served in-process), `Token(now, id, email)` and `SecondStepToken` mint tokens. The one fake every test that needs a signed-in caller uses |

**Keys.** The key set is fetched on the first token and refreshed once an hour. A token naming an unknown key id makes the API fetch it again, at most once a minute, so a rotation is picked up and a stream of forged ids can't make a fetch per request. The fetch never runs under the verifier's lock and is shared: tokens that need it wait for the one in flight, and a token whose key is already known never waits, even while the key-set host is slow or down (the hourly refresh runs beside the request). If a fetch fails the last good set keeps working. A key Auth has revoked stops working as soon as the refreshed set arrives, within an hour and one fetch.

**The id.** An account's id is Supabase Auth's user id (the token's `sub`). It is not a foreign key to `auth.users`, which doesn't exist on the plain Postgres the tests run on.

**The city.** It is the city's slug, a plain column until the fixture city's ticket (#610) adds the `cities` table, the constraint and the check that the city is listed (until then `Join` accepts only `Config.DefaultCity`, `da-nang-hoi-an`). Display languages are `people.Languages` (`en`, `vi`): the languages of the open cities.

## Tests

Package tests go through the exported API on the throwaway Postgres (`internal/testdb`) with a `clock.Fake`; the token tests sign with generated ES256 and RS256 keys and serve them from an `httptest` server.

| File | Proves |
|---|---|
| `people_test.go` | The first call makes the account with the token's email, a city and a language; nothing sent means the defaults; a second call changes nothing; malformed input, an unlisted city and an unlisted language are refused and leave no account; one account per email |
| `token_test.go` | ES256 and RS256 verify; a token signed with another key, an unknown key id, another algorithm, HS256 or `none` is refused; an expired token is refused on the fake clock; key rotation and the fetch limit; a slow key-set fetch (on a rotation or the hourly refresh) doesn't hold up a token whose key is known, and concurrent tokens share one fetch |
| `supabase_config_test.go` | `supabase/config.toml` sets a 6-digit code, the local mail catcher, templates that show the code, and `enable_confirmations` on |
