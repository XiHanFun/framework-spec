# RFC-0021 工作流：持久化执行

> 状态：已采纳 | 日期：2026-10-26，同日修订（ADR-0020：`Workflow.Database`、触发式启动、事件网关、外部任务、HTTP 活动、补偿活动、跳转与迁移、认领 / 退回 / 催办、提醒与审批阈值、保留期清理） | 对应 ADR：ADR-0016、ADR-0020 | 参考实现：`XiHan.Framework.Workflow.Abstractions`、`XiHan.Framework.Workflow.InMemory`、`XiHan.Framework.Workflow.Database` | 一致性：`XiHan.Framework.Workflow.Conformance`

## 摘要

工作流子系统把"一张有向图上的长时间执行"做成与请求同构的调用：定义描述节点、转移与变量；实例是定义的一次执行，状态全部落在存储里；每个节点的活动在新作用域、新上下文里经完整内核管道执行；等待（人工任务、外部任务、定时、信号、子流程、重试、超时）以书签挂起，书签被消费时从存储恢复执行。引擎与协议性的规则在抽象包里，Provider 只实现三个存储、实例锁与到期书签的轮询驱动。能力 `workflow.execution@1`。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `workflow.execution@1` | 定义目录、实例执行、书签、用户任务、外部任务；`IWorkflowEngine`、`IWorkflowDefinitionCatalog`、`IWorkflowTaskService`、`IWorkflowExternalTaskService` | `Workflow.InMemory`（有界内存存储、进程内实例锁、单进程轮询） |

Provider 实现 `IWorkflowDefinitionStore`、`IWorkflowInstanceStore`、`IWorkflowBookmarkStore` 与 `IWorkflowInstanceLock`，并在应用进入服务状态后驱动 `DueBookmarkPoller`。`Workflow.Database`（依赖 `persistence.relational@1`）用租约行做分布式实例锁，多实例同时只有一个执行者；到期书签由持有轮询租约的节点轮询（其他节点同时轮询也只是无效的重复尝试）。

## 名字与值

