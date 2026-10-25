# RFC-0021 工作流：持久化执行

> 状态：已采纳 | 日期：2026-10-26 | 对应 ADR：ADR-0016 | 参考实现：`XiHan.Framework.Workflow.Abstractions`、`XiHan.Framework.Workflow.InMemory` | 一致性：`XiHan.Framework.Workflow.Conformance`

## 摘要

工作流子系统把"一张有向图上的长时间执行"做成与请求同构的调用：定义描述节点、转移与变量；实例是定义的一次执行，状态全部落在存储里；每个节点的活动在新作用域、新上下文里经完整内核管道执行；等待（人工任务、定时、信号、子流程、重试、超时）以书签挂起，书签被消费时从存储恢复执行。引擎与协议性的规则在抽象包里，Provider 只实现三个存储、实例锁与到期书签的轮询驱动。能力 `workflow.execution@1`。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `workflow.execution@1` | 定义目录、实例执行、书签、用户任务；`IWorkflowEngine`、`IWorkflowDefinitionCatalog`、`IWorkflowTaskService` | `Workflow.InMemory`（有界内存存储、进程内实例锁、单进程轮询） |

Provider 实现 `IWorkflowDefinitionStore`、`IWorkflowInstanceStore`、`IWorkflowBookmarkStore` 与 `IWorkflowInstanceLock`，并在应用进入服务状态后驱动 `DueBookmarkPoller`；多实例部署的 Provider（`Workflow.Database`，后续）用分布式锁与领导租约保证同一实例同一时刻只有一个执行者、到期书签只被一个节点轮询。

## 名字与值

