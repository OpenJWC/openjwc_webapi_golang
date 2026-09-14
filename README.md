# OpenJWC Go

OpenJWC 的独立 Go 后端：**一个二进制、SQLite 本地数据、Bubble Tea 管理终端**。无需 Python、Node、Pi、LanceDB、独立数据库服务或 Docker。当前运行目标为 Linux。

## 能力

- 保留客户端 v1/v2 路径、主要响应字段、设备协议及旧 Android 原始文本 SSE；契约见 [`api/openapi/openapi.yaml`](api/openapi/openapi.yaml)。
- **不提供管理 Web API、管理 JWT、静态前端、Swagger UI 或浏览器管理面板**。
- 注册审核、用户启停与权限、API Key、投稿审核、资讯、系统设置、日志摘要、备份和恢复均有 TUI 入口。
- SQLite WAL、独立只读池、单写连接、事务迁移、脚本摘要验证、全文索引校验和重建、在线一致备份、离线恢复。
- 原生 Go Agent loop：OpenAI 兼容模型接口，只能使用资讯虚拟文件系统的 `ls / grep / cat / head`，不能执行真实 Bash。
- 内置三个学校站点爬虫；每日按配置时间总结前一日资讯，失败可重试，已完成日报不重复生成。

这是按新需求重新实现的服务，不是对 Python 内部模块逐一翻译。**兼容性变化、资源上限与未验证项见 [迁移说明](docs/migration.md) 和 [验证说明](docs/verification.md)**，上线前请先在测试环境演练。

## 构建与运行

Go 最低版本保持 `go.mod` 中的 `1.26.5`。

```bash
make check
make race
make build

./bin/openjwc serve  # 前台调试；后台部署使用下文的 service install / start
```

默认监听 `127.0.0.1:8080`，数据目录为当前服务账号的 `~/.local/state/openjwc`。无需从项目目录运行，迁移 SQL、时区数据和许可证已编入二进制。

```bash
curl http://127.0.0.1:8080/healthz
# {"status":"ok"}
```

另一个终端中，以**同一服务账号**执行：

```bash
./bin/openjwc
# 或 ./bin/openjwc tui
```

安装到 PATH 后直接输入 `openjwc`。退出 TUI、断开 SSH 不会关闭由 systemd 托管的后端。

## TUI 操作

- Bubbles 表格按中文/emoji 显示宽度对齐；窄屏隐藏低优先级列，Enter 查看完整记录。
- 默认 Vim 普通模式：`h/l` 或方向键切换页面，`j/k` 选择，`gg/G` 首尾，`Ctrl+d/u` 半页，`?` 帮助。
- `[ / ]` 仅对有前后页的数据库列表生效；单页、空表和非分页页面不会翻页。
- `/` 过滤**当前页**，不是全库搜索；Esc 清除过滤。
- `Enter` 查看详情；详情使用 `j/k`、`gg/G`、半页键滚动，Esc 返回。
- 表单默认普通模式，`j/k/Tab` 切换字段，`i` 开始输入；Esc 返回普通模式，再 Esc 取消表单。
- 单行输入 Enter 完成编辑，多行输入 Enter 换行、`Ctrl+s` 完成编辑，`Ctrl+u` 清空。
- 最后一个确认字段通过 `i` 输入 `yes`，Esc 回普通模式后 Enter 提交。危险动作不因普通导航误触发。
- 日报生成改为 `S`，避免与 `gg` 冲突；其他业务键以当前页面帮助为准。
- 首页「服务管理」可操作用户级部署；「爬虫任务」可查询后台进度并显式取消，关闭 TUI 不取消已接纳的爬虫任务。
- `q` 退出界面；`Ctrl+c` 退出并取消尚未完成的前台管理请求。退出后显示简洁摘要，不回显密钥。

### 首次配置

