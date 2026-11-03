# RFC-0009 消息、CloudEvents 信封与发件箱

> 状态：已采纳（2026-10-23 修订：批量发布；2026-10-26 修订：流能力与 Kafka Provider；2026-11-01 修订：内存代理主题数上限、发布确认、发布侧补齐扩展、发件箱尝试上限与死信、流的死信发布失败回退重投；同日再修订：内存代理丢弃没人订阅的主题的消息、RabbitMQ 的队列只由订阅方声明并丢弃没有订阅者的消息、订阅方式与广播订阅；同日再修订：删除内存代理的主题数上限；同日再修订：实例标识可配置、Kafka 主题不存在维持失败并要求预建主题；2026-11-03 修订：发件箱把代理抛出与返回失败一样计入尝试、重试逐条发布、一轮遇到抛出即停止发布） | 日期：2026-10-21 | 对应 ADR：ADR-0019（流）、ADR-0064（发件箱死信）、ADR-0074（内存代理丢弃无人订阅的消息）、ADR-0079（RabbitMQ 队列只由订阅方声明、广播订阅）、ADR-0082（删除主题数上限）、ADR-0085（实例标识可配置、Kafka 预建主题）、ADR-0108（发件箱计入代理抛出） | 参考实现：`XiHan.Framework.Messaging.Abstractions`、`XiHan.Framework.Messaging.InMemory`、`XiHan.Framework.Messaging.RabbitMQ`、`XiHan.Framework.Messaging.Kafka` | 一致性：`XiHan.Framework.Messaging.Conformance`

## 摘要

消息是跨进程的调用：信封用 CloudEvents 1.0，投递经过与请求相同的内核管道，传输只负责至少一次投递、重试与死信。事务性发件箱把"写业务数据"与"发消息"放进同一个工作单元。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `messaging.queue@1` | 工作队列：发布、按主题消费、至少一次、重试、死信 | `Messaging.InMemory`（单进程） |
| `messaging.stream@1` | 有序分区流：在队列语义之上，按分区键有序、按消费组保存位点、可按时刻重放 | —（`Messaging.Kafka`） |

`Messaging.InMemory` 每个被订阅的主题（本进程登记了处理器，任一订阅方式）一条有界通道（`MaxQueuedMessages`，默认 10000），满时发布返回 `MESSAGING_UNAVAILABLE_002`；通道在首次发布或消费时创建、不回收；通道数不超过订阅表里的主题数，订阅表在装配时定好（代码与插件登记的处理器）、之后只读，不随发布增长，所以不另设主题数上限（2026-11-01 修订，ADR-0082：原来的 `MaxTopics` 选项删除，只为它登记的 `MESSAGING_UNAVAILABLE_003` 撤销登记、序号不复用）。发布到没人订阅的主题：消息丢弃，发布照常成功（批量逐项成功），不缓冲、不建通道，代理记一条 Debug 级别的日志（ADR-0074）——内存代理没有跨进程的消费者，缓冲没人消费的消息只会占着内存直到通道排满；调度、工作流等框架事件照常发布到应用不一定订阅的主题，这是常态，所以不按 Warning 记。需要先发后订的留存语义用 RabbitMQ（订阅方第一次启动建好持久队列之后，它离线期间的消息留在队列里）或 Kafka（按位点重放）。

## 信封

CloudEvents 1.0：`id`、`source`、`type` 必填；`time`、`subject`、`datacontenttype`（默认 `application/json`）、`data`。线上编码为结构化 JSON（`application/cloudevents+json`）：JSON 数据内联在 `data`，其他数据放 `data_base64`，扩展属性是顶层成员。

框架扩展属性：`xihantenant`（→ Baggage `xihan.tenant`）、`xihanprincipal`（→ `xihan.principal`）、`xihancorrelation`（→ `xihan.correlation`）、`xihanlocale`（→ `xihan.locale`）、`traceparent`（→ 追踪 ID、父跨度与采样标志）、`xihankey`（分区键，见「流」）。扩展属性名 `^[a-z0-9]{1,20}$`。前四个与 Baggage 的对应以 `conventions/baggage-keys.md` 的 CloudEvents 载体为准（`EnvelopeExtensions.BaggageMappings`）；幂等键 `xihan.idempotency-key` 不进入消息（投递第 4 条）。