- 定义名 `WorkflowName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同；一个定义名下多个版本）。
- 节点 ID、转移 ID：`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`，定义内唯一。
- 活动类型 `ActivityType`：`^[A-Za-z][A-Za-z0-9._-]{0,63}$`；内置活动用 PascalCase 名（`Start`、`UserTask`）。
- 实例、节点实例、书签、任务的 ID 由引擎生成：UUID v7 的 32 位十六进制；存储不解释它们。
- **值模型是 JSON**：变量、节点属性、输入、输出、节点状态与书签载荷都是 `System.Text.Json.Nodes.JsonObject`（值为 `JsonNode?`：null、bool、string、number、数组、对象）。存储以 JSON 文本持久化（`WorkflowJsonContext` 源生成，AOT 可用），内存实现保存深拷贝（读己之写，不泄漏引用）。`WorkflowVariables` 是对 `JsonObject` 的只读视图加类型化读取（`Get<T>`、`TryGet<T>`）。
- 时间一律 `DateTimeOffset`（UTC），取自 `TimeProvider`。租户 ID 是字符串（RFC-0007）。

## 定义

`WorkflowDefinition`（不可变记录）：`Id`、`Name`（`WorkflowName`）、`Version`（≥ 1）、`DisplayName`、`Description`、`Category`、`Status`（`Draft` / `Published` / `Disabled` / `Archived`）、`EnableCompensation`、`Nodes`、`Transitions`、`Variables`、`TenantId`、`CreatedAt`、`PublishedAt`、`Properties`（`JsonObject`，应用扩展）。

- `WorkflowNode`：`Id`、`DisplayName`、`ActivityType`、`Properties`（`JsonObject`，字符串值支持 `{{ 表达式 }}` 模板）、`RetryPolicy`（`MaxAttempts` ≥ 1 含首次、`FirstDelay`、`BackoffFactor` ≥ 1）、`Timeout`（挂起后多久超时）、`ContinueOnError`。
- `WorkflowTransition`：`Id`、`SourceNodeId`、`TargetNodeId`、`Condition`（表达式，空为无条件）、`Priority`（小者先）、`IsDefault`。
- `WorkflowVariableDefinition`：`Name`、`Required`、`DefaultValue`（`JsonNode?`）、`Description`。

**校验**（`WorkflowDefinitionValidator`，发布与启动前执行；失败 → `WORKFLOW_VALIDATION_001`，`details.errors` 列出全部问题）：名字与版本合法；至少一个节点；节点 ID 唯一；每个活动类型已登记；恰好一个 `Start` 且无入边；转移两端存在；条件表达式语法合法；每个节点最多一条默认出边；`Join` 的 `Mode`、`UserTask` 的 `CompletionPolicy` 与 `EmptyAssigneeStrategy` 取值合法；`End` 的 `Outcome` 不为空串；每个 `UserTask` 至少一个指派来源且规则类型已登记；所有节点自 `Start` 可达。

**目录**（`IWorkflowDefinitionCatalog`）：`CreateAsync(definition)`（新版本号 = 同名最大版本 + 1，状态 Draft）、`UpdateAsync(definition)`（只允许 Draft）、`PublishAsync(ids[])`（只允许 Draft，先校验）、`NewVersionAsync(name)`（以最新版本为底复制成 Draft）、`DisableAsync(ids[])`（只允许 Published）、`ArchiveAsync(ids[])`、`DeleteAsync(ids[])`（只允许 Draft）、`FindAsync(id)`、`FindPublishedAsync(name, version?)`、`ListAsync(name?, status?)`。按标识的操作都是批量、逐项结果；状态不允许 → `WORKFLOW_CONFLICT_002`。`WorkflowDefinitionBuilder` 以流式 API 生成定义；`WorkflowDefinitionJson.Serialize / Deserialize` 为定义的 JSON 往返（应用的设计器与存储用）。

## 活动

- `IActivity.ExecuteAsync(ActivityContext, ct)` → `Result<ActivityOutcome>`；`IResumableActivity.ResumeAsync(ActivityResumeContext, ct)`（书签被消费后调用，上下文带书签与恢复输入）；`ICompensableActivity.CompensateAsync(ActivityContext, ct)`（取消且定义启用补偿时按完成顺序逆序调用）。失败的 `Result` 即故障；抛出异常翻译为 `WORKFLOW_UNEXPECTED_001` 的故障。
- `ActivityOutcome`：`Complete(outputs?, outcome?)`、`EndInstance(outputs?, outcome?)`、`Suspend(bookmarks…)`、`CompleteWithChildren(children, outputs?, outcome?)`、`SuspendWithChildren(children, bookmarks…)`。输出合并进实例变量；`outcome` 写进节点输出，并写进实例变量 `outcome`（最近一次带结果的节点），出边条件与后继决策节点都按它选边（没有结果的节点不覆盖）。
- 活动是作用域服务：`services.AddActivity<TActivity>(type, outgoing: ActivityOutgoing.AllMatched)`，登记进 `ActivityRegistry`；重复登记同一类型是编程错误。活动经 `ActivityContext.Services` 解析其他服务，经 `ActivityContext.Invocation` 拿到本次执行的调用上下文。
- **执行方式**：每次节点执行开新作用域，`App.CreateContext` 构造上下文：`xihan.correlation` = 实例 ID，`xihan.tenant` = 实例租户（有则设），经完整管道（认证 → 租户 → 授权 → … → Observe）执行；取消令牌是宿主停止令牌而不是调用方的令牌（调用方的令牌只约束等锁与首次写入）。
- 内置活动（`ActivityTypes`）：`Start`、`End`（可带 `Outcome`：先调用全部 `IWorkflowOutcomeHandler`，再结束实例）、`Terminate`（`Reason`）、`SetVariable`（`Values` 字面量、`Expressions` 表达式）、`Decision`（出边 Exclusive）、`Parallel`（出边 All）、`Join`（`Mode` = `WaitAll` / `WaitAny`）、`Delay`（`Duration` 秒或表达式，Timer 书签）、`UserTask`、`PublishEvent`（`EventName`、`Payload`，发布 `workflow.custom` 消息）、`WaitSignal`（`SignalName`、`AcceptAnyCorrelation`，Signal 书签）、`SubWorkflow`（`DefinitionName`、`DefinitionVersion`、`Variables`、`VariableExpressions`、`WaitForCompletion`、`ResultVariable`、`FailOnChildFault`）、`ForEach`（`Items`、`ItemVariable`、`DefinitionName`、`Parallel`、`FailFast`、`ResultVariable`）、`Log`（`Message`、`Level`）、`Fault`（`Message`）。4.x 的 `Http` 与 `Script` 不进框架（应用按需登记自己的活动）。
- 出边选择（`ActivityOutgoing`）：`AllMatched`（默认）取全部无条件或条件为真的出边，一条都没有时取默认出边；`Exclusive` 按优先级取第一条无条件或条件为真的，没有则默认出边；`All` 取全部；`None` 不流出。`AllMatched` / `Exclusive` 一条也选不到 → 实例故障。与 4.x 的差别：默认出边在两种模式下都是"兜底"，不再在 `AllMatched` 下无条件流出。
- `Join`：按入边计数；`WaitAll` 全部到达才执行，`WaitAny` 首个到达执行、其余到达忽略直至一轮结束。波次结束时仍有未凑齐的 Join 而没有书签 → 实例故障（`workflow.join.dead`）。

## 实例与执行

`WorkflowInstance`：`Id`、`DefinitionId`、`DefinitionName`、`DefinitionVersion`、`DisplayName`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Terminated`）、`Variables`、`JoinStates`、`CorrelationId`、`StarterId`、`ParentInstanceId`、`ParentNodeInstanceId`、`Depth`、`TenantId`、`CreatedAt`、`StartedAt`、`EndedAt`、`Fault`（`Error?`）、`FaultNodeId`、`FaultNodeInstanceId`、`CancellationReason`、`Outcome`。`WorkflowNodeInstance`：`Id`、`InstanceId`、`NodeId`、`ActivityType`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Compensated`）、`Attempt`（从 1 起，重试复用同一记录）、`StartedAt`、`EndedAt`、`Inputs`、`Outputs`、`State`、`Fault`、`CompensatedAt`。

- `StartAsync(WorkflowStartRequest)`：按 `DefinitionId` 或 `Name` + `Version`（缺省最新已发布）找定义，必须 Published（否则 `WORKFLOW_NOTFOUND_001`）；声明的变量缺省补默认值，缺必填 → `WORKFLOW_VALIDATION_002`；深度超过 `MaxSubWorkflowDepth` → `WORKFLOW_VALIDATION_002`。租户取请求的 `TenantId`，缺省取当前调用的租户。实例先写入存储再加锁执行第一个波次。
- **波次**：在实例锁内从队列逐个执行节点直到队列空或实例不再 Running；一个波次执行节点数超过 `MaxNodeExecutionsPerBurst` → 实例故障（`workflow.engine.runaway`）。波次结束：有 `EndInstance` 或没有剩余书签且没有未凑齐的 Join → `Completed`；否则实例保持 Running 等待书签。子流程启动与通知父流程在锁释放后进行（避免死锁）。
- **书签**（`WorkflowBookmark`：`Id`、`InstanceId`、`NodeId`、`NodeInstanceId`、`Kind`、`Key`、`Payload`、`DueAt`、`CorrelationId`、`CreatedAt`、`TenantId`）。种类 `BookmarkKinds`：`UserTask`（Key = 指派人）、`Timer`、`Signal`（Key = 信号名）、`SubWorkflow`（Key = 父节点实例 ID）、`Retry`、`NodeTimeout`。`ResumeAsync(bookmarkId, inputs?, expectedKey?)`：消费书签（删除）后恢复节点；实例不是 Running → `WORKFLOW_CONFLICT_003`（`DueBookmarkPoller` 对此类书签把 `DueAt` 后推 `NotResumableBackoff`）；`expectedKey` 与书签 `Key` 不同 → `WORKFLOW_CONFLICT_003`。`SignalAsync(name, payload?, correlationId?)` 消费当前租户下匹配的全部 Signal 书签（`CorrelationId` 为空的书签接受任意关联），返回消费数。
- **故障与重试**：节点失败时按顺序：有重试策略且未用尽 → 节点 Faulted、插入 Retry 书签（`DueAt` = now + `FirstDelay × BackoffFactor^(Attempt−1)`），实例保持 Running；`ContinueOnError` → 节点 Faulted、变量 `lastError` = 错误码与消息、以 `outcome = "error"` 流出；否则实例 Faulted（记录 `Fault`、`FaultNodeId`、`FaultNodeInstanceId`）。`RetryAsync(ids[])` 只接受 Faulted 且有 `FaultNodeInstanceId` 的实例：清故障、从故障节点重试（`Attempt + 1`）；引擎级故障（死 Join、波次上限）不可重试 → `WORKFLOW_CONFLICT_001`。
- **控制**（批量，逐项结果）：`SuspendAsync(ids[], reason?)`（只允许 Running）、`ResumeInstancesAsync(ids[])`（只允许 Suspended；不重新驱动书签，到期书签由轮询接续）、`CancelAsync(ids[], reason?)`、`TerminateAsync(ids[], reason?)`（Running / Suspended / Faulted 可取消或终止，已结束的幂等返回；清掉非终态节点与全部书签；取消且 `EnableCompensation` 时执行补偿；级联到未结束的子流程并通知父流程）、`RetryAsync(ids[])`。状态不允许 → `WORKFLOW_CONFLICT_001`。单实例的写法是扩展方法。
- **子流程**：`SubWorkflow` / `ForEach` 的子实例带 `ParentInstanceId`、`ParentNodeInstanceId`、`Depth + 1`，继承父实例的租户、`CorrelationId` 与 `StarterId`；子实例到达终态时以 `SubWorkflow` 书签（Key = 父节点实例 ID）回调父实例，输入 `childInstanceId`、`childStatus`、`childVariables`、`childFault`；父节点实例 `State.childInstanceIds` 记录本轮启动的子实例，过期回调忽略。父实例正常完成不级联到不等待的子实例；`End(Outcome)` 与 `Terminate` 级联终止。
- **锁**：`IWorkflowInstanceLock.AcquireAsync(instanceId, ct)` → `Result<IAsyncDisposable>`；等待超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。不可重入。
- **事件**：装配了 `messaging.queue` 时经 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/workflow`，`Type` = 主题，`Data` = `WorkflowEventData` JSON，扩展 `tenant` / `correlation`）；没有发布者时跳过；发布失败只记日志。主题：`workflow.instance.started|completed|faulted|canceled|terminated|suspended|resumed`、`workflow.node.executing|executed|faulted`（`WillRetry`）、`workflow.task.created|completed|transferred`、`workflow.custom`（`Subject` = 事件名）。

