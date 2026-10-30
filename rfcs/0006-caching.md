# RFC-0006 缓存、分布式锁与幂等

> 状态：已采纳（2026-10-23 修订：批量方法；2026-10-26 修订：`IDistributedCache` 适配、两级缓存） | 日期：2026-10-21 | 对应 ADR：— | 参考实现：`XiHan.Framework.Caching.Abstractions`、`XiHan.Framework.Caching.InMemory`、`XiHan.Framework.Caching.Redis` | 一致性：`XiHan.Framework.Caching.Conformance`

## 摘要

缓存子系统只定义一个字节级存储契约，类型化缓存、分布式锁与幂等去重都建立在它之上。Provider 只需实现五个原子操作，就自动获得锁与幂等。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `caching.local@1` | 进程内缓存，`ICache` | `Caching.InMemory`（`Caching.Hybrid` 以两级缓存顶替） |
| `caching.distributed@1` | 跨进程共享缓存，`ISharedCache`、`IDistributedLock`、幂等中间件 | `Caching.InMemory`（单实例下正确；多实例换 `Caching.Redis`） |

内存实现同时声明两种能力；另有显式 Provider 提供 `caching.distributed` 时，内存实现只为被选中的 `caching.local` 注册服务（RFC-0002 第 4 条）。

## 存储契约

| 操作 | 语义 |
| --- | --- |
| `Get` | 不存在或已过期为无值；滑动过期的条目在读取时延长 |
| `Set` | 写入或覆盖 |
| `TryAdd` | 不存在（或已过期）才写入，原子 |
| `Remove` | 删除并报告是否存在 |
| `TryRemove(expected)` | 当前值等于期望值才删除，原子 |
| `GetMany(keys)` | 逐键同 `Get`，一次往返；结果与键顺序一一对应 |
| `SetMany(entries)` | 逐项同 `Set`（每项自带过期选项），一次往返；同一键出现多次后者覆盖 |
| `RemoveMany(keys)` | 逐键同 `Remove`，一次往返；结果与键顺序一一对应，同一键出现多次第二次为假 |

批量方法按 `conventions/batch-first.md`：空集合不触后端；基础设施失败整批抛出 `CACHING_UNAVAILABLE_001`；`TryAdd` 与 `TryRemove` 保持单键（它们是锁与幂等的原子原语，批量没有意义）。Provider 用原生批量（Redis `MGET`、管道里的 `SET` 带过期、管道里的 `DEL`），内存实现一次加锁逐项处理。

- 键：非空、无空白与控制字符、不超过 512 字符、区分大小写。
- 过期：绝对存活或滑动过期，二选一；都不设时用 Provider 的默认存活时间。
- 基础设施失败抛出 `CACHING_UNAVAILABLE_001`，不伪装成未命中。
- 返回的字节不可修改；实现要么复制，要么保证不被改写。

### 键管理（可选，ADR-0055）

管理界面用的 `ICacheStoreInspector`，存储能做就实现（内存与 Redis 实现，远程插件不实现）。管理界面从容器取 `ICacheStore`，转成它即可用；读写值仍走 `ICacheStore`。

| 操作 | 语义 |
| --- | --- |
| `ListKeys(query)` | 按前缀（`CacheKeyQuery.Prefix`，空为全部）分页列出键，`Limit` 1–1000（默认 100），`Continuation` 续页（没有更多为空） |
| `Inspect(key)` | 条目的载荷字节数、剩余存活时间（不过期为空）与滑动窗口；不存在或已过期为无值 |
| `RemoveByPrefix(prefix)` | 删除前缀下的全部键，返回删除数；前缀不能为空 |

- 前缀与键同文法（无空白与控制字符，不超过 512 字符），区分大小写。
- 内存实现按键的序数顺序分页，续页标记是上一页最后一个键。
- Redis 实现按单个端点的 `SCAN` 分页：续页标记是 SCAN 游标，页大小是提示值，同一个键可能出现在两页；前缀里的 glob 字符转义。

### 变更通知（可选，ADR-0055）

共享存储实现 `ICacheChangeFeed`：`Subscribe(handler)` 收到本实例与其他实例对存储的写入与删除（`CacheChange`：键列表，或按前缀删除时的前缀）。

- 本实例的变更在写入完成时同步通知，其他实例的经后端转发，不回送给发出者。
- 通知只用于失效本地副本，可能丢失（断线期间），不是可靠的消息。
- Redis 实现在每次写入与删除后，以发后不等的方式向频道 `<KeyPrefix>__changes` 发布（`XiHan:Caching:Redis:PublishChanges`，默认开）。

