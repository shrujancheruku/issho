# people

Owns accounts: who a person is to Issho from the first signed-in call ([People](../../../docs/spec/people.md#sign-in-and-the-account), [Architecture and stack](../../../docs/spec/architecture.md#the-seams)). An **account** is an email, a city and a display language, nothing more. The table is `app.accounts` (`db/migrations/00003_accounts.sql`); no other package reads it.

The profile (`app.profiles`, `db/migrations/00005_profiles.sql`) is the answers asked at the first signup; standing, join links and consent are this package's too but arrive with their own tickets, each in a table keyed by the account's id.

## Sign-in

Sign-in is Supabase Auth's: a 6-digit code emailed to the person (`supabase/README.md`). This package never sees a code or a password. The app gets an access token from Auth and sends it as `Authorization: Bearer`; the API checks it with a `Verifier` and makes the account the first time it is used.

## Using it

| Function | What |
|---|---|
| `NewVerifier(clk, VerifierConfig)` | Checks Supabase Auth's access tokens against the project's JWKS. Accepts ES256 and RS256 and nothing else (not HS256, which would need the JWT secret the API never holds, and not `none`). Checks expiry and not-before on the injected clock, the issuer, the `authenticated` audience and role, and a subject. `Verify` returns `Claims` (subject, email, second step: the token's `aal2`) or an error |
| `New(clk, pool, cities, Config)` | The account service. `Config.DefaultCity` is the city of a first call that names none; which cities a call may name is `cities`' to say |
| `Service.Join(ctx, Claims, Preferences)` | The caller's account, made if this is the first call: the token's email (lowercased), the city and display language in `Preferences` or their defaults (`DefaultCity`, English). Once made it is returned as it is and `Preferences` are not applied (they are checked on every call): changing a city or language is a settings call. `ErrInvalid` for malformed input, a city `cities` doesn't list or a display language other than `en` or `vi` (nothing is made), `ErrEmailTaken` for an email another account holds |
| `Service.Account(ctx, id)` | An account by id, or `ErrNotFound` |
| `NewFakeIssuer()` | Stands in for Supabase Auth in tests: `Verifier(clk)` trusts its key (the key set is served in-process), `Token(now, id, email)` and `SecondStepToken` mint tokens. The one fake every test that needs a signed-in caller uses |
| `Service.SaveProfile(ctx, id, ProfileInput)` | Makes the account's profile on the first call, changes it on later ones: the name they go by (1 to 40 characters, `policy`), `Gender` (`woman`, `man`, `non-binary`, `prefer-not-to-say`), spoken languages (at least one language code, each listed once), whether they live in the city, and the code of conduct accepted (`CodeOfConductVersion` is stored). The first call also needs the date of birth, which is checked on the city's date (`cities` time zone through `cityclock`) and thrown away: under `policy.MinAge` is `ErrUnderage`, and only `said_adult_on`, the city's day it was said, is stored. A later call with a date of birth is `ErrInvalid` (it is asked once and not changed). "Only women and non-binary people" is for those whose gender is woman or non-binary (`ErrWomenOnlyNotAvailable`); a person who had it on and changes their gender to another answer has it turned off. `ErrNotFound` for no such account; every refusal saves nothing |
| `Service.Profile(ctx, id)` | The account's own profile, or `ErrProfileRequired`. There is no function that returns another person's profile |
| `Service.RequireProfile(ctx, id)` | `nil`, or `ErrProfileRequired`: what a signup checks first. The RPC layer wraps it as `rpc.RequireProfile` |

**Keys.** The key set is fetched on the first token and refreshed once an hour. A token naming an unknown key id makes the API fetch it again, at most once a minute, so a rotation is picked up and a stream of forged ids can't make a fetch per request. The fetch never runs under the verifier's lock and is shared: tokens that need it wait for the one in flight, and a token whose key is already known never waits, even while the key-set host is slow or down (the hourly refresh runs beside the request). If a fetch fails the last good set keeps working, but only up to `KeySetMaxAge` (3 hours) from its last successful fetch: past it no token verifies until a fetch succeeds (fail closed), and each such refusal is logged (never the token or its key id). So a key Auth has revoked stops working as soon as the refreshed set arrives, and at the latest 3 hours after the last good fetch, even while the key-set host is unreachable.

**The id.** An account's id is Supabase Auth's user id (the token's `sub`). It is not a foreign key to `auth.users`, which doesn't exist on the plain Postgres the tests run on.

**The city.** It is the city's slug, a foreign key to `app.cities` (`db/migrations/00004_cities_and_plans.sql`), and `Join` asks `internal/cities` whether the city is listed, so a city that opens is no change here. In development and tests the fixture city is the one listed (`internal/cities`, `fixtures/`); a database with no city lists none, and nobody can join. Display languages are `people.Languages` (`en`, `vi`): the languages of the open cities.

## Tests

Package tests go through the exported API on the throwaway Postgres (`internal/testdb`) with a `clock.Fake`; the token tests sign with generated ES256 and RS256 keys and serve them from an `httptest` server.

| File | Proves |
|---|---|
| `people_test.go` | The first call makes the account with the token's email, a city and a language; nothing sent means the defaults; a second call changes nothing; malformed input, an unlisted city and an unlisted language are refused and leave no account; a city `cities` lists makes an account even when it isn't the default, and with none listed nobody can join; the database refuses an account in an unlisted city; one account per email |
| `profile_test.go` | The profile holds its answers and reads back; the name's 1 to 40 characters (counted as characters); gender is one of four and required; at least one language, any language; the code of conduct must be accepted; under 18 on the day by the city's clock (a person who turns 18 today in the city, still 17 in UTC) is refused and an adult is accepted; the date of birth is in no column and the day said is the city's; everything but the date of birth can change and a repeat date of birth is refused; the women-only setting for woman and non-binary only, switched at any time, off by itself after a gender change; `ErrProfileRequired` for an account with none and `ErrNotFound` for no account |
| `token_test.go` | ES256 and RS256 verify; a token signed with another key, an unknown key id, another algorithm, HS256 or `none` is refused; an expired token is refused on the fake clock; key rotation and the fetch limit; a key set that can't be refreshed stops verifying at its maximum age (logged without the token) and comes back on the next good fetch; a slow key-set fetch (on a rotation or the hourly refresh) doesn't hold up a token whose key is known, and concurrent tokens share one fetch |
| `supabase_config_test.go` | `supabase/config.toml` sets a 6-digit code, the local mail catcher, templates that show the code, and `enable_confirmations` on |