- 定义名 `WorkflowName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同；一个定义名下多个版本）。
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
- **触发式启动**：`Start` 节点属性 `SignalName`（信号启动：`SignalAsync(name, payload, correlationId)` 在恢复书签之后，为每个 `Start.SignalName` 等于该名字的已发布定义（调用方租户或无租户的定义）启动一个实例，变量 `signal` = 载荷，关联 ID = 信号的关联 ID）、`Cron`（定时启动，`CronExpression` 文法，可带 `TimeZone`：发布时创建定义级 `DefinitionTimer` 书签（`InstanceId` = 定义 ID），到期由轮询驱动启动一个实例并推算下一次；禁用、归档、删除定义时删除该书签）。

**校验**（`WorkflowDefinitionValidator`，发布与启动前执行；失败 → `WORKFLOW_VALIDATION_001`，`details.errors` 列出全部问题）：名字与版本合法；至少一个节点；节点 ID 唯一；每个活动类型已登记；恰好一个 `Start` 且无入边；转移两端存在；条件表达式与输出映射表达式语法合法；每个节点最多一条默认出边；`Join` 的 `Mode`、`UserTask` 的 `CompletionPolicy`、`EmptyAssigneeStrategy` 与 `ApprovalThreshold` 取值合法；`End` 的 `Outcome` 不为空串；每个 `UserTask` 至少一个指派来源且规则类型已登记；`Start.Cron` 是合法 cron；`EventGateway` 至少一个信号；`Http` 的 `Url` 非空、`Method` 合法；所有节点自 `Start` 可达。

**目录**（`IWorkflowDefinitionCatalog`）：`CreateAsync(definition)`（新版本号 = 同名最大版本 + 1，状态 Draft）、`UpdateAsync(definition)`（只允许 Draft）、`PublishAsync(ids[])`（只允许 Draft，先校验；带 `Start.Cron` 的建定义级定时书签）、`NewVersionAsync(name)`（以最新版本为底复制成 Draft）、`DisableAsync(ids[])`（只允许 Published）、`ArchiveAsync(ids[])`、`DeleteAsync(ids[])`（只允许 Draft）、`FindAsync(id)`、`FindPublishedAsync(name, version?)`、`ListAsync(name?, status?)`、`ActivityTypes()`（已登记活动的描述符：类型、显示名、分类、输入属性，供设计器使用）。按标识的操作都是批量、逐项结果；状态不允许 → `WORKFLOW_CONFLICT_002`。`WorkflowDefinitionBuilder` 以流式 API 生成定义；`WorkflowDefinitionJson.Serialize / Deserialize` 为定义的 JSON 往返（应用的设计器与存储用）。

## 活动

- `IActivity.ExecuteAsync(ActivityContext, ct)` → `Result<ActivityOutcome>`；`IResumableActivity.ResumeAsync(ActivityResumeContext, ct)`（书签被消费后调用，上下文带书签与恢复输入）；`ICompensableActivity.CompensateAsync(ActivityContext, ct)`（补偿时按完成顺序逆序调用）。失败的 `Result` 即故障；抛出异常翻译为 `WORKFLOW_UNEXPECTED_001` 的故障。
- `ActivityOutcome`：`Complete(outputs?, outcome?)`、`EndInstance(outputs?, outcome?)`、`Suspend(bookmarks…)`、`CompleteWithChildren(children, outputs?, outcome?)`、`SuspendWithChildren(children, bookmarks…)`。输出合并进实例变量；`outcome` 写进节点输出，并写进实例变量 `outcome`（最近一次带结果的节点），出边条件与后继决策节点都按它选边（没有结果的节点不覆盖）。
- 活动是作用域服务：`services.AddActivity<TActivity>(type, outgoing: ActivityOutgoing.AllMatched, descriptor?)`，登记进 `ActivityRegistry`；重复登记同一类型是编程错误。活动经 `ActivityContext.Services` 解析其他服务，经 `ActivityContext.Invocation` 拿到本次执行的调用上下文。
- **执行方式**：每次节点执行开新作用域，`App.CreateContext` 构造上下文：`xihan.correlation` = 实例 ID，`xihan.tenant` = 实例租户（有则设），经完整管道（认证 → 租户 → 授权 → … → Observe）执行；取消令牌是宿主停止令牌而不是调用方的令牌（调用方的令牌只约束等锁与首次写入）。
- 内置活动（`ActivityTypes`）：`Start`（可延迟启动：`WorkflowStartRequest.StartAt` 时以 Timer 书签挂起到该时刻）、`End`（可带 `Outcome`：先调用全部 `IWorkflowOutcomeHandler`，再结束实例）、`Terminate`（`Reason`）、`SetVariable`（`Values` 字面量、`Expressions` 表达式）、`Decision`（出边 Exclusive）、`Parallel`（出边 All）、`Join`（`Mode` = `WaitAll` / `WaitAny`）、`Delay`（`Duration` 秒或表达式，或 `Until` 时刻表达式，或 `Cron` + `TimeZone` 的下一次；Timer 书签）、`UserTask`、`PublishEvent`（`EventName`、`Payload`，发布 `workflow.custom` 消息）、`WaitSignal`（`SignalName`、`AcceptAnyCorrelation`，Signal 书签）、`EventGateway`（`Signals` 列表 + 可选 `TimeoutSeconds`：先到的信号消费、其余书签清掉，结果标签 = 信号名或 `timeout`，输出 `signal`、`payload`；BPMN 的事件网关）、`SubWorkflow`（`DefinitionName`、`DefinitionVersion`、`Variables`、`VariableExpressions`、`WaitForCompletion`、`ResultVariable`、`FailOnChildFault`）、`ForEach`（`Items`、`ItemVariable`、`DefinitionName`、`Parallel`、`FailFast`、`ResultVariable`）、`Log`（`Message`、`Level`）、`Fault`（`Message`）、`Compensate`（`NodeIds` 可选：对已完成的指定节点（缺省为本节点之前全部已完成节点）逆序执行补偿后继续；BPMN 的补偿抛出事件）、`Http`（`Url`、`Method`、`Headers`、`Body`、`ContentType`、`TimeoutSeconds`、`ResultVariable` 默认 `http`、`FailOnErrorStatus` 默认 `true`；输出 `{ status, headers, body }`，JSON 响应解析为对象；经 `WorkflowHttpClient`，应用可替换）、`ExternalTask`（`Topic`、`Payload`、`LockSeconds` 默认 300：External 书签，由外部工作者经 `IWorkflowExternalTaskService` 领取并完成；Camunda 的外部任务）。`Script` 活动不进框架：表达式语言与 `SetVariable` 覆盖其场景，脚本引擎属于第三方依赖。
- 出边选择（`ActivityOutgoing`）：`AllMatched`（默认）取全部无条件或条件为真的出边，一条都没有时取默认出边；`Exclusive` 按优先级取第一条无条件或条件为真的，没有则默认出边；`All` 取全部；`None` 不流出。`AllMatched` / `Exclusive` 一条也选不到 → 实例故障。与 4.x 的差别：默认出边在两种模式下都是"兜底"，不再在 `AllMatched` 下无条件流出。
- `Join`：按入边计数；`WaitAll` 全部到达才执行，`WaitAny` 首个到达执行、其余到达忽略直至一轮结束。波次结束时仍有未凑齐的 Join 而没有书签 → 实例故障（`workflow.join.dead`）。

## 实例与执行

`WorkflowInstance`：`Id`、`DefinitionId`、`DefinitionName`、`DefinitionVersion`、`DisplayName`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Terminated`）、`Variables`、`JoinStates`、`CorrelationId`、`StarterId`、`ParentInstanceId`、`ParentNodeInstanceId`、`Depth`、`TenantId`、`CreatedAt`、`StartedAt`、`EndedAt`、`Fault`（`Error?`）、`FaultNodeId`、`FaultNodeInstanceId`、`CancellationReason`、`Outcome`。`WorkflowNodeInstance`：`Id`、`InstanceId`、`NodeId`、`ActivityType`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Compensated`）、`Attempt`（从 1 起，重试复用同一记录）、`StartedAt`、`EndedAt`、`Inputs`、`Outputs`、`State`、`Fault`、`CompensatedAt`。

- `StartAsync(WorkflowStartRequest)`：按 `DefinitionId` 或 `Name` + `Version`（缺省最新已发布）找定义，必须 Published（否则 `WORKFLOW_NOTFOUND_001`）；声明的变量缺省补默认值，缺必填 → `WORKFLOW_VALIDATION_002`；深度超过 `MaxSubWorkflowDepth` → `WORKFLOW_VALIDATION_002`。租户取请求的 `TenantId`，缺省取当前调用的租户。`ReusePolicy`（按 `CorrelationId` + 定义名找未结束的实例：`AllowDuplicate` 默认；`RejectActive` → `WORKFLOW_CONFLICT_005`；`ReturnActive` 返回已有实例；没有 `CorrelationId` 时忽略）。`StartAt` 为延迟启动。定义有 `Timeout` 时建 `InstanceTimeout` 书签。实例先写入存储再加锁执行第一个波次。
- **波次**：在实例锁内从队列逐个执行节点直到队列空或实例不再 Running；一个波次执行节点数超过 `MaxNodeExecutionsPerBurst` → 实例故障（`workflow.engine.runaway`）。波次结束：有 `EndInstance` 或没有剩余书签（实例级超时书签不计）且没有未凑齐的 Join → `Completed`；否则实例保持 Running 等待书签。子流程启动与通知父流程在锁释放后进行（避免死锁）。
- **书签**（`WorkflowBookmark`：`Id`、`InstanceId`、`NodeId`、`NodeInstanceId`、`Kind`、`Key`、`Payload`、`DueAt`、`CorrelationId`、`CreatedAt`、`TenantId`）。种类 `BookmarkKinds`：`UserTask`（Key = 指派人）、`Timer`、`Signal`（Key = 信号名）、`SubWorkflow`（Key = 父节点实例 ID）、`Retry`、`NodeTimeout`、`External`（Key = 主题）、`Reminder`（用户任务提醒）、`InstanceTimeout`（实例级超时）、`DefinitionTimer`（定时启动，`InstanceId` = 定义 ID）。`ResumeAsync(bookmarkId, inputs?, expectedKey?)`：消费书签（删除）后恢复节点；实例不是 Running → `WORKFLOW_CONFLICT_003`（`DueBookmarkPoller` 对此类书签把 `DueAt` 后推 `NotResumableBackoff`）；`expectedKey` 与书签 `Key` 不同 → `WORKFLOW_CONFLICT_003`。`Reminder`、`InstanceTimeout`、`DefinitionTimer` 由引擎直接处理，不经活动。`SignalAsync(name, payload?, correlationId?)` 消费当前租户下匹配的全部 Signal 书签（`CorrelationId` 为空的书签接受任意关联），再按触发式启动规则启动实例，返回消费数（含启动数）。
- **故障与重试**：节点失败时按顺序：有重试策略且未用尽 → 节点 Faulted、插入 Retry 书签（`DueAt` = now + `FirstDelay × BackoffFactor^(Attempt−1)`），实例保持 Running；`ContinueOnError` → 节点 Faulted、变量 `lastError` = 错误码与消息、以 `outcome = "error"` 流出；否则实例 Faulted（记录 `Fault`、`FaultNodeId`、`FaultNodeInstanceId`；`CompensateOnFault` 时补偿）。`RetryAsync(ids[])` 只接受 Faulted 且有 `FaultNodeInstanceId` 的实例：清故障、从故障节点重试（`Attempt + 1`）；引擎级故障（死 Join、波次上限）不可重试 → `WORKFLOW_CONFLICT_001`。
- **控制**（批量，逐项结果）：`SuspendAsync(ids[], reason?)`（只允许 Running）、`ResumeInstancesAsync(ids[])`（只允许 Suspended；不重新驱动书签，到期书签由轮询接续）、`CancelAsync(ids[], reason?)`、`TerminateAsync(ids[], reason?)`（Running / Suspended / Faulted 可取消或终止，已结束的幂等返回；清掉非终态节点与全部书签；取消且 `EnableCompensation` 时执行补偿；级联到未结束的子流程并通知父流程）、`RetryAsync(ids[])`、`SetVariablesAsync(updates[])`（Running / Suspended：合并变量、删除指定键）、`JumpAsync(jumps[])`（Running / Suspended / Faulted：取消全部未结束节点与书签、清 Join 状态，从目标节点重新执行；目标不存在 → `WORKFLOW_VALIDATION_002`）、`MigrateAsync(migrations[])`（Running / Suspended 迁到同名的另一个已发布版本：每个未结束节点在目标定义里必须存在且活动类型相同，否则 `WORKFLOW_VALIDATION_002`；只改定义引用，节点实例与书签原样保留）、`PurgeAsync(request)`（删除早于给定时刻结束的实例及其节点实例与书签，可按状态与定义名过滤，返回条数；`XiHan:Workflow:RetentionDays` > 0 时由轮询器每 `HousekeepingIntervalSeconds` 自动执行）。状态不允许 → `WORKFLOW_CONFLICT_001`。单实例的写法是扩展方法。
- **子流程**：`SubWorkflow` / `ForEach` 的子实例带 `ParentInstanceId`、`ParentNodeInstanceId`、`Depth + 1`，继承父实例的租户、`CorrelationId` 与 `StarterId`；子实例到达终态时以 `SubWorkflow` 书签（Key = 父节点实例 ID）回调父实例，输入 `childInstanceId`、`childStatus`、`childVariables`、`childFault`；父节点实例 `State.childInstanceIds` 记录本轮启动的子实例，过期回调忽略。父实例正常完成不级联到不等待的子实例；`End(Outcome)` 与 `Terminate` 级联终止。
- **锁**：`IWorkflowInstanceLock.AcquireAsync(instanceId, ct)` → `Result<IAsyncDisposable>`；等待超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。不可重入。
- **事件**：装配了 `messaging.queue` 时经 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/workflow`，`Type` = 主题，`Data` = `WorkflowEventData` JSON，扩展 `tenant` / `correlation`）；没有发布者时跳过；发布失败只记日志。主题：`workflow.instance.started|completed|faulted|canceled|terminated|suspended|resumed|jumped|migrated|timedout`、`workflow.node.executing|executed|faulted`（`WillRetry`）、`workflow.task.created|completed|transferred|claimed|released|returned|urged|reminder`、`workflow.external.created|completed|failed`、`workflow.custom`（`Subject` = 事件名）。

