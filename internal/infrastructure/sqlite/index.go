package sqlite

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// rebuildIndex 从权威资讯表事务性重建派生全文索引，不修改资讯本身。
func (store *Store) rebuildIndex(ctx context.Context, request admin.Request) (admin.Response, error) {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO notice_search(notice_search) VALUES('rebuild')"); err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}
