# 外部爬虫 NDJSON 协议

## 边界与配置

OpenJWC 保留内置 `jwc`、`cs`、`xsxy` 适配器，同时可以在一次任务中串行运行最多 16 个外部爬虫。外部程序是服务管理员明确安装的**受信任本机代码**：主程序不会把 SQLite 连接、管理 socket 或模型凭据作为协议参数传入，但同一 Unix 账号下的进程并不构成安全沙箱。系统级部署应把程序放在 `/usr/local/libexec/openjwc` 等服务账号可执行、不可写且不受 `ProtectHome=true` 阻止的位置。

配置使用 `OPENJWC_CRAWLER_PROGRAMS` JSON 数组；名称必须为 1～64 位小写 ASCII 字母、数字、点、下划线或连字符，不能占用内置名称。路径必须是规范绝对路径；参数由 `execve` 原样传入，不经过 shell。配置会出现在 systemd unit 中，**不得把令牌、密码或 Cookie 放入路径或参数**：

```bash
export OPENJWC_CRAWLER_PROGRAMS='[
  {"name":"physics","path":"/usr/local/libexec/openjwc/physics-crawler","args":["--campus","main"]},
  {"name":"graduate","path":"/usr/local/libexec/openjwc/graduate-crawler"}
]'
```

`openjwc service install` 会把配置保存进私有部署记录和 unit；更改已安装部署仍需显式卸载后重装。前台 `serve` 直接读取当前环境。未配置时不会发现或执行 PATH 中的任意程序。

所有爬虫共享服务拥有的单任务队列和十分钟总预算。内置来源和外部程序按配置顺序串行执行；一个来源失败不会撤销已保存资讯，后续来源仍会尝试。取消任务会向外部程序的独立进程组发送 SIGKILL，不遗留调用方拥有的后台任务。

## 标准输入请求

每次运行启动一个新进程。主程序向 stdin 写入且只写入一个 JSON 行，然后关闭 stdin：

```json
{"version":1,"type":"crawl.request","run_id":"...","source":"physics","since":"2026-08-15T10:00:00Z","max_pages":100}
```

- `run_id` 只用于该次运行日志关联，不是凭据。
- `source` 等于配置名称，输出不能改写来源。
- `since` 是 RFC 3339 回溯边界；程序决定如何处理置顶和无日期条目。
- `max_pages` 是扫描上限，不保证每站都存在传统分页。

## 标准输出事件

stdout 只能输出 UTF-8 NDJSON，每行一个对象。版本固定为 1，未知字段、空终止、终止后数据、超过 1 MiB 的单行、32 MiB 总输出、20000 个事件或 10000 条资讯都会使该来源失败。诊断日志应写 stderr；当前服务不把 stderr 内容转发到客户端，以免泄露外部程序环境信息。

进度字段是非递减累计值；每个字段均可省略以保留上一次值：

```json
{"version":1,"type":"progress","scanned":12,"skipped":3,"failed":1,"detail":"第 2 页"}
```

资讯事件不接受外部 ID。主程序校验字段，以规范 `detail_url` 的 SHA-256 生成兼容 ID，再通过领域构造器写入 SQLite：

```json
{"version":1,"type":"notice","notice":{"label":"培养通知","title":"选课安排","published_at":"2026-09-14T00:00:00Z","detail_url":"https://example.edu/notices/42","is_page":true,"content":"正文","attachments":[{"name":"附件","url":"https://example.edu/files/a.pdf"}]}}
```

`published_at` 必须是 RFC 3339。标题最多 1024 字节、标签 256 字节、正文 2 MiB、详情 URL 2048 字节、附件最多 50 个；URL 只能是无凭据的绝对 HTTP(S) 地址。所有 JSON 字段均使用示例中的小写 snake_case。

正常结束必须输出终止事件。示例重复最终累计值；未变化的累计字段也可以省略：

```json
{"version":1,"type":"completed","scanned":12,"skipped":3,"failed":1,"detail":"抓取完成"}
```

无法可信完成时输出：

```json
{"version":1,"type":"failed","scanned":12,"skipped":3,"failed":1,"detail":"上游栏目结构变化"}
```

随后以 0 状态退出；进程非零退出、无终止事件、终止后的空行或数据、累计值回退或其他协议错误同样记为失败。`detail` 最多 512 个 Unicode 字符，控制字符会被清理。`saved` 由主程序按实际成功写入计算，外部程序不能声明。

## 本地协议验收

`tools/crawlmock` 是开发示例，不编入生产二进制。先构建绝对路径的示例，再使用临时数据库运行：

```bash
go build -o /tmp/openjwc-crawlmock ./tools/crawlmock
export OPENJWC_CRAWLER_PROGRAMS='[{"name":"local-demo","path":"/tmp/openjwc-crawlmock","args":["--label","协议测试"]}]'
export OPENJWC_DATA_DIR="$(mktemp -d)"
export OPENJWC_HTTP_ADDRESS=127.0.0.1:18081
./bin/openjwc serve
```

另一个终端继承相同变量后运行 `./bin/openjwc`，进入「爬虫任务」按 `c` 派发、`r` 刷新；完成后可在「资讯」看到“外部爬虫协议测试”。只在临时目录使用示例，它会写入一条测试资讯。