## 用户任务

- `UserTask` 活动属性：`Assignees`（列表）、`AssigneesExpression`、`AssigneeRules`（`[{ type, values }]`）三者合并去重；`ExcludeStarter`；`EmptyAssigneeStrategy`（`Fault` 默认 / `AutoApprove` / `AutoReject` / `Fallback` + `FallbackAssigneeRules`）；`CompletionPolicy`（`Any` 默认 / `All` / `Sequential`）；`ApprovalThreshold`（`All` 下的通过阈值：人数或百分比如 `"60%"`，缺省全部；通过数达到即通过，剩余人数不可能达到即以 `rejected` 完成）；`Title`（模板，缺省节点名）；`FormData`；`FormKey`；`Priority`；`DueAfterSeconds` / `DueAt`（截止时刻，写进任务视图）；`ReminderAfterSeconds`（每隔该时长发布一次 `workflow.task.reminder`，直至任务结束）；`CcUserIds` / `CcRules`；`AllowedOutcomes`（缺省 `approved` / `rejected`）。
- 挂起时每个指派人一个 `UserTask` 书签（载荷 `title`、`formData`、`formKey`、`priority`、`dueAt`、`ccUserIds`），`Sequential` 一次只有当前指派人的一个；节点 `Timeout` 另插一个 `NodeTimeout` 书签，超时以 `outcome = "timeout"` 完成节点；`ReminderAfterSeconds` 另插 `Reminder` 书签。
- `UserTask`（视图）：`Id`（= 书签 ID）、`InstanceId`、`InstanceTitle`、`DefinitionName`、`NodeId`、`NodeInstanceId`、`Title`、`AssigneeId`、`ClaimedBy`、`Priority`、`DueAt`、`FormKey`、`CorrelationId`、`StarterId`、`FormData`、`CreatedAt`、`TenantId`。
- `IWorkflowTaskService`：`PendingAsync(assigneeId)` / `PendingByInstanceAsync(instanceId)` / `QueryAsync(UserTaskQuery)`（指派人或实例必填其一，可按定义名、节点、创建时间过滤，分页；只含当前租户、实例 Running 的任务）；`CompleteAsync(completions[])`（`UserTaskCompletion`：`TaskId`、`ActorId`、`Outcome`、`Comment`、`Variables`；操作者必须是该书签的指派人 → 否则 `WORKFLOW_DENIED_001`；`Outcome` 不在允许范围 → `WORKFLOW_VALIDATION_002`；`Any` 首个通过即完成节点、`All` 按阈值完成（其间合并变量并保留其余书签）、`Sequential` 逐个；非 `approved` 的结果立即以该结果完成节点）；`TransferAsync(transfers[])`（改书签 `Key`，目标已是指派人 → `WORKFLOW_CONFLICT_004`）；`AddAssigneesAsync(additions[])`（`Any` / `All` 立即加书签，`Sequential` 排到队尾）；`ClaimAsync(claims[])`（`Any` 策略下认领：其他指派人的书签撤下，节点状态记 `claimedBy`；已被认领 → `WORKFLOW_CONFLICT_004`）；`ReleaseAsync(releases[])`（释放认领，恢复其他指派人的书签）；`ReturnAsync(returns[])`（退回：`TargetNodeId` 缺省为本节点之前最近完成的 `UserTask` 节点，没有则退回 `Start` 之后的首个节点；以跳转实现，节点历史记 `returned`）；`UrgeAsync(urges[])`（催办：发布 `workflow.task.urged`，不改状态）。节点 `State` 记 `assignees`、`approved`、`rejected`、`claimedBy`、`history`（`actorId`、`outcome`、`comment`、`at`）。
- 指派规则 `AssigneeRule(Type, Values)`：`IAssigneeProvider`（`RuleType`、`Validate(rule)`、`ResolveAsync(AssigneeResolution)`）按类型登记（`AddAssigneeProvider<T>()`，同类型重复登记是编程错误）；内置 `User`（原样）、`Starter`（实例发起人）、`Expression`（表达式求值为列表或逗号分隔字符串）。解析结果裁剪、去重、保序；未登记的类型 → `WORKFLOW_NOTFOUND_004`；Provider 返回失败即节点故障（不得以空列表掩盖数据不可用）。

