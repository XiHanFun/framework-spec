# RFC-0006 缓存、分布式锁与幂等

> 状态：已采纳（2026-10-23 修订：批量方法；2026-10-26 修订：`IDistributedCache` 适配、两级缓存；2026-11-01 修订：幂等回放操作的响应；2026-11-01 修订：键列表的每页上限越界返回校验错误；2026-11-12 修订：幂等作用域键里主体带长度前缀、外层请求把幂等键交给内层调用；2026-11-14 修订：代数缓存区、组件登记缓存类型、两级缓存回填核对失效；2026-11-24 修订：键与前缀的长度按 UTF-8 字节计，代数缓存区的直写上限随之按字节计；同日再修订：`GetOrCreateAsync` 合并同键的并发未命中，两级缓存自己合并、不再调用 L2 的 `GetOrCreate`；2026-11-26 修订：键与前缀不能含孤立的代理项，代数缓存区散列这样的范围与条目时按 WTF-8 取字节，远程 Provider 的适配器严格编码键；同日写明：插件代理交回空载荷时幂等记下没有响应体，回放出同样的空载荷；2026-11-27 修订：Redis 实现的键前缀不含孤立的代理项） | 日期：2026-10-21 | 对应 ADR：ADR-0064（幂等响应回放）、ADR-0084（键列表分页越界）、ADR-0229（代数缓存区与跨实例失效）、ADR-0271（合并同键并发）、ADR-0274（.NET 的键长度按字节计）、ADR-0276（键长度的单位）、ADR-0316（键里的孤立代理项）、ADR-0318（幂等记下没有响应体）、ADR-0321（Redis 键前缀里的孤立代理项） | 参考实现：`XiHan.Framework.Caching.Abstractions`、`XiHan.Framework.Caching.InMemory`、`XiHan.Framework.Caching.Redis` | 一致性：`XiHan.Framework.Caching.Conformance` | 一致性档：`caching`

## 摘要

缓存子系统只定义一个字节级存储契约，类型化缓存、分布式锁与幂等去重都建立在它之上。Provider 只需实现下文「存储契约」的操作，就自动获得锁与幂等。

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
| `TryRefresh(expected, ttl)` | 当前值等于期望值才把到期时刻重设为现在加 `ttl`，原子；不存在、已过期或值不同为假，不改值，不改滑动窗口（2026-11-13 增补，ADR-0215） |
| `GetMany(keys)` | 逐键同 `Get`，一次往返；结果与键顺序一一对应 |
| `SetMany(entries)` | 逐项同 `Set`（每项自带过期选项），一次往返；同一键出现多次后者覆盖 |
| `RemoveMany(keys)` | 逐键同 `Remove`，一次往返；结果与键顺序一一对应，同一键出现多次第二次为假 |

批量方法按 `conventions/batch-first.md`：空集合不触后端；基础设施失败整批抛出 `CACHING_UNAVAILABLE_001`；`TryAdd`、`TryRemove` 与 `TryRefresh` 保持单键（它们是锁与幂等的原子原语，批量没有意义）。Provider 用原生批量（Redis `MGET`、管道里的 `SET` 带过期、管道里的 `DEL`），内存实现一次加锁逐项处理。

- 键：非空、无空白与控制字符、不含孤立的代理项、按 UTF-8 编码后不超过 512 字节、区分大小写（2026-11-24 修订，ADR-0276；以前写「不超过 512 字符」，没说单位：.NET 按 UTF-16 码元计，Go SDK 按 UTF-8 字节计，含非 ASCII 字符的键两边判得不一样，.NET 收下的键远程插件会以 `CACHING_VALIDATION_001` 拒绝）。空白指 RFC-0001「空白字符」的 25 个码位，控制字符指 Unicode 的 Cc 类（U+0000–U+001F 与 U+007F–U+009F；同日写明，.NET 的 `char.IsControl` 与 Go 的 `unicode.IsControl` 本来就是它）。长度只看编码后的字节数，不看字符数或 UTF-16 码元数：256 个两字节字符正好到上限，171 个三字节字符（513 字节）已经超过。向量 `conformance/vectors/caching/cache-keys.json`。
  - 孤立的代理项（2026-11-26 修订，ADR-0316；以前键的文法不排除它）与 RFC-0012「标识」同义：不在一对之中的 U+D800–U+DFFF 码元，即单独的高代理项、单独的低代理项、颠倒的一对与末尾没有低代理项跟着的高代理项；成对的代理项是平面以外的一个字符，照常合法。以码位表示字符串的实现（Python）里不能有 U+D800–U+DFFF 的码位，以字节表示字符串的实现（Go、PHP）里键必须是合法的 UTF-8。
  - 理由：键要编成 UTF-8 字节交给后端（Redis 的键、远程 Provider 操作载荷里的 JSON 字符串），孤立的代理项编不成 UTF-8。宽松的编码把它换成 U+FFFD（System.Text.Json 写成转义的 `\uFFFD`），两个不同的键就撞成同一个条目：一个键的写入覆盖另一个键的值，锁与幂等记录也落到别人的条目上。不合法与其余文法同样处理：`CacheKey` 的构造抛出 `ArgumentException`，`CacheKey.IsValid` 为假。
