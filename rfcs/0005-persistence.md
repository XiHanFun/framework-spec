# RFC-0005 领域协议与持久化

> 状态：已采纳（2026-10-23 修订：批量方法；2026-10-27 修订：SqlSugar 多连接落点与分表、查询时间范围提示、领域事件派发；2026-10-27 修订：雪花标识生成器；2026-10-28 修订：可空值类型判空语义；2026-10-28 修订：字符串列长约定；2026-10-28 修订：DateTimeOffset 读取按 Kind 还原 UTC；2026-10-28 修订：InitTables 加列缺省值；2026-11-01 修订：协议字段只由 Provider 写入、并发新增同一标识、内存实现读写复制与提交时校验、雪花标识的启停顺序与调用里的不可用结果；2026-11-01 修订：判空与取反嵌套在与 / 或条件里的翻译；2026-11-01 修订：只读操作开只读工作单元、嵌套调用、查询取出条数超限是校验错误；2026-11-01 修订：索引声明与唯一键重复；2026-11-01 修订：运行时数据源；2026-11-01 修订：库结构读取与实体目录；2026-11-01 修订：只读工作单元里的写入是 500 类缺陷（`PERSISTENCE_UNEXPECTED_001`）、软删除实体的唯一索引是只约束未删除行的部分唯一索引） | 日期：2026-10-21 | 对应 ADR：0005、0013、0022、0023、0025、0033、0035、0036、0037、0040、0063、0072、0073、0078、0081 | 参考实现：`XiHan.Framework.Domain`、`XiHan.Framework.Persistence.Abstractions`、`XiHan.Framework.Persistence.InMemory`、`XiHan.Framework.Persistence.EntityFrameworkCore`、`XiHan.Framework.Persistence.SqlSugar` | 一致性：`XiHan.Framework.Persistence.Conformance`

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
- 仓储的写入在写入时拒绝、不触后端：`AddAsync` / `UpdateAsync` / `RemoveAsync` 返回 `PERSISTENCE_UNEXPECTED_001`，`AddRangeAsync` / `UpdateRangeAsync` / `RemoveRangeAsync` 逐项都是它（空集合不算写入，仍返回空列表）。拒绝同时把工作单元记为失败：处理器忽略这次失败而返回成功时，提交回滚并返回同一错误，调用以它结束——误写不会被静默忽略，也不会变成未处理异常。
- Provider 暴露给直接写入的入口——EF Core 与 SqlSugar 工作单元的 `EnsureTransactionAsync`（直接用 `DbContext` 或 SqlSugar 客户端写入之前开启事务）——在只读工作单元里抛出携带 `PERSISTENCE_UNEXPECTED_001` 的 `FrameworkException`，写入不会执行；Transaction 阶段的中间件把它转为同码的结果并回滚，不经 Observe 阶段包成 `KERNEL_UNEXPECTED_001`——错误码直接指出是只读写入，文案按调用方区域本地化。EF Core 提交时另查变更跟踪器：绕过仓储改了跟踪实体而未保存的，同样以 `PERSISTENCE_UNEXPECTED_001` 回滚。不经这两处、直接对客户端执行的写入在 Provider 视野之外，不受守卫，也不在事务里。
- `PERSISTENCE_UNEXPECTED_001` 是操作实现的缺陷（只读操作写了数据），不是调用方能修正的输入，也不是权限不足或可重试的不可用：HTTP 500、gRPC `Internal`、命令行按 UNEXPECTED 的退出码，与其他服务端缺陷一起进 5xx 告警；熔断计数（RFC-0004 计 UNAVAILABLE / TIMEOUT / UNEXPECTED，按操作 ID，只影响这个有缺陷的操作）。同一服务作用域里再经管道的嵌套调用另以 `PERSISTENCE_VALIDATION_002` 拒绝（见「嵌套调用」），不是这个码。
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
| `AddAsync` | 写入创建时间、租户、并发戳 | `PERSISTENCE_CONFLICT_002` 标识重复、`PERSISTENCE_CONFLICT_003` 唯一键重复、`PERSISTENCE_VALIDATION_001` 缺租户 |
| `UpdateAsync` | 写入修改时间、刷新并发戳 | `PERSISTENCE_NOTFOUND_001`、`PERSISTENCE_CONFLICT_001`、`PERSISTENCE_CONFLICT_003` |
| `RemoveAsync` | 软删除协议打标，否则物理删除 | `PERSISTENCE_NOTFOUND_001`、`PERSISTENCE_CONFLICT_003`（只在唯一索引的列里显式写了删除标记时：软删除撞上同一业务键已删除的行，见「索引声明」） |
| `FindManyAsync(ids)` | 按标识集合查找；只返回存在且可见的实体，顺序按入参，缺失省略，重复标识只返回一次 | — |
| `AddRangeAsync(entities)` | 逐项同 `AddAsync`，一次写入 | 逐项结果 |
| `UpdateRangeAsync(entities)` | 逐项同 `UpdateAsync`，一次写入 | 逐项结果 |
| `RemoveRangeAsync(entities)` | 逐项同 `RemoveAsync`，一次写入 | 逐项结果 |