## 外部任务

`ExternalTask` 活动以 `External` 书签（Key = 主题，载荷 `topic`、`payload`、`lockedBy`、`lockedUntil`、`notBefore`）挂起。`IWorkflowExternalTaskService`：`FetchAsync(topic, workerId, lockDuration, maxTasks)`（领取当前租户下该主题未锁定（或锁已过期）且已到 `notBefore` 的任务：在实例锁内把 `lockedBy` / `lockedUntil` 写回书签；返回 `ExternalTask` 视图：`Id`、`InstanceId`、`Topic`、`Payload`、`Variables`、`LockedUntil`、`Attempt`）；`CompleteAsync(completions[])`（`taskId`、`workerId`、`outputs`：工作者必须持有未过期的锁 → 否则 `WORKFLOW_DENIED_002`；输出合并进实例变量后节点完成）；`FailAsync(failures[])`（`taskId`、`workerId`、`error`、`retryAfter?`：有 `retryAfter` 时解锁并把 `notBefore` 后推、任务 `attempt + 1`；没有时节点以该错误故障，按节点重试策略处理）；`ExtendLockAsync(extensions[])`。锁到期的任务自动解锁（书签 `DueAt` = `lockedUntil`，到期由轮询驱动、活动重新挂起）。

## 表达式

