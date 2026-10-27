# RFC-0005 领域协议与持久化

> 状态：已采纳（2026-10-23 修订：批量方法；2026-10-27 修订：SqlSugar 多连接落点与分表、查询时间范围提示、领域事件派发；2026-10-27 修订：雪花标识生成器） | 日期：2026-10-21 | 对应 ADR：0005、0013、0022、0023、0025、0033 | 参考实现：`XiHan.Framework.Domain`、`XiHan.Framework.Persistence.Abstractions`、`XiHan.Framework.Persistence.InMemory`、`XiHan.Framework.Persistence.EntityFrameworkCore`、`XiHan.Framework.Persistence.SqlSugar` | 一致性：`XiHan.Framework.Persistence.Conformance`

## 摘要

领域层只定义协议，不提供基类；持久化层定义仓储、工作单元与查询的语义，Provider 按实体实现的协议自动维护审计、租户、并发与软删除字段。换 Provider 只改配置与包引用，业务代码不变。

## 领域协议

| 协议 | 字段 | Provider 行为 |
| --- | --- | --- |
| `IEntity<TKey>` | `Id` | 标识；`TKey` 必须可比较相等 |
| `IAggregateRoot<TKey>` | `DomainEvents`、`ClearDomainEvents()` | 仓储把本工作单元内新增 / 更新 / 删除过的聚合登记到 `IUnitOfWork.DomainEvents`（按首次写入顺序、同一实例一次）；Transaction 阶段在处理器成功后、提交前把它们尚未派发的事件交给 `IDomainEventDispatcher`（由 Messaging 子系统注册，见 RFC-0009），派发成功即清空缓冲，派发失败回滚并以失败结果结束调用；有事件却没有注册派发器 → `PERSISTENCE_VALIDATION_003`（不静默丢弃） |
| `IHasCreationTime` | `CreatedAt` | 新增时写入当前 UTC 时间 |
| `IHasModificationTime` | `ModifiedAt` | 更新时写入当前 UTC 时间 |
| `IHasConcurrencyStamp` | `ConcurrencyStamp` | 新增与更新时刷新；更新时比对，不符返回 `PERSISTENCE_CONFLICT_001` |
| `ISoftDeletable` | `IsDeleted`、`DeletedAt` | 删除改为打标；查询默认过滤 |
| `IMultiTenant` | `TenantId` | 新增时写入当前租户；查询只返回当前租户的实体；无当前租户时新增返回 `PERSISTENCE_VALIDATION_001` |
| `IValueObject` | — | 标记；用 `record` 取得值相等 |
| `ISpecification<T>` | `Criteria`、`IsSatisfiedBy` | 同一条件既能内存判定也能交给查询提供器；`And` / `Or` / `Not` 组合必须共享同一参数 |

领域事件集合有界（默认 256），超出抛出 `InvalidOperationException`：那是忘记提交的编程错误。

## 工作单元

- 作用域生命周期，一次调用一个；由 Transaction 阶段的 `UnitOfWorkMiddleware` 开启，用安全上下文中的租户作为当前租户。
- 状态机：`NotStarted → Active → Committed | RolledBack`。违反顺序抛出携带 `PERSISTENCE_VALIDATION_002` 的 `FrameworkException`。
- 调用结果为成功则提交；结果为失败或抛出异常则回滚。提交失败（并发冲突、标识重复、容量）作为调用结果返回。
- 只读工作单元拒绝写入，返回 `PERSISTENCE_VALIDATION_002`；Provider 可在写入时或提交时拒绝。
- 读己之写：同一工作单元内新增、更新、删除对随后的查找与查询可见；提交前对其他工作单元不可见。
- 领域事件在提交前派发（`UnitOfWorkMiddleware`：处理器成功 → 派发 → 提交）：装配了发件箱时事件随业务写入同一事务落进发件箱表，提交后由投递循环发出；没有发件箱时直接发布到代理，发布失败回滚业务写入（至多一次、先于提交）。派发只在 Transaction 阶段的中间件里发生，直接使用工作单元的调用方自己负责。

## 仓储

