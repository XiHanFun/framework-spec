# RFC-0001 原语层

> 状态：已采纳（2026-10-28 修订：消息键首段放开下划线；2026-11-01 修订：机密遮蔽 `Redaction`；2026-11-16 修订：无值成功与 Error 成员的线上编码、消息键最长 128 个字符、接口版本只在依赖里带 `^`；同日再修订：重复成员不合法、新增原语 `TraceParent`、`WithBaggage` 核对值；同日第三次修订：JSON 文本的共同读取检查、重复成员在任何一层都不合法、文法按整串匹配；2026-11-19 修订：.NET 的 Result 与 Error 线上编码在内核的 `ResultJson`；同日再修订：`Error` 的文案不能只有空白，空白字符写死；2026-11-24 修订：收调用上下文的契约不另收取消令牌；同日再修订：.NET 映射补上 `DeadlineText` 与 `ActivationEvent`；同日第三次修订：JSON 文本的共同检查覆盖消息信封，.NET 映射补上 `TimestampText` 与内核的 `JsonText`；2026-11-25 修订：.NET 映射里 JSON 文本检查的入口改名 `JsonText.FindProblem`，问题带种类；同日写明：以字符串交来的文本严格编码成 UTF-8，孤立的代理项不合法；2026-11-26 写明：写出与转交的 JSON 文本同样在字符串与 UTF-8 之间严格转换；同日再修订：写出与转交扩到 SDK 以字符串交回的应答载荷与发布的 `data`、宿主转交的插件应答载荷，以及写进 JSON 的命令行字段选项、插件配置节与信封属性；同日再写明：操作的类型化响应写不成合法的 JSON 文本即写不出，不写成 U+FFFD 或转义） | 日期：2026-10-21 | 对应 ADR：0002、0038、0086、0233（SDK 共用的线上细节）、0235、0236、0244（.NET 的线上编码）、0263、0278（截止时间文本与激活事件的 .NET 类型）、0283（时间戳文本与 JSON 文本检查的 .NET 入口）、0291（JSON 文本检查的 .NET 入口改名与问题的种类）、0301（以字符串交来的 JSON 文本严格编码）、0312（写出与转交的 JSON 文本严格编码）、0315（写出与转交扩到应答、发布的数据与写进 JSON 的字符串）、0318（类型化的响应写不成合法 JSON 即写不出） | 参考实现：`framework/src/XiHan.Framework.Primitives` | 一致性档：`core`

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
{ "ok": true }
{ "ok": false, "error": { "code": "PERSISTENCE_CONFLICT_001", "message": "…", "messageKey": "persistence.entity.stale_stamp", "details": { … }, "inner": { … } } }
```

各语言按同一套规则读写（2026-11-16 修订，ADR-0233）。以前只给了有值的写法，无值成功被各 SDK 分别写成 `{"ok":true}` 与 `{"ok":true,"value":{}}`，互相读不出：

- `ok` 必填，是布尔。
- 有值成功（`Result<T>`）：`value` 必填。值类型是 `Maybe<T>` 时，无值的 `value` 是 `null`；其余类型的 `value` 不能是 `null`。
- 无值成功（非泛型 `Result`，或其他语言里值类型为单元类型的结果）：只写 `{"ok":true}`，不写 `value`。读取时带了 `value` 即不合法，`{}` 与 `null` 也一样。
- 失败：`error` 必填，不能带 `value`；成功不能带 `error`。
- `Error` 写出时成员顺序是 `code`、`message`、`messageKey`、`details`、`inner`：
  - `code` 必填，符合错误码文法。
  - `message` 必填，是字符串，至少含一个「空白字符」之外的字符：空串与只有空白都不合法（2026-11-19 写明，ADR-0245；以前只说非空，.NET 拒绝只有空白、其他语言照收）。
  - `messageKey` 只在有键时写（Literal 没有键），写了就必须符合消息键文法。
  - `details` 为空时不写，写了就必须是 JSON 对象。
  - 没有内部错误时不写 `inner`，写了就必须是 `Error`。
  - 可选成员缺失表示没有，写成 `null` 不合法。
- 读取方忽略未知成员：v1 期间编码只增不改，新成员一律可选。接收方按 JSON 值比较，不比较成员顺序与空白。
- 任何一层出现重复的成员名都不合法，含 `value`、`details` 与 `inner` 的内部，不取第一个也不取最后一个（2026-11-16 写明，ADR-0235；ADR-0236 扩到任何一层，以前只说同一层，各 SDK 查的深度不同）。读取先过「JSON 文本」的共同检查。
- 值与细节里的数字按 RFC-0010「模板」的值模型读：64 位有符号范围内的整数写法是整数，原样往返（9007199254740993 不能变成 9007199254740992），其余是双精度浮点（ADR-0236）。写出的数字必须让对方按同一模型读回同一个值：整数值的浮点写成带小数点或指数的写法（`2.0`、`-0.0`、`1E+17`），否则对方读成整数，`D` 与 `X` 的结果就变了；非有限的浮点在 JSON 里写不出，写成字符串 `NaN`、`Infinity`、`-Infinity`，与模板里的文本相同。

向量 `conformance/vectors/primitives/wire-encoding.json` 列出两类写法：必须读得出的（连同再写出时的规范形式），以及必须拒绝的。

### JSON 文本

XiHan 读取的 JSON 文本（Result 与 Error 的线上编码、操作载荷、清单、消息信封的结构化 JSON）先过同一遍检查，任一条不满足即不合法（2026-11-16 写明，ADR-0236；消息信封是 2026-11-24 加进来的，见 RFC-0009「信封」，ADR-0283）：

- 语法按 RFC 8259：不允许注释、尾逗号、单引号、`NaN` / `Infinity`、前导零、正号与多个根值；空文本与只有空白也不是 JSON。文本按 UTF-8 编码，开头的 BOM 是语法错误。以字符串交来的文本先严格编码成 UTF-8 再检查：字符串里孤立的代理项（.NET、Java、JavaScript 以 UTF-16 存放字符串，Python 的 `str` 也放得下代理项）编码不成 UTF-8，与非法的 UTF-8 字节序列一样不合法，位置写 `$`，不先换成 U+FFFD 再读（2026-11-25 写明，ADR-0301；以前 .NET 读插件写来的错误 `details` 时用宽松的编码，孤立的代理项悄悄变成 U+FFFD，读出一份本不合法的细节）。
- 嵌套不超过 64 层，最外层的对象或数组是第 1 层。
- 字符串与成员名必须是合法的 Unicode：没有非法的 UTF-8 字节序列（含过长编码与编码成 UTF-8 的代理项），没有原始的控制字符，转义只用 RFC 8259 的那几种；`\uD800`–`\uDBFF` 后必须紧跟 `\uDC00`–`\uDFFF`，单独的低代理项也不合法。
- 任何一层都不能有重复的成员名。名字按转义还原后逐个 UTF-16 码元比较，`"a"` 与 `"\u0061"` 重复；大小写不同不算重复。
- 语法错误不论出现在哪里都优先于重复成员。报告位置时，语法错误只写 `$`（各语言的解析器报告的位置不同），重复成员写第一处重复（第二次出现的成员）的路径，写法同 RFC-0011 规则 2。

写出与转交同一个口径（2026-11-26 写明，ADR-0312；以前只写了读取一侧，.NET 在下面列出的几处用宽松的编码）：XiHan 在字符串与 UTF-8 字节之间转换一份 JSON 文本、再写出或转交给别处读时，转换是严格的，两个方向都不先换成 U+FFFD。

- 以字符串给出、要变成字节写出或转交的（命令行 `invoke --json` 的载荷，`protocol/commandline.md`；SDK 以字符串交给宿主的回调载荷，`plugin/invocation.md` §5；SDK 的处理器以字符串交回的操作应答载荷与插件以字符串给出的发布信封 `data`，`plugin/invocation.md` §1、§5（2026-11-26 再写明，ADR-0315）；宿主从文本读取器拿到的一行 MCP 消息，如 .NET 的 `McpStdioTransport.RunAsync`）：孤立的代理项编码不成 UTF-8，按那一处不合法的 JSON 处理，不发出、不转交。
- 以字节收到、要先解码成字符串再解析的（Wasm 载体的消息，`plugin/wasm.md` §2；`xihan setup` 要合并的配置文件，RFC-0016「工具链：编码助手的知识库服务」）：非法的 UTF-8 字节序列按不是 JSON 处理。
- 以字节收到、不解析就转交出去的（宿主把插件交回的操作应答载荷交给协议适配器与调用方，`plugin/invocation.md` §1；2026-11-26 再写明，ADR-0315；以前 .NET 原样交回，之后各协议宽松地解码或转写）：转交之前过上面的共同检查，没过即按对方不合规处理，不转交。
- 把字符串写进一份要交给别处读的 JSON 文本的（命令行的字段选项，`protocol/commandline.md`；宿主写给插件的配置节，`plugin/lifecycle.md` §2；消息信封的结构化 JSON 里的属性，RFC-0009「信封」）：字符串（成员名与值）里有孤立的代理项时按那一处不合法的输入处理，不写出、不转交（2026-11-26 修订，ADR-0315）。JSON 写出器缺省把孤立的代理项换成 U+FFFD（System.Text.Json 写成转义的 U+FFFD），得到的同样是另一份合法的 JSON。
- 把操作的类型化响应写成载荷的（宿主与 SDK 的 XGI 入口，RFC-0011 规则 10 的「编码」；插件一侧写进 `InvokeResponse.payload` 的就是它，`plugin/invocation.md` §1）：响应里的字符串（成员名与值，含字典的键）有孤立的代理项，或者在以字节存放字符串的语言（Go、PHP）里不是合法的 UTF-8，即响应写不出，以 `KERNEL_UNEXPECTED_001` + `kernel.invocation.unhandled_exception` 结束，不写成 U+FFFD，也不写成 `\ud800` 这样的转义（2026-11-26 再写明，ADR-0318；以前 System.Text.Json 与 Go 的 `encoding/json` 写成 U+FFFD，调用照常成功，TypeScript 与 Python 的写出器写成转义、插件交回之后由宿主按上面一条拒绝，Java 在插件一侧就失败）。

换成 U+FFFD 得到的仍是一份合法的 JSON：收的一方看不出交来的文本是坏的，转交出去的已经是另一份文本（写回文件时连原来的字节也改掉了）。上面列出之外经 JSON 写出器写出的字符串值怎样处理孤立的代理项，不在这一条里：类型化的请求对象（类型化调用为 Schema 校验编出的那一份，RFC-0011「校验」），AI 工具的参数，消息与领域事件的数据，持久化、缓存与审计记录的序列化（幂等记录里的响应就是上一条写出的载荷，不另外写），Result 与 Error 的线上编码与错误细节，HTTP 的 Problem Details、MCP 的应答与日志（2026-11-26 修订，ADR-0315；以前这一句把经 JSON 写出器写出的字符串值整个排除在外，命令行的字段选项、插件的配置节与信封的属性照常写成 U+FFFD；同日再修订，ADR-0318：类型化的响应对象从这里移到上一条，移的只是操作的成功响应编成载荷的那一处）。

向量 `conformance/vectors/primitives/json-text.json`；Go 一致性模块的 `rules.CheckJSONText` 是同一检查的独立实现，.NET 的是内核的 `JsonText.FindProblem`（`XiHan.Framework.Kernel.Json`，内核之外读这类文本的包也用它，ADR-0283；2026-11-25 修订，ADR-0291，原名 `JsonText.Check`）：通过时返回无值的 `Maybe`，否则是第一个问题 `JsonTextProblem { Kind, Path }`。`Kind` 把上面所说的语法错误按出处分成 `Syntax`（RFC 8259 的文法，含 BOM、原始的控制字符与 RFC 8259 之外的转义）、`NestingTooDeep`、`InvalidUtf8` 与 `LoneSurrogate`，重复成员是 `DuplicateMember`，几种语法错误之间按在文本里出现的先后报第一处。这是 .NET 的诊断信息，规则与报告的位置仍照上面几条。以前各语言的解析器对这些输入各有取舍：Go 与 JavaScript 接受孤立的代理项，Python 接受很深的嵌套，System.Text.Json 不查未知成员里的重复成员。

### 文法的匹配

规范里的正则文法（消息键、错误码、Baggage 键、清单与能力 ID、操作 ID、`traceparent` 等）都按整串匹配，结尾的换行不例外（2026-11-16 写明，ADR-0236）。.NET、Python、PHP 与 Java 的 `$` 在末尾的换行前也算匹配，以前 .NET 接受 `xihan.tenant\n` 这样的键：.NET 的正则以 `\z` 结尾，Python 用 `re.fullmatch`，PHP 加 `D` 修饰符，Java 用 `matches()`。各文法向量都带结尾换行的反例。

### 空白字符

规范里说「空白」都指这 25 个码位（2026-11-19 写明，ADR-0245）：U+0009–U+000D、U+0020、U+0085、U+00A0、U+1680、U+2000–U+200A、U+2028、U+2029、U+202F、U+205F、U+3000。它等于 Unicode 的 White_Space 属性，也等于 .NET 的 `char.IsWhiteSpace` 与正则 `\s`。

各语言按这张表判断，不用内建的判断，因为它们各不相同：

- Java 的 `Character.isWhitespace` 不含 U+0085、U+00A0、U+2007、U+202F，另含 U+001C–U+001F。
- Python 的 `str.isspace` 另含 U+001C–U+001F。
- ECMAScript 的 `\s` 不含 U+0085，另含 U+FEFF。
- PHP 的 `ctype_space` 只认 ASCII。
- RE2（Go）的 `\s` 只认 ASCII。

用到它的地方：`Error` 的文案（「Result」与「Error 与 ErrorCode」）、插件握手行与回调端点（`plugin/handshake.md` §2）、读入插件错误的 `text`（`plugin/invocation.md` §3）、缓存键与键前缀（RFC-0006「存储契约」）。

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
- 自由文本（`Literal` 的 `text`、`WithText` 的参数）不能为空，也不能只有空白（「空白字符」），否则是参数非法（2026-11-19 写明，ADR-0245）。带键的错误不需要自由文本：`Message` 是按键渲染的文案，或键与参数的描述。
- `MessageKey` 文法 `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`，最长 128 个字符（`MessageTemplate.MaxKeyLength`，2026-11-16 写明，ADR-0233）（首段与后段同形，操作 ID 的资源段把连字符换成下划线后可直接作首段，如 `tenant_members.list.description`），文案在语言文件里（RFC-0010）；`message` 线上字段是发送端已解析的文案，`messageKey` 让接收端可再翻译。

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
    public const int MaxValueLength = 256;
    public static bool IsAcceptableValue(string? value);   // 非空、按 UTF-16 码元不超过 256、不含控制字符（ADR-0204）
}

// W3C traceparent 的解析与写出（2026-11-16 增补，ADR-0235）：各载体共用一份文法
public readonly record struct TraceParent(string TraceId, string SpanId, bool Sampled)
{
    public static bool TryParse(string? text, out TraceParent value);   // 只认版本 00、小写十六进制；trace-id 或 parent-id 全零不合法
    public override string ToString();                                   // 00-<trace-id>-<parent-id>-<00|01>
}

// 截止时间文本的解析与写出（2026-11-24 增补，ADR-0278）：HTTP 载体与插件协议共用 RFC-0003 规则 8 的一条文法
public static class DeadlineText
{
    public static bool TryParse(string? text, out DateTimeOffset deadline);   // 整串匹配，不裁剪首尾空白；读不出为 false；读出的时刻偏移为零，比 100 纳秒细的小数截断
    public static string Format(DateTimeOffset deadline);                      // UTC、七位小数、+00:00 偏移
}

// 时间戳文本的解析（2026-11-24 增补，ADR-0283）：RFC-0003 规则 8 的读法只有这一份实现，DeadlineText.TryParse 转给它，消息信封的 time（RFC-0009「信封」）也用它
public static class TimestampText
{
    public static bool TryParse(string? text, out DateTimeOffset timestamp);   // 整串匹配，不裁剪首尾空白；读不出为 false；读出的时刻偏移为零，比 100 纳秒细的小数截断
}
```