- 过期：绝对存活或滑动过期，二选一；都不设时用 Provider 的默认存活时间。
- 基础设施失败抛出 `CACHING_UNAVAILABLE_001`，不伪装成未命中。
- 返回的字节不可修改；实现要么复制，要么保证不被改写。

### 键管理（可选，ADR-0055）

管理界面用的 `ICacheStoreInspector`，存储能做就实现（内存与 Redis 实现，远程插件不实现）。管理界面从容器取 `ICacheStore`，转成它即可用；读写值仍走 `ICacheStore`。

| 操作 | 语义 |
| --- | --- |
| `ListKeys(query)` | 按前缀（`CacheKeyQuery.Prefix`，空为全部；不合法不在构造时抛出，返回 `CACHING_VALIDATION_004`）分页列出键，`PageSize` 1–1000（默认 100），`ContinuationToken` 续页（没有更多为空）；返回 `Result<CacheKeyPage>`。`PageSize` 越界不在构造时抛出：存储先经 `CacheKeyQuery.Validate()` 核对，越界返回 `CACHING_VALIDATION_003`（HTTP 400，消息键 `caching.inspector.page_size_invalid`，参数 `pageSize`、`max`），不访问后端——查询常由管理界面的请求绑定，越界是调用方可以改正的输入（2026-11-01 修订，ADR-0084）；基础设施失败仍抛 `CACHING_UNAVAILABLE_001` |
| `Inspect(key)` | 条目的载荷字节数、剩余存活时间（不过期为空）与滑动窗口；不存在或已过期为无值 |
| `RemoveByPrefix(prefix)` | 删除前缀下的全部键，返回 `Result<long>`（删除数）；前缀为空或不合法返回 `CACHING_VALIDATION_004`、不访问后端（2026-11-11 修订，ADR-0156）；基础设施失败仍抛出 |

- 前缀与键同文法（无空白与控制字符、不含孤立的代理项，按 UTF-8 编码后不超过 512 字节，与键是同一条规则，只是可以为空；2026-11-24 修订，ADR-0276；孤立的代理项 2026-11-26 修订，ADR-0316），区分大小写。
- 内存实现按键的序数顺序分页，续页标记是上一页最后一个键。
- Redis 实现按单个端点的 `SCAN` 分页：续页标记是 SCAN 游标，页大小是提示值，同一个键可能出现在两页；前缀里的 glob 字符转义。

对象条目（ADR-0058）：进程内的类型化缓存把对象引用存在自己的表里，不经 `ICache.Store` 的字节存储，`ICacheStoreInspector` 看不到。这样的缓存实现可选的 `ICacheInspector`：

| 操作 | 语义 |
| --- | --- |
| `ListKeys(query)` | 同字节存储：按前缀分页列出对象条目的键，`PageSize` 越界同样返回 `CACHING_VALIDATION_003` |
| `Inspect(key)` | 条目里对象的类型全名、剩余存活时间（不过期为空）与滑动窗口；不存在或已过期为无值；不读出值、不延长滑动过期 |
| `RemoveByPrefix(prefix)` | 删除前缀下的对象条目，返回 `Result<long>`；前缀为空或不合法同样返回 `CACHING_VALIDATION_004` |

- 管理界面把 `ICache` / `ISharedCache` 转成 `ICacheInspector` 得到对象条目，再把它的 `Store` 转成 `ICacheStoreInspector` 得到字节条目，两份合起来是这个缓存的全部条目，来源即条目所在的层；同一个键可以两边都有。
- 删单个对象条目用 `ICache.RemoveAsync`，读值用 `ICache.GetAsync<object>`。
- `InMemoryObjectCache`（`caching.local` 与内存实现的 `caching.distributed`）实现它；序列化缓存（Redis 等）的条目都在字节存储里，不实现。
- 两级缓存的 L1 只订阅 L2 存储的变更通知：L2 是进程内对象缓存时（单实例开发环境），经本接口删除的对象条目在 L1 里最长 `LocalTimeToLive` 后才消失。

