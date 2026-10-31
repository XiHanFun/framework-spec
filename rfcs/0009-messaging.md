# RFC-0009 消息、CloudEvents 信封与发件箱

> 状态：已采纳（2026-10-23 修订：批量发布；2026-10-26 修订：流能力与 Kafka Provider；2026-11-01 修订：内存代理主题数上限、发布确认、发布侧补齐扩展、发件箱尝试上限与死信） | 日期：2026-10-21 | 对应 ADR：ADR-0019（流）、ADR-0064（发件箱死信） | 参考实现：`XiHan.Framework.Messaging.Abstractions`、`XiHan.Framework.Messaging.InMemory`、`XiHan.Framework.Messaging.RabbitMQ`、`XiHan.Framework.Messaging.Kafka` | 一致性：`XiHan.Framework.Messaging.Conformance`

## 摘要

消息是跨进程的调用：信封用 CloudEvents 1.0，投递经过与请求相同的内核管道，传输只负责至少一次投递、重试与死信。事务性发件箱把"写业务数据"与"发消息"放进同一个工作单元。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `messaging.queue@1` | 工作队列：发布、按主题消费、至少一次、重试、死信 | `Messaging.InMemory`（单进程） |
| `messaging.stream@1` | 有序分区流：在队列语义之上，按分区键有序、按消费组保存位点、可按时刻重放 | —（`Messaging.Kafka`） |

`Messaging.InMemory` 每个主题一条有界通道（`MaxQueuedMessages`，默认 10000），满时发布返回 `MESSAGING_UNAVAILABLE_002`；主题在首次发布或订阅时创建、不回收，主题数上限 `MaxTopics`（默认 1000），已达上限时发布到新主题逐项返回 `MESSAGING_UNAVAILABLE_003`（不创建主题），订阅的主题超出上限时启动失败。

## 信封

CloudEvents 1.0：`id`、`source`、`type` 必填；`time`、`subject`、`datacontenttype`（默认 `application/json`）、`data`。线上编码为结构化 JSON（`application/cloudevents+json`）：JSON 数据内联在 `data`，其他数据放 `data_base64`，扩展属性是顶层成员。

框架扩展属性：`xihantenant`（→ Baggage `xihan.tenant`）、`xihanprincipal`（→ `xihan.principal`）、`xihancorrelation`（→ `xihan.correlation`）、`xihanlocale`（→ `xihan.locale`）、`traceparent`（→ 追踪 ID 与父跨度）、`xihankey`（分区键，见「流」）。扩展属性名 `^[a-z0-9]{1,20}$`。前四个与 Baggage 的对应以 `conventions/baggage-keys.md` 的 CloudEvents 载体为准（`EnvelopeExtensions.BaggageMappings`）；幂等键 `xihan.idempotency-key` 不进入消息（投递第 4 条）。

发布侧补齐：`DirectPublisher` 与 `OutboxPublisher` 在调用内发布时（`IInvocationAccessor` 有当前上下文），给每个信封补上缺失的扩展：

| 扩展 | 取值 |
| --- | --- |
| `traceparent` | `00-<TraceId>-<SpanId>-01`，取当前调用的追踪 ID 与跨度 ID |
| `xihantenant` | 安全上下文的租户；没有再取 Baggage `xihan.tenant` |
| `xihanprincipal` | 已认证主体的 ID；匿名时取 Baggage `xihan.principal` |
| `xihancorrelation` | Baggage `xihan.correlation` |
| `xihanlocale` | Baggage `xihan.locale` |

- 信封上已有的扩展保持不变：调用方显式设置的值优先。
- 不在调用内（后台循环、调用之外的作用域）不补。
- 代理（`IMessageBroker`）与发件箱投递循环不补：发件箱里的信封在写入时已经补齐，投递时原样发出。

派发侧按同一张对应表把扩展放进新调用的 Baggage，`traceparent` 合法时成为新调用的追踪 ID 与父跨度，不合法视为缺失。

主题文法 `^[a-z0-9][a-z0-9._-]{0,127}$`；死信主题为主题加 `DeadLetterSuffix`（默认 `.dlq`）。

