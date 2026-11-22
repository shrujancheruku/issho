-- +goose Up

-- A seat that said yes to an outing that isn't set yet. The spec's stages have
-- no word for it ("Going: they said yes and the outing is set"), but the yes
-- has to be kept somewhere until the third one sets the outing, and it counts
-- as a yes (docs/spec/outings.md, "Invited"). Setting the outing makes every
-- accepted seat going; an outing that ends not set leaves them accepted, which
-- is how "those who said yes count as left waiting" is read back.
alter table app.seats drop constraint seats_state_check;
alter table app.seats add constraint seats_state_check check (state in (
  'invited', 'accepted', 'going', 'declined', 'unanswered', 'full', 'cancelled', 'late_cancel', 'removed'));

-- +goose Down

-- A yes that was waiting for the outing to be set goes back to being an
-- invitation, which is what it was before it was answered.
update app.seats set state = 'invited' where state = 'accepted';
alter table app.seats drop constraint seats_state_check;
alter table app.seats add constraint seats_state_check check (state in (
  'invited', 'going', 'declined', 'unanswered', 'full', 'cancelled', 'late_cancel', 'removed'));
