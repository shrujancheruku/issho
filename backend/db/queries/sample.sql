-- A sample query, so that sqlc has something to compile and a test something to
-- call (db/queries_test.go). Delete it with the first real query.

-- name: CountJobs :one
select count(*) from river.river_job;