### 批量（`conventions/batch-first.md`）

- 单条是批量的特例；框架内部与示例处理多个实体时调用批量方法。
- 逐项结果与入参顺序一一对应：标识重复、缺租户、不存在、并发冲突按项报告；通过校验的项一次写入（Provider 用原生多行写入），写入阶段的基础设施失败（`PERSISTENCE_UNAVAILABLE_003`、容量）对这些项统一报告，整个工作单元随调用结果回滚。
- 批内按入参顺序逐项施加语义：同一批新增同一标识两次，第二次返回 `PERSISTENCE_CONFLICT_002`；删除同一实体两次，第二次返回 `PERSISTENCE_NOTFOUND_001`。
- 写入阶段才发现的标识重复（检查之后另一个工作单元提交了同一标识，见「工作单元」）按写入阶段的失败处理：这次写入的项统一返回 `PERSISTENCE_CONFLICT_002`，`id` 是按入参顺序第一个撞上的标识，工作单元随后只能回滚。撞上唯一索引同样：这次写入的项统一返回 `PERSISTENCE_CONFLICT_003`，`index` 是第一个撞上的索引名（见「索引声明」）。
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

## 索引声明

实体在类上以 `Persistence.Abstractions` 的 `[EntityIndex(params string[] properties)]` 声明索引，Provider 据此建索引；实体只引用契约包，不标某个 Provider 的特性（SqlSugar 的 `[SugarIndex]`、EF Core 的 `[Index]`）。`EntityIndexes.Resolve(entityType, table)` 是共用实现，Provider 不自行推名字，两个关系型 Provider 建出同名、同列的索引。

| 项 | 规则 |
| --- | --- |
| 列 | 属性名（`nameof`），按声明顺序，全部升序；同一实体可声明多个，`IsUnique = true` 为唯一索引 |
| 名字 | `IX_`（唯一为 `UX_`）+ 表名 + 每列一段缩写，以 `_` 连接。缩写是属性名每个词的前两个字符：词从首字符、小写或数字之后的大写字母、连续大写里后跟小写的那个大写字母开始，`_` 只分词不计入（`TenantId → TeId`、`NextFireAt → NeFiAt`、`HTTPServerName → HTSeNa`）。表名是 Provider 映射出的表名，字母、数字、`_` 以外的字符换成 `_` |
| 长度 | 不超过 63 个字符（`EntityIndexes.MaxNameLength`，PostgreSQL 标识符上限；MySQL 64、SQL Server 128）；超过是声明错误，不截断——PostgreSQL 会静默截断，下一次建表按全名查不到已有索引，再建即撞名 |
| 软删除 | 实体实现 `ISoftDeletable` 时，唯一索引是只约束未删除行的部分唯一索引（`EntityIndex.ExcludesDeleted`）：列就是声明的列，条件 `IsDeleted = false`（PostgreSQL、SQLite 的 `WHERE`，SQL Server 的筛选索引），名字仍按声明的列；未删除的行之间唯一，已删除的行不受约束——同一业务键删除、重建、再删除任意多次都不冲突。声明的列里显式写了 `IsDeleted` 时不加条件，按普通唯一索引约束这些列（同一业务键最多一行已删除），是不支持部分索引的数据库上的显式选择 |
| 部分索引的数据库 | 支持：PostgreSQL、SQLite、SQL Server。其余不支持或未验证的数据库（MySQL 系、Oracle 系、达梦、ClickHouse 等，以及源自 PostgreSQL 但未验证的人大金仓、openGauss、GaussDB、瀚高、Vastbase、PolarDB 等）上，有软删除实体声明了唯一索引（未显式含 `IsDeleted`）即启动失败（`FrameworkException`，`PERSISTENCE_UNAVAILABLE_008`，列出实体、索引与数据库类型），不静默退化成普通唯一索引或不建索引 |
| 空值 | 唯一索引含可空列时，NULL 的语义随数据库：PostgreSQL、SQLite、MySQL 不约束含 NULL 的行，SQL Server 把 NULL 当作相等，EF Core 在 SQL Server 上给这类唯一索引自动加 `IS NOT NULL` 过滤。需要空值参与唯一性的列存非空哨兵值 |
| 声明错误 | 没有列、列名空白或重复、列不是公开实例属性、名字超长、同一实体两个索引同名（不区分大小写）或同列：Provider 建模或建表时抛 `InvalidOperationException` |

