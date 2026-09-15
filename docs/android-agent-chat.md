# Android Agent Chat 接入指南

本文面向 Android Kotlin 客户端，说明如何登录、调用 OpenJWC Agent Chat v2、解析工具事件、维护界面状态，并从旧版 v1 Chat 平滑迁移。公开协议以 [`api/openapi/openapi.yaml`](../api/openapi/openapi.yaml) 为准；服务端设计与 VFS 细节见 [`chat-events.md`](chat-events.md)。

## 1. 接入目标

推荐新客户端使用：

```text
POST /api/v2/client/chat
Content-Type: application/json
Accept: text/event-stream
```

v2 始终返回标准 SSE，每个 `data` 字段都是一个 JSON 事件。移动端可以实时展示 Agent 的资讯检索进度，但不会收到隐藏推理、系统提示词、完整工具结果或模型凭据。

工具轮的文字从不公开；所有最终回答都经一次禁用工具的流式轮生成，使用 `streaming-final` 真实逐字输出。仅当供应商返回整段 JSON（不支持 SSE）时才回退为 `buffered-final`。客户端始终应按增量追加正文。

以下内容不是移动端能力：

- 移动端不能直接执行 Agent 的 VFS 工具。
- 移动端不能访问宿主 Shell 或服务器文件系统。
- `tool.started` 的 `summary` 仅用于展示，不是完整检索结果。
- SSE `id` 只用于排障，不支持断线重放。

## 2. 前置条件

用户必须同时满足：

1. 账号处于启用状态；
2. 拥有资讯浏览权限；
3. 拥有智能问答权限；
4. 使用登录时相同的设备 ID；
5. 服务端已配置可用模型及 `llm_api_key`。

问答权限依赖浏览权限。管理员只打开 Chat 而关闭 Browse 时，请求仍会返回 `403`。

### 2.1 Android 网络配置

声明网络权限：

```xml
<uses-permission android:name="android.permission.INTERNET" />
```

生产环境应使用 HTTPS。若测试服务器仍为明文 HTTP，Android 9 及以上默认可能拒绝连接。只在受控测试构建中放行明文流量，例如在 `src/debug/AndroidManifest.xml` 中设置：

```xml
<application android:usesCleartextTraffic="true" />
```

不要把全局明文放行带入正式发布包。密码、Bearer Token、设备 ID 和问答内容通过 HTTP 传输时都可能被窃听或篡改。

## 3. 推荐依赖

示例使用 OkHttp、Kotlin 协程和 kotlinx.serialization。版本应由客户端现有依赖目录统一管理：

```kotlin
implementation("com.squareup.okhttp3:okhttp:<version>")
implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:<version>")
implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:<version>")
```

Chat v2 是带 JSON 请求体的 POST SSE。推荐直接用 OkHttp 读取 `ResponseBody.source()`，而不是假定只支持 GET 的浏览器式 `EventSource`。普通登录接口仍可由 Retrofit 调用。

## 4. 设备身份与登录

客户端首次启动时生成一个随机 UUID，并持久化到 DataStore。不要每次登录或每次问答都生成新值，否则会占用新的设备名额。

```kotlin
val deviceId = UUID.randomUUID().toString()
```

登录接口：

```text
POST /api/v2/client/auth/login
X-Device-ID: <稳定设备ID>
Content-Type: application/json
```

请求模型：

```kotlin
@Serializable
data class LoginRequest(
    val account: String,
    @SerialName("password_hash") val password: String,
    @SerialName("device_name") val deviceName: String,
)

@Serializable
data class LoginData(
    val token: String,
    val username: String,
    val email: String,
)

@Serializable
data class LoginResponse(
    val msg: String,
    val data: LoginData,
)
```

`password_hash` 是为兼容旧协议保留的字段名。客户端应填写用户输入的原始密码材料，不应自行计算 bcrypt。服务端负责验证已有密码摘要。

登录成功后：

