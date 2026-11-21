-- TEMPORARY, #605's proof that a failed migration stops the deploy. Reverted by
-- the next commit; never meant to land.

-- +goose Up
SELECT 1/0;

-- +goose Down
SELECT 1;