| 操作 | 语义 | 可恢复失败 |
| --- | --- | --- |
| `FindAsync(id)` | 已软删除或不属于当前租户视为不存在 | — |
| `ListAsync(query)` | 过滤、排序、分页；`Take` 上限默认 1000，Provider 不得超出；`TimeRange`（`Within(from, to)`）是时间范围提示——按时间分表 / 分区的 Provider 据此只扫描范围内的表，不是过滤条件，不分表的实体与不支持范围的 Provider 忽略它 | — |
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
- 空集合不触后端、返回空列表，但仍要求活动的工作单元。
- 读己之写对批量同样成立：批量新增后同一工作单元内可查到全部实体。

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
| `Persistence.InMemory` | default | 每类实体一张有界表；覆盖层实现读己之写，提交时原子应用；存引用不拷贝 |
| `Persistence.EntityFrameworkCore` | provider | 应用持有 `DbContext`；工作单元在首次写入时开启数据库事务，每次写入立即 `SaveChanges`，批量写入是一次 `AddRange` / 多实体跟踪加一次 `SaveChanges`；软删除与租户过滤用 `EF.Property` 显式施加，不用全局过滤器；`ApplyPersistenceConventions()` 把并发戳设为并发令牌；游离实体更新先比对并发戳 |
| `Persistence.SqlSugar` | provider | 作用域内的 `ISqlSugarClient` 由特性按回调创建或由应用注册，可以带多个连接（`ConfigId`）：特性创建时默认连接的 `ConfigId` 是 `default`，带租户分片的调用再加一个 `shard` 连接；每个实体落在哪个连接由 `IEntityConnectionSelector` 决定（默认规则：SqlSugar `[Tenant(configId)]` 特性指定的连接 → 实现 `IMultiTenant` 且客户端有 `shard` 连接时 `shard` → 否则默认连接），平台目录表始终走默认连接、租户表走分片连接；工作单元经 `AsTenant()` 开启、提交、回滚事务，同一调用触及的全部连接一起提交或回滚（顺序提交，不是两阶段提交）；每次写入立即执行；批量用 `Insertable(list)` / `Updateable(list)` / `Deleteable().In(ids)`；软删除与租户过滤以表达式显式施加；`SqlSugarConventions.Apply()` 按协议配置主键、并发戳长度、`DateTimeOffset` 的 UTC 往返、可空列与忽略列，实体不继承 SqlSugar 基类；更新与删除先在事务内加行锁取回当前行比对并发戳；带 SqlSugar `[SplitTable]` 的实体自动走分表入口——查询在过滤条件之后 `SplitTable(from, to)`（按 `TimeRange` 的 UTC 时刻，没有提示扫全部分表），写入经 `InsertableByObject(x).SplitTable()` 等按 `[SplitField]` 落表，分表实体不加行锁靠并发戳，启动建表走 `CodeFirst.SplitTables()` |

## 一致性

Provider 必须继承 `RepositoryConformance` 并通过全部用例，含时间范围提示对不分表实体不过滤的用例与批量用例：批量新增读己之写、批内重复标识逐项报告、批量更新刷新并发戳且过期戳按项冲突、批量删除软删 / 物理删与不存在按项报告、`FindManyAsync` 的顺序与缺失、空集合。套件以 `Testing.TestHost` 走完整管道，用自带的 Tenant 阶段中间件从 Baggage 的 `xihan.tenant` 解析租户，不依赖 MultiTenancy 子系统。

标识生成器必须通过 `IdGeneratorConformance<TKey>`：顺序生成与多线程并发生成不重复、时钟前进后生成的标识更大；`GuidV7IdGenerator` 与 `SnowflakeIdGenerator` 都通过。

## 错误码

见 `conventions/error-codes.md` 中 `PERSISTENCE_*`。

## 不做什么

- 不提供实体基类、审计基类或"全能仓储"。
- 不在抽象层暴露 `IQueryable`；复杂查询由 Provider 专属包（如 `Persistence.EntityFrameworkCore` 的 `DbContext`）承担。
- 不提供按查询条件的批量更新 / 删除（`UPDATE … WHERE`）：它绕过协议维护的审计、并发与软删除字段，由 Provider 专属包按需提供。
- 不提供跨 Provider 的分布式事务。