1. 在客户端提交注册申请，在「注册审核」页面批准。
2. 在「用户权限」页面独立配置启用、浏览资讯、智能问答、投稿和设备上限。新用户默认允许资讯与投稿，**问答默认关闭**；问答同时要求资讯权限。
3. 在「系统设置」配置 `llm_base_url`、`llm_api_key`、`llm_model` 和 `system_prompt`。服务端点填写 API 前缀，例如 `https://api.openai.com/v1`。
4. 在「爬虫任务」启动后台抓取，按 `r` 查询来源进度，或启用 `crawler_enabled`。
5. 配置 `daily_enabled=true`；默认 `Asia/Shanghai` 每天 `00:10` 生成**前一日**日报。`daily_prompt` 控制日报侧重。
6. API Key 创建后仅显示一次；通过「API Key」页面的 `b` 关联用户。关联后继承该用户的最新权限，并按密钥自身配额绑定设备。

供应商凭据不出现在设置列表或审计记录中；数据库和备份仍须按秘密材料保护。

## 配置与部署

| 环境变量 | 默认值 / 说明 |
| --- | --- |
| `OPENJWC_DATA_DIR` | `~/.local/state/openjwc`；必须为私有 `0700` 目录 |
| `OPENJWC_HTTP_ADDRESS` | `127.0.0.1:8080` |
| `OPENJWC_TLS_CERT` | 可选 PEM 证书路径 |
| `OPENJWC_TLS_KEY` | 可选 PEM 私钥路径；与证书同时配置 |
| `OPENJWC_CRAWLER_PROGRAMS` | 可选外部爬虫 JSON 数组；只接受显式名称、绝对路径和参数，不经过 shell |

`.env` 不会被隐式读取，需要由 shell 或服务管理器注入。模型设置、提示词、内置爬虫和日报开关由 TUI 保存到 SQLite；外部爬虫程序属于部署配置，通过环境变量显式注入并在安装时固化。

默认推荐用户级 systemd 部署，完整说明见 [服务管理](docs/service.md)：

```bash
# 必要时由管理员显式授权；程序不自动提权：
sudo loginctl enable-linger "$USER"
openjwc service install --now
openjwc status
openjwc stop
openjwc start
openjwc service uninstall  # 停止并取消部署，保留数据库/备份/配置
```

安装后使用稳定的二进制路径。`--system` 显式选择系统级服务，需要专用账号和 root 授权；旧手工 unit 不会被自动接管或覆盖。`-h/--help`、子命令帮助、`--version` 可独立使用；交互终端帮助及退出摘要使用块状 OpenJWC 字标，支持 `NO_COLOR` 和 `--no-banner`，管道输出不包含字标装饰。

服务账号已存在时不要重复创建。公开部署应配置内置 TLS 或可信反向代理，不要明文传输凭据；反向代理并非运行依赖。代理部署下登录限流默认按直连来源地址生效，不信任任意 `X-Forwarded-For`。

## 备份与恢复

- 「概览」→ `b`：输入新备份路径，在线生成一致快照并验证；目标不能已存在，目录权限须为 `0700`。
- 「概览」→ `c`：SQLite 页、外键和 FTS 内容一致性检查。
- 「概览」→ `e`：从资讯主表重建派生全文索引。
- 停止服务后运行 `openjwc recover`，进入离线恢复 TUI。恢复先校验副本，旧库及 WAL/SHM 保留在 `recovery-*` 隔离目录。
- 同一目录有进程锁，不能同时运行两个服务或在线恢复。

请将备份复制到独立存储，并定期演练恢复。WAL 不能代替备份，也不能防止磁盘硬件损坏。

## 开发与验证

```bash
make check
make race
make bench
make build
bin/openjwc licenses
```

测试使用临时数据库和模拟模型，不访问仓库 `data/`、真实账号或收费模型。中文注释和文件行数规则由 `internal/quality/style_test.go` 自动检查。

- [服务安装、启停与卸载](docs/service.md)
- [Chat 事件、客户端示例、VFS 与本地测试](docs/chat-events.md)
- [外部爬虫 NDJSON v1 协议](docs/crawler-protocol.md)
- [架构](docs/architecture.md)
- [迁移与兼容差异](docs/migration.md)
- [验证与容量边界](docs/verification.md)
- [依赖审查](docs/dependencies.md)
- [独立可读性审查与修订](docs/readability-review.md)