### 变更通知（可选，ADR-0055）

共享存储实现 `ICacheChangeFeed`：`Subscribe(handler)` 收到本实例与其他实例对存储的写入与删除（`CacheChange`：键列表，或按前缀删除时的前缀）。

- 本实例的变更在写入完成时同步通知，其他实例的经后端转发，不回送给发出者。
- 通知只用于失效本地副本，可能丢失（断线期间），不是可靠的消息。
- Redis 实现在每次写入与删除后，以发后不等的方式向频道 `<KeyPrefix>__changes` 发布（`XiHan:Caching:Redis:PublishChanges`，默认开）。
- Redis 实现的键前缀 `KeyPrefix`（`XiHan:Caching:Redis:KeyPrefix`，默认 `xihan:`）加在每个键与变更频道名之前，可以为空，不能有孤立的代理项（RFC-0001「孤立的代理项」），不合即启动失败：选项的初始化器与从配置读取都抛出 `ArgumentException`（2026-11-27 修订，ADR-0321；以前前缀不核对：Redis 的键按宽松的 UTF-8 编码，只差在孤立代理项上的两个前缀落到同一个键空间里，两个应用读写彼此的条目）。

## 类型化缓存

`ICache`：`GetAsync<T>`、`SetAsync<T>`、`RemoveAsync`、`GetOrCreateAsync<T>`（合并同键的并发未命中，见下），以及批量的 `GetManyAsync<T>(keys)`、`SetManyAsync<T>(entries, options)`、`RemoveManyAsync(keys)`，语义与存储契约的批量方法一致，结果与入参顺序一一对应。进程内实现直接存对象引用，不序列化；跨进程实现用 `SerializingCache` 把存储与 `ICacheSerializer` 组合。默认序列化器是 System.Text.Json，要求选项带 `TypeInfoResolver`，通过 `JsonSerializerOptions.GetTypeInfo` 取类型信息，AOT 下无反射；反射版只在允许反射的运行时可用。

`GetOrCreateAsync<T>` 在同一个缓存实例上，对同一个键、同一种结果类型的并发未命中只回源一次（2026-11-24 修订，ADR-0271；以前未命中时各自执行工厂、不合并并发，热点键过期时同时到来的调用各查一次源）：

- 先到的调用领头：工厂在它自己的上下文里、用它自己的令牌执行，其余调用等它的结局，拿到的是它的结果，过期选项也以它为准。所以同一个键必须对应同一份数据，与调用方是谁无关。
- 取消：等待的调用取消只结束自己的等待。领头的调用取消时工厂随它取消，异常只抛给它，等待的调用里先到的一个接手、执行它自己的工厂。令牌已经取消的调用未命中时不执行工厂，也不等待。
- 失败：工厂抛出而领头的调用没有取消时，当时在等的调用收到同一个异常，不缓存，下一次调用重新回源。缓存后端故障照常抛出 `CACHING_UNAVAILABLE_001`，不改为直接回源。
- 脱钩：回源期间经本实例写入或删除这个键（`Set`、`SetMany`、`Remove`、`RemoveMany`，对象缓存另有按前缀删除），在途的回源即脱钩——它照常结束，已经在等的调用照常拿到结果，但结果不写回，之后到达的调用不加入它、重新读缓存。别的实例的变更与绕过缓存直接经 `ICache.Store` 的写入看不见，不脱钩；要让失效之后开始的读取一定看到新数据并且跨实例成立，用下面的代数缓存区。
- 不合并的：不同的缓存实例之间（要跨实例只回源一次，由应用用分布式锁包住）；同一个键的不同结果类型；工厂里再读调用链上正在回源的键（否则是等自己）。这几种照没有合并时各自回源。两条执行流的工厂互相读对方的键会死锁。
- 过期选项在未命中时、回源之前校验。.NET 的在途表是 `InFlightLoadTable`（每个缓存实例一张，只存在途的键，回源结束或被取代即移除），自写 `ICache` 的实现用它得到同样的语义。

