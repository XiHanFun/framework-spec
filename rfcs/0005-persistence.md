# RFC-0005 领域协议与持久化

> 状态：已采纳（2026-10-23 修订：批量方法；2026-10-27 修订：SqlSugar 多连接落点与分表、查询时间范围提示、领域事件派发；2026-10-27 修订：雪花标识生成器；2026-10-28 修订：可空值类型判空语义；2026-10-28 修订：字符串列长约定；2026-10-28 修订：DateTimeOffset 读取按 Kind 还原 UTC；2026-10-28 修订：InitTables 加列缺省值；2026-11-01 修订：协议字段只由 Provider 写入、并发新增同一标识、内存实现读写复制与提交时校验、雪花标识的启停顺序与调用里的不可用结果；2026-11-01 修订：判空与取反嵌套在与 / 或条件里的翻译；2026-11-01 修订：只读操作开只读工作单元、嵌套调用、查询取出条数超限是校验错误） | 日期：2026-10-21 | 对应 ADR：0005、0013、0022、0023、0025、0033、0035、0036、0037、0040、0063、0073 | 参考实现：`XiHan.Framework.Domain`、`XiHan.Framework.Persistence.Abstractions`、`XiHan.Framework.Persistence.InMemory`、`XiHan.Framework.Persistence.EntityFrameworkCore`、`XiHan.Framework.Persistence.SqlSugar` | 一致性：`XiHan.Framework.Persistence.Conformance`

## 摘要

领域层只定义协议，不提供基类；持久化层定义仓储、工作单元与查询的语义，Provider 按实体实现的协议自动维护审计、租户、并发与软删除字段。换 Provider 只改配置与包引用，业务代码不变。

## 领域协议

| 协议 | 字段 | Provider 行为 |
| --- | --- | --- |
| `IEntity<TKey>` | `Id` | 标识；`TKey` 必须可比较相等 |
| `IAggregateRoot<TKey>` | `DomainEvents`、`ClearDomainEvents()` | 仓储把本工作单元内新增 / 更新 / 删除过的聚合登记到 `IUnitOfWork.DomainEvents`（按首次写入顺序、同一实例一次）；Transaction 阶段在处理器成功后、提交前把它们尚未派发的事件交给 `IDomainEventDispatcher`（由 Messaging 子系统注册，见 RFC-0009），派发成功即清空缓冲，派发失败回滚并以失败结果结束调用；有事件却没有注册派发器 → `PERSISTENCE_VALIDATION_003`（不静默丢弃） |
| `IHasCreationTime` | `CreatedAt` | 新增时写入当前 UTC 时间；之后不再改变，见下文「协议字段只由 Provider 写入」 |
| `IHasModificationTime` | `ModifiedAt` | 更新时写入当前 UTC 时间 |
| `IHasConcurrencyStamp` | `ConcurrencyStamp` | 新增与更新时刷新；更新时比对，不符返回 `PERSISTENCE_CONFLICT_001` |
| `ISoftDeletable` | `IsDeleted`、`DeletedAt` | 删除改为在存储的当前行上打标；更新沿用存储的删除标记；查询默认过滤 |
| `IMultiTenant` | `TenantId` | 新增时写入当前租户，之后不再改变（更新与删除不能把实体改到别的租户）；查询只返回当前租户的实体；无当前租户时新增返回 `PERSISTENCE_VALIDATION_001` |
| `IValueObject` | — | 标记；用 `record` 取得值相等 |
| `ISpecification<T>` | `Criteria`、`IsSatisfiedBy` | 同一条件既能内存判定也能交给查询提供器；`And` / `Or` / `Not` 组合必须共享同一参数；可空值类型（`DateTimeOffset?`、`int?` 等）与 null 的 `==` / `!=` 按存在性翻译（`IS NULL` / `IS NOT NULL`，与 `HasValue` 等价），Provider 不得译成 `= NULL`；判空与取反出现在 `&&` / `||` 的任意嵌套位置（如 `A && B && (x == null || …)`、`A && (!(…) || …)`）都照常翻译，Provider 不得漏写运算符 |

领域事件集合有界（默认 256），超出抛出 `InvalidOperationException`：那是忘记提交的编程错误。