- Token 有效期为 30 天；
- 同一设备重新登录会撤销该设备的旧 Token；
- 后续请求应同时发送 Token 与同一 `X-Device-ID`；
- Token 不透明，客户端不得解析或依赖其内部格式；
- 迁移用户必须重新登录，旧登录会话不会迁移。

应把 Token 存入受 Android Keystore 保护的存储，不得写入普通日志、崩溃报告、埋点或 URL。

## 5. Chat 请求契约

```kotlin
@Serializable
data class ChatHistoryMessage(
    val role: String,
    val content: String,
)

@Serializable
data class ChatRequest(
    @SerialName("user_query") val userQuery: String,
    val history: List<ChatHistoryMessage> = emptyList(),
    @SerialName("notice_ids") val noticeIds: List<String> = emptyList(),
    val stream: Boolean = true,
)
```

请求示例：

```json
{
  "user_query": "最近有哪些考试通知？",
  "history": [
    {"role": "user", "content": "只关注本科生"},
    {"role": "assistant", "content": "好的"}
  ],
  "notice_ids": [],
  "stream": true
}
```

边界：

| 字段 | 约束 |
| --- | --- |
| `user_query` | 必填且非空，最多 16000 字节 |
| `history` | 最多 20 条，只允许 `user` 和 `assistant` |
| 历史与问题总量 | 最多 48000 字节 |
| `notice_ids` | 最多 10 个；每个 ID 非空且最多 256 字节 |
| 请求体 | 最多 1 MiB |
| `stream` | v2 中不改变响应类型，始终返回 SSE |

服务端按 Go 字符串长度执行部分边界检查。中文和 emoji 应按 UTF-8 字节数保守计算，而不是只计算 Kotlin `String.length`。

历史消息只发送生成下一轮答案确实需要的最近上下文。不要发送系统消息、工具消息、工具结果或整个本地会话数据库。

## 6. 事件模型

为了兼容未来新增字段，事件类型保留为字符串，不建议直接使用严格枚举反序列化：

```kotlin
@Serializable
data class ChatEvent(
    val version: Int,
    @SerialName("run_id") val runId: String,
    val sequence: Int,
    val type: String,
    @SerialName("tool_id") val toolId: String? = null,
    val name: String? = null,
    val summary: String? = null,
    val status: String? = null,
    @SerialName("duration_ms") val durationMs: Long? = null,
    val text: String? = null,
    val delivery: String? = null,
    val code: String? = null,
)
```

事件含义：

| `type` | 移动端动作 |
| --- | --- |
| `run.started` | 建立运行状态；交付方式以 answer.delta 的 `delivery` 为准 |
| `tool.started` | 按 `tool_id` 新建工具卡片 |
| `tool.completed` | 更新同一工具卡片的状态和耗时 |
| `answer.delta` | 把 `text` 追加到最终答案 |
| `run.completed` | 标记成功并结束流 |
| `run.failed` | 显示安全错误摘要并结束流 |

典型事件流：

```text
id: 25c1...:1
event: run.started
data: {"version":1,"run_id":"25c1...","sequence":1,"type":"run.started"}

id: 25c1...:2
event: tool.started
data: {"version":1,"run_id":"25c1...","sequence":2,"type":"tool.started","tool_id":"tool-1","name":"bash","summary":"ls /recent"}

id: 25c1...:3
event: tool.completed
data: {"version":1,"run_id":"25c1...","sequence":3,"type":"tool.completed","tool_id":"tool-1","name":"bash","status":"completed","duration_ms":8}

id: 25c1...:4
event: answer.delta
data: {"version":1,"run_id":"25c1...","sequence":4,"type":"answer.delta","text":"回答正文","delivery":"buffered-final"}

id: 25c1...:5
event: run.completed
data: {"version":1,"run_id":"25c1...","sequence":5,"type":"run.completed","status":"completed"}

```

工具的 `status=failed` 不等于整次运行失败。模型可能根据失败观察继续选择其他检索方式，客户端应保留失败工具卡片并继续消费后续事件。

## 7. OkHttp 配置

