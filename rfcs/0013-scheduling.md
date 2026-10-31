# RFC-0013 调度与领导选举

> 状态：已采纳 | 日期：2026-10-23，2026-10-26 修订（ADR-0020：作业存储、动态作业、多触发器、misfire、暂停、历史、集群分发） | 对应 ADR：ADR-0020 | 参考实现：`XiHan.Framework.Scheduling.Abstractions`、`XiHan.Framework.Scheduling.InMemory`、`XiHan.Framework.Scheduling.Database` | 一致性：`XiHan.Framework.Scheduling.Conformance`

## 摘要

调度子系统把"按计划运行的工作"做成与请求同构的调用：作业是作用域服务，每次运行在新作用域、新上下文里经完整内核管道执行。引擎（`JobSchedulerEngine`）与协议性的规则（计划推算、misfire、并发、恢复、历史）在抽象包里，Provider 只实现作业存储 `IJobStore` 与领导选举 `ILeaderElection`：内存实现是单进程，数据库实现是多实例集群——每次按计划的触发在整个集群只运行一次，由抢到触发器的节点运行；节点失联由领导者恢复。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `scheduling.jobs@1` | 作业与触发器的登记、持久化、按计划运行、手动触发、暂停与恢复、运行历史，`IJobScheduler` | `Scheduling.InMemory`（进程内存储，单进程轮询） |
| `scheduling.leader@1` | 租约式领导选举，`ILeaderElection` | `Scheduling.InMemory`（进程内租约；多实例换 `Scheduling.Database` / `.Kubernetes`） |

## 作业与触发器

Quartz 的"作业（做什么）"与"触发器（何时做）"分离：一个作业可以有多个触发器，每个触发器各自推算下一次运行。