### 协议字段只由 Provider 写入

创建时间、租户只在新增时写入，删除标记只经删除写入，修改时间与并发戳由每次更新写入。更新时创建时间、租户与删除标记沿用存储里当前行的值，删除只写删除标记与删除时间；调用方实例上的这些值——自己构造的游离实例，或读出后改过的实例——一律不生效，更新不会把行改到别的租户或改掉创建时间，也不会借更新打上或去掉删除标记。写入成功后调用方实例上是存储里的协议字段值。`PersistenceConventions.ApplyOnUpdate(entity, stored, now)` 是 Provider 共用的实现：把存储行的创建时间、租户与删除标记写回调用方实例，再写修改时间、刷新并发戳。

## 工作单元

- 作用域生命周期，一次调用一个；由 Transaction 阶段的 `UnitOfWorkMiddleware` 开启，用安全上下文中的租户作为当前租户。调用带操作描述且契约只读（`OperationContract.IsReadOnly`：动词 `get` / `list`，或声明了 `ReadOnly = true`，RFC-0011）时开只读工作单元；其余调用——写操作，以及不带操作描述的调用（消息处理器、作业、直接交给 `App.InvokeAsync` 的处理器）——开可写工作单元。
- 状态机：`NotStarted → Active → Committed | RolledBack`。违反顺序抛出携带 `PERSISTENCE_VALIDATION_002` 的 `FrameworkException`。
- 调用结果为成功则提交；结果为失败或抛出异常则回滚。提交失败（并发冲突、标识重复、容量）作为调用结果返回。
- 并发新增同一标识：两个工作单元先后新增同一标识，检查时对方尚未提交——先提交者的行保留，后到者以 `PERSISTENCE_CONFLICT_002` 失败，不静默覆盖。关系型 Provider 在写入时由主键约束发现（写入等对方事务结束才报错），数据库报错后 Provider 核对该标识的行确已存在才按标识重复报告，否则异常原样抛出；此后该工作单元只能回滚，处理器忽略这次失败而返回成功时，提交回滚并返回同一错误。内存实现在提交时发现，提交返回该错误。
- 读己之写：同一工作单元内新增、更新、删除对随后的查找与查询可见；提交前对其他工作单元不可见；回滚后存储里不留任何痕迹，包括 Provider 写入的修改时间、并发戳与删除标记。
- 领域事件在提交前派发（`UnitOfWorkMiddleware`：处理器成功 → 派发 → 提交）：装配了发件箱时事件随业务写入同一事务落进发件箱表，提交后由投递循环发出；没有发件箱时直接发布到代理，发布失败回滚业务写入（至多一次、先于提交）。派发只在 Transaction 阶段的中间件里发生，直接使用工作单元的调用方自己负责。

### 只读工作单元

- 只读工作单元不开数据库事务；查找、按标识集合查找、查询与计数照常。
- 仓储的写入在写入时拒绝、不触后端：`AddAsync` / `UpdateAsync` / `RemoveAsync` 返回 `PERSISTENCE_DENIED_001`，`AddRangeAsync` / `UpdateRangeAsync` / `RemoveRangeAsync` 逐项都是它（空集合不算写入，仍返回空列表）。拒绝同时把工作单元记为失败：处理器忽略这次失败而返回成功时，提交回滚并返回同一错误，调用以它结束——误写不会被静默忽略，也不会变成未处理异常。
- Provider 暴露给直接写入的入口——EF Core 与 SqlSugar 工作单元的 `EnsureTransactionAsync`（直接用 `DbContext` 或 SqlSugar 客户端写入之前开启事务）——在只读工作单元里抛出携带 `PERSISTENCE_DENIED_001` 的 `FrameworkException`，写入不会执行；Transaction 阶段的中间件把它转为同码的结果并回滚，不经 Observe 阶段包成 `KERNEL_UNEXPECTED_001`。EF Core 提交时另查变更跟踪器：绕过仓储改了跟踪实体而未保存的，同样以 `PERSISTENCE_DENIED_001` 回滚。不经这两处、直接对客户端执行的写入在 Provider 视野之外，不受守卫，也不在事务里。
- `PERSISTENCE_DENIED_001` 是操作实现的缺陷（只读操作写了数据），不是调用方能修正的输入，也不是可重试的不可用：HTTP 403、gRPC `PermissionDenied`、命令行按 DENIED 的退出码；熔断不计数（RFC-0004 只计 UNAVAILABLE / TIMEOUT / UNEXPECTED）。
- 管道外自己开工作单元的代码由 `UnitOfWorkOptions(TenantId, IsReadOnly)` 决定，`UnitOfWorkOptions.Default` 是无租户、可写；框架的单例存储（租户目录与授予的读取、审计查询、租约探测）读取时显式开只读工作单元，写入（发件箱、审计落库、调度与工作流存储）开可写工作单元，不受操作契约影响。

