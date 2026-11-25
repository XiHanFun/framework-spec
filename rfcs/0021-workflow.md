# RFC-0021 工作流：持久化执行

> 状态：已采纳 | 日期：2026-10-26，同日修订（ADR-0020：`Workflow.Database`、触发式启动、事件网关、外部任务、HTTP 活动、补偿活动、跳转与迁移、认领 / 退回 / 催办、提醒与审批阈值、保留期清理；2026-11-01 修订：外部任务领取的条数上限与下推、重用策略查重与创建原子、定义按租户隔离、实例按租户隔离；2026-11-01 修订：引擎写入随调用方事务、分区隔离与随租户分库（平台与租户互不可见，取消全局定义），待办查询的过滤、排序与分页下推到存储并带总数；2026-11-01 修订：只读调用里的写入是 500 类缺陷；2026-11-01 修订：实例查询与清理的分页越界返回校验错误；2026-11-01 修订：租约表可以指定命名连接，SQLite 上放到另一个库文件；2026-11-01 修订：应用不在服务状态时不持有轮询领导权；2026-11-01 修订：EF Core 上的租约表连接；2026-11-01 修订：轮询周期中途进入排空即停下、不告警；2026-11-02 修订：HTTP 活动的超时按引擎时钟计时、按令牌归因；2026-11-02 修订：定义列表按关键字、名字、分类与状态在存储里分页并带总数，返回摘要；实例查询可给一组关联 ID、只取顶层与标题关键字；待办查询按任务标题或实例标题的关键字过滤；引擎按实例列出全部等待点的摘要；一致性套件在代理一侧记录事件，装了发件箱的宿主照样通过；2026-11-02 修订：定义的关键字匹配名字、显示名与说明，实例另匹配关联 ID，待办另匹配定义名与关联 ID；2026-11-02 修订：内存实现同样在只读调用里拒绝写入；2026-11-02 修订：关键字只做小写折叠，`%`、`_`、`[`、`\` 不再换成全角；2026-11-02 修订：关键字查询列改名定为已定；2026-11-02 修订：兼容库上的关键字随持久化维持按通配匹配定为已定；2026-11-02 修订：EF Core 在 SQLite 上的包含同样译成 `LIKE`，小写折叠的理由随之改写；2026-11-02 修订：定义与实例查询可选排序字段与方向（白名单），定义查询可按创建时间区间过滤；2026-11-02 修订：按状态排序按生命周期的先后；2026-11-03 修订：定义与实例查询支持多字段排序，`SortBy` / `Descending` 换成有序的排序项列表 `Sorts`；2026-11-03 修订：故障原因用消息键与参数表示，按调用的区域本地化；2026-11-03 修订：定义校验与表达式的诊断、`lastError` 变量、活动描述符与状态名也用消息键与参数；2026-11-03 再修订：定义 JSON 里模型会拒绝的值在读入时按同一份规则核对；2026-11-03 修订：定义 JSON 里没写的成员取模型的初值；2026-11-03 修订：一致性套件的记录器只解析工作流事件，宿主里别的发布端的消息照常确认；2026-11-04 修订：活动抛出与 HTTP 连接失败的 `reason` 只写异常类型；同日再修订：`Http` 活动的缺省客户端过出站地址策略，新增消息键 `workflow.http.address_denied`，一致性用例 42；同日再修订：表达式的长度与嵌套上限；2026-11-12 修订：`Http` 活动的正文读到上限即停，`TimeoutSeconds` 不超过引擎选项 `MaxHttpTimeoutSeconds`；同日再修订：`ForEach` 的项数、实例变量的大小与 `+` 拼出的字符串有上限，级联覆盖全部子实例；2026-11-24 修订：一致性向量；同日再修订：表达式小语言的词法、文法、每个运算符与函数的语义、数字与时间的文本、模板、上限与错误逐条写明，数字的文本、时间、容器的文本、求值的先后与溢出等处修订，新增消息键 `workflow.expression.number_out_of_range`，表达式的一致性向量；2026-11-25 修订：表达式新增内置函数 `format(x, n)`；同日再修订：一次求值的工作量上限 16777216 与计数规则，新增消息键 `workflow.expression.budget_exceeded`；同日第三次修订：关键字的折叠写明是逐个码位的 Unicode 简单小写映射（`İ` 折成 `i`，平面以外的字母同样折叠）；同日第四次修订：`Delay.Until`、`UserTask.DueAt` 与按时间读的值改用表达式的时间文法；同日第五次修订：表达式的 `upper` / `lower` 改成逐个码位的 Unicode 简单大小写映射、与关键字的折叠同一份映射（以前 U+0130 与 U+0131 实现自定），关键字的折叠与 `upper` / `lower` 写明结果以实现所带的 Unicode 数据为准、向量只用早已分配的字符；同日第六次修订：工作量里读一个数字按它的写法的长度计（变量里的是 JSON 文本里的原文，以前是 0），表达式得出的数字写成 JSON 时写数字的文本；同日第七次修订：`Workflow.Database` 写明查询列的回填——`DatabaseWorkflowBackfill` 按分区分批从文档重新算出查询列，可以重复运行，实例行与书签行在实例锁内改写；2026-11-26 修订：工作量里 `len` 读是字符串的实参（以前只读数字），`isNullOrEmpty` 只读到能判定为止——非空的字符串记 1，空串、数字、布尔与容器不读（以前读数字的全文、不读字符串）） | 对应 ADR：ADR-0182、ADR-0181、ADR-0120、ADR-0117、ADR-0016、ADR-0020、ADR-0066、ADR-0077、ADR-0081、ADR-0082、ADR-0084、ADR-0087、ADR-0096、ADR-0097、ADR-0099、ADR-0101、ADR-0102、ADR-0103、ADR-0104、ADR-0105、ADR-0106、ADR-0107、ADR-0115、ADR-0277（一致性向量）、ADR-0287（表达式）、ADR-0296（`format`、工作量、关键字的小写、活动里的时间）、ADR-0303（`upper` / `lower` 的大小写映射、Unicode 数据的依据、读数字的工作量）、ADR-0304（`Workflow.Database` 的查询列回填）、ADR-0309（`len` 与 `isNullOrEmpty` 的工作量） | 参考实现：`XiHan.Framework.Workflow.Abstractions`、`XiHan.Framework.Workflow.InMemory`、`XiHan.Framework.Workflow.Database` | 一致性：`XiHan.Framework.Workflow.Conformance`，向量 `conformance/vectors/workflow/` | 一致性档：`workflow`

## 摘要

工作流子系统把"一张有向图上的长时间执行"做成与请求同构的调用：定义描述节点、转移与变量；实例是定义的一次执行，状态全部落在存储里；每个节点的活动在新作用域、新上下文里经完整内核管道执行；等待（人工任务、外部任务、定时、信号、子流程、重试、超时）以书签挂起，书签被消费时从存储恢复执行。引擎与协议性的规则在抽象包里，Provider 只实现三个存储、实例锁与到期书签的轮询驱动。能力 `workflow.execution@1`。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `workflow.execution@1` | 定义目录、实例执行、书签、用户任务、外部任务；`IWorkflowEngine`、`IWorkflowDefinitionCatalog`、`IWorkflowTaskService`、`IWorkflowExternalTaskService` | `Workflow.InMemory`（有界内存存储、进程内实例锁、单进程轮询） |

Provider 实现 `IWorkflowDefinitionStore`、`IWorkflowInstanceStore`、`IWorkflowBookmarkStore`（作用域服务，按分区读写）、`IWorkflowTransaction`（作用域服务，与调用方事务的关系）与 `IWorkflowInstanceLock`，并在应用进入服务状态后驱动 `DueBookmarkPoller`——循环可以在特性启动时就起，但每个周期（`RunCycleAsync`）先看应用是否在服务状态，启动中与排空中的周期不轮询、不清理、不记告警（管道这时不接受调用，逐个分区按不可达告警只是噪声；2026-11-01 修订，ADR-0084）；周期开始之后才进入排空的，下一个失败的分区调用（扫描、恢复或清理）发现应用已不在服务状态就停下这个周期，剩下的分区不再调用、这些失败不记告警（2026-11-01 修订，ADR-0087）。手动调用的 `PollOnceAsync`、`HousekeepAsync` 不在周期里，失败照实告警。`Workflow.Database`（依赖 `persistence.relational@1`，可选依赖 `multitenancy.context@1`）在调用方的工作单元里读写、租户的记录随租户分库，用平台库里的租约行做分布式实例锁，多实例同时只有一个执行者；到期书签由持有轮询租约的节点逐个分区轮询（其他节点同时轮询也只是无效的重复尝试）；领导权只在应用处于服务状态时拿与续，启动中与排空中的周期不拿也不续，持有就立即释放（2026-11-01 修订，ADR-0087）。

## 名字与值

- 定义名 `WorkflowName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同；一个定义名下多个版本；名字与版本号在分区里唯一，见「分区」）。
- 节点 ID、转移 ID：`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`，定义内唯一。
- 活动类型 `ActivityType`：`^[A-Za-z][A-Za-z0-9._-]{0,63}$`；内置活动用 PascalCase 名（`Start`、`UserTask`）。
- 实例、节点实例、书签、任务的 ID 由引擎生成：UUID v7 的 32 位十六进制；存储不解释它们。
- **值模型是 JSON**：变量、节点属性、输入、输出、节点状态与书签载荷都是 `System.Text.Json.Nodes.JsonObject`（值为 `JsonNode?`：null、bool、string、number、数组、对象）。存储以 JSON 文本持久化（`WorkflowJsonContext` 源生成，AOT 可用），内存实现保存深拷贝（读己之写，不泄漏引用）。`WorkflowVariables` 是对 `JsonObject` 的只读视图加类型化读取（按名的索引器与 `TryGet<T>`，没有、值为 JSON null 或转换失败时返回 false）。
- 时间一律 `DateTimeOffset`（UTC），取自 `TimeProvider`。租户 ID 是字符串（RFC-0007）。

## 定义

`WorkflowDefinition`（不可变记录）：`Id`、`Name`（`WorkflowName`）、`Version`（≥ 1）、`DisplayName`、`Description`、`Category`（最多 `MaxCategoryLength` = 128 个字符，与数据库实现的分类列同宽；超长构造即抛 `ArgumentException`，`WorkflowJson.DeserializeDefinition` 把它转成 `WORKFLOW_VALIDATION_001`；2026-11-02 修订，ADR-0096）、`Status`（`Draft` / `Published` / `Disabled` / `Archived`）、`EnableCompensation`（取消时补偿）、`CompensateOnFault`（故障时也补偿）、`Timeout`（实例级超时：启动后超过即按 `TimeoutAction` 处理）、`TimeoutAction`（`Fault` 默认 / `Terminate`）、`Nodes`、`Transitions`、`Variables`、`TenantId`、`CreatedAt`、`PublishedAt`、`Properties`（`JsonObject`，应用扩展）。

- `WorkflowNode`：`Id`、`DisplayName`、`ActivityType`、`Properties`（`JsonObject`，字符串值支持 `{{ 表达式 }}` 模板）、`RetryPolicy`（`MaxAttempts` ≥ 1 含首次、`FirstDelay`、`BackoffFactor` ≥ 1）、`Timeout`（挂起后多久超时）、`ContinueOnError`、`OutputMappings`（变量名 → 表达式：节点完成后以 `outputs` 为作用域求值并写入实例变量，Camunda 的输出映射）。
- `WorkflowTransition`：`Id`、`SourceNodeId`、`TargetNodeId`、`Condition`（表达式，空为无条件）、`Priority`（小者先）、`IsDefault`。
- `WorkflowVariableDefinition`：`Name`、`Required`、`DefaultValue`（`JsonNode?`）、`Description`。
- **触发式启动**：`Start` 节点属性 `SignalName`（信号启动：`SignalAsync(name, payload, correlationId)` 在恢复书签之后，为调用方分区里每个 `Start.SignalName` 等于该名字的已发布定义启动一个实例，变量 `signal` = 载荷，关联 ID = 信号的关联 ID）、`Cron`（定时启动，`CronExpression` 文法，可带 `TimeZone`：发布时创建定义级 `DefinitionTimer` 书签（`InstanceId` = 定义 ID），到期由轮询驱动启动一个实例并推算下一次；禁用、归档、删除定义时删除该书签）。

**校验**（`WorkflowDefinitionValidator`，发布与启动前执行；失败 → `WORKFLOW_VALIDATION_001`，消息键 `workflow.definition.invalid`（参数 `count`），`details.errors` 是全部问题，每条一个 `SchemaViolation`：`Path` 是定义 JSON 里的位置（`$.nodes[2]`、`$.nodes[2].properties.Cron`、`$.nodes[2].activityType`、`$.transitions[0].condition`、`$.transitions[0].sourceNodeId`，定义级的问题是 `$.nodes`），`MessageKey` 与 `Arguments` 是这条问题的消息键与参数，`Reason` 是按进程默认区域渲染的文本；宿主把它写成 Problem Details 的字段级 `errors` 并按调用方的区域翻译，进程内用 `Diagnose(definition)` 取同一组问题；2026-11-03 修订，ADR-0106）：名字与版本合法；至少一个节点；节点 ID 唯一；每个活动类型已登记；恰好一个 `Start` 且无入边；转移两端存在；条件表达式与输出映射表达式语法合法；每个节点最多一条默认出边；`Join` 的 `Mode`、`UserTask` 的 `CompletionPolicy`、`EmptyAssigneeStrategy` 与 `ApprovalThreshold` 取值合法；`End` 的 `Outcome` 不为空串；每个 `UserTask` 至少一个指派来源且规则类型已登记；`Start.Cron` 是合法 cron；`EventGateway` 至少一个信号；`Http` 的 `Url` 非空、`Method` 合法；所有节点自 `Start` 可达。

**分区**（ADR-0077，推翻 ADR-0066 第 5、6 节的全局定义与"无租户的调用看得到全部"）：工作流的数据分成平台分区与每个租户一个分区。一次调用只在自己的分区里读写——分区是当前调用的租户（RFC-0007），无租户的调用（平台）是平台分区。定义、实例、节点实例与书签都属于一个分区（`TenantId` 为分区的租户，平台为空），之后不变。

- 看得到、改得动的只有自己分区的：租户看不到平台与别的租户的定义、实例、书签，平台也看不到任何租户的。看不到的一律按不存在处理（定义 `WORKFLOW_NOTFOUND_001`、实例 `WORKFLOW_NOTFOUND_002`、书签 `WORKFLOW_NOTFOUND_003`），不泄露别的分区有没有这条记录。没有全局定义：平台建的定义对租户不可见，各租户共用的流程模板要在每个租户里各建一份（应用的种子负责）。
- 名字与版本号在分区里唯一：(`TenantId`, `Name`, `Version`) 唯一，平台与各租户的同名定义各自从版本 1 编号。按名字解析（启动、`FindPublishedAsync`、子流程与 ForEach、信号启动）只在调用方的分区里找，找不到不退回别的分区。
- 不能跨分区写：建定义时定义的 `TenantId` 给了别的租户（平台写租户、租户写平台或别的租户）→ `WORKFLOW_NOTFOUND_001`；启动请求没有租户字段，实例总是属于调用方的分区。
- 随租户分库：数据库实现里租户分区的记录与业务实体一样经持久化的多租户与分片路由落到租户的库（RFC-0005、RFC-0007），平台分区的记录在平台库，见「默认实现与 Provider」。

**目录**（`IWorkflowDefinitionCatalog`）：`CreateAsync(definition)`（定义属于调用方的分区；新版本号 = 分区里同名最大版本 + 1，状态 Draft）、`UpdateAsync(definition)`（只允许 Draft）、`PublishAsync(ids[])`（只允许 Draft，先校验；带 `Start.Cron` 的建定义级定时书签）、`NewVersionAsync(name)`（以调用方分区里的最新版本为底复制成 Draft，新草稿留在同一分区；没有 → `WORKFLOW_NOTFOUND_001`）、`DisableAsync(ids[])`（只允许 Published）、`ArchiveAsync(ids[])`、`DeleteAsync(ids[])`（只允许 Draft）、`FindAsync(id)`、`FindPublishedAsync(name, version?)`（按名字在调用方分区里解析）、`QueryAsync(WorkflowDefinitionQuery)` → `WorkflowDefinitionPage(Items, Total)`（见下文「定义列表」）、`ActivityTypes()`（已登记活动的描述符：类型、显示名、分类、输入属性，供设计器使用；显示名、分类显示名、说明与输入说明是消息键，按调用方的区域渲染后返回，2026-11-03 修订，ADR-0106）。按标识的操作都是批量、逐项结果；看不到或改不动 → `WORKFLOW_NOTFOUND_001`，状态不允许 → `WORKFLOW_CONFLICT_002`。`WorkflowDefinitionBuilder` 以流式 API 生成定义；`WorkflowJson.Serialize(definition)` / `WorkflowJson.DeserializeDefinition(json) → Result<WorkflowDefinition>` 为定义的 JSON 往返（应用的设计器与存储用；`WorkflowJson` 同样提供实例、节点实例与书签的往返与深拷贝）。定义 JSON 的必填成员只有定义的 `id`、`name`，节点的 `id`、`activityType`，转移的 `id`、`sourceNodeId`、`targetNodeId`，变量的 `name`；其余成员可省略，读入时取模型的初值，与直接构造模型一致（`version` 1、重试策略 `maxAttempts` 1、`firstDelay` 10 秒、`backoffFactor` 2 等；写成 `null` 与不写相同）；写出时写出全部非空成员，等于初值的也写出，往返稳定（2026-11-03 修订，ADR-0107）。