发布侧补齐：`DirectPublisher` 与 `OutboxPublisher` 在调用内发布时（`IInvocationAccessor` 有当前上下文），给每个信封补上缺失的扩展：

| 扩展 | 取值 |
| --- | --- |
| `traceparent` | `00-<TraceId>-<父跨度>-<标志>`：当前 Activity 与调用同一 trace 且有采样决定（Observe 阶段的 `xihan.invoke` 或它的子 Span）时，父跨度与标志取它的；否则取调用的 `SpanId` 与 `TraceFlags`，上游「不采样」写 `00`，没有标志（链路的根）按已采样写 `01`（RFC-0003 规则 7） |
| `xihantenant` | 安全上下文的租户；没有再取 Baggage `xihan.tenant` |
| `xihanprincipal` | 已认证主体的 ID；匿名时取 Baggage `xihan.principal` |
| `xihancorrelation` | Baggage `xihan.correlation` |
| `xihanlocale` | Baggage `xihan.locale` |

- 信封上已有的扩展保持不变：调用方显式设置的值优先。
- 不在调用内（后台循环、调用之外的作用域）不补。
- 代理（`IMessageBroker`）与发件箱投递循环不补：发件箱里的信封在写入时已经补齐，投递时原样发出。
- 推给外部系统的 CloudEvents Webhook 发布者不补，也不外推任何内部标识（`protocol/cloudevents.md`「出站」，ADR-0074）：租户、主体、关联等是本系统的内部标识，不推给第三方。

派发侧按同一张对应表把扩展放进新调用的 Baggage，`traceparent` 合法时成为新调用的追踪 ID、父跨度与采样标志（与 HTTP 入站一致，上游「不采样」原样保留），不合法视为缺失。

主题文法 `^[a-z0-9][a-z0-9._-]{0,127}$`；死信主题为主题加 `DeadLetterSuffix`（默认 `.dlq`）。

信封在 HTTP 上的 CloudEvents 协议绑定（binary / structured / batch 入站，binary / structured 出站 Webhook）由 `Protocols.CloudEvents` 提供，见 `protocol/cloudevents.md`。

## 投递

1. 应用用 `AddMessageHandler<THandler>(topic, mode)` 订阅，`mode` 缺省为竞争消费（见「订阅方式」）；处理器是作用域服务。
2. 代理为每个订阅（主题加订阅方式）消费，把投递交给派发器；投递带订阅方式（`MessageDelivery.Mode`）。派发器为每条消息开新作用域、构造调用上下文（Baggage 与追踪来自扩展属性）、经完整管道依次执行该主题、该方式的处理器，第一个失败即调用结果。
3. 处理器返回失败或抛异常都算投递失败。代理重试到 `MaxAttempts`（默认 3，含首次），之后竞争订阅进入死信，广播订阅丢弃并记日志。重试的顺序由代理决定（内存实现排到队尾）。
4. 派发不写幂等键：重试必须重新执行处理器；去重由处理器按 `id` + `source` 自理，或在 P3 由操作契约提供消息级去重。
5. 代理在应用进入服务状态之前不派发。
6. 发布是批量优先的（`conventions/batch-first.md`）：`IMessagePublisher` 与 `IMessageBroker` 提供 `PublishManyAsync(topic, envelopes)`，逐项结果与入参顺序一一对应，单条是它的特例。主题不合法时每一项都是 `MESSAGING_VALIDATION_001`；同一主题内保持入参顺序；重复 `id` 照发，去重是消费端的事。Provider 用代理的原生批量（RabbitMQ 在同一通道上连续发布并一次等待确认），内存实现一次入队。空集合不触代理。
7. 发布成功表示代理已收下：远程代理的发布等到代理的确认（RabbitMQ 的发布确认、Kafka 的投递报告）才返回成功，没有得到确认的发布不报告成功。重试与死信的重新发布同样等确认，确认了才了结原消息（确认或提交位点），否则原消息留给代理再投递，不丢消息。