信封在 HTTP 上的 CloudEvents 协议绑定（binary / structured / batch 入站，binary / structured 出站 Webhook）由 `Protocols.CloudEvents` 提供，见 `protocol/cloudevents.md`。

## 投递

1. 应用用 `AddMessageHandler<THandler>(topic)` 订阅；处理器是作用域服务。
2. 代理为每个订阅主题消费，把投递交给派发器；派发器为每条消息开新作用域、构造调用上下文（Baggage 与追踪来自扩展属性）、经完整管道依次执行该主题的处理器，第一个失败即调用结果。
3. 处理器返回失败或抛异常都算投递失败。代理重试到 `MaxAttempts`（默认 3，含首次），之后进入死信。重试的顺序由代理决定（内存实现排到队尾）。
4. 派发不写幂等键：重试必须重新执行处理器；去重由处理器按 `id` + `source` 自理，或在 P3 由操作契约提供消息级去重。
5. 代理在应用进入服务状态之前不派发。
6. 发布是批量优先的（`conventions/batch-first.md`）：`IMessagePublisher` 与 `IMessageBroker` 提供 `PublishManyAsync(topic, envelopes)`，逐项结果与入参顺序一一对应，单条是它的特例。主题不合法时每一项都是 `MESSAGING_VALIDATION_001`；同一主题内保持入参顺序；重复 `id` 照发，去重是消费端的事。Provider 用代理的原生批量（RabbitMQ 在同一通道上连续发布并一次等待确认），内存实现一次入队。空集合不触代理。
7. 发布成功表示代理已收下：远程代理的发布等到代理的确认（RabbitMQ 的发布确认、Kafka 的投递报告）才返回成功，没有得到确认的发布不报告成功。重试与死信的重新发布同样等确认，确认了才了结原消息（确认或提交位点），否则原消息留给代理再投递，不丢消息。

`Messaging.RabbitMQ`：主题映射为持久队列 `<QueuePrefix><topic>`（默认 `xihan.`）；发布通道开启发布确认并由客户端跟踪，消息以 `mandatory` 发出，`basic.nack` 与 `basic.return` 都按未确认处理，逐项返回 `MESSAGING_UNAVAILABLE_001`（批量也逐项），连接或通道异常整批抛出；失败的投递重新发布到队尾（尝试次数在消息头 `x-xihan-attempt`）或死信队列 `<queue><DeadLetterSuffix>`，重新发布确认后才 `basic.ack` 原消息，未确认时 `basic.nack` 并重新入队。

## 流

`messaging.stream@1` 的 Provider 同时提供 `messaging.queue@1`，在队列语义之上保证：

1. 分区键：信封扩展属性 `xihankey` 决定分区，同一主题内同键的消息按发布顺序投递；没有键的消息由 Provider 分配分区，只在主题只有一个分区时全局有序。
2. 消费位点：按消费组保存，投递完成（成功、重试耗尽进死信、无法解码进死信）后提交；重启续接，仍是至少一次。重试在原位进行（不重新入队），保持分区内顺序。
3. 重放：`IMessageReplayer.ReplayAsync(topic, since)` 用独立的消费位点把该时刻起的消息重新投递给当前订阅的处理器，经完整管道、按同样的重试与死信规则；不影响正常消费的位点；主题不存在或该时刻之后没有消息时成功返回。重放是有意的重复投递，去重仍由处理器自理。

`Messaging.Kafka`：主题映射为 `<TopicPrefix><topic>`（默认 `xihan.`），消费组 `GroupId`（默认 `xihan`），每个订阅主题一个消费者；生产者幂等且 `acks=all`，批量发布用客户端的原生批量后一次等待全部投递报告；无法解码或重试耗尽的消息发布到 `<topic><DeadLetterSuffix>`；`Properties:<key>` 透传 librdkafka 配置（`security.protocol`、`sasl.*`、`ssl.*`）。连接不上在启动时失败。

## 领域事件

聚合（`IAggregateRoot<TKey>`，RFC-0005）在工作单元内记录的领域事件由 Messaging 子系统派发：

