# RFC-0019 远程 Provider：子系统能力经插件操作提供

> 状态：已采纳 | 日期：2026-10-23 | 对应 ADR：0011 | 参考实现：`XiHan.Framework.Caching.Remote`（适配器）、`framework-sdk-go` 的 `caching` 包与 `examples/cache`（插件） | 一致性：`XiHan.Framework.Caching.Conformance`、`conformance/vectors/provider/` | 一致性档：`provider:caching.distributed@1`

## 摘要

一个子系统能力（如 `caching.distributed`）可以由任意语言写的进程外插件提供：宿主侧一个 `<领域>.Remote` 适配器声明该能力并实现子系统的抽象，插件实现本规范为该能力规定的一组固定操作，二者之间只有插件协议 `xihan.plugin.v1`（RFC-0017）与 XGI 的 JSON 载荷。本 RFC 定义通用模式，并给出第一组操作集：`caching.distributed@1`（与 RFC-0006 的存储契约一一对应）。

## 动机

ADR-0006 把进程外插件协议定为"跨语言插件与远程 Provider 的基础"，但协议只搬运载荷，不说明一个插件怎样"成为 Provider"。没有固定的操作集，每种语言的实现都得各自猜 .NET 抽象的形状；有了它，Go、Rust 或任何语言的实现只要通过同一套一致性套件就能替换 `Caching.Redis` 这样的 .NET Provider。

## 通用模式

规则以规范性文本 `provider/README.md`「规则」为准，下面各条括号里是那里的条号；这里只留每一条的要点与理由。

1. **适配器提供能力**（规则 1）。宿主侧的包 `<领域>.Remote` 在清单里声明 `provides`，像其他 Provider 一样被能力求解选中。能力的选择、依赖与注册逻辑都留在宿主，插件不感知。
2. **操作集是规范性内容**（规则 2）。每个能力的操作集在 `provider/` 下各有一个文件，定义操作 ID、请求与响应的 JSON 形态、语义与错误码；文件名取操作集 ID 去掉 `@` 与大版本、点换成连字符（`caching.distributed@1` 是 `provider/caching-distributed.md`）。操作是适配器与插件之间的私有契约，所以不对外发布。
3. **适配器直接执行代理操作**（规则 3；.NET 为 `IOperationInvoker.ExecuteAsync`）。不经过宿主管道的理由：子系统抽象本身会在管道里被中间件使用（幂等中间件用缓存存储），走管道会递归；这些调用也不需要授权与幂等。
4. **错误映射**（规则 4）。传输层失败映射为该子系统的「不可用」错误码（缓存为 `CACHING_UNAVAILABLE_001`），不伪装成业务结果。
5. **校验在两侧**（规则 5）。插件也必须校验的理由：操作集的任何实现都可能被其他宿主直接调用。
6. **一致性**（规则 7）。缓存的套件是 `CacheStoreConformance`；`conformance/vectors/provider/` 的向量给出操作 ID 与请求 / 响应样例，供各语言实现的单元测试使用。
7. **并发与截止时间**（规则 6）。插件自行串行化来保证原子操作的原子性。截止时间取调用上下文的（适配器配置默认 5 秒），超时按不可用处理。

## `caching.distributed@1` 的操作集

第一组操作集，操作、字段、语义与错误以规范性文本 `provider/caching-distributed.md` 为准：资源名 `cache-entries`，操作与 RFC-0006 的存储契约一一对应（单条的读、写、不存在才写、删除、按期望值删除、按期望值续期，以及读、写、删除的批量形式），值是 base64 编码的字节。其中 `try-refresh` 是 2026-11-13 增补的（ADR-0215），锁续约用它。

## 与 .NET 的映射

- `XiHan.Framework.Caching.Remote`（Ring 3）：`RemoteCachingFeature` 声明 `caching.distributed@1`（角色 provider），`RemoteCacheStore : ICacheStore` 按操作集调用代理操作，按接口登记为 `ICacheStore`，其余注册（`ISharedCache`、`IDistributedLock`、幂等中间件）与 `Caching.Redis` 相同地搭在它上面；配置节 `XiHan:Caching:Remote`（`TimeoutMilliseconds`，正整数，默认 5000；配置了但不是正整数时装配失败，不退回默认值）。
- 插件由 `Plugins.OutOfProcess` 按目录发现并登记代理操作；适配器在首次使用时按 ID 取描述，取不到即 `CACHING_UNAVAILABLE_001`。
- Go：`framework-sdk-go` 的 `caching` 包提供 `Store` 接口、`MemoryStore` 与 `RegisterOperations`，`examples/cache` 是示例插件。

## 错误码

沿用 `CACHING_VALIDATION_001` / `CACHING_VALIDATION_002` / `CACHING_UNAVAILABLE_001`；不新增。

## 非目标与未决

- **非目标**：插件清单里声明 `provides` 并由宿主自动生成适配器——每个能力的抽象形状不同，适配器必须按子系统手写。
- **未决**：其他子系统（对象存储、消息、调度）的操作集随各自的远程 Provider 示例补充。批量操作已随 `conventions/batch-first.md` 定下（2026-10-23）。
