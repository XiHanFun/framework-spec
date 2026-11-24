# RFC-0003 调用上下文与传播

> 状态：已采纳（2026-10-29 修订：调用来源、端点身份；2026-11-01 修订：追踪标志与 Activity 父级、Baggage 在载体编解码处按已登记过滤；同日再修订：第 1 条写明下游写下的取消结果按超时归因；同日第三次修订：截止时间与调用方取消同时触发时报超时；2026-11-16 修订：根调用写出的 trace-flags 与采样位；同日再修订：`traceparent` 的文法、写出只带采样位、插件元数据与消息信封同一取法；同日第三次修订：取消与截止时间只经令牌通知，不强行中止处理器；2026-11-24 修订：截止时间文本的文法（规则 8），HTTP 载体的 `xihan-deadline` 与插件元数据共用；同日再修订：.NET 的两处载体改由原语 `DeadlineText` 读写；同日第三次修订：规则 8 的读法也是消息信封 `time` 的文法，.NET 的解析在原语 `TimestampText`） | 日期：2026-10-21 | 对应 ADR：0002、0043（调用来源）、0048（端点身份）、0065（追踪标志与 Activity 父级）、0088、0092（超时归因）、0233（写出的标志）、0235、0236、0276（截止时间的文本）、0278（.NET 的 `DeadlineText`）、0283（消息信封的 `time` 共用规则 8 的读法） | 参考实现：`InvocationContext`、`DeadlineMiddleware`、`InvocationOrigin`、`TraceParent`、`DeadlineText` | 一致性档：`core`、`host`

## 摘要

一次调用在管道内的上下文是 `InvocationContext`；跨进程传播的部分由 Baggage 键与载体映射定义。上下文不绑定 HTTP。

## 字段

| 字段 | 内容 | 跨进程传播 |
| --- | --- | --- |
| `CancellationToken` | 取消 | 由载体的连接取消或插件协议取消驱动 |
| `Deadline` | 绝对截止时间（UTC），基于 `TimeProvider` | `xihan-deadline`（文本的文法见规则 8） |
| `TraceId` / `SpanId` | W3C Trace Context：trace-id 与 parent-id | `traceparent` |
| `TraceFlags` | W3C trace-flags（采样位 `01`）；没有上游时为空，这次调用是链路的根 | `traceparent` |
| `Baggage` | 已登记的键（`spec/conventions/baggage-keys.md`） | W3C `baggage` |
| `Items` | 请求作用域对象袋 | 不传播 |
| `Services` | 作用域服务提供器 | 不传播 |

## 规则

1. 取消与截止时间都只经取消令牌通知，调用器与内置步骤不强行中止中间件或处理器（2026-11-16 写明，ADR-0236）：不取消处理器所在的任务、协程或线程，不丢弃它的 future。令牌触发之后处理器照常返回结果的，调用以那个结果结束；只有处理器抛出取消、或把取消写成结果时，才按下面的规则归因。以前 Python SDK 在处理器第一次让出时就取消它，与 .NET 不同。
   截止时间与取消并存：到达截止时间时取消令牌必须触发，错误码 `KERNEL_TIMEOUT_001`；截止时间到达之前的外部取消为 `KERNEL_CANCELLED_001`。截止令牌已触发即报超时，调用方同时取消也一样：下游抛出的取消与下游写成 `KERNEL_CANCELLED_001` 结果的取消都由截止时间中间件按这一条归因为 `KERNEL_TIMEOUT_001`（2026-11-01 起的行为变化，RFC-0004「行为变化：截止时间到了一律报超时」，ADR-0088、ADR-0092）。