## 用户任务

- `UserTask` 活动属性：`Assignees`（列表）、`AssigneesExpression`、`AssigneeRules`（`[{ type, values }]`）三者合并去重；`ExcludeStarter`；`EmptyAssigneeStrategy`（`Fault` 默认 / `AutoApprove` / `AutoReject` / `Fallback` + `FallbackAssigneeRules`）；`CompletionPolicy`（`Any` 默认 / `All` / `Sequential`）；`Title`（模板，缺省节点名）；`FormData`；`CcUserIds` / `CcRules`；`AllowedOutcomes`（缺省 `approved` / `rejected`）。
- 挂起时每个指派人一个 `UserTask` 书签（载荷 `title`、`formData`、`ccUserIds`），`Sequential` 一次只有当前指派人的一个；节点 `Timeout` 另插一个 `NodeTimeout` 书签，超时以 `outcome = "timeout"` 完成节点。
- `UserTask`（视图）：`Id`（= 书签 ID）、`InstanceId`、`InstanceDisplayName`、`DefinitionName`、`NodeId`、`NodeInstanceId`、`Title`、`AssigneeId`、`CorrelationId`、`FormData`、`CreatedAt`、`TenantId`。
- `IWorkflowTaskService`：`PendingAsync(assigneeId)` / `PendingByInstanceAsync(instanceId)`（只含当前租户、实例 Running 的任务）；`CompleteAsync(completions[])`（`UserTaskCompletion`：`TaskId`、`ActorId`、`Outcome`、`Comment`、`Variables`；操作者必须是该书签的指派人 → 否则 `WORKFLOW_DENIED_001`；`Outcome` 不在允许范围 → `WORKFLOW_VALIDATION_002`；`Any` 首个通过即完成节点、`All` 全部通过才完成（其间合并变量并保留其余书签）、`Sequential` 逐个；非 `approved` 的结果立即以该结果完成节点）；`TransferAsync(transfers[])`（改书签 `Key`，目标已是指派人 → `WORKFLOW_CONFLICT_004`）；`AddAssigneesAsync(additions[])`（`Any` / `All` 立即加书签，`Sequential` 排到队尾）。节点 `State` 记 `assignees`、`approved`、`history`（`actorId`、`outcome`、`comment`、`at`）。
- 指派规则 `AssigneeRule(Type, Values)`：`IAssigneeProvider`（`RuleType`、`Validate(rule)`、`ResolveAsync(AssigneeResolution)`）按类型登记（`AddAssigneeProvider<T>()`，同类型重复登记是编程错误）；内置 `User`（原样）、`Starter`（实例发起人）、`Expression`（表达式求值为列表或逗号分隔字符串）。解析结果裁剪、去重、保序；未登记的类型 → `WORKFLOW_NOTFOUND_004`；Provider 返回失败即节点故障（不得以空列表掩盖数据不可用）。