**定义列表**（2026-11-02 修订，ADR-0096）：`QueryAsync(WorkflowDefinitionQuery)` 只列调用方分区的定义，返回这一页的摘要 `WorkflowDefinitionSummary`（`Id`、`Name`、`Version`、`DisplayName`、`Description`、`Category`、`Status`、`TenantId`、`CreatedAt`、`PublishedAt`，以及显示名或名字 `Title`；不含节点、转移、变量与扩展属性，要整份定义按 ID 调 `FindAsync`）与同条件的总数 `Total`。`WorkflowDefinitionQuery`：`Keyword`（匹配名字、显示名或说明，规则见下文「关键字」）、`Name`、`Category`（精确匹配）、`Status`、`CreatedFrom` / `CreatedTo`（创建时间，下界含、上界不含；2026-11-02 修订，ADR-0102），同时给出的取交集；排序见下文「列表排序」，缺省名字升序、版本降序；`Skip` ≥ 0、`Take` 取 1 到 `MaxTake`（1000，与持久化的查询条数上限一致），缺省 0 与 100，越界不在构造时抛出，`QueryAsync` 返回 `WORKFLOW_VALIDATION_002`（HTTP 400，消息键 `workflow.definition.query_page_invalid`，参数 `skip`、`take`、`max`），不进存储也不开事务。过滤、排序、分页与计数都由存储执行（`IWorkflowDefinitionStore.ListSummariesAsync` / `CountAsync`，数据库实现是 SQL），这一页与总数在同一次调用里读。

**列表排序**（2026-11-02 修订，ADR-0102；2026-11-03 修订，ADR-0104：多字段）：定义查询与实例查询各有一个有序的排序项列表 `Sorts`（`WorkflowDefinitionSort` / `WorkflowInstanceSort`，每项是白名单里的字段 `Field` 加方向 `Descending`，缺省升序），第一项是主排序字段，后面的依次是次级排序字段（列表里按住 Shift 点列追加次级排序）。排序在存储里执行（数据库实现是 SQL 的 `ORDER BY`，各项按列表的顺序），排序项之后再接一串固定的次序键补成全序，同值的行在各页之间不重不漏。

| 查询 | 排序项的字段（白名单） | `Sorts` 为空或没给时（缺省） | 次序键（接在全部排序项之后，排序项里已有的字段跳过） | 项数上限 |
| --- | --- | --- | --- | --- |
| `WorkflowDefinitionQuery` | `Name`（名字）、`Title`（显示名，没有取名字）、`Version`、`Status`、`CreatedAt`、`PublishedAt` | 名字升序（再接次序键：版本降序），与原来相同 | 名字升序、版本降序；数据库实现最后再按 ID 升序 | `MaxSorts` = 6 |
| `WorkflowInstanceQuery` | `CreatedAt`、`EndedAt`、`Status`、`DefinitionName`、`Title`（实例标题） | 创建时间降序，与原来相同 | 创建时间降序、写入先后降序、ID 降序；排序项里有 `CreatedAt` 时，写入先后与 ID 随它的方向 | `MaxSorts` = 5 |

- 项数上限等于可排序字段的个数：每个字段最多出现一次。
- 非法值，按下面的顺序检查、遇到第一个就返回，`QueryAsync` / `ListInstancesAsync` 返回 `WORKFLOW_VALIDATION_002`（HTTP 400），不进存储：项数超过 `MaxSorts`（消息键 `workflow.definition.query_sorts_too_many` / `workflow.instance.query_sorts_too_many`，参数 `count`、`max`）；某一项为空或字段不是白名单里的值——枚举越界，如请求里给了数字 99（`workflow.definition.query_sort_invalid` / `workflow.instance.query_sort_invalid`，参数 `sort`）；同一字段出现两次（`workflow.definition.query_sort_duplicate` / `workflow.instance.query_sort_duplicate`，参数 `sort`，不论两次的方向是否相同）。`CountInstancesAsync` 不看排序、不校验（与分页一样）。存储收到不合法的排序项抛出 `ArgumentException` / `ArgumentOutOfRangeException`（调用方已校验，是编程错误）。
- 文本字段（名字、标题、定义名）按数据库列的排序规则比较，内存实现按序数比较：大小写、标点与非拉丁文字的先后可能随库不同（与原来的名字排序一样）。
- 状态按生命周期的先后比较（2026-11-02 修订，ADR-0103）：定义 `Draft` → `Published` → `Disabled` → `Archived`；实例 `Running` → `Suspended` → `Completed` → `Canceled` → `Faulted` → `Terminated`。升序是这个顺序，降序反过来；同状态的按后面的排序项与次序键。这个顺序就是两个状态枚举的声明顺序，以后新增状态要在 RFC 里定它的位置。序号由抽象包的 `WorkflowStatusOrder.Of(status)` 公开一份，各存储共用，新增状态只改这一处（2026-11-11 修订，ADR-0162）。
- 可空的时间（`PublishedAt`、`EndedAt`）：没有值的行的位置照数据库——PostgreSQL、Oracle 当作最大（升序在后、降序在前），SQLite、MySQL、SQL Server 当作最小；内存实现当作最小。持久化的排序没有统一空值位置的选项，有值的行之间的先后各实现相同。
- 标题排序按标题的前 `WorkflowSortKeys.MaxTitleLength`（256）个字符（`WorkflowSortKeys.Title`，不拆开代理对），前 256 个字符相同的按后面的排序项与次序键；数据库实现把它存进 `Title` 列。
- 多字段：排序项的个数与组合不限于界面上的点法（主次字段、各自的方向任意组合）；没给到的字段由固定的次序键决定先后。

**关键字**（`WorkflowKeyword`，列表查询的关键字都按这套规则；2026-11-02 修订，ADR-0096、ADR-0099）：关键字去掉首尾空白，为空或只有空白不过滤；关键字与被搜的文本都先折叠——逐个码位取小写（见下文「关键字的小写」），字段分隔符 U+001F 换成空格——再按序数判断包含，任一被搜字段包含即匹配，不跨字段匹配。关键字里的 `%`、`_`、`[`、`\` 按字面匹配，与全角的 `％`、`＿`、`［`、`＼` 是不同的字符：按字面由持久化保证（RFC-0005「规约里的字符串匹配」：`Contains` 是字面匹配，SqlSugar 在 SQLite、PostgreSQL、MySQL、SQL Server、Oracle 上译成带转义的 `LIKE`，EF Core 按字面，在 SQLite 上同样译成带转义的 `LIKE`），内存实现按序数包含。小写折叠要保留：内存实现的序数比较与 PostgreSQL、Oracle 上的匹配区分大小写，SQLite 上（EF Core 与 SqlSugar 都译成 `LIKE`，2026-11-02 修订，ADR-0101）只对 ASCII 不区分，MySQL 与 SQL Server 按排序规则，两边都折叠成小写，各 Provider 与内存实现的结果才一致；数据库的排序规则另有放宽（如 MySQL 缺省排序规则不区分重音）时以排序规则为准。数据库实现把被搜文本的折叠结果存进查询列（多个字段各自折叠后以 U+001F 拼成一列，`WorkflowKeyword.Join`），SQL 里对它做包含判断。持久化还没保证字面匹配的库上（SqlSugar 在 MySQL、PostgreSQL、Oracle 的兼容库与其余数据库上沿用不转义的 `LIKE`，ADR-0098），关键字里的 `%`、`_` 照那个库的 `LIKE` 当通配，PostgreSQL 系的兼容库上以 `\` 结尾的关键字可能报错——与应用自己实体的 `Contains` 查询相同。这是已定的（2026-11-02，ADR-0101）：这些库在真实库上验证之后由持久化加进名单，工作流不另做绕法。三处查询各自匹配的字段（2026-11-02 修订，ADR-0099）：

| 查询 | 匹配的字段 |
| --- | --- |
| 定义列表 `WorkflowDefinitionQuery` | 名字、显示名、说明 |
| 实例列表 `WorkflowInstanceQuery` | 实例标题、关联 ID |
| 待办 `UserTaskQuery` | 任务标题、实例标题、定义名、关联 ID（所属实例的；用户任务书签的关联 ID 就取自所属实例） |

**关键字的小写**（2026-11-25 修订，ADR-0296；以前写「不变文化的小写」，没说是哪一种映射：.NET 的不变文化不转换 `İ`（U+0130）、不转换基本多文种平面以外的字母，Go 的 `unicode.ToLower` 两者都转换）：逐个码位取 Unicode 的简单小写映射——UnicodeData.txt 的 Simple_Lowercase_Mapping 字段，没有这个映射的码位不变——不做 SpecialCasing.txt 里的完全映射，也不做按上下文、按区域的映射。所以 `İ`（U+0130）折成 `i`（U+0069），不是 `i̇`（U+0069 U+0307）；`ẞ`（U+1E9E）折成 `ß`（U+00DF）；`Σ` 不论在不在词尾都折成 `σ`（U+03C3），不折成 `ς`；`K`（U+212A 开尔文符号）折成 `k`；`ǅ` 与 `Ǆ` 折成 `ǆ`；切罗基大写字母折成小写字母（`Ꭰ` U+13A0 → `ꭰ` U+AB70）；基本多文种平面以外的字母同样折叠（`𐐀` U+10400 → `𐐨` U+10428）；`ı`（U+0131）本身是小写，不变。这种映射不改变 UTF-16 的长度（不会把基本多文种平面里的字符映射到平面以外，反之亦然），折叠前后的长度相同。含孤立代理项的文本不在规定之内。**结果以实现所带的 Unicode 数据为准**，规范不钉 Unicode 的版本（2026-11-25 写明，ADR-0303；以前只说映射取自实现所带的数据）：Unicode 的字符编码稳定性政策（Case Pair Stability）保证已分配的字符一旦互为大小写就一直互为大小写，两个已分配而不成对的字符以后也不会成对，所以已有的映射不随版本改变；版本之间的差别只在新收录的字符，以及因为新收录了与它成对的字符才多出映射的已有字符（Unicode 8.0 收录切罗基小写字母之后，原有的切罗基大写字母才有小写映射；16.0、17.0 收录 `Ɤ`（U+A7CB）、`Ƛ`（U+A7DC）之后，原有的 `ɤ`（U+0264）、`ƛ`（U+019B）才有大写映射）。两个实现因此只在其中一方的 Unicode 数据还没有收录的字符、以及与这些字符成对的已有字符上可能转换得不一样，这些字符怎样转换实现自定；向量只用早已分配的字符：只收 Unicode 9.0 及更早就已分配、映射之后没有变过的字符（2026-11-25 逐个核对过 `keyword-matching.json` 与 `expression-functions.json` 里的全部非 ASCII 字符，最晚的是 9.0 的阿德拉姆字母，ADR-0303）。表达式的 `upper` / `lower` 用同一份映射与同一条依据（见「内置函数」）。数据库实现的查询列存的是写入时的折叠结果：这次修订只改变含 `İ` 与平面以外大写字母的文本的折叠，含这些字符的已有行要重新折叠回填，否则关键字里的这些字符匹配不到它们；回填用「默认实现与 Provider」里 `Workflow.Database` 的查询列回填（2026-11-25 写明，ADR-0304）。

## 活动

- `IActivity.ExecuteAsync(ActivityContext, ct)` → `Result<ActivityOutcome>`；`IResumableActivity.ResumeAsync(ActivityResumeContext, ct)`（书签被消费后调用，上下文带书签与恢复输入）；`ICompensableActivity.CompensateAsync(ActivityContext, ct)`（补偿时按完成顺序逆序调用）。失败的 `Result` 即故障；抛出异常翻译为 `WORKFLOW_UNEXPECTED_001` 的故障。上下文交给扩展代码（活动、结果处理器 `WorkflowOutcomeContext`、指派规则 `AssigneeResolution`）的实例与节点实例是只读视图 `WorkflowInstanceView` / `WorkflowNodeInstanceView`，跟随引擎正在修改的对象；状态变更只经引擎（活动经输出与结果），活动的私有状态经上下文的 `State` 读写（2026-11-11 修订，ADR-0133）。
- `ActivityOutcome`：`Complete(outputs?, outcome?)`、`EndInstance(outputs?, outcome?)`、`Suspend(bookmarks…)`、`CompleteWithChildren(children, outputs?, outcome?)`、`SuspendWithChildren(children, bookmarks…)`。输出合并进实例变量；`outcome` 写进节点输出，并写进实例变量 `outcome`（最近一次带结果的节点），出边条件与后继决策节点都按它选边（没有结果的节点不覆盖）。
- 活动是作用域服务：`services.AddActivity<TActivity>(type, outgoing: ActivityOutgoing.AllMatched, descriptor?)`，登记进 `ActivityRegistry`；重复登记同一类型是编程错误。活动经 `ActivityContext.Services` 解析其他服务，经 `ActivityContext.Invocation` 拿到本次执行的调用上下文。
- **执行方式**（ADR-0077）：节点在引擎这次操作所在的调用里执行——同一服务作用域、同一调用上下文、同一事务：活动经 `ActivityContext.Services` 取到的仓储、结果处理器的写入与引擎的写入一起提交或回滚，活动看到的主体、租户与追踪就是这次调用的。引擎的操作总是在一次调用里：调用方的（加入它的事务），或调用方之外使用时引擎自己开的（见「事务」）。取消令牌是这次调用的令牌：取消即整个操作随事务回滚。活动失败（失败的 `Result` 或异常）按「故障与重试」处理，它在失败之前做的写入留在同一事务里、不单独回滚：会写入的活动与结果处理器先校验、后写入。
- 内置活动（`ActivityTypes`）：`Start`（可延迟启动：`WorkflowStartRequest.StartAt` 时以 Timer 书签挂起到该时刻）、`End`（可带 `Outcome`：先调用全部 `IWorkflowOutcomeHandler`，再结束实例）、`Terminate`（`Reason`）、`SetVariable`（`Values` 字面量、`Expressions` 表达式）、`Decision`（出边 Exclusive）、`Parallel`（出边 All）、`Join`（`Mode` = `WaitAll` / `WaitAny`）、`Delay`（`Duration` 秒或表达式，或 `Until` 时刻表达式（读法见下文「活动里的时间」），或 `Cron` + `TimeZone` 的下一次；Timer 书签）、`UserTask`、`PublishEvent`（`EventName`、`Payload`，发布 `workflow.custom` 消息）、`WaitSignal`（`SignalName`、`AcceptAnyCorrelation`，Signal 书签）、`EventGateway`（`Signals` 列表 + 可选 `TimeoutSeconds`：先到的信号消费、其余书签清掉，结果标签 = 信号名或 `timeout`，输出 `signal`、`payload`；BPMN 的事件网关）、`SubWorkflow`（`DefinitionName`、`DefinitionVersion`、`Variables`、`VariableExpressions`、`WaitForCompletion`、`ResultVariable`、`FailOnChildFault`）、`ForEach`（`Items`、`ItemVariable`、`DefinitionName`、`Parallel`、`FailFast`、`ResultVariable`）、`Log`（`Message`、`Level`）、`Fault`（`Message`）、`Compensate`（`NodeIds` 可选：对已完成的指定节点（缺省为本节点之前全部已完成节点）逆序执行补偿后继续；BPMN 的补偿抛出事件）、`Http`（`Url`、`Method`、`Headers`、`Body`、`ContentType`、`TimeoutSeconds`、`ResultVariable` 默认 `http`、`FailOnErrorStatus` 默认 `true`；输出 `{ status, headers, body }`，JSON 响应解析为对象；经 `WorkflowHttpClient`，应用可替换）、`ExternalTask`（`Topic`、`Payload`：External 书签；活动没有锁时长属性，锁时长由工作者领取时 `FetchAsync` 的 `lockDuration` 给出，由外部工作者经 `IWorkflowExternalTaskService` 领取并完成；Camunda 的外部任务）。`Script` 活动不进框架：表达式语言与 `SetVariable` 覆盖其场景，脚本引擎属于第三方依赖。
- **活动里的时间**（2026-11-25 修订，ADR-0296）：`Delay` 的 `Until`、`UserTask` 的 `DueAt` 与按时间读的值（`WorkflowVariables.TryGet<DateTimeOffset>`、`WorkflowValues.TryConvert<DateTimeOffset>` 与 `TryConvert<DateTime>`，引擎读书签载荷里的时刻也是）都按「表达式」的「时间」文法读字符串：`YYYY-MM-DD`，或接 `T`、`t` 或一个空格再接 `HH:MM`，秒、一到九位小数与偏移可省；没有时间是当天零点，没有偏移是 UTC，首尾的空白不裁剪，比 100 纳秒更细的小数截断；读出的时刻换算成 UTC（偏移为零）。`Until` 当表达式求值（字面的时间写成字符串 `'2026-01-02T08:00:00Z'`），结果不是合文法的字符串 → 节点故障 `WORKFLOW_VALIDATION_002`（`workflow.activity.property_not_datetime`，参数 `name`）。`DueAt` 是合文法的字符串时就是那个时刻，否则当表达式求值：求值失败是表达式的错误（`WORKFLOW_VALIDATION_003`），结果不是合文法的字符串是 `property_not_datetime`。以前交给 .NET 的日期解析：`12:30` 补上机器当天的日期，`01/02/2026`、`Jan 2, 2026`、`2026-1-2` 都读得出，结果随机器的日子与时区变；现在都不是时间（`12:30` 作为 `DueAt` 当表达式求值，是语法错误 `unexpected_character`）。任务视图与书签载荷里的 `dueAt` 写成 UTC 的时间文本（与截止时间的写法相同），以前保留属性里写的偏移。
- 出边选择（`ActivityOutgoing`）：`AllMatched`（默认）取全部无条件或条件为真的出边，一条都没有时取默认出边；`Exclusive` 按优先级取第一条无条件或条件为真的，没有则默认出边；`All` 取全部；`None` 不流出。`AllMatched` / `Exclusive` 一条也选不到 → 实例故障。默认出边在两种模式下都是"兜底"。
- `Join`：按入边计数；`WaitAll` 全部到达才执行，`WaitAny` 首个到达执行、其余到达忽略直至一轮结束。波次结束时仍有未凑齐的 Join 而没有书签 → 实例故障（`workflow.join.dead`）。
- **HTTP 活动的目标地址**（2026-11-04 修订，ADR-0117）：地址由流程定义给出，租户管理员可编辑，不可信。缺省的 `WorkflowHttpClient` 过出站地址策略（RFC-0008），缺省只能连公网地址，重定向的目标同样检查。目标被拒绝是节点故障 `WORKFLOW_UNEXPECTED_001`（消息键 `workflow.http.address_denied`，参数 `host` 是请求里的主机名，不带解析出的地址），不发起连接。应用注册自己的 `WorkflowHttpClient` 时由它决定能连哪里。构造改成 `WorkflowHttpClient(OutboundAddressPolicy)` 与 `WorkflowHttpClient(HttpClient)`，没有无参构造。
- **HTTP 活动的响应体与超时上限**（2026-11-12 修订，ADR-0181）：地址指向的服务器不可信。
  - 正文边读边计数，保留前 4 MiB，读到第 4 MiB 多一个字节即停，结果的 `truncated` 为真、`body` 是前 4 MiB 的文本（不解析 JSON）；其余不读，释放响应时连接随之中止。`Content-Length` 不可信，不按它预先分配。
  - `TimeoutSeconds` 写了就必须是正的有限秒数（`workflow.activity.property_not_seconds`），且不超过引擎选项 `MaxHttpTimeoutSeconds`（缺省 300；`workflow.activity.property_seconds_over_limit`，参数 `name`、`max`）。两者都是 `WORKFLOW_VALIDATION_002`：定义校验报在 `$.nodes[i].properties.TimeoutSeconds`；活动发请求之前再核对一次（定义可能早于上限发布，或宿主之后调低了上限），不合法即节点故障、不发请求。以前不是正数时静默取缺省值，没有上限。上限只管定义里写的值，引擎选项 `HttpTimeoutSeconds` 是宿主自己的配置，不受它限制。
- **HTTP 活动的超时与取消**（2026-11-02 修订，ADR-0097）：`Http` 活动自己持有超时令牌——`TimeoutSeconds`（缺省取引擎选项 `HttpTimeoutSeconds`）按 `ActivityContext.Clock`（引擎的 `TimeProvider`）落成定时取消，与调用方令牌（活动收到的取消令牌，即这次调用的令牌）链接后交给 `WorkflowHttpClient.SendAsync(request, ct)`；客户端不另计时。时限从交给客户端起到读完响应体：客户端读到响应头即交回（`ResponseHeadersRead`，2026-11-12 修订，ADR-0181；以前是 `ResponseContentRead`，先整体缓冲），活动在同一个时限、同一对令牌下读正文，读正文时的取消与超时按下列同一规则归因，读到一半连接断开与发送失败同为 `workflow.http.failed`（`status` 0，`reason` 是异常类型）。归因只看令牌，客户端抛出与写成结果同一规则（与 RFC-0004 `DeadlineMiddleware`、RFC-0022「超时与取消」同一形状）：
  1. 超时令牌已触发即超时，不论调用方令牌是否也已触发：客户端抛出的取消（不论报的是哪个令牌）、应用替换的处理器接住取消后交回的响应，都按超时报告——节点故障 `WORKFLOW_UNEXPECTED_001`（消息键 `workflow.http.timed_out`，没有参数；2026-11-03 修订，ADR-0105，以前是 `workflow.http.failed` 带 `details.status` 0、`details.reason` `timeout`），响应不写进变量。
  2. 超时令牌没有触发、调用方令牌已触发即取消：客户端抛出的取消原样抛出；交回的响应不写进变量，改为抛出调用方的 `OperationCanceledException`。引擎把它当作调用被取消，整个操作随调用回滚，不记成节点故障。
  3. 两个令牌都没有触发：客户端抛出的取消是它自己的超时（如应用替换的 `HttpClient` 设了 `Timeout`），按超时报告；`TimeoutException` 在任何情况下都按超时报告；`HttpRequestException` 是 `details.reason` 为异常消息的同码故障；响应照常处理。
  - 行为变化（2026-11-02）：以前计时器按墙钟在客户端里，归因按「调用方令牌没有触发的取消即超时」推断：两个同时触发时原样抛出调用方的取消，处理器交回的响应当作成功，`TimeoutException` 是活动抛出的未处理异常（`details.reason` 是它的消息）。`WorkflowHttpClient.SendAsync` 去掉 `timeout` 参数。

## 实例与执行

`WorkflowInstance`：`Id`、`DefinitionId`、`DefinitionName`、`DefinitionVersion`、`DisplayName`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Terminated`）、`Variables`、`JoinStates`、`CorrelationId`、`StarterId`、`ParentInstanceId`、`ParentNodeInstanceId`、`Depth`、`TenantId`、`CreatedAt`、`StartedAt`、`EndedAt`、`Fault`（`Error?`）、`FaultNodeId`、`FaultNodeInstanceId`、`CancellationReason`、`Outcome`。`WorkflowNodeInstance`：`Id`、`InstanceId`、`NodeId`、`ActivityType`、`Status`（`Running` / `Suspended` / `Completed` / `Canceled` / `Faulted` / `Compensated`）、`Attempt`（从 1 起，重试复用同一记录）、`StartedAt`、`EndedAt`、`Inputs`、`Outputs`、`State`、`Fault`、`CompensatedAt`。

