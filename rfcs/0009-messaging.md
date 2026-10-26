# RFC-0009 消息、CloudEvents 信封与发件箱

> 状态：已采纳（2026-10-23 修订：批量发布；2026-10-26 修订：流能力与 Kafka Provider） | 日期：2026-10-21 | 对应 ADR：ADR-0019（流） | 参考实现：`XiHan.Framework.Messaging.Abstractions`、`XiHan.Framework.Messaging.InMemory`、`XiHan.Framework.Messaging.RabbitMQ`、`XiHan.Framework.Messaging.Kafka` | 一致性：`XiHan.Framework.Messaging.Conformance`

## 摘要

消息是跨进程的调用：信封用 CloudEvents 1.0，投递经过与请求相同的内核管道，传输只负责至少一次投递、重试与死信。事务性发件箱把"写业务数据"与"发消息"放进同一个工作单元。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `messaging.queue@1` | 工作队列：发布、按主题消费、至少一次、重试、死信 | `Messaging.InMemory`（单进程） |
| `messaging.stream@1` | 有序分区流：在队列语义之上，按分区键有序、按消费组保存位点、可按时刻重放 | —（`Messaging.Kafka`） |

## 信封

CloudEvents 1.0：`id`、`source`、`type` 必填；`time`、`subject`、`datacontenttype`（默认 `application/json`）、`data`。线上编码为结构化 JSON（`application/cloudevents+json`）：JSON 数据内联在 `data`，其他数据放 `data_base64`，扩展属性是顶层成员。

框架扩展属性：`xihantenant`（→ Baggage `xihan.tenant`）、`xihancorrelation`（→ `xihan.correlation`）、`traceparent`（→ 追踪 ID 与父跨度）、`xihankey`（分区键，见「流」）。扩展属性名 `^[a-z0-9]{1,20}$`。

主题文法 `^[a-z0-9][a-z0-9._-]{0,127}$`；死信主题为主题加 `DeadLetterSuffix`（默认 `.dlq`）。

信封在 HTTP 上的 CloudEvents 协议绑定（binary / structured / batch 入站，binary / structured 出站 Webhook）由 `Protocols.CloudEvents` 提供，见 `protocol/cloudevents.md`。

## 投递

1. 应用用 `AddMessageHandler<THandler>(topic)` 订阅；处理器是作用域服务。
2. 代理为每个订阅主题消费，把投递交给派发器；派发器为每条消息开新作用域、构造调用上下文（Baggage 与追踪来自扩展属性）、经完整管道依次执行该主题的处理器，第一个失败即调用结果。
3. 处理器返回失败或抛异常都算投递失败。代理重试到 `MaxAttempts`（默认 3，含首次），之后进入死信。重试的顺序由代理决定（内存实现排到队尾）。
4. 派发不写幂等键：重试必须重新执行处理器；去重由处理器按 `id` + `source` 自理，或在 P3 由操作契约提供消息级去重。
5. 代理在应用进入服务状态之前不派发。
6. 发布是批量优先的（`conventions/batch-first.md`）：`IMessagePublisher` 与 `IMessageBroker` 提供 `PublishManyAsync(topic, envelopes)`，逐项结果与入参顺序一一对应，单条是它的特例。主题不合法时每一项都是 `MESSAGING_VALIDATION_001`；同一主题内保持入参顺序；重复 `id` 照发，去重是消费端的事。Provider 用代理的原生批量（RabbitMQ 在同一通道上连续发布并一次等待确认），内存实现一次入队。空集合不触代理。

## 流

`messaging.stream@1` 的 Provider 同时提供 `messaging.queue@1`，在队列语义之上保证：

1. 分区键：信封扩展属性 `xihankey` 决定分区，同一主题内同键的消息按发布顺序投递；没有键的消息由 Provider 分配分区，只在主题只有一个分区时全局有序。
2. 消费位点：按消费组保存，投递完成（成功、重试耗尽进死信、无法解码进死信）后提交；重启续接，仍是至少一次。重试在原位进行（不重新入队），保持分区内顺序。
3. 重放：`IMessageReplayer.ReplayAsync(topic, since)` 用独立的消费位点把该时刻起的消息重新投递给当前订阅的处理器，经完整管道、按同样的重试与死信规则；不影响正常消费的位点；主题不存在或该时刻之后没有消息时成功返回。重放是有意的重复投递，去重仍由处理器自理。

`Messaging.Kafka`：主题映射为 `<TopicPrefix><topic>`（默认 `xihan.`），消费组 `GroupId`（默认 `xihan`），每个订阅主题一个消费者；生产者幂等且 `acks=all`，批量发布用客户端的原生批量后一次等待全部投递报告；无法解码或重试耗尽的消息发布到 `<topic><DeadLetterSuffix>`；`Properties:<key>` 透传 librdkafka 配置（`security.protocol`、`sasl.*`、`ssl.*`）。连接不上在启动时失败。

## 领域事件

聚合（`IAggregateRoot<TKey>`，RFC-0005）在工作单元内记录的领域事件由 Messaging 子系统派发：

1. 应用登记事件类型：`services.AddDomainEvent<OrderPlaced>(topic: "orders", type: "orders.placed", jsonContext.OrderPlaced, subject: e => $"order/{e.OrderId:N}")`——主题（主题文法）、CloudEvents `type`、源生成的序列化元数据（不经反射）与可选的 `subject` 选择器进 `DomainEventRegistry`（容量 10000）；首次登记同时注册 `IDomainEventDispatcher` 的实现 `DomainEventDispatcher`（作用域，用当前作用域的 `IMessagePublisher`）。同一事件类型重复登记抛 `FrameworkException`（`MESSAGING_VALIDATION_004`）。
2. 派发时机由 Transaction 阶段决定（RFC-0005）：处理器成功后、提交前，把工作单元登记的聚合的事件按登记映射成信封：`id` 为新 UUID v7（`N` 格式）、`source` 为 `/{topic}`、`type` 为登记的类型、`subject` 由选择器给出（没有则省略）、`time` 为 `IDomainEvent.OccurredAt`、`data` 为事件对象的 JSON；当前工作单元有租户时带 `xihantenant` 扩展。
3. 同一主题的事件一次 `PublishManyAsync`（批量优先），主题按首次出现顺序；任一项失败即派发失败（返回第一个失败），调用回滚。全部成功后清空每个聚合的缓冲。
4. 装配了发件箱时 `IMessagePublisher` 是 `OutboxPublisher`：事件随业务写入同一事务落进发件箱表；没有发件箱时直接发布到代理。
5. 事件类型未登记 → `MESSAGING_VALIDATION_003`（不静默跳过）。

## 发件箱

`OutboxFeature` 需要 `messaging.queue@1` 与 `persistence.relational@1`：

- 把 `IMessagePublisher` 换成 `OutboxPublisher`：有活动工作单元时写入 `OutboxMessage` 表（随业务事务提交或回滚；批量发布用仓储的 `AddRangeAsync` 一次写入），没有时直接发布。
- 后台循环按 `PollInterval`（默认 1 秒）每轮一个工作单元：取未投递的消息（按创建时间，最多 `BatchSize`），发布，成功记投递时间，失败记次数与原因。投递是至少一次：循环提交前崩溃会重复投递。

## 一致性

Provider 必须继承 `MessageBrokerConformance` 并通过全部用例，含批量发布：一次发布多条保持顺序、逐项结果、空集合、主题不合法逐项失败。提供 `messaging.stream@1` 的 Provider 还要继承 `MessageStreamConformance`（同键有序、按时刻重放）。

## 错误码

见 `conventions/error-codes.md` 中 `MESSAGING_*`。