- SqlSugar：约定在表服务回调（`EntityNameService`，已有回调先执行、表名以它们为准）里把解析结果交给 SqlSugar（`SugarIndexAttribute`），`InitTables` 建表后按名字查不到才建（PostgreSQL 折成小写的名字按不区分大小写查到），已有的不改、不删，再次建表不重复建；表里已有重复数据时唯一索引建不出来，`InitTables` 失败。SqlSugar 的索引入口写不出条件，部分唯一索引不交给它：`SqlSugarConventions.InitTables(client, entityTypes)` 建表后按数据库方言补建（PostgreSQL、SQLite `CREATE UNIQUE INDEX IF NOT EXISTS … WHERE`，名字在 PostgreSQL 上同样是小写；SQL Server 查 `sys.indexes` 后 `CREATE UNIQUE INDEX … WHERE`，键含可空列时条件另加 `IS NOT NULL`，与 EF Core 在 SQL Server 上的缺省一致），已有同名索引不改、不删；特性开着 `InitTables` 时经它建表，应用自己建表（租户独立库）也要经它。特性启动时不论是否建表都核对：已登记的实体有部分唯一索引、而所落连接的数据库类型不在支持之列即启动失败。分表实体声明索引抛 `NotSupportedException`（分表用 SqlSugar 自己的 `[SugarIndex]` 与 `{split_table}`）。
- EF Core：`ApplyPersistenceConventions(Database)` 映射为 `HasIndex(列).IsUnique(…).HasDatabaseName(名字)`，表名取调用时的映射（`ToTable` 在它之前配置）；从属类型与共享类型实体跳过。部分唯一索引另加 `HasFilter`，条件按 `DatabaseFacade.ProviderName` 写方言（Npgsql `"IsDeleted" = false`、SQLite `"IsDeleted" = 0`、SQL Server `[IsDeleted] = 0`，键含可空列时另加 `IS NOT NULL`），列名取调用时的映射；其余关系型 Provider 在建模时抛出（`PERSISTENCE_UNAVAILABLE_008`），非关系型 Provider 本来不建索引、不加条件。特性启动时在独立作用域里取一次模型，建模的错误（索引声明不合法、数据库不支持部分索引）在启动时失败，不等第一次使用。
- 内存实现不建索引、不校验唯一性（只用于开发与测试）。
- 唯一键重复：关系型 Provider 的新增、更新与删除写入失败、又不是标识重复时，核对实体的唯一索引——这次写入里两项同键，或库里另一行（标识不同）同键；部分唯一索引只比较未删除的写入项与未删除的行——撞上即返回 `PERSISTENCE_CONFLICT_003`（参数 `entity`、`index`），这次写入的项统一报告，工作单元随后只能回滚，处理器忽略这次失败而返回成功时提交回滚并返回同一错误（与写入阶段的标识重复相同）。SqlSugar 另开不在事务里的连接查已提交的行（PostgreSQL 系语句失败后原事务已中止），EF Core 在 SaveChanges 回到的保存点之后于同一事务里查。核对不到的冲突（大小写不敏感排序规则下只差大小写、键含空值、SqlSugar 上与本工作单元先前未提交的行冲突）异常原样抛出，调用以 `KERNEL_UNEXPECTED_001` 结束。
- 框架自己的数据库记录（发件箱、审计、授予、翻译、调度、工作流）按实际的轮询与查询条件声明索引，清单见 ADR-0072。

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

