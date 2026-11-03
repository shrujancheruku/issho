-- +goose Up

-- One definition each of the domain rules that queries used to copy (#186):
-- attended (and so met), suspended, and kept apart. Queries read these views
-- instead of repeating the rule. Additive: nothing the deployed code reads is
-- touched, so the old code keeps working between `kamal migrate` and
-- `kamal deploy`.
--
-- security_invoker: a view runs with its caller's rights, as the app.* tables
-- do. No grants to anon/authenticated (the Data API is disabled; see the
-- baseline's revokes).

-- One row per group membership, and whether the person attended (GLOSSARY.md,
-- "Attended"): they confirmed, their group went ahead (the outing is done and
-- the group wasn't cancelled), and they checked in or their feedback says they
-- went. Feedback that says they didn't go wins over a check-in, hence the
-- coalesce: the feedback answer when there is one, else the check-in.
create view app.attendance with (security_invoker = true) as
select m.user_id,
       m.group_id,
       m.event_id,
       e.date,
       m.response,
       -- The group went ahead: a finished outing whose group wasn't cancelled.
       (e.status = 'done' and g.status <> 'cancelled') as went_ahead,
       (m.response = 'confirmed' and e.status = 'done' and g.status <> 'cancelled'
        and coalesce(f.attended, m.checked_in_at is not null)) as attended
from app.group_members m
join app.event_groups g on g.id = m.group_id
join app.events e on e.id = m.event_id
left join app.event_feedback f on f.group_id = m.group_id and f.user_id = m.user_id;

-- Everyone suspended now (GLOSSARY.md, "Suspended"): the open suspension, of
-- which a person has at most one (suspensions_open_idx).
create view app.suspended_users with (security_invoker = true) as
select s.user_id, s.id as suspension_id
from app.suspensions s
where s.lifted_at is null;

-- Pairs kept apart for good: someone reported the other, or said they'd rather
-- not be in a group with them again. Directional (user_id made the report or
-- the pick); callers that need either direction check both. Whatever a
-- moderator decides about a report, the pair stays apart.
create view app.kept_apart with (security_invoker = true) as
select r.reporter_id as user_id, r.reported_id as other_id
from app.reports r
where r.reporter_id is not null
union
select k.picker_id, k.picked_id
from app.mate_picks k
where k.pick = 'avoid';

-- +goose Down
drop view if exists app.kept_apart;
drop view if exists app.suspended_users;
drop view if exists app.attendance;
