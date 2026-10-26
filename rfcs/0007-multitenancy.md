# RFC-0007 多租户

> 状态：已采纳（2026-10-26 修订：租户分片；2026-10-27 修订：租户目录的数据库 Provider） | 日期：2026-10-21 | 对应 ADR：ADR-0018（分片）、ADR-0026（数据库 Provider） | 参考实现：`XiHan.Framework.MultiTenancy.Abstractions`、`XiHan.Framework.MultiTenancy.InMemory`、`XiHan.Framework.MultiTenancy.Database` | 一致性：`XiHan.Framework.MultiTenancy.Conformance`

## 摘要

租户是调用上下文的一部分，由 Tenant 阶段一次解析、校验并写入安全上下文；之后的阶段（授权、工作单元、缓存键）只读它。没有环境态访问器。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `multitenancy.context@1` | 租户解析、校验与存储 | `MultiTenancy.InMemory`（从配置装载租户）；`MultiTenancy.Database` 是租户目录进数据库的显式 Provider |
| `multitenancy.sharding@1` | 按租户选择数据连接（分库） | `MultiTenancy.InMemory`（从租户属性推出）；`MultiTenancy.Database` 同样从数据库里的租户属性推出 |

## 租户 ID

`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`，区分大小写。它出现在 Baggage `xihan.tenant`、`xrn` 资源名的租户段、持久化实体的 `TenantId` 中，三处同一文法。

## 解析

1. 按注册顺序调用 `ITenantResolver`，第一个有值的结果生效。默认顺序：已认证主体的 `tenant` 声明 → Baggage `xihan.tenant`。已认证身份所属的租户优先于客户端自报的租户。
2. 没有解析到时用 `DefaultTenant`；仍没有时，`RequireTenant` 为真返回 `MULTITENANCY_VALIDATION_002`，否则无租户继续。
3. 校验格式，不合法返回 `MULTITENANCY_VALIDATION_001`。
4. `ValidateAgainstStore` 为真（默认）时核对 `ITenantStore`：不存在返回 `MULTITENANCY_NOTFOUND_001`，禁用返回 `MULTITENANCY_DENIED_001`；显示名随之写入上下文。
5. 写入 `SecurityContext.Tenant`。

宿主适配器（P3）把 HTTP 头或主机名映射为 Baggage，不直接写租户。

## 存储

`ITenantStore`：按 ID 查找、枚举。租户信息含显示名、启用标记与字符串属性（租户专属连接、配额等）。内存实现有界（默认 1000），从 `XiHan:MultiTenancy:Tenants` 装载。

`MultiTenancy.Database`（ADR-0026）：租户是实体行 `TenantRecord`（ID、显示名、启用标记、属性 JSON、创建 / 修改时间与并发戳；不按租户过滤、不软删除），`ITenantStore` 单例在新作用域直接开只读工作单元读仓储（不经管道），按租户有界缓存查找结果（含"不存在"）并按 TTL 失效（`XiHan:MultiTenancy:Database`：`CacheSize` 默认 10000、`CacheTimeToLiveSeconds` 默认 30 且 0 为不过期、`MaxListedTenants` 默认 10000）；`TenantRecordStore` 在调用方的工作单元里批量新增 / 更新 / 删除并逐项返回结果，写入后失效目录与分片解析器的缓存。它不在清单里声明对 `persistence.relational@1` 的依赖（持久化 Provider 以可选依赖引用 `multitenancy.sharding@1`，再声明即成环），改为启动期核对能力图谱里有持久化 Provider，没有即以 `KERNEL_CAPABILITY_001` 中断启动。

## 分片

`multitenancy.sharding@1`：按租户选择数据连接。Provider 实现 `ITenantShardResolver`：`ResolveAsync(tenantId)` 返回 `Result<Maybe<TenantShard>>`——无分片（共库租户、未知租户）是成功且无值，租户存储不可达返回 `MULTITENANCY_UNAVAILABLE_002`；`Invalidate(tenantId?)` 丢弃缓存（空为全部）。

- `TenantShard`：租户 ID、连接串、可选的提供者名（小写，如 `postgresql`）、透传给持久化 Provider 的属性。
- 默认解析器 `TenantStoreShardResolver` 从 `ITenantStore` 的租户属性推出分片：`ConnectionString` 有值即分库，`DatabaseProvider` 为提供者名，`Shard.<key>` 透传为 `<key>`（用点不用冒号：属性来自配置时冒号是节分隔符）。结果按租户缓存（含"无分片"），容量 `ShardCacheSize`（默认 10000，满时整体清空）；租户连接变更后调用 `Invalidate`。
- Tenant 阶段在租户写入安全上下文之后解析分片（装配了解析器才解析），写进调用上下文（`GetTenantShard` / `SetTenantShard`）；解析失败即调用失败，不降级为共库。无租户的调用没有分片。
- 持久化 Provider 在作用域内创建客户端时读当前调用的分片，不触存储、不阻塞：`TenantShardBinding.Resolve` 把分片表示成连接绑定（主值是分片连接串，属性并入原绑定属性、分片属性与 `tenant` / `provider`）；清单以可选依赖声明 `multitenancy.sharding@1?`。分片是**实体级**落点而不是整个作用域的切换：带分片的调用里，Provider 同时持有应用默认连接与分片连接，租户数据（实现 `IMultiTenant` 的实体）走分片连接，平台目录表（租户、授予、翻译、发件箱等不带租户的实体）仍走默认连接，两者在同一工作单元里一起提交或回滚（`Persistence.SqlSugar` 经多 `ConfigId` 客户端与 `AsTenant()` 事务实现；`Persistence.EntityFrameworkCore` 一个 `DbContext` 一个连接，分片调用整体切到分片连接，平台表要另起 `DbContext` 由应用处理）。分片库的表结构由应用管理，框架的建表开关只作用于默认连接。

## 一致性

Provider 必须继承 `TenantStoreConformance` 并通过全部用例；分片解析器必须继承 `TenantShardResolverConformance`（分库租户解析出连接串、小写提供者名与去前缀的透传属性；共库与未知租户为成功无值；失效后重解析结果一致）。

## 错误码

见 `conventions/error-codes.md` 中 `MULTITENANCY_*`。