## 表达式

`IWorkflowExpressionEvaluator`：`Evaluate(expression, scope)` → `Result<JsonNode?>`、`EvaluateCondition` → `Result<bool>`（结果不是 bool 即失败）、`Render(template, scope)` → `Result<string>`（`{{ 表达式 }}` 占位，引号内的 `}}` 不算结束）。默认实现是无依赖的小语言：`||`、`&&`、`== !=`、`< <= > >=`、`+ -`、`* / %`、一元 `! -`、成员 `.x` 与索引 `[i]`、字面量（数字为 decimal、单双引号字符串、`true` / `false` / `null`）、标识符与函数调用；`+` 任一侧是字符串即拼接；`==` 跨数字类型宽松、字符串序数比较；未知根变量是错误（不是 null），缺少的成员或键是 null；函数 `len`、`contains`、`startsWith`、`endsWith`、`upper`、`lower`、`trim`、`isNullOrEmpty`、`abs`、`min`、`max`、`round`、`toNumber`、`toString`、`now`、`date`（名字不区分大小写）。语法或求值失败 → `WORKFLOW_VALIDATION_003`。已解析的表达式按文本缓存，上限 `ExpressionCacheSize`。

## 存储契约

三个存储都不按租户过滤（调用方负责），不做乐观并发（实例锁串行化写入），写入按批量（ADR-0012）：