Agent 最长运行两分钟，服务端每十秒发送一次 SSE 注释心跳。客户端应允许长读取，但保留整体调用上限：

```kotlin
val chatHttpClient = OkHttpClient.Builder()
    .connectTimeout(10, TimeUnit.SECONDS)
    .writeTimeout(10, TimeUnit.SECONDS)
    .readTimeout(0, TimeUnit.MILLISECONDS)
    .callTimeout(130, TimeUnit.SECONDS)
    .retryOnConnectionFailure(false)
    .build()
```

不要在网络拦截器中记录 `Authorization`、登录请求体或 Chat 正文。若必须调试，只记录状态码、事件类型、`run_id` 的短前缀和序号。

## 8. SSE 分帧器

网络数据块与 SSE 帧没有一一对应关系。一个 JSON 事件可能被拆成多个 TCP 块，多个事件也可能一次到达。必须按空行分帧，并合并同一帧中的多个 `data:` 行。

```kotlin
data class SseFrame(
    val id: String?,
    val event: String?,
    val data: String,
)

class SseParser {
    private var id: String? = null
    private var event: String? = null
    private val data = mutableListOf<String>()

    fun accept(line: String): SseFrame? {
        if (line.isEmpty()) return dispatch()
        if (line.startsWith(":")) return null

        val separator = line.indexOf(':')
        val field = if (separator < 0) line else line.substring(0, separator)
        var value = if (separator < 0) "" else line.substring(separator + 1)
        if (value.startsWith(" ")) value = value.substring(1)

        when (field) {
            "id" -> id = value
            "event" -> event = value
            "data" -> data += value
        }
        return null
    }

    private fun dispatch(): SseFrame? {
        if (data.isEmpty()) {
            id = null
            event = null
            return null
        }
        return SseFrame(id, event, data.joinToString("\n")).also {
            id = null
            event = null
            data.clear()
        }
    }
}
```

以 `:` 开头的 `: ping` 是心跳注释，应忽略且不刷新界面。Okio 的 `readUtf8Line()` 可以同时处理 LF 和 CRLF。EOF 时不要把未以空行结束的半帧当成成功事件。

## 9. 流式 Repository 实现

以下实现通过有界 Flow 传播背压，在收集协程取消时关闭 HTTP Call：

```kotlin
class ChatHttpException(
    val status: Int,
    val responseText: String,
) : IOException("Chat HTTP $status")

class ChatInterruptedException : IOException("Chat stream ended without terminal event")

class ChatRepository(
    private val baseUrl: HttpUrl,
    private val client: OkHttpClient,
    private val json: Json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
    },
) {
    private val jsonType = "application/json; charset=utf-8".toMediaType()

    fun events(
        token: String,
        deviceId: String,
        input: ChatRequest,
    ): Flow<ChatEvent> = callbackFlow {
        val body = json.encodeToString(input).toRequestBody(jsonType)
        val request = Request.Builder()
            .url(baseUrl.resolve("/api/v2/client/chat")!!)
            .header("Authorization", "Bearer $token")
            .header("X-Device-ID", deviceId)
            .header("Accept", "text/event-stream")
            .post(body)
            .build()
        val call = client.newCall(request)

        call.enqueue(object : Callback {
            override fun onFailure(call: Call, error: IOException) {
                close(error)
            }

            override fun onResponse(call: Call, response: Response) {
                response.use {
                    val responseBody = response.body
                    if (!response.isSuccessful) {
                        val text = responseBody.string().take(4096)
                        close(ChatHttpException(response.code, text))
                        return
                    }
                    val contentType = response.header("Content-Type").orEmpty()
                    if (!contentType.startsWith("text/event-stream")) {
                        close(IOException("Unexpected Content-Type: $contentType"))
                        return
                    }

                    val parser = SseParser()
                    val protocol = ChatProtocolState()
                    try {
                        val source = responseBody.source()
                        while (true) {
                            val line = source.readUtf8Line() ?: break
                            val frame = parser.accept(line) ?: continue
                            val event = json.decodeFromString<ChatEvent>(frame.data)
                            protocol.accept(frame, event)
                            if (trySendBlocking(event).isFailure) {
                                call.cancel()
                                return
                            }
                            if (protocol.terminal) {
                                close()
                                return
                            }
                        }
                        if (!protocol.terminal) close(ChatInterruptedException())
                    } catch (error: Throwable) {
                        close(error)
                    }
                }
            }
        })

        awaitClose { call.cancel() }
    }.buffer(capacity = 8)
}
```

