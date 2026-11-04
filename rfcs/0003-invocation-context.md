# RFC-0003 调用上下文与传播

> 状态：已采纳（2026-10-29 修订：调用来源、端点身份；2026-11-01 修订：追踪标志与 Activity 父级、Baggage 在载体编解码处按已登记过滤；同日再修订：第 1 条写明下游写下的取消结果按超时归因；同日第三次修订：截止时间与调用方取消同时触发时报超时） | 日期：2026-10-21 | 对应 ADR：0002、0043（调用来源）、0048（端点身份）、0065（追踪标志与 Activity 父级）、0088、0092（超时归因） | 参考实现：`InvocationContext`、`DeadlineMiddleware`、`InvocationOrigin`

## 摘要

一次调用在管道内的上下文是 `InvocationContext`；跨进程传播的部分由 Baggage 键与载体映射定义。上下文不绑定 HTTP。

## 字段

| 字段 | 内容 | 跨进程传播 |
| --- | --- | --- |
| `CancellationToken` | 取消 | 由载体的连接取消或插件协议取消驱动 |
| `Deadline` | 绝对截止时间（UTC），基于 `TimeProvider` | `xihan-deadline`（RFC 3339） |
| `TraceId` / `SpanId` | W3C Trace Context：trace-id 与 parent-id | `traceparent` |
| `TraceFlags` | W3C trace-flags（采样位 `01`）；没有上游时为空，这次调用是链路的根 | `traceparent` |
| `Baggage` | 已登记的键（`spec/conventions/baggage-keys.md`） | W3C `baggage` |
| `Items` | 请求作用域对象袋 | 不传播 |
| `Services` | 作用域服务提供器 | 不传播 |

## 规则

1. 截止时间与取消并存：到达截止时间时取消令牌必须触发，错误码 `KERNEL_TIMEOUT_001`；截止时间到达之前的外部取消为 `KERNEL_CANCELLED_001`。截止令牌已触发即报超时，调用方同时取消也一样：下游抛出的取消与下游写成 `KERNEL_CANCELLED_001` 结果的取消都由截止时间中间件按这一条归因为 `KERNEL_TIMEOUT_001`（2026-11-01 起的行为变化，RFC-0004「行为变化：截止时间到了一律报超时」，ADR-0088、ADR-0092）。
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

## .NET 形态

见 RFC-0001 `InvocationContext`。内核在 Ingress 阶段从载体构造上下文，在 Egress 阶段把需要传播的字段写回载体。

上下文只经参数传递给操作处理器与中间件；调用链深处不传上下文的服务（领域服务、仓储装饰器、从旧版本迁来的"当前用户 / 当前租户"包装）经作用域服务 `IInvocationAccessor.Current` 取当前调用的上下文（2026-10-24 增补）：内核在调用进入管道前设置、离开时恢复上一层，同一服务作用域内的嵌套调用各看到自己的上下文；不在调用内（作用域刚创建、后台任务、调用已结束）为无值。访问器不是传播载体，不进 Baggage。

## 一致性向量

`spec/conformance/vectors/context/*.json`：各载体的编码与解码、截止时间与取消的时序、未登记键剥离。
