# MCP 载体

> 规范性。参考实现：`XiHan.Framework.Protocols.Mcp`。Model Context Protocol 版本 `2025-06-18`，消息为 JSON-RPC 2.0，传输为 Streamable HTTP 与 stdio。

## 上下文映射（RFC-0003）

Streamable HTTP 传输复用 HTTP 载体的头部映射（`protocol/http.md`）：trace、Baggage、截止时间、`Accept-Language`、`Authorization` 都取自承载 JSON-RPC 消息的 `POST` 请求头。每个 `tools/call` 是一次独立调用：以同一请求头构造新的调用上下文（自己的服务作用域、工作单元与管道），不与承载请求的外层调用共享。

stdio 传输（命令行宿主的 `mcp` 命令：客户端启动服务端进程，经 stdin / stdout 交换消息）没有请求头。按 MCP 规范，stdio 服务端从运行环境取凭据，不走 HTTP 授权流程；上下文取自启动这个进程的命令行选项与环境变量，与命令行载体（`protocol/commandline.md`）同一套映射，整个会话不变：

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| Credential | `--authorization '<scheme> <value>'` 或环境变量 `XIHAN_AUTHORIZATION` | 选项优先；原样进入每个 `tools/call` 的上下文，由该次调用的 Authenticate 阶段解释 |
| Baggage `xihan.tenant` | `--tenant <id>` | 由 Tenant 阶段按 RFC-0007 解析与核对。这是本机操作者给的，HTTP 传输的 `baggage` 头不收这个键（ADR-0121） |
| Baggage `xihan.correlation` | `--correlation <id>` | |
| Baggage `xihan.locale` | `--locale <locale>` | 规范化后写入；`tools/list` 的 `title` / `description` 与失败文案据此翻译（经 RFC-0010 协商），没有时为默认区域 |
| Deadline | `--timeout <seconds>` | 每个 `tools/call` 的时限，从这次调用开始计 |
| Trace | — | 每条消息生成新的 trace-id |

- 每条消息一个服务作用域；每个 `tools/call` 以上述值新建调用上下文，经完整内核管道，调用来源记 `mcp` 与 tool 名（没有远端地址与 User-Agent）。
- stdio 没有外层认证：凭据无效时每个需要认证的 `tools/call` 都是 `isError` 结果（`SECURITY_DENIED_003`），会话继续；不需要认证的 tool 照常执行。
- 选项不合法（凭据不是 `<scheme> <value>`、时限不是正数）时命令不进入会话：stderr 写一行用法错误（`HOSTING_VALIDATION_001`），退出码 64，不读 stdin。

## 操作映射（RFC-0011）

- tool 名按 `conventions/operation-ids.md` 推导：`{resource}_{verb}`；显式覆盖写在 `XiHan:Protocols:Mcp:Tools:<操作 ID>`。文法 `^[A-Za-z0-9_-]{1,64}$`；不合法或两个操作同名即启动失败（`PROTOCOLS_VALIDATION_004` / `PROTOCOLS_CONFLICT_001`）。
- 发布范围：`Expose` 含 `Mcp` 且非 `Stability.Internal` 的操作。
- `tools/list`（不分页）：
  - `inputSchema` = 操作的请求 Schema（`OperationSchema.Request`，`type` 须为 `object`，否则 `{"type":"object"}`）；
  - `outputSchema` = 响应 Schema，仅当其 `type` 为 `object`；
  - `title` / `description` = 消息目录里 `<操作 ID>.title` / `<操作 ID>.description`（ID 里的 `-` 换成 `_`）按调用方区域的文案，没有就不给；
  - `annotations.readOnlyHint` / `destructiveHint` / `idempotentHint` 按契约：只读契约（`get` / `list`，或声明了只读的非标准动词，RFC-0011）只读且幂等，`update` 幂等，`delete` 破坏性且幂等，`create` 既不破坏也不幂等，其他动词按 MCP 默认（可能破坏、不幂等）。
- `tools/call`：`arguments`（缺省为 `{}`，必须是对象）就是 XGI 载荷。成功：`content[0]` 为 `text`，文本是响应 JSON；响应为对象时同时给 `structuredContent`。操作失败（校验、未授权、冲突……）：`isError: true`，`content[0].text` = `<错误码>: <文案>`，文案按调用方区域翻译；不是协议错误。
- JSON-RPC 错误：`error.data.code` 携带框架错误码。

| 情形 | `error.code` | `error.data.code` | HTTP |
| --- | --- | --- | --- |
| 请求体不是 JSON | -32700 | `PROTOCOLS_VALIDATION_001` | 400 |
| 不是 JSON-RPC 2.0 请求（非对象、`jsonrpc` 不是 `2.0`、`method` 缺失、`id` 为 `null`、`params` 非对象） | -32600 | `PROTOCOLS_VALIDATION_001` | 400 |
| `MCP-Protocol-Version` 头不受支持 | -32600 | `PROTOCOLS_VALIDATION_002` | 400 |
| 方法不受支持 | -32601 | `PROTOCOLS_NOTFOUND_001` | 200 |
| `tools/call` 缺 `name` | -32602 | `PROTOCOLS_VALIDATION_001` | 200 |
| tool 未登记 | -32602 | `PROTOCOLS_NOTFOUND_002` | 200 |
| `arguments` 不是对象 | -32602 | `PROTOCOLS_VALIDATION_003` | 200 |

## 生命周期与传输

- `initialize`：答复 `protocolVersion`（固定为本服务端版本；客户端请求其他版本时也如此，由客户端决定是否继续）、`capabilities.tools.listChanged = false`、`serverInfo`（`name` 默认应用名、`version` 默认框架版本，可由 `XiHan:Protocols:Mcp:ServerName` / `ServerVersion` 覆盖）、可选 `instructions`（`XiHan:Protocols:Mcp:Instructions`）。
- `ping` → `{}`。
- 通知（没有 `id` 的消息，如 `notifications/initialized`）与客户端发来的响应（有 `result` / `error` 而无 `method`）不答复：HTTP 202，无正文。
- `POST <pattern>`（默认 `/mcp`）一条消息一个答复，`Content-Type: application/json; charset=utf-8`；不开 SSE 流，不建会话（不发 `Mcp-Session-Id`）；`GET` / `DELETE` 为 405。
- Streamable HTTP 的请求凭据由宿主在外层认证：凭据无效时整个请求以 Problem Details 拒绝（401 `SECURITY_DENIED_003`，带 `WWW-Authenticate` 质询，见 `protocol/http.md`），不进入 JSON-RPC；应用未就绪时 503。按契约的授权在每个 `tools/call` 的管道里判定，未授权是 `isError` 结果。
- stdio：stdin 每行一条消息（空行跳过），答复是一行紧凑 JSON 写到 stdout，通知与客户端发来的响应不答复；stdin 关闭即结束会话，命令以 0 退出。日志只写 stderr。消息顺序处理，一条处理完再读下一条。`mcp` 命令把 stdin 的字节按 UTF-8 解码（开头的 UTF-8 BOM 不算内容），与控制台代码页无关——Windows 上重定向的标准输入若按控制台输入代码页（如 936、437）解码，非 ASCII 的参数会成乱码；答复只含 ASCII 字符（JSON 里的非 ASCII 字符一律转义为 `\uXXXX`），按任何兼容 ASCII 的控制台输出代码页写出都是同样的字节，即合法的 UTF-8（ADR-0083）。
