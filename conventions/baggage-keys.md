# Baggage 键

调用上下文随请求跨进程传播的键值。键名由本文登记，格式 `xihan.<name>`，小写，连字符分隔。

| 键 | 含义 | 值格式 |
| --- | --- | --- |
| `xihan.tenant` | 当前租户 | 租户 ID 字符串 |
| `xihan.principal` | 当前主体 | 主体 ID 字符串 |
| `xihan.correlation` | 业务关联 ID | 不透明字符串 |
| `xihan.idempotency-key` | 幂等键 | 不透明字符串 |
| `xihan.locale` | 请求文化 | BCP 47 |

## 载体

| 载体 | 映射 |
| --- | --- |
| HTTP | W3C `baggage` 头，入站不收 `xihan.tenant` 与 `xihan.principal`（RFC-0003 规则 3）；Trace 走 `traceparent`；截止时间走 `xihan-deadline`（文法见 RFC-0003 规则 8） |
| gRPC | 同名元数据键，入站取舍同 HTTP |
| CloudEvents | 扩展属性，去掉点与连字符：`xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale`；Trace 走 `traceparent`（CloudEvents 分布式追踪扩展）。`xihan.idempotency-key` 不进入消息，见下文 |
| 插件协议 | `metadata` 映射，键名原样 |

敏感信息不得进入 Baggage；主体只传 ID，不传声明集。

值的约束（2026-11-12 修订，ADR-0204；以前不校验，调用方能把换行写进关联 ID 伪造日志行，超长的值进日志、审计与信封）：已登记键的值非空、最多 256 个字符、不含控制字符与孤立的代理项（孤立的代理项 2026-11-27 修订，ADR-0321），不合的成员在建调用上下文时丢弃（与 W3C Baggage 忽略不合法的成员一致）。各语言一致的口径（2026-11-16 写明，ADR-0233；以前没说怎么计长，各 SDK 有的按 UTF-16 码元、有的按码点）：

- 长度按 UTF-16 码元计，与 .NET、JavaScript、Java 的字符串长度相同，基本多文种平面以外的字符算两个。
- 控制字符指 U+0000–U+001F 与 U+007F–U+009F（.NET 的 `char.IsControl`）。
- 孤立的代理项照 RFC-0001「孤立的代理项」判定（2026-11-27 修订，ADR-0321；以前只拒控制字符）：值出进程时编成 UTF-8（HTTP 的 `baggage` 头按 UTF-8 百分号编码、插件协议的 protobuf 字符串、消息信封的扩展属性），宽松的编码把孤立的代理项换成 U+FFFD，下游读到的两个不同的值是同一个；CloudEvents 的写出一侧还因此拒绝整个信封（RFC-0009「信封」，ADR-0315）。以码位表示字符串的语言（Python）里值不能有 U+D800–U+DFFF 的码位，以字节表示字符串的语言（Go、PHP）里值必须是合法的 UTF-8。
- 外部输入在进入上下文之前丢弃不合的成员，原有的同名值保留，与 W3C 忽略不合法的成员一致。外部输入指：
  - 宿主：各载体的入站、`App.CreateContext` 的选项、插件协议的元数据、Wasm 中间件返回的 Baggage。
  - 语言 SDK：由一组 Baggage 构造上下文的入口。
- 程序内逐项派生（.NET 的 `InvocationContext.WithBaggage(key, value)` 与各 SDK 的同类方法）给已登记的键一个不合的值，是编程错误，抛出参数异常（或该语言的等价错误），不静默丢弃，也不保留原值，与键不合文法时一样（2026-11-16 修订，ADR-0235）。以前 .NET 的 `WithBaggage` 不核对值，Wasm 中间件把插件给的值原样并进上下文；各 SDK 对这种情形有的保留原值、有的去掉键。幂等键在两种情形下都例外。
- 上下文本身只携带已经通过的值，不再逐次过滤。
- 向量见 `conformance/vectors/primitives/baggage-keys.json` 的 `values`；JSON 字符串写不出孤立的代理项，这样的样例在 `acceptableUtf16` 与 `unacceptableUtf16` 里以 `utf16` 列出 UTF-16 码元，组字符串的写法同 RFC-0012「一致性」（2026-11-27 增补，ADR-0321）。

`xihan.idempotency-key` 例外：丢了它重复的调用会执行两次，不合法的幂等键由 Idempotency 阶段以校验错误拒绝。有孤立的代理项的幂等键同样不合法（它要合缓存键的文法，RFC-0006「存储契约」，ADR-0316）。写出时也不把它换成 U+FFFD：HTTP 载体写 `baggage` 头（.NET 的 `HttpInvocation.FormatBaggage`）遇到已登记键的值有孤立的代理项即抛出参数异常，不经宽松的百分号编码写成 `%EF%BF%BD`（2026-11-27 修订，ADR-0321；以前照写，两个只差在孤立代理项上的幂等键到了下游就是同一个，下游的 Idempotency 阶段把后一次调用当成前一次的重试，只回放前一次的结果）。宿主把调用、投递与 Wasm 中间件的钩子交给插件时写插件协议的 `metadata`（`plugin/invocation.md` §2）同样不写：已登记键的值有孤立的代理项即这一次失败，不交给插件（2026-11-27 修订，ADR-0334；以前 gRPC 载体的 protobuf 编码把它换成 U+FFFD，插件拿到的两个不同的幂等键是同一个，Wasm 载体的 JSON 写出器抛出异常，调用以 `KERNEL_UNEXPECTED_001` 结束）。没有 Idempotency 阶段的宿主不核对幂等键，Wasm 中间件也可能排在 Idempotency 阶段之前，这一条是幂等键交给插件之前的那道核对。上下文里只有幂等键可能带着这样的值，其余已登记的键在进入上下文时已经核对过。`xihan.locale` 还要合 BCP 47 的文法，HTTP 入站不合即当作没给，`Accept-Language` 照常协商。

消息不携带幂等键：派发为每条消息开新调用且不写幂等键（RFC-0009 投递第 4 条）。幂等键随消息传下去，同一请求发出的几条消息与同一条消息的重试会被 Idempotency 阶段当成同一次调用，第一次之后只回放结果、不再执行处理器。CloudEvents 载体因此只映射上表四个键；发布侧与派发侧的规则见 RFC-0009「信封」。
