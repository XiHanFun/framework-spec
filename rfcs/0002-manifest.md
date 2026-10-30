# RFC-0002 清单

> 状态：已采纳（2026-10-27 修订：健康项的实现义务） | 日期：2026-10-21 | 对应 ADR：0002、0006、0030（健康项） | 参考实现：`XiHan.Framework.Kernel.Capabilities.CapabilityResolver`、`XiHan.Framework.Kernel.Hosting.PeriodicHealthContributor`

## 摘要

每个能力包、Provider、插件、宿主、协议适配器带一份 `xihan.manifest.json`，声明身份、提供与依赖的能力、资源绑定、贡献点、激活事件与权限。清单是装配的事实源；.NET 实现由源生成器从 `[Capability]` 生成并嵌入程序集。Schema 见 `spec/manifest/manifest.schema.json`。

## 字段

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `id` | 是 | `xihan.<领域>.<名称>` |
| `package` | 否 | 所属包 ID |
| `version` | 是 | SemVer |
| `primitives` | 是 | 依赖的原语层大版本 |
| `stability` | 是 | 五级稳定性 |
| `kind` | 是 | `capability` / `provider` / `plugin` / `host` / `protocol` |
| `role` | 否 | `provider`（默认）或 `default`：无显式 Provider 时被选中 |
| `isolation` | 插件必填 | `in-process` / `wasm` / `out-of-process` |
| `provides` | 否 | 提供的能力及接口版本 |
| `requires` | 否 | 依赖的能力、版本范围、是否可选 |
| `bindings` | 否 | 需要宿主注入的资源 |
| `contributes` | 否 | 管道中间件、操作、配置节与 Schema、权限码、健康项 |
| `activation` | 否 | 激活事件，默认 `onStartup` |
| `permissions` | 插件 | 所需权限，宿主配置授予 |
| `entry` | 否 | 入口类型 |

## 解析规则

1. 收集：引用的程序集内嵌清单 + 插件目录中的清单文件。
2. 校验：Schema、`primitives` 与当前原语层大版本相同、`id` 唯一。
3. 求解：按能力 ID 与版本范围求解；同一能力多个候选时，`XiHan:Providers:<能力ID>` 指定 → 唯一的 `role: provider` 候选 → 唯一的 `role: default` 候选 → 否则失败并列出候选（`KERNEL_CAPABILITY_004`）。没有被任何人依赖的能力按同一规则选出提供者。
4. 选中结果回填到节点：每个节点知道自己被选中提供哪些能力（`SelectedCapabilities`），只为被选中的能力注册服务与中间件；声明了能力却一个都没被选中的节点（被顶替的 default）不装配、不启动，图里标为 `shadowed`。
4. 排序：拓扑排序；循环报 `KERNEL_CAPABILITY_002`，缺失报 `KERNEL_CAPABILITY_001`，版本冲突报 `KERNEL_CAPABILITY_003`，报错必须列出冲突双方。
5. 可选依赖缺失不阻止启动，对应特性进入 `Degraded`。
6. 输出已解析图；`xihan graph` 与启动日志打印同一份。

## 健康项

`contributes.health` 声明的每个条目名（`^[a-z][a-z0-9.-]*$`，约定 `<领域>.<机制>.<对象>`，如 `caching.redis.connection`、`persistence.inmemory.capacity`）必须由同一个包以内核的 `IHealthContributor` 实现并登记，反之登记了 `IHealthContributor` 的包必须声明条目（插件宿主例外：它们的条目名来自各插件自己的清单）；参考实现的架构测试核对两者一致。条目进入宿主健康快照的 `contributions`，Degraded / Failed 只把应用从 Ready 压成 Degraded，不退出。

- 连接类 Provider（数据库、缓存、消息代理、对象存储等外部服务）用内核的 `PeriodicHealthContributor(name, probe, interval)` 周期探测：启动时同步探一次，之后按 `HealthProbeIntervalSeconds`（各 Provider 配置节，默认 15）重探；探针失败或超时记 `KERNEL_UNAVAILABLE_005`（`name`、`reason`）并报 Degraded，恢复后报 Ready；探测不经管道、不受应用服务状态影响。
- 容量类 Provider（内存实现的存储、队列）同步上报：达到容量即 Degraded，错误码取所属领域的 UNAVAILABLE 容量码（如 `CACHING_UNAVAILABLE_002`、`OBJECTSTORAGE_UNAVAILABLE_002`）。
- 条目详情（ADR-0054）：`ContributorStatus` 除名字、状态与错误外可带：
  - `Description`：检查的是什么。
  - `CheckedAt`：检查时刻。
  - `Duration`：检查耗时。
  - `Data`：名字到标量的字典（字符串、布尔、数值或空），不是标量即 `ArgumentException`。

  周期探测的条目由 `PeriodicHealthContributor` 记下每次探测完成的时刻与耗时，说明由构造参数给出；容量类条目在 `Data` 里给出已用量与上限。宿主探针端点的 `contributions` 照写（`description`、`checkedAt`、`durationMs`、`data`，没有的省略）。
- 自带刷新循环的 Provider（如数据库翻译加载器）按最近一次刷新的结果上报，不另起探测循环。

## 与 .NET 的映射

```csharp
[Capability("persistence.efcore",
    Provides = ["persistence.relational@1"],
    Requires = ["kernel.hosting@^1", "multitenancy.context@^1?"],
    Role = CapabilityRole.Provider)]
public sealed class EntityFrameworkCoreFeature : IFeature, IHostedFeature { … }
```

生成器从特性产出清单并作为嵌入资源 `xihan.manifest.json` 写入程序集；`bindings`、`contributes` 由同一特性的可选参数或伴随特性声明。

## 一致性向量

`spec/conformance/vectors/manifest/*.json`：合法清单、每种非法清单的期望错误、求解用例（缺失、循环、冲突、多候选、默认角色、可选依赖）。
