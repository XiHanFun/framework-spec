# RFC-0021 工作流：持久化执行

> 状态：已采纳 | 日期：2026-10-26，同日修订（ADR-0020：`Workflow.Database`、触发式启动、事件网关、外部任务、HTTP 活动、补偿活动、跳转与迁移、认领 / 退回 / 催办、提醒与审批阈值、保留期清理；2026-11-01 修订：外部任务领取的条数上限与下推、重用策略查重与创建原子、定义按租户隔离、实例按租户隔离；2026-11-01 修订：引擎写入随调用方事务、分区隔离与随租户分库（平台与租户互不可见，取消全局定义），待办查询的过滤、排序与分页下推到存储并带总数；2026-11-01 修订：只读调用里的写入是 500 类缺陷；2026-11-01 修订：实例查询与清理的分页越界返回校验错误；2026-11-01 修订：租约表可以指定命名连接，SQLite 上放到另一个库文件） | 对应 ADR：ADR-0016、ADR-0020、ADR-0066、ADR-0077、ADR-0081、ADR-0082、ADR-0084 | 参考实现：`XiHan.Framework.Workflow.Abstractions`、`XiHan.Framework.Workflow.InMemory`、`XiHan.Framework.Workflow.Database` | 一致性：`XiHan.Framework.Workflow.Conformance`

## 摘要

工作流子系统把"一张有向图上的长时间执行"做成与请求同构的调用：定义描述节点、转移与变量；实例是定义的一次执行，状态全部落在存储里；每个节点的活动在新作用域、新上下文里经完整内核管道执行；等待（人工任务、外部任务、定时、信号、子流程、重试、超时）以书签挂起，书签被消费时从存储恢复执行。引擎与协议性的规则在抽象包里，Provider 只实现三个存储、实例锁与到期书签的轮询驱动。能力 `workflow.execution@1`。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `workflow.execution@1` | 定义目录、实例执行、书签、用户任务、外部任务；`IWorkflowEngine`、`IWorkflowDefinitionCatalog`、`IWorkflowTaskService`、`IWorkflowExternalTaskService` | `Workflow.InMemory`（有界内存存储、进程内实例锁、单进程轮询） |

Provider 实现 `IWorkflowDefinitionStore`、`IWorkflowInstanceStore`、`IWorkflowBookmarkStore`（作用域服务，按分区读写）、`IWorkflowTransaction`（作用域服务，与调用方事务的关系）与 `IWorkflowInstanceLock`，并在应用进入服务状态后驱动 `DueBookmarkPoller`。`Workflow.Database`（依赖 `persistence.relational@1`，可选依赖 `multitenancy.context@1`）在调用方的工作单元里读写、租户的记录随租户分库，用平台库里的租约行做分布式实例锁，多实例同时只有一个执行者；到期书签由持有轮询租约的节点逐个分区轮询（其他节点同时轮询也只是无效的重复尝试）。

## 名字与值