- 名字 `JobName`：`^[a-z0-9][a-z0-9._-]{0,127}$`（与主题文法相同）；触发器名、领导选举的资源名同文法。触发器键 `TriggerKey(Job, Trigger)`。
- 作业定义 `JobDefinition`：`Name`、`JobType`（实现 `IJob` 的类型，必须已登记）、`Triggers`（可空：没有触发器的作业只能手动触发，持久保留）、`Data`（`JsonObject`，运行时随 `JobRun.Data` 传给作业）、`AllowConcurrent`（默认 `false`：整个集群同一作业同一时刻最多一次运行，按计划的触发被阻塞直至上一次结束、手动触发返回 `SCHEDULING_CONFLICT_001`）、`RunOnEveryInstance`（默认 `false`：按计划的触发在集群只运行一次；`true`：每个实例各自按本地时钟运行，不经存储抢占）、`RequestsRecovery`（默认 `false`：运行所在节点失联时由领导者重新入队一次，`JobTrigger.Recovery`）、`Retry`（重试策略，空为不重试，见下）、`Timeout`（单次执行时限，空为不限，见下）、`Description`。便利构造 `JobDefinition(name, schedule, jobType)` 生成一个名为 `default` 的触发器。
- 触发器定义 `JobTriggerDefinition`：`Name`、`Schedule`、`Data`（覆盖作业 `Data` 的同名键）、`Priority`（默认 5，到期的触发器多于可运行名额时大者先）、`MisfirePolicy`、`Description`。
- 作业类型登记：`services.AddJobType<TJob>()`（键 = 类型全名）或 `AddJobType<TJob>(key)`；`services.AddJob<TJob>(definition)` / `AddJob<TJob>(name, schedule)` 同时登记类型与作业（启动时写入存储，来源 `Code`）。同名重复 `AddJob` 是编程错误；运行时 `ScheduleAsync` 同名即替换。`JobTypeRegistry.Keys` 按序数列出已登记的类型键，管理界面据此列出运行时新建作业可选的类型（ADR-0053）。
- 计划 `JobSchedule`：`Manual`（只手动触发）、`Every(interval, repeatCount?)`（上一次完成后再等一个间隔，固定延迟；`RepeatCount` 为总运行次数，空为无限）、`FromCron(expression, timeZoneId?)`（cron，默认 UTC；时区按 `TimeZoneInfo` ID）、`Once(at)`（一次）；任何计划可带 `StartAt`（之前不运行）、`EndAt`（之后转为 `Complete`）与 `Calendar`（排除日历名）。`ToString()` / `Parse` 为文本往返（`manual`、`every 00:10:00 x5`、`cron 0 0 9 * * MON-FRI tz=Asia/Shanghai`、`once 2026-…`，可附 `start=`、`end=`、`calendar=`），存储以文本持久化。
- cron `CronExpression`：5 段（`m h dom mon dow`，秒 = 0）或 6 段（`s m h dom mon dow`），可选第 7 段年份；`*`、`?`、列表、范围、步长、月与星期名、`L`、`L-n`、`W`、`LW`、`nL`、`n#k`；星期 0/7 = 周日、1 = 周一（Unix 约定，不是 Quartz 的 1 = 周日）；日与星期同时限定取并集；`Next(after)` 按 UTC，`Next(after, timeZone)` 按该时区墙上时间，夏令时跳过的时刻不运行、重复的时刻只运行第一次；搜索上限 5 年。
- 日历 `IJobCalendar.IsExcluded(at)`：`services.AddJobCalendar(name, calendar)` 按名登记；内置 `HolidayCalendar`（日期集合）、`WeeklyCalendar`（星期集合）、`DailyCalendar`（每日时段）、`CronCalendar`（匹配 cron 的时刻）、`CompositeCalendar`（任一排除即排除）。推算下一次运行时跳过被排除的时刻（`Every` 的下一次也顺延到首个未排除时刻）。未登记的日历名 → `SCHEDULING_VALIDATION_001`。
- misfire：触发器的到期时刻落后当前时刻超过 `MisfireThreshold`（默认 60 秒；进程停机、暂停期间、节点失联都会造成）时按 `MisfirePolicy`：`RunOnce`（默认，立即运行一次，再从现在推算下一次）、`Skip`（不补，从现在推算下一次）、`RunAll`（按原计划逐次补齐，每个轮询周期补一次直至追平）。
- 重试（ADR-0060）：`JobRetryPolicy`：`MaxRetries`（1–100，失败后最多再运行几次）、`Delay`（第一次重试前的等待，大于零）、`Backoff`（`Fixed` 每次都等 `Delay`；`Exponential` 第 n 次重试等 `Delay × 2^(n-1)`）、`MaxDelay`（指数退避的上限，空为不设）；等待时间与上限都在 30 天以内，指数退避算出的等待超过 30 天按 30 天。
  - 只有失败的运行重试：作业返回失败、抛出异常或超过执行时限。被取消的运行（`CancelRunsAsync`、宿主停止）与失联的运行（`Lost`）不重试。
  - 重试是一次新运行：`Trigger` 为 `Retry`，`Attempt` 加一（首次为 1），`RetryOf` 是首次运行的 ID，`Data` 与失败的那次相同。
  - 集群唯一的作业把重试写成存储里的一次性触发器，到期时刻 = 失败时刻 + 等待时间，任一节点抢占运行，进程重启不丢；不允许并发的作业照常独占（上一次未结束时阻塞到它结束）。`RunOnEveryInstance` 的作业在失败的实例上进程内等待重试，实例停止时未到期的重试丢弃。
  - 手动 `TriggerAsync` 失败也按策略重试，调用方拿到的是第一次的结果。
- 执行时限（ADR-0060）：`Timeout`（大于零、不超过 30 天）到时取消作业拿到的令牌；作业返回之前时限已过，运行记为失败，错误码 `SCHEDULING_TIMEOUT_001`（作业不响应取消时，运行在它返回后才结束）。时限内取消请求或宿主停止仍记为取消。
- 重试策略与执行时限随作业持久化，代码作业指纹包含它们（设置了才计入，没设的作业指纹不变），运行时 `ScheduleAsync` 修改与 `RestoreAsync` 恢复同改计划的语义；`JobStatus.Job` 带出两项。
- 运行 `JobRun`：`Id`、`Name`、`TriggerName`（手动为空）、`ScheduledAt`、`Trigger`（`Schedule` / `Manual` / `Recovery` / `Retry`）、`Attempt`（第几次尝试，从 1 起）、`RetryOf`（重试时为首次运行的 ID）、`Data`（作业 `Data` ← 触发器 `Data` ← 手动触发的 `Data` 逐层覆盖）。`JobDispatcher` 为每次运行开新作用域，构造上下文（关联 ID = 运行 ID，运行信息可由 `JobDispatcher.GetRun(context)` 取得），经完整管道执行 `IJob.RunAsync`；返回失败或抛出异常都算失败（异常翻译为 `SCHEDULING_UNEXPECTED_001`）。不写幂等键。作业拿到的取消令牌在 `CancelRunsAsync` 与宿主停止时取消。

