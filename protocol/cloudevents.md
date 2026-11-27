# CloudEvents 载体

> 规范性。参考实现：`XiHan.Framework.Protocols.CloudEvents`。消息子系统信封（RFC-0009）的 CloudEvents 1.0 HTTP 协议绑定。

## 信封映射（RFC-0009）

信封就是 CloudEvents 1.0 事件：`id`、`source`、`type` 必填，`time`、`subject`、`dataschema`、`datacontenttype`、`data` 可选（`dataschema` 的取值规则见 RFC-0009「信封」：与 `source` 一样只要求是非空字符串，不核对 URI 语法），扩展属性名 `^[a-z0-9]{1,20}$` 且不是 CloudEvents 的规范属性名与结构化 JSON 格式的成员名（保留名的清单见 RFC-0009「信封」；2026-11-25 修订，ADR-0298）。`dataschema` 在 structured 与 batch 里是同名的成员，在 binary 里是 `ce-dataschema` 头（2026-11-25 再修订，ADR-0305；ADR-0298 时信封不承载它，带这个成员或这个头即信封不合法）。三种模式里空值都不合法：structured 与 batch 的 `"dataschema": ""`，binary 的空 `ce-dataschema` 头（2026-11-27 修订，ADR-0323；以前 binary 的空头与没有这个头相同）。`subject` 与 `time` 同样（2026-11-27 再修订，ADR-0332）：structured 与 batch 的 `"subject": ""`（以前读成空串的 `subject`；CloudEvents 1.0 规定 `subject` 出现时必须是非空字符串）与 `"time": ""`（本来就不合 `time` 的文法），binary 的空 `ce-subject` 与 `ce-time` 头（以前与没有这个头相同）。插件协议的 `Envelope.data_schema`、`subject` 与 `time` 空串表示没有（`plugin/invocation.md` §4），那是 proto3 的字符串字段分不出空串与没有的约定，与这里的读法不同、也不随之改变。`ce-data` 头仍不合法：`data` 是结构化 JSON 格式的成员名，是保留名。框架扩展 `xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale`（按 `conventions/baggage-keys.md` 的 CloudEvents 载体进入 Baggage）与 `traceparent`（追踪）在派发时进入调用上下文。

## 入站（RFC-0003 上下文：承载请求的头部按 `protocol/http.md` 映射；事件自身的上下文来自扩展属性）

| 模式 | 识别 | 事件 |
| --- | --- | --- |
| structured | `Content-Type: application/cloudevents+json` | 请求体为结构化 JSON（`CloudEventsFormat`） |
| batch | `Content-Type: application/cloudevents-batch+json` | 请求体为结构化 JSON 的数组，按顺序处理 |
| binary | 其他内容类型且有 `ce-specversion` 头 | `ce-id`、`ce-source`、`ce-type` 必填，`ce-time`（与结构化 JSON 的 `time` 同一条文法，RFC-0009「信封」第 3 条）、`ce-subject`、`ce-dataschema`（解码之后照原样作为 `dataschema`，ADR-0305）可选，三者的值为空都与结构化 JSON 的空串一样不合法（`ce-dataschema` 2026-11-27 修订，ADR-0323；`ce-subject` 与 `ce-time` 同日再修订，ADR-0332；以前都与没有这个头相同），其余 `ce-*` 为扩展属性；`Content-Type` 为数据内容类型（缺省 `application/json`），请求体为数据；`ce-*` 头值按下面「binary 的头值」解码 |

