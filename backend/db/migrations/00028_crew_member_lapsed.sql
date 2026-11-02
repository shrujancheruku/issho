-- +goose Up
-- A member who never answered an offer by its deadline. Before, they stayed
-- "offered" for good, so an active crew listed them as still deciding for as
-- long as it lived. responded_at is set when the offer was decided, which the
-- (response = 'offered') = (responded_at is null) check requires.
alter table app.crew_members
  drop constraint crew_members_response_check,
  add constraint crew_members_response_check check (response in ('offered', 'accepted', 'declined', 'left', 'lapsed'));

-- Offers decided before this migration already left their silent members
-- "offered": settle those too.
update app.crew_members m set response = 'lapsed', responded_at = coalesce(c.closed_at, c.formed_at, c.offer_until)
from app.crews c
where c.id = m.crew_id and c.status <> 'offered' and m.response = 'offered';

-- +goose Down
update app.crew_members set response = 'offered', responded_at = null where response = 'lapsed';
alter table app.crew_members
  drop constraint crew_members_response_check,
  add constraint crew_members_response_check check (response in ('offered', 'accepted', 'declined', 'left'));