1. 应用登记事件类型：`services.AddDomainEvent<OrderPlaced>(topic: "orders", type: "orders.placed", jsonContext.OrderPlaced, subject: e => $"order/{e.OrderId:N}")`——主题（主题文法）、CloudEvents `type`、源生成的序列化元数据（不经反射）与可选的 `subject` 选择器进 `DomainEventRegistry`（容量 10000）；首次登记同时注册 `IDomainEventDispatcher` 的实现 `DomainEventDispatcher`（作用域，用当前作用域的 `IMessagePublisher`）。同一事件类型重复登记抛 `FrameworkException`（`MESSAGING_VALIDATION_004`）。
2. 派发时机由 Transaction 阶段决定（RFC-0005）：处理器成功后、提交前，把工作单元登记的聚合的事件按登记映射成信封：`id` 为新 UUID v7（`N` 格式）、`source` 为 `/{topic}`、`type` 为登记的类型、`subject` 由选择器给出（没有则省略）、`time` 为 `IDomainEvent.OccurredAt`、`data` 为事件对象的 JSON；当前工作单元有租户时带 `xihantenant` 扩展；其余扩展由发布者按「信封」一节从当前调用补齐（追踪链、关联 ID 不断开）。
3. 同一主题的事件一次 `PublishManyAsync`（批量优先），主题按首次出现顺序；任一项失败即派发失败（返回第一个失败），调用回滚。全部成功后清空每个聚合的缓冲。
4. 装配了发件箱时 `IMessagePublisher` 是 `OutboxPublisher`：事件随业务写入同一事务落进发件箱表；没有发件箱时直接发布到代理。
5. 事件类型未登记 → `MESSAGING_VALIDATION_003`（不静默跳过）。

## 发件箱

`OutboxFeature` 需要 `messaging.queue@1` 与 `persistence.relational@1`：

- 把 `IMessagePublisher` 换成 `OutboxPublisher`：有活动工作单元时写入 `OutboxMessage` 表（随业务事务提交或回滚；批量发布用仓储的 `AddRangeAsync` 一次写入），没有时直接发布。
- 后台循环按 `PollInterval`（默认 1 秒）每轮一个工作单元：取到期的消息（未投递、未进死信、`NextAttemptAt` 为空或已到；按创建时间，最多 `BatchSize`，默认 100），按主题批量发布，每条计一次尝试：
  - 成功记投递时间 `DispatchedAt`。
  - 失败记原因 `LastError`，`NextAttemptAt` 推后 `RetryDelay`（默认 5 秒）乘以 2 的（尝试次数减一）次方，不超过 `MaxRetryDelay`（默认 5 分钟）。
  - 尝试达到 `MaxAttempts`（默认 10），或载荷无法解码（`MESSAGING_VALIDATION_002`，重试也解不开），记 `DeadLetteredAt` 进死信：不再取到，行保留供排查；把 `DeadLetteredAt` 置空、`NextAttemptAt` 设为现在即重新投递。
  - 退避中与死信的消息不占批次，永久失败的消息不挡后面的消息。
- 代理整批抛出（不可达）时本轮回滚，不计尝试：代理故障不消耗尝试次数，恢复后照常投递。
- 投递成功的行保留 `DispatchedRetention`（默认 1 天）后由循环删除，每轮最多 `BatchSize` 行，与投递同一工作单元；为 0 时投递成功即在同一工作单元删除。死信行不自动删除。
- 表结构：`Topic` 长 128（主题文法上限），`LastError` 为长文本，`NextAttemptAt` 与 `DeadLetteredAt` 可空（加在已有数据的表上时旧行为空，视为到期、未进死信）。
- 投递是至少一次：循环提交前崩溃会重复投递。

## 一致性

Provider 必须继承 `MessageBrokerConformance` 并通过全部用例，含批量发布：一次发布多条保持顺序、逐项结果、空集合、主题不合法逐项失败。提供 `messaging.stream@1` 的 Provider 还要继承 `MessageStreamConformance`（同键有序、按时刻重放）。

## 错误码

见 `conventions/error-codes.md` 中 `MESSAGING_*`。