2. 下游调用的截止时间不得晚于上游。
3. Baggage 只传已登记的键（`conventions/baggage-keys.md`）：HTTP、gRPC、MCP 的网络入站不收 `xihan.tenant` 与 `xihan.principal`，它们不能由调用方自报（2026-11-04 修订，ADR-0121）；每个载体的入站只收已登记的键，格式合法但未登记的 `xihan.` 键与其他键一律丢弃；出站（HTTP 的 `baggage` 头、插件元数据、消息扩展属性）只写已登记的键。进程内可以用 `WithBaggage` 带格式合法的其他 `xihan.<name>` 键，它们不出进程。剥离发生在载体的编解码处，没有单独的 Egress 阶段中间件。
4. 安全上下文（主体、租户）由 Authenticate 与 Tenant 阶段写入，之后不可变；Baggage 中只传 ID。
5. 载体映射见 `spec/conventions/baggage-keys.md`。
6. 调用来源（2026-10-29 增补，ADR-0043）：协议适配器与派发器建上下文时在 `Items` 写入一次 `InvocationOrigin`，之后只读、不跨进程传播；访问日志、异常日志、登录与会话记录据此取客户端信息，与协议无关。字段为协议、请求目标、远端地址（IP 文本，不含端口，IPv4 映射的 IPv6 还原为 IPv4）、User-Agent 与来源页，以及宿主匹配到的端点的路由模板与名字（ADR-0048，没有匹配到端点或协议没有端点时为空），没有的记空。内部发起的调用（缓存回源、存储维护、测试宿主）没有来源。

| 协议 | 请求目标 | 远端地址 / User-Agent / 来源页 |
| --- | --- | --- |
| `http` | 方法与路径（含 PathBase，不含查询串），如 `GET /api/orders/42`；另记路由模板（如 `/api/orders/{id}`）与端点名 | 连接对端地址（宿主在接入点之前启用转发头处理时为转发头改写后的客户端地址）、`User-Agent`、`Referer` |
| `grpc` | 方法全名，如 `/orders.Orders/Get` | 承载它的 HTTP/2 请求 |
| `mcp` | `tools/call` 的 tool 名 | HTTP 传输沿用外层请求；stdio 传输没有 |
| `cli` | 被调用的操作 ID | 没有 |
| `messaging` | 主题 | 没有 |
| `job` | 作业名 | 没有 |
| `workflow` | 活动类型 | 没有 |

7. 追踪（2026-11-01 增补，ADR-0065）：
   - 来源：载体带合法的 `traceparent` 时取它的 trace-id、parent-id 与 trace-flags，上游「不采样」（`-00`）原样保留；没有时并入宿主当前的 W3C Activity（ASP.NET Core 的请求 Span、外层调用的 Span），取它的 trace-id 与 span-id，它有采样决定（经具名 ActivitySource 创建，或有父级）时连同标志；都没有时内核生成 trace-id 与 parent-id，标志为空。只给 trace-id 不给标志的创建方按上游已采样处理。
   - Observe 阶段的 `xihan.invoke` Span：当前 Activity 与上下文同一 trace 且有采样决定时是它的子 Span；否则上下文有标志时以 trace-id、parent-id 与标志为远端父级，采样随上游；标志为空时它是链路的根 Span——trace-id 用上下文的（与 Problem Details、审计、日志里的一致），没有父级，采样交给采样器的根规则。
   - 文法（2026-11-16 写明，ADR-0235）：各载体与语言 SDK 只认 `^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`，trace-id 或 parent-id 全零也不合法（W3C）。其他版本（含 `ff`）、大写十六进制、多余的段都不合法。不合法的 `traceparent` 当作没有给出，不报错、不把坏值带进链路。.NET 由原语 `TraceParent.TryParse` 统一判断，HTTP、消息派发与插件回调三处载体共用。
   - 是否采样只看标志的最低位（W3C 的 `sampled`），其余位不进上下文。
   - 写出（2026-11-16 写明，ADR-0233、ADR-0235）：上下文里的标志为空只表示「没有上游」，写出的 `traceparent` 总带标志，版本是 `00`，标志只写采样位，即 `00` 或 `01`。
     - 写的是这次调用的采样决定：当前 Activity 是这次调用的 Span（Observe 阶段的 `xihan.invoke` 或它的子 Span）时取它的 span-id 与标志，否则取上下文的 parent-id 与标志。
     - 上下文的标志为空、又没有采样器的实现（不带 Observe 阶段的语言 SDK）写 `01`，即按已采样传下去，由下游的采样器决定。
     - 上游给的 `00` 原样传下去。消息信封与插件调用的元数据用同一个取法；以前插件元数据把标志写死为 `01`，上游的「不采样」传不到插件。
   - 向量 `conformance/vectors/primitives/trace-context.json`：合法与不合法的 `traceparent`、解析结果与写出的文本。
