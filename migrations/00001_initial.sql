-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE notices (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    published_at TEXT NOT NULL,
    detail_url TEXT NOT NULL,
    is_page INTEGER NOT NULL CHECK (is_page IN (0, 1)),
    content TEXT NOT NULL DEFAULT '',
    is_pushed INTEGER NOT NULL DEFAULT 0 CHECK (is_pushed IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX notices_published_at_idx ON notices (published_at DESC, id DESC);
CREATE INDEX notices_label_published_at_idx ON notices (label, published_at DESC);

CREATE TABLE notice_attachments (
    notice_id TEXT NOT NULL REFERENCES notices (id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    name TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL,
    PRIMARY KEY (notice_id, position)
);

CREATE TABLE api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash TEXT NOT NULL UNIQUE,
    owner_name TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    max_devices INTEGER NOT NULL DEFAULT 3 CHECK (max_devices > 0),
    total_requests INTEGER NOT NULL DEFAULT 0 CHECK (total_requests >= 0),
    created_at TEXT NOT NULL
);

CREATE TABLE api_key_devices (
    api_key_id INTEGER NOT NULL REFERENCES api_keys (id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    bound_at TEXT NOT NULL,
    PRIMARY KEY (api_key_id, device_id)
);

CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE submissions (
    id TEXT PRIMARY KEY,
    api_key_id INTEGER NOT NULL REFERENCES api_keys (id),
    label TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    published_at TEXT,
    detail_url TEXT NOT NULL DEFAULT '',
    is_page INTEGER NOT NULL CHECK (is_page IN (0, 1)),
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    review TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX submissions_api_key_id_created_at_idx ON submissions (api_key_id, created_at DESC);
CREATE INDEX submissions_status_created_at_idx ON submissions (status, created_at DESC);

CREATE TABLE submission_attachments (
    submission_id TEXT NOT NULL REFERENCES submissions (id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    url TEXT NOT NULL,
    PRIMARY KEY (submission_id, position)
);

CREATE TABLE mottos (
    date TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    author TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS mottos;
DROP TABLE IF EXISTS submission_attachments;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS api_key_devices;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS notice_attachments;
DROP TABLE IF EXISTS notices;