## 运行时数据源

应用在运行时按自己的记录（名字、数据库类型、连接串，通常存在应用自己的库里）连外部库，如代码生成的外部数据源与零代码运行时。`Persistence.Abstractions` 的 `DataSourceRegistry`（单例）按定义建连接、按名缓存，Provider 经 `IDataSourceProvider` 接入（`AddDataSourceProvider<T>`）。运行时数据源与应用自己的连接互不相干：不加进作用域客户端、不参与实体落点、不进工作单元，约定连接 `default` / `shard` 与租户分片照旧。

| 项 | 规则 |
| --- | --- |
| 定义 | `DataSourceDefinition(name, databaseType, connectionString)`。名字 `^[a-z0-9][a-z0-9-]{0,63}$`，`default` 保留给应用的缺省连接；数据库类型不区分大小写，`DatabaseTypes` 列出常用名（`postgresql`、`mysql`、`sqlserver`、`sqlite`、`oracle`），Provider 认得哪些由 Provider 决定；`Describes(other)`：名字、数据库类型（不区分大小写）与连接串（区分大小写）都相同才是同一个连接 |
| 取用 | `GetOrAddAsync(definition)`：同名同定义返回已有实例；没有就建；定义变了建新实例替换旧的并释放旧的（替换时仍在旧实例上进行的调用可能失败）。建实例不连库，连不上的库照样登记，使用时报告。`GetAsync(name)` 只取本进程已登记的，没有 → `PERSISTENCE_NOTFOUND_002` |
| 失效 | 显式失效加定义比对，不用配置变化令牌（定义来自应用的库，不来自 `IConfiguration`）：记录改了、停用或删了，调 `RemoveAsync(names)`（批量，逐项返回是否存在过，同一批里重复的名字第二次为否，空集合返回空列表）；每次使用都以记录里当前的定义调 `GetOrAddAsync`，定义变了即替换。注册表是进程内的：多实例时改记录的实例调 `RemoveAsync`，其他实例在下一次以新定义 `GetOrAddAsync` 时替换；所以读库结构等入口收定义而不收名字，只按名字 `GetAsync` 的代码看不到别的实例上的修改 |
| 容量 | 登记数上限 `XiHan:Persistence:DataSources:MaxCount`（缺省 64，1–1024）。满了再登记新名字 → `PERSISTENCE_UNAVAILABLE_007`，不淘汰已有的；替换同名的不占名额 |
| 测试连接 | `TestAsync(definition)`：按定义建临时实例，打开一次连接再关闭，然后释放；不登记、不占名额、不影响同名的已登记实例。成功返回成功，连不上 → `PERSISTENCE_UNAVAILABLE_006`（参数 `name`、`reason`）；等多久由调用方的取消令牌与连接串里驱动自己的连接超时决定 |
| 健康 | 每个已登记的数据源一条 `persistence.datasource.<name>`（数据 `databaseType`、`provider`），按 `XiHan:Persistence:DataSources:HealthProbeIntervalSeconds`（缺省 15，也是单次探测的超时）在后台逐个打开连接再关闭，一轮没探完跳过下一轮；新登记的立即探一次。尚未探过 Started，成功 Ready，失败或超时 Degraded（`PERSISTENCE_UNAVAILABLE_006`），Degraded 不影响 readiness；探测期间被替换或移除的结果丢弃 |
| 机密 | 连接串不进日志、错误与健康详情。`DataSourceDefinition.ToString()` 不含连接串；错误与健康项的 `reason` 取驱动异常消息（含内层异常），先经 `ConnectionStrings.Redact(text, connectionString)` 把整串连接串与口令类键（键名含 `password`、`pwd`、`secret`、`token`、`key`，不区分大小写）的值换成 `***`；驱动消息可能带主机名、端口、库名与用户名，不带口令。日志只记名字与数据库类型 |
| 失败 | 数据库类型 Provider 不认识、连接串格式错 → `PERSISTENCE_VALIDATION_006`，同名的已登记实例不受影响；没有支持运行时数据源的 Provider → `PERSISTENCE_UNAVAILABLE_008` |

