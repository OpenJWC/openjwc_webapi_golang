-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1 CHECK(is_active IN (0,1)),
    can_browse INTEGER NOT NULL DEFAULT 1 CHECK(can_browse IN (0,1)),
    can_chat INTEGER NOT NULL DEFAULT 0 CHECK(can_chat IN (0,1)),
    can_submit INTEGER NOT NULL DEFAULT 1 CHECK(can_submit IN (0,1)),
    max_devices INTEGER NOT NULL DEFAULT 3 CHECK(max_devices BETWEEN 1 AND 100),
    created_at TEXT NOT NULL
);
CREATE TABLE user_registrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
    review TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE user_devices (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_uuid TEXT NOT NULL,
    device_name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    last_login TEXT NOT NULL,
    PRIMARY KEY(user_id,device_uuid)
);
CREATE INDEX user_devices_expiry_idx ON user_devices(expires_at);
CREATE TABLE user_submissions (
    id TEXT PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    label TEXT NOT NULL,
    title TEXT NOT NULL,
    published_at TEXT NOT NULL,
    detail_url TEXT NOT NULL,
    is_page INTEGER NOT NULL CHECK(is_page IN (0,1)),
    content TEXT NOT NULL,
    attachments TEXT NOT NULL CHECK(json_valid(attachments)),
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
    review TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX user_submissions_user_idx ON user_submissions(user_id,created_at DESC);
CREATE INDEX user_submissions_status_idx ON user_submissions(status,created_at DESC);
CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE import_runs (
    source_hash TEXT PRIMARY KEY,
    imported_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE import_runs;
DROP TABLE audit_events;
DROP TABLE user_submissions;
DROP TABLE user_devices;
DROP TABLE user_registrations;
DROP TABLE users;