语义见 RFC-0003。`WithBaggage` 给已登记的键（幂等键除外）一个 `IsAcceptableValue` 不接受的值时抛出 `ArgumentException`，与键不合文法时一样：程序内逐项派生给出的值不合法是编程错误。外部输入（载体、插件协议、Wasm 中间件）在交给 `WithBaggage` 之前按 `conventions/baggage-keys.md` 丢弃不合的成员（2026-11-16 修订，ADR-0235；以前 `WithBaggage` 不核对值，Wasm 中间件把插件给的值原样并进上下文）。

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
| `ActivationEvent` | 激活事件的文法：`IsValid(value)` 判断一项是不是 `onStartup`、`onOperation:<模式>` 或 `onEvent:<模式>`（RFC-0002 解析规则 2 第 3 步）；`Manifest.Activation` 的初始化校验与内核读清单时的形状检查共用（2026-11-24 增补，ADR-0278） |

接口版本是正整数，清单里写成十进制数字字符串（`"1"`）。只有依赖（`requires[].version` 与依赖文本）可以写成 `^N`，与 `N` 等价；`primitives` 与 `provides[].version` 只写数字（2026-11-16 写明，ADR-0233；以前的「`^N` 与 `N` 等价」没说只限依赖，与 Schema 不一致）。收集、解析与求解是内核的职责。

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
  - 收调用上下文的契约不另收取消令牌：取消与截止时间只在上下文的令牌上（ADR-0157、ADR-0263）。
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

