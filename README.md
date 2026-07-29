# OpenJWC WebAPI Go

OpenJWC WebAPI 的 Go 重置版。本仓库当前处于基础架构阶段，目标是在保持客户端、管理前端、爬虫和 QQ 机器人兼容性的前提下，逐步替换原 Python 后端。

## 当前内容

- 使用标准库 `net/http` 和 `log/slog` 的可运行服务骨架
- 支持优雅关闭的 `openjwc-api` 入口
- 通知、API Key、投稿和对话消息领域模型
- 通知与投稿仓储端口
- SQLite 初始迁移设计
- OpenAPI 健康检查契约
- 架构、相关仓库和候选依赖调研

完整调研与架构决策见 [`docs/architecture.md`](docs/architecture.md)。

## 目录结构

```text
.
├── api/openapi/                 OpenAPI 契约
├── cmd/openjwc-api/             服务进程入口
├── docs/                        架构与调研文档
├── internal/
│   ├── app/                     依赖组装
│   ├── config/                  环境配置
│   ├── domain/
│   │   ├── access/              API Key 与设备配额
│   │   ├── chat/                对话消息
│   │   ├── notice/              通知与仓储端口
│   │   └── submission/          投稿审核与仓储端口
│   └── transport/httpapi/       HTTP 路由与处理器
├── migrations/                  SQLite 迁移
├── data/                        本地运行数据
└── logs/                        本地运行日志
```

只在功能落地时创建对应包，避免维护没有实现的预设目录。

## 环境要求

- Go 版本见 `go.mod`
- 当前阶段不需要第三方依赖或外部服务

## 本地运行

```bash
cp .env.example .env
set -a && . ./.env && set +a
go run ./cmd/openjwc-api
```

默认监听 `:8080`。验证健康状态：

```bash
curl http://localhost:8080/healthz
```

预期响应：

```json
{"status":"ok"}
```

## 开发检查

```bash
make check
```

也可以分别执行：

```bash
gofmt -w $(find . -name '*.go' -type f)
go vet ./...
go test ./...
```

详细开发规则见 [`AGENTS.md`](AGENTS.md)。

## 后续里程碑

1. 根据旧版真实响应样例补全 OpenAPI 契约和兼容测试。
2. 接入 SQLite 驱动与迁移工具，实现通知只读接口。
3. 实现 API Key、设备绑定、管理员认证和投稿审核。
4. 接入 Rust 爬虫同步，再实现 LLM 与向量检索适配器。
5. 更新私有部署仓库并执行端到端迁移验证。