## 调度器 `IJobScheduler`

全部按批量、逐项结果：

- `ScheduleAsync(definitions[])`：新增或整体替换作业与触发器（来源 `Runtime`；替换代码登记的作业时来源记为 `Overridden`，见下；替换保留暂停状态与同名触发器的上一次运行时刻），立即生效，集群内其他节点在下一个轮询周期看到。作业类型未登记 → `SCHEDULING_NOTFOUND_002`；触发器名重复、日历未登记、计划不合法 → `SCHEDULING_VALIDATION_001`。
- `UnscheduleAsync(names[])`：删除作业与触发器（进行中的运行照常结束，历史保留）；未登记 → `SCHEDULING_NOTFOUND_001`。删除代码登记的作业只到下次启动：启动时按代码重新写入。
- `RestoreAsync(names[])`（ADR-0053）：把运行时改过的代码作业恢复为代码定义（来源回到 `Code`，保留暂停状态与同名触发器的进度）；作业没有代码定义 → `SCHEDULING_NOTFOUND_005`。
- `PauseAsync(names[])` / `ResumeAsync(names[])`：暂停或恢复作业的全部触发器（暂停期间手动触发仍可用）；恢复时按 misfire 策略处理暂停期间错过的运行。`PauseTriggersAsync(keys[])` / `ResumeTriggersAsync(keys[])` 作用于单个触发器；触发器不存在 → `SCHEDULING_NOTFOUND_003`。
- `TriggerAsync(requests[])`（`JobTriggerRequest(Name, Data?)`）：在本实例立即运行并等待结果，逐项 `JobTriggerResult`；不受暂停与领导限制；未登记 → `SCHEDULING_NOTFOUND_001`，正在运行且不允许并发 → `SCHEDULING_CONFLICT_001`。
- `EnqueueAsync(requests[])`：插入一个立即到期的一次性触发器（名 `manual-<id>`），由集群中任一节点运行，不等待；返回触发器名。
- `CancelRunsAsync(runIds[])`：请求取消运行——本实例的运行立即取消其令牌，其他节点的运行在下一个轮询周期取消；不在运行中 → `SCHEDULING_NOTFOUND_004`。取消的运行记为 `Canceled`，等待它的手动触发得到 `SCHEDULING_CONFLICT_003`。
- `StatusAsync(name)` / `StatusesAsync()`：`JobStatus`（作业记录、是否暂停、每个触发器的状态与下一次 / 上一次运行时刻与次数、运行中数量、最近一次完成的运行与结果）。
- `RunsAsync(query)`：运行历史（按作业、状态、时间范围、触发器名、执行节点、有无错误，分页，计划时刻降序）；`CountRunsAsync(query)` 给出同一组过滤项下的总数（不看分页），管理界面分页用（ADR-0053）。

调度器在应用进入服务状态之前不按计划运行；启动后首个周期把代码登记的作业写入存储：

- `ScheduleAsync`、`RestoreAsync`、`UnscheduleAsync` 与 `TriggerAsync`、暂停 / 恢复一样，先把代码作业写进存储：宿主刚启动、首个轮询周期之前的修改也按下面的规则判定来源（ADR-0060）。
- 每个代码作业记下代码定义的指纹 `CodeFingerprint`（类型键、数据、标志、重试策略与执行时限（设置了才计入）与各触发器的 SHA-256）。
- 存储里的作业来源是 `Overridden`（运行时 `ScheduleAsync` 改过的代码作业），且记下的指纹与当前代码定义相同时，保留运行时的修改。
- 其余情况（新作业、来源是 `Code`、代码定义变了）按代码写入，来源 `Code`；覆盖掉运行时修改时记一条警告。
- 暂停状态一律保留。

停止时等待进行中的运行结束（按宿主停止令牌取消）。

## 存储 `IJobStore`

Provider 实现，所有方法返回 `Result`、批量优先、存储不可达 → `SCHEDULING_UNAVAILABLE_003`，容量已满（内存实现的历史）→ `SCHEDULING_UNAVAILABLE_004`：

