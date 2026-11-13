# RFC-0001 原语层

> 状态：已采纳（2026-10-28 修订：消息键首段放开下划线；2026-11-01 修订：机密遮蔽 `Redaction`） | 日期：2026-10-21 | 对应 ADR：0002、0038、0086 | 参考实现：`framework/src/XiHan.Framework.Primitives`

## 摘要

定义 `XiHan.Framework.Primitives`（Ring 0）的全部公开类型及其跨语言语义与线上编码。原语层零行为、零依赖、只增不改。

## 动机

十年稳定要求只冻结极小的一层。原语层是所有包、所有语言 SDK、所有协议适配器共享的词汇；它一旦漂移，上面的一切都要跟着变。

## 公开面（.NET 形态）

命名空间 `XiHan.Framework.Primitives`，清单模型在子命名空间 `XiHan.Framework.Primitives.Manifests`。完整签名以 `PublicAPI.Unshipped.txt` 为准，5.0.0 发布时转入 `PublicAPI.Shipped.txt` 并冻结。

### Result

```csharp
public readonly record struct Result
{
    public bool IsSuccess { get; }
    public bool IsFailure { get; }
    public Error? Error { get; }                        // 失败时非 null；default 值视为 PRIMITIVES_VALIDATION_002
    public static Result Success();
    public static Result Failure(Error error);
    public static Result<T> Success<T>(T value);        // 值不能为 null
    public static Result<T> Failure<T>(Error error);
    public static implicit operator Result(Error error);
    public TOut Match<TOut>(Func<TOut> onSuccess, Func<Error, TOut> onFailure);
    public Result Bind(Func<Result> next);
    public void ThrowIfFailure();                       // 抛 FrameworkException(Error.Code)
}

public readonly record struct Result<T>
{
    public bool IsSuccess { get; }
    public bool IsFailure { get; }
    public Error? Error { get; }
    public static implicit operator Result<T>(T value);
    public static implicit operator Result<T>(Error error);
    public bool TryGetValue([MaybeNullWhen(false)] out T value);
    public T ValueOrThrow();
    public T ValueOr(T fallback);
    public TOut Match<TOut>(Func<T, TOut> onSuccess, Func<Error, TOut> onFailure);
    public Result<TOut> Map<TOut>(Func<T, TOut> map);
    public Result<TOut> Bind<TOut>(Func<T, Result<TOut>> bind);
    public Result ToResult();
}
```

工厂方法放在非泛型 `Result` 上，不在泛型类型上声明静态成员。成功值不允许为 `null`："成功但没有值"用 `Result` 或 `Result<Maybe<T>>`。

线上编码（JSON）：

```json
{ "ok": true, "value": … }
{ "ok": false, "error": { "code": "PERSISTENCE_CONFLICT_001", "message": "…", "messageKey": "persistence.entity.stale_stamp", "details": { … }, "inner": { … } } }
```

### Error 与 ErrorCode