### 嵌套调用

- 一次调用一个工作单元，各调用按自己的契约开自己的。在调用里经管道发起另一次调用（`IOperationInvoker.InvokeAsync`、`App.InvokeAsync`）要新建服务作用域：`CreateAsyncScope()` 后用 `App.CreateContext(scope.ServiceProvider, …)` 建上下文（AI 工具、MCP 的每次 `tools/call` 都这样做），内层得到自己的工作单元。
- 只读不向内、也不向外传播：只读操作里调用写操作，写操作在自己的可写工作单元里写入、成功即自行提交，外层随后失败也不回滚它（不在外层的事务里），外层自己的仓储仍然只读；写操作里调用只读操作，内层只读。要与外层同进退的写入不应拆成嵌套调用。
- 同一服务作用域里再经管道（用外层的上下文、由它派生的上下文，或以 `context.Services` 新建的上下文）时，作用域里的工作单元已经开始过：中间件不再开始它、不执行内层的处理器，以 `PERSISTENCE_VALIDATION_002` 的结果结束内层调用，外层工作单元的状态不变。内层不加入外层的工作单元：加入会让内层写入的成败取决于外层是否只读，内层失败时已写入的部分又会随外层一起提交。

## 仓储

| 操作 | 语义 | 可恢复失败 |
| --- | --- | --- |
| `FindAsync(id)` | 已软删除或不属于当前租户视为不存在 | — |
| `ListAsync(query)` | 过滤、排序、分页；`Take` 取 1 到 `EntityQuery.DefaultMaxTake`（1000），缺省取上限；超过上限以 `PERSISTENCE_VALIDATION_005` 拒绝（见下文「查询的取出条数」），不静默截断，要更多行就分页；Provider 不得返回超过 `Take` 的结果；`TimeRange`（`Within(from, to)`）是时间范围提示——按时间分表 / 分区的 Provider 据此只扫描范围内的表，不是过滤条件，不分表的实体与不支持范围的 Provider 忽略它 | — |
| `CountAsync(query)` | 忽略排序与分页 | — |
| `AddAsync` | 写入创建时间、租户、并发戳 | `PERSISTENCE_CONFLICT_002` 标识重复、`PERSISTENCE_VALIDATION_001` 缺租户 |
| `UpdateAsync` | 写入修改时间、刷新并发戳 | `PERSISTENCE_NOTFOUND_001`、`PERSISTENCE_CONFLICT_001` |
| `RemoveAsync` | 软删除协议打标，否则物理删除 | `PERSISTENCE_NOTFOUND_001` |
| `FindManyAsync(ids)` | 按标识集合查找；只返回存在且可见的实体，顺序按入参，缺失省略，重复标识只返回一次 | — |
| `AddRangeAsync(entities)` | 逐项同 `AddAsync`，一次写入 | 逐项结果 |
| `UpdateRangeAsync(entities)` | 逐项同 `UpdateAsync`，一次写入 | 逐项结果 |
| `RemoveRangeAsync(entities)` | 逐项同 `RemoveAsync`，一次写入 | 逐项结果 |

### 批量（`conventions/batch-first.md`）

