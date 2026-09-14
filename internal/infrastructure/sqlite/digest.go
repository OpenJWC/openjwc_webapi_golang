package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/report"
	"time"
)

// DailySources 返回当日稳定排序的资讯标识，超预算时由日报服务拒绝发布。
func (store *Store) DailySources(ctx context.Context, day string) ([]string, error) {
	date, err := time.Parse("2006-01-02", day)
	if err != nil {
		return nil, err
	}
	rows, err := store.reader.QueryContext(ctx, "SELECT id FROM notices WHERE published_at>=? AND published_at<? ORDER BY published_at,id LIMIT 101", date.Format(time.RFC3339), date.AddDate(0, 0, 1).Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DailyStatus 返回持久化日报状态，未生成时返回空字符串。
func (store *Store) DailyStatus(ctx context.Context, day string) (report.Status, error) {
	var status report.Status
	err := store.reader.QueryRowContext(ctx, "SELECT status FROM daily_reports WHERE day=?", day).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return status, err
}

// SaveDaily 保存阶段结果，完成后不可被并发重试覆盖。
func (store *Store) SaveDaily(ctx context.Context, day string, status report.Status, content string, count int) error {
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return err
	}
	if !status.Valid() {
		return fmt.Errorf("日报状态无效")
	}
	_, err := store.writer.ExecContext(ctx, `INSERT INTO daily_reports(day,status,content,source_count,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(day) DO UPDATE SET status=excluded.status,content=excluded.content,source_count=excluded.source_count,updated_at=excluded.updated_at WHERE daily_reports.status<>'completed'`, day, status, content, count, now())
	return err
}

// DailyContent 只读取已完成日报，不暴露生成中的半成品。
func (store *Store) DailyContent(ctx context.Context, day string) (string, bool, error) {
	var content string
	err := store.reader.QueryRowContext(ctx, "SELECT content FROM daily_reports WHERE day=? AND status='completed'", day).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return content, err == nil, err
}

// JobSuccess 返回后台任务最近一次成功时间。
func (store *Store) JobSuccess(ctx context.Context, name string) (time.Time, error) {
	var value string
	err := store.reader.QueryRowContext(ctx, "SELECT last_success FROM job_state WHERE name=?", name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, value)
}

// MarkJobSuccess 在任务完成后记录持久化调度进度。
func (store *Store) MarkJobSuccess(ctx context.Context, name string) error {
	_, err := store.writer.ExecContext(ctx, "INSERT INTO job_state(name,last_success) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET last_success=excluded.last_success", name, now())
	return err
}
