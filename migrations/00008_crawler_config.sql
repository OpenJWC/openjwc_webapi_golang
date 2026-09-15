CREATE TABLE crawler_config (
    name TEXT PRIMARY KEY CHECK (
        length(name) BETWEEN 1 AND 64
        AND name = lower(name)
        AND name NOT GLOB '*[^a-z0-9._-]*'
        AND substr(name, 1, 1) GLOB '[a-z0-9]'
    ),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    interval_minutes INTEGER NOT NULL DEFAULT 480 CHECK (interval_minutes BETWEEN 5 AND 10080),
    updated_at TEXT NOT NULL DEFAULT ''
);