- 定义名 `WorkflowName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同；一个定义名下多个版本；名字与版本号在分区里唯一，见「分区」）。
- 节点 ID、转移 ID：`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`，定义内唯一。
- 活动类型 `ActivityType`：`^[A-Za-z][A-Za-z0-9._-]{0,63}$`；内置活动用 PascalCase 名（`Start`、`UserTask`）。
- 实例、节点实例、书签、任务的 ID 由引擎生成：UUID v7 的 32 位十六进制；存储不解释它们。
- **值模型是 JSON**：变量、节点属性、输入、输出、节点状态与书签载荷都是 `System.Text.Json.Nodes.JsonObject`（值为 `JsonNode?`：null、bool、string、number、数组、对象）。存储以 JSON 文本持久化（`WorkflowJsonContext` 源生成，AOT 可用），内存实现保存深拷贝（读己之写，不泄漏引用）。`WorkflowVariables` 是对 `JsonObject` 的只读视图加类型化读取（`Get<T>`、`TryGet<T>`）。
- 时间一律 `DateTimeOffset`（UTC），取自 `TimeProvider`。租户 ID 是字符串（RFC-0007）。

## 定义

`WorkflowDefinition`（不可变记录）：`Id`、`Name`（`WorkflowName`）、`Version`（≥ 1）、`DisplayName`、`Description`、`Category`、`Status`（`Draft` / `Published` / `Disabled` / `Archived`）、`EnableCompensation`（取消时补偿）、`CompensateOnFault`（故障时也补偿）、`Timeout`（实例级超时：启动后超过即按 `TimeoutAction` 处理）、`TimeoutAction`（`Fault` 默认 / `Terminate`）、`Nodes`、`Transitions`、`Variables`、`TenantId`、`CreatedAt`、`PublishedAt`、`Properties`（`JsonObject`，应用扩展）。

- `WorkflowNode`：`Id`、`DisplayName`、`ActivityType`、`Properties`（`JsonObject`，字符串值支持 `{{ 表达式 }}` 模板）、`RetryPolicy`（`MaxAttempts` ≥ 1 含首次、`FirstDelay`、`BackoffFactor` ≥ 1）、`Timeout`（挂起后多久超时）、`ContinueOnError`、`OutputMappings`（变量名 → 表达式：节点完成后以 `outputs` 为作用域求值并写入实例变量，Camunda 的输出映射）。
- `WorkflowTransition`：`Id`、`SourceNodeId`、`TargetNodeId`、`Condition`（表达式，空为无条件）、`Priority`（小者先）、`IsDefault`。
- `WorkflowVariableDefinition`：`Name`、`Required`、`DefaultValue`（`JsonNode?`）、`Description`。
- **触发式启动**：`Start` 节点属性 `SignalName`（信号启动：`SignalAsync(name, payload, correlationId)` 在恢复书签之后，为调用方分区里每个 `Start.SignalName` 等于该名字的已发布定义启动一个实例，变量 `signal` = 载荷，关联 ID = 信号的关联 ID）、`Cron`（定时启动，`CronExpression` 文法，可带 `TimeZone`：发布时创建定义级 `DefinitionTimer` 书签（`InstanceId` = 定义 ID），到期由轮询驱动启动一个实例并推算下一次；禁用、归档、删除定义时删除该书签）。

**校验**（`WorkflowDefinitionValidator`，发布与启动前执行；失败 → `WORKFLOW_VALIDATION_001`，`details.errors` 列出全部问题）：名字与版本合法；至少一个节点；节点 ID 唯一；每个活动类型已登记；恰好一个 `Start` 且无入边；转移两端存在；条件表达式与输出映射表达式语法合法；每个节点最多一条默认出边；`Join` 的 `Mode`、`UserTask` 的 `CompletionPolicy`、`EmptyAssigneeStrategy` 与 `ApprovalThreshold` 取值合法；`End` 的 `Outcome` 不为空串；每个 `UserTask` 至少一个指派来源且规则类型已登记；`Start.Cron` 是合法 cron；`EventGateway` 至少一个信号；`Http` 的 `Url` 非空、`Method` 合法；所有节点自 `Start` 可达。

**分区**（ADR-0077，推翻 ADR-0066 第 5、6 节的全局定义与"无租户的调用看得到全部"）：工作流的数据分成平台分区与每个租户一个分区。一次调用只在自己的分区里读写——分区是当前调用的租户（RFC-0007），无租户的调用（平台）是平台分区。定义、实例、节点实例与书签都属于一个分区（`TenantId` 为分区的租户，平台为空），之后不变。

- 看得到、改得动的只有自己分区的：租户看不到平台与别的租户的定义、实例、书签，平台也看不到任何租户的。看不到的一律按不存在处理（定义 `WORKFLOW_NOTFOUND_001`、实例 `WORKFLOW_NOTFOUND_002`、书签 `WORKFLOW_NOTFOUND_003`），不泄露别的分区有没有这条记录。没有全局定义：平台建的定义对租户不可见，各租户共用的流程模板要在每个租户里各建一份（应用的种子负责）。
- 名字与版本号在分区里唯一：(`TenantId`, `Name`, `Version`) 唯一，平台与各租户的同名定义各自从版本 1 编号。按名字解析（启动、`FindPublishedAsync`、子流程与 ForEach、信号启动）只在调用方的分区里找，找不到不退回别的分区。
- 不能跨分区写：建定义时定义的 `TenantId` 给了别的租户（平台写租户、租户写平台或别的租户）→ `WORKFLOW_NOTFOUND_001`；启动请求没有租户字段，实例总是属于调用方的分区。
- 随租户分库：数据库实现里租户分区的记录与业务实体一样经持久化的多租户与分片路由落到租户的库（RFC-0005、RFC-0007），平台分区的记录在平台库，见「默认实现与 Provider」。

**目录**（`IWorkflowDefinitionCatalog`）：`CreateAsync(definition)`（定义属于调用方的分区；新版本号 = 分区里同名最大版本 + 1，状态 Draft）、`UpdateAsync(definition)`（只允许 Draft）、`PublishAsync(ids[])`（只允许 Draft，先校验；带 `Start.Cron` 的建定义级定时书签）、`NewVersionAsync(name)`（以调用方分区里的最新版本为底复制成 Draft，新草稿留在同一分区；没有 → `WORKFLOW_NOTFOUND_001`）、`DisableAsync(ids[])`（只允许 Published）、`ArchiveAsync(ids[])`、`DeleteAsync(ids[])`（只允许 Draft）、`FindAsync(id)`、`FindPublishedAsync(name, version?)`（按名字在调用方分区里解析）、`ListAsync(name?, status?)`（只列调用方分区的）、`ActivityTypes()`（已登记活动的描述符：类型、显示名、分类、输入属性，供设计器使用）。按标识的操作都是批量、逐项结果；看不到或改不动 → `WORKFLOW_NOTFOUND_001`，状态不允许 → `WORKFLOW_CONFLICT_002`。`WorkflowDefinitionBuilder` 以流式 API 生成定义；`WorkflowJson.Serialize(definition)` / `WorkflowJson.DeserializeDefinition(json) → Result<WorkflowDefinition>` 为定义的 JSON 往返（应用的设计器与存储用；`WorkflowJson` 同样提供实例、节点实例与书签的往返与深拷贝）。

## 活动

- `IActivity.ExecuteAsync(ActivityContext, ct)` → `Result<ActivityOutcome>`；`IResumableActivity.ResumeAsync(ActivityResumeContext, ct)`（书签被消费后调用，上下文带书签与恢复输入）；`ICompensableActivity.CompensateAsync(ActivityContext, ct)`（补偿时按完成顺序逆序调用）。失败的 `Result` 即故障；抛出异常翻译为 `WORKFLOW_UNEXPECTED_001` 的故障。
- `ActivityOutcome`：`Complete(outputs?, outcome?)`、`EndInstance(outputs?, outcome?)`、`Suspend(bookmarks…)`、`CompleteWithChildren(children, outputs?, outcome?)`、`SuspendWithChildren(children, bookmarks…)`。输出合并进实例变量；`outcome` 写进节点输出，并写进实例变量 `outcome`（最近一次带结果的节点），出边条件与后继决策节点都按它选边（没有结果的节点不覆盖）。
- 活动是作用域服务：`services.AddActivity<TActivity>(type, outgoing: ActivityOutgoing.AllMatched, descriptor?)`，登记进 `ActivityRegistry`；重复登记同一类型是编程错误。活动经 `ActivityContext.Services` 解析其他服务，经 `ActivityContext.Invocation` 拿到本次执行的调用上下文。
- **执行方式**（ADR-0077）：节点在引擎这次操作所在的调用里执行——同一服务作用域、同一调用上下文、同一事务：活动经 `ActivityContext.Services` 取到的仓储、结果处理器的写入与引擎的写入一起提交或回滚，活动看到的主体、租户与追踪就是这次调用的。引擎的操作总是在一次调用里：调用方的（加入它的事务），或调用方之外使用时引擎自己开的（见「事务」）。取消令牌是这次调用的令牌：取消即整个操作随事务回滚。活动失败（失败的 `Result` 或异常）按「故障与重试」处理，它在失败之前做的写入留在同一事务里、不单独回滚：会写入的活动与结果处理器先校验、后写入。
- 内置活动（`ActivityTypes`）：`Start`（可延迟启动：`WorkflowStartRequest.StartAt` 时以 Timer 书签挂起到该时刻）、`End`（可带 `Outcome`：先调用全部 `IWorkflowOutcomeHandler`，再结束实例）、`Terminate`（`Reason`）、`SetVariable`（`Values` 字面量、`Expressions` 表达式）、`Decision`（出边 Exclusive）、`Parallel`（出边 All）、`Join`（`Mode` = `WaitAll` / `WaitAny`）、`Delay`（`Duration` 秒或表达式，或 `Until` 时刻表达式，或 `Cron` + `TimeZone` 的下一次；Timer 书签）、`UserTask`、`PublishEvent`（`EventName`、`Payload`，发布 `workflow.custom` 消息）、`WaitSignal`（`SignalName`、`AcceptAnyCorrelation`，Signal 书签）、`EventGateway`（`Signals` 列表 + 可选 `TimeoutSeconds`：先到的信号消费、其余书签清掉，结果标签 = 信号名或 `timeout`，输出 `signal`、`payload`；BPMN 的事件网关）、`SubWorkflow`（`DefinitionName`、`DefinitionVersion`、`Variables`、`VariableExpressions`、`WaitForCompletion`、`ResultVariable`、`FailOnChildFault`）、`ForEach`（`Items`、`ItemVariable`、`DefinitionName`、`Parallel`、`FailFast`、`ResultVariable`）、`Log`（`Message`、`Level`）、`Fault`（`Message`）、`Compensate`（`NodeIds` 可选：对已完成的指定节点（缺省为本节点之前全部已完成节点）逆序执行补偿后继续；BPMN 的补偿抛出事件）、`Http`（`Url`、`Method`、`Headers`、`Body`、`ContentType`、`TimeoutSeconds`、`ResultVariable` 默认 `http`、`FailOnErrorStatus` 默认 `true`；输出 `{ status, headers, body }`，JSON 响应解析为对象；经 `WorkflowHttpClient`，应用可替换）、`ExternalTask`（`Topic`、`Payload`：External 书签；活动没有锁时长属性，锁时长由工作者领取时 `FetchAsync` 的 `lockDuration` 给出，由外部工作者经 `IWorkflowExternalTaskService` 领取并完成；Camunda 的外部任务）。`Script` 活动不进框架：表达式语言与 `SetVariable` 覆盖其场景，脚本引擎属于第三方依赖。
- 出边选择（`ActivityOutgoing`）：`AllMatched`（默认）取全部无条件或条件为真的出边，一条都没有时取默认出边；`Exclusive` 按优先级取第一条无条件或条件为真的，没有则默认出边；`All` 取全部；`None` 不流出。`AllMatched` / `Exclusive` 一条也选不到 → 实例故障。与 4.x 的差别：默认出边在两种模式下都是"兜底"，不再在 `AllMatched` 下无条件流出。
- `Join`：按入边计数；`WaitAll` 全部到达才执行，`WaitAny` 首个到达执行、其余到达忽略直至一轮结束。波次结束时仍有未凑齐的 Join 而没有书签 → 实例故障（`workflow.join.dead`）。

## 实例与执行

`WorkflowInstance`：`Id`、`DefinitionId`、`DefinitionName`、`DefinitionVersion`、`DisplayName`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Terminated`）、`Variables`、`JoinStates`、`CorrelationId`、`StarterId`、`ParentInstanceId`、`ParentNodeInstanceId`、`Depth`、`TenantId`、`CreatedAt`、`StartedAt`、`EndedAt`、`Fault`（`Error?`）、`FaultNodeId`、`FaultNodeInstanceId`、`CancellationReason`、`Outcome`。`WorkflowNodeInstance`：`Id`、`InstanceId`、`NodeId`、`ActivityType`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Compensated`）、`Attempt`（从 1 起，重试复用同一记录）、`StartedAt`、`EndedAt`、`Inputs`、`Outputs`、`State`、`Fault`、`CompensatedAt`。

- `StartAsync(WorkflowStartRequest)`：按 `DefinitionId` 或 `Name` + `Version`（缺省最新已发布）在调用方的分区里找定义，必须 Published（否则 `WORKFLOW_NOTFOUND_001`）；声明的变量缺省补默认值，缺必填 → `WORKFLOW_VALIDATION_002`；深度超过 `MaxSubWorkflowDepth` → `WORKFLOW_VALIDATION_002`。实例属于调用方的分区。`ReusePolicy`（按 `CorrelationId` + 定义名在同一分区里找未结束的实例：`AllowDuplicate` 默认；`RejectActive` → `WORKFLOW_CONFLICT_005`；`ReturnActive` 返回已有实例；没有 `CorrelationId` 时忽略）。要查重时，查重与写入新实例在同一把 `IWorkflowInstanceLock` 锁里完成（锁键 `start.` + 分区租户、定义名与关联 ID 的 SHA-256 十六进制），锁持有到这次操作的事务结束：并发的同键启动串行化，后到者一定看得到先到者提交的实例，`RejectActive` 只有一个成功、`ReturnActive` 都拿到同一个实例；等锁超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。`StartAt` 为延迟启动。定义有 `Timeout` 时建 `InstanceTimeout` 书签。实例先写入存储再加锁执行第一个波次。
- **波次**：在实例锁内从队列逐个执行节点直到队列空或实例不再 Running；一个波次执行节点数超过 `MaxNodeExecutionsPerBurst` → 实例故障（`workflow.engine.runaway`）。波次结束：有 `EndInstance` 或没有剩余书签（实例级超时书签不计）且没有未凑齐的 Join → `Completed`；否则实例保持 Running 等待书签。子流程启动与通知父流程在锁释放后进行（避免死锁）。
- **书签**（`WorkflowBookmark`：`Id`、`InstanceId`、`NodeId`、`NodeInstanceId`、`Kind`、`Key`、`Payload`、`DueAt`、`CorrelationId`、`CreatedAt`、`TenantId`）。种类 `BookmarkKinds`：`UserTask`（Key = 指派人）、`Timer`、`Signal`（Key = 信号名）、`SubWorkflow`（Key = 父节点实例 ID）、`Retry`、`NodeTimeout`、`External`（Key = 主题）、`Reminder`（用户任务提醒）、`InstanceTimeout`（实例级超时）、`DefinitionTimer`（定时启动，`InstanceId` = 定义 ID）。`ResumeAsync(bookmarkId, inputs?, expectedKey?)`：消费书签（删除）后恢复节点；实例不是 Running → `WORKFLOW_CONFLICT_003`（`DueBookmarkPoller` 对此类书签把 `DueAt` 后推 `NotResumableBackoff`）；`expectedKey` 与书签 `Key` 不同 → `WORKFLOW_CONFLICT_003`。`Reminder`、`InstanceTimeout`、`DefinitionTimer` 由引擎直接处理，不经活动。`SignalAsync(name, payload?, correlationId?)` 消费调用方分区里匹配的全部 Signal 书签（`CorrelationId` 为空的书签接受任意关联），再按触发式启动规则启动实例，返回消费数（含启动数）。
- **故障与重试**：节点失败时按顺序：有重试策略且未用尽 → 节点 Faulted、插入 Retry 书签（`DueAt` = now + `FirstDelay × BackoffFactor^(Attempt−1)`），实例保持 Running；`ContinueOnError` → 节点 Faulted、变量 `lastError` = 错误码与消息、以 `outcome = "error"` 流出；否则实例 Faulted（记录 `Fault`、`FaultNodeId`、`FaultNodeInstanceId`；`CompensateOnFault` 时补偿）。`RetryAsync(ids[])` 只接受 Faulted 且有 `FaultNodeInstanceId` 的实例：清故障、从故障节点重试（`Attempt + 1`）；引擎级故障（死 Join、波次上限）不可重试 → `WORKFLOW_CONFLICT_001`。
- **实例的分区**：实例属于启动它的调用的分区（`TenantId`，之后不变；子流程与父实例同分区）。调用只看得到、操作得了自己分区的实例与书签，别的分区的一律按不存在处理，不暴露它们是否存在：按实例 ID 的控制（`SuspendAsync`、`ResumeInstancesAsync`、`CancelAsync`、`TerminateAsync`、`RetryAsync`、`SetVariablesAsync`、`JumpAsync`、`MigrateAsync`）与实例查询 → `WORKFLOW_NOTFOUND_002`；按书签 ID 的 `ResumeAsync`（含定义级定时书签）与用户任务、外部任务的操作 → `WORKFLOW_NOTFOUND_003`；`PurgeAsync` 只清自己分区的实例；信号只消费自己分区的书签、只启动自己分区的定义。平台与租户对等：平台调用同样看不到、操作不了任何租户的实例。引擎内部的接续（轮询恢复、子流程启动与回调、级联取消）在实例的分区里进行：轮询器以书签的租户开调用。
- **查询**：`FindInstanceAsync(id)`、`ListInstancesAsync(query)`、`CountInstancesAsync(query)`、`NodeInstancesAsync(instanceId)` 只查调用方的分区；`WorkflowInstanceQuery` 没有租户字段——平台调用查到的就是平台的实例（原来"租户为空即不过滤全部租户"的语义取消），没有跨租户的查询。按 ID 找不到或在别的分区为无值 / `WORKFLOW_NOTFOUND_002`。`WorkflowInstanceQuery` 的分页（`Skip` ≥ 0、`Limit` 1～`MaxLimit`（1000），默认 0 与 100）不在构造时抛出：越界时 `ListInstancesAsync` 返回 `WORKFLOW_VALIDATION_002`（HTTP 400，消息键 `workflow.instance.query_page_invalid`，参数 `skip`、`limit`、`max`），与待办查询一致——查询常由请求绑定，越界是调用方可以改正的输入（2026-11-01 修订，ADR-0082）；`CountInstancesAsync` 不看分页，不校验。
- **控制**（批量，逐项结果）：`SuspendAsync(ids[], reason?)`（只允许 Running）、`ResumeInstancesAsync(ids[])`（只允许 Suspended；不重新驱动书签，到期书签由轮询接续）、`CancelAsync(ids[], reason?)`、`TerminateAsync(ids[], reason?)`（Running / Suspended / Faulted 可取消或终止，已结束的幂等返回；清掉非终态节点与全部书签；取消且 `EnableCompensation` 时执行补偿；级联到未结束的子流程并通知父流程）、`RetryAsync(ids[])`、`SetVariablesAsync(updates[])`（Running / Suspended：合并变量、删除指定键）、`JumpAsync(jumps[])`（Running / Suspended / Faulted：取消全部未结束节点与书签、清 Join 状态，从目标节点重新执行；目标不存在 → `WORKFLOW_VALIDATION_002`）、`MigrateAsync(migrations[])`（Running / Suspended 迁到同一名字空间里同名的另一个已发布版本，别的名字空间的同名定义按不存在处理：每个未结束节点在目标定义里必须存在且活动类型相同，否则 `WORKFLOW_VALIDATION_002`；只改定义引用，节点实例与书签原样保留）、`PurgeAsync(request)`（删除早于给定时刻结束的实例及其节点实例与书签，可按状态与定义名过滤，返回条数；一次最多 `Limit` 条（1～1000，默认 1000），越界不在构造时抛出，返回 `WORKFLOW_VALIDATION_002`（消息键 `workflow.purge.limit_invalid`，参数 `limit`、`max`）；`XiHan:Workflow:RetentionDays` > 0 时由轮询器每 `HousekeepingIntervalSeconds` 自动执行）。状态不允许 → `WORKFLOW_CONFLICT_001`。单实例的写法是扩展方法。
- **子流程**：`SubWorkflow` / `ForEach` 的子实例带 `ParentInstanceId`、`ParentNodeInstanceId`、`Depth + 1`，继承父实例的租户、`CorrelationId` 与 `StarterId`；子实例到达终态时以 `SubWorkflow` 书签（Key = 父节点实例 ID）回调父实例，输入 `childInstanceId`、`childStatus`、`childVariables`、`childFault`；父节点实例 `State.childInstanceIds` 记录本轮启动的子实例，过期回调忽略。父实例正常完成不级联到不等待的子实例；`End(Outcome)` 与 `Terminate` 级联终止。
- **锁**：`IWorkflowInstanceLock.AcquireAsync(instanceId, ct)` → `Result<IAsyncDisposable>`；等待超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。Provider 的锁不可重入，与调用方事务无关（数据库实现的租约行走自己的连接与事务，立即对别的节点可见、不随调用方回滚）。锁键是不透明字符串：实例 ID，定时启动用定义 ID，重用策略查重用 `start.` 开头的派生键，Provider 不得假设它是实例 ID（长度不超过 128）。引擎在一个服务作用域里取得的锁持有到作用域结束（调用方事务提交或回滚之后）才释放：别的执行者不会在提交之前读到旧状态再改写它；同一作用域里再取空闲的锁直接复用（子流程回调父实例、同一请求办理两个任务），正在使用中再取立即以 `WORKFLOW_TIMEOUT_001` 返回（同一执行流的重入，等也等不到）。调用方事务里等锁最多 `LockTimeout`：两个事务交叉等对方持有的锁时一方超时、回滚并释放，不会无限等待。
- **事件**：装配了 `messaging.queue` 时经这次调用作用域里的 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/workflow`，`Type` = 主题，`Data` = `WorkflowEventData` JSON，扩展 `tenant` / `correlation`）；没有发布者时跳过；发布失败只记日志。事件与事务的关系（ADR-0077）：事件在事务里发布——装配了发件箱（`OutboxFeature`，RFC-0009）时写进与引擎写入同一事务的发件箱表，提交后由投递循环发出，回滚即丢弃；没有发件箱时直接发布到代理，可能早于提交、回滚时不撤回。要求事件与状态一致的应用必须装配发件箱。主题：`workflow.instance.started|completed|faulted|canceled|terminated|suspended|resumed|jumped|migrated|timedout`、`workflow.node.executing|executed|faulted`（`WillRetry`）、`workflow.task.created|completed|transferred|claimed|released|returned|urged|reminder`、`workflow.external.created|completed|failed`、`workflow.custom`（`Subject` = 事件名）。

