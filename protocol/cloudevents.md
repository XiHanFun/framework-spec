# CloudEvents 载体

> 规范性。参考实现：`XiHan.Framework.Protocols.CloudEvents`。消息子系统信封（RFC-0009）的 CloudEvents 1.0 HTTP 协议绑定。

## 信封映射（RFC-0009）

信封就是 CloudEvents 1.0 事件：`id`、`source`、`type` 必填，`time`、`subject`、`datacontenttype`、`data` 可选，扩展属性名 `^[a-z0-9]{1,20}$`。框架扩展 `xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale`（按 `conventions/baggage-keys.md` 的 CloudEvents 载体进入 Baggage）与 `traceparent`（追踪）在派发时进入调用上下文。

## 入站（RFC-0003 上下文：承载请求的头部按 `protocol/http.md` 映射；事件自身的上下文来自扩展属性）

| 模式 | 识别 | 事件 |
| --- | --- | --- |
| structured | `Content-Type: application/cloudevents+json` | 请求体为结构化 JSON（`CloudEventsFormat`） |
| batch | `Content-Type: application/cloudevents-batch+json` | 请求体为结构化 JSON 的数组，按顺序处理 |
| binary | 其他内容类型且有 `ce-specversion` 头 | `ce-id`、`ce-source`、`ce-type` 必填，`ce-time`（RFC 3339）、`ce-subject` 可选，其余 `ce-*` 为扩展属性；`Content-Type` 为数据内容类型（缺省 `application/json`），请求体为数据；头值按百分号解码 |

- 都不满足 → 400 `PROTOCOLS_VALIDATION_008`；信封不合法（规范版本、必填属性、时间、扩展名）→ 400 `MESSAGING_VALIDATION_002`。
- 主题 = `XiHan:Protocols:CloudEvents:Topics:<type>`，否则 `type` 本身（须满足主题文法），否则 400 `PROTOCOLS_VALIDATION_009`。
- `Inbound = Dispatch`（默认）：`MessageDispatcher` 在请求内以新作用域、新上下文经完整内核管道执行本实例里该主题的处理器（`MessageDelivery.Attempt = 1`）：竞争订阅与广播订阅（RFC-0009「订阅方式」）各一次投递，先竞争后广播；派发只发生在接收请求的实例，要让广播订阅到达每个实例用 `Inbound = Publish`；成功 202 无正文；处理器失败按错误 KIND 映射 HTTP 状态（RFC 9457 Problem Details），发送方据此重试；主题在本实例没有任何处理器（两种订阅方式都没有）→ 404 `PROTOCOLS_NOTFOUND_003`。
- `Inbound = Publish`：经 `IMessagePublisher` 发布，成功即 202；至少一次投递、重试与死信由代理负责（含发件箱）。
- batch：先把每个事件的类型解析成主题，任一事件没有映射整批 400、不处理任何事件；派发模式逐个处理、第一个失败即停止并作为响应，发布模式按主题分组（保持首次出现的主题顺序与组内顺序）各一次批量发布 `PublishManyAsync(topic, envelopes)`、第一个失败的结果作为响应；已处理的不回滚（至少一次语义，处理器按 `id` + `source` 去重）。
- 端点不带授权要求（已定，2026-11-04，ADR-0121）：扩展属性 `xihantenant`、`xihanprincipal` 是发送方给的，派发时不与请求的身份核对，能调端点的调用方就能选租户、写审计主体。这是这个协议的设计，框架不收紧。权限由宿主在端点上加，只开给可信的内部服务。

## 出站

- `CloudEventsWebhookPublisher.PublishAsync(topic, envelope)`：POST 到 `XiHan:Protocols:CloudEvents:Webhooks:<topic>`；`Outbound = Binary`（默认，`ce-*` 头 + 数据体，头值按规范百分号编码非可打印 ASCII）或 `Structured`（`application/cloudevents+json` 体）。
- `PublishManyAsync(topic, envelopes)`（批量优先）：一条沿用上面的模式；多条用 batch 内容模式（`application/cloudevents-batch+json`，结构化事件的数组、顺序同入参）一次 POST，接收方须支持该模式，一个响应对整批生效（逐项结果相同）。
- 没有配置 → `PROTOCOLS_NOTFOUND_004`；响应非 2xx → `PROTOCOLS_UNAVAILABLE_001`（`status` 在详情里）；连接失败 → `PROTOCOLS_UNAVAILABLE_002`。重试由调用方决定（经发件箱投递即获得至少一次语义）。
- 使用命名 HttpClient `xihan.protocols.cloudevents`。
- 过出站地址策略（已定，RFC-0008、ADR-0117）：地址是运维配的，但接收方是第三方，它的 3xx 能把请求引到内网，所以这个命名客户端的连接与重定向都检查。接收方在内网时要放行。被拒绝按连接失败报告 `PROTOCOLS_UNAVAILABLE_002`，`reason` 是 `OutboundAddressDeniedException` 的类型全名，不发起连接。整个替换主处理器后由替换后的处理器决定能连哪里。
- HTTP 层的追踪头缺省不传播（已定，ADR-0083）：这个命名客户端不写 W3C 追踪上下文头 `traceparent`、`tracestate`、`baggage`（也不写旧格式的 `Request-Id`、`Correlation-Context`）。启用追踪时 .NET 的 HttpClient 本会按当前 Activity 写这些头，与信封扩展无关，同样会把内部的追踪 ID 与 Baggage 推给第三方，与下一条同理。`XiHan:Protocols:CloudEvents:PropagateTraceContext = true`（缺省 `false`，只接受 `true` / `false`，否则装配时 `ArgumentException`）显式打开，按 .NET 的全局传播器（`DistributedContextPropagator.Current`）写。只作用于这个命名客户端的主处理器（`SocketsHttpHandler.ActivityHeadersPropagator`），不改进程全局的传播；出站请求自身的客户端跨度照常按追踪配置记录。应用整个替换这个客户端的主处理器时由替换后的处理器决定是否传播；只想调连接参数而保留缺省，用 `UseSocketsHttpHandler` 配置同一个处理器。
- 不外推内部标识（已定，ADR-0074）：信封按调用方给出的原样发出，Webhook 发布者不像内部发布者（RFC-0009「发布侧补齐」）那样从当前调用补扩展，`xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale` 与 `traceparent` 都不由框架加上。理由：Webhook 的接收方是第三方，租户 ID、主体 ID、关联 ID 是本系统的内部标识，推出去会暴露内部结构与身份信息，并让第三方依赖本该随时可变的内部 ID；追踪链要不要跨出组织边界也由调用方决定。要给接收方的扩展（包括 `traceparent`）由调用方显式写进信封，或以上一条的配置打开 HTTP 头的传播。
