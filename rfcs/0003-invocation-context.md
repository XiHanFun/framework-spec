# RFC-0003 调用上下文与传播

> 状态：已采纳（2026-10-29 修订：调用来源） | 日期：2026-10-21 | 对应 ADR：0002、0043（调用来源） | 参考实现：`InvocationContext`、`DeadlineMiddleware`、`InvocationOrigin`

## 摘要

一次调用在管道内的上下文是 `InvocationContext`；跨进程传播的部分由 Baggage 键与载体映射定义。上下文不绑定 HTTP。

## 字段

| 字段 | 内容 | 跨进程传播 |
| --- | --- | --- |
| `CancellationToken` | 取消 | 由载体的连接取消或插件协议取消驱动 |
| `Deadline` | 绝对截止时间（UTC），基于 `TimeProvider` | `xihan-deadline`（RFC 3339） |
| `TraceId` / `SpanId` | W3C Trace Context | `traceparent` |
| `Baggage` | 已登记的键（`spec/conventions/baggage-keys.md`） | W3C `baggage` |
| `Items` | 请求作用域对象袋 | 不传播 |
| `Services` | 作用域服务提供器 | 不传播 |

## 规则

1. 截止时间与取消并存：到达截止时间时取消令牌必须触发，错误码 `KERNEL_TIMEOUT_001`；外部取消为 `KERNEL_CANCELLED_001`。
2. 下游调用的截止时间不得晚于上游。
3. Baggage 只传已登记的键；未登记键在 Egress 阶段剥离。
4. 安全上下文（主体、租户）由 Authenticate 与 Tenant 阶段写入，之后不可变；Baggage 中只传 ID。
5. 载体映射见 `spec/conventions/baggage-keys.md`。
6. 调用来源（2026-10-29 增补，ADR-0043）：协议适配器与派发器建上下文时在 `Items` 写入一次 `InvocationOrigin`，之后只读、不跨进程传播；访问日志、异常日志、登录与会话记录据此取客户端信息，与协议无关。字段为协议、请求目标、远端地址（IP 文本，不含端口，IPv4 映射的 IPv6 还原为 IPv4）、User-Agent 与来源页，没有的记空。内部发起的调用（缓存回源、存储维护、测试宿主）没有来源。

| 协议 | 请求目标 | 远端地址 / User-Agent / 来源页 |
| --- | --- | --- |
| `http` | 方法与路径（含 PathBase，不含查询串），如 `GET /api/orders` | 连接对端地址（宿主在接入点之前启用转发头处理时为转发头改写后的客户端地址）、`User-Agent`、`Referer` |
| `grpc` | 方法全名，如 `/orders.Orders/Get` | 承载它的 HTTP/2 请求 |
| `mcp` | `tools/call` 的 tool 名 | HTTP 传输沿用外层请求；stdio 传输没有 |
| `cli` | 被调用的操作 ID | 没有 |
| `messaging` | 主题 | 没有 |
| `job` | 作业名 | 没有 |
| `workflow` | 活动类型 | 没有 |

## .NET 形态

见 RFC-0001 `InvocationContext`。内核在 Ingress 阶段从载体构造上下文，在 Egress 阶段把需要传播的字段写回载体。

上下文只经参数传递给操作处理器与中间件；调用链深处不传上下文的服务（领域服务、仓储装饰器、从旧版本迁来的"当前用户 / 当前租户"包装）经作用域服务 `IInvocationAccessor.Current` 取当前调用的上下文（2026-10-24 增补）：内核在调用进入管道前设置、离开时恢复上一层，同一服务作用域内的嵌套调用各看到自己的上下文；不在调用内（作用域刚创建、后台任务、调用已结束）为无值。访问器不是传播载体，不进 Baggage。

## 一致性向量

`spec/conformance/vectors/context/*.json`：各载体的编码与解码、截止时间与取消的时序、未登记键剥离。
