-- +goose Up
CREATE VIRTUAL TABLE notice_search USING fts5(
    title,
    content,
    content='notices',
    content_rowid='rowid',
    tokenize='trigram'
);

CREATE TRIGGER notice_search_insert AFTER INSERT ON notices BEGIN
    INSERT INTO notice_search(rowid, title, content) VALUES(new.rowid, new.title, new.content);
END;

CREATE TRIGGER notice_search_delete AFTER DELETE ON notices BEGIN
    INSERT INTO notice_search(notice_search, rowid, title, content)
    VALUES('delete', old.rowid, old.title, old.content);
END;

CREATE TRIGGER notice_search_update AFTER UPDATE ON notices BEGIN
    INSERT INTO notice_search(notice_search, rowid, title, content)
    VALUES('delete', old.rowid, old.title, old.content);
    INSERT INTO notice_search(rowid, title, content) VALUES(new.rowid, new.title, new.content);
END;

INSERT INTO notice_search(notice_search) VALUES('rebuild');

-- +goose Down
DROP TRIGGER notice_search_update;
DROP TRIGGER notice_search_delete;
DROP TRIGGER notice_search_insert;
DROP TABLE notice_search;