.NET 的原语层不含序列化代码（保持零行为、零依赖与零反射）。Result 与 Error 的线上编码在内核的 `XiHan.Framework.Kernel.Json.ResultJson`（2026-11-19 修订，ADR-0244；在此之前 .NET 没有这套编码，参考实现在 `core` 里缺 `wire-encoding.json` 这个向量）：

- 写出用 `Write(Result)`、`Write<T>(Result<T>, JsonTypeInfo<T>)`、`WriteMaybe<T>(Result<Maybe<T>>, JsonTypeInfo<T>)` 与 `WriteError(Error)`，读取用 `Read`、`Read<T>`、`ReadMaybe<T>` 与 `ReadError`。无值成功对应非泛型 `Result`，值类型为 `Maybe<T>` 的结果走 `*Maybe` 这一对方法；值经调用方给的源生成元数据读写，不用反射。
- 读取返回结果的结果。外层失败表示编码不合法，码是 `PRIMITIVES_VALIDATION_001`，与各 SDK 抛出的码相同；消息键是 `kernel.wire.invalid_json`（没过「JSON 文本」的检查）或 `kernel.wire.invalid`（不合「Result」一节的规则），参数 `reason` 是出错的路径，如 `$.value`、`$.error.inner.code`。外层成功时，内层才是对方在线上说的成功或失败。
- 带 `messageKey` 的错误读成带键的 `Error`，`Text` 取 `message`；没有键的读成 `Error.Literal`。`message` 为空或只有空白（「空白字符」）时读成不合法，与「Result」一节一致。
- 值里数字的写法由 `JsonTypeInfo<T>` 决定。System.Text.Json 把整数值的 `double` 写成 `2`，无类型的对端会按值模型读成整数；要按值模型原样往返任意 JSON，值的类型用 `JsonElement`。错误细节总是按值模型读写。

