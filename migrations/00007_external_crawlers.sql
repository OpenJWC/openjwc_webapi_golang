CREATE TABLE crawl_sources_external (
    name TEXT PRIMARY KEY CHECK (
        length(name) BETWEEN 1 AND 64
        AND name = lower(name)
        AND name NOT GLOB '*[^a-z0-9._-]*'
        AND substr(name, 1, 1) GLOB '[a-z0-9]'
    ),
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

INSERT INTO crawl_sources_external(name,state,scanned,saved,skipped,failed,started_at,finished_at,last_success,detail)
SELECT name,state,scanned,saved,skipped,failed,started_at,finished_at,last_success,detail FROM crawl_sources;

DROP TABLE crawl_sources;
ALTER TABLE crawl_sources_external RENAME TO crawl_sources;