- `IWorkflowDefinitionStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`FindAsync(name, version)`、`FindLatestPublishedAsync(name)`、`MaxVersionAsync(name)`、`ListAsync(name?, status?)`（名字升序、版本降序）、`SaveAsync(definitions[])`（新增或整体替换）、`DeleteAsync(ids[])`。
- `IWorkflowInstanceStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`ListAsync(WorkflowInstanceQuery)`（`Status`、`DefinitionName`、`CorrelationId`、`ParentInstanceId`、`Limit` ≤ 1000；创建时间降序）、`SaveAsync(instances[])`、`DeleteAsync(ids[])`（级联节点实例）；节点实例 `FindNodeInstanceAsync(id)`、`NodeInstancesAsync(instanceId)`（开始时间升序、同刻按创建顺序）、`SaveNodeInstancesAsync(nodeInstances[])`。
- `IWorkflowBookmarkStore`：`FindAsync(id)`、`FindManyAsync(ids[])`、`ByInstanceAsync(instanceId)`、`ByNodeInstanceAsync(nodeInstanceId)`、`DueAsync(now, limit)`（`DueAt` ≤ now，`DueAt` 升序）、`ByKindAndKeyAsync(kind, key)`（创建时间升序）、`BySignalAsync(name, correlationId?)`、`SaveAsync(bookmarks[])`、`DeleteAsync(ids[])`、`DeleteByInstanceAsync(instanceIds[])`。
- 存储不可达 → `WORKFLOW_UNAVAILABLE_001`；容量已满（内存默认实现）→ `WORKFLOW_UNAVAILABLE_002`。

## 默认实现

`Workflow.InMemory`（`role: default`）：三个存储是有界内存表（`XiHan:Workflow:InMemory:MaxDefinitions` 默认 1000、`MaxInstances` 10000、`MaxNodeInstances` 100000、`MaxBookmarks` 100000），保存深拷贝；实例锁是按实例的进程内信号量；`DueBookmarkPoller` 由特性在应用进入服务状态后以 `XiHan:Workflow:InMemory:PollIntervalSeconds`（默认 1）轮询，每次取 `BatchSize`（默认 100）条到期书签逐条恢复，停止时等待进行中的波次。引擎选项（`XiHan:Workflow`）：`MaxNodeExecutionsPerBurst`（1000）、`MaxSubWorkflowDepth`（16）、`LockTimeoutSeconds`（10）、`NotResumableBackoffSeconds`（300）、`ExpressionCacheSize`（10000）。

## 一致性

Provider 继承 `WorkflowStoreConformance`（三个存储的查询、排序、批量与级联语义）与 `WorkflowEngineConformance`（顺序流、决策、并行与汇合、三种完成策略的用户任务、转办与加签、延时与超时、信号、子流程与 ForEach、重试与继续、取消 / 终止 / 挂起 / 恢复、事件主题）并通过全部用例。

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
| `Http` / `Script` 活动 | 不提供 |
| 默认出边在 `AllMatched` 下总是流出 | 默认出边只作兜底 |