- SqlSugar：数据库类型是 `DbType` 枚举的名字（不区分大小写，`Custom` 与数字除外）。`SqlSugarDataSource.Client` 是每个数据源一个的 `SqlSugarScope`（线程安全，`IsAutoCloseConnection`），不施加实体约定、不挂过滤器与 AOP、不进工作单元，PostgreSQL 上不自动转小写（标识符按库里的原样引用）。登记时由驱动解析连接串，格式错即拒绝；探测与测试连接打开原生 `DbConnection` 再关闭。
- EF Core 与内存实现：不支持，注册表照样注册，取用、测试连接返回 `PERSISTENCE_UNAVAILABLE_008`。EF Core 的数据库 Provider（`UseNpgsql` 等）由应用选、框架包不引用，按数据库类型建连接需要对应的 Provider 包；内存实现没有数据库。

## 库结构读取与实体目录

`ISchemaReader` 读关系库的表与列，`IEntityCatalog` 枚举已登记的实体与映射；代码生成据此导入表、按最新库结构同步、沿用已有实体。

| 操作 | 语义 |
| --- | --- |
| `ListTablesAsync(dataSource, query)` | 表（不含列）：名字、注释、物理表、是否分表；按名字排序（不区分大小写）；`Keyword` 在名字与注释里不区分大小写地包含匹配；`Skip` / `Take` 分页，`Take` 取 1 到 `EntityQuery.DefaultMaxTake`（1000），超过在构造查询时以 `PERSISTENCE_VALIDATION_005` 拒绝（调用里转为校验结果）；返回过滤后的总数。库里的表一次读出、在内存里过滤与分页 |
| `GetTablesAsync(dataSource, tables)` | 按表名批量读结构（含列）。外层结果是整批的：一次最多 `SchemaLimits.MaxTablesPerRead`（100）张，超过 → `PERSISTENCE_VALIDATION_005`（参数 `count`、`max`）；连接或读取失败整批 `PERSISTENCE_UNAVAILABLE_006`。成功时逐项结果与入参一一对应：名字先精确匹配、再按不区分大小写的唯一匹配（分表用逻辑名），找不到该项 `PERSISTENCE_NOTFOUND_003`（参数 `dataSource`、`table`）；重复的名字各自返回同一结构；空集合不连库、返回空列表；每张表读列之前检查取消。`GetTableAsync` 是单项特例 |
| `IEntityCatalog.ListAsync()` | 每个以 `AddRepository` 登记的实体一条 `EntityMapping`（按登记顺序，同一实体登记多次只一条）：实体类型、标识类型、表名、是否分表、实现的协议 `EntityProtocols`（聚合根、创建时间、修改时间、并发戳、软删除、多租户）、列映射（属性、列名、CLR 类型、是否主键、是否可空，不映射的属性不列）、`[EntityIndex]` 声明的索引（`EntityIndexes.Resolve`，与建出的同名；Provider 自己的索引特性不在内） |

- 数据源：`null` 是应用的缺省连接（错误里称作 `default`），读取另开一个不在事务里的连接（PostgreSQL 系语句失败会中止原事务）；否则是运行时数据源的定义，读取前经 `GetOrAddAsync` 登记，读到的总是记录里当前的连接，定义不合法、登记已满按「运行时数据源」的错误。
- 列：名字、数据库报告的类型名（不含长度与精度，如 `varchar`、`int8`、`numeric`、`TEXT`）、序号、主键、自增（identity、serial、`AUTOINCREMENT`）、可空、长度（定点数族以外的类型；字符与二进制类型是声明的长度，SQLite、PostgreSQL 上整数与日期等为 null，其他数据库可能报告字节数或显示宽度）、精度与小数位（只给 `decimal` / `numeric` / `number` / `money` 族）、注释、缺省值（数据库报告的原文，如 `x`、`nextval(…::regclass)`）；数据库没有的项为 null。主键列一律报告为不可空。
- 分表折叠：已登记实体带 SqlSugar `[SplitTable]` 时，按它的表名模板（`{year}` 四位数字，`{month}`、`{day}` 两位）不区分大小写地匹配物理表，折叠成一张逻辑表：名字是模板去掉占位符、去掉首尾的 `_`（`Order_{year}{month}{day}` → `Order`），`PhysicalTables` 新到旧，注释与列取最新的物理表。缺省连接与运行时数据源都按已登记实体的模板折叠；没有登记实体的分表不折叠，物理表各自出现；物理表名不能按名字读（只认逻辑名）。
- 实体目录的表名与列名是映射声明的写法（分表是逻辑名）；PostgreSQL 等把未加引号的标识符折成小写的库里物理名可能只差大小写，对照时不区分大小写。
- 数据库限制：SQLite 没有表与列注释（`Comment` 为 null），`INTEGER PRIMARY KEY` 报告为可空（读取时按主键改为不可空），长度、精度与小数位取自声明的类型（SQLite 不强制），自增只认 `AUTOINCREMENT`。PostgreSQL 的注释、identity 与 serial 都能读到。SQL Server、MySQL、Oracle 的数值精度取 SqlSugar 报告的长度，本仓库未实测。
- SqlSugar：读表 `DbMaintenance.GetTableInfoList`、读列 `GetColumnInfosByTableName`（同步 API，在调用线程上执行）；实体目录取 `EntityMaintenance.GetEntityInfo`（施加了协议约定的映射）。
- EF Core：实体目录取 DbContext 模型（`GetTableName()`、`GetColumnName(表)`，影子属性不列）；库结构读取不支持（`PERSISTENCE_UNAVAILABLE_008`）：跨数据库的结构读取在 EF Core 里是设计期脚手架服务（`Microsoft.EntityFrameworkCore.Design` 的 `IDatabaseModelFactory`，按数据库 Provider 程序集反射发现），ADO.NET 的 `GetSchema` 各驱动返回的列不同、`Microsoft.Data.Sqlite` 不实现。
- 内存实现：实体目录的表名取实体类型名、列取有公开 setter 的非集合属性（列名即属性名，这是它的存储形态）；库结构读取不支持（`PERSISTENCE_UNAVAILABLE_008`）。