`Messaging.RabbitMQ`（ADR-0079）：

- 发布方只声明交换机：消息发到持久的 direct 交换机 `Exchange`（默认 `xihan`），路由键 `<QueuePrefix><topic>`（前缀默认 `xihan.`，不同前缀的应用共用一个交换机也互不串扰）。发布方不声明队列。
- 队列只由订阅方声明：订阅方启动时声明持久队列 `<QueuePrefix><topic>` 并以同名路由键绑定到交换机。订阅方第一次启动之前发布的消息没有队列可进；之后订阅方离线期间的消息留在队列里。
- 发布通道开启发布确认并由客户端跟踪，消息以 `mandatory` 发出。`basic.nack` 按未确认处理，逐项返回 `MESSAGING_UNAVAILABLE_001`（批量也逐项）；连接或通道异常整批抛出。
- `basic.return` 说明路由键上没有任何队列绑定，即没有订阅者：与内存代理一致，消息丢弃，发布照常成功（批量逐项成功），代理记一条 Debug 级别的日志。它不是发布失败，发件箱不为它重试、不把它记进死信——调度、工作流等框架事件照常发到应用不一定订阅的主题。
- 失败的投递经默认交换机定向重新发布：未到上限回到所消费队列的队尾（尝试次数在消息头 `x-xihan-attempt`），到了上限进死信队列 `<queue><DeadLetterSuffix>`。死信队列由死信的一方（订阅方）声明为持久队列，并以同名路由键绑定到交换机，即主题加后缀的竞争队列。定向重新发布被退回说明目标队列已不存在，按未确认处理。重新发布确认后才 `basic.ack` 原消息，未确认时 `basic.nack` 并重新入队。
- 先前版本由发布方声明、没有人订阅的队列（例如 `scheduling.job.*` 对应的队列）不再收到新消息；框架不删除任何队列，已经堆积的队列由运维删除。
- 以上队列都属于竞争订阅；广播订阅的队列见「订阅方式」。

## 订阅方式

订阅登记带订阅方式（`SubscriptionMode`，ADR-0079）：

| 方式 | 谁收到 | 用途 |
| --- | --- | --- |
| `Competing`（缺省） | 订阅了这个主题的实例里只有一个收到（工作队列） | 业务事件、领域事件、需要恰好处理一次的工作 |
| `Broadcast` | 每个在线实例各收到一份 | 断开本实例上的连接、清本地缓存这类每个节点都要做的事 |