```csharp
public sealed record Error
{
    // 可选的细节与内部错误用逐级重载，不用默认值（「可选参数」）
    public Error(string code, string messageKey);
    public Error(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);
    public Error(string code, string messageKey, IReadOnlyDictionary<string, object?>? details, Error? inner);
    public string Code { get; }
    public string? MessageKey { get; }                              // 消息键；Literal 创建的为 null
    public string? Text { get; }                                    // 已解析的文案；未解析为 null
    public string Message { get; }                                  // Text，否则按进程默认区域解析，再否则"键 (参数)"
    public IReadOnlyDictionary<string, object?>? Details { get; }   // 模板参数，也是结构化细节；按引用比较
    public Error? Inner { get; }
    public string Kind { get; }                                     // 错误码的 KIND 段
    public static Error Literal(string code, string text, IReadOnlyDictionary<string, object?>? details, Error? inner);   // 线上还原、异常转换；另有 (code, text) 与 (code, text, details)
    public static Error Validation(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);       // 另有 (code, messageKey)；NotFound、Conflict、Denied、Unavailable 同
    public static Error NotFound(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);
    public static Error Conflict(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);
    public static Error Denied(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);
    public static Error Unavailable(string code, string messageKey, IReadOnlyDictionary<string, object?>? details);
    public static Error Unexpected(string code, string messageKey, IReadOnlyDictionary<string, object?>? details, Error? inner);   // 另有 (code, messageKey) 与 (code, messageKey, details)
    public Error WithCode(string code);                             // 换码，保留键、文案、参数
    public Error WithText(string text);                             // 填入已解析文案
}

// 消息原语（RFC-0010）：键文法与模板格式化、参数字典、key.L()、环境作用域
public static class MessageTemplate { IsValidKey; ValidateKey; DomainOf; Describe; Format; TryFormat; PlaceholdersOf; }
public static class MessageArguments { Of(params (string, object?)[]); Positional(params object?[]); }
public static class MessageKeyExtensions { L(this string key); L(key, params (string, object?)[]); L(key, params object?[]); L(key, IReadOnlyDictionary); }
public interface IMessageResolver { bool TryResolve(string key, string? locale, IReadOnlyDictionary<string, object?>? arguments, out string? text); }
public sealed class MessageScope : IDisposable { static Current; static Default; static Use(resolver, locale); static SetDefault(resolver, locale); Resolve(key, arguments); }

public static class ErrorCode
{
    public const string InvalidArgument = "PRIMITIVES_VALIDATION_001";
    public const string UninitializedResult = "PRIMITIVES_VALIDATION_002";
    public static bool IsValid(string? code);
    public static string Validate(string code, [CallerArgumentExpression(nameof(code))] string? paramName = null);   // 调用方信息参数，由编译器填写
    public static string KindOf(string code);
    public static string DomainOf(string code);
}
```

- `Code` 必须符合 `<领域大写>_<KIND>_<NNN>`；工厂方法校验 KIND 段与方法名一致。
- `Message` 面向人，不参与程序判断；`Details` 只允许 JSON 可表示的值。
- `MessageKey` 文法 `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`（首段与后段同形，操作 ID 的资源段把连字符换成下划线后可直接作首段，如 `tenant_members.list.description`），文案在语言文件里（RFC-0010）；`message` 线上字段是发送端已解析的文案，`messageKey` 让接收端可再翻译。

### Maybe

```csharp
public static class Maybe
{
    public static Maybe<T> Some<T>(T value);            // 值不能为 null
    public static Maybe<T> None<T>();
    public static Maybe<T> From<T>(T? value) where T : class;
    public static Maybe<T> From<T>(T? value) where T : struct;
}

public readonly record struct Maybe<T>
{
    public bool HasValue { get; }                       // default 即无值
    public static implicit operator Maybe<T>(T value);
    public bool TryGetValue([MaybeNullWhen(false)] out T value);
    public T ValueOr(T fallback);
    public T ValueOrThrow();
    public TOut Match<TOut>(Func<T, TOut> onSome, Func<TOut> onNone);
    public Maybe<TOut> Map<TOut>(Func<T, TOut> map);
    public Maybe<TOut> Bind<TOut>(Func<T, Maybe<TOut>> bind);
    public Result<T> ToResult(Error error);
}
```

线上编码：有值为值本身，无值为 `null`。

### TypedId

```csharp
public interface ITypedId<out TValue> where TValue : notnull
{
    TValue Value { get; }
}

public readonly record struct TypedId<TTag, TValue>(TValue Value) : ITypedId<TValue>
    where TValue : notnull, IEquatable<TValue>;
```

`TTag` 只用于区分类型，不承载数据。线上编码为 `TValue` 的编码（string / uuid / int64）；用户自定义的标识类型实现 `ITypedId<TValue>` 即可被生成器与协议适配器识别。

### InvocationContext 与 BaggageKeys