- `StartAsync(WorkflowStartRequest)`：按 `DefinitionId` 或 `Name` + `Version`（缺省最新已发布）在调用方的分区里找定义，必须 Published（否则 `WORKFLOW_NOTFOUND_001`）；声明的变量缺省补默认值，缺必填 → `WORKFLOW_VALIDATION_002`；深度超过 `MaxSubWorkflowDepth` → `WORKFLOW_VALIDATION_002`。实例属于调用方的分区。`ReusePolicy`（按 `CorrelationId` + 定义名在同一分区里找未结束的实例：`AllowDuplicate` 默认；`RejectActive` → `WORKFLOW_CONFLICT_005`；`ReturnActive` 返回已有实例；没有 `CorrelationId` 时忽略）。要查重时，查重与写入新实例在同一把 `IWorkflowInstanceLock` 锁里完成（锁键 `start.` + 分区租户、定义名与关联 ID 的 SHA-256 十六进制），锁持有到这次操作的事务结束：并发的同键启动串行化，后到者一定看得到先到者提交的实例，`RejectActive` 只有一个成功、`ReturnActive` 都拿到同一个实例；等锁超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。`StartAt` 为延迟启动。定义有 `Timeout` 时建 `InstanceTimeout` 书签。实例先写入存储再加锁执行第一个波次。子流程的实例 ID、父实例、父节点实例与深度只由引擎在启动子实例时设置，不是启动请求的公开字段：调用方能写的话，同分区的调用方可以伪造父链接，子实例结束时恢复别人子流程的书签，负的深度还能绕过 `MaxSubWorkflowDepth`（2026-11-11 修订，ADR-0132）。
- **波次**：在实例锁内从队列逐个执行节点直到队列空或实例不再 Running；一个波次执行节点数超过 `MaxNodeExecutionsPerBurst` → 实例故障（`workflow.engine.runaway`）。波次结束：有 `EndInstance` 或没有剩余书签（实例级超时书签不计）且没有未凑齐的 Join → `Completed`；否则实例保持 Running 等待书签。子流程启动与通知父流程在锁释放后进行（避免死锁）。
- **书签**（`WorkflowBookmark`：`Id`、`InstanceId`、`NodeId`、`NodeInstanceId`、`Kind`、`Key`、`Payload`、`DueAt`、`CorrelationId`、`CreatedAt`、`TenantId`）。种类 `BookmarkKinds`：`UserTask`（Key = 指派人）、`Timer`、`Signal`（Key = 信号名）、`SubWorkflow`（Key = 父节点实例 ID）、`Retry`、`NodeTimeout`、`External`（Key = 主题）、`Reminder`（用户任务提醒）、`InstanceTimeout`（实例级超时）、`DefinitionTimer`（定时启动，`InstanceId` = 定义 ID）。`ResumeAsync(bookmarkId, inputs?, expectedKey?)`：消费书签（删除）后恢复节点；实例不是 Running → `WORKFLOW_CONFLICT_003`（`DueBookmarkPoller` 对此类书签把 `DueAt` 后推 `NotResumableBackoff`）；`expectedKey` 与书签 `Key` 不同 → `WORKFLOW_CONFLICT_003`。`UserTask` 与 `External` 书签只能经 `IWorkflowTaskService` 与 `IWorkflowExternalTaskService` 消费——两个服务校验指派人与持锁的工作者并发布完成事件——`ResumeAsync` 遇到这两种 → `WORKFLOW_CONFLICT_003`（消息键 `workflow.bookmark.service_owned`，参数 `id`、`kind`），别的分区的仍按不存在处理；轮询解锁到期的外部任务是引擎内部的消费，不受此限。`ResumeAsync` 不校验调用者身份、`inputs` 原样交给活动，是给受信代码的系统级入口，应用不得把它直接开放给最终用户（2026-11-12 修订，ADR-0171）。`Reminder`、`InstanceTimeout`、`DefinitionTimer` 由引擎直接处理，不经活动。`SignalAsync(name, payload?, correlationId?)` 消费调用方分区里匹配的全部 Signal 书签（`CorrelationId` 为空的书签接受任意关联），再按触发式启动规则启动实例，返回消费数（含启动数）。
- **故障与重试**：节点失败时按顺序：有重试策略且未用尽 → 节点 Faulted、插入 Retry 书签（`DueAt` = now + `FirstDelay × BackoffFactor^(Attempt−1)`），实例保持 Running；`ContinueOnError` → 节点 Faulted、变量 `lastError` = 错误的 `WorkflowFault` 结构（`code`、`message`、`messageKey`、`arguments`、`inner`；表达式照旧读 `code` 与按进程默认区域渲染的 `message`，展示用 `WorkflowJson.DeserializeFault(...).Localize()`；2026-11-03 修订，ADR-0106）、以 `outcome = "error"` 流出；否则实例 Faulted（记录 `Fault`、`FaultNodeId`、`FaultNodeInstanceId`；`CompensateOnFault` 时补偿）。`RetryAsync(ids[])` 只接受 Faulted 且有 `FaultNodeInstanceId` 的实例：清故障、从故障节点重试（`Attempt + 1`）；引擎级故障（死 Join、波次上限）不可重试 → `WORKFLOW_CONFLICT_001`。
- **故障原因的本地化**（2026-11-03 修订，ADR-0105）：实例与节点实例上的 `Fault`（`WorkflowFault`）以及事件载荷里的 `fault` 存错误码 `Code`、消息键 `MessageKey`、参数 `Arguments`（值按不变文化转成字符串，与幂等记录同一做法）、内层原因 `Inner`（子流程的故障、存储不可达的根因），以及按进程默认区域渲染的 `Message`（日志用）。面向人的文案不存进库：展示用 `Fault.Localize()`——按当前消息作用域（调用链里是调用方的区域，RFC-0010）渲染消息键与参数，有内层原因时以 `workflow.fault.caused_by`（参数 `message`、`cause`）接上内层的文案；返回给调用方用 `Fault.ToError()`——带键、参数与内层的错误，由宿主在 Egress 翻译成调用方的语言。没有消息键的故障（自由文本，或升级前写入的）按 `Message` 原文展示。
  - 引擎与内置活动的失败原因全部是消息键与参数，参数里不再放英文句子：节点超时 `workflow.node.timed_out`（参数 `node`），活动属性不合法按种类各有键（`workflow.activity.property_not_seconds`、`property_not_datetime`、`property_empty`、`property_not_cron`、`property_not_time_zone`、`property_cron_exhausted`、`property_no_signal`、`property_needs_correlation`、`property_not_array`、`property_not_url`、`property_not_http_method`、`property_header_invalid`、`property_not_media_type`、`property_not_expression`、`property_not_workflow_name`、`property_unknown_value`、`property_not_threshold`、`property_not_integer`），指派规则（`workflow.assignee.rules_not_array`、`rule_type_missing`、`expression_count`、`users_missing`），表达式的结果不是布尔与模板占位符没闭合（`workflow.expression.not_boolean`、`unterminated_placeholder`），HTTP 超时 `workflow.http.timed_out`，`Fault` 节点 `workflow.activity.fault_raised`，没有开始节点 `workflow.definition.start_missing`，引擎自己开的调用没有工作单元 `workflow.store.unit_of_work_missing`，待办与外部任务的输入（`workflow.task.query_target_missing`、`workflow.external.lock_duration_invalid`、`max_tasks_invalid`（参数 `count`、`max`）、`retry_after_negative`）。只承载英文原因的 `workflow.task.query_invalid` 与 `workflow.assignee.rule_invalid` 删除，没有别的调用点；自定义指派规则用应用自己的消息键。
  - 应用或第三方给的文本原样放在参数里，外层的概括键照常本地化：HTTP 应答的原因短语（`workflow.http.failed` 的 `reason`）、外部任务工作者报的错误（`workflow.external.failed` 的 `reason`）、`Fault` 节点作者写的消息（`workflow.activity.fault_raised` 的 `message`）、存储与驱动的异常（`workflow.store.unavailable` 的 `reason`）、自定义活动经 `ActivityContext.PropertyInvalid(name, reason)` 给的原因。活动抛出（`workflow.activity.failed`）与 HTTP 连接失败（`workflow.http.failed`）的 `reason` 只写异常类型的全名，完整异常进日志（2026-11-04，ADR-0115）：故障会存进实例、经接口返回，异常消息常带内部主机与端口；要让调用方看到说明，活动返回失败，不靠抛异常。
  - 设计期诊断（2026-11-03 修订，ADR-0106）：定义校验的逐条问题见上文「校验」，每条有自己的消息键（结构问题 `workflow.definition.*`，属性问题沿用 `workflow.activity.property_*`，指派规则问题是 Provider 给的错误，`AssigneeResolver.Validate` 返回错误列表）；表达式的语法与求值诊断每种一个键（`workflow.expression.empty`、`unexpected_character`、`token_expected`、`member_expected`、`unexpected_end`、`unterminated_string`、`unknown_variable`、`unknown_function`、`argument_count`、`argument_range`、`member_of_non_object`、`index_out_of_range`、`index_of_scalar`、`operands_not_scalar`、`operands_not_boolean`、`operands_not_numeric`、`not_number`、`not_date`、`division_by_zero`，以及后来加的 `too_long`、`too_deep`、`string_too_long`、`number_out_of_range`、`budget_exceeded`，参数都有 `expression`，位置从 1 起；全部的键、参数与何时报出见「表达式」的「错误」），错误码仍是 `WORKFLOW_VALIDATION_003`，`workflow.expression.invalid` 删除；`WorkflowJson.DeserializeDefinition` 的错误：JSON 为 `null` 是 `workflow.definition.json_null`；语法错误与值的类型不符是 `json_malformed`（参数 `path`、`line`、`position`）；模型会拒绝的值在构造模型之前按与模型同一份规则核对（2026-11-03 再修订，ADR-0106）——缺少必填成员（取自序列化器的元数据；非必填成员没写时取模型的初值，不报，ADR-0107）`member_missing`（参数 `member`）、名字 `name_invalid`、定义 ID `id_empty`、版本 `version_invalid`、分类长度 `category_too_long`、定义与节点的超时 `timeout_invalid`、节点 ID `node_id_invalid`、活动类型 `activity_type_invalid`、转移 ID `transition_id_invalid`、变量名 `variable_name_empty`、重试策略 `retry_attempts_invalid`、`retry_delay_negative`、`retry_backoff_invalid`（都在 `workflow.definition.` 下），以 `workflow.definition.invalid`（参数 `count`）与 `details.errors` 的违规列表返回，形状同发布时的校验；核对通过的文档构造模型不再抛出，模型自己的英文异常只兜住编程错误；`json_value_invalid` 删除。
  - 状态名（2026-11-03 修订，ADR-0106）：模型、查询结果、事件与回调输入里的状态是枚举值，不变；给人看的文案里不放状态名——状态不允许操作按状态各一个键（`workflow.instance.is_running` … `is_terminated`、`workflow.definition.is_draft` … `is_archived`，参数 `id`、`status`，`workflow.instance.status` 与 `workflow.definition.status` 删除），状态的展示名是 `workflow.instance_status.*`、`workflow.node_status.*`、`workflow.definition_status.*`（`WorkflowStatusMessages.KeyOf` / `Localize`）。