- `IsShared`：存储是否跨实例共享（抢占即集群唯一）。`false`（内存实现）时不共享的"集群只运行一次"作业只在持有领导租约的实例上按计划运行；`true`（数据库实现）时由抢占保证。
- 作业：`FindJobAsync`、`ListJobsAsync`、`SaveJobsAsync`（新增或替换定义与暂停状态；已有行的 `ActiveRunId` 由存储保留，只有 `TryBeginRunAsync` / `CompleteRunAsync` / `RecoverNodesAsync` 改它）、`DeleteJobsAsync`（级联触发器）。`JobRecord`：定义的持久化形态（`JobTypeName` 为类型键）加 `Paused`、`ActiveRunId`（不允许并发时正在运行的运行 ID）、`Source`（`Code` / `Runtime` / `Overridden`）、`CodeFingerprint`（代码作业最近一次按代码写入时的指纹，运行时作业为空）、`UpdatedAt`。
- 触发器：`TriggersAsync(job?)`、`SaveTriggersAsync`、`DeleteTriggersAsync`、`AcquireDueAsync(dueBefore, limit, nodeId, now)`（原子地把 `Waiting` 且到期的非本地触发器改成 `Acquired(nodeId)`，按到期时刻升序、优先级降序取至多 `limit` 条；多个节点同时调用时每条触发器只被一个节点拿到）。`TriggerRecord`：定义加 `State`（`Waiting` / `Paused` / `Acquired` / `Blocked` / `Complete` / `Error`）、`NextFireAt`、`PreviousFireAt`、`AcquiredBy`、`AcquiredAt`、`FireCount`、`Error`、`Local`。
- 运行：`TryBeginRunAsync(run, exclusive)`（写入运行记录；`exclusive` 时原子地占用作业的 `ActiveRunId`，已被占用返回 `false`）、`CompleteRunAsync(run)`（写回结果，释放 `ActiveRunId`，把该作业 `Blocked` 的触发器改回 `Waiting`）、`RunsAsync(query)`、`CountRunsAsync(query)`、`RunningAsync(job?)`、`LastRunsAsync(jobs[])`、`RequestCancelAsync(runIds[])`、`CancelRequestedRunsAsync(nodeId)`、`PurgeRunsAsync(before)`。
- 节点：`HeartbeatAsync(node)`、`NodesAsync()`、`RecoverNodesAsync(nodeIds[], now)`（失联节点 `Acquired` / `Blocked` 的触发器改回 `Waiting`，`Running` 的运行记为 `Lost`，释放其 `ActiveRunId`，删除节点记录，返回标记为 `Lost` 的运行）。

## 引擎

`JobSchedulerEngine`（抽象包，Provider 装配并启停）每个 `PollInterval`（默认 1 秒）执行一个周期：

1. 心跳（`IsShared` 时）；尝试获取或续约领导租约（`LeaseResource`，默认 `scheduling.jobs`）。
2. 领导者每 `HousekeepingInterval`（默认 60 秒）做一次维护：心跳超过 `NodeTimeout`（默认 90 秒）的节点视为失联并恢复，`RequestsRecovery` 的失联运行以 `Recovery` 触发重新入队；删除早于 `HistoryRetention`（默认 7 天，0 = 不删）的历史。
3. 取消：本节点被请求取消的运行取消其令牌。
4. 抢占：可运行名额 = `MaxConcurrentRuns`（默认 32）− 本节点运行中数量；`AcquireDueAsync(now, 名额, nodeId)`。对每条：作业暂停或类型未登记 → 触发器改为 `Paused` / `Error`；misfire 按策略；`TryBeginRunAsync` 失败（不允许并发且上一次未结束）→ 触发器 `Blocked`，上一次结束时自动回到 `Waiting` 并按 misfire 策略处理；否则推算下一次（cron / 一次性从计划时刻推算并立刻回到 `Waiting`，间隔计划完成后再推算，次数用尽或超过 `EndAt` → `Complete`）并在后台运行。运行结束时先推算间隔触发器的下一次、再把运行标为完成，状态里不会出现「已不在运行、也没有下一次」的空档。同一节点上对触发器的读改写（抢占后运行、结束时推算、暂停与恢复、登记与删除）互斥，抢占到的触发器在互斥内按存储里的当前状态重读，暂停等写入不会被随后的写回覆盖。
5. 本地触发器（`RunOnEveryInstance`）由每个实例按自己的时钟推算与运行，不经抢占；运行同样写入历史。