`IWorkflowExpressionEvaluator`：`Evaluate(expression, scope)` → `Result<JsonNode?>`、`EvaluateCondition` → `Result<bool>`（结果不是 bool 即失败）、`Render(template, scope)` → `Result<string>`（`{{ 表达式 }}` 占位，引号内的 `}}` 不算结束）。默认实现是无依赖的小语言：`||`、`&&`、`== !=`、`< <= > >=`、`+ -`、`* / %`、一元 `! -`、成员 `.x` 与索引 `[i]`、字面量（数字为 decimal、单双引号字符串、`true` / `false` / `null`）、标识符与函数调用；`+` 任一侧是字符串即拼接；`==` 跨数字类型宽松、字符串序数比较；未知根变量是错误（不是 null），缺少的成员或键是 null；函数 `len`、`contains`、`startsWith`、`endsWith`、`upper`、`lower`、`trim`、`isNullOrEmpty`、`abs`、`min`、`max`、`round`、`toNumber`、`toString`、`now`、`date`（名字不区分大小写）。语法或求值失败 → `WORKFLOW_VALIDATION_003`。已解析的表达式按文本缓存，上限 `ExpressionCacheSize`。

## 存储契约

三个存储都不按租户过滤（调用方负责），不做乐观并发（实例锁串行化写入），写入按批量（ADR-0012）：