- **实例的分区**：实例属于启动它的调用的分区（`TenantId`，之后不变；子流程与父实例同分区）。调用只看得到、操作得了自己分区的实例与书签，别的分区的一律按不存在处理，不暴露它们是否存在：按实例 ID 的控制（`SuspendAsync`、`ResumeInstancesAsync`、`CancelAsync`、`TerminateAsync`、`RetryAsync`、`SetVariablesAsync`、`JumpAsync`、`MigrateAsync`）与实例查询 → `WORKFLOW_NOTFOUND_002`；按书签 ID 的 `ResumeAsync`（含定义级定时书签）与用户任务、外部任务的操作 → `WORKFLOW_NOTFOUND_003`；`PurgeAsync` 只清自己分区的实例；信号只消费自己分区的书签、只启动自己分区的定义。平台与租户对等：平台调用同样看不到、操作不了任何租户的实例。引擎内部的接续（轮询恢复、子流程启动与回调、级联取消）在实例的分区里进行：轮询器以书签的租户开调用。
- **查询**：`FindInstanceAsync(id)`、`ListInstancesAsync(query)`、`CountInstancesAsync(query)`、`NodeInstancesAsync(instanceId)`、`BookmarksAsync(instanceId)` 只查调用方的分区；`WorkflowInstanceQuery` 没有租户字段——平台调用查到的就是平台的实例（原来"租户为空即不过滤全部租户"的语义取消），没有跨租户的查询。按 ID 找不到或在别的分区为无值 / `WORKFLOW_NOTFOUND_002`。`WorkflowInstanceQuery` 的分页（`Skip` ≥ 0、`Take` 1～`MaxTake`（1000），默认 0 与 100）不在构造时抛出：越界时 `ListInstancesAsync` 返回 `WORKFLOW_VALIDATION_002`（HTTP 400，消息键 `workflow.instance.query_page_invalid`，参数 `skip`、`take`、`max`），与待办查询一致——查询常由请求绑定，越界是调用方可以改正的输入（2026-11-01 修订，ADR-0082）；`CountInstancesAsync` 不看分页，不校验。`BookmarksAsync(instanceId)`（2026-11-02 修订，ADR-0096）是面向应用的只读查询，列出实例的全部等待点（全部书签种类）的摘要 `WorkflowBookmarkSummary`（`Id`、`InstanceId`、`NodeId`、`NodeInstanceId`、`Kind`、`Key`、`DueAt`、`CorrelationId`、`CreatedAt`；不含载荷），创建时间升序，应用的实例详情用它而不必经存储契约；实例不存在或在别的分区 → `WORKFLOW_NOTFOUND_002`（定义 ID 也是，定义级定时书签不经它列出）。过滤项（2026-11-02 修订，ADR-0096）：`Status` / `Statuses`、`DefinitionName`、`CorrelationIds`、`ParentInstanceId`、`TopLevelOnly`、`StarterId`、`Keyword`、`CreatedFrom` / `CreatedTo`、`EndedBefore`，同时给出的取交集。`CorrelationIds`（取代单个的 `CorrelationId`）：实例的关联 ID 是其中之一即匹配，`null` 不过滤，空集合不匹配任何实例（列表为空、计数为 0，不进存储——与 `Statuses` 的空集合视同未限定不同，免得一页没有单据时查出全部实例），重复的只算一个；最多 `MaxCorrelationIds`（1000）个且不能有空白的 ID，否则 `ListInstancesAsync` 与 `CountInstancesAsync` 都返回 `WORKFLOW_VALIDATION_002`（消息键 `workflow.instance.query_correlations_invalid`，参数 `count`、`max`），不进存储。`TopLevelOnly`：只取 `ParentInstanceId` 为空的实例（子流程继承父实例的发起人与关联 ID，「我发起的」与按单据查进度要用它排除子流程）。`Keyword`：匹配实例标题 `Title`（启动时给的 `DisplayName`，没给取定义的显示名，再没有取定义名）或关联 ID `CorrelationId`（2026-11-02 修订，ADR-0099），规则见「定义」一节的「关键字」。排序：有序的排序项列表 `Sorts`（字段 `CreatedAt`、`EndedAt`、`Status`、`DefinitionName`、`Title` 加方向，空或没给时创建时间降序），规则见「定义」一节的「列表排序」（2026-11-02 修订，ADR-0102；2026-11-03 修订，ADR-0104）。过滤、排序、分页与计数都由存储执行。
- **批量语义**（引擎、任务与外部任务服务相同，2026-11-11 修订，ADR-0149）：
  - 结果与入参等长、同序；空集合直接返回空结果，不开事务、不访问存储；重复的 ID 按入参顺序逐项施加。
  - 自己开调用时整批一个事务：逐项失败不回滚其余项，失败的项没有写入；提交失败时每一项都报告提交的错误。在调用方的工作单元里时并入调用方的事务。
  - 取消在任何时刻都整批回滚并抛出，事务里不留下半批。
  - 不强加批次上限，批次大小由调用方控制。
- **控制**（批量，逐项结果）：`SuspendAsync(ids[], reason?)`（只允许 Running）、`ResumeInstancesAsync(ids[])`（只允许 Suspended；不重新驱动书签，到期书签由轮询接续）、`CancelAsync(ids[], reason?)`、`TerminateAsync(ids[], reason?)`（Running / Suspended / Faulted 可取消或终止，已结束的幂等返回；清掉非终态节点与全部书签；取消且 `EnableCompensation` 时执行补偿；级联到未结束的子流程并通知父流程）、`RetryAsync(ids[])`、`SetVariablesAsync(updates[])`（Running / Suspended：合并变量、删除指定键）、`JumpAsync(jumps[])`（Running / Suspended / Faulted：取消全部未结束节点与书签、清 Join 状态，从目标节点重新执行；目标不存在 → `WORKFLOW_VALIDATION_002`）、`MigrateAsync(migrations[])`（Running / Suspended 迁到同一名字空间里同名的另一个已发布版本，别的名字空间的同名定义按不存在处理：每个未结束节点在目标定义里必须存在且活动类型相同，否则 `WORKFLOW_VALIDATION_002`；只改定义引用，节点实例与书签原样保留）、`PurgeAsync(request)`（删除早于给定时刻结束的实例及其节点实例与书签，可按状态与定义名过滤，返回条数；一次最多 `Limit` 条（1～1000，默认 1000），越界不在构造时抛出，返回 `WORKFLOW_VALIDATION_002`（消息键 `workflow.purge.limit_invalid`，参数 `limit`、`max`）；`XiHan:Workflow:RetentionDays` > 0 时由轮询器每 `HousekeepingIntervalSeconds` 自动执行）。状态不允许 → `WORKFLOW_CONFLICT_001`。单实例的写法是扩展方法。
- **子流程**：`SubWorkflow` / `ForEach` 的子实例带 `ParentInstanceId`、`ParentNodeInstanceId`、`Depth + 1`，继承父实例的租户、`CorrelationId` 与 `StarterId`（所以按发起人或关联 ID 查顶层实例要加 `TopLevelOnly`）；子实例到达终态时以 `SubWorkflow` 书签（Key = 父节点实例 ID）回调父实例，输入 `childInstanceId`、`childStatus`、`childVariables`、`childFault`（子实例故障的结构，即 `WorkflowFault` 的 JSON：`code`、`message`、`messageKey`、`arguments`、`inner`；没有故障为空；2026-11-03 修订，ADR-0105）；父节点按子实例没有正常完成失败时按子实例的状态各一个键：`workflow.child.faulted`、`workflow.child.canceled`、`workflow.child.terminated`（参数 `id`、`status`；2026-11-03 修订，ADR-0106），手工恢复书签给了别的状态时是 `workflow.child.failed`（状态原样），子实例的故障作为内层原因；父节点实例 `State.childInstanceIds` 记录本轮启动的子实例，过期回调忽略。父实例正常完成不级联到不等待的子实例；`End(Outcome)` 与 `Terminate` 级联终止。级联取消与终止逐页处理父实例的全部子实例（2026-11-12 修订，ADR-0182；以前只处理最新的 1000 个）。
- **扇出与变量大小的上限**（2026-11-12 修订，ADR-0182）：流程定义由租户用户编辑，一个定义不能耗尽宿主内存。
  - `ForEach` 的项数不超过引擎选项 `MaxForEachItems`（缺省 1000，顺序与并行都算），超过即节点故障 `WORKFLOW_VALIDATION_002`（消息键 `workflow.activity.property_too_many_items`，参数 `name`、`count`、`max`），一个子实例也不启动。
  - 实例变量按存储的写法（缺省转义的 UTF-8 JSON）计数，不超过 `MaxVariablesBytes`（缺省 8 MiB），消息键 `workflow.instance.variables_too_large`（参数 `size`、`max`）。启动（含子实例与信号启动）、改变量与跳转时超过是 `WORKFLOW_VALIDATION_002`，操作不写回；节点的输出与输出映射合并之后超过时节点故障 `WORKFLOW_UNEXPECTED_001`，变量放回写入之前的值、节点实例不留输出，重试与继续出错照常。
  - `ForEach` 只在给了 `ResultVariable` 时收集子实例变量；收集的结果原地追加、边收边计，累计超过 `MaxVariablesBytes` 即节点故障，不等全部子实例结束。