所需主要 import：

```kotlin
import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.channels.trySendBlocking
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.buffer
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.Callback
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
```

这里显式把移动端 Flow 缓冲限制为 8，保持小且有界的背压队列。相邻的 `callbackFlow` 与 `buffer` 会由协程库融合；不要额外叠加无界缓冲，也不要在网络层丢弃工具或终止事件。

## 10. 协议状态校验

客户端应至少验证版本、运行标识、序号、SSE 元数据和工具配对：

```kotlin
class ChatProtocolException(message: String) : IOException(message)

class ChatProtocolState {
    private var runId: String? = null
    private var nextSequence = 1
    private val runningTools = mutableSetOf<String>()
    var terminal: Boolean = false
        private set

    fun accept(frame: SseFrame, value: ChatEvent) {
        if (terminal) throw ChatProtocolException("event after terminal")
        if (value.version != 1) throw ChatProtocolException("unsupported version")
        if (value.sequence != nextSequence) throw ChatProtocolException("sequence gap")
        if (frame.event != value.type) throw ChatProtocolException("event type mismatch")
        if (frame.id != "${value.runId}:${value.sequence}") {
            throw ChatProtocolException("event id mismatch")
        }

        if (runId == null) {
            if (value.type != "run.started" || value.sequence != 1) {
                throw ChatProtocolException("missing run.started")
            }
            runId = value.runId
        } else if (runId != value.runId) {
            throw ChatProtocolException("run_id changed")
        }

        when (value.type) {
            "tool.started" -> {
                val toolId = value.toolId ?: throw ChatProtocolException("missing tool_id")
                if (!runningTools.add(toolId)) {
                    throw ChatProtocolException("duplicate tool_id")
                }
            }
            "tool.completed" -> {
                val toolId = value.toolId ?: throw ChatProtocolException("missing tool_id")
                if (!runningTools.remove(toolId)) {
                    throw ChatProtocolException("tool was not started")
                }
            }
            "run.completed", "run.failed" -> terminal = true
        }
        nextSequence++
    }
}
```

如果收到同版本的未知非终止事件，可以记录并忽略其展示，但仍必须推进已验证的序号。若产品要求严格锁定协议，可改为直接拒绝，并在升级客户端后放行。

不要自动携带 `Last-Event-ID` 重连。服务端不保存可重放事件；重试会创建新的 `run_id`、重新计费，并可能重新执行工具。

## 11. ViewModel 与界面状态

建议把网络事件转换为单向数据流：

```kotlin
data class ToolUiState(
    val id: String,
    val name: String,
    val summary: String,
    val status: String,
    val durationMs: Long? = null,
)

data class ChatUiState(
    val running: Boolean = false,
    val runId: String? = null,
    val delivery: String? = null,
    val answer: String = "",
    val tools: List<ToolUiState> = emptyList(),
    val completed: Boolean = false,
    val error: String? = null,
)
```

事件归约规则：

- `run.started`：清空旧的运行态内容，记录 `run_id`；
- `tool.started`：追加一张“执行中”工具卡片；
- `tool.completed`：按 `tool_id` 更新，不按列表位置猜测；
- `answer.delta`：使用 `answer += text.orEmpty()`，并记录该事件的 `delivery`；
- `run.completed`：只有此时才能把答案记入成功历史；
- `run.failed`：显示 `summary`，保留工具轨迹供用户理解失败位置；
- `streaming-final` 后收到 `run.failed`：显示“回答未完成”，不能保存已收文本；
- 无终止事件 EOF：显示“回答中断”，不能保存为成功答案。