1. 登记：`AddMessageHandler<THandler>(topic, SubscriptionMode.Broadcast)`；不写方式即竞争消费。同一主题可以同时有两种方式的处理器：订阅表按（主题，方式）分组，代理按（主题，方式）各消费一份，派发器只执行与投递的方式相同的处理器。同一条消息到达一个实例的竞争订阅与广播订阅是两次独立的投递，各自计尝试次数、各自重试。插件在握手的 `Subscription.mode` 里声明（`plugin/invocation.md`）。
2. 广播是尽力而为：只有订阅时在线、与代理连着的实例收到；实例不在线、重启中、与代理断开期间发布的消息收不到，上线后也不回放历史。需要不丢的用竞争订阅。
3. 广播的失败：在本实例内按 `MaxAttempts` 重试，到上限后丢弃并记 Warning 日志，不进死信——死信是竞争订阅的共享留存，每个实例各自的一份失败没有地方可以重放；无法解码的消息同样丢弃并记日志。
4. 广播不共享去重：每个实例各自执行处理器，派发照旧不写幂等键；代理重投时同一实例可能收到重复的投递，处理器要能承受。
5. 流的重放（`IMessageReplayer`）只投给竞争订阅：广播订阅不回放历史。
6. 实例标识 `MessagingInstance.Id`：消息子系统登记的单例，每个宿主一个（ADR-0085）。
   - 来源：配置了 `XiHan:Messaging:InstanceId`（`MessagingOptions.InstanceId`）时取配置值；没有配置时在构造单例时生成，由机器名（小写，`[a-z0-9-]` 以外的字符换成 `-`，去掉开头的 `-`，最多 32 个字符，什么都不剩时取 `host`）、进程 ID 与 8 位随机十六进制以 `-` 连接，每次启动都不同。选项里只有配置值（没有配置时为空），生成的值不进选项：同样的配置读出同样的选项。
   - 文法 `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`：1 到 64 个字符，字母或数字开头，其余是字母、数字、`.`、`_`、`-`；不含 `@`（名字里实例标识前的分隔符）。合文法的值可直接用在 RabbitMQ 队列名与 Kafka 消费组名里；生成的标识也合这个文法。配置了但不合文法（空字符串、空白、其他字符、超长）在装配时抛出 `ArgumentException`，启动失败；不想配置就不写这个键。
   - 必须每个同时在线的实例各不相同。Kafka 上两个实例同一标识就是同一个广播消费组，组内分摊分区，广播静默退化成竞争（每条只有一个实例收到），不报错；RabbitMQ 上后启动的实例声明同名的独占队列被代理拒绝（`RESOURCE_LOCKED`），订阅失败、启动失败（`MESSAGING_UNAVAILABLE_001`）。适合配成部署平台保证唯一的值，如 Pod 名（Kubernetes 用下行 API 把 `metadata.name` 设进环境变量 `XiHan__Messaging__InstanceId`）或主机名加副本序号；多个副本共用的配置文件里不要写死。
   - 固定标识只换来在代理的管理界面里认得出实例，广播语义不变：Kafka 的广播消费组仍不提交位点、从最新位点开始，重启不回放；RabbitMQ 的广播队列仍随连接删除。代价是旧连接没有及时断开（节点失联、进程被冻结）时，用同一标识重启的实例要等代理发现旧连接已断：RabbitMQ 在旧连接的心跳超时、删除旧的独占队列之前声明失败、启动失败，Kafka 在旧成员的会话超时（`session.timeout.ms`）之前分不到分区、收不到广播。缺省的生成值每次都不同，没有这个问题。

Provider：

- `Messaging.InMemory`：单进程就是一个实例，竞争订阅与广播订阅都在本进程收到一次（各一次投递）；广播投递失败到上限后丢弃，不进死信列表。没有跨进程的语义。
- `Messaging.RabbitMQ`：广播订阅为每个实例声明独占、非持久、随连接删除的队列 `<QueuePrefix><topic>@<InstanceId>`（`@` 不在主题文法里，不会与竞争队列或死信队列重名），以主题的路由键绑定到交换机；与代理断开时队列随连接删除，客户端自动恢复连接时重新声明，断开期间的消息不进它。重试经默认交换机定向发回这个队列。
- `Messaging.Kafka`：广播订阅用本实例独占的消费组 `<GroupId>@<InstanceId>`（组内只有本实例，分到全部分区），没有已提交位点时从最新位点开始（`auto.offset.reset=latest`），不提交位点：消费组空了不留位点，重启时没有已提交的位点（生成的标识每次不同，是新的消费组；配置了固定标识时组名不变，但从未提交过位点），不回放历史。分区分配是异步的，启动后要等消费组分到分区（通常几秒）才开始收；组内再均衡同样从最新位点续上。

## 流

`messaging.stream@1` 的 Provider 同时提供 `messaging.queue@1`，在队列语义之上保证：

1. 分区键：信封扩展属性 `xihankey` 决定分区，同一主题内同键的消息按发布顺序投递；没有键的消息由 Provider 分配分区，只在主题只有一个分区时全局有序。
2. 消费位点：按消费组保存，投递完成（成功、重试耗尽进死信、无法解码进死信）后提交；重启续接，仍是至少一次。重试在原位进行（不重新入队），保持分区内顺序。死信发布没有得到确认（投递第 7 条）、或投递因代理不可达等异常没有完成时，投递未完成：不提交位点，消费者回退到这条消息，等 `RedeliveryDelay` 后重新投递（处理器重新执行），期间该分区不往后读；消费者本身出错（不是某条消息的错误）同样等一会儿再取。消费循环只在停止时退出，停止时未完成的消息不提交，重启后重新投递。重放中的死信发布没有得到确认时，重放返回 `MESSAGING_UNAVAILABLE_001`。
3. 重放：`IMessageReplayer.ReplayAsync(topic, since)` 用独立的消费位点把该时刻起的消息重新投递给当前竞争订阅的处理器（广播订阅不回放），经完整管道、按同样的重试与死信规则；不影响正常消费的位点；主题不存在或该时刻之后没有消息时成功返回。重放是有意的重复投递，去重仍由处理器自理。