- 都不满足 → 400 `PROTOCOLS_VALIDATION_008`；信封不合法 → 400 `MESSAGING_VALIDATION_002`：structured 与 batch 的每个事件按 RFC-0009「信封」的五条判定（2026-11-24 修订，ADR-0283；以前这里只列规范版本、必填属性、时间、扩展名四类，`time` 没有偏移时 .NET 按本机时区读），binary 核对规范版本、必填属性、`ce-time` 的文法与扩展名，以及头值的解码、`ce-dataschema` 不为空与声明为 JSON 的数据（后三项 2026-11-27 补，ADR-0323），`ce-subject` 不为空（同日再补，ADR-0332）。空的 `ce-time` 头与结构化 JSON 的 `"time": ""` 同一个错误（`messaging.envelope.invalid_time`，参数 `time` 为空串），空的 `ce-subject` 头与 `"subject": ""` 同一个错误（`messaging.envelope.invalid`，与空的 `ce-dataschema` 头同一条路，ADR-0323）。
- binary 的头值（2026-11-27 写明，ADR-0323；以前 .NET 用 `Uri.UnescapeDataString`，非法的 UTF-8 百分号序列如 `%ED%A0%80`、结尾截断的 `%C3` 与后面不是两位十六进制数字的 `%` 原样留成百分号文本，照常读出）：每个 `ce-*` 头的值先按 RFC 7230 §3.2.6 解开双引号括起的部分，再做一轮百分号解码，即 CloudEvents HTTP 协议绑定 1.0.2 §3.1.3.2 的解码（解开双引号是 2026-11-27 写明的，ADR-0332；以前 .NET 把引号读成值的一部分，CloudEvents 要求接收方支持，较早版本的发送方不编码空格与双引号、可能用引号括起整个值）：
  - 解开双引号：从左到右，引号之外的字符照原样；`"` 开始一个 quoted-string，到下一个没有转义的 `"` 为止，两个引号不算内容；quoted-string 里 `\` 与紧跟的一个字符（quoted-pair）是这个字符本身，其余字符照原样。括起的部分与没括起的部分可以相连（`a"b c"d` 读成 `ab cd`）。引号之外的 `\` 是普通字符。
  - 百分号解码：把上一步得到的值逐个字符换成字节，`%` 与紧跟的两位十六进制数字（大小写都收）是一个字节，其余字符是它自己的 UTF-8 编码，再把这些字节按 UTF-8 解码成属性的值。不必要的编码照收（`%41` 读成 `A`）。解开双引号之后的文本只做这一轮，`"%2541"` 读成 `%41`，不再读成 `A`。

  下面任一情形即这个头的值读不出，信封不合法，为 400 `MESSAGING_VALIDATION_002`，消息键 `protocols.cloudevents.header_invalid`，参数 `header` 是头名（小写）：
  - 坏的 quoted-string（2026-11-27 写明，ADR-0332）：引号不配对（开始的 `"` 之后再没有结束的 `"`，如 `a"b`）；quoted-string 里有 RFC 7230 的 qdtext 不允许的字符（水平制表符之外的控制字符 U+0000–U+001F 与 U+007F）；`\` 在 quoted-string 的末尾、后面没有字符，或后面是这样的控制字符。U+0080 及以上的字符按 obs-text 照收。
  - `%` 后不足两个字符，或不是两位十六进制数字。
  - 换出的字节不是合法的 UTF-8：过长编码（如 `%C0%A0`）、编码成 UTF-8 的代理项（`%ED%A0%80`–`%ED%BF%BF`）、截断的多字节字符（如结尾的 `%C3`）、超出 U+10FFFF 的码位。
  - 值里直接就有孤立的代理项（RFC-0001「孤立的代理项」），编不成 UTF-8。