8. 截止时间的文本（2026-11-24 写明，ADR-0276）：截止时间出进程时写成同一种文本，HTTP 载体的 `xihan-deadline` 头（gRPC 的同名元数据、MCP 的 Streamable HTTP 用的也是它）与插件协议的 `InvocationMetadata.deadline` 共用一条文法。以前只有插件协议一侧写了文法（`plugin/invocation.md` §2，ADR-0237），HTTP 载体只写着「RFC 3339，UTC」：.NET 的 HTTP 入口用通用的日期解析，空格代替 `T`、没有偏移、没有秒、只有日期、偏移不带冒号、首尾带空白、HTTP 日期（RFC 1123）与 `11/18/2026` 这样的写法都读得出，没有偏移的当作 UTC，只有时间的补上当天的日期，比 100 纳秒更细的小数四舍五入而不是截断，`9999-12-31T23:59:59.999999999Z` 因此读不出。
   - 读法：`^[0-9]{4}-[0-9]{2}-[0-9]{2}[Tt][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?([Zz]|[+-][0-9]{2}:[0-9]{2})\z`，按整串匹配（RFC-0001「文法的匹配」）；年 `0001`–`9999`、月日按公历（含闰年）、时 `00`–`23`、分 `00`–`59`、秒 `00`–`59`（不认闰秒 `60`）、偏移的时 `00`–`23` 与分 `00`–`59`；换算成 UTC 之后（不计小数）在 `0001-01-01T00:00:00` 到 `9999-12-31T23:59:59` 之间；比实现的时钟精度更细的小数截断，不进位。它是 RFC 3339 `date-time` 的子集：不认闰秒，小数一到九位。
   - 读不出（不合文法或越界，含空串）就当作没有给出截止时间：不报错，也不换别的格式再试。.NET 由原语 `DeadlineText.TryParse` 统一判断、`DeadlineText.Format` 写出，HTTP 入口与插件协议两处载体共用（ADR-0278）。这条读法同时是消息信封 `time` 的文法（RFC-0009「信封」第 3 条，2026-11-24，ADR-0283）：那里读不出是信封不合法，不是当作没有；.NET 的解析只有一份，在原语 `TimestampText.TryParse`，`DeadlineText.TryParse` 转给它。
   - 写法：框架写出时一律 UTC、七位小数、`+00:00` 偏移（.NET 的 `O` 格式，如 `2026-11-18T03:25:00.1234567+00:00`），小数不足七位补零，比 100 纳秒更细的部分截断。调用方自己写 `xihan-deadline` 头时，任何合文法的写法都读得出。
   - 各载体怎样取到这段文本、与载体自己的截止时间怎样并存：HTTP 见 `protocol/http.md`「上下文映射」（与 `grpc-timeout` 的先后见 `protocol/grpc.md`），插件协议见 `plugin/invocation.md` §2（与 gRPC 调用自身的截止时间取较早者）。命令行与 MCP 的 stdio 传输不传文本，用相对的 `--timeout`（`protocol/commandline.md`、`protocol/mcp.md`）。
   - 向量：`conformance/vectors/plugin/deadlines.json`（合法与不合法的文本、读出的时刻、写出的文本，两个载体共用）、`conformance/vectors/context/http-deadline.json`（HTTP 请求头到截止时间，含 `grpc-timeout`）。

## .NET 形态

见 RFC-0001 `InvocationContext`。内核在 Ingress 阶段从载体构造上下文，在 Egress 阶段把需要传播的字段写回载体。

上下文只经参数传递给操作处理器与中间件；调用链深处不传上下文的服务（领域服务、仓储装饰器、"当前用户 / 当前租户"包装）经作用域服务 `IInvocationAccessor.Current` 取当前调用的上下文（2026-10-24 增补）：内核在调用进入管道前设置、离开时恢复上一层，同一服务作用域内的嵌套调用各看到自己的上下文；不在调用内（作用域刚创建、后台任务、调用已结束）为无值。访问器不是传播载体，不进 Baggage。

## 一致性向量

`spec/conformance/vectors/context/http-deadline.json`（2026-11-24 增补，ADR-0276）：HTTP 载体从 `xihan-deadline` 与 `grpc-timeout` 读出的截止时间；截止时间文本的文法在 `plugin/deadlines.json`（规则 8）。`context/` 下其余的向量规划中：各载体其余字段的编码与解码、截止时间与取消的时序、未登记键剥离，目前由各实现移植的测试覆盖；Baggage 键与值的约束在 `primitives/baggage-keys.json`，`traceparent` 在 `primitives/trace-context.json`。