在 ViewModel 中保存当前 Job，并阻止同一用户重复发送：

```kotlin
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.launch

private var chatJob: Job? = null

fun send(request: ChatRequest) {
    if (chatJob?.isActive == true) return
    chatJob = viewModelScope.launch {
        repository.events(token, deviceId, request)
            .catch { error -> reduceTransportFailure(error) }
            .collect { event -> reduce(event) }
    }
}

fun cancel() {
    chatJob?.cancel()
}
```

旋转屏幕时不要重新发起请求。让 ViewModel 持有收集任务，Compose 或 Fragment 只观察 `StateFlow`。用户主动取消、退出会话或 ViewModel 清理时应取消 Call；服务端检测断连后会取消该次 Agent 运行。

对于 `buffered-final`，界面应显示“正在检索”及工具卡片，收到答案后一次展示。对 `streaming-final`，可随分片追加正文；不要对 buffered-final 伪造逐字输出，也不要把工具摘要拼进答案正文。

## 12. 错误处理

### 12.1 SSE 开始前

服务端用 HTTP 状态码和 JSON 错误响应：

| 状态 | 含义与建议 |
| --- | --- |
| `401` | Token 无效、过期或设备 ID 不匹配；清理会话并重新登录 |
| `403` | 用户缺少 Browse 或 Chat 权限；不要自动重试 |
| `409` | 登录设备配额或其他状态冲突；引导用户解绑设备 |
| `422` | 请求字段、角色、长度或 JSON 无效；修正本地请求 |
| `429` | 同一用户已有问答；读取 `Retry-After` 后由用户重试 |
| `503` | Chat 端点或 HTTP 接入容量不可用；保留问题但不要忙循环 |
| `504` | 流建立前请求已超时；允许用户手动重试更小问题 |

校验错误的 `detail` 可能是字符串，也可能是数组。错误 DTO 应允许两种形态，或在 Repository 中保留截断后的原始错误正文。

### 12.2 SSE 开始后

HTTP 状态已经是 `200`，运行错误通过以下终止事件表达：

```json
{
  "version": 1,
  "run_id": "...",
  "sequence": 6,
  "type": "run.failed",
  "code": "agent_timeout",
  "summary": "问答未完成，请重试或缩小查询范围"
}
```

只展示安全 `summary` 与稳定 `code`，不要尝试从文本推断供应商内部错误。工具失败会在 `tool.completed.code` 中给出 `tool_invalid_command`、`tool_unsupported_command` 或 `tool_read_failed`；运行失败会给出 `agent_busy`、`model_unavailable`、`agent_timeout`、`model_protocol_error`、`agent_configuration_error` 或 `agent_failed`。服务端对瞬时模型故障（网络、429、5xx、流截断）已自动重试一次；`model_unavailable` 表示重试后仍失败，客户端可提示稍后再试。网络断开、解析失败、事件消费者失败，或 EOF 无终止事件统一归类为“未完成”，并与 `run.failed` 区分，以便排障。

一次用户最多有一个在途问答，服务端全局最多四个 Agent。自动重试必须克制：建议只对连接建立前的瞬时网络失败提供一次用户确认后的重试，不要自动重放已经收到工具事件的运行。

## 13. v1 迁移方案

旧接口继续可用：

```text
POST /api/v1/client/chat
```

| 能力 | v1 | v2 |
| --- | --- | --- |
| 非流式最终答案 | `stream=false` 返回 JSON | 不支持，始终 SSE |
| 流格式 | raw-text `data:` | 标准 SSE + JSON |
| 工具进度 | 不提供 | `tool.started/completed` |
| 成功终止 | `[DONE]` | `run.completed` |
| 失败终止 | 文本降级或 HTTP 错误 | `run.failed` |
| 答案交付 | 最终文本 | 工具轮为 `buffered-final`；预算收束轮可为 `streaming-final` |

推荐迁移步骤：

