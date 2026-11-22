# seed

`make seed` loads the development fixture (`../../fixtures`) into the local database: the city, then its plans. Run `make migrate-up` first. Running it again replaces the fixture's rows with the fixture's.

It reads `DATABASE_URL` (`make seed` points it at the local Supabase database) and **refuses any database that isn't on this machine**, before it connects and without printing the URL: the host must be `localhost` or a loopback address, with no `host=` override in the query. The production image doesn't build it. See `../../fixtures/README.md`.
