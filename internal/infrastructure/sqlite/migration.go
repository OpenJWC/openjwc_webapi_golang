package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/migrations"
)

// migrate 在单个写事务内按文件名升级并校验已执行脚本的摘要。
func (store *Store) migrate(ctx context.Context) error {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始数据库迁移: %w", err)
	}
	defer tx.Rollback()
	var versioned, existing int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&versioned); err != nil {
		return err
	}
	if versioned == 0 {
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&existing); err != nil {
			return err
		}
		if existing != 0 {
			return fmt.Errorf("拒绝自动升级未版本化数据库，请使用旧版迁移工具导入新数据库")
		}
	}
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)"); err != nil {
		return err
	}
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		return err
	}
	var applied int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		return err
	}
	if applied > len(files) {
		return fmt.Errorf("数据库版本高于当前二进制，拒绝降级启动")
	}
	for _, name := range files {
		if err = applyMigration(ctx, tx, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// applyMigration 校验脚本不可变性，只执行尚未应用的正向迁移。
func applyMigration(ctx context.Context, tx *sql.Tx, name string) error {
	content, err := migrations.Files.ReadFile(name)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	checksum := hex.EncodeToString(digest[:])
	var stored string
	err = tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE name=?", name).Scan(&stored)
	if err == nil {
		if stored != checksum {
			return fmt.Errorf("迁移脚本 %s 与已执行版本不一致", name)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	up, _, _ := strings.Cut(string(content), "-- +goose Down")
	if _, err = tx.ExecContext(ctx, up); err != nil {
		return fmt.Errorf("执行迁移 %s: %w", name, err)
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(name,checksum,applied_at) VALUES(?,?,?)", name, checksum, now())
	return err
}
