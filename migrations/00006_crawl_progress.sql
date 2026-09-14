CREATE TABLE crawl_sources (
    name TEXT PRIMARY KEY CHECK (name IN ('jwc','cs','xsxy')),
    state TEXT NOT NULL CHECK (state IN ('running','completed','failed','canceled')),
    scanned INTEGER NOT NULL DEFAULT 0 CHECK (scanned >= 0),
    saved INTEGER NOT NULL DEFAULT 0 CHECK (saved >= 0),
    skipped INTEGER NOT NULL DEFAULT 0 CHECK (skipped >= 0),
    failed INTEGER NOT NULL DEFAULT 0 CHECK (failed >= 0),
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL DEFAULT '',
    last_success TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT ''
);
