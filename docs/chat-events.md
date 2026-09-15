# Chat 事件与检索协议

## 两个入口，一个 Agent 内核

- `POST /api/v1/client/chat`：保留旧 JSON 回复和 Android raw-text SSE 格式，只有答案和旧心跳，不插入工具 JSON。
- `POST /api/v2/client/chat`：标准 SSE + JSON，始终返回事件流，输入的 `stream` 不改变此端点的响应类型。
- 两者要求当前浏览及问答权限，共用每用户一个在途请求与全局四个 Agent。轮数、工具数、工具结果和超时由本地管理员在系统设置中调整，但始终受编译期硬上限约束。
- 没有向量数据库或 embedding。独立 `/api/v1/client/notices/search` 继续为快速词法检索，不调用模型。实际旧字段 `similarity_score=1`、`distance=0` 是弃用占位，不是相似度、距离或置信度；`min_similarity` 只沿用占位值筛选逻辑。响应头明确 `X-OpenJWC-Search-Mode: lexical` 和 `X-OpenJWC-Similarity: deprecated-placeholder`。

## 事件格式

```text
id: <run_id>:<sequence>
event: tool.started
data: {"version":1,"run_id":"...","sequence":2,"type":"tool.started","tool_id":"tool-1","name":"bash","summary":"ls /recent"}

```

| type | 含义 |
| --- | --- |
| `run.started` | 已进入一次事件运行；包含答案交付方式 |
| `tool.started` | 工具尝试开始，参数摘要已通过基础校验，最终路径与参数仍可能被工具拒绝 |
| `tool.completed` | 对应工具结束；同一 `tool_id`，状态为 completed 或 failed，可含安全 code 与 duration_ms |
| `answer.delta` | 可展示的最终答案文本；delivery 指明 buffered-final 或 streaming-final |
| `run.completed` | 运行成功终止 |
| `run.failed` | 运行失败终止，稳定 code 和安全摘要，不泄露供应商响应 |

版本为 1；sequence 从 1 单调递增，run_id 每次独立，tool_id 在运行内唯一。工具失败可以作为观察交给模型继续检索，不等于整个运行失败。心跳是 `: ping` 注释，无需展示。SSE id 仅用于定位，不支持重放或断线续跑。

工具可用轮始终使用 `delivery: buffered-final`：模型可能在同轮 content 后才声明工具调用，不能提前公开中间正文。若工具轮数、工具调用数或累计工具结果预算耗尽，服务端只会额外尝试一次 `tool_choice: none` 的收束回答；该轮 SSE 分片以 `delivery: streaming-final` 真实转发。收束轮不能调用工具，也不会在模型不可用、协议错误、取消或总超时后重试。非流式供应商回退仍标记 `buffered-final`。

管理员可在本地 TUI 设置 `agent_max_model_rounds`、`agent_max_tool_calls`、`agent_max_tools_per_round`、`agent_max_tool_result_bytes`、`agent_max_total_tool_bytes`、`agent_model_timeout_seconds` 与 `agent_run_timeout_seconds`。移动端请求不能修改它们；每项均有编译期硬上下限，且累计工具结果不得低于单次上限、总超时不得低于模型超时。

流开始前的身份、参数和每用户容量错误使用 HTTP 状态码。开始后的错误使用 `run.failed`；连接中断而没有终止事件应显示“未完成”，不能把已有文本当作成功答案。`streaming-final` 已发出的分片若随后收到 `run.failed`，同样属于未完成，不能保存为成功答案。断连会取消该请求；队列最多八个事件，每次网络写入有十秒期限。

客户端只渲染工具名称、受限摘要、耗时、状态、安全 code 及答案；不应使用 innerHTML。协议不包含隐藏思考、系统提示词、完整工具正文或模型凭据。工具轨迹只属于本地展示状态，不能写回下一轮 history；history 只允许已完成的 user/assistant 消息。

## 客户端消费示例

Android Kotlin 客户端的登录、OkHttp POST SSE、协议状态机、ViewModel、错误处理和 v1 迁移方案见 [`android-agent-chat.md`](android-agent-chat.md)。

下面只展示协议接入，不代表仓库之外的 Android 或机器人客户端已经完成适配：

```javascript
const response = await fetch('/api/v2/client/chat', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
  body: JSON.stringify({ user_query: question, history: [] }),
  signal: abortController.signal,
});
if (!response.ok) throw new Error(`HTTP ${response.status}`);
const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '', terminal = false;
try {
  while (!terminal) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer = (buffer + decoder.decode(value, { stream: true })).replace(/\r\n/g, '\n');
    let split;
    while ((split = buffer.indexOf('\n\n')) >= 0) {
      const frame = buffer.slice(0, split);
      buffer = buffer.slice(split + 2);
      const data = frame.split('\n').filter(line => line.startsWith('data:'))
        .map(line => line.slice(5).replace(/^ /, '')).join('\n');
      if (!data) continue;
      const event = JSON.parse(data);
      if (event.version !== 1) throw new Error('Unsupported event version');
      renderEvent(event);
      terminal = event.type === 'run.completed' || event.type === 'run.failed';
    }
  }
  if (!terminal) throw new Error('Chat stream interrupted');
} finally {
  await reader.cancel();
}
```

## 本地端到端测试

仓库提供 `tools/chatmock`，它是开发测试程序，不会编入 `bin/openjwc`。测试服务器固定先请求 `ls /recent`，收到工具结果后再返回答案，因此可以同时验证模型 SSE、Agent 循环、VFS、公开认证和 v2 事件。请使用临时数据目录，避免创建测试账号或设置到真实数据库。

依次打开三个终端。终端一运行仅监听 loopback 的模拟模型：

```bash
go run ./tools/chatmock -listen 127.0.0.1:18080
```