`Messaging.Kafka`：主题映射为 `<TopicPrefix><topic>`（默认 `xihan.`），竞争订阅的消费组 `GroupId`（默认 `xihan`），每个订阅（主题加订阅方式）一个消费者；生产者幂等且 `acks=all`，批量发布用客户端的原生批量后一次等待全部投递报告；无法解码或重试耗尽的消息发布到 `<topic><DeadLetterSuffix>`，没有落盘或发布失败时按「流」第 2 条回退，`RedeliveryDelaySeconds`（默认 5）秒后重新投递；`Properties:<key>` 透传 librdkafka 配置（`security.protocol`、`sasl.*`、`ssl.*`）。连接不上在启动时失败。发布到不存在的主题时，主题由 broker 按 `auto.create.topics.enable`（Kafka 缺省开启）自动建出；没人订阅的主题里的消息只占到保留期（`retention.ms`，缺省 7 天）为止，没有消费组也就没有积压，不会像队列那样无限堆积（ADR-0079）。broker 关闭了自动建主题时，主题不存在是部署错误，框架不替它建，也不把它当作没有订阅者（ADR-0085）：
- 发布：客户端判定主题不存在后失败（`MESSAGING_UNAVAILABLE_001`，librdkafka 的 `topic.metadata.propagation.max.ms`，缺省 30 秒），批量发布整批抛出；直接发布的把异常抛给发布方，经发件箱的按发件箱的规则退避重试、到上限进死信（「发件箱」；2026-11-03 之前发件箱遇到抛出整轮回滚、不计尝试，这条消息与同批的消息会一直卡住，ADR-0108）。
- 订阅：消费者每次取消息都报主题不存在，记 Warning 日志后继续等，主题建好之后才开始收。
- 死信：死信主题不存在时死信发布失败，按「流」第 2 条回退、重新投递，这条消息所在的分区停在原地。
- 所以这类部署必须预先建好（名字都带 `TopicPrefix`）：应用发布与订阅的主题；被订阅主题的死信主题 `<topic><DeadLetterSuffix>`；框架自己发布的事件主题——装配了调度时 `scheduling.job.*` 与 `scheduling.run.*` 共 9 个（RFC-0013「事件」），装配了工作流时 `workflow.*`（RFC-0021「事件」），没人订阅的也要建。
- 不把「主题不存在」当作没有订阅者丢弃：Kafka 的主题是否存在与有没有订阅者无关，主题名写错、`TopicPrefix` 配错也会被吞成 Debug 日志，消息无声丢失。

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
- 后台循环按 `PollInterval`（默认 1 秒）每轮一个工作单元（只覆盖发件箱行的状态写回与保留期清理；代理不在事务里，发出去的收不回来）：取到期的消息（未投递、未进死信、`NextAttemptAt` 为空或已到；按创建时间，最多 `BatchSize`，默认 100，取 1 到 `EntityQuery.DefaultMaxTake`（1000）：每轮按它分页查发件箱表，超出在解析配置时拒绝），按下面的发布单元发布，发布过的每条计一次尝试：
  - 成功记投递时间 `DispatchedAt`。
  - 失败记原因 `LastError`，`NextAttemptAt` 推后 `RetryDelay`（默认 5 秒）乘以 2 的（尝试次数减一）次方，不超过 `MaxRetryDelay`（默认 5 分钟）。
  - 尝试达到 `MaxAttempts`（默认 10），或载荷无法解码（`MESSAGING_VALIDATION_002`，重试也解不开），记 `DeadLetteredAt` 进死信：不再取到，行保留供排查；把 `DeadLetteredAt` 置空、`NextAttemptAt` 设为现在即重新投递。
  - 退避中与死信的消息不占批次，永久失败的消息不挡后面的消息。