- 单条是批量的特例；框架内部与示例处理多个实体时调用批量方法。
- 逐项结果与入参顺序一一对应：标识重复、缺租户、不存在、并发冲突按项报告；通过校验的项一次写入（Provider 用原生多行写入），写入阶段的基础设施失败（`PERSISTENCE_UNAVAILABLE_003`、容量）对这些项统一报告，整个工作单元随调用结果回滚。
- 批内按入参顺序逐项施加语义：同一批新增同一标识两次，第二次返回 `PERSISTENCE_CONFLICT_002`；删除同一实体两次，第二次返回 `PERSISTENCE_NOTFOUND_001`。
- 写入阶段才发现的标识重复（检查之后另一个工作单元提交了同一标识，见「工作单元」）按写入阶段的失败处理：这次写入的项统一返回 `PERSISTENCE_CONFLICT_002`，`id` 是按入参顺序第一个撞上的标识，工作单元随后只能回滚。
- 空集合不触后端、返回空列表，但仍要求活动的工作单元。
- 读己之写对批量同样成立：批量新增后同一工作单元内可查到全部实体。

### 查询的取出条数

- 一次查询最多取 `EntityQuery.DefaultMaxTake`（1000）行。上限是存储一侧的容量约束，调用方事先不知道，传了更大的页大小是能改正的输入：构造查询时（`Page(skip, take)`、`with { Take = … }`）抛出携带 `PERSISTENCE_VALIDATION_005` 的 `FrameworkException`；Transaction 阶段的 `UnitOfWorkMiddleware` 回滚并把它转为同码的 VALIDATION 结果（HTTP 400、gRPC `InvalidArgument`，消息键 `persistence.query.take_exceeded`，参数 `take` 为请求的条数、`max` 为上限），不经 Observe 阶段包成 `KERNEL_UNEXPECTED_001`。
- 转换只在 Transaction 阶段里：查询在它之外构造（管道外的后台代码、Transaction 之前的阶段）时仍以异常抛出——那里的取出条数由代码给出，超限是编程错误。框架自己的分页都不超过上限：发件箱 `BatchSize` 在选项里限 1–1000，审计查询的页大小先经 `AUDITING_VALIDATION_001` 校验，目录加载、授予、调度与工作流存储的分页取常量上限、`Math.Min(上限, 剩余)` 或查询类型里已限 1–1000 的 `Limit`；`TenantRecordStore`、`MessageTranslationStore`、`PermissionGrantRecordStore` 的列表方法把调用方的 `skip` / `take` 原样传入，超限按上一条成为结果。
- `Skip` 为负、`Take` 小于 1 构不成查询，是编程错误，构造时抛出 `ArgumentOutOfRangeException`，不转为结果。
- 页大小来自调用方时，应用在请求契约上声明范围（`[Range(1, EntityQuery.DefaultMaxTake)]` 或更小、`skip` 的 `[Range(0, …)]`），Validate 阶段以 `KERNEL_VALIDATION_003` 拒绝并指出字段，进不到 Transaction 与 Handle；Transaction 阶段的转换保证没有声明时也以校验错误结束。

在 Transaction 阶段之外使用仓储是装配错误，每个操作抛出携带 `PERSISTENCE_UNAVAILABLE_001` 的 `FrameworkException`，不返回 `Result`。

## 实体登记

应用用 `AddRepository<TEntity, TKey>()` 登记实体；Provider 的特性在配置阶段通过 `IRepositoryRegistrar` 为每个登记注册闭合泛型的 `IRepository<TEntity, TKey>`。不用开放泛型注册，不用反射扫描；AOT 友好。

## 标识生成

`IIdGenerator<TKey>` 是标识生成端口。持久化 Provider 注册 `IIdGenerator<Guid>`（`GuidV7IdGenerator`：UUID 版本 7，时间有序）；应用显式装配 `SnowflakeIdFeature` 后得到 `IIdGenerator<long>`（`SnowflakeIdGenerator`），Profile 不缺省装配它。

### 雪花标识

| 位 | 含义 | 范围 |
| --- | --- | --- |
| 63 | 符号位，恒为 0 | — |
| 62–22（41 位） | 自 epoch 起的毫秒数 | 约 69.7 年 |
| 21–12（10 位） | 机器位 | 0–1023 |
| 11–0（12 位） | 同一毫秒内的序列 | 0–4095 |