```csharp
public sealed class InvocationContext
{
    public required CancellationToken CancellationToken { get; init; }
    public DateTimeOffset? Deadline { get; init; }
    public required string TraceId { get; init; }                        // 32 位小写十六进制
    public required string SpanId { get; init; }                         // 16 位小写十六进制
    public required IReadOnlyDictionary<string, string> Baggage { get; init; }   // 键必须符合 xihan.<name>；构造时复制成只读副本
    public IDictionary<object, object?> Items { get; }                   // 不传播；派生上下文共享，可并发读写
    public required IServiceProvider Services { get; init; }
    public bool TryGetBaggage(string key, [MaybeNullWhen(false)] out string value);
    public InvocationContext WithBaggage(string key, string value);
    public bool IsExpired(DateTimeOffset now);
    public TimeSpan? RemainingTime(DateTimeOffset now);
}

public static class BaggageKeys
{
    public const string Tenant = "xihan.tenant";
    public const string Principal = "xihan.principal";
    public const string Correlation = "xihan.correlation";
    public const string IdempotencyKey = "xihan.idempotency-key";
    public const string Locale = "xihan.locale";
    public static IReadOnlyList<string> Registered { get; }
    public static bool IsWellFormed(string? key);
    public static bool IsRegistered(string? key);
}
```

语义见 RFC-0003。

### 稳定性、生命周期与管道阶段

```csharp
public enum Stability { Stable, Preview, Experimental, Internal, Deprecated }

[AttributeUsage(AttributeTargets.All, Inherited = false, AllowMultiple = false)]
public sealed class ApiLevelAttribute(Stability stability, string since) : Attribute
{
    public Stability Stability { get; }
    public string Since { get; }
    public string? ScheduledStable { get; init; }
}

public enum LifecycleState { Declared, Resolved, Configured, Started, Ready, Degraded, Draining, Stopped, Failed }

public static class LifecycleStateExtensions
{
    public static bool CanTransitionTo(this LifecycleState from, LifecycleState to);
    public static bool IsServing(this LifecycleState state);     // Ready 或 Degraded
    public static bool IsTerminal(this LifecycleState state);    // Stopped
}

public enum PipelineStage { Ingress, Observe, Resilience, Authenticate, Tenant, Authorize, Validate, Idempotency, Transaction, Handle, Egress }

public static class PipelineStageExtensions
{
    public static IReadOnlyList<PipelineStage> All { get; }
    public static string ToManifestName(this PipelineStage stage);
    public static bool TryParse(string? name, out PipelineStage stage);
}
```

合法迁移表是规范性内容，完整列表在 `spec/conformance/vectors/primitives/lifecycle-transitions.json`。管道阶段的名称与顺序是规范性内容（RFC-0004）。

### 异常与框架信息

```csharp
public class FrameworkException : Exception
{
    public FrameworkException(string code, string message);
    public FrameworkException(string code, string message, Exception? innerException);
    public string Code { get; }
    public Error ToError();
}

public static class FrameworkInfo
{
    public const string Name = "XiHan Framework";
    public const string SpecVersion = "1";
    public static string Version { get; }         // 构建期生成的常量，不经反射
    public static Uri Repository { get; }
    public static Uri Documentation { get; }
}
```

### 机密遮蔽

```csharp
public static class Redaction
{
    public const string Mask = "***";
    public static string? Redact(string? value);   // null 原样返回，其余（含空串）一律 Mask
}
```

选项与携带凭据的记录在 `ToString()`（记录的 `PrintMembers`）里以它打印口令、密钥、令牌与整串连接串（RFC-0014「选项里的凭据」）。`Mask` 与 `ConnectionStrings.Mask`、`SecretValue.Redacted` 是同一个文本。

### 清单模型（`Manifests`）

与 `spec/manifest/manifest.schema.json` 一一对应的不可变记录，只做数据与格式校验：

| 类型 | 内容 |
| --- | --- |
| `Manifest` | 全部字段；`Validate()` 检查跨字段约束（插件必须声明隔离、非插件不得声明权限、不得重复提供或依赖） |
| `ManifestKind` / `ManifestRole` / `PluginIsolation` | 枚举 |
| `CapabilityDeclaration` | `id@version`，可 `Parse` / `TryParse` |
| `CapabilityRequirement` | `id@[^]version[?]`，可 `Parse` / `TryParse`，`IsSatisfiedBy(declaration)` |
| `Binding` | 绑定名与资源类型 |
| `Contributions` / `PipelineContribution` / `ConfigContribution` | 贡献点 |
| `CapabilityId` / `ManifestId` | 格式校验与领域段提取 |