- 发布单元：首次投递的消息按主题合成一次 `PublishManyAsync`（主题按首次出现的顺序，组内保持创建顺序）；重试的消息（`Attempts` 不为 0）每条单独一次 `PublishManyAsync`。代理整批抛出时分不出是哪一条，上一次失败时它可能就是让整批失败的那条，单独发布才不会让同批的别的消息跟着它一起退避、一起进死信。
- 代理抛出与返回失败同样计入（2026-11-03 修订，ADR-0108；此前整轮回滚、不计尝试，某条消息让代理一直抛出时它永远进不了死信，同批与排在后面的消息也一直发不出去）：
  - 抛出的那次调用里的每条记一次失败的尝试，按上面的规则退避或进死信。`LastError` 与代理返回的失败同形，是 `MESSAGING_UNAVAILABLE_001`，原因 `reason` 取框架异常（`FrameworkException`，代理契约里的基础设施失败）的消息；别的异常的消息可能带连接串、主机名等，不进发件箱行，原因只写异常类型名。
  - 每次抛出的调用记一条 Warning 日志（带异常，不带载荷）；每条消息的失败与进死信照旧各记一条。
  - 本轮随即停止发布：代理可能整体不可用，余下的消息不计尝试、不写回，留到下一轮；本轮已有的结果照常提交。一轮至多一次抛出的调用，代理不可达时不会把整批逐条试一遍，一轮的时长以一次调用的超时为界。
  - 不是调用方取消引起的 `OperationCanceledException`（代理客户端自己的超时）也是一次失败。
- 调用方取消（宿主停止：`OperationCanceledException` 且循环的令牌已取消）不是失败：本轮回滚，不计尝试、不进死信，本轮已发出的下一轮重发。存储失败（读不出、写不回、提交失败）同样回滚本轮，循环记日志后继续。
- 代理长时间不可达时，积压的消息按同样的规则累计尝试；一轮只有一次调用抛出，消息累计尝试比逐条失败更慢。缺省参数下一条消息连续失败约 20 分钟才进死信，需要更长的容忍度时调大 `MaxAttempts` 或 `MaxRetryDelay`；进了死信的按上面的办法重投。
- 投递成功的行保留 `DispatchedRetention`（默认 1 天）后由循环删除，每轮最多 `BatchSize` 行，与投递同一工作单元；为 0 时投递成功即在同一工作单元删除。死信行不自动删除。
- 表结构：`Topic` 长 128（主题文法上限），`LastError` 为长文本，`NextAttemptAt` 与 `DeadLetteredAt` 可空（加在已有数据的表上时旧行为空，视为到期、未进死信）。
- 投递是至少一次：循环提交前崩溃会重复投递。

## 一致性

Provider 必须继承 `MessageBrokerConformance` 并通过全部用例，含批量发布：一次发布多条保持顺序、逐项结果、空集合、主题不合法逐项失败；发布到没人订阅的主题（单条与批量）成功；同一主题的竞争订阅与广播订阅各收到一次；两个实例连同一个代理时广播每个实例都收到、竞争只有一个实例收到（跨宿主共享代理的 Provider，内存代理以 `SupportsMultipleInstances = false` 跳过）。提供 `messaging.stream@1` 的 Provider 还要继承 `MessageStreamConformance`（同键有序、按时刻重放）。

发件箱的存储经仓储，内存持久化与数据库 Provider 都要通过 `OutboxConformance`（Preview，`CreateFeature()` 返回持久化特性；2026-11-03，ADR-0108）：代理对某条消息一直抛出时按尝试上限、退避进死信，同批的其他消息照常发出；返回失败与抛出逐轮状态相同；调用方取消回滚本轮、不计尝试；不是调用方取消的 `OperationCanceledException` 计为失败。

## 错误码

见 `conventions/error-codes.md` 中 `MESSAGING_*`。