- epoch 缺省 `2026-01-01T00:00:00Z`，可配；同一套数据的 epoch 一经写入不得再改，否则已有标识解析出的时间戳会错位。
- 机器位两种来源，不做随机：`XiHan:Persistence:Snowflake:WorkerId` 显式指定；未指定时由容器里的 `IDistributedLock`（`caching.distributed` 的 Provider 注册）租约分配——启动时从 0 起逐个尝试锁资源 `persistence:snowflake:worker:<n>`，取得第一个空位，存活时间 `LeaseTimeToLiveSeconds`（缺省 30），每三分之一存活时间续约一次（锁不续约，续约 = 释放再获取同一机器位；释放与重新获取之间空位被别的实例拿走，或存储不可达，都按丢失处理并重新分配）；本地按获取时刻 + 存活时间判定有效，过期未续上即没有机器位。容器里没有分布式锁、或 1024 个机器位都被占用时启动失败。
- 时钟回拨不超过 `ClockDriftToleranceMilliseconds`（缺省 100）时等待时钟追平；超过时抛出携带 `PERSISTENCE_UNAVAILABLE_004` 的 `FrameworkException`。同一毫秒序列用尽时等待下一毫秒。生成器线程安全，单实例上限 4096 个 / 毫秒。
- 没有机器位（租约尚未取得或已丢失）时抛出携带 `PERSISTENCE_UNAVAILABLE_005` 的 `FrameworkException`；租约状态以健康项 `persistence.snowflake.worker` 上报（持有 Ready，没有 Degraded）。
- 调用里抛出的 `PERSISTENCE_UNAVAILABLE_004` / `005` 由 `SnowflakeIdFeature` 登记在 Resilience 阶段末尾的中间件转为同码的 UNAVAILABLE 结果（HTTP 503、gRPC `Unavailable`，可重试；消息键 `persistence.snowflake.clock_moved_backwards` / `persistence.snowflake.worker_unavailable`），不经 Observe 阶段包成 `KERNEL_UNEXPECTED_001`。它在幂等与工作单元之外：异常经过内层时工作单元已回滚、幂等标记已清除，重试会重新执行；截止时间与熔断在外层照常计数。其他异常不受影响；管道之外直接调用生成器仍以异常失败。
- `SnowflakeIdFeature` 的清单声明可选依赖 `caching.distributed@1`：装配了提供它的 Provider 时，能力图谱把雪花特性排在其后——锁存储启动之后才取租约、停止时先释放租约再停锁存储，与 `UseFeature` 的先后无关。固定机器位且没有任何 `caching.distributed` Provider 时，按 RFC-0002 可选依赖缺失，特性进入 `Degraded`，生成不受影响。
- `SnowflakeId.Parse(value, epoch)`（或生成器的 `Parse(value)`）还原时间戳、机器位与序列。
- 线格式保持 JSON number（框架标准类型），不转字符串；超过 2^53 的值在 JavaScript 的 `number` 里会丢精度，前端类型化客户端应按字符串处理。

## 能力与配置

| 项 | 值 |
| --- | --- |
| 能力 | `persistence.relational@1`；`persistence.document@1` 预留 |
| 配置节 | `XiHan:Persistence`；内存实现 `XiHan:Persistence:InMemory:MaxEntitiesPerType`（默认 100000）；雪花标识 `XiHan:Persistence:Snowflake`：`Epoch`（ISO 8601，默认 `2026-01-01T00:00:00Z`）、`WorkerId`（0–1023，缺省租约分配）、`ClockDriftToleranceMilliseconds`（默认 100）、`LeaseTimeToLiveSeconds`（默认 30） |
| 默认实现 | `XiHan.Framework.Persistence.InMemory`，`role: default`，`stability: preview`，只用于开发与测试 |

## Provider

