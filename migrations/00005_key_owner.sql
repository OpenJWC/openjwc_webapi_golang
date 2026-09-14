-- +goose Up
ALTER TABLE api_keys ADD COLUMN user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX api_keys_user_idx ON api_keys(user_id);

-- +goose Down
DROP INDEX api_keys_user_idx;
ALTER TABLE api_keys DROP COLUMN user_id;