框架组件放进缓存的类型用 `CacheSerializerRegistration.AddTypeInfo(services, resolver)` 登记自己的源生成上下文（容器里一项 `CacheTypeInfoSource`，同一个解析器只登记一次）。应用登记了 `JsonSerializerOptions` 时，序列化器的解析器是应用的解析器在前、这些上下文在后，应用的上下文不必认识框架的类型；应用选项没有解析器仍是配置错误。应用自己登记 `ICacheSerializer` 时要自行处理这些类型（2026-11-14 增补，ADR-0229）。

## 代数缓存区

`GenerationCache<T>` 架在 `ICache` 上，是按范围失效、丢弃失效之前开始的读取的读穿缓存区（2026-11-14 增补，ADR-0229）。框架里会被写入改变、多实例要一致的缓存（授予、租户目录）都用它。

- 位置是（范围，条目）。范围是失效的单位，条目区分同一范围下的值，两者都是任意文本。键是 `<区>:gen`（区代数）、`<区>:gen:<范围>`（范围代数）、`<区>:entry:<范围>:<条目>`；范围与条目写成 `<长度>.<文本>`，含空白、控制字符或按 UTF-8 编码后超过 128 字节的写成 `#<SHA-256 十六进制>`（2026-11-24 修订，ADR-0276；以前是「超过 128 个字符」，按 UTF-16 码元计：键的上限改按字节计之后，区名、范围与条目各 128 个非 ASCII 字符拼出的键会超过 512 字节）。区名是合法的缓存键文本，按 UTF-8 编码后不超过 128 字节（同日写明；.NET 的 `GenerationCache<T>.MaxRegionBytes`，原名 `MaxRegionLength`、按 UTF-16 码元计，ADR-0274），这样拼出的键不会超过键的上限。
  - 散列取文本的 UTF-8 字节。有孤立代理项的范围与条目不是合法的键文本，总是写成散列形式，散列的输入按 WTF-8 取字节：成对的代理项照 UTF-8 编成 4 个字节，每个孤立的代理项照 UTF-8 三字节的写法编成 `ED A0 80`–`ED BF BF`，不换成 U+FFFD（2026-11-26 修订，ADR-0316；以前按宽松的 UTF-8 编码，孤立的代理项都换成 U+FFFD）。合法文本的 WTF-8 就是它的 UTF-8，散列与以前相同；`ED A0 80`–`ED BF BF` 不出现在合法的 UTF-8 里，所以不同的文本散列的输入总不相同。理由：宽松编码让 `a\uD800` 与 `a\uDBFF` 散列成同一个键；键的文法拒收孤立的代理项之后，这样的范围在内存缓存上也走散列形式。授予缓存以主体 ID 作范围，一个主体就会读到另一个主体的授予。
- 区与每个范围各有一个代数（随机令牌），也存在 `ICache` 里。条目存储为 `GenerationCacheEntry<T>`（区代数、范围代数、值），读取时两个代数都等于当前值才算命中。
- `GetOrLoadMany(keys, load)`：结果与入参顺序一一对应，重复的位置只读一次、只查一次源，空集合不触缓存。先取代数，缺代数（从未写过、过期或被淘汰）就先写下新代数，再读条目；未命中的一次交给 `load`，结果连同读取开始时的代数写回。`load` 返回失败时原样返回、不写缓存；返回条数不符是编程错误。
- `Invalidate(scopes)` 给每个范围换新代数，一次批量写入，空集合什么也不做；`InvalidateAll()` 换区代数。失效之后开始的读取看到新代数；失效之前开始的读取写回的条目带着旧代数，任何共用这份缓存的实例都不会当作命中。
- 条目按构造时给的存活时间过期，代数活到它的 10 倍；代数丢失只会让该范围多查一次源。
- 缓存后端故障（`CACHING_UNAVAILABLE_001`）时，读取直接查源、不写回，结果仍然正确，每个区每 10 秒最多记一次警告；失效抛给调用方。
- `ICache` 是两级缓存时，代数与条目都在 L1 命中，读取不出进程；一个实例换代数，经变更通知让各实例 L1 里的旧代数失效。

## 分布式锁