## 类型化缓存

`ICache`：`GetAsync<T>`、`SetAsync<T>`、`RemoveAsync`、`GetOrCreateAsync<T>`（未命中时各自执行工厂，不合并并发），以及批量的 `GetManyAsync<T>(keys)`、`SetManyAsync<T>(entries, options)`、`RemoveManyAsync(keys)`，语义与存储契约的批量方法一致，结果与入参顺序一一对应。进程内实现直接存对象引用，不序列化；跨进程实现用 `SerializingCache` 把存储与 `ICacheSerializer` 组合。默认序列化器是 System.Text.Json，要求选项带 `TypeInfoResolver`，通过 `JsonSerializerOptions.GetTypeInfo` 取类型信息，AOT 下无反射；反射版只在允许反射的运行时可用。

## 分布式锁

租约锁：`TryAcquire(resource, ttl)` 即 `TryAdd("lock:" + resource, token, ttl)`；释放即 `TryRemove(key, token)`。不可重入、不续约；到期后他人可获取，原持有者释放返回 `false`。

## 幂等

Idempotency 阶段的中间件按 Baggage `xihan.idempotency-key` 去重，作用域为（租户、主体、键）：

1. 首次调用用 `TryAdd` 写入"处理中"标记（`InProgressTimeToLive`，默认 60 秒）并执行处理器。
2. 处理器完成后用调用结果（成功，或错误码与消息）替换标记，保留 `TimeToLive`（默认 24 小时）。
3. 处理器抛出异常时比较删标记，允许重试。
4. 重复调用：标记仍为"处理中"返回 `CACHING_CONFLICT_002`；已完成则回放结果、不执行处理器，并在上下文标记为回放。

回放的是调用结果，不是响应体；操作契约（P3）在此之上回放响应。

## 两级缓存

`Caching.Hybrid` 提供 `caching.local@1`、依赖 `caching.distributed@1`：`ICache` 是 L1（进程内对象引用，有界 LRU）在前、L2（被选中的 `ISharedCache`）在后的两级缓存。

- 读：L1 命中直接返回；未命中读 L2，命中后写进 L1。批量读先在 L1 取全部，只把未命中的键一次交给 L2。
- 写与删：先 L2 再 L1；`GetOrCreate` 两级都未命中才执行工厂，结果写两级。
- L1 的存活时间是条目过期与 `LocalTimeToLive`（默认 30 秒）的较小者，滑动过期在 L1 里按绝对时间处理；`LocalCapacity`（默认 10000）满时淘汰最久未访问的条目。
- 跨实例失效（ADR-0055）：L2 的存储实现了 `ICacheChangeFeed` 时（Redis），本实例与其他实例对存储的写入与删除，都会让 L1 里的这些键（或前缀）立即失效；管理界面直接改存储也一样。
- 没有变更通知时（远程插件等），别的实例的写入不会主动失效本实例的 L1：读到旧值的时间最长为 `LocalTimeToLive`；通知丢失时也以此为上限。需要更强一致的键应直接用 `ISharedCache`。
- `ICache.Store` 是 L2 的字节存储；L1 直接存对象引用，调用方不得修改取回的对象。

## IDistributedCache 适配

.NET 原生的 `Microsoft.Extensions.Caching.Distributed.IDistributedCache` 由 `CacheStoreDistributedCache` 架在 `ICacheStore` 上，沿用该接口的代码（会话、数据保护、第三方库）不改调用点：

- 键须满足 `CacheKey` 文法，否则抛 `ArgumentException`。
- 绝对过期（时刻或相对现在，两者都给取较早者）换算成 `TimeToLive`，已过去的时刻抛 `ArgumentOutOfRangeException`；没有绝对过期时滑动过期对应 `SlidingExpiration`；都给时绝对优先（存储不能同时表达两者）；都没有则不过期。
- `Refresh` 是一次读取（读取即续期）；`Remove` 不存在的键不报错。
- 同步成员在适配器边界阻塞等待异步存储，调用方应优先用异步成员。
- 提供 `caching.distributed@1` 的 Provider 被选中时登记它（`AddXiHanDistributedCache`，幂等）；应用自己登记的 `IDistributedCache` 优先。

## 一致性

Provider 必须继承 `CacheStoreConformance` 并通过全部用例（含基于存储的锁与批量：`GetMany` 的顺序与未命中、`SetMany` 的覆盖与各自过期、`RemoveMany` 的逐键结果、空集合）。过期用例用真实时间等待。

## 错误码

见 `conventions/error-codes.md` 中 `CACHING_*`。