| Provider | 角色 | 要点 |
| --- | --- | --- |
| `Persistence.InMemory` | default | 每类实体一张有界表；覆盖层实现读己之写；读写都复制（字段逐一的浅复制）：查找与查询交出副本，写入存调用方实例的副本，读出的实例改了不经仓储写回就碰不到存储，集合等引用类型字段（含领域事件缓冲）仍与副本共享、不在隔离范围内；覆盖层记下每个写过的标识首次读到的已提交状态，提交时按涉及的表依次加锁、逐一校验（新增的标识已被提交 → `PERSISTENCE_CONFLICT_002`；更新或删除的行已不在、带并发戳的行戳已变 → `PERSISTENCE_CONFLICT_001`；容量 → `PERSISTENCE_UNAVAILABLE_002`），全部通过才应用，跨表原子 |
| `Persistence.EntityFrameworkCore` | provider | 应用持有 `DbContext`；工作单元在首次写入时开启数据库事务，每次写入立即 `SaveChanges`，批量写入是一次 `AddRange` / 多实体跟踪加一次 `SaveChanges`；软删除与租户过滤用 `EF.Property` 显式施加，不用全局过滤器；`ApplyPersistenceConventions()` 把并发戳设为并发令牌；游离实体更新先比对并发戳；更新与删除时创建时间、租户与删除标记回到跟踪实例的原值（OriginalValues），写入路径按原值判定可见性，提交时保存的残余变更同样不改创建时间与租户；新增撞上主键约束时，SaveChanges 已回到自动建的保存点，在同一事务里核对标识，确已存在即按标识重复报告并把这次新增的实体移出跟踪 |
| `Persistence.SqlSugar` | provider | 作用域内的 `ISqlSugarClient` 由特性按回调创建或由应用注册，可以带多个连接（`ConfigId`）：特性创建时默认连接的 `ConfigId` 是 `default`，带租户分片的调用再加一个 `shard` 连接；每个实体落在哪个连接由 `IEntityConnectionSelector` 决定（默认规则：SqlSugar `[Tenant(configId)]` 特性指定的连接 → 实现 `IMultiTenant` 且客户端有 `shard` 连接时 `shard` → 否则默认连接），平台目录表始终走默认连接、租户表走分片连接；`[Tenant]` 指向客户端里没有的连接是装配错误，抛出携带 `PERSISTENCE_VALIDATION_004` 的 `FrameworkException`，不回退到默认连接，缺省落点规则下带 `[Tenant]` 的已登记实体在启动时就校验（`shard` 只在带分片的调用里存在，不在启动时校验）；工作单元经 `AsTenant()` 开启、提交、回滚事务，同一调用触及的全部连接一起提交或回滚（顺序提交，不是两阶段提交）；每次写入立即执行；批量用按对象入口 `InsertableByObject(list)` / `UpdateableByObject(list)` / `DeleteableByObject(list)`（删除按主键，开启差异日志的实体走泛型构建器），各一次调用；软删除与租户过滤以表达式显式施加；规约先改写再交给 SqlSugar——可空值类型与 null 的 `==` 改写成 `SqlFunc.EqualsNull(x, null)`、`!=` 改写成 `HasValue`，逻辑非下推到叶子（双重否定消去、德摩根展开、比较换反运算、`!x.HasValue` 换判空），因为 SqlSugar 把提升比较译成 `= NULL`，又会在与 / 或条件右侧的「取反作左操作数」处漏写运算符；`SqlSugarConventions.Apply()` 按协议配置主键、并发戳长度、`DateTimeOffset` 的 UTC 往返（写入取 UTC 时刻；读取按驱动交出的 `DateTime.Kind` 还原——Npgsql legacy 时间戳模式把 `timestamptz` 换算成本地时间、Kind=Local 交出，须换回 UTC，否则多出本地时区偏移且经实体回写逐次累加；Utc / Unspecified 按 UTC；一致性套件除 SQLite 外还要在真实 PostgreSQL 上跑，环境变量 `XIHAN_TEST_POSTGRES`）、可空列与忽略列，实体不继承 SqlSugar 基类；更新与删除先在事务内加行锁取回当前行比对并发戳，更新时创建时间、租户与删除标记换成当前行的值，软删除在当前行上打标；新增撞上主键约束时另开一个不在事务里的连接（`CopyNew`，同一份连接配置）核对标识是否已被提交（PostgreSQL 系语句失败后事务已中止，不能在原事务里再查），确已存在即按标识重复报告；带 SqlSugar `[SplitTable]` 的实体自动走分表入口——查询在过滤条件之后 `SplitTable(from, to)`（按 `TimeRange` 的 UTC 时刻，没有提示扫全部分表），写入经 `InsertableByObject(x).SplitTable()` 等按 `[SplitField]` 落表，分表实体不加行锁靠并发戳，启动建表走 `CodeFirst.SplitTables()`字符串列的长度：`[StringLength(n)]`（或 `[MaxLength(n)]`）→ 长度 n，`Persistence.Abstractions` 的 `[LongText]`（或无参 `[MaxLength]`）→ 数据库的长文本类型，DataAnnotations 的 `[MaxLength]` 构造函数带 `RequiresUnreferencedCode`、框架记录不用；没有注解的沿用 SqlSugar 默认（PostgreSQL 为 `varchar(255)`），框架自己的数据库记录全部显式标注（架构测试门禁）；`InitTables`（ADR-0040）建缺表、加缺列，删列、改名、改类型不在契约内，是开发与测试的便利，生产库结构演进走迁移脚本——给已有行的表加非可空列时，PostgreSQL 上非可空的数值 / 枚举 / 布尔 / Guid 列由约定配 CLR 缺省值（`0` / `false` / 全零 Guid）回填旧行，SQLite 由 SqlSugar 加成可空列（旧行读回 CLR 缺省值），非可空字符串与日期时间列及其他数据库不受支持（声明成可空，或重建 / 迁移）；约定只作用于实体，声明在 BCL 类型上的属性原样交还 |