## 能力与配置

| 项 | 值 |
| --- | --- |
| 能力 | `persistence.relational@1`；`persistence.document@1` 预留 |
| 配置节 | `XiHan:Persistence`；内存实现 `XiHan:Persistence:InMemory:MaxEntitiesPerType`（默认 100000）；雪花标识 `XiHan:Persistence:Snowflake`：`Epoch`（ISO 8601，默认 `2026-01-01T00:00:00Z`）、`WorkerId`（0–1023，缺省租约分配）、`ClockDriftToleranceMilliseconds`（默认 100）、`LeaseTimeToLiveSeconds`（默认 30）；运行时数据源 `XiHan:Persistence:DataSources`：`MaxCount`（1–1024，默认 64）、`HealthProbeIntervalSeconds`（默认 15） |
| 默认实现 | `XiHan.Framework.Persistence.InMemory`，`role: default`，`stability: preview`，只用于开发与测试 |

## Provider

| Provider | 角色 | 要点 |
| --- | --- | --- |
| `Persistence.InMemory` | default | 每类实体一张有界表；不建索引、不校验唯一索引；覆盖层实现读己之写；读写都复制（字段逐一的浅复制）：查找与查询交出副本，写入存调用方实例的副本，读出的实例改了不经仓储写回就碰不到存储，集合等引用类型字段（含领域事件缓冲）仍与副本共享、不在隔离范围内；覆盖层记下每个写过的标识首次读到的已提交状态，提交时按涉及的表依次加锁、逐一校验（新增的标识已被提交 → `PERSISTENCE_CONFLICT_002`；更新或删除的行已不在、带并发戳的行戳已变 → `PERSISTENCE_CONFLICT_001`；容量 → `PERSISTENCE_UNAVAILABLE_002`），全部通过才应用，跨表原子 |
| `Persistence.EntityFrameworkCore` | provider | 应用持有 `DbContext`；工作单元在首次写入时开启数据库事务，每次写入立即 `SaveChanges`，批量写入是一次 `AddRange` / 多实体跟踪加一次 `SaveChanges`；软删除与租户过滤用 `EF.Property` 显式施加，不用全局过滤器；`ApplyPersistenceConventions(Database)` 把并发戳设为并发令牌、实体上的 `[EntityIndex]` 映射为 `HasIndex`（软删除实体的唯一索引加 `HasFilter`，见「索引声明」）；游离实体更新先比对并发戳；更新与删除时创建时间、租户与删除标记回到跟踪实例的原值（OriginalValues），写入路径按原值判定可见性，提交时保存的残余变更同样不改创建时间与租户；新增撞上主键约束时，SaveChanges 已回到自动建的保存点，在同一事务里核对标识，确已存在即按标识重复报告并把这次新增的实体移出跟踪；写入撞上唯一索引时在同一事务里核对，按唯一键重复报告 |
| `Persistence.SqlSugar` | provider | 作用域内的 `ISqlSugarClient` 由特性按回调创建或由应用注册，可以带多个连接（`ConfigId`）：特性创建时默认连接的 `ConfigId` 是 `default`，带租户分片的调用再加一个 `shard` 连接；每个实体落在哪个连接由 `IEntityConnectionSelector` 决定（默认规则：SqlSugar `[Tenant(configId)]` 特性指定的连接 → 实现 `IMultiTenant` 且客户端有 `shard` 连接时 `shard` → 否则默认连接），平台目录表始终走默认连接、租户表走分片连接；`[Tenant]` 指向客户端里没有的连接是装配错误，抛出携带 `PERSISTENCE_VALIDATION_004` 的 `FrameworkException`，不回退到默认连接，缺省落点规则下带 `[Tenant]` 的已登记实体在启动时就校验（`shard` 只在带分片的调用里存在，不在启动时校验）；工作单元经 `AsTenant()` 开启、提交、回滚事务，同一调用触及的全部连接一起提交或回滚（顺序提交，不是两阶段提交）；每次写入立即执行；批量用按对象入口 `InsertableByObject(list)` / `UpdateableByObject(list)` / `DeleteableByObject(list)`（删除按主键，开启差异日志的实体走泛型构建器），各一次调用；软删除与租户过滤以表达式显式施加；规约先改写再交给 SqlSugar——可空值类型与 null 的 `==` 改写成 `SqlFunc.EqualsNull(x, null)`、`!=` 改写成 `HasValue`，逻辑非下推到叶子（双重否定消去、德摩根展开、比较换反运算、`!x.HasValue` 换判空），因为 SqlSugar 把提升比较译成 `= NULL`，又会在与 / 或条件右侧的「取反作左操作数」处漏写运算符；`SqlSugarConventions.Apply()` 按协议配置主键、并发戳长度、`[EntityIndex]` 的索引（`InitTables` 建出，部分唯一索引由 `SqlSugarConventions.InitTables` 补建，见「索引声明」）、`DateTimeOffset` 的 UTC 往返（写入取 UTC 时刻；读取按驱动交出的 `DateTime.Kind` 还原——Npgsql legacy 时间戳模式把 `timestamptz` 换算成本地时间、Kind=Local 交出，须换回 UTC，否则多出本地时区偏移且经实体回写逐次累加；Utc / Unspecified 按 UTC；一致性套件除 SQLite 外还要在真实 PostgreSQL 上跑，环境变量 `XIHAN_TEST_POSTGRES`）、可空列与忽略列，实体不继承 SqlSugar 基类；更新与删除先在事务内加行锁取回当前行比对并发戳，更新时创建时间、租户与删除标记换成当前行的值，软删除在当前行上打标；新增撞上主键约束时另开一个不在事务里的连接（`CopyNew`，同一份连接配置）核对标识是否已被提交（PostgreSQL 系语句失败后事务已中止，不能在原事务里再查），确已存在即按标识重复报告，写入撞上唯一索引时同样另开连接核对、按唯一键重复报告；带 SqlSugar `[SplitTable]` 的实体自动走分表入口——查询在过滤条件之后 `SplitTable(from, to)`（按 `TimeRange` 的 UTC 时刻，没有提示扫全部分表），写入经 `InsertableByObject(x).SplitTable()` 等按 `[SplitField]` 落表，分表实体不加行锁靠并发戳，启动建表走 `CodeFirst.SplitTables()`字符串列的长度：`[StringLength(n)]`（或 `[MaxLength(n)]`）→ 长度 n，`Persistence.Abstractions` 的 `[LongText]`（或无参 `[MaxLength]`）→ 数据库的长文本类型，DataAnnotations 的 `[MaxLength]` 构造函数带 `RequiresUnreferencedCode`、框架记录不用；没有注解的沿用 SqlSugar 默认（PostgreSQL 为 `varchar(255)`），框架自己的数据库记录全部显式标注（架构测试门禁）；`InitTables`（ADR-0040）建缺表、加缺列，删列、改名、改类型不在契约内，是开发与测试的便利，生产库结构演进走迁移脚本——给已有行的表加非可空列时，PostgreSQL 上非可空的数值 / 枚举 / 布尔 / Guid 列由约定配 CLR 缺省值（`0` / `false` / 全零 Guid）回填旧行，SQLite 由 SqlSugar 加成可空列（旧行读回 CLR 缺省值），非可空字符串与日期时间列及其他数据库不受支持（声明成可空，或重建 / 迁移）；约定只作用于实体，声明在 BCL 类型上的属性原样交还 |