- binary 声明为 JSON 的数据（2026-11-27 写明，ADR-0323；以前原样收下，structured 会拒绝的数据照常派发或发布，发布到远程代理时才在写出一侧失败）：数据内容类型以 `application/json` 开头、或去掉参数之后以 `+json` 结尾（`Content-Type` 缺省时是 `application/json`）的数据，过与 structured 相同的检查。把数据当作结构化 JSON 文档里的 `data`（文档是第 1 层，`data` 是第 2 层），这份文档要过 RFC-0001「JSON 文本」的共同检查：数据不是 JSON、嵌套超过 63 层（文档超过 64 层）、字符串或成员名不是合法的 Unicode、任何一层有重复的成员名，都是信封不合法。错误与 structured 读同一份文档时相同：400 `MESSAGING_VALIDATION_002`，消息键 `messaging.envelope.invalid_json`，`reason` 是文档里的位置（语法错误为 `$`，重复成员如 `$.data.a`）。读出的信封因此都写得成结构化 JSON。请求体为空时没有数据，不做这道检查。
- 主题 = `XiHan:Protocols:CloudEvents:Topics:<type>`；没有映射的类型 400 `PROTOCOLS_VALIDATION_009`。打开 `XiHan:Protocols:CloudEvents:AcceptUnmappedTypes`（缺省 `false`）时，没有映射的类型以 `type` 本身为主题（须满足主题文法，否则同样 400）（2026-11-12 修订，ADR-0200；以前缺省就以类型本身为主题，只被允许投递某一种事件的发送方能投进任何有订阅的主题，包括插件订阅的主题）。
- `Inbound = Dispatch`（默认）：`MessageDispatcher` 在请求内以新作用域、新上下文经完整内核管道执行本实例里该主题的处理器（`MessageDelivery.Attempt = 1`）：竞争订阅与广播订阅（RFC-0009「订阅方式」）各一次投递，先竞争后广播；派发只发生在接收请求的实例，要让广播订阅到达每个实例用 `Inbound = Publish`；成功 202 无正文；处理器失败按错误 KIND 映射 HTTP 状态（RFC 9457 Problem Details），发送方据此重试；主题在本实例没有任何处理器（两种订阅方式都没有）→ 404 `PROTOCOLS_NOTFOUND_003`。
- `Inbound = Publish`：经 `IMessagePublisher` 发布，成功即 202；至少一次投递、重试与死信由代理负责（含发件箱）。
- 上限（2026-11-12 修订，ADR-0187）：请求体不超过 `XiHan:Protocols:CloudEvents:MaxRequestBodyBytes`（缺省 1 MiB），边读边计数，超过即 400 `PROTOCOLS_VALIDATION_013`（参数 `max`）；batch 的事件数不超过 `MaxBatchSize`（缺省 100），超过即 400 `PROTOCOLS_VALIDATION_014`（参数 `count`、`max`）。两者都在解析主题之前判定，一个事件也不处理。以前两者都没有上限，只受宿主服务器的请求体上限约束。重放不另设防线：事件按至少一次的语义处理，处理器按 `id` + `source` 去重。
- batch：先把每个事件的类型解析成主题，任一事件没有映射整批 400、不处理任何事件；派发模式逐个处理、第一个失败即停止并作为响应，发布模式按主题分组（保持首次出现的主题顺序与组内顺序）各一次批量发布 `PublishManyAsync(topic, envelopes)`、第一个失败的结果作为响应；已处理的不回滚（至少一次语义，处理器按 `id` + `source` 去重）。
- 端点不带授权要求（已定，2026-11-04，ADR-0121）：扩展属性 `xihantenant`、`xihanprincipal` 是发送方给的，派发时不与请求的身份核对，能调端点的调用方就能选租户、写审计主体。这是这个协议的设计，框架不收紧。权限由宿主在端点上加，只开给可信的内部服务。

## 出站