## 事务

ADR-0077：引擎、定义目录、任务服务与外部任务服务的每个操作都在一个事务里，业务在自己的调用里发起流程、办理任务时，这个事务就是业务的事务。

- **加入调用方的事务**：服务所在的作用域里 `IWorkflowTransaction.IsActive` 为真时——数据库实现：在调用里且 Transaction 阶段已开启工作单元——操作在调用方的事务里执行：存储读写、活动与结果处理器的写入、经发件箱发出的事件都随调用方提交或回滚。业务回滚后引擎不留任何状态（实例、节点实例、书签、发件箱里的事件）；业务提交时与引擎写入一起落库，例如"写申请并发起流程"、"授予并完成实例"都是原子的。
- **自己开调用**：不在可加入的事务里时（到期书签轮询、定时器、后台恢复、调用之外的直接使用、Tenant / Authorize 阶段里的调用），引擎在新的服务作用域里经完整管道开一次调用（来源协议 `workflow`、目标是操作名），以调用方的租户（Baggage `xihan.tenant`，并预先写进安全上下文）进入，操作在那次调用的事务里执行：单个操作失败即回滚；批量操作整批一个事务，逐项失败不回滚其余项（失败的项没有写入），提交失败时每一项都报告提交的错误。应用不在服务状态时返回 `WORKFLOW_UNAVAILABLE_001`。自己开的调用里仍不能加入事务（持久化没有在 Transaction 阶段开启工作单元）是装配错误，返回 `WORKFLOW_UNAVAILABLE_001`，不再嵌套。
- **只读的调用**：调用方的工作单元只读（RFC-0005「只读工作单元」）时，查询照常；写入操作在取锁、执行活动之前即返回 `PERSISTENCE_UNEXPECTED_001`（`IWorkflowTransaction.EnsureWritable`；只读操作里发起或推进流程是操作实现的缺陷，HTTP 500），只读调用里的活动副作用不会先发生。
- **锁与事务的先后**：实例锁在调用方事务里获取，锁行自己提交（见「锁」），持有到作用域结束才释放：事务提交之后别的执行者才拿得到锁、读到新状态；事务回滚后作用域结束即释放，不泄漏。作用域必须被释放（管道的请求作用域、`await using` 的作用域）：没有释放的作用域会一直持有并续约它的锁。
- **不加入事务的 Provider**：内存实现没有事务，`IsActive` 只看是否在调用里；写入立即生效、调用方回滚不撤销，也不看调用是否只读。一致性套件以 `JoinsCallerTransaction` 区分两种 Provider。
- **SQLite**：只有一个写者，调用方事务写过之后再在另一个连接上写租约行要等调用方事务结束，即等自己；在 SQLite 上要把租约表（`WorkflowLockRecord`）经 `IEntityConnectionSelector` 放到另一个库文件。行级锁的数据库（PostgreSQL、MySQL、SQL Server）不受影响。