## 一致性

Provider 必须继承 `RepositoryConformance` 并通过全部用例，含时间范围提示对不分表实体不过滤的用例与批量用例：批量新增读己之写、批内重复标识逐项报告、批量更新刷新并发戳且过期戳按项冲突、批量删除软删 / 物理删与不存在按项报告、`FindManyAsync` 的顺序与缺失、空集合；可空值类型与 null 的相等 / 不等按存在性过滤（字面量 null、闭包里的 null 变量、左右互换、与 `HasValue` 一致，组合到别的条件里也成立）；一次查询最多取 `EntityQuery.DefaultMaxTake` 条、超过的取出条数在构造查询时以 `PERSISTENCE_VALIDATION_005` 拒绝，调用里先写入再构造超限查询的调用以该码结束、写入回滚；判空与取反嵌套在与 / 或条件里照常过滤（或条件在与条件右侧、判空或取反是或条件的左操作数、取反整个与 / 或条件、双重否定）；更新沿用存储的创建时间、租户与删除标记、删除不改创建时间（游离实例与读出后改过的实例两种）；两个工作单元先后新增同一标识只有一个成功、另一个返回 `PERSISTENCE_CONFLICT_002`、留下的是成功者的行；回滚的调用在存储里不留修改、并发戳与删除标记，提交前另一个工作单元读到的是旧值；只读操作里单条与批量的新增、更新、删除都返回 `PERSISTENCE_DENIED_001`、存储不变，处理器忽略拒绝而返回成功时调用仍以它结束，查询照常；只读操作在新的服务作用域里调用写操作，写入照常提交，外层仍拒绝自己的写入；同一作用域里再经管道的调用以 `PERSISTENCE_VALIDATION_002` 结束、处理器不执行。套件以 `Testing.TestHost` 走完整管道，用自带的 Tenant 阶段中间件从 Baggage 的 `xihan.tenant` 解析租户，不依赖 MultiTenancy 子系统。

标识生成器必须通过 `IdGeneratorConformance<TKey>`：顺序生成与多线程并发生成不重复、时钟前进后生成的标识更大；`GuidV7IdGenerator` 与 `SnowflakeIdGenerator` 都通过。

## 错误码

见 `conventions/error-codes.md` 中 `PERSISTENCE_*`。

## 不做什么

- 不提供实体基类、审计基类或"全能仓储"。
- 不在抽象层暴露 `IQueryable`；复杂查询由 Provider 专属包（如 `Persistence.EntityFrameworkCore` 的 `DbContext`）承担。
- 不提供按查询条件的批量更新 / 删除（`UPDATE … WHERE`）：它绕过协议维护的审计、并发与软删除字段，由 Provider 专属包按需提供。
- 不提供跨 Provider 的分布式事务。
