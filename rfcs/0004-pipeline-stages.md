# RFC-0004 管道阶段与排序

> 状态：已采纳（2026-10-28 修订：跨包依赖必须声明约束） | 日期：2026-10-21 | 对应 ADR：0002 | 参考实现：`XiHan.Framework.Kernel.Pipeline.PipelineBuilder`

## 摘要

每次调用经过一条固定阶段的管道。特性按阶段贡献中间件，不需要知道其他中间件；排序确定、可打印、可测试。

## 阶段

顺序固定：

```text
ingress → observe → resilience → authenticate → tenant → authorize → validate → idempotency → transaction → handle → egress
```

| 阶段 | 职责 |
| --- | --- |
| `ingress` | 协议消息归一化、大小限制、构造上下文 |
| `observe` | Trace、日志、指标 |
| `resilience` | 超时、限流、并发上限、熔断：内核内置 `DeadlineMiddleware`（截止时间，`KERNEL_TIMEOUT_001`）与 `ResilienceMiddleware`（按操作 ID 的熔断 / 固定窗口限流 / 并发上限，`KERNEL_UNAVAILABLE_002` – `004`，见「Resilience 阶段」） |
| `authenticate` | 主体解析 |
| `tenant` | 租户解析与隔离上下文 |
| `authorize` | 策略判定 |
| `validate` | 契约校验：内核内置 `ValidationMiddleware` 按操作的请求 JSON Schema 校验载荷（`KERNEL_VALIDATION_003`），解码后对实现 `IValidatableObject` 的请求执行自校验（`KERNEL_VALIDATION_004`），不通过不进 Handle（RFC-0011） |
| `idempotency` | 幂等键去重 |
| `transaction` | 工作单元 |
| `handle` | 调用操作 |
| `egress` | 结果与错误翻译、响应信封、写回传播字段 |

## Resilience 阶段

内核内置两个中间件，都归属内核清单、放在阶段开头，`DeadlineMiddleware` 在前：

- `DeadlineMiddleware`：按调用的截止时间取消下游，超时 `KERNEL_TIMEOUT_001`、外部取消 `KERNEL_CANCELLED_001`。
- `ResilienceMiddleware`：只作用于带操作描述的调用（宿主自定义端点、消息派发不受限），按操作 ID 各持一份限制实例，判定顺序固定为熔断 → 限流 → 并发；任一拒绝都以 UNAVAILABLE 种类的内核错误码结束调用、不进后续阶段，`details` 带 `id`，限流与熔断另带 `retryAfter`（秒，向上取整、至少 1）。没有任何限制的操作直接透传。

| 限制 | 语义 | 拒绝码 |
| --- | --- | --- |
| 熔断 | 连续失败达到 `FailureThreshold` 即打开 `BreakDuration`；到期半开，只放一次试探，成功关闭、失败重新打开；只有 KIND 为 `UNAVAILABLE` / `TIMEOUT` / `UNEXPECTED` 的结果与未处理异常算失败，校验、不存在、拒绝等调用方错误不算；打开期间仍在途的调用结束时不改变状态 | `KERNEL_UNAVAILABLE_004` |
| 限流 | 固定窗口：`Window` 内最多 `PermitLimit` 次，超出的最多 `QueueLimit` 个排队到下一窗口，其余拒绝 | `KERNEL_UNAVAILABLE_002` |
| 并发 | 同时执行最多 `ConcurrencyLimit` 次，超出的最多 `ConcurrencyQueueLimit` 个排队，其余拒绝 | `KERNEL_UNAVAILABLE_003` |

排队等待受调用的取消令牌约束：截止时间到了由 `DeadlineMiddleware` 按 `KERNEL_TIMEOUT_001` 结束，不会无限等。

配置节 `XiHan:Kernel:Resilience`：节本身的键是默认策略（`PermitLimit`、`WindowSeconds`、`QueueLimit`、`ConcurrencyLimit`、`ConcurrencyQueueLimit`、`FailureThreshold`、`BreakSeconds`），`Operations:<操作 ID>` 下同名键覆盖该操作，缺省键沿用默认策略；非法值在构建应用时拒绝。