## 用户任务

- `UserTask` 活动属性：`Assignees`（列表）、`AssigneesExpression`、`AssigneeRules`（`[{ type, values }]`）三者合并去重；`ExcludeStarter`；`EmptyAssigneeStrategy`（`Fault` 默认 / `AutoApprove` / `AutoReject` / `Fallback` + `FallbackAssigneeRules`）；`CompletionPolicy`（`Any` 默认 / `All` / `Sequential`）；`ApprovalThreshold`（`All` 下的通过阈值：人数或百分比如 `"60%"`，缺省全部；通过数达到即通过，剩余人数不可能达到即以 `rejected` 完成）；`Title`（模板，缺省节点名）；`FormData`；`FormKey`；`Priority`；`DueAfterSeconds` / `DueAt`（截止时刻，写进任务视图）；`ReminderAfterSeconds`（每隔该时长发布一次 `workflow.task.reminder`，直至任务结束）；`CcUserIds` / `CcRules`；`AllowedOutcomes`（缺省 `approved` / `rejected`）。
- 挂起时每个指派人一个 `UserTask` 书签（载荷 `title`、`formData`、`formKey`、`priority`、`dueAt`、`ccUserIds`），`Sequential` 一次只有当前指派人的一个；节点 `Timeout` 另插一个 `NodeTimeout` 书签，超时以 `outcome = "timeout"` 完成节点；`ReminderAfterSeconds` 另插 `Reminder` 书签。
- `UserTask`（视图）：`Id`（= 书签 ID）、`InstanceId`、`InstanceTitle`、`DefinitionName`、`NodeId`、`NodeInstanceId`、`Title`、`AssigneeId`、`ClaimedBy`、`Priority`、`DueAt`、`FormKey`、`CorrelationId`、`StarterId`、`FormData`、`CreatedAt`、`TenantId`。
- `IWorkflowTaskService`：`PendingAsync(assigneeId)` / `PendingByInstanceAsync(instanceId)` / `QueryAsync(UserTaskQuery)` → `UserTaskPage(Items, Total)`（指派人或实例必填其一，可按定义名、节点、创建时间过滤，按创建时间升序或降序（`Descending`）、同刻按写入先后；过滤、排序与分页由存储执行（`IWorkflowBookmarkStore.ListUserTasksAsync` / `CountUserTasksAsync`），`Total` 是同条件的总数；`Skip` ≥ 0、`Limit` 取 1 到 `UserTaskQuery.MaxLimit`（1000，与持久化的查询条数上限一致），越界在查询时返回 `WORKFLOW_VALIDATION_002`（HTTP 400），不在构造时抛出；只含调用方分区、实例 Running 的任务）；`CompleteAsync(completions[])`（`UserTaskCompletion`：`TaskId`、`ActorId`、`Outcome`、`Comment`、`Variables`；操作者必须是该书签的指派人 → 否则 `WORKFLOW_DENIED_001`；`Outcome` 不在允许范围 → `WORKFLOW_VALIDATION_002`；`Any` 首个通过即完成节点、`All` 按阈值完成（其间合并变量并保留其余书签）、`Sequential` 逐个；非 `approved` 的结果立即以该结果完成节点）；`TransferAsync(transfers[])`（改书签 `Key`，目标已是指派人 → `WORKFLOW_CONFLICT_004`）；`AddAssigneesAsync(additions[])`（`Any` / `All` 立即加书签，`Sequential` 排到队尾）；`ClaimAsync(claims[])`（`Any` 策略下认领：其他指派人的书签撤下，节点状态记 `claimedBy`；已被认领 → `WORKFLOW_CONFLICT_004`）；`ReleaseAsync(releases[])`（释放认领，恢复其他指派人的书签）；`ReturnAsync(returns[])`（退回：`TargetNodeId` 缺省为本节点之前最近完成的 `UserTask` 节点，没有则退回 `Start` 之后的首个节点；以跳转实现，节点历史记 `returned`）；`UrgeAsync(urges[])`（催办：发布 `workflow.task.urged`，不改状态）。节点 `State` 记 `assignees`、`approved`、`rejected`、`claimedBy`、`history`（`actorId`、`outcome`、`comment`、`at`）。
- 指派规则 `AssigneeRule(Type, Values)`：`IAssigneeProvider`（`RuleType`、`Validate(rule)`、`ResolveAsync(AssigneeResolution)`）按类型登记（`AddAssigneeProvider<T>()`，同类型重复登记是编程错误）；内置 `User`（原样）、`Starter`（实例发起人）、`Expression`（表达式求值为列表或逗号分隔字符串）。解析结果裁剪、去重、保序；未登记的类型 → `WORKFLOW_NOTFOUND_004`；Provider 返回失败即节点故障（不得以空列表掩盖数据不可用）。

