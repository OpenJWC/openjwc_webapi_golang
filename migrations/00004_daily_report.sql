-- +goose Up
CREATE TABLE daily_reports (
    day TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK(status IN ('running','failed','completed')),
    content TEXT NOT NULL DEFAULT '',
    source_count INTEGER NOT NULL DEFAULT 0 CHECK(source_count>=0),
    updated_at TEXT NOT NULL
);
CREATE TABLE job_state (
    name TEXT PRIMARY KEY,
    last_success TEXT NOT NULL
);

-- +goose Down
DROP TABLE job_state;
DROP TABLE daily_reports;