租约锁：`TryAcquire(resource, ttl)` 即 `TryAdd("lock:" + resource, token, ttl)`；释放即 `TryRemove(key, token)`。不可重入；可续约：`LockHandle.TryExtend(ttl)` 即 `TryRefresh(key, token, ttl)`，仍持有才把到期时刻重设为现在加 `ttl` 并返回 `true`，已到期、被他人持有或句柄已释放返回 `false`（2026-11-13 修订，ADR-0215；以前不续约，长期持有的租约只能释放再获取，释放的那一刻别人就能拿走）。到期后他人可获取，原持有者释放返回 `false`。`TryAcquire` 被占用时只是无值；要把占用当作调用失败返回时用结果式的 `Acquire(resource, ttl)`（`DistributedLockExtensions.AcquireAsync`），被占用返回 `CACHING_CONFLICT_001`（消息键 `caching.lock.held`，参数 `resource`）。

## 幂等

Idempotency 阶段的中间件按 Baggage `xihan.idempotency-key` 去重，作用域为（租户、主体、键）：

- 作用域键 `idem:<租户或 ->:<主体>:<键>`，主体写成 `<长度>.<主体 ID>`，匿名为 `-`（2026-11-12 修订，ADR-0205；以前主体 ID 直接拼接，它能含冒号，主体 `a:b` 配键 `c` 与主体 `a` 配键 `b:c` 共用一条记录）。
- 匿名调用共用一个命名空间：没有能区分匿名调用方的身份，键就是唯一的保护，调用方要用猜不到的随机键（至少 128 位）。已知并接受，拒绝匿名的幂等调用会让公开的幂等端点不能用（ADR-0205）。
- 端点为请求里的消息另起内层调用（MCP 的 `tools/call`）时，外层请求不按幂等键去重，键交给内层调用（2026-11-12 修订，ADR-0205；以前外层先写下「处理中」，同一个键的内层调用随即撞上它）。

1. 首次调用用 `TryAdd` 写入"处理中"标记（`InProgressTimeToLive`，默认 60 秒）并执行处理器。
2. 处理器完成后用调用结果（成功，或错误码与消息）替换标记，保留 `TimeToLive`（默认 24 小时）。操作调用（上下文里有操作描述，RFC-0011）另记操作 ID；成功时把上下文里的响应对象按描述编成 XGI 的 JSON 一起记（`EncodeResponse`，已经过响应过滤，与首次调用拿到的一致）。编出的是空载荷时（插件代理交回的应答没有响应体，`plugin/invocation.md` §1）记下没有响应体，不当作 JSON 解析（2026-11-26 写明，ADR-0318；以前 .NET 把空载荷当作 JSON 解析而抛出：插件的操作已经执行，首次调用却以 `KERNEL_UNEXPECTED_001` 结束，「处理中」的标记留到过期）。
3. 处理器抛出异常时比较删标记，允许重试。
4. 重复调用：标记仍为"处理中"返回 `CACHING_CONFLICT_002`；已完成则回放、不执行处理器，并在上下文标记为回放：
   - 记录的操作 ID 必须与本次调用的一致（都不是操作调用时都为空），否则返回 `CACHING_CONFLICT_003`（消息键 `caching.idempotency.key_reused`）：同一个键被另一个操作、或在操作与非操作调用之间复用，不把别人的结果当成本次的结果。
   - 失败记录回放为同一个错误。
   - 成功的操作调用把记录的响应解码（`OperationDescriptor.DecodeResponse`）放回上下文，调用器的 XGI 入口返回同样的载荷，类型化入口返回同样的响应对象，HTTP 端点写出同样的状态码与正文（RFC-0011）。记录里既没有响应、也没有记下没有响应体，或者解码失败，返回 `CACHING_CONFLICT_003`（消息键 `caching.idempotency.response_unavailable`），不返回空载荷；记下没有响应体的按描述解码空载荷，插件代理得到同样的空载荷，与首次调用交回的相同（2026-11-26 写明，ADR-0318）。

回放的是首次调用的结果与（操作调用的）响应，不比对请求内容：同一个键配不同的请求体照样回放首次的响应，键由调用方按「一次业务意图一个键」生成。

## 两级缓存

`Caching.Hybrid` 提供 `caching.local@1`、依赖 `caching.distributed@1`：`ICache` 是 L1（进程内对象引用，有界 LRU）在前、L2（被选中的 `ISharedCache`）在后的两级缓存。

