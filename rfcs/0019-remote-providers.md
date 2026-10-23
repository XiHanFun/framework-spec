# RFC-0019 远程 Provider：子系统能力经插件操作提供

> 状态：已采纳 | 日期：2026-10-23 | 对应 ADR：0011 | 参考实现：`XiHan.Framework.Caching.Remote`（适配器）、`framework-sdk-go` 的 `caching` 包与 `examples/cache`（插件） | 一致性：`XiHan.Framework.Caching.Conformance`、`conformance/vectors/provider/`

## 摘要

一个子系统能力（如 `caching.distributed`）可以由任意语言写的进程外插件提供：宿主侧一个 `<领域>.Remote` 适配器声明该能力并实现子系统的抽象，插件实现本规范为该能力规定的一组固定操作，二者之间只有插件协议 `xihan.plugin.v1`（RFC-0017）与 XGI 的 JSON 载荷。本 RFC 定义通用模式，并给出第一组操作集：`caching.distributed@1`（RFC-0006 的五个原子存储操作）。

## 动机

ADR-0006 把进程外插件协议定为"跨语言插件与远程 Provider 的基础"，但协议只搬运载荷，不说明一个插件怎样"成为 Provider"。没有固定的操作集，每种语言的实现都得各自猜 .NET 抽象的形状；有了它，Go、Rust 或任何语言的实现只要通过同一套一致性套件就能替换 `Caching.Redis` 这样的 .NET Provider。

## 通用模式

1. **适配器提供能力**。宿主侧的包 `<领域>.Remote`（RFC-0002 的 `kind: provider`、`role: provider`）在清单里声明 `provides`，像其他 Provider 一样被能力求解选中；插件清单不写 `provides`，只在 `contributes.operations` 列出操作集。能力的选择、依赖与注册逻辑都留在宿主，插件不感知。
2. **操作集是规范性内容**。每个能力的操作集在 `provider/<能力 ID>.md` 定义：操作 ID、请求与响应的 JSON 形态、语义、错误码。操作 ID 固定，`stability` 为 `internal`，`expose` 为空：它们是适配器与插件之间的私有契约，协议适配器不对外发布。
3. **适配器直接执行代理操作**。适配器按操作 ID 从宿主的操作登记表取代理操作描述并直接执行（.NET 为 `IOperationInvoker.ExecuteAsync`），不经过宿主管道：子系统抽象本身会在管道里被中间件使用（幂等中间件用缓存存储），走管道会递归；这些调用也不需要授权与幂等。
4. **错误映射**。插件返回的业务失败（`Status.error`）原样成为子系统错误；传输层失败（插件未加载、操作未登记、进程不可达、超时）由适配器映射为该子系统的"不可用"错误码（缓存为 `CACHING_UNAVAILABLE_001`），细节里带原错误码，不伪装成业务结果。
5. **校验在两侧**。适配器在宿主侧先按子系统规则校验输入（缓存键、过期选项），插件也必须校验：操作集的任何实现都可能被其他宿主直接调用。
6. **一致性**。适配器 + 插件的组合必须通过该子系统的一致性套件（缓存为 `CacheStoreConformance`）；`conformance/vectors/provider/` 的向量给出操作 ID 与请求 / 响应样例，供各语言实现的单元测试使用。
7. **并发与截止时间**。宿主会并发调用操作集；插件保证原子操作的原子性并自行串行化。每次调用带调用上下文的截止时间（适配器配置默认 5 秒），超时按不可用处理。

## `caching.distributed@1` 的操作集

详见 `provider/caching-distributed.md`。概要：

| 操作 | 请求 | 响应 | 语义（RFC-0006） |
| --- | --- | --- | --- |
| `cache-entries.get` | `{ "key" }` | `{ "value": base64 \| null }` | 不存在或已过期为 `null`；滑动过期在读取时延长 |
| `cache-entries.set` | `{ "key", "value", "ttlMs"?, "slidingMs"? }` | `{}` | 写入或覆盖 |
| `cache-entries.try-add` | 同 `set` | `{ "added": bool }` | 不存在（或已过期）才写入，原子 |
| `cache-entries.remove` | `{ "key" }` | `{ "removed": bool }` | 删除并报告是否存在 |
| `cache-entries.try-remove` | `{ "key", "expected" }` | `{ "removed": bool }` | 当前值等于期望值才删除，原子 |
| `cache-entries.get-many` | `{ "keys": [] }` | `{ "values": [base64 \| null] }` | 逐键同 `get`，结果与键顺序一一对应 |
| `cache-entries.set-many` | `{ "entries": [{ "key", "value", "ttlMs"?, "slidingMs"? }] }` | `{}` | 逐项同 `set`，同一键后者覆盖 |
| `cache-entries.remove-many` | `{ "keys": [] }` | `{ "removed": [bool] }` | 逐键同 `remove`，结果与键顺序一一对应 |

值与期望值是标准 base64（带填充）；`ttlMs` 与 `slidingMs` 二选一，都不给时用插件的默认存活时间；键按 RFC-0006：非空、无空白与控制字符、不超过 512 字符、区分大小写。错误：键不合法 `CACHING_VALIDATION_001`，选项不合法 `CACHING_VALIDATION_002`，后端失败 `CACHING_UNAVAILABLE_001`。

## 与 .NET 的映射（P4-07）

- `XiHan.Framework.Caching.Remote`（Ring 3）：`RemoteCachingFeature` 声明 `caching.distributed@1`（角色 provider），`RemoteCacheStore : ICacheStore` 按操作集调用代理操作，其余注册（`ISharedCache`、`IDistributedLock`、幂等中间件）与 `Caching.Redis` 相同地搭在 `ICacheStore` 上；配置节 `XiHan:Caching:Remote`（`TimeoutMilliseconds`，默认 5000）。
- 插件由 `Plugins.OutOfProcess` 按目录发现并登记代理操作；适配器在首次使用时按 ID 取描述，取不到即 `CACHING_UNAVAILABLE_001`。
- Go：`framework-sdk-go` 的 `caching` 包提供 `Store` 接口、`MemoryStore` 与 `RegisterOperations`，`examples/cache` 是示例插件。

## 错误码

沿用 `CACHING_VALIDATION_001` / `CACHING_VALIDATION_002` / `CACHING_UNAVAILABLE_001`；不新增。

## 非目标与未决

- **非目标**：插件清单里声明 `provides` 并由宿主自动生成适配器——每个能力的抽象形状不同，适配器必须按子系统手写。
- **未决**：其他子系统（对象存储、消息、调度）的操作集随各自的远程 Provider 示例补充。批量操作已随 `conventions/batch-first.md` 定下（2026-10-23）。