接口版本是整数；`^N` 与 `N` 等价。收集、解析与求解是内核的职责。

清单记录的构造只收必需字段（`Binding(name, type)`、`CapabilityRequirement(id, version)`、`ConfigContribution(section)`、`PipelineContribution(middleware, stage)`），可选字段（`Required`、`Optional`、`Schema`、`Before`、`After`）经初始化器设置：原语层只增不改，以后加字段只加可选的初始化字段，不改构造的参数表。`TryParse` 失败时输出为空（.NET 以 `[NotNullWhen(true)]` 标注）。`Error.Unexpected` 的参数顺序与构造一致：先细节、后内部错误（2026-11-11 修订，ADR-0137）。

## 稳定性级别

`Stability` 的五个级别是对使用者的承诺，以同一大版本内的小版本与补丁版本衡量；大版本之间的破坏另列破坏清单。

| 级别 | 同一大版本内 | 破坏的要求 |
| --- | --- | --- |
| `Stable` | 不破坏 | 不允许；要去掉只能先转 `Deprecated`，到下一个大版本移除 |
| `Preview` | 小版本可以破坏，补丁版本不破坏 | 每一处破坏单独登记豁免，写明原因，并在更新日志给出迁移方式 |
| `Experimental` | 小版本可以破坏或移除，补丁版本不破坏 | 同 `Preview` |
| `Internal` | 不承诺 | 只供声明它的包自己使用，被其他包引用即构建失败 |
| `Deprecated` | 保留至少两个小版本，下一个大版本前的最后一个小版本升为编译错误 | 只在大版本移除 |

- **什么算破坏**：删除公开成员；改签名，含参数、默认值与可空注解；给使用者或 Provider 要实现的接口、抽象类加抽象成员；改基类或加密封；签名不变但语义改变（返回值、失败方式、副作用）也算。
- **生效级别**：成员没有单独标注时取所在类型的级别；成员单独标注的级别不得高于所在类型。
- **只升不降**：同一大版本内，级别可以在任意小版本从 `Experimental` 升到 `Preview` 再升到 `Stable`，不能从 `Stable` 降回 `Preview` 或 `Experimental` 来绕开破坏限制；`Stable` 要去掉只能走 `Deprecated`。
- **`Stable` 不依赖不稳定的类型**：`Stable` 成员的签名（参数、返回值、基类、实现的接口、泛型约束，泛型实参一并算）不得引用 `Preview` 或 `Experimental` 的类型，否则后者的破坏会连带破坏前者。实现了 `Preview` 接口的实现类型随接口保持 `Preview`；`Stable` 类型里个别成员要引用不稳定类型时，那个成员单独标注较低的级别。只做组装的元包（Profile）级别不高于它组装的组件。.NET 由分析器 XH0006（签名引用）与 XH0007（成员级别高于所在类型）在编译期检查（2026-11-11 修订，ADR-0167）。
- **原语层**：整体冻结（见「冻结规则」），不适用破坏豁免。
- **数据类型的形状**：会演进或跨 Provider 边界的数据类型（请求、响应、状态、事件、记录）不用位置记录。构成身份的一两个必需成员经构造给出（照清单记录的写法），其余成员一律经初始化器给出，必需的标 `required`；以后加字段只加可选的初始化字段，不改构造的参数表，也不产生新的解构签名。初始化器在设值时校验，`with` 走同一校验。位置记录只留给封闭的小值，如 `TypedId`、`ResourceName`（2026-11-11 修订，ADR-0142；逐项结果不再另建记录，见批量优先，ADR-0154）。
- **缺失用 `null`**：公开契约里可选的值缺失时用 `null`，不用空字符串或默认实例，例如匿名主体的 ID、全局资源的租户段。
  - 例外是持久化写入的协议字段（`TenantId`、`ConcurrencyStamp`）：已持久化的行一定有值，所以类型不可空；新建实体在新增前以空字符串占位，新增时由持久化覆写。
  - 存储格式可以另行编码，在契约边界换算。例如授予表以空字符串表示全局租户，因为唯一索引对 NULL 的语义各数据库不一致（2026-11-11 修订，ADR-0145）。
