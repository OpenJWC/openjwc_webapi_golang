package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/submission"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/contribution"
)

// Submit 在写入事务中复核投稿权限，避免停用与投稿竞态。
func (store *Store) Submit(ctx context.Context, item submission.Submission) error {
	id, err := strconv.ParseInt(item.SubmitterID(), 10, 64)
	if err != nil || id < 1 || item.ID() == "" {
		return fmt.Errorf("投稿身份无效")
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var allowed bool
	if err = tx.QueryRowContext(ctx, "SELECT is_active AND can_submit FROM users WHERE id=?", id).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return access.ErrForbidden
	}
	var limit int
	if err = tx.QueryRowContext(ctx, "SELECT coalesce((SELECT CAST(value AS INTEGER) FROM system_settings WHERE key='submission_max_length'),10000)").Scan(&limit); err != nil {
		return err
	}
	if utf8.RuneCountInString(item.Content()) > limit {
		return access.ErrConflict
	}
	var existing int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM user_submissions WHERE id=?", item.ID()).Scan(&existing); err != nil {
		return err
	}
	if existing != 0 {
		return access.ErrConflict
	}
	attachments := item.Attachments()
	if attachments == nil {
		attachments = []string{}
	}
	encoded, err := json.Marshal(attachments)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_submissions(id,user_id,label,title,published_at,detail_url,is_page,content,attachments,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID(), id, item.Label(), item.Title(), item.PublishedAt().Format(time.RFC3339), item.DetailURL(), item.IsPage(), item.Content(), string(encoded), now(), now())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Submissions 返回当前用户投稿的公开投影。
func (store *Store) Submissions(ctx context.Context, id int64) ([]contribution.Summary, error) {
	rows, err := store.reader.QueryContext(ctx, "SELECT id,label,title,substr(published_at,1,10),detail_url,is_page,status,review FROM user_submissions WHERE user_id=? ORDER BY created_at DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]contribution.Summary, 0)
	for rows.Next() {
		var item contribution.Summary
		if err = rows.Scan(&item.ID, &item.Label, &item.Title, &item.Date, &item.URL, &item.IsPage, &item.Status, &item.Review); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// reviewSubmission 将审核通过与资讯发布绑定为同一事务。
func (store *Store) reviewSubmission(ctx context.Context, request admin.Request) (admin.Response, error) {
	decision := request.Values["decision"]
	if decision != "approved" && decision != "rejected" {
		return admin.Response{}, fmt.Errorf("审核结果必须为 approved 或 rejected")
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, "SELECT status FROM user_submissions WHERE id=?", request.ID).Scan(&status); err != nil {
		return admin.Response{}, err
	}
	if status != "pending" {
		return admin.Response{}, access.ErrConflict
	}
	if decision == "approved" {
		var length, limit int
		if err = tx.QueryRowContext(ctx, "SELECT length(content),coalesce((SELECT CAST(value AS INTEGER) FROM system_settings WHERE key='submission_max_length'),10000) FROM user_submissions WHERE id=?", request.ID).Scan(&length, &limit); err != nil {
			return admin.Response{}, err
		}
		if length > limit {
			return admin.Response{}, fmt.Errorf("投稿超过当前正文上限，不能批准")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO notices(id,label,title,published_at,detail_url,is_page,content,created_at,updated_at)
			SELECT id,CASE WHEN label='' THEN '用户投稿' ELSE label END,title,published_at,CASE WHEN detail_url='' THEN 'urn:openjwc:submission:'||id ELSE detail_url END,is_page,content,?,?
			FROM user_submissions WHERE id=?`, now(), now(), request.ID)
		if err != nil {
			return admin.Response{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO notice_attachments(notice_id,position,url)
			SELECT s.id,j.key,j.value FROM user_submissions s,json_each(s.attachments) j WHERE s.id=?`, request.ID)
		if err != nil {
			return admin.Response{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE user_submissions SET status=?,review=?,updated_at=? WHERE id=? AND status='pending'", decision, request.Values["review"], now(), request.ID); err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}