## 一致性

Provider 必须继承 `RepositoryConformance` 并通过全部用例，含时间范围提示对不分表实体不过滤的用例与批量用例：批量新增读己之写、批内重复标识逐项报告、批量更新刷新并发戳且过期戳按项冲突、批量删除软删 / 物理删与不存在按项报告、`FindManyAsync` 的顺序与缺失、空集合；可空值类型与 null 的相等 / 不等按存在性过滤（字面量 null、闭包里的 null 变量、左右互换、与 `HasValue` 一致，组合到别的条件里也成立）；一次查询最多取 `EntityQuery.DefaultMaxTake` 条、超过的取出条数在构造查询时以 `PERSISTENCE_VALIDATION_005` 拒绝，调用里先写入再构造超限查询的调用以该码结束、写入回滚；判空与取反嵌套在与 / 或条件里照常过滤（或条件在与条件右侧、判空或取反是或条件的左操作数、取反整个与 / 或条件、双重否定）；更新沿用存储的创建时间、租户与删除标记、删除不改创建时间（游离实例与读出后改过的实例两种）；两个工作单元先后新增同一标识只有一个成功、另一个返回 `PERSISTENCE_CONFLICT_002`、留下的是成功者的行；回滚的调用在存储里不留修改、并发戳与删除标记，提交前另一个工作单元读到的是旧值；只读操作里单条与批量的新增、更新、删除都返回 `PERSISTENCE_UNEXPECTED_001`、存储不变，处理器忽略拒绝而返回成功时调用仍以它结束，查询照常；只读操作在新的服务作用域里调用写操作，写入照常提交，外层仍拒绝自己的写入；同一作用域里再经管道的调用以 `PERSISTENCE_VALIDATION_002` 结束、处理器不执行。套件以 `Testing.TestHost` 走完整管道，用自带的 Tenant 阶段中间件从 Baggage 的 `xihan.tenant` 解析租户，不依赖 MultiTenancy 子系统。

