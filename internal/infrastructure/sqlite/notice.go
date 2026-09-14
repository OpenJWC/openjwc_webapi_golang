package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// Save 原子保存资讯及附件，全文索引由同事务触发器维护。
func (store *Store) Save(ctx context.Context, item notice.Notice) error {
	if item.ID() == "" {
		return fmt.Errorf("不能保存未初始化资讯")
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始保存资讯: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO notices
		(id,label,title,published_at,detail_url,is_page,content,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		label=excluded.label,title=excluded.title,published_at=excluded.published_at,
		detail_url=excluded.detail_url,is_page=excluded.is_page,content=excluded.content,updated_at=excluded.updated_at`,
		item.ID(), item.Label(), item.Title(), item.PublishedAt().Format(time.RFC3339), item.DetailURL(), item.IsPage(), item.Content(), now(), now())
	if err != nil {
		return fmt.Errorf("保存资讯主体: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM notice_attachments WHERE notice_id=?", item.ID()); err != nil {
		return err
	}
	for index, attachment := range item.Attachments() {
		if _, err = tx.ExecContext(ctx, "INSERT INTO notice_attachments(notice_id,position,name,url) VALUES(?,?,?,?)", item.ID(), index, attachment.Name, attachment.URL); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FindByID 在同一快照内读取资讯正文与附件。
func (store *Store) FindByID(ctx context.Context, id string) (notice.Notice, bool, error) {
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return notice.Notice{}, false, err
	}
	defer tx.Rollback()
	input, err := scanNotice(tx.QueryRowContext(ctx, "SELECT id,label,title,published_at,detail_url,is_page,content FROM notices WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return notice.Notice{}, false, nil
	}
	if err != nil {
		return notice.Notice{}, false, fmt.Errorf("读取资讯: %w", err)
	}
	input.Attachments, err = readAttachments(ctx, tx, id)
	if err != nil {
		return notice.Notice{}, false, err
	}
	item, err := notice.New(input)
	return item, err == nil, err
}

// noticeScanner 是数据库行到领域输入转换所需的最小接口。
type noticeScanner interface{ Scan(...interface{}) error }

// scanNotice 将持久化时间转换为领域时间，损坏数据不会伪装成空结果。
func scanNotice(row noticeScanner) (notice.CreateInput, error) {
	var input notice.CreateInput
	var published string
	err := row.Scan(&input.ID, &input.Label, &input.Title, &published, &input.DetailURL, &input.IsPage, &input.Content)
	if err != nil {
		return input, err
	}
	input.PublishedAt, err = time.Parse(time.RFC3339, published)
	return input, err
}

// readAttachments 读取同一事务快照中的有序附件。
func readAttachments(ctx context.Context, tx *sql.Tx, id string) ([]notice.Attachment, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name,url FROM notice_attachments WHERE notice_id=? ORDER BY position", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]notice.Attachment, 0)
	for rows.Next() {
		var attachment notice.Attachment
		if err = rows.Scan(&attachment.Name, &attachment.URL); err != nil {
			return nil, err
		}
		items = append(items, attachment)
	}
	return items, rows.Err()
}

// DeleteNotice 删除资讯及其关联索引，返回是否确有记录被删除。
func (store *Store) DeleteNotice(ctx context.Context, id string) (bool, error) {
	result, err := store.writer.ExecContext(ctx, "DELETE FROM notices WHERE id=?", id)
	if err != nil {
		return false, fmt.Errorf("删除资讯: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