终端二构建并以前台模式运行临时 OpenJWC：

```bash
make build
export OPENJWC_DATA_DIR="/tmp/openjwc-chat-local-$(id -u)"
# 若该目录已用于旧测试，请换一个明确的新后缀；不要指向真实数据目录。
install -d -m 0700 "$OPENJWC_DATA_DIR"
export OPENJWC_HTTP_ADDRESS=127.0.0.1:18081
printf '终端三必须使用同一路径：%s\n' "$OPENJWC_DATA_DIR"
./bin/openjwc serve
```

终端三复用该目录，配置模拟模型并创建一次性测试用户。以下管理调用只经过权限为 0600 的本地 Unix socket；注册 ID 和用户 ID 为 1 仅因为数据库是全新的：

```bash
export OPENJWC_DATA_DIR="/tmp/openjwc-chat-local-$(id -u)"
SOCKET="$OPENJWC_DATA_DIR/admin.sock"
test -S "$SOCKET" || printf '未找到管理 socket：%s\n' "$SOCKET"
admin() {
  test -S "$SOCKET" || { printf '管理 socket 不可用：%s\n' "$SOCKET" >&2; return 1; }
  curl -fsS --unix-socket "$SOCKET" \
    -H 'Content-Type: application/json' -d "$1" \
    http://localhost/command
}

admin '{"action":"set-setting","id":"llm_base_url","values":{"value":"http://127.0.0.1:18080/v1"}}'
admin '{"action":"set-setting","id":"llm_api_key","values":{"value":"local-test"}}'
admin '{"action":"set-setting","id":"llm_model","values":{"value":"local-test"}}'

curl -fsS -H 'Content-Type: application/json' \
  -d '{"username":"chat-local","email":"chat-local@example.com","password_hash":"local-password"}' \
  http://127.0.0.1:18081/api/v2/client/auth/register
admin '{"action":"review-registration","id":"1","values":{"decision":"approved"}}'
admin '{"action":"permissions","id":"1","values":{"active":"true","browse":"true","chat":"true","submit":"true","max_devices":"3"}}'

TOKEN="$(curl -fsS -H 'Content-Type: application/json' -H 'X-Device-ID: chat-local-device' \
  -d '{"account":"chat-local","password_hash":"local-password","device_name":"local-terminal"}' \
  http://127.0.0.1:18081/api/v2/client/auth/login | jq -r .data.token)"
curl -NfsS -H "Authorization: Bearer $TOKEN" -H 'X-Device-ID: chat-local-device' \
  -H 'Content-Type: application/json' -d '{"user_query":"验证本地 Agent 工具调用"}' \
  http://127.0.0.1:18081/api/v2/client/chat
```

预期事件顺序为 `run.started → tool.started → tool.completed → answer.delta → run.completed`，工具摘要为 `ls /recent`，交付方式为 `buffered-final`。测试结束后在终端二和终端一按 Ctrl+C；确认路径仍以 `/tmp/openjwc-chat-local-` 开头后删除该测试目录。

如果 `SOCKET` 意外变成 `/admin.sock`，说明 `OPENJWC_DATA_DIR` 为空；不要继续注册或登录，应先修正目录并通过 `test -S`。函数结尾的 shell 分隔符是语法，不应写成 URL 的一部分；多行定义可以避免误写成 `http://localhost/command\;`。

也可以把模型设置换成支持 OpenAI `/v1/chat/completions`、SSE 和 tools 的 Ollama 或 llama.cpp。远程地址仍强制 HTTPS；明文 HTTP 只允许 `localhost`、`127.0.0.0/8` 或 `::1`，且 `llm_api_key` 仍需填写任意非空测试值。真实供应商测试只需配置其 HTTPS API 前缀、模型和密钥，其他认证与 curl 步骤不变。

仅需快速回归而不启动进程时运行：

```bash
go test ./internal/service/agent ./internal/transport/httpapi
```

## VFS 与时效性

规范文件身份始终为 `/notices/<base64url-ID>.md`。以下目录只是查询视图，不落地 Markdown、不挂载宿主目录：

```text
ls /
ls /recent
ls /by-label
ls /by-label/<URL编码类别>
ls /by-date
ls /by-date/2026/09
find /recent -type f
grep '考试' /notices --from 2026-09-01 --to 2026-09-30 --sort newest | head -n 10
ls /notices --label '教务信息' --page 2
head -n 20 /notices/<编码>.md
cat /notices/<编码>.md 12000 12000
```

- 默认最新优先，可显式选择 `relevance`；词法匹配由 FTS5/BM25 或短词子串查询实现。
- `find` 仅列出虚拟目录中的规范资讯文件，可选 `-type f`；最多一个管道，且右侧只能为 `head -n N` 或 `head -N`。它们都由 Go 的 VFS 解释，不执行宿主 Shell、不支持重定向、变量、命令替换、`-exec` 或宿主路径。
- `--to` 包含当天；近期目录为最近 30 个 UTC 日期；月份目录按实际数据覆盖范围生成，可能存在空月份。
- 路径范围与显式日期求交集，不能通过参数越过目录含义；最多每页 20 条。
- 资讯 ID 不随目录变化；类别仍采用数据库当前单个 label，不引入多标签模型。
- cat 的偏移与长度以 Unicode 字符计，单次最多 12000 字符，有明确续读位置；head 只读开头。工具总预算仍然有效，分段能力不意味着无限上下文。
- 正文包含发布日期、官方链接与附件链接；附件未下载、未解析。
- 系统提示包含当前时间/时区、库中覆盖范围和最近完整成功抓取时间。它们不是业务截止时间；回答截止日期需要正文证据。未命中应换关键词或扩大历史范围，不能直接断言不存在。