## 外部任务

`ExternalTask` 活动以 `External` 书签（Key = 主题，载荷 `topic`、`payload`、`lockedBy`、`lockedUntil`、`notBefore`）挂起。`IWorkflowExternalTaskService`：`FetchAsync(topic, workerId, lockDuration, maxTasks)`（领取调用方分区里该主题未锁定（或锁已过期）且已到 `notBefore` 的任务，按创建时间最多 `maxTasks` 条：`maxTasks` 取 1 到 `MaxTasksPerFetch`（100），`lockDuration` 须为正，越界 → `WORKFLOW_VALIDATION_002`；可领取时刻与条数经 `IWorkflowBookmarkStore.AvailableAsync` 下推到存储查询，不读回该主题的全部书签；候选在实例锁内重读，仍可领取才把 `lockedBy` / `lockedUntil` 写回书签，被别的工作者抢先或实例已不在运行中的跳过，所以结果可能少于 `maxTasks`；返回 `ExternalTask` 视图：`Id`、`InstanceId`、`Topic`、`Payload`、`Variables`、`LockedUntil`、`Attempt`）；`CompleteAsync(completions[])`（`taskId`、`workerId`、`outputs`：工作者必须持有未过期的锁 → 否则 `WORKFLOW_DENIED_002`；输出合并进实例变量后节点完成）；`FailAsync(failures[])`（`taskId`、`workerId`、`error`、`retryAfter?`：有 `retryAfter` 时解锁并把 `notBefore` 后推、任务 `attempt + 1`；没有时节点以该错误故障，按节点重试策略处理）；`ExtendLockAsync(extensions[])`。锁到期的任务自动解锁（书签 `DueAt` = `lockedUntil`，到期由轮询驱动、活动重新挂起）。