1. 保留旧 v1 Repository，不修改已有线上解析器；
2. 新增独立的 v2 DTO、SSE Parser 和状态归约器；
3. 用远程开关让测试用户进入 v2；
4. 对比最终答案展示、取消、401、403、429 和断网行为；
5. 确认稳定后默认使用 v2；
6. 回滚时只切回 v1，不复用 v2 JSON 解析器解释 raw-text 帧。

不要尝试让同一个解析器同时猜测 v1 与 v2。两者虽然都使用 `text/event-stream`，帧内语义完全不同。

## 14. 测试清单

### 14.1 Parser 单元测试

至少覆盖：

- 一个网络块包含多个 SSE 帧；
- 一个 JSON 事件被拆成多个网络块；
- LF 与 CRLF；
- 多个 `data:` 行合并；
- `: ping` 心跳忽略；
- JSON 文本中包含换行转义；
- sequence 跳号或重复；
- `run_id` 中途改变；
- `tool.completed` 没有对应开始事件；
- 工具 `failed` 后运行继续成功；
- `run.failed` 正常终止；
- EOF 无终止事件；
- 用户取消后 Call 被取消；
- 未知 JSON 字段不导致崩溃；
- `version != 1` 明确失败。

MockWebServer 可用 `setChunkedBody` 制造与 SSE 帧边界不同的网络分块。断流测试应在 `run.completed` 之前关闭响应，断言 UI 为“未完成”。

### 14.2 Repository 测试

验证请求包含：

- `Authorization: Bearer <token>`；
- 与登录相同的 `X-Device-ID`；
- `Accept: text/event-stream`；
- 正确的 JSON 字段名；
- 取消 Flow 后底层 Call 取消；
- 非 2xx 响应未进入 SSE Parser。

### 14.3 联调验收

依次验证：

1. 登录并保存稳定设备 ID；
2. 无 Chat 权限返回 `403`；
3. 授予 Browse 与 Chat 后可运行；
4. 界面按 `tool_id` 更新工具卡片；
5. `answer.delta` 不覆盖之前片段；
6. 只有 `run.completed` 才保存成功消息；
7. 飞行模式或主动取消后显示中断；
8. 连续点击发送不会创建第二个请求；
9. 应用旋转不重复发问；
10. 日志和崩溃报告不含 Token、密码、正文或工具摘要。

仓库内的 `tools/chatmock` 可用于不调用付费模型的端到端联调，完整启动步骤见 [`chat-events.md`](chat-events.md#本地端到端测试)。

## 15. 命令行对照请求

移动端联调前可用 curl 确认服务端协议：

```bash
BASE_URL='https://example.invalid'
TOKEN='登录返回的不透明Token'
DEVICE_ID='登录时使用的固定设备ID'
QUESTION='最近有哪些考试通知？'

curl --no-buffer --fail-with-body \
  -X POST "$BASE_URL/api/v2/client/chat" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Device-ID: $DEVICE_ID" \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  --data-binary "$(
    python3 -c 'import json,sys; print(json.dumps({
      "user_query": sys.argv[1],
      "history": [],
      "notice_ids": [],
      "stream": True
    }, ensure_ascii=False))' "$QUESTION"
  )"
```

预期至少收到 `run.started` 和一个终止事件。是否出现工具事件取决于模型是否选择检索；不能把“没有工具事件”直接判定为协议错误。成功运行通常为：

```text
run.started → tool.started → tool.completed → answer.delta → run.completed
```

## 16. 发布检查

上线前确认：

- Release 构建只允许 HTTPS；
- 没有网络日志记录敏感 Header 或正文；
- Token 与设备 ID 持久化策略稳定；
- v1 与 v2 Parser 相互隔离；
- 所有终止路径都停止加载动画；
- EOF 无终止事件不会保存错误答案；
- 工具失败不会提前结束 UI；
- `buffered-final` 不呈现伪流式动画，`streaming-final` 才增量渲染；
- 401、403、409、422、429、503、504 均有用户可理解提示；
- 服务端升级出现未知字段时客户端不会崩溃；
- 协议版本变化时客户端明确提示升级，而不是静默误解析。
