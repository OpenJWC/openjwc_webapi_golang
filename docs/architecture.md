# 架构与依赖调研

## 调研范围

本次设计参考以下项目及官方资料：

- 本地 Python 后端：`/home/moonhalf/Projects/python_projects/OpenJWC-webapi`
- Python 后端仓库：<https://github.com/OpenJWC/OpenJWC-webapi>
- 爬虫：<https://github.com/OpenJWC/JwcCrawler>
- Android 客户端：<https://github.com/OpenJWC/OpenJWCClient>
- 管理前端：<https://github.com/OpenJWC/OpenJWC-web-frontend>
- 私有部署编排：<https://github.com/OpenJWC/OpenJWC-Server>
- QQ 机器人：<https://github.com/OpenJWC/openjwc-qqbot>
- Go 模块布局：<https://go.dev/doc/modules/layout>
- Go 代码评审建议：<https://go.dev/wiki/CodeReviewComments>

## 现有系统结论

Python 后端采用 FastAPI 路由、服务和模型分层，提供客户端与管理员两组 `/api/v1` 接口。主要能力包括通知同步与查询、API Key 及设备绑定、投稿审核、管理员 JWT、系统设置、格言、语义搜索、LLM 对话、监控和日志读取。

当前数据层使用 SQLite，附件和绑定设备以 JSON 字符串保存；向量数据使用 ChromaDB。AI 调用依赖 DeepSeek、智谱和 OpenAI 兼容客户端，爬虫由独立的 Rust 二进制执行。现有项目未配置自动化测试，数据库建表语句中还存在格言表字段逗号缺失等风险，因此重置版不应逐文件翻译，而应先固定契约和领域约束。

相关仓库形成了清晰边界：

- `JwcCrawler` 使用 Rust、reqwest、scraper 和 serde，产出通知数据。
- `OpenJWCClient` 使用 Kotlin、Compose、Retrofit、Room 和 DataStore，消费客户端 API。
- `OpenJWC-web-frontend` 使用 React、Redux Toolkit、Axios、React Hook Form 和 Zod，消费管理 API。
- `OpenJWC-Server` 通过 Docker Compose 编排后端、爬虫工作进程和前端，并共享数据卷。
- `openjwc-qqbot` 使用 NoneBot、HTTPX、APScheduler 和 SQLite，是通知接口的另一个消费者。

这意味着 Go 重置版必须优先保持外部 HTTP 行为和爬虫数据兼容，而不需要保留 Python 内部类结构。

## 架构选择

采用“薄分层 + 按领域聚合”的单体服务，暂不拆分微服务：

```text
cmd/openjwc-api/          进程入口、信号与生命周期
internal/app/             依赖组装
internal/config/          环境配置
internal/domain/          纯领域模型与端口
internal/transport/httpapi/ HTTP 路由与 DTO
api/openapi/              对外 API 契约
migrations/               SQLite 迁移
```

依赖方向为 `transport -> service -> domain`，基础设施适配器实现领域或服务层定义的小接口。领域包不依赖 HTTP、SQL、第三方 SDK 或全局单例。只有进程入口负责组装具体实现。

当前只实现可运行的健康检查和核心领域模型，不提前创建空服务或空适配器。后续按通知、访问控制、投稿、对话的纵向功能逐步增加用例和实现。

## 核心数据设计

首批领域结构包括：

- `notice.Notice`：通知标识、标签、标题、发布日期、详情链接、正文和附件。
- `access.APIKey`：密钥摘要、所有者、启停状态、设备配额和绑定设备。
- `submission.Submission`：投稿内容及 `pending -> approved/rejected` 的单向审核状态机。
- `chat.Message`：限制为 system、user、assistant 三种角色的对话消息。

领域实体隐藏字段，通过带语义的构造函数创建，并在构造时维护不变量。切片和映射在输入、输出时复制，避免外部修改内部状态。

初始迁移对原 SQLite 结构作以下调整：

- 日期统一存为 UTC RFC 3339 文本，Go 内部使用 `time.Time`。
- 附件、API Key 设备绑定改为关联表，避免 JSON 字段无法约束和索引。
- API Key 只保存摘要，不保存可直接使用的明文。
- 审核状态和布尔字段添加 `CHECK` 约束。
- 所有列表关键路径添加组合索引。
- 数据库变更交给版本化迁移，不在应用启动时动态执行建表字符串。

## 依赖策略

当前骨架只使用标准库：

- HTTP：Go 1.22 以后 `net/http.ServeMux` 已支持方法和路径匹配，现阶段无需额外路由库。
- 日志：使用 `log/slog` 输出结构化日志。
- 测试：使用 `testing`，避免在需求尚未稳定时引入断言框架。

进入对应实现阶段后再评估以下依赖，并锁定直接依赖版本：

| 能力 | 首选方案 | 选择理由 |
| --- | --- | --- |
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go，容器构建不依赖 CGO |
| 数据迁移 | `github.com/pressly/goose/v3` | 支持 SQL 迁移和 SQLite |
| JWT | `github.com/golang-jwt/jwt/v5` | API 稳定且维护活跃 |
| 密码哈希 | `golang.org/x/crypto/bcrypt` | 与现有 bcrypt 数据兼容 |
| OpenAPI 生成 | `github.com/oapi-codegen/oapi-codegen/v2` | 契约优先并减少重复 DTO 代码 |
| SQL 生成 | `github.com/sqlc-dev/sqlc` | 查询增多后提供编译期类型检查 |

LLM 与向量检索先定义内部端口，再决定使用官方 SDK还是直接调用 OpenAI 兼容 HTTP API。ChromaDB 的数据兼容和迁移成本需要单独验证，不能在调研不足时替换向量引擎。

## 实施顺序

1. 固定旧版关键响应样例，并补充 OpenAPI 契约测试。
2. 实现 SQLite 迁移、通知仓储和只读通知接口。
3. 实现 API Key 摘要校验、设备绑定和限额事务。
4. 实现管理员认证、设置和投稿审核。
5. 接入爬虫同步，并保证重复导入幂等。
6. 最后接入 LLM、向量检索、监控和部署编排。

每个阶段都应运行格式化、静态检查、单元测试和接口兼容测试；不以一次性完整复刻替代可验证的增量迁移。