```json
{
  "XiHan": {
    "Kernel": {
      "Resilience": {
        "PermitLimit": 1000,
        "WindowSeconds": 1,
        "ConcurrencyLimit": 64,
        "FailureThreshold": 5,
        "BreakSeconds": 30,
        "Operations": {
          "reports.export": { "ConcurrencyLimit": 2, "PermitLimit": 10, "WindowSeconds": 60 }
        }
      }
    }
  }
}
```

`XiHan:Kernel:UseBuiltInMiddleware=false` 时两个中间件都不登记，配置节不读取。按调用方（主体、租户）分桶与自适应并发不在内核范围，应用在本阶段加自己的中间件。

## 排序规则

1. 先按阶段。
2. 阶段内按 `before` / `after` 约束做拓扑排序；约束成环报 `KERNEL_PIPELINE_001`。
3. 仍未定序的按包 ID 字典序。
4. 显式 `Use<T>()` 不带阶段时归入 `transaction` 阶段末尾。
6. 阶段内先按 `Placement`（Beginning 在前），再按归属（清单 ID 字典序），再按注册顺序，最后应用 `before` / `after` 约束做稳定拓扑排序。
5. 未知阶段名报 `KERNEL_PIPELINE_002`。
8. 跨包依赖必须显式声明：一个包的中间件要用到另一个包中间件的产出（认证后的主体、解析后的租户、开启的工作单元）时，必须以 `before` / `after` 声明次序，不能依赖第 6 条的基准次序——归属字典序只保证结果可复现（`xihan.basicapp.identity` 会排在 `xihan.security.jwt` 之前），不表达依赖；内核不按清单的 `requires` 推导中间件次序。
7. 同一中间件类型默认只登记一次；需要同类型多个实例时（如插件贡献的多个中间件）按名字登记，名字在类型内唯一，打印时以 `类型名[名字]` 标出；按类型声明的 `before` / `after` 在该类型有多个实例时不可用（`KERNEL_PIPELINE_003`）。

## 打印格式

`xihan pipeline dump` 与启动日志输出同一格式，一行一个中间件：

```text
ingress      XiHan.Framework.Kernel.Pipeline.RequestNormalizationMiddleware
observe      XiHan.Framework.Kernel.Pipeline.ObserveMiddleware
...
handle       XiHan.Framework.Kernel.Pipeline.OperationDispatchMiddleware
egress       XiHan.Framework.Protocols.Http.ProblemDetailsMiddleware
```

## .NET 形态

```csharp
public interface IPipelineMiddleware
{
    ValueTask InvokeAsync(InvocationContext context, PipelineStep nextStep);
}

public delegate ValueTask PipelineStep(InvocationContext context);

// PipelineStage 枚举定义在 XiHan.Framework.Primitives（RFC-0001），阶段名与顺序为规范性内容
public enum Placement { Beginning, End }

public sealed class PipelineBuilder
{
    public PipelineBuilder Use<TMiddleware>() where TMiddleware : IPipelineMiddleware;
    public PipelineBuilder UseAt<TMiddleware>(PipelineStage stage, Placement placement = Placement.End, string owner = "app") where TMiddleware : IPipelineMiddleware;
    public PipelineBuilder UseInstance<TMiddleware>(TMiddleware instance, PipelineStage stage, Placement placement = Placement.End, string owner = "app") where TMiddleware : IPipelineMiddleware;
    public PipelineBuilder Before<TMiddleware, TOther>() where TMiddleware : IPipelineMiddleware where TOther : IPipelineMiddleware;
    public PipelineBuilder After<TMiddleware, TOther>() where TMiddleware : IPipelineMiddleware where TOther : IPipelineMiddleware;
    public PipelineStep Build(PipelineStep terminal);          // 中间件在每次调用时从 InvocationContext.Services 解析
    public IReadOnlyList<MiddlewareDescriptor> Describe();
    public string Dump();
}
```

管道的第二形态（阶段内并行分支）不在本 RFC 范围。

## 一致性向量

`spec/conformance/vectors/pipeline/*.json`：给定贡献集合的期望顺序、成环用例、未知阶段用例。