- 读：L1 命中直接返回；未命中读 L2，命中后写进 L1。批量读先在 L1 取全部，只把未命中的键一次交给 L2。
- 写与删：先 L2 再 L1；`GetOrCreate` 两级都未命中才执行工厂，结果写两级。同键并发由两级缓存自己合并（2026-11-24 修订，ADR-0271；以前 L1 未命中时把整件事交给 L2 的 `GetOrCreate`，并发调用各读各的 L2）：L1 未命中的并发调用只读一次 L2，对 L2 是一次读、一次写，不调用它的 `GetOrCreate`；本实例的写入与删除、L2 存储的变更通知都让在途的回源脱钩，脱钩的回源不写 L2、不回填 L1。
- L1 的存活时间是条目过期与 `LocalTimeToLive`（默认 30 秒）的较小者，滑动过期在 L1 里按绝对时间处理；`LocalCapacity`（默认 10000）满时淘汰最久未访问的条目。
- 跨实例失效（ADR-0055）：L2 的存储实现了 `ICacheChangeFeed` 时（Redis），本实例与其他实例对存储的写入与删除，都会让 L1 里的这些键（或前缀）立即失效；管理界面直接改存储也一样。
- 回填核对失效（2026-11-14 修订，ADR-0229；以前读 L2 途中到达的失效通知挡不住读回的旧值回填 L1，旧值留到 `LocalTimeToLive`）：L1 按键散列分 64 个条带记失效轮次，删除一个键让它所在条带的轮次加一，按前缀删除让全部条带加一。读 L2（`Get`、`GetMany`、`GetOrCreate`）之前记下轮次，回填前核对，途中有过失效就不回填，下一次读取再去 L2。写入之后照旧写进 L1。
- 没有变更通知时（远程插件等），别的实例的写入不会主动失效本实例的 L1：读到旧值的时间最长为 `LocalTimeToLive`；通知丢失时也以此为上限。需要更强一致的键应直接用 `ISharedCache`。
- `ICache.Store` 是 L2 的字节存储；L1 直接存对象引用，调用方不得修改取回的对象。

## IDistributedCache 适配

.NET 原生的 `Microsoft.Extensions.Caching.Distributed.IDistributedCache` 由 `CacheStoreDistributedCache` 架在 `ICacheStore` 上，沿用该接口的代码（会话、数据保护、第三方库）不改调用点：

- 键须满足 `CacheKey` 文法，否则抛 `ArgumentException`。
- 绝对过期（时刻或相对现在，两者都给取较早者）换算成 `TimeToLive`，已过去的时刻抛 `ArgumentOutOfRangeException`；没有绝对过期时滑动过期对应 `SlidingExpiration`；都给时绝对优先（存储不能同时表达两者）；都没有时是 `CacheEntryOptions.Default`，即存储的默认存活时间（内存与 Redis 实现缺省 5 分钟，远程插件用自己的默认），不是不过期。
- `Refresh` 是一次读取（读取即续期）；`Remove` 不存在的键不报错。
- 同步成员在适配器边界阻塞等待异步存储，调用方应优先用异步成员。
- 提供 `caching.distributed@1` 的 Provider 被选中时登记它（`AddXiHanDistributedCache`，幂等）；应用自己登记的 `IDistributedCache` 优先。

## 一致性

Provider 必须继承 `CacheStoreConformance` 并通过全部用例（含基于存储的锁与批量：`GetMany` 的顺序与未命中、`SetMany` 的覆盖与各自过期、`RemoveMany` 的逐键结果、空集合）。含多字节字符、恰好 512 字节的键完整往返：两个只差最后一个字节的最长键是两个条目，存储不截断、不折叠键，实现了 `ICacheStoreInspector` 的存储按含多字节字符的前缀列出并删除它们（2026-11-24 增补，ADR-0274）。过期用例用真实时间等待。实现了 `ICacheStoreInspector` 的存储另验证键列表的每页上限：0 与 1001 返回 `CACHING_VALIDATION_003`，1 与 1000 照常分页。

向量 `conformance/vectors/caching/cache-keys.json`（2026-11-24 增补，ADR-0276；2026-11-26 补上孤立的代理项与成对的代理项，ADR-0316）：键与键前缀必须接受与必须拒绝的样例——正好 512 字节与 513 字节的键、UTF-16 码元数不超而字节数超过的键、空白与控制字符、孤立的代理项（JSON 字符串写不出它，这几条以 `utf16` 列出码元，组字符串的写法同 RFC-0012「一致性」）与成对的代理项；判定键的类型（.NET 的 `CacheKey`）与远程 Provider 的插件一侧都回放它。

## 错误码

见 `conventions/error-codes.md` 中 `CACHING_*`。