其他语言的 SDK 可以把编码放在原语包里，这属于各语言自选的内部形态；编码本身以「Result」一节的规则与向量为准。

## 冻结规则

只增不改；破坏即新包 `XiHan.Framework.Primitives.V2` 并存。公开面以 `PublicAPI.Shipped.txt` 为准。

## 一致性向量

`spec/conformance/vectors/primitives/`：`error-codes.json`、`lifecycle-transitions.json`、`capability-requirements.json`、`baggage-keys.json`（含 Baggage 值的约束）、`wire-encoding.json`（Result 与 Error 的编码）、`message-templates.json`（消息键文法、模板格式化与 `Describe`，RFC-0010）、`kernel-messages.json`（SDK 与宿主共用的内核错误码、消息键、参数与 en-US 模板，RFC-0011）、`trace-context.json`（`traceparent` 的解析与写出，RFC-0003）、`json-text.json`（JSON 文本的共同读取检查，见上文「JSON 文本」）、`operation-ids.json`（操作 ID 文法与 128 个字符的上限，RFC-0011）。TypedId 的编码就是底层值，不另设向量。清单、管道与操作载荷的向量分别在 `vectors/manifest/`、`vectors/pipeline/`、`vectors/operations/`（RFC-0002、0004、0011）。

## 备选方案

- 把原语放进 Kernel：否决，内核会随小版本变化，原语层必须独立冻结。
- 用异常代替 Result：否决，可恢复失败不应走异常，且跨语言不可移植。
- 在泛型类型上声明工厂静态成员（`Result<T>.Success`）：否决，统一放在非泛型 `Result` / `Maybe` 上。
