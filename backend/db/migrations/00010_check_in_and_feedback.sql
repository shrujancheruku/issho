-- +goose Up

-- Check-in and feedback's first three questions (docs/spec/outings.md, "On the
-- day" and "Feedback"). `outings` owns every table here; no other package reads
-- them.

-- The window of check-in and feedback is the plan's stated end on the city's
-- clock, so an outing keeps the end and the city's time zone as they were when
-- the group was formed, as it keeps the start: a later change to a plan or a
-- city can't move a window someone has been told about.
alter table app.outings add column ends_at text;
alter table app.outings add column time_zone text;
update app.outings o set ends_at = p.ends_at, time_zone = c.time_zone
from app.plans p join app.cities c on c.slug = p.city
where p.id = o.plan_id;
alter table app.outings alter column ends_at set not null;
alter table app.outings alter column time_zone set not null;
alter table app.outings add constraint outings_ends_at_check
  check (ends_at ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$' and ends_at > starts_at);
alter table app.outings add constraint outings_time_zone_check check (time_zone <> '');

-- A seat's history. "People who were going at the start" is decided from when a
-- seat became going and when it left going, never from its state now: a person
-- who cancelled after the start was going at the start. A seat leaves going once
-- (a cancel); it can't come back, so two instants are the whole history.
alter table app.seats add column going_at timestamptz;
alter table app.seats add column left_going_at timestamptz;
-- One tap, no proof of place: the instant of the tap.
alter table app.seats add column checked_in_at timestamptz;
-- What the other people of the outing call this seat in feedback. The API never
-- shows an account's id, so feedback names a person by this handle instead,
-- which is random, means nothing outside the outing and can't be linked to a
-- person, an email or another outing.
alter table app.seats add column handle uuid not null default gen_random_uuid();

-- Seats made before this migration: those going or cancelled became going when
-- they were invited, as far as is known, and those cancelled left then too, so
-- none of them is taken to have been going at a start that is still to come.
update app.seats set going_at = invited_at where state in ('going', 'cancelled', 'late_cancel');
update app.seats set left_going_at = invited_at where state in ('cancelled', 'late_cancel');

alter table app.seats add constraint seats_handle_key unique (outing_id, handle);
-- Only a seat that was going can have left going or checked in.
alter table app.seats add constraint seats_history_check check (
  (left_going_at is null or going_at is not null) and (checked_in_at is null or going_at is not null));

-- Feedback's first question, "did you go?": one row for each person who has
-- answered it. A no ends the feedback there, so a person who didn't go has no
-- other rows. It can be changed until feedback closes.
create table app.feedback (
  outing_id   uuid not null,
  account_id  uuid not null,
  went        boolean not null,
  answered_at timestamptz not null,
  updated_at  timestamptz not null,
  primary key (outing_id, account_id),
  foreign key (outing_id, account_id) references app.seats (outing_id, account_id) on delete cascade
);

-- "Who else was there?" is everyone ticked by default, so only an untick is
-- stored. Nobody can see who unticked them.
create table app.feedback_unticks (
  outing_id  uuid not null,
  rater_id   uuid not null,
  subject_id uuid not null,
  primary key (outing_id, rater_id, subject_id),
  foreign key (outing_id, rater_id) references app.feedback (outing_id, account_id) on delete cascade,
  foreign key (outing_id, subject_id) references app.seats (outing_id, account_id) on delete cascade,
  constraint feedback_unticks_other_check check (rater_id <> subject_id)
);

-- "Who would you meet again?": each answer is a pick, private to the person who
-- gave it. Neither the picked person nor anyone else is ever told, so nothing
-- reads this table by picked_id for an API (a mutual pick is the seat order's,
-- milestone 4).
create table app.picks (
  outing_id  uuid not null,
  picker_id  uuid not null,
  picked_id  uuid not null,
  created_at timestamptz not null,
  primary key (outing_id, picker_id, picked_id),
  foreign key (outing_id, picker_id) references app.feedback (outing_id, account_id) on delete cascade,
  foreign key (outing_id, picked_id) references app.seats (outing_id, account_id) on delete cascade,
  constraint picks_other_check check (picker_id <> picked_id)
);

-- +goose Down

drop table app.picks;
drop table app.feedback_unticks;
drop table app.feedback;
alter table app.seats drop constraint seats_history_check;
alter table app.seats drop constraint seats_handle_key;
alter table app.seats drop column handle;
alter table app.seats drop column checked_in_at;
alter table app.seats drop column left_going_at;
alter table app.seats drop column going_at;
alter table app.outings drop constraint outings_time_zone_check;
alter table app.outings drop constraint outings_ends_at_check;
alter table app.outings drop column time_zone;
alter table app.outings drop column ends_at;
