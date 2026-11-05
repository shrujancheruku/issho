# The backup accessory's image (config/deploy.yml, docs/deploy.md): the
# Postgres client the nightly pg_dump needs, plus curl, baked in so the
# container starts without reaching Alpine's package mirrors.
#
# The major version must stay equal to the database's: pg_dump refuses to
# dump a server newer than itself. Bump it with the Supabase project's
# Postgres version (the image tag in config/deploy.yml follows it).
FROM postgres:18-alpine
RUN apk add --no-cache curl