运行时数据源：支持它的 Provider 给出一个连得上的定义与一个连不上的定义（子类覆写），用例验证测试连接成功与失败（失败是 `PERSISTENCE_UNAVAILABLE_006`，错误消息与参数里没有连接串与口令）、同定义复用、定义变了替换、批量移除后 `GetAsync` 返回 `PERSISTENCE_NOTFOUND_002`；不给定义的 Provider（EF Core、内存实现）断言取用与测试连接都返回 `PERSISTENCE_UNAVAILABLE_008`，不返回空结果冒充。

库结构与实体目录（`SchemaConformance`）：实体目录列出套件的三种实体各一条（重复登记只一条），标识类型、协议与主键列对；读结构的 Provider 列出这三张表、关键字与分页照常、读到主键列（不可空）与普通列、不存在的表该项 `PERSISTENCE_NOTFOUND_003`、超过 100 张整批 `PERSISTENCE_VALIDATION_005`、空集合返回空列表、经运行时数据源读同一个库结果一致；不读结构的 Provider 断言列表与读取都返回 `PERSISTENCE_UNAVAILABLE_008`。

标识生成器必须通过 `IdGeneratorConformance<TKey>`：顺序生成与多线程并发生成不重复、时钟前进后生成的标识更大；`GuidV7IdGenerator` 与 `SnowflakeIdGenerator` 都通过。

## 错误码

见 `conventions/error-codes.md` 中 `PERSISTENCE_*`。

## 不做什么

- 不提供实体基类、审计基类或"全能仓储"。
- 不在抽象层暴露 `IQueryable`；复杂查询由 Provider 专属包（如 `Persistence.EntityFrameworkCore` 的 `DbContext`）承担。
- 不提供按查询条件的批量更新 / 删除（`UPDATE … WHERE`）：它绕过协议维护的审计、并发与软删除字段，由 Provider 专属包按需提供。
- 不提供跨 Provider 的分布式事务。
