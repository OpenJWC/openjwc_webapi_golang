package migrations

import "embed"

// Files 将版本化 SQL 随二进制分发，部署不需要额外迁移目录。
//
//go:embed *.sql
var Files embed.FS