- `CloudEventsWebhookPublisher.PublishAsync(topic, envelope)`：POST 到 `XiHan:Protocols:CloudEvents:Webhooks:<topic>`；`Outbound = Binary`（默认，`ce-*` 头 + 数据体；信封有 `dataschema` 时写 `ce-dataschema` 头，没有则不写，ADR-0305）或 `Structured`（`application/cloudevents+json` 体）。binary 的头值按 CloudEvents HTTP 协议绑定 1.0.2 §3.1.3.2 百分号编码：空格（U+0020）、双引号（U+0022）、`%`（U+0025）与可打印 ASCII（U+0021–U+007E）之外的字符按 UTF-8 字节写成 `%XX`（十六进制大写），其余字符原样；写出的值因此不含空格与双引号，不用引号括起（2026-11-27 修订，ADR-0332；以前空格与双引号原样写出，CloudEvents 1.0.2 要求发送方编码这两个字符）。
- 写不出的信封在发出之前拒绝，不发请求，几种内容模式同一个口径（RFC-0009「信封」的写出）：structured 与 batch 经结构化 JSON 的写出拒绝（.NET 的 `CloudEventsFormat.Encode`）；binary 在写任何头之前核对 `id`、`source`、`type`、`subject`、`dataschema`、`datacontenttype`（写进 `Content-Type`）与扩展属性的值，有孤立的代理项即抛出携带 `MESSAGING_VALIDATION_002` 的异常，消息带属性名，核对的属性与先后都与结构化 JSON 的写出相同（RFC-0001「写出与转交」；2026-11-26 写明，ADR-0318；以前 .NET 的 binary 模式不经这道核对，头值的百分号编码把孤立的代理项编成 U+FFFD 的 `%EF%BF%BD`，Webhook 缺省的单条发布照样写成 U+FFFD，接收方读到的是另一个值）。binary 声明为 JSON 的数据（判定同入站）同样照结构化 JSON 的写出核对：数据不是 JSON、有重复的成员名、嵌套让结构化文档超过 64 层，或字符串里有不成对的代理项转义或不合法的 UTF-8 字节，都在写任何头之前抛出，异常与 structured 的相同（2026-11-27 写明，ADR-0323；以前 binary 把数据原样作请求体，structured 会拒绝的数据照发）。
- `PublishManyAsync(topic, envelopes)`（批量优先）：一条沿用上面的模式；多条用 batch 内容模式（`application/cloudevents-batch+json`，结构化事件的数组、顺序同入参）一次 POST，接收方须支持该模式，一个响应对整批生效（逐项结果相同）。
- 没有配置 → `PROTOCOLS_NOTFOUND_004`；响应非 2xx → `PROTOCOLS_UNAVAILABLE_001`（`status` 在详情里）；连接失败 → `PROTOCOLS_UNAVAILABLE_002`。结果只看状态码，读到响应头即判定，响应体不读（2026-11-12 修订，ADR-0188；以前先把响应体整个缓冲进内存）。重试由调用方决定（经发件箱投递即获得至少一次语义）。
- 使用命名 HttpClient `xihan.protocols.cloudevents`。
- 过出站地址策略（已定，RFC-0008、ADR-0117）：地址是运维配的，但接收方是第三方，它的 3xx 能把请求引到内网，所以这个命名客户端的连接与重定向都检查。接收方在内网时要放行。被拒绝按连接失败报告 `PROTOCOLS_UNAVAILABLE_002`，`reason` 是 `OutboundAddressDeniedException` 的类型全名，不发起连接。整个替换主处理器后由替换后的处理器决定能连哪里。
- HTTP 层的追踪头缺省不传播（已定，ADR-0083）：这个命名客户端不写 W3C 追踪上下文头 `traceparent`、`tracestate`、`baggage`（也不写旧格式的 `Request-Id`、`Correlation-Context`）。启用追踪时 .NET 的 HttpClient 本会按当前 Activity 写这些头，与信封扩展无关，同样会把内部的追踪 ID 与 Baggage 推给第三方，与下一条同理。`XiHan:Protocols:CloudEvents:PropagateTraceContext = true`（缺省 `false`，只接受 `true` / `false`，否则装配时 `ArgumentException`）显式打开，按 .NET 的全局传播器（`DistributedContextPropagator.Current`）写。只作用于这个命名客户端的主处理器（`SocketsHttpHandler.ActivityHeadersPropagator`），不改进程全局的传播；出站请求自身的客户端跨度照常按追踪配置记录。应用整个替换这个客户端的主处理器时由替换后的处理器决定是否传播；只想调连接参数而保留缺省，用 `UseSocketsHttpHandler` 配置同一个处理器。
- 不外推内部标识（已定，ADR-0074）：信封按调用方给出的原样发出，Webhook 发布者不像内部发布者（RFC-0009「发布侧补齐」）那样从当前调用补扩展，`xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale` 与 `traceparent` 都不由框架加上。理由：Webhook 的接收方是第三方，租户 ID、主体 ID、关联 ID 是本系统的内部标识，推出去会暴露内部结构与身份信息，并让第三方依赖本该随时可变的内部 ID；追踪链要不要跨出组织边界也由调用方决定。要给接收方的扩展（包括 `traceparent`）由调用方显式写进信封，或以上一条的配置打开 HTTP 头的传播。
