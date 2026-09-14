package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store 由服务进程独占生命周期，使用单写连接和独立只读连接池。
type Store struct {
	reader *sql.DB
	writer *sql.DB
	path   string
}

// Open 打开私有目录中的数据库并完成完整性检查与事务迁移。
func Open(ctx context.Context, path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析数据库路径: %w", err)
	}
	if _, markerErr := os.Lstat(filepath.Join(filepath.Dir(absolute), "recovery-in-progress")); !errors.Is(markerErr, os.ErrNotExist) {
		return nil, fmt.Errorf("数据库恢复未完成，拒绝启动或创建空库")
	}
	if err = privateFile(absolute); err != nil {
		return nil, err
	}
	writer, err := sql.Open("sqlite", connectionURL(absolute, false))
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	store := &Store{writer: writer, path: absolute}
	if err = store.initialize(ctx); err != nil {
		writer.Close()
		return nil, err
	}
	store.reader, err = sql.Open("sqlite", connectionURL(absolute, true))
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("打开只读池: %w", err)
	}
	store.reader.SetMaxOpenConns(8)
	store.reader.SetMaxIdleConns(8)
	if err = store.reader.PingContext(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("连接只读池: %w", err)
	}
	return store, nil
}

// privateFile 拒绝符号链接和宽松目录权限，防止凭据与 WAL 泄露。
func privateFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建数据库目录: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("检查数据库目录: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("数据库目录必须是权限 0700 的真实目录: %s", dir)
	}
	info, err = os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("数据库必须是权限 0600 的普通文件: %s", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查数据库文件: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("创建数据库文件: %w", err)
	}
	return file.Close()
}

// connectionURL 为每条连接设置外键、忙等待和同步策略。
func connectionURL(path string, readonly bool) string {
	location := url.URL{Scheme: "file", Path: path}
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "synchronous(FULL)")
	if readonly {
		query.Set("mode", "ro")
		query.Add("_pragma", "query_only(1)")
	} else {
		query.Set("_txlock", "immediate")
	}
	location.RawQuery = query.Encode()
	return location.String()
}

// initialize 在对外接流量前验证数据库并升级结构。
func (store *Store) initialize(ctx context.Context) error {
	var mode string
	if err := store.writer.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return fmt.Errorf("启用 WAL: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("SQLite 未进入 WAL 模式: %s", mode)
	}
	if err := checkDatabase(ctx, store.writer, "PRAGMA quick_check"); err != nil {
		return err
	}
	return store.migrate(ctx)
}

// checkDatabase 校验数据库页结构，损坏时拒绝继续提供服务。
func checkDatabase(ctx context.Context, db *sql.DB, statement string) error {
	rows, err := db.QueryContext(ctx, statement)
	if err != nil {
		return fmt.Errorf("执行数据库检查: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("读取数据库检查结果: %w", err)
		}
		if result != "ok" {
			return fmt.Errorf("数据库完整性检查失败，请离线恢复备份")
		}
	}
	return rows.Err()
}

// Check 执行完整页校验和外键校验，不自动覆盖损坏的数据。
func (store *Store) Check(ctx context.Context) error {
	if err := checkDatabase(ctx, store.reader, "PRAGMA integrity_check"); err != nil {
		return err
	}
	rows, err := store.reader.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("检查外键: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("数据库存在外键约束损坏")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if _, err := store.writer.ExecContext(ctx, "INSERT INTO notice_search(notice_search,rank) VALUES('integrity-check',1)"); err != nil {
		return fmt.Errorf("全文索引完整性检查失败: %w", err)
	}
	return nil
}

// Close 在所有请求和后台工作退出后释放连接池。
func (store *Store) Close() error {
	var err error
	if store.reader != nil {
		err = store.reader.Close()
	}
	return errors.Join(err, store.writer.Close())
}

// now 返回统一的 UTC 持久化时间。
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