## 表达式

`IWorkflowExpressionEvaluator`：`Evaluate(expression, scope)` → `Result<Maybe<JsonNode>>`（结果是 `null`——字面量 `null`、缺少的成员或键、值为 null 的变量——时是成功的无值，不是失败；ADR-0059）、`EvaluateCondition` → `Result<bool>`（结果不是 bool 即失败，无值也算不是 bool）、`Render(template, scope)` → `Result<string>`（`{{ 表达式 }}` 占位，引号内的 `}}` 不算结束，无值渲染成空串）。活动里的 `ActivityContext.EvaluateProperty(name)` 同样返回 `Result<Maybe<JsonNode>>`：没有这个属性是无值，字符串属性当表达式求值，其余属性原样作为有值。默认实现是无依赖的小语言：`||`、`&&`、`== !=`、`< <= > >=`、`+ -`、`* / %`、一元 `! -`、成员 `.x` 与索引 `[i]`、字面量（数字为 decimal、单双引号字符串、`true` / `false` / `null`）、标识符与函数调用；`+` 任一侧是字符串即拼接；`==` 跨数字类型宽松、字符串序数比较；未知根变量是错误（不是 null），缺少的成员或键是 null；函数 `len`、`contains`、`startsWith`、`endsWith`、`upper`、`lower`、`trim`、`isNullOrEmpty`、`abs`、`min`、`max`、`round`、`toNumber`、`toString`、`now`、`date`（名字不区分大小写）。语法或求值失败 → `WORKFLOW_VALIDATION_003`。已解析的表达式按文本缓存，上限 `ExpressionCacheSize`。

