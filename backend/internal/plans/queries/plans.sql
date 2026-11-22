-- The plans tables, app.plans, app.plan_texts, app.plan_stops and
-- app.plan_stop_texts; only internal/plans calls these.

-- name: ListPlansInCity :many
select * from app.plans where city = $1 order by starts_at, id;

-- name: GetPlan :one
select * from app.plans where id = $1;

-- name: ListPlanTexts :many
select * from app.plan_texts where plan_id = any($1::uuid[]);

-- name: ListPlanStops :many
select * from app.plan_stops where plan_id = any($1::uuid[]) order by plan_id, position;

-- name: ListPlanStopTexts :many
select * from app.plan_stop_texts where plan_id = any($1::uuid[]);

-- Loading the fixture again replaces a plan whole: its rows are deleted and
-- written again, so a stop dropped from the fixture doesn't linger.
-- name: DeletePlan :exec
delete from app.plans where id = $1;

-- name: InsertPlan :exec
insert into app.plans (id, city, price, meeting_address, parts_of_day, starts_at, ends_at)
values ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertPlanText :exec
insert into app.plan_texts (plan_id, language, title, description, meeting_point, what_to_bring)
values ($1, $2, $3, $4, $5, $6);

-- name: InsertPlanStop :exec
insert into app.plan_stops (plan_id, position, stay_minutes, leg_mode, leg_minutes)
values ($1, $2, $3, $4, $5);

-- name: InsertPlanStopText :exec
insert into app.plan_stop_texts (plan_id, position, language, name, note)
values ($1, $2, $3, $4, $5);
