# 依赖审查

Go 最低版本保持 `1.26.5`，没有为了依赖升级工具链要求。直接依赖全部锁定版本并由 `go.sum` 校验。

| 依赖 | 版本 | 标准库不足之处 | 维护与许可证检查 |
| --- | --- | --- | --- |
| `modernc.org/sqlite` | `v1.58.0` | `database/sql` 不包含 SQLite 驱动；需要纯 Go、WAL、FTS5 | 活跃发布；检查了模块声明、历史撤回说明、BSD-3-Clause 与 SQLite/附带组件许可 |
| `github.com/charmbracelet/bubbletea` | `v1.3.10` | 标准库没有终端事件循环、窗口事件与 TUI 生命周期 | 用户指定框架，选择稳定 v1；MIT；检查模块声明与许可证 |
| `github.com/charmbracelet/bubbles` | `v1.0.0` | 标准库没有表格、viewport、输入与键位帮助组件 | 检查本地模块声明及 MIT 许可证；该版本明确依赖现有 Bubble Tea v1.3.10，最低 Go 1.24.2，不提高本项目要求 |
| `github.com/charmbracelet/x/ansi` | `v0.11.6` | 按终端单元宽度处理 ANSI 与 Unicode | Charmbracelet 维护，MIT；Bubbles 配套版本 |
| `github.com/charmbracelet/x/term` | `v0.2.2` | 探测终端类型及尺寸，帮助和管道输出分离 | Charmbracelet 维护，MIT；Bubbles 配套版本 |
| `github.com/charmbracelet/lipgloss` | `v1.1.0` | 为 Bubble Tea 提供终端布局、颜色与折行 | 与所选 Bubble Tea 配套；MIT；已是其依赖 |
| `golang.org/x/crypto` | `v0.57.0` | 标准库没有 bcrypt，需要验证旧密码哈希 | Go 团队维护；BSD-3-Clause |
| `golang.org/x/net` | `v0.58.0` | 标准库没有容错 HTML DOM 解析器 | Go 团队维护；BSD-3-Clause；只使用 HTML 解析 |

已检查 `go list -m all` 与各模块 `go.mod`。主要运行时间接依赖分为：

- SQLite 的纯 Go libc、内存和数学适配组件；没有数据库守护进程，也不要求动态加载系统 SQLite。
- Bubble Tea 的终端、ANSI、字符宽度与跨平台输入组件；不包含浏览器前端。
- Go 的 `x/sys`、`x/text` 等底层库。

`go mod tidy` 还可能下载上游测试/生成器依赖，这不代表它们被链接进服务。实际运行时依赖许可证由以下命令从 `go list -deps -json ./cmd/openjwc` 收集：

```bash
go run ./tools/licenses
```

结果保存为 `internal/license/third_party.txt` 并嵌入二进制，`openjwc licenses` 可查看。移植的官方爬虫栏目配置另保留 `OpenJWC/JwcCrawler` 的 MIT 许可证。外部爬虫协议和 `tools/chatmock`、`tools/crawlmock` 只使用 Go 标准库，不增加生产依赖；操作者需自行审核所配置外部二进制的许可证与供应链。

没有引入通用 Agent SDK、JWT 库、SQL 生成器、迁移框架、Web 路由框架或真实 shell 解释器。当前规模使用标准库 HTTP、版本化 SQL 和显式工具白名单即可。

许可证和维护状态检查不等于漏洞扫描或供应链审计。升级依赖后应重新生成许可材料、运行测试和竞态检查，并在发布流程执行 `govulncheck`。

本轮检查了 Bubbles 的依赖声明，新增/升级的运行时间接依赖包括 Unicode 显示宽度、grapheme 分段、clipboard 和终端组件；未加入浏览器或守护进程运行时。维护归属和配套版本来自模块声明与项目归属，不代表完成了漏洞数据库扫描。最终许可证清单以生成器读取的实际链接依赖为准。