- **标识值**：子系统里以字符串承载、带文法校验的标识（缓存键、对象键、容器名、作业名、工作流名、活动类型、密钥名等）是值类型，各自在构造时校验。
  - 从字符串创建会校验并在不合法时抛出，所以只能经构造或显式转换；隐式转换不得抛出，因而不提供从字符串的隐式转换。
  - 值类型的 `default` 不是合法标识：读它的文本抛出 `InvalidOperationException`，`ToString` 返回空串。Provider 第一次读文本就失败，接收标识的入口不必逐一判断 `default`；由多个标识组成的值（如授予主体、触发器键）在构造时拒绝 `default` 成员。
  - 来自不可信输入的文本先用各类型的 `IsValid` 判断，不靠捕获异常（2026-11-11 修订，ADR-0152）。
- **查找的命名**：方法名决定返回的形状，调用方看名字就知道「没有」怎么表示。
  - `Find…` 返回 `Maybe<T>`（异步与可失败时包在 `Result` 或任务里），没有是正常结果，例如按标识查实体、按名查绑定、读调用上下文里的条目。
  - `TryGet…` 只用于 `bool` 加 `out` 参数，与 BCL 的 `TryGetValue` 一致。
  - `Get…` 必定交出值：没有时是失败结果（如 `NOT_FOUND`）或编程错误，不返回 `null` 或空的 `Maybe`。
  - 例外是键值存储的读：缓存条目的 `GetAsync` / `GetManyAsync` 沿用键值存储的通用词汇返回 `Maybe`（见批量优先的命名表）；实现 BCL 或第三方接口的成员照原接口命名（2026-11-11 修订，ADR-0153）。
- **分页**：两种形状，各用一套固定的成员名。
  - 偏移分页用于管理查询（可跳页、要总数）：查询带 `Skip` 与 `Take`，上限常量 `MaxTake`；结果页是 `Items` 与 `long Total`。
  - 续页分页用于存储列举（大集合、后端游标）：查询带 `PageSize` 与 `ContinuationToken`，上限常量 `MaxPageSize`，缺省 `DefaultPageSize`；结果页是 `Items` 与 `ContinuationToken`，没有下一页为 `null`，令牌不透明、原样带回。
  - 分页越界不在构造时抛出，由读取方法以校验错误返回，参数名与成员名一致（`skip`、`take`、`pageSize`、`max`）。
  - 结果页按初始化器形状给出，不用位置记录（2026-11-11 修订，ADR-0154）。
- **可选参数**：公开方法、构造与索引器不声明默认值，默认值会编译进调用方，以后加参数或改默认值就破坏二进制兼容。
  - 一两个可选值用重载补齐：至少有不带它们的短重载与参数齐全的长重载，中间一级按需要提供，如 `DomainEventBuffer()` 与 `DomainEventBuffer(int capacity)`、`Error.Unexpected(code, messageKey)` / `(code, messageKey, details)` / `(code, messageKey, details, inner)`。短重载转调长重载，默认值只写在短重载的实现里。
  - 更多的可选值，或以后还会增加的，收进单次调用的选项记录，以后加字段只加属性，如 `App.CreateContext(services, InvocationContextOptions, cancellationToken)`、`BlobPutOptions`、`UnitOfWorkOptions`。
  - `CancellationToken` 放在最后，不设默认值。
  - 调用方信息参数（.NET 的 `CallerArgumentExpression`、`CallerMemberName` 等）由编译器填写，不算可选参数。
  - .NET 由分析器 XH0008 对生效级别为 `Stable` 的成员在编译期检查；`Preview` 与 `Experimental` 可以在小版本破坏，升 `Stable` 时这条规则随之生效（2026-11-11 修订，ADR-0167）。
  - 例外是面向源码使用的工具链包（`Testing`、`Conformance`）：测试项目总是随框架重新编译，可选参数保留，新参数只加在末尾（2026-11-11 修订，ADR-0157）。
