-- +goose Up
-- An area's outline, from its city pack (e.g. a ward's administrative
-- boundary). Venue discovery searches all of it and places each venue in the
-- area that contains it; areas without one keep using their point only.
alter table app.areas add column boundary geography(multipolygon, 4326);

-- +goose Down
alter table app.areas drop column boundary;
