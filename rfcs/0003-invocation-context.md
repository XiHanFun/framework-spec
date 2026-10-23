# RFC-0003 调用上下文与传播

> 状态：已采纳 | 日期：2026-10-21 | 对应 ADR：0002 | 参考实现：`InvocationContext`、`DeadlineMiddleware`

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

## .NET 形态

见 RFC-0001 `InvocationContext`。内核在 Ingress 阶段从载体构造上下文，在 Egress 阶段把需要传播的字段写回载体。

上下文只经参数传递给操作处理器与中间件；调用链深处不传上下文的服务（领域服务、仓储装饰器、从旧版本迁来的"当前用户 / 当前租户"包装）经作用域服务 `IInvocationAccessor.Current` 取当前调用的上下文（2026-10-24 增补）：内核在调用进入管道前设置、离开时恢复上一层，同一服务作用域内的嵌套调用各看到自己的上下文；不在调用内（作用域刚创建、后台任务、调用已结束）为无值。访问器不是传播载体，不进 Baggage。

## 一致性向量

`spec/conformance/vectors/context/*.json`：各载体的编码与解码、截止时间与取消的时序、未登记键剥离。