- **Provider 契约的演进**：Provider 要实现的接口一经发布就被各家实现锁住。新能力与新条件按下面的方式加，旧 Provider 不会在不知情时给出错误或放宽了的结果（2026-11-13 修订，ADR-0228）。
  - 新能力走可选接口：另建一个接口，能做的 Provider 实现它，调用方按类型判断（.NET 用 `is`），没有就走降级路径或返回「不支持」的错误。先例有 `ICacheStoreInspector`、`IBlobMultipartStore`、`IPermissionGrantAdministration`、`ITenantAdministration`。
  - 不在已有的 Provider 接口上加成员：对 `Stable` 这是破坏；`Preview` 与 `Experimental` 只在这项能力是每个 Provider 都必须有、缺了契约就不成立时，才登记豁免这样加（如锁续约要的比较后重设到期，ADR-0215）。
  - 不用带默认实现的接口成员补能力：默认实现不是静默降级就是抛出，前者掩盖缺失，后者把编译期就能发现的缺失推到运行期。
  - 查询带修订号：交给 Provider 的查询（过滤项与排序键）各有一个修订号常量 `Revision`，5.0 的全部条件属于修订 1；以后新增条件时常量加 1，新条件记在新修订号下。查询按这次实际用到的条件得出所需修订号，没有用到新条件时它不变，旧 Provider 照常服务。
  - Provider 在入口以编译时取到的修订号核对（.NET 写作 `query.Validate(AuditQuery.Revision)`，常量在编译时写进 Provider），所需修订号更高即返回 `KERNEL_VALIDATION_007`，不读不认识的条件，也不给出放宽了的结果；签名不返回 `Result` 的（仓储的 `ListAsync`、`CountAsync`）以同一错误码抛出。跨进程的 Provider（远程操作集）在请求里带上所需修订号，由插件按自己实现到的修订号核对。
  - 选项记录（如 `BlobPutOptions`、`UnitOfWorkOptions`）新增的字段，被旧 Provider 忽略时必须仍然安全，只是增强没有生效；忽略了会放宽访问、丢数据或改变结果的，不加字段，走可选接口。

各语言实现把「登记豁免」落到自己的工具链上。.NET 用包验证的 `CompatibilitySuppressions.xml` 与 PublicAPI 文件的 `*REMOVED*` 行，门禁核对豁免只指向非 `Stable` 的成员（ADR-0131）。

## 跨语言语义

| 原语 | 必须一致 |
| --- | --- |
| Result | 成功与失败互斥；失败必有 Error；JSON 形态 |
| Error | Code 格式与 KIND 段；Details 仅 JSON 值；Inner 链 |
| Maybe | 与 `null` 的编码等价，但类型层面不可为空 |
| TypedId | 编码为底层值；标签不上线 |
| InvocationContext | 字段集合、W3C 格式、Baggage 键格式与传播载体（RFC-0003） |
| Stability | 五级含义、只升不降、`Stable` 不依赖不稳定的类型、弃用流程与 Provider 契约的演进（见「稳定性级别」） |
| LifecycleState | 九态与迁移表 |
| PipelineStage | 十一阶段的名称与顺序 |
| 清单 | Schema、ID 格式、依赖文本语法 |

线上 JSON 编码由协议适配器与源生成器实现，原语层不含序列化代码（保持零行为与零反射）。

## 冻结规则

只增不改；破坏即新包 `XiHan.Framework.Primitives.V2` 并存。公开面以 `PublicAPI.Shipped.txt` 为准。

## 一致性向量

`spec/conformance/vectors/primitives/`：`error-codes.json`、`lifecycle-transitions.json`、`capability-requirements.json`、`baggage-keys.json`。Result/Error/Maybe/TypedId 的编码向量随协议适配器（P3）补充。

## 备选方案

- 把原语放进 Kernel：否决，内核会随小版本变化，原语层必须独立冻结。
- 用异常代替 Result：否决，可恢复失败不应走异常，且跨语言不可移植。
- 在泛型类型上声明工厂静态成员（`Result<T>.Success`）：否决，统一放在非泛型 `Result` / `Maybe` 上。