- `IWorkflowDefinitionStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`FindAsync(name, version)`、`FindLatestPublishedAsync(name)`、`MaxVersionAsync(name)`、`ListAsync(name?, status?)`（名字升序、版本降序）、`SaveAsync(definitions[])`（新增或整体替换）、`DeleteAsync(ids[])`。
- `IWorkflowInstanceStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`ListAsync(WorkflowInstanceQuery)`（`Status` / `Statuses`、`DefinitionName`、`CorrelationId`、`ParentInstanceId`、`StarterId`、`TenantId`、`CreatedFrom` / `CreatedTo`、`EndedBefore`、`Skip`、`Limit` ≤ 1000；创建时间降序）、`CountAsync(query)`、`SaveAsync(instances[])`、`DeleteAsync(ids[])`（级联节点实例）；节点实例 `FindNodeInstanceAsync(id)`、`NodeInstancesAsync(instanceId)`（开始时间升序、同刻按创建顺序）、`SaveNodeInstancesAsync(nodeInstances[])`。
- `IWorkflowBookmarkStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`ByInstanceAsync(instanceId)`、`DueAsync(now, limit)`（`DueAt` ≤ now，`DueAt` 升序）、`ByKindAndKeyAsync(kind, key)`（创建时间升序）、`BySignalAsync(name, correlationId?)`、`SaveAsync(bookmarks[])`、`DeleteAsync(ids[])`、`DeleteByInstanceAsync(instanceIds[])`。
- 存储不可达 → `WORKFLOW_UNAVAILABLE_001`；容量已满（内存默认实现）→ `WORKFLOW_UNAVAILABLE_002`。

## 默认实现与 Provider

- `Workflow.InMemory`（`role: default`）：三个存储是有界内存表（`XiHan:Workflow:InMemory:MaxDefinitions` 默认 1000、`MaxInstances` 10000、`MaxNodeInstances` 100000、`MaxBookmarks` 100000），保存深拷贝；实例锁是按实例的进程内信号量；`DueBookmarkPoller` 由特性在应用进入服务状态后以 `XiHan:Workflow:InMemory:PollIntervalSeconds`（默认 1）轮询，每次取 `BatchSize`（默认 100）条到期书签逐条恢复，停止时等待进行中的波次。
- `Workflow.Database`（`role: provider`，依赖 `persistence.relational@1`）：定义、实例、节点实例、书签以 JSON 文档列加查询列的实体经仓储读写，每个存储操作在自己的管道调用（Transaction 阶段）里提交；实例锁是租约行（`XiHan:Workflow:Database:LockLeaseSeconds` 默认 60，持有期间自动续约，并发戳 CAS 接管过期租约）；轮询只在持有 `workflow.poller` 租约的节点上进行（`PollIntervalSeconds`、`BatchSize` 同上）。
- 引擎选项（`XiHan:Workflow`）：`MaxNodeExecutionsPerBurst`（1000）、`MaxSubWorkflowDepth`（16）、`LockTimeoutSeconds`（10）、`NotResumableBackoffSeconds`（300）、`ExpressionCacheSize`（10000）、`RetentionDays`（0 = 不自动清理）、`HousekeepingIntervalSeconds`（3600）、`HttpTimeoutSeconds`（30）。

## 一致性

Provider 继承 `WorkflowStoreConformance`（三个存储的查询、排序、分页与计数、批量与级联语义、实例锁）与 `WorkflowEngineConformance`（顺序流、决策、并行与汇合、三种完成策略与阈值的用户任务、认领与退回、转办与加签、延时与超时、信号与事件网关、信号与定时启动、子流程与 ForEach、外部任务、HTTP 活动、补偿活动、重试与继续、取消 / 终止 / 挂起 / 恢复 / 跳转 / 迁移 / 改变量、实例级超时、清理、事件主题）并通过全部用例。

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
