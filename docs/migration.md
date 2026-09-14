# 迁移与兼容性

## 已核对来源

| 组件 | 读取基线 |
| --- | --- |
| 本地 Python 后端 | `d1458f820d52e98050c762acc5c89bd0def951be` 对应工作树 |
| 官方爬虫 | `OpenJWC/JwcCrawler@bedeb669ccadd292888a6f357c876769236bc736` |
| Android 客户端 | `OpenJWC/OpenJWCClient@f970dcc1a0973c3108cf9083fec5097c74d72494` |
| QQ 机器人 | `OpenJWC/openjwc-qqbot@9279aa3a17bf3df5e74ff5760c68ebd2ab5f19ea` |

参考了 Python 路由、模型、SQLite schema 与 E2E 测试；Android 的 `net/chat/SendMessage.kt`；QQ 机器人的通知与检索模型；Rust 爬虫的三个站点配置与 URL ID 生成方法。

注意：Python 工作树的多数 v1 资讯/聊天接口已经使用 v2 用户令牌，而 v1 设备接口仍使用 API Key。Go 保留这一区分，并新增“API Key 显式关联用户后继承权限”的访问方式。

## 上线顺序

1. 在测试环境构建并运行 `make check`、`make race`。
2. 备份旧部署。为旧 SQLite 创建**一致快照**，不要只复制仍在写入的主 `.db` 而丢弃 WAL。
3. 启动指向全新私有目录的 Go 服务。**不要把 `OPENJWC_DATA_DIR` 指向旧数据目录**。
4. 在 TUI「概览」→ `i` 输入旧 SQLite 快照路径。使用 systemd 示例时，先将快照放到服务账号可读的 `/var/lib/openjwc/import/`，目录 `0700`、文件 `0600`。
5. 迁移成功后检查用户、待审核注册和 API Key，配置用户权限，并让用户重新登录。
6. 需要给机器人使用的旧 API Key，可在 TUI 显式关联用户。关联变更会清除旧绑定，下一次业务请求重新按配额绑定。
7. 手工运行爬虫重新建立资讯库，核对栏目和中文搜索，再启用定时任务。
8. 配置模型凭据与模型名，在测试账号下验证问答、取消及权限拒绝，之后启用日报。
9. 演练一次在线备份及离线恢复，再切换旧客户端的服务地址。

迁移只读取旧库，不在 Python 仓库内修改文件，不自动迁移任何实际运行数据。失败记录会使本次目标事务整体回滚；不能用“跳过坏记录”掩盖用户丢失。

## 数据范围

导入：

- `users`：ID、用户名、邮箱、bcrypt 密码哈希、启停状态。
- `user_registrations`：ID、资料、密码哈希、审核状态与意见。
- `api_keys`：ID、所有者、启停、配额、历史请求数；明文转换为 SHA-256。
- API Key 的 `bound_devices` JSON 转换成有约束的关联表。

不导入：

- LanceDB、旧 SQLite 资讯缓存、向量、嵌入缓存、标签向量。
- 用户登录令牌、旧会话和管理员 JWT；用户需要重新登录。
- Web 管理员账号/密码、旧模型凭据和旧系统设置；从 TUI 重新配置。
- 历史投稿、历史格言、旧运行日志；如需保留这些额外历史数据，应先另行明确迁移范围，不要删除旧库归档。

首次导入要求目标身份表为空，避免静默覆盖或错误合并同名用户。成功来源记录有幂等标记。新设备与新增权限采用 Go 默认值，问答默认关闭。

## 明确的协议与行为变化

| 项目 | 新行为 |
| --- | --- |
| 管理 API 与浏览器 UI | 全部移除，公开访问返回 404；仅保留客户端兼容范围 |
| 客户端 token | 新登录产生不透明高熵令牌；客户端保存/提交方式不变，旧会话失效 |
| 权限 | 停用账号不能继续用旧令牌；浏览、问答、投稿分开配置 |
| API Key | 仅持久化摘要；新建后需显式关联用户才可访问业务接口；旧 v1 设备查询仍支持已迁移绑定 |
| 搜索 | FTS5 字面检索替代语义/向量检索；旧字段 similarity_score=1、distance=0 是弃用占位，不是语义分数；不保证旧语义召回率或排序 |
| 日期 | 资讯继续返回 `YYYY-MM-DD`；设备时间使用 UTC RFC 3339，而非旧无时区字符串 |
| SSE | 保留 Android 使用的原始文本 `data:` 帧与 `[DONE]`；推理期间空心跳，答案完成后输出，不是供应商逐 token 透传 |
| 校验 | 拒绝超大请求、越界分页、客户端 system/tool 历史；422 使用兼容数组形状但不保证逐字段错误文案与 FastAPI 完全一致 |
| 每日一言 | 返回 TUI 配置的文字/作者，不再隐式请求第三方随机语录 |
| 爬虫 | 默认三站使用原生 HTTP/HTML；可选受信任外部二进制通过有界 NDJSON v1 接入。主程序仍负责校验和 SQLite，不运行 Crawl4AI/浏览器或恢复嵌入流程 |

正常响应的路径和关键字段已由 Go 兼容测试覆盖；这不等同于对所有历史分支、所有不合法输入做过 Python/Go 差分测试。

## 恢复演练

```bash
sudo systemctl stop openjwc
sudo -u openjwc env OPENJWC_DATA_DIR=/var/lib/openjwc openjwc recover
sudo systemctl start openjwc
```

在离线恢复页面输入一致备份路径并确认。旧主库及侧文件保存在 `recovery-*` 内的 `previous.db`、`previous.db-wal`、`previous.db-shm`，不自动删除。

如果断电发生在恢复切换期间，`recovery-in-progress` 会阻止服务创建空库。此时应保留目录现场，根据标记指向的隔离目录检查已完成阶段；**不要盲目删除标记或 WAL**。当前版本对此类恢复中断采取拒绝启动和保留证据策略，而不是猜测并覆盖数据。

## 后续兼容扩展

旧 chat 继续使用原格式，内部是 Agentic 检索，不保证旧 RAG 的召回等价。新的 `/api/v2/client/chat` 单独承载 JSON 工具事件，当前最终答案为 buffered-final；现有客户端不升级也不会收到混入正文的工具 JSON。

独立旧搜索不调用 LLM、不增加问答权限要求。新增响应头明确词法模式，旧分数/距离仅为兼容占位并在契约中标记弃用。没有新增独立 v2 搜索接口。

部署改为 CLI 封装 systemd，`serve` 明确为前台调试入口。已有手工 unit 不会被静默覆盖，卸载默认保留原数据库和配置；迁移前请阅读 [service.md](service.md)。