- **锁**：`IWorkflowInstanceLock.AcquireAsync(instanceId, ct)` → `Result<IAsyncDisposable>`；等待超过 `LockTimeout` → `WORKFLOW_TIMEOUT_001`。Provider 的锁不可重入，与调用方事务无关（数据库实现的租约行走自己的连接与事务，立即对别的节点可见、不随调用方回滚）。锁键是不透明字符串：实例 ID，定时启动用定义 ID，重用策略查重用 `start.` 开头的派生键，Provider 不得假设它是实例 ID（长度不超过 128）。引擎在一个服务作用域里取得的锁持有到作用域结束（调用方事务提交或回滚之后）才释放：别的执行者不会在提交之前读到旧状态再改写它；同一作用域里再取空闲的锁直接复用（子流程回调父实例、同一请求办理两个任务），正在使用中再取立即以 `WORKFLOW_TIMEOUT_001` 返回（同一执行流的重入，等也等不到）。调用方事务里等锁最多 `LockTimeout`：两个事务交叉等对方持有的锁时一方超时、回滚并释放，不会无限等待。
- **事件**：装配了 `messaging.queue` 时经这次调用作用域里的 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/workflow`，`Type` = 主题，`Data` = `WorkflowEventData` JSON，扩展 `tenant` / `correlation`）；没有发布者时跳过；发布失败只记日志。事件与事务的关系（ADR-0077）：事件在事务里发布——装配了发件箱（`OutboxFeature`，RFC-0009）时写进与引擎写入同一事务的发件箱表，提交后由投递循环发出，回滚即丢弃；没有发件箱时直接发布到代理，可能早于提交、回滚时不撤回。要求事件与状态一致的应用必须装配发件箱。主题：`workflow.instance.started|completed|faulted|canceled|terminated|suspended|resumed|jumped|migrated|timedout`、`workflow.node.executing|executed|faulted`（`WillRetry`）、`workflow.task.created|completed|transferred|claimed|released|returned|urged|reminder`、`workflow.external.created|completed|failed`、`workflow.custom`（`Subject` = 事件名）。

## 事务

ADR-0077：引擎、定义目录、任务服务与外部任务服务的每个操作都在一个事务里，业务在自己的调用里发起流程、办理任务时，这个事务就是业务的事务。

- **加入调用方的事务**：服务所在的作用域里 `IWorkflowTransaction.IsActive` 为真时——数据库实现：在调用里且 Transaction 阶段已开启工作单元——操作在调用方的事务里执行：存储读写、活动与结果处理器的写入、经发件箱发出的事件都随调用方提交或回滚。业务回滚后引擎不留任何状态（实例、节点实例、书签、发件箱里的事件）；业务提交时与引擎写入一起落库，例如"写申请并发起流程"、"授予并完成实例"都是原子的。
- **自己开调用**：不在可加入的事务里时（到期书签轮询、定时器、后台恢复、调用之外的直接使用、Tenant / Authorize 阶段里的调用），引擎在新的服务作用域里经完整管道开一次调用（来源协议 `workflow`、目标是操作名），以调用方的租户（Baggage `xihan.tenant`，并预先写进安全上下文）进入，操作在那次调用的事务里执行：单个操作失败即回滚；批量操作整批一个事务，逐项失败不回滚其余项（失败的项没有写入），提交失败时每一项都报告提交的错误。应用不在服务状态时返回 `WORKFLOW_UNAVAILABLE_001`。自己开的调用里仍不能加入事务（持久化没有在 Transaction 阶段开启工作单元）是装配错误，返回 `WORKFLOW_UNAVAILABLE_001`，不再嵌套。
- **只读的调用**：调用只读（操作契约只读，RFC-0005「只读工作单元」：动词 `get` / `list` 或声明了 `ReadOnly`）时，查询照常；写入操作在取锁、执行活动之前即返回 `PERSISTENCE_UNEXPECTED_001`（`IWorkflowTransaction.EnsureWritable`；只读操作里发起或推进流程是操作实现的缺陷，HTTP 500），只读调用里的活动副作用不会先发生。两种实现结果相同（2026-11-02 修订，ADR-0099）：数据库实现看加入的工作单元是否只读（Transaction 阶段按操作契约开只读工作单元），内存实现看这次调用的操作契约是否只读——没有装持久化、没有工作单元时也一样拒绝。
- **锁与事务的先后**：实例锁在调用方事务里获取，锁行自己提交（见「锁」），持有到作用域结束才释放：事务提交之后别的执行者才拿得到锁、读到新状态；事务回滚后作用域结束即释放，不泄漏。作用域必须被释放（管道的请求作用域、`await using` 的作用域）：没有释放的作用域会一直持有并续约它的锁。
- **不加入事务的 Provider**：内存实现没有事务，`IsActive` 只看是否在调用里；写入立即生效、调用方回滚不撤销。只读的调用照上一条拒绝写入（2026-11-02 修订，ADR-0099；原来不看调用是否只读，装了发件箱时写入成功、事件写进只读工作单元被拒，调用以 `PERSISTENCE_UNEXPECTED_001` 结束，活动的副作用已经发生）。一致性套件以 `JoinsCallerTransaction` 区分两种 Provider。
- **SQLite**：只有一个写者，调用方事务写过之后再在另一个连接上写租约行要等调用方事务结束，即等自己；在 SQLite 上要把租约表（`WorkflowLockRecord`）经 `IEntityConnectionSelector` 放到另一个库文件。行级锁的数据库（PostgreSQL、MySQL、SQL Server）不受影响。

## 用户任务

- `UserTask` 活动属性：`Assignees`（列表）、`AssigneesExpression`、`AssigneeRules`（`[{ type, values }]`）三者合并去重；`ExcludeStarter`；`EmptyAssigneeStrategy`（`Fault` 默认 / `AutoApprove` / `AutoReject` / `Fallback` + `FallbackAssigneeRules`）；`CompletionPolicy`（`Any` 默认 / `All` / `Sequential`）；`ApprovalThreshold`（`All` 下的通过阈值：人数或百分比如 `"60%"`，缺省全部；通过数达到即通过，剩余人数不可能达到即以 `rejected` 完成）；`Title`（模板，缺省节点名）；`FormData`；`FormKey`；`Priority`；`DueAfterSeconds` / `DueAt`（截止时刻，写进任务视图；`DueAt` 的读法见「活动」一节的「活动里的时间」）；`ReminderAfterSeconds`（每隔该时长发布一次 `workflow.task.reminder`，直至任务结束）；`CcUserIds` / `CcRules`；`AllowedOutcomes`（缺省 `approved` / `rejected`）。
- 挂起时每个指派人一个 `UserTask` 书签（载荷 `title`、`formData`、`formKey`、`priority`、`dueAt`、`ccUserIds`），`Sequential` 一次只有当前指派人的一个；节点 `Timeout` 另插一个 `NodeTimeout` 书签，超时以 `outcome = "timeout"` 完成节点；`ReminderAfterSeconds` 另插 `Reminder` 书签。
- `UserTask`（视图）：`Id`（= 书签 ID）、`InstanceId`、`InstanceTitle`、`DefinitionName`、`NodeId`、`NodeInstanceId`、`Title`、`AssigneeId`、`ClaimedBy`、`Priority`、`DueAt`、`FormKey`、`CorrelationId`、`StarterId`、`FormData`、`CreatedAt`、`TenantId`。
- `IWorkflowTaskService`：待办一律经 `QueryAsync(UserTaskQuery)` 分页读取（不分页的 `PendingAsync` / `PendingByInstanceAsync` 已删除，2026-11-11 修订，ADR-0149）→ `UserTaskPage { Items, Total }`（指派人或实例必填其一，可按定义名、节点、创建时间与关键字 `Keyword`（任务标题、实例标题、定义名或关联 ID 任一包含即匹配，规则见「定义」一节的「关键字」；2026-11-02 修订，ADR-0096、ADR-0099）过滤，按排序项 `Sorts`（`UserTaskSort`，字段目前只有创建时间 `CreatedAt`，缺省升序，项数上限 `MaxSorts`；与定义、实例查询同一写法，2026-11-11 修订，ADR-0162）、同刻按写入先后；过滤、排序与分页由存储执行（`IWorkflowBookmarkStore.ListUserTasksAsync` / `CountUserTasksAsync`），`Total` 是同条件的总数；`Skip` ≥ 0、`Take` 取 1 到 `UserTaskQuery.MaxTake`（1000，与持久化的查询条数上限一致），越界在查询时返回 `WORKFLOW_VALIDATION_002`（HTTP 400），不在构造时抛出；只含调用方分区、实例 Running 的任务）；`CompleteAsync(completions[])`（`UserTaskCompletion`：`TaskId`、`ActorId`、`Outcome`、`Comment`、`Variables`；操作者必须是该书签的指派人 → 否则 `WORKFLOW_DENIED_001`；`Outcome` 不在允许范围 → `WORKFLOW_VALIDATION_002`；`Any` 首个通过即完成节点、`All` 按阈值完成（其间合并变量并保留其余书签）、`Sequential` 逐个；非 `approved` 的结果立即以该结果完成节点）；`TransferAsync(transfers[])`（改书签 `Key`，目标已是指派人 → `WORKFLOW_CONFLICT_004`）；`AddAssigneesAsync(additions[])`（`Any` / `All` 立即加书签，`Sequential` 排到队尾）；`ClaimAsync(claims[])`（`Any` 策略下认领：其他指派人的书签撤下，节点状态记 `claimedBy`；已被认领 → `WORKFLOW_CONFLICT_004`）；`ReleaseAsync(releases[])`（释放认领，恢复其他指派人的书签）；`ReturnAsync(returns[])`（退回：`TargetNodeId` 缺省为本节点之前最近完成的 `UserTask` 节点，没有则退回 `Start` 之后的首个节点；以跳转实现，节点历史记 `returned`）；`UrgeAsync(urges[])`（催办：发布 `workflow.task.urged`，不改状态）。节点 `State` 记 `assignees`、`approved`、`rejected`、`claimedBy`、`history`（`actorId`、`outcome`、`comment`、`at`）。
- 指派规则 `AssigneeRule(Type, Values)`：`IAssigneeProvider`（`RuleType`、`Validate(rule)`、`ResolveAsync(AssigneeResolution)`）按类型登记（`AddAssigneeProvider<T>()`，同类型重复登记是编程错误）；内置 `User`（原样）、`Starter`（实例发起人）、`Expression`（表达式求值为列表或逗号分隔字符串）。解析结果裁剪、去重、保序；未登记的类型 → `WORKFLOW_NOTFOUND_004`；Provider 返回失败即节点故障（不得以空列表掩盖数据不可用）。

## 外部任务

`ExternalTask` 活动以 `External` 书签（Key = 主题，载荷 `topic`、`payload`、`lockedBy`、`lockedUntil`、`notBefore`）挂起。`IWorkflowExternalTaskService`：`FetchAsync(topic, workerId, lockDuration, maxTasks)`（领取调用方分区里该主题未锁定（或锁已过期）且已到 `notBefore` 的任务，按创建时间最多 `maxTasks` 条：`maxTasks` 取 1 到 `MaxTasksPerFetch`（100），`lockDuration` 须为正，越界 → `WORKFLOW_VALIDATION_002`；可领取时刻与条数经 `IWorkflowBookmarkStore.AvailableAsync` 下推到存储查询，不读回该主题的全部书签；候选在实例锁内重读，仍可领取才把 `lockedBy` / `lockedUntil` 写回书签，被别的工作者抢先或实例已不在运行中的跳过，所以结果可能少于 `maxTasks`；返回 `ExternalTask` 视图：`Id`、`InstanceId`、`Topic`、`Payload`、`Variables`、`LockedUntil`、`Attempt`）；`CompleteAsync(completions[])`（`taskId`、`workerId`、`outputs`：工作者必须持有未过期的锁 → 否则 `WORKFLOW_DENIED_002`；输出合并进实例变量后节点完成）；`FailAsync(failures[])`（`taskId`、`workerId`、`error`、`retryAfter?`：有 `retryAfter` 时解锁并把 `notBefore` 后推、任务 `attempt + 1`；没有时节点以该错误故障，按节点重试策略处理）；`ExtendLockAsync(extensions[])`。锁到期的任务自动解锁（书签 `DueAt` = `lockedUntil`，到期由轮询驱动、活动重新挂起）。

## 表达式

转移条件、输出映射、`SetVariable` 的 `Expressions`、`SubWorkflow` 的 `VariableExpressions`、表达式指派、当表达式求值的节点属性与 `{{ 表达式 }}` 模板共用一个无依赖的小语言。它没有赋值、循环与副作用：同一段文本、同一组变量与同一个时刻，在任何实现上求出同一个结果或同一个错误。下面各小节是它的全部规定（2026-11-24 写明，ADR-0287；以前这一节只列了运算符与函数的名字，细节只在 .NET 的实现里）；标「修订」的是这次连 .NET 一起改了的地方，没有标的照 .NET 原来的行为写下。

`IWorkflowExpressionEvaluator`：`Validate(expression)`（只解析：语法、函数名与实参个数）、`Evaluate(expression, scope)` → `Result<Maybe<JsonNode>>`（结果是 `null`——字面量 `null`、缺少的成员或键、值为 null 的变量——时是成功的无值，不是失败；ADR-0059）、`EvaluateCondition` → `Result<bool>`（见「条件」）、`Render(template, scope)` → `Result<string>`（见「模板」）。活动里的 `ActivityContext.EvaluateProperty(name)` 同样返回 `Result<Maybe<JsonNode>>`：没有这个属性是无值，字符串属性当表达式求值，其余属性原样作为有值。语法或求值失败 → `WORKFLOW_VALIDATION_003`，消息键见「错误」。已解析的表达式按文本缓存，上限 `ExpressionCacheSize`。

### 长度与上限

- 长度与位置都按 UTF-16 码元计，与 .NET、JavaScript、Java 的字符串长度相同，基本多文种平面以外的字符算两个（2026-11-24 写明，ADR-0287；以前写「字符」，没说单位）。
- 表达式最长 4000（空白也算），超过 → `workflow.expression.too_long`（`length`、`max`），先于其他任何检查。
- 嵌套最多 64 层：括号、函数的实参表（不论几个实参只算一层，没有实参不算）、索引 `[]`、一元 `!` 与 `-` 各算一层，算进同一个计数；并列的不累加，链（`a + b + c`、`a.b.c`）不算嵌套。进到第 65 层 → `workflow.expression.too_deep`（`max`）。解析是递归下降，栈溢出接不住，进程直接退出，所以这两个上限在解析时判（2026-11-04 修订，ADR-0120）。实现必须能求值长度以内最长的链（两千个操作数的 `1+1+…`）。
- `+` 拼出的字符串不超过 4194304，在分配之前按两侧长度核对，超过 → `workflow.expression.string_too_long`（`length` 是两侧长度之和，`max`；2026-11-12 修订，ADR-0182）：`s + s` 每求值一次长度翻倍。模板渲染出的文本同一个上限（见「模板」）。
- **工作量**（2026-11-25 修订，ADR-0296；以前写「没有另设求值步数的上限」——运算次数虽不超过表达式的长度，每一步却可以读写几百万个码元或遍历几十万个值，长度以内写得出要算几秒到几分钟的表达式）：一次 `Evaluate` 或 `EvaluateCondition` 从 0 起累计工作量，一次 `Render` 的全部占位共用一份；累计超过 16777216（4194304 的 4 倍）→ `workflow.expression.budget_exceeded`（`max`）。工作量只取决于表达式、值与求值的次序，与平台和机器的快慢无关，单位是 UTF-16 码元，按两条记：
  1. **步**：每开始求值一个节点记 64。节点是字面量（数字、字符串、`true`、`false`、`null`）、变量、成员访问 `.name`、索引 `[i]`、函数调用、一元运算与二元运算；括号只分组，不是节点；`&&`、`||` 短路而没有求值的右操作数不记。
  2. **读**：运算求值完全部操作数之后、核对它们之前，记它读的操作数（见下表）的读取量。读取量：字符串是它的长度；**数字是它的写法的长度**（2026-11-25 修订，ADR-0303；以前是 0——变量里原文几百万位的 JSON 数字每读一次都要把原文重新扫描一遍，参考实现上约 7 毫秒，计量却是零）：从 JSON 文本读进来的数字（实例变量与 `outputs` 里的）是它在那段 JSON 文本里的原文的长度（`1e2` 是 3，`1.50` 是 4，`-0` 是 2），表达式得出的数字（字面量、运算与函数的结果）是数字的文本的长度（字面量 `1.50` 是 3，`1.5 + 1.5` 的结果是 1）；数组与对象是 64 乘以它里面的值的个数（它自己算一个，元素与成员的值逐层都算），加上它里面全部字符串、数字与成员名的长度；`null` 与布尔是 0。系数与字符串相同，每个字符记 1：把数字的写法读成数字，与把数字文本的字符串读成数字是同一种扫描（参考实现上一位约 1.5 纳秒，与扫描字符串的每个码元同一量级），`'123'` 与变量里的 `123` 当数字用时读取量相同。

  | 运算 | 读 |
  | --- | --- |
  | `+`，任一侧是字符串（拼接） | 两侧 |
  | 两侧都不是字符串的 `+`（加法）、`-`、`*`、`/`、`%`、一元 `-`、`<`、`<=`、`>`、`>=`，`abs`、`min`、`max`、`round`、`toNumber`、`format`，被索引的是数组时的 `[i]` | 其中是字符串或数字的操作数（实参、下标；2026-11-25 修订，ADR-0303：以前只读字符串，加法不读） |
  | `==`、`!=` | 两侧都是标量：其中是字符串或数字的；两侧都是容器：两侧；其余（有一侧是 `null`，或一侧是容器、另一侧是标量）：不读 |
  | `contains(x, y)`，`x` 是数组 | `x`，再加上 `x` 的元素个数乘以 `y` 的读取量 |
  | `contains(x, y)`（`x` 不是数组）、`startsWith`、`endsWith`、`upper`、`lower`、`trim`、`toString`、`date` | 全部实参 |
  | 被索引的是对象时的 `[i]` | 下标 |
  | `len` | 是字符串或数字的实参（2026-11-26 修订，ADR-0309；以前只读数字——字符串的文本虽然就是它自己，数出它有多少个 UTF-16 码元却要走完全文：从 JSON 文本读来的字符串要先解码，UTF-8 或按码位存字符串的语言要逐个码位换算，参考实现上四百万个码元约两毫秒，计量却只有一步）：字符串记长度，数字记它的写法的长度（2026-11-25 新增，ADR-0303：数字的文本要先从它的写法读出值再写出来）；容器的长度是个数，不读 |
  | `isNullOrEmpty` | 只读到能判定为止（2026-11-26 修订，ADR-0309；以前读是数字的实参、不读字符串）：是非空字符串的实参记 1——看到第一个码元就知道不是空串，与它多长无关；空串记 0；`null`、布尔、数字与容器不读——看类型与个数就能判定，布尔与数字的文本总不是空串。实现判定空串时不能取出全文（JSON 文本里空串只有 `""` 一种写法，看原文的长度即可） |
  | 模板里的每个占位 | 占位里的表达式的结果，在求值之后、接进渲染的文本之前 |
  | 其余：`!`、`&&`、`\|\|`、`.name`、被索引的是 `null` 或标量时的 `[i]`、`now`、字面量与变量 | 不读 |

  - 每记一笔就与上限比较，超过即停：步在开始求值这个节点时、先于它的操作数记；读先于这个运算自己的核对。所以 `k + k + k + k + k` 在工作量以内，报的是 `string_too_long`；`nope + 1` 照旧是 `unknown_variable`。模板的字面文本不记（渲染出的文本受 4194304 约束）。在模板里，求值占位里的表达式时超过的，`expression` 参数是那段表达式；记占位的结果时超过的，是模板（与 `string_too_long` 相同）。
  - 例：`len(k + k)`（`k` 是 1048576 个码元的字符串）是 4 步（`len`、`+`、两个 `k`）256，加上拼接读的两侧 2097152 与 `len` 读拼出的字符串 2097152，合计 4194560（2026-11-26 修订，ADR-0309；以前 `len` 不读字符串，是 2097408）；`isNullOrEmpty(k)` 是 2 步 128 加上 1，合计 129；`a == b`（`a`、`b` 各是 1000 个 `0` 的数组）是 3 步 192，加上两侧各 64 × 1001 + 1000（每个 `0` 的写法是 1），合计 130320；`n + 0`（`n` 是变量里写成 `1.` 接 4193998 个 `0` 的数字，值是 1）是 3 步 192，加上 `n` 的原文 4194000 与 `0` 的 1，合计 4194193；`contains(roles, 'admin')`（`roles` 是 10 个 5 个码元的字符串）是 3 步 192，加上 `roles` 的 64 × 11 + 50 = 754 与 10 × 5 = 50，合计 996。
  - 上限的取法：长度以内最长的表达式是四千个节点上下，只计步是 256000 上下；把四个 1048576 的字符串拼到拼接的上限（`k + k + k + k`）是 9437632；渲染出 4194304 个码元的模板是 4194304 加上占位的步——都在上限以内，正常的表达式离它更远。反过来，读写几百万个码元的运算做几次、遍历几十万个值的容器一两次、把原文四百万位的数字读四次就撞上：一次求值的 CPU 时间因此有界（参考实现上在一百毫秒以内），不再随表达式的写法放大。上限是常量，不做配置（与 4194304 同一个理由，ADR-0287 备选 13）。
- 错误里带出的文本——每个错误的 `expression` 参数与 `not_date` 的 `value`——最多 200：超过的取前 200 个码元再接 `…`（U+2026）；第 200 个码元是代理对的前一半时只取 199 个，不留下半个字符（2026-11-24 修订，ADR-0287；以前会把代理对从中间切开，`not_date` 的 `value`、`not_boolean` 与 `unterminated_placeholder` 的 `expression` 不截断）。

### 值

值模型是 JSON（「名字与值」）：`null`、布尔、数字、字符串、数组、对象。下文「标量」指布尔、数字与字符串，「容器」指数组与对象。字符串是合法的 Unicode（RFC-0001「JSON 文本」，含孤立代理项的字符串不在规定之内），容器的嵌套不超过 64 层。没有单独的时间类型：时间是字符串（见「时间」）。.NET 里由 CLR 类型构造、还没有过存储的值（`double`、`DateTimeOffset`、`Guid` 等）按它写成 JSON 之后的形式参与求值，与写进存储再读回来的值相同。

**数字**是十进制定点数 m × 10^(−s)：m 是绝对值不超过 2^96 − 1（79228162514264337593543950335）的整数，s 是 0 到 28 的整数。只看数值：`1.50` 与 `1.5` 是同一个数，没有负零、无穷与非数。

- **放进数字**：一个精确的数学值 v 放进数字，是取最大的 s（不超过 28）使 v × 10^s 按「一半取偶」舍入成整数之后绝对值不超过 2^96 − 1，结果是这个整数 × 10^(−s)；s 取到 0 仍然超过就是放不下。能精确表示的值不变；整数位越多，留得下的小数位越少：1 / 3 是 28 位小数的 `0.3333333333333333333333333333`，100 / 3 是 27 位小数，80 / 9 的 29 位有效数字放不下，是 `8.888888888888888888888888889`。数值上它就是 .NET 的 `decimal`（96 位整数加 0 到 28 的小数位数）的四则运算的结果。
- **数字文本**：字符串当数字用时（算术、比较、相等、下标与下面的数字函数）按这条文法读，整串匹配：`^[\t\n\v\f\r ]*[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?[\t\n\v\f\r ]*$`——首尾的空白只认 U+0009–U+000D 与 U+0020，数字只认 ASCII 的，没有千分位、十六进制、`Infinity` 与 `NaN`。它的数学值放进数字，放不下的不是数字文本；指数写多长都按数学值：`1e-400` 与 `0e99999999999999999999` 是 0，`1e400` 不是数字文本（2026-11-24 修订，ADR-0287；以前交给 .NET 的数字解析：结尾带 NUL 字符的字符串也读得出，位数与指数都上千时读出的不是数学值）。
- 下文「能当数字」指：值是数字，或是数字文本的字符串。布尔、`null` 与容器都不能当数字。
- 变量里的 JSON 数字按它的十进制写法的数学值放进数字：`1e2` 是 100，`1.50` 是 1.5，位数超出的照上面舍入。放不下的 JSON 数字（如 `1e40`）在表达式里怎样表现实现自定。
- **数字的文本**：十进制，不用指数；有小数部分时写小数点与小数位，不带结尾的零，没有小数部分时不写小数点；负数以 `-` 开头；零写 `0`。如 `1.5`、`-0.25`、`100`、`0.0000000000000000000000000001`（2026-11-24 修订，ADR-0287；以前 .NET 照 `decimal` 内部记着的小数位数写出：`1.50` 写成 `1.50`，`1.5 + 1.5` 写成 `3.0`，`2.50 * 2` 写成 `5.00`，同一个数值因来历不同写出不同的文本，别的语言也无从得知这个位数）。
- **表达式得出的数字写成 JSON**（结果存进变量、作为输出映射与 `SetVariable` 的值交出）时写数字的文本：字面量与运算、函数的结果只有数值，没有别的写法（2026-11-25 写明，ADR-0303；以前 .NET 照 `decimal` 内部记着的小数位数写：`1.5 + 1.5` 存进变量是 `3.0`，字面量 `1.50` 是 `1.50`）。所以这样的数字存进变量再读回来，原文就是数字的文本，读它的工作量与存之前相同（见「长度与上限」的「工作量」）；变量里原来就有的数字照它的原文，不改写。

**值的文本**——拼接、模板、`toString` 与各字符串函数把值当文本用时：

- `null` 是空串；布尔是 `true` / `false`；数字是数字的文本；字符串是它自己。
- 数组与对象是紧凑的 JSON 文本，写法与消息模板里的容器相同（RFC-0010「模板」）：没有空白；对象的成员按名字的序数顺序（逐个 UTF-16 码元比较）；里面的 `null` 写 `null`，布尔写 `true` / `false`，数字写数字的文本；字符串按 RFC-0010 的 JSON 字符串写法（只转义 `"`、`\` 与 U+0000–U+001F，非 ASCII 字符与 `<`、`>`、`&`、`'`、`+`、`/` 原样）；超过 64 层的容器写成 `null`（2026-11-24 修订，ADR-0287；以前是 System.Text.Json 的缺省写法：非 ASCII 字符与 `<`、`>`、`&`、`'`、`+`、`"` 写成 `\uXXXX`，数字照读入时的原文（`2.50`、`1e2`），成员照原来的顺序）。

### 词法

- 空白是 RFC-0001「空白字符」的 25 个码位，可以出现在任意两个记号之间，没有别的作用；字符串字面量里的空白是内容。没有注释。
- **数字字面量**：从一个十进制数字、或后面紧跟十进制数字的 `.` 开始，取尽连续的十进制数字与 `.`；取到的文本必须合 `[0-9]+(\.[0-9]*)?|\.[0-9]+`（`007`、`1.`、`.5` 都合法），它的数值放进数字，不合文法或放不下 → `workflow.expression.not_number`（`value` 是取到的文本），如 `1.2.3`、`1.5.x`（取到的是 `1.5.`）、`79228162514264337593543950336`。取文本时「十进制数字」按 Unicode 一般类别 Nd 判，所以 `٣`、`１２` 这样的非 ASCII 数字也走到这条错误。没有符号（`-5` 是一元 `-` 作用于 `5`）、指数（`1e3` 是 `1` 后面多出 `e`）、十六进制与分隔符。
- **字符串字面量**：`'…'` 或 `"…"`，到同一种引号为止。里面 `\n`、`\t`、`\r` 是换行、制表符与回车，`\` 后面跟别的任何一个字符表示那个字符本身（`\\`、`\'`、`\"`；`\u0041` 是 `u0041` 五个字符，没有 Unicode 转义）；其余字符（含换行与另一种引号）是内容。到结尾还没闭合（含以单个 `\` 结尾）→ `workflow.expression.unterminated_string`。
- **标识符**：字母或 `_` 开头，后接字母、十进制数字或 `_`，区分大小写。字母是 Unicode 一般类别 L（Lu、Ll、Lt、Lm、Lo），十进制数字是 Nd，都只看基本多文种平面——平面以外的字符不是字母也不是数字；汉字、带重音的字母都能作变量名。类别取自实现所带的 Unicode 数据，新近收录、各版本判得不一样的字符算不算字母实现自定。
- 关键字 `true`、`false`、`null` 只认小写；`True`、`NULL` 是普通标识符。
- 运算符与标点：`||`、`&&`、`==`、`!=`、`<=`、`>=`、`<`、`>`、`+`、`-`、`*`、`/`、`%`、`!`、`.`、`[`、`]`、`(`、`)`、`,`。`<`、`>`、`!` 后面紧跟 `=` 时总读成两个字符的运算符（`!=a` 不是 `!` 接 `=a`）；两个字符的运算符中间不能有空白；单独的 `=`、`&`、`|` 不是记号。

### 文法

```text
expression     = or ;
or             = and { "||" and } ;
and            = equality { "&&" equality } ;
equality       = comparison { ( "==" | "!=" ) comparison } ;
comparison     = additive { ( "<=" | ">=" | "<" | ">" ) additive } ;
additive       = multiplicative { ( "+" | "-" ) multiplicative } ;
multiplicative = unary { ( "*" | "/" | "%" ) unary } ;
unary          = ( "!" | "-" ) unary | postfix ;
postfix        = primary { "." identifier | "[" expression "]" } ;
primary        = "(" expression ")" | number | string | "true" | "false" | "null"
               | identifier [ "(" [ expression { "," expression } ] ")" ] ;
```

- 整段文本正好是一个 `expression`。优先级从低到高就是上面的次序，二元运算符都左结合：`10 - 4 - 3` 是 3，`1 < 2 < 3` 是 `(1 < 2) < 3`。
- `.` 后面的成员名是标识符，关键字的拼写也可以（`a.null` 读名为 `null` 的成员）；别的名字用索引取：`a['0']`、`a["with space"]`。
- 标识符后面（可以隔着空白）是 `(` 即函数调用，否则是变量；变量与函数可以同名。后缀只接在原子之后：`(a).b[0]` 合法，`a.b(1)` 不合法。
- 解析时的错误，`Validate` 就能报出，报最先遇到的一个：

  | 情形 | 消息键（都在 `workflow.expression.` 下）与参数 |
  | --- | --- |
  | 去掉空白之后什么都没有 | `empty` |
  | 该是原子的地方已经到结尾，如 `1 +`、`(`、`len(` | `unexpected_end` |
  | 该是原子的地方是别的字符（`+1`、`()`、`[1]`），或一个完整的表达式之后还有字符（`1 2`、`1)`、`a = b`、`1e3`） | `unexpected_character`：`character` 是那个字符（基本多文种平面以外的是整个字符），`position` 是它的位置，从 1 起 |
  | 括号、实参表或索引没有等到 `)`、`]` | `token_expected`：`token` 是 `)` 或 `]`，`position` 是它该出现的位置（跳过空白之后；到了结尾是长度加 1） |
  | `.` 后面不是标识符 | `member_expected`：`position` 是该出现成员名的位置（跳过空白之后） |
  | 数字字面量、字符串字面量、长度与嵌套 | `not_number`、`unterminated_string`、`too_long`、`too_deep`，见上文 |
  | 函数名不认识 | `unknown_function`：`function` 是原样的拼写 |
  | 实参个数不对 | 个数固定的函数是 `argument_count`（`function`、`count`），个数是一个范围的是 `argument_range`（`function`、`min`、`max`） |

  函数名与实参个数在读完这次调用的 `)` 时核对，先核对名字：实参里的错误先报，`nope(1 +)` 是 `unexpected_character`；`nope(1) +` 是 `unknown_function`；没等到 `)` 的 `nope(1` 是 `token_expected`。

### 求值

- **作用域**：标识符按名字（区分大小写）查，先查引擎为这次求值临时加的名字，再查实例变量。临时的名字只有输出映射里的 `outputs`（本节点的输出）；没有别的内置名字。查不到 → `unknown_variable`（`name`），不是 `null`；查到而值是 `null` 就是 `null`。
- **次序**（2026-11-24 修订，ADR-0287；以前 `/` 与 `%` 先求值右操作数并先看除数，`-`、`*` 核对完左操作数才求值右操作数，`round` 先求值第二个实参）：二元运算符先求值左操作数、再求值右操作数，两个都求值之后才核对类型（先左后右），除法与取余最后才看除数是不是零；函数的实参从左到右全部求值之后才核对；索引先求值被索引的值、再求值下标。`&&` 与 `||` 例外，见下。遇到第一个错误即停：`nope / 0` 是 `unknown_variable`，`'x' / 0` 是 `operands_not_numeric`，`'x' - nope` 是 `unknown_variable`。

### 运算符

| 运算符 | 操作数 | 结果 |
| --- | --- | --- |
| `\|\|`、`&&` | 布尔 | 布尔。先求值左操作数，它必须是布尔；`\|\|` 的左操作数为真、`&&` 的左操作数为假时结果就是它，右操作数不求值也不核对（`true \|\| nope`、`false && 1` 都成功）；否则求值右操作数，它也必须是布尔，结果是它 |
| `!` | 布尔 | 取反 |
| `==`、`!=` | 任意 | 布尔，见下；`!=` 是 `==` 的否定 |
| `<`、`<=`、`>`、`>=` | 两个标量 | 布尔，见下 |
| `+` | 见右 | 任一侧是字符串时是拼接：两侧各取值的文本接起来（`1 + '2'` 是 `'12'`，`'a' + null` 是 `'a'`，`'' + 1.50` 是 `'1.5'`）；否则两侧都必须是数字，结果是和放进数字 |
| `-`、`*`、`/`、`%` | 两侧都能当数字 | 差、积、商放进数字；`%` 是截断除法的余数（a − b × trunc(a / b)），符号随被除数，总是精确的：`-7 % 3` 是 −1，`7 % -3` 是 1，`5.5 % 2` 是 1.5。除数是零 → `division_by_zero` |
| 一元 `-` | 能当数字 | 相反数（`-'5'` 是 −5，`-0` 是 0） |

- 「布尔」只指 JSON 的 `true` 与 `false`：字符串 `'true'`、数字与 `null` 都不是。操作数不是布尔 → `operands_not_boolean`（`operator` 是 `||`、`&&` 或 `!`）。
- 算术的操作数不能当数字 → `operands_not_numeric`（`operator` 是 `+`、`-`、`*`、`/`、`%`，一元负也是 `-`）。注意 `+` 只在两侧都不是字符串时才是加法，这时数字文本不出现（它是字符串，走拼接）。
- 和、差、积、商放不下 → `number_out_of_range`（`operator`；2026-11-24 修订，ADR-0287：以前 .NET 的溢出异常直接逃出求值器）。
- **相等**：
  1. 有一侧是 `null`：两侧都是 `null` 才相等。
  2. 两侧都是标量：都是布尔，按值；否则两侧都能当数字，按数值（`1 == 1.0`、`1 == '1'`、`1000 == '1e3'`、`'007' == '7'` 都为真）；否则按两侧值的文本的序数比较（`true == 'true'` 为真，`'abc' == 'ABC'`、`1 == 'one'`、`true == 1` 为假）。字符串不做 Unicode 规范化；时间文本不按时刻比（时刻相同、写法不同的两段文本不相等，要按时刻比先各自过 `date()`）。
  3. 其余（至少一侧是容器）：按结构深比较——数组长度相同且逐项相等，对象成员名相同且各成员相等（不看成员的顺序），容器里的标量不做上面的转换：类型相同才可能相等，数字按放进数字之后的数值，字符串逐个码元；容器与标量不相等（`[1, 2]` 不等于它的文本 `'[1,2]'`）。
- **比较**：两侧必须都是标量，有一侧是 `null` 或容器 → `operands_not_scalar`（`operator`）。两侧都能当数字时按数值（`'10' > '9'`、`'10' > 9` 为真）；否则两侧都是合「时间」文法的字符串时按时刻；否则按两侧值的文本的序数——逐个 UTF-16 码元比较，较短的是较长的前缀时较短的在前（`'a' < 'B'` 为假，`10 < '9a'` 为真，`true > false` 为真）。

### 成员与索引

- `a.name`：`a` 是对象时取名为 `name` 的成员，没有这个成员是 `null`；`a` 是 `null` 时是 `null`（所以 `a.b.c` 在中途缺少成员时一路是 `null`）；`a` 是标量或数组 → `member_of_non_object`（`member`）。成员名与变量名一样按序数比较、区分大小写（2026-11-24 修订，ADR-0287；以前 .NET 里区不区分取决于对象是怎样造出来的：按不区分大小写的选项反序列化出来的对象不区分，写进存储再读回来又区分）。
- `a[i]`，先求值 `a` 再求值 `i`：
  - `a` 是 `null`：结果是 `null`，不看 `i` 的类型。
  - `a` 是数组：`i` 必须能当数字，否则 `operands_not_numeric`（`operator` 是 `[]`）；下标截去小数部分（向零取整，`arr[1.9]` 是 `arr[1]`），从 0 起；小于 0 或不小于长度 → `index_out_of_range`（`index` 是截去小数之后的下标，`length` 是数组的长度；2026-11-24 修订，ADR-0287：以前下标超出 32 位整数时 .NET 的转换异常逃出求值器）。
  - `a` 是对象：取名为 `i` 的值的文本的成员（`obj[0]` 取名为 `0` 的成员，`obj[null]` 取名为空串的成员），没有是 `null`。
  - `a` 是标量 → `index_of_scalar`。

### 内置函数

函数名不区分 ASCII 字母的大小写（`LEN`、`isnullorempty`）。实参个数在解析时核对。下表的「文本」指值的文本，「数字」指能当数字的值——不能当数字 → `operands_not_numeric`，`operator` 是表里写的函数名。

| 函数 | 结果 |
| --- | --- |
| `len(x)` | 数字：`null` 是 0，数组是元素个数，对象是成员个数，标量是它的文本的长度（UTF-16 码元：`len(12345)` 是 5，`len(1.50)` 是 3，`len('😀')` 是 2） |
| `contains(x, y)` | 布尔：`x` 是数组时，有没有一个元素与 `y` 相等（「相等」的规则，只看这一层）；否则 `x` 的文本是否包含 `y` 的文本，按序数（任何文本都包含空串，所以 `contains(x, null)` 在 `x` 不是数组时为真） |
| `startsWith(x, y)`、`endsWith(x, y)` | 布尔：`x` 的文本是否以 `y` 的文本开头 / 结尾，按序数 |
| `upper(x)`、`lower(x)` | 字符串：`x` 的文本逐个码位取 Unicode 的简单大写 / 小写映射——UnicodeData.txt 的 Simple_Uppercase_Mapping / Simple_Lowercase_Mapping 字段，没有映射的码位不变——一个码位对一个码位，不做 SpecialCasing.txt 的完全映射，不随区域、不看上下文；与「定义」的「关键字的小写」是同一份映射（2026-11-25 修订，ADR-0303；以前 U+0130 与 U+0131 怎样转换实现自定，.NET 的不变文化不转换它们）。所以 `lower('İ')` 是 `i`、`upper('ı')` 是 `I`，`upper('i')` 是 `I` 而不是 `İ`；`ß`、`ŉ`、`ﬀ` 的大写不变（完全映射才是两个字符）；`lower('ẞ')` 是 `ß`，`upper('ſ')` 是 `S`；`Σ` 的小写总是 `σ`；`ǅ` 的大写是 `Ǆ`、小写是 `ǆ`；基本多文种平面以外的字母按整个码位转换（`upper('𐐨')` 是 `𐐀`）。简单映射不跨平面，转换前后的长度相同。结果以实现所带的 Unicode 数据为准，依据与向量的取字同「关键字的小写」 |
| `trim(x)` | 字符串：`x` 的文本去掉首尾的空白（RFC-0001「空白字符」） |
| `isNullOrEmpty(x)` | 布尔：`null`、空串、空数组、空对象为真，其余为假（`0`、`false`、只有空格的字符串都为假） |
| `abs(x)` | 数字的绝对值 |
| `min(x, y)`、`max(x, y)` | 两个数字里较小 / 较大的 |
| `round(x)`、`round(x, n)` | 数字 `x` 舍入到 `n` 位小数（缺省 0），正好一半时远离零（`round(2.5)` 是 3，`round(-2.5)` 是 −3，`round(1.005, 2)` 是 1.01）。`n` 是数字，截去小数部分之后必须在 0 到 28 之间，否则 `number_out_of_range`（`operator` 是 `round`；2026-11-24 修订，ADR-0287：以前 .NET 的参数异常逃出求值器） |
| `format(x, n)` | 字符串：数字 `x` 写成恰好 `n` 位小数的定长文本（2026-11-25 新增，ADR-0296）。先照 `round(x, n)` 舍入（正好一半时远离零），再写出：整数部分照数字的文本，负数以 `-` 开头，没有 `+`；`n` 大于 0 时接小数点 `.` 与 `n` 位小数，不足的补零，`n` 为 0 时不写小数点；没有千分位，不随区域。舍入之后是零的不写负号（数字没有负零）：`format(-0.001, 2)` 是 `0.00`，`format(-0.005, 2)` 是 `-0.01`。如 `format(19.9, 2)` 是 `19.90`，`format(2.5, 0)` 是 `3`，`format('1e2', 1)` 是 `100.0`。`n` 必须能当数字，它的值必须是 0 到 28 之间的整数（`2.0` 与 `'2'` 可以，`2.5` 不行——与 `round` 截去小数部分不同），否则 `number_out_of_range`（`operator` 是 `format`）；`x` 先于 `n` 核对。结果最长 59 个码元（符号、29 位整数、小数点、28 位小数），远在字符串的上限以内；它是字符串，不受数字的范围约束，当数字用时（`toNumber`、算术、比较）读回的就是 `round(x, n)` |
| `toNumber(x)` | 数字：`x` 能当数字时是它的数值 |
| `toString(x)` | 字符串：`x` 的文本（`null` 是空串） |
| `now()` | 字符串：求值时的时刻的时间文本 |
| `date(x)` | 字符串：`x` 的文本按「时间」的文法读出的时刻，写成时间文本；读不出 → `not_date`（`value` 是 `x` 的文本） |

`operands_not_numeric` 的 `operator` 取值：`abs`、`min`、`max`、`round`、`format`、`toNumber`。

### 时间

- **时间文本的读法**（比较与 `date()` 用；2026-11-24 修订，ADR-0287）：`^[0-9]{4}-[0-9]{2}-[0-9]{2}([Tt ][0-9]{2}:[0-9]{2}(:[0-9]{2}(\.[0-9]{1,9})?)?([Zz]|[+-][0-9]{2}:[0-9]{2})?)?$`，按整串匹配，首尾的空白不裁剪。只有日期是当天零点；没有秒是零秒；没有偏移是 UTC；日期与时间之间是 `T`、`t` 或一个空格。取值范围与截止时间的文本相同（RFC-0003 规则 8）：年 `0001`–`9999`、月日按公历（含闰年）、时 `00`–`23`、分与秒 `00`–`59`（不认闰秒）、偏移的时 `00`–`23` 与分 `00`–`59`，换算成 UTC 之后在 `0001-01-01T00:00:00` 到 `9999-12-31T23:59:59` 之间；比 100 纳秒更细的小数截断，不进位。它是 RFC-0003 规则 8 的读法的超集。以前交给 .NET 的日期解析：月份名、斜线日期（`01/02/2026`）、不补零的月日、只有年月都读得出，只有时间的（`12:30`）补上机器当天的日期——同一段文本在不同的日子读出不同的时刻，两个这样的字符串比大小的结果也跟着变。
- **时间文本的写法**（`now()` 与 `date()` 的结果）：与截止时间相同（RFC-0003 规则 8）——UTC、七位小数、`+00:00`，如 `2026-01-02T03:04:05.0000000+00:00`，时钟精度不足的补零，比 100 纳秒更细的截断。它定宽 33 个字符，同一个时刻只有一种写法：两个这样的文本按序数比较的先后就是时刻的先后，`date(a) == date(b)` 就是按时刻比相等。
- `now()` 与 `date()` 的结果是字符串（2026-11-24 修订，ADR-0287；以前 .NET 返回进程内的时间值：它的文本带七位小数，写进变量、过一遍存储之后成了不带多余小数位的字符串，同一个值前后写出不同的文本）。

### 条件

`EvaluateCondition`（转移条件）：求值的结果必须是布尔，否则——含 `null`、字符串 `'true'`、数字——→ `workflow.expression.not_boolean`；求值本身失败时是那个错误。

### 模板

`Render(template, scope)` 把模板里的每个 `{{ 表达式 }}` 换成表达式结果的文本，结果总是字符串（整个模板只有一个占位时也是：`{{ 1 + 1 }}` 渲染成字符串 `2`）：

1. 从左到右找下一个 `{{`。没有：余下的文本原样写出，结束。
2. `{{` 之前的文本原样写出。从 `{{` 之后逐个字符找结束的 `}}`：不在引号里时，`'` 或 `"` 进入这种引号，相邻的两个 `}` 就是结束；在引号里时，`\` 连同它后面的一个字符一起跳过，同一种引号退出。找到结尾还没有 → `unterminated_placeholder`（`expression` 是模板）。
3. `{{` 与 `}}` 之间的文本去掉首尾的空白，当表达式求值（为空是 `empty`），把结果的文本写出：`null` 是空串，字符串原样、不加引号也不转义，数字是数字的文本，容器是紧凑的 JSON。求值失败即渲染失败，`expression` 参数是这段去掉首尾空白的文本，位置按它算。
4. 从 `}}` 之后继续。写出的文本不再解释：结果里的 `{{` 不是占位。

占位之外的 `{`、`}`、`}}` 与 `\` 都是普通文本；要写出字面的 `{{` 用 `{{ '{{' }}`。`{{ name }}}` 是占位后面跟一个 `}`；`{{{ name }}}` 的第三个 `{` 进了表达式，是 `unexpected_character`。模板本身不限长（它是定义里的属性值），每个占位里的表达式各自受 4000 的上限；整个模板的全部占位共用一份工作量，每个占位的结果在接上之前另记它的读取量（见「长度与上限」的「工作量」）；写出的文本累计不超过 4194304——每接上一段字面文本或一个占位的文本之前按累计长度核对，超过 → `string_too_long`（`length` 是接上这一段之后的长度，`max`，`expression` 是模板；2026-11-24 修订，ADR-0287：以前渲染的结果没有上限，占位可以是整个变量的 JSON 文本，一个不长的模板能渲染出几个 GB）。

### 错误

错误码都是 `WORKFLOW_VALIDATION_003`，消息键在 `workflow.expression.` 下，每个错误都带 `expression` 参数（出错的表达式，按「长度与上限」截断；模板的 `unterminated_placeholder`、`string_too_long` 与记占位的结果时超过的 `budget_exceeded` 带的是模板）：

| 消息键 | 其余参数 | 何时 |
| --- | --- | --- |
| `empty`、`unexpected_end`、`unterminated_string` | | 解析 |
| `unexpected_character` | `character`、`position` | 解析 |
| `token_expected` | `token`、`position` | 解析 |
| `member_expected` | `position` | 解析 |
| `not_number` | `value` | 解析 |
| `too_long` | `length`、`max` | 解析 |
| `too_deep` | `max` | 解析 |
| `unknown_function` | `function` | 解析 |
| `argument_count` | `function`、`count` | 解析 |
| `argument_range` | `function`、`min`、`max` | 解析 |
| `unknown_variable` | `name` | 求值 |
| `operands_not_boolean`、`operands_not_numeric`、`operands_not_scalar` | `operator` | 求值 |
| `division_by_zero`、`index_of_scalar` | | 求值 |
| `number_out_of_range` | `operator`（`+`、`-`、`*`、`/`、`round` 或 `format`） | 求值（2026-11-24 新增，ADR-0287；`format` 是 2026-11-25 加的，ADR-0296） |
| `budget_exceeded` | `max` | 求值、渲染（2026-11-25 新增，ADR-0296） |
| `member_of_non_object` | `member` | 求值 |
| `index_out_of_range` | `index`、`length` | 求值 |
| `not_date` | `value` | 求值 |
| `string_too_long` | `length`、`max` | 求值、渲染 |
| `not_boolean` | | 条件 |
| `unterminated_placeholder` | | 渲染 |

「解析」的错误只校验语法（`Validate`、定义校验）就报出；其余的语法合法，求值时才报。

## 存储契约

三个存储是作用域服务，按分区读写：每个方法的第一个参数 `tenantId` 是分区（空为平台），只返回这个分区的记录，写入的记录的 `TenantId` 必须等于它（不等是编程错误，抛出 `ArgumentException`，不改写、不落库）；不同分区的同一 ID 互不相干。数据库实现在调用方的工作单元里读写，分区必须等于工作单元的租户（不等抛出 `InvalidOperationException`）。不做乐观并发（实例锁串行化写入），写入按批量（ADR-0012）：

- `IWorkflowDefinitionStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`FindAsync(tenantId, name, version)`、`FindLatestPublishedAsync(tenantId, name)`、`MaxVersionAsync(tenantId, name)`、`ListAsync(tenantId, name?, status?)`（整份定义，名字升序、版本降序，不分页；引擎的信号启动用）、`ListSummariesAsync(tenantId, WorkflowDefinitionQuery)`（定义列表：按关键字、名字、分类、状态、创建时间区间过滤，按排序项 `Sorts` 与次序键排序（缺省名字升序、版本降序），跳过 `Skip` 条后最多 `Take` 条，返回摘要；分页与排序字段已由调用方校验）、`CountAsync(tenantId, WorkflowDefinitionQuery)`（同条件的总数，不看分页）、`SaveAsync(tenantId, definitions[])`（新增或整体替换）、`DeleteAsync(tenantId, ids[])`。
- `IWorkflowInstanceStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`ListAsync(tenantId, WorkflowInstanceQuery)`（`Status` / `Statuses`、`DefinitionName`、`CorrelationIds`（空集合不匹配）、`ParentInstanceId`、`TopLevelOnly`、`StarterId`、`Keyword`、`CreatedFrom` / `CreatedTo`、`EndedBefore`、`Skip`、`Take` ≤ 1000；按排序项 `Sorts` 与次序键排序，缺省创建时间降序；分页、关联 ID 集合的上限与排序字段已由调用方校验）、`CountAsync(tenantId, query)`、`SaveAsync(tenantId, instances[])`、`DeleteAsync(tenantId, ids[])`（级联节点实例）；节点实例 `FindNodeInstanceAsync(tenantId, id)`、`NodeInstancesAsync(tenantId, instanceId)`（开始时间升序、同刻按创建顺序）、`SaveNodeInstancesAsync(tenantId, nodeInstances[])`。
- `IWorkflowBookmarkStore`：`FindAsync(tenantId, id)`、`FindManyAsync(tenantId, ids[])`、`ByInstanceAsync(tenantId, instanceId)`、`DueAsync(tenantId, now, limit)`（`DueAt` ≤ now，`DueAt` 升序；轮询器逐个分区调用）、`ByKindAndKeyAsync(tenantId, kind, key)`（创建时间升序）、`AvailableAsync(tenantId, kind, key, now, limit)`（种类与键匹配、`DueAt` 为空或 ≤ now，创建时间升序，最多 limit 条；外部任务领取用）、`BySignalAsync(tenantId, name, correlationId?)`、`ListUserTasksAsync(tenantId, UserTaskQuery)`（种类为 `UserTask`、所属实例 Running 的书签，指派人匹配书签的键，其余条件匹配实例、节点、所属实例的定义名与创建时间，关键字匹配书签载荷的 `title`、所属实例的标题、定义名或关联 ID；按创建时间排序、同刻按写入先后，跳过 `Skip` 条后最多 `Take` 条；实例状态变化后书签随之进出结果）、`CountUserTasksAsync(tenantId, UserTaskQuery)`（同条件的总数）、`SaveAsync(tenantId, bookmarks[])`、`DeleteAsync(tenantId, ids[])`、`DeleteByInstanceAsync(tenantId, instanceIds[])`。
- `IWorkflowTransaction`（作用域）：`IsActive`（当前作用域在不在可以加入的调用方事务里）、`EnsureWritable()`（只读的调用里返回拒绝写入的错误：数据库实现看工作单元，内存实现看操作契约），见「事务」。
- 存储不可达 → `WORKFLOW_UNAVAILABLE_001`；容量已满（内存默认实现）→ `WORKFLOW_UNAVAILABLE_002`。

## 默认实现与 Provider

- **轮询**（`DueBookmarkPoller`）：每个周期覆盖平台分区与租户目录（`ITenantStore`，装配了多租户时）里启用的每个租户分区：逐个分区开一次调用取最多 `BatchSize` 条到期书签，合并后按到期时间取前 `BatchSize` 条，逐条在以书签租户开的调用里恢复（每次恢复一个事务；Tenant 阶段为它绑定租户的分片）；实例不在运行中的在同一调用里把到期时间后推 `NotResumableBackoff`。停用的租户不轮询，它的到期书签等重新启用后再接续；新租户在进入租户目录后的下一个周期纳入；一个分区不可达（库连不上、租户校验失败）只记日志、不挡住别的分区。到了 `HousekeepingIntervalSeconds` 且 `RetentionDays` > 0 时逐个分区清理。
- `Workflow.InMemory`（`role: default`）：三个存储是有界内存表（`XiHan:Workflow:InMemory:MaxDefinitions` 默认 1000、`MaxInstances` 10000、`MaxNodeInstances` 100000、`MaxBookmarks` 100000，全部分区合计），键是分区加 ID，保存深拷贝；没有事务（见「事务」）；实例锁是按实例的进程内信号量；`DueBookmarkPoller` 由特性在应用进入服务状态后以 `XiHan:Workflow:InMemory:PollIntervalSeconds`（默认 1）轮询，每次最多恢复 `BatchSize`（默认 100）条，停止时等待进行中的波次。
- `Workflow.Database`（`role: provider`，依赖 `persistence.relational@1`，可选依赖 `multitenancy.context@1`）：定义、实例、节点实例、书签以 JSON 文档列加查询列的实体在调用方的工作单元里经仓储读写。租户分区的四张表 `WorkflowDefinitionRecord`、`WorkflowInstanceRecord`、`WorkflowNodeInstanceRecord`、`WorkflowBookmarkRecord` 实现 `IMultiTenant`：持久化按工作单元的租户写入与过滤，租户有独立库时落它的分片连接；平台分区的四张表 `PlatformWorkflow*Record` 不实现 `IMultiTenant`，落平台库。书签行另有待办关键字下推用的两列（2026-11-02 修订，ADR-0096、ADR-0099）：`SearchTitle`（载荷 `title` 折叠后的值，写书签时算出）与 `SearchInstanceText`（所属实例一侧的被搜文本：实例标题、关联 ID、定义名各自折叠后以 U+001F 拼成，即实例行的 `SearchText` 再拼上折叠后的定义名；写书签时从实例行算出、实例新增或这几个字段变了同步，与定义名、实例状态同一路径），都是长文本、可空，升级前的行要回填；关键字对两列各自判非空后做包含判断、取或。定义行另有定义列表下推用的两列（2026-11-02 修订，ADR-0096）：`Category`（varchar(128)，精确过滤）与 `SearchText`（长文本，名字、显示名、说明各自折叠后以 U+001F 拼成，关键字对它做包含判断）；两列可空只为给已有行的表加列，存储写入的行总有值，升级前写入的行要从 `Document` 回填，没回填的行不被分类与关键字匹配。实例行另有 `SearchText`（长文本，实例标题与关联 ID 各自折叠后以 U+001F 拼成，关键字对它做包含判断；可空、回填同上；2026-11-02 修订，ADR-0099：原来的 `SearchTitle` 只有标题，改名并加入关联 ID；书签行的 `SearchInstanceTitle` 同样改名 `SearchInstanceText`，两处列名已定，ADR-0101），定义行与实例行另有排序用的 `Title`（varchar(256)，可空只为给已有行的表加列；`WorkflowSortKeys.Title(标题)`，定义是显示名或名字、实例是实例标题；2026-11-02 修订，ADR-0102），升级前写入的行要回填，没回填的行按标题排序时位置照数据库的空值规则；定义行与实例行另有状态序号 `StatusOrder`（整数，可空只为给已有行的表加列；状态在生命周期里的序号，定义 `Draft` 0、`Published` 1、`Disabled` 2、`Archived` 3，实例 `Running` 1、`Suspended` 2、`Completed` 3、`Canceled` 4、`Faulted` 5、`Terminated` 6；存储在每次写入行时随 `Status` 一起写，新增、状态变更、迁移、跳转、重试等都经整行写入；按状态排序时对它 `ORDER BY`；序号是存储内部的细节，不是公开常量；2026-11-02 修订，ADR-0103），升级前写入的行要按 `Status` 回填，没回填的行按状态排序时位置照数据库的空值规则；排序与定义的创建时间区间不加新索引（定义每个分区的行数少，实例按创建时间排序仍走 `(TenantId, CreatedAt)`、按结束时间走 `(TenantId, EndedAt)`，按标题、状态、定义名排序是对过滤后的行排序）。实例表加索引 `(TenantId, StarterId, CreatedAt)`（平台表 `(StarterId, CreatedAt)`），供按发起人倒序分页。书签行另有待办查询下推用的三列：`NodeId`（取自书签）、`DefinitionName` 与 `InstanceStatus`（取自所属实例，写书签时从实例行读，实例新增或状态变化时由实例存储同步到它的书签行，都在同一事务里）。定义的唯一键：租户定义 `(Name, TenantId, Version)`、平台定义 `(Name, Version)`，都不含可为空的列，每一行都受唯一索引约束（平台定义不以哨兵租户值存进租户表：`IMultiTenant` 的表在无租户的调用里写不进、读不到，见 ADR-0077）。实例锁是平台库里的租约行（`WorkflowLockRecord`；`XiHan:Workflow:Database:LockLeaseSeconds` 默认 60，持有期间自动续约，并发戳 CAS 接管过期租约；每一步在自己的作用域与可写工作单元里提交，不经管道）；轮询只在持有 `workflow.poller` 租约的节点上进行，领导权全局一份（`PollIntervalSeconds`、`BatchSize` 同上）；应用不在服务状态的周期不拿也不续领导权，持有就立即释放——排空要等依赖工作流的特性逐个停完，续着领导权会让在服务的另一个实例一直拿不到，释放后它下一个周期就接手（排空开始后至多一个轮询周期；2026-11-01 修订，ADR-0087）。租约表的连接（2026-11-01 修订，ADR-0084）：`XiHan:Workflow:Database:LockConnection` 给租约表指定命名连接（RFC-0005「命名连接」；空白视为未配置）——特性把 `WorkflowLockRecord` 登记到这个连接，租约的每一步与健康探针开绑定这个连接的工作单元，事务只开在租约库上。SQLite 上必须配：SQLite 只有一个写者，引擎在调用方事务里写过之后再写租约行，同一个库文件上要等的正是调用方自己的事务，忙等超时后以 `WORKFLOW_TIMEOUT_001` 失败；租约放到另一个库文件，业务调用不碰它，两边的事务互不等待。不配置时租约表照旧在默认连接（平台库），PostgreSQL 等行级锁的数据库不需要配。持久化 Provider 不支持命名连接（内存实现）时装配即失败（`PERSISTENCE_UNAVAILABLE_008`），连接不在 SqlSugar 客户端里时启动即失败（`PERSISTENCE_VALIDATION_004`）。EF Core 以 `XiHan:Persistence:EntityFrameworkCore:Connections:<name>` 声明这个连接（2026-11-01 修订，ADR-0087）：租约的每一步与健康探针本来就在自己的作用域里开绑定连接的工作单元，作用域里的 DbContext 按租约库的连接串建出；连接没有声明时装配即失败（`PERSISTENCE_VALIDATION_004`）。
- `Workflow.Database` 的**查询列回填**（2026-11-25 写明，ADR-0304；以前上文只写「升级前写入的行要回填」，没有给出做法——表结构由应用管理，包里没有架构版本与迁移）：查询列是定义行的 `StatusOrder`、`Category`、`SearchText`、`Title`，实例行的 `StatusOrder`、`SearchText`、`Title`，书签行的 `NodeId`、`DefinitionName`、`InstanceStatus`、`SearchTitle`、`SearchInstanceText`，都由行里的文档（书签行的定义名、实例状态与实例一侧的关键字另取所属实例的行）按存储写入行时同一份算法算出。规则变了（ADR-0296 改了关键字的小写，含 `İ` 或平面以外大写字母的行）或列是后来加的（升级前的行为空）时，由应用在升级之后调用单例 `DatabaseWorkflowBackfill` 的 `RunAsync(partitions, batchSize)` 把已有的行重新算出：
  - **按分区**：`partitions` 的每一项（`null` 为平台，其余是租户 ID）一项结果，与入参等长同序（批量优先的逐项结果）；一个分区失败（存储不可达 `WORKFLOW_UNAVAILABLE_001`、分片解析、提交或文档读不出的错误）只让那一项失败，别的分区照常；空集合不触存储，同一分区出现两次就回填两次。不经管道：租户分区在新作用域里绑定租户的分片，不看租户是否启用。空白的租户 ID 是编程错误，在动任何分区之前抛出 `ArgumentException`。
  - **分批**：定义与实例各按创建时间升序、同刻按 ID，每批至多 `batchSize` 行（1 到 `MaxBatchSize` = 1000，与一次查询最多取出的行数相同；越界抛出 `ArgumentOutOfRangeException`），连同归属它们的书签（按所属实例的 ID 或定义级定时书签的定义 ID，每页至多 `batchSize` 行）在一个只读事务里扫描，要改写的另开可写事务。游标只用创建时间的 `>=` 与同刻行的偏移，不靠字符串的大小比较（规约不保证它能翻译）；扫描之中别的写入删掉了同刻的行或书签时，后面的行在偏移里前移，可能有行这次没被扫到，再运行一次即可，新写入的行由存储按新规则算好。
  - **可以重复运行**：算出的值与行里现有的值相同的行不改写（文本列的空串与 `NULL` 当作相同：Oracle 把空串存成 `NULL`），所以同一分区第二次运行改写的行数是零。
  - **与引擎互斥**：实例行与归属它的书签行在实例锁内重读、再改写——锁的键是实例 ID 或定义 ID，与引擎写这些行时取的锁相同（「存储契约」：实例锁串行化写入）；一次只持有一个锁，引擎的一次调用可能先后持有几个锁，回填同时持有多个就可能与它互相等到超时。锁在 `LockTimeout` 内拿不到的这次跳过、计入结果的 `Busy`，它的行原样留着。定义行的写入本来不加锁，回填在一个短事务里重读再写，与同时进行的定义编辑之间只剩这一个窗口，宜在没有人编辑定义时运行。
  - **可取消**：每批之前检查取消，取消时抛出 `OperationCanceledException`；已经提交的批留着，进行中的那一批回滚。
  - **结果** `DatabaseWorkflowBackfillReport`：`Scanned`（扫描读到的行，定义、实例与书签合计）、`Rewritten`（改写的行）、`Busy`（锁被占用而跳过的键）、`Batches`（定义与实例读到了行的批数）。内存实现每次现算，没有查询列，不用回填。
- 引擎选项（`XiHan:Workflow`）：`MaxNodeExecutionsPerBurst`（1000）、`MaxSubWorkflowDepth`（16）、`LockTimeoutSeconds`（10）、`NotResumableBackoffSeconds`（300）、`ExpressionCacheSize`（10000）、`RetentionDays`（0 = 不自动清理）、`HousekeepingIntervalSeconds`（3600）、`HttpTimeoutSeconds`（30）、`MaxHttpTimeoutSeconds`（300，ADR-0181）、`MaxForEachItems`（1000）、`MaxVariablesBytes`（8388608，ADR-0182）。引擎与两个 Provider 的选项读取时值不合法即抛出（解析不了 → `ArgumentException`，超出取值范围 → `ArgumentOutOfRangeException`，消息带配置键与原值），应用启动失败，不按缺省处理；`XiHan:Workflow:Database:HolderId` 空白视为未配置。

## 一致性

Provider 继承 `WorkflowStoreConformance`（三个存储的查询、排序、分页与计数、批量与级联语义、可领取书签的过滤与上限、待办查询的过滤 / 排序 / 分页 / 计数且随实例状态进出、待办的关键字（任务标题、实例标题、定义名或关联 ID，实例标题变了随之同步）、按分区的名字空间、分区互不可见、写入别的分区的记录即抛出、实例锁、定义查询的过滤 / 排序 / 分页 / 计数与关键字按字面且不跨字段匹配、实例查询的关联 ID 集合、只取顶层与关键字（标题或关联 ID）、定义查询的创建时间区间、定义与实例查询每个排序字段的升降序与次序键（含标题的前 256 个字符相同、可空时间只比较有值的行、状态按生命周期且状态变了随之换位置）、多字段排序（主次字段的组合、升降混合、含状态字段、分页稳定））与 `WorkflowEngineConformance`（顺序流、决策、并行与汇合、三种完成策略与阈值的用户任务、认领与退回、转办与加签、延时与超时、信号与事件网关、信号与定时启动、子流程与 ForEach、外部任务、HTTP 活动、补偿活动、重试与继续、取消 / 终止 / 挂起 / 恢复 / 跳转 / 迁移 / 改变量、实例级超时、清理、事件主题、并发的拒绝重复启动只建一个实例、定义按分区隔离、实例按分区隔离、业务回滚丢弃引擎写入、办理任务随业务提交或回滚、实例锁持有到调用结束且回滚后释放、只读调用在执行活动之前拒绝写入、轮询覆盖平台与每个启用的租户、目录的定义查询与分页越界、实例查询只取顶层与一组关联 ID 及其越界、定义与实例查询的排序经引擎下推且排序项越界（字段不在白名单、重复、项数超过上限）是校验错误、实例经引擎改了状态之后按状态排序的位置随之变化、按实例列出全部等待点且分区隔离）并通过全部用例；事务相关的用例按 `JoinsCallerTransaction` 断言（内存实现为否），只读调用的用例两种实现同一断言（2026-11-02 修订，ADR-0099）。套件在代理一侧记录引擎发出的事件（2026-11-02 修订，ADR-0096）：记录器 `ConformanceLog` 登记成 `IMessageBroker`（Provider 以 `TryAdd` 登记代理与直接发布者，先登记的记录器不被顶替；没装消息 Provider 的宿主由套件补上直接发布者），宿主装了发件箱时事件随事务写进发件箱，夹具 `WorkflowEngineHost` 在每次操作结束之后调 `DrainOutboxAsync` 把已提交的事件投递给记录器——回滚的不送达，与「事件」一节的语义一致；同一信封按 ID 只记一次。记录器只把来源是 `/workflow`（`WorkflowTopics.Source`）的信封当工作流事件解析；宿主里别的发布端（调度的作业事件、应用自己的消息）送来的照常确认成功、不解析，只记主题（2026-11-03 修订，ADR-0096）——它们不因套件失败或记告警，发件箱的投递轮次也不因别的主题回滚。记录器只记录、不消费，套件运行时宿主别的消息订阅收不到消息。应用可以在装了发件箱、装了调度等会发布事件的特性的真实宿主上跑套件。

向量（档 `workflow`，`conformance/vectors/workflow/`；2026-11-24，ADR-0277，以前这个档没有向量）：

| 文件 | 内容 |
| --- | --- |
| `workflow-names.json` | 定义名、节点与转移的 ID、活动类型三条文法 |
| `status-order.json` | 定义与实例的状态按生命周期先后的次序 |
| `keyword-matching.json` | 列表查询的关键字：折叠（Unicode 的简单小写映射，2026-11-25 补了 `İ`、平面以外的字母等用例，ADR-0296）与按字面的包含匹配 |
| `workflow-event-topics.json` | 引擎发布的 25 个事件主题与信封的 `source` |
| `expression-syntax.json` | 表达式的词法与文法：字面量、空白、标识符、优先级与结合性、成员与索引与调用的写法，解析时的每一种错误与位置 |
| `expression-operators.json` | 每个运算符与每种操作数的组合，数字的舍入与范围，数字文本，除以零与求值的先后，拼接，相等，比较，逻辑的短路，成员与索引 |
| `expression-functions.json` | 17 个内置函数的结果与错误（`format` 是 2026-11-25 加的，ADR-0296；`upper` / `lower` 的 `İ`、`ı`、`ẞ`、`ſ`、`ǅ`、开尔文符号、没有简单映射的字符、平面以外的字母与不变的字符是 2026-11-25 补的，ADR-0303），数字文本与时间文本的合法与不合法写法 |
| `expression-templates.json` | 模板的渲染与失败，条件 |
| `expression-limits.json` | 长度、嵌套、拼接与渲染的上限两侧，链不算嵌套，错误里带出的文本的截断，工作量的计数与上限两侧（2026-11-25 加的，ADR-0296；读数字按原文的长度、原文很长的数字变量与读它到上限两侧是同日加的，ADR-0303；`len` 读字符串及其到上限两侧、`isNullOrEmpty` 只读到能判定为止是 2026-11-26 加的，ADR-0309） |

表达式的五个文件是 2026-11-24 加的（ADR-0287；ADR-0277 那一轮因为正文没有逐条写出而没有收）：期望值全部由 .NET 的求值器算出，`workflow.expression.` 下的每个消息键至少有一条用例。Go 一致性模块只读取并核对这五个文件的结构，不另实现一个求值器；实现的回放照向量文件顶部的说明。

向量不收的：表达式里标了实现自定的几处（放不下的 JSON 数字、新近收录的字符算不算字母）、关键字的折叠与 `upper` / `lower` 里各实现的 Unicode 数据可能不同的字符（新近收录的字符与因此才有了映射的已有字符），与含孤立代理项的字符串；定义的校验、实例的执行、书签与待办（有状态，由 `WorkflowStoreConformance` 与引擎的行为套件核对）。

## 错误码

见 `conventions/error-codes.md` 中 `WORKFLOW_*`。