事件：装配了 `messaging.queue` 时经 `IMessagePublisher` 发布 CloudEvents 信封（`Source` = `/scheduling`，`Data` = `SchedulingEventData` JSON）：`scheduling.job.scheduled|unscheduled|paused|resumed`、`scheduling.run.started|succeeded|failed|canceled|lost`；发布失败只记日志。

## 领导选举

- `ILeaderElection.TryAcquire(resource, ttl)`：空闲（或租约已过期）时授予 `LeaderLease`；被持有返回 `SCHEDULING_CONFLICT_002`。
- `LeaderLease.Renew()` 延长 `ttl`；`Release()` 立即放弃；过期后他人可获取，原持有者续约失败。不可重入。持有者标识是实例身份（进程内实现每次获取生成新值，Kubernetes 实现是 Pod 名，数据库实现是节点 ID）。
- Provider：`Scheduling.InMemory`（进程内）、`Scheduling.Kubernetes`（`coordination.k8s.io` Lease，直接调用 API Server；存活时间按秒向上取整写入，API Server 不可达 → `SCHEDULING_UNAVAILABLE_002`）、`Scheduling.Database`（租约行，并发戳 CAS 接管）。

## Provider

- `Scheduling.InMemory`（`role: default`）：`InMemoryJobStore`（进程内，历史上限 `XiHan:Scheduling:InMemory:MaxRuns` 默认 1000，超出丢弃最旧）、进程内领导选举、装配引擎。单进程；多实例部署请换数据库实现，或保留内存存储并换 `Scheduling.Kubernetes` 的领导选举（此时集群唯一的作业只在领导者上运行）。
- `Scheduling.Database`（`role: provider`，依赖 `persistence.relational@1`）：作业、触发器、运行、节点、租约五张表经仓储读写，每个存储操作在自己的管道调用（Transaction 阶段）里提交，抢占与租约接管依赖并发戳 CAS；节点 ID `XiHan:Scheduling:NodeId`（默认机器名 + 进程 ID + 随机）。多实例部署下作业在节点间分发：每条到期触发器由先抢到的节点运行，不允许并发的作业全集群串行。

引擎选项（`XiHan:Scheduling`）：`PollIntervalSeconds`（1）、`NodeId`、`MaxConcurrentRuns`（32）、`MisfireThresholdSeconds`（60）、`LeaseResource`（`scheduling.jobs`）、`LeaseTimeToLiveSeconds`（30）、`NodeTimeoutSeconds`（90）、`HousekeepingIntervalSeconds`（60）、`HistoryRetentionDays`（7）。

## 一致性

Provider 必须继承 `JobSchedulerConformance`（触发与状态、间隔与 cron 计划、并发、失败、重试与执行时限、动态登记与删除、暂停与恢复、misfire、多触发器与优先级、历史、取消、本地作业）、`JobStoreConformance`（存储的查询、抢占原子性、独占运行、恢复）与 `LeaderElectionConformance` 并通过全部用例。调度器用例用真实时间等待（条件成立即止）；选举的过期用例按套件传入的时钟推进，与机器快慢无关。

## 错误码

见 `conventions/error-codes.md` 中 `SCHEDULING_*`。

## 与 4.x 的对应

| 4.x | 5.0 |
| --- | --- |
| `IJobWorker` + `XiHanJobBuilder.AddJob<T>()` | `IJob` + `services.AddJob<TJob>(name, schedule)` |
| `DynamicJobWorker`（每租户的运行时作业） | `IJobScheduler.ScheduleAsync`（运行时作业，持久化在存储里） |
| Quartz `JobDataMap` / 多触发器 / misfire / 暂停 / 集群 | `JobDefinition.Data`、`Triggers`、`MisfirePolicy`、`PauseAsync`、`Scheduling.Database` |
| `LeaderOnly` | `RunOnEveryInstance`（取反） |
| 任务的 `maxRetryCount` / `timeoutSeconds` | `JobDefinition.Retry`（另有固定 / 指数退避与上限）/ `JobDefinition.Timeout` |