## 存储契约

三个存储是作用域服务，按分区读写：每个方法的第一个参数 `tenantId` 是分区（空为平台），只返回这个分区的记录，写入的记录的 `TenantId` 必须等于它（不等是编程错误，抛出 `ArgumentException`，不改写、不落库）；不同分区的同一 ID 互不相干。数据库实现在调用方的工作单元里读写，分区必须等于工作单元的租户（不等抛出 `InvalidOperationException`）。不做乐观并发（实例锁串行化写入），写入按批量（ADR-0012）：

- `IWorkflowDefinitionStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`FindAsync(tenantId, name, version)`、`FindLatestPublishedAsync(tenantId, name)`、`MaxVersionAsync(tenantId, name)`、`ListAsync(tenantId, name?, status?)`（名字升序、版本降序）、`SaveAsync(tenantId, definitions[])`（新增或整体替换）、`DeleteAsync(tenantId, ids[])`。
- `IWorkflowInstanceStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`ListAsync(tenantId, WorkflowInstanceQuery)`（`Status` / `Statuses`、`DefinitionName`、`CorrelationId`、`ParentInstanceId`、`StarterId`、`CreatedFrom` / `CreatedTo`、`EndedBefore`、`Skip`、`Limit` ≤ 1000；创建时间降序）、`CountAsync(tenantId, query)`、`SaveAsync(tenantId, instances[])`、`DeleteAsync(tenantId, ids[])`（级联节点实例）；节点实例 `FindNodeInstanceAsync(tenantId, id)`、`NodeInstancesAsync(tenantId, instanceId)`（开始时间升序、同刻按创建顺序）、`SaveNodeInstancesAsync(tenantId, nodeInstances[])`。
- `IWorkflowBookmarkStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`ByInstanceAsync(tenantId, instanceId)`、`DueAsync(tenantId, now, limit)`（`DueAt` ≤ now，`DueAt` 升序；轮询器逐个分区调用）、`ByKindAndKeyAsync(tenantId, kind, key)`（创建时间升序）、`AvailableAsync(tenantId, kind, key, now, limit)`（种类与键匹配、`DueAt` 为空或 ≤ now，创建时间升序，最多 limit 条；外部任务领取用）、`BySignalAsync(tenantId, name, correlationId?)`、`ListUserTasksAsync(tenantId, UserTaskQuery)`（种类为 `UserTask`、所属实例 Running 的书签，指派人匹配书签的键，其余条件匹配实例、节点、所属实例的定义名与创建时间；按创建时间排序、同刻按写入先后，跳过 `Skip` 条后最多 `Limit` 条；实例状态变化后书签随之进出结果）、`CountUserTasksAsync(tenantId, UserTaskQuery)`（同条件的总数）、`SaveAsync(tenantId, bookmarks[])`、`DeleteAsync(tenantId, ids[])`、`DeleteByInstanceAsync(tenantId, instanceIds[])`。
- `IWorkflowTransaction`（作用域）：`IsActive`（当前作用域在不在可以加入的调用方事务里）、`EnsureWritable()`（只读事务里返回拒绝写入的错误），见「事务」。
- 存储不可达 → `WORKFLOW_UNAVAILABLE_001`；容量已满（内存默认实现）→ `WORKFLOW_UNAVAILABLE_002`。

## 默认实现与 Provider

- **轮询**（`DueBookmarkPoller`）：每个周期覆盖平台分区与租户目录（`ITenantStore`，装配了多租户时）里启用的每个租户分区：逐个分区开一次调用取最多 `BatchSize` 条到期书签，合并后按到期时间取前 `BatchSize` 条，逐条在以书签租户开的调用里恢复（每次恢复一个事务；Tenant 阶段为它绑定租户的分片）；实例不在运行中的在同一调用里把到期时间后推 `NotResumableBackoff`。停用的租户不轮询，它的到期书签等重新启用后再接续；新租户在进入租户目录后的下一个周期纳入；一个分区不可达（库连不上、租户校验失败）只记日志、不挡住别的分区。到了 `HousekeepingIntervalSeconds` 且 `RetentionDays` > 0 时逐个分区清理。
- `Workflow.InMemory`（`role: default`）：三个存储是有界内存表（`XiHan:Workflow:InMemory:MaxDefinitions` 默认 1000、`MaxInstances` 10000、`MaxNodeInstances` 100000、`MaxBookmarks` 100000，全部分区合计），键是分区加 ID，保存深拷贝；没有事务（见「事务」）；实例锁是按实例的进程内信号量；`DueBookmarkPoller` 由特性在应用进入服务状态后以 `XiHan:Workflow:InMemory:PollIntervalSeconds`（默认 1）轮询，每次最多恢复 `BatchSize`（默认 100）条，停止时等待进行中的波次。
- `Workflow.Database`（`role: provider`，依赖 `persistence.relational@1`，可选依赖 `multitenancy.context@1`）：定义、实例、节点实例、书签以 JSON 文档列加查询列的实体在调用方的工作单元里经仓储读写。租户分区的四张表 `WorkflowDefinitionRecord`、`WorkflowInstanceRecord`、`WorkflowNodeInstanceRecord`、`WorkflowBookmarkRecord` 实现 `IMultiTenant`：持久化按工作单元的租户写入与过滤，租户有独立库时落它的分片连接；平台分区的四张表 `PlatformWorkflow*Record` 不实现 `IMultiTenant`，落平台库。书签行另有待办查询下推用的三列：`NodeId`（取自书签）、`DefinitionName` 与 `InstanceStatus`（取自所属实例，写书签时从实例行读，实例新增或状态变化时由实例存储同步到它的书签行，都在同一事务里）。定义的唯一键：租户定义 `(Name, TenantId, Version)`、平台定义 `(Name, Version)`，都不含可为空的列，每一行都受唯一索引约束（平台定义不以哨兵租户值存进租户表：`IMultiTenant` 的表在无租户的调用里写不进、读不到，见 ADR-0077）。实例锁是平台库里的租约行（`WorkflowLockRecord`；`XiHan:Workflow:Database:LockLeaseSeconds` 默认 60，持有期间自动续约，并发戳 CAS 接管过期租约；每一步在自己的作用域与可写工作单元里提交，不经管道）；轮询只在持有 `workflow.poller` 租约的节点上进行，领导权全局一份（`PollIntervalSeconds`、`BatchSize` 同上）。租约表的连接（2026-11-01 修订，ADR-0084）：`XiHan:Workflow:Database:LockConnection` 给租约表指定命名连接（RFC-0005「命名连接」；空白视为未配置）——特性把 `WorkflowLockRecord` 登记到这个连接，租约的每一步与健康探针开绑定这个连接的工作单元，事务只开在租约库上。SQLite 上必须配：SQLite 只有一个写者，引擎在调用方事务里写过之后再写租约行，同一个库文件上要等的正是调用方自己的事务，忙等超时后以 `WORKFLOW_TIMEOUT_001` 失败；租约放到另一个库文件，业务调用不碰它，两边的事务互不等待。不配置时租约表照旧在默认连接（平台库），PostgreSQL 等行级锁的数据库不需要配。持久化 Provider 不支持命名连接（EF Core、内存实现）时装配即失败（`PERSISTENCE_UNAVAILABLE_008`），连接不在 SqlSugar 客户端里时启动即失败（`PERSISTENCE_VALIDATION_004`）。
- 引擎选项（`XiHan:Workflow`）：`MaxNodeExecutionsPerBurst`（1000）、`MaxSubWorkflowDepth`（16）、`LockTimeoutSeconds`（10）、`NotResumableBackoffSeconds`（300）、`ExpressionCacheSize`（10000）、`RetentionDays`（0 = 不自动清理）、`HousekeepingIntervalSeconds`（3600）、`HttpTimeoutSeconds`（30）。引擎与两个 Provider 的选项读取时值不合法即抛出（解析不了 → `ArgumentException`，超出取值范围 → `ArgumentOutOfRangeException`，消息带配置键与原值），应用启动失败，不按缺省处理；`XiHan:Workflow:Database:HolderId` 空白视为未配置。

## 一致性

Provider 继承 `WorkflowStoreConformance`（三个存储的查询、排序、分页与计数、批量与级联语义、可领取书签的过滤与上限、待办查询的过滤 / 排序 / 分页 / 计数且随实例状态进出、按分区的名字空间、分区互不可见、写入别的分区的记录即抛出、实例锁）与 `WorkflowEngineConformance`（顺序流、决策、并行与汇合、三种完成策略与阈值的用户任务、认领与退回、转办与加签、延时与超时、信号与事件网关、信号与定时启动、子流程与 ForEach、外部任务、HTTP 活动、补偿活动、重试与继续、取消 / 终止 / 挂起 / 恢复 / 跳转 / 迁移 / 改变量、实例级超时、清理、事件主题、并发的拒绝重复启动只建一个实例、定义按分区隔离、实例按分区隔离、业务回滚丢弃引擎写入、办理任务随业务提交或回滚、实例锁持有到调用结束且回滚后释放、只读调用在执行活动之前拒绝写入、轮询覆盖平台与每个启用的租户）并通过全部用例；事务相关的用例按 `JoinsCallerTransaction` 断言（内存实现为否）。

## 错误码

见 `conventions/error-codes.md` 中 `WORKFLOW_*`。

## 与 4.x 的对应

| 4.x | 5.0 |
| --- | --- |
| `IWorkflowEngine` 抛 `WorkflowException` | 返回 `Result`，错误码 `WORKFLOW_*` |
| `IWorkflowDefinitionManager` | `IWorkflowDefinitionCatalog`（批量） |
| `IWorkflowUserTaskService` | `IWorkflowTaskService`（批量） |
| `Dictionary<string, object?>` 变量 | `JsonObject` / `WorkflowVariables` |
| `IClock`、`DateTime` | `TimeProvider`、`DateTimeOffset` |
| `long? TenantId` | `string? TenantId` |
| `ILocalEventBus` 事件记录 | `workflow.*` 主题的消息 |
| `IDistributedLock` 实例锁 | `IWorkflowInstanceLock`（Provider 提供） |
| `WorkflowTimerWorker` | `DueBookmarkPoller`（Provider 驱动） |
| `Http` 活动 | `Http` 活动（`WorkflowHttpClient`） |
| `Script` 活动 | 不提供（表达式语言 + `SetVariable`） |
| 默认出边在 `AllMatched` 下总是流出 | 默认出边只作兜底 |
