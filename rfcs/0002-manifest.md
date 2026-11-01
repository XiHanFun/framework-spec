# RFC-0002 清单

> 状态：已采纳（2026-10-27 修订：健康项的实现义务；2026-11-01 修订：被顶替的节点登记与能力选择无关的服务；2026-11-01 修订：配置节可带子节；2026-11-01 修订：手写清单，`[Capability]` 生成器顺延；2026-11-01 修订：插件声明的权限码由宿主登记；2026-11-01 修订：选中区分被要求与兜底选出） | 日期：2026-10-21 | 对应 ADR：0002、0006、0030（健康项）、0075（被顶替的节点、配置节的子节、生成器顺延）、0080（插件声明的权限码）、0082（被要求的选中） | 参考实现：`XiHan.Framework.Kernel.Capabilities.CapabilityResolver`、`XiHan.Framework.Kernel.Hosting.PeriodicHealthContributor`

## 摘要

每个能力包、Provider、插件、宿主、协议适配器带一份 `xihan.manifest.json`，声明身份、提供与依赖的能力、资源绑定、贡献点、激活事件与权限。清单是装配的事实源；.NET 实现在特性的 `Manifest` 属性里手写清单，`[Capability]` 清单生成器顺延（见下文「与 .NET 的映射」）。Schema 见 `spec/manifest/manifest.schema.json`。

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
| `contributes` | 否 | 管道中间件、操作、配置节与 Schema、权限码（`contributes.permissions`，权限码文法同 RFC-0008，不重复；插件的由宿主替它登记，见 `plugin/invocation.md` §1）、健康项；配置节 `contributes.config.section` 是领域节 `XiHan:<领域>`，可再带一层或多层子节（如 `XiHan:Caching:Redis`），文法与写法见下文「配置节」 |
| `activation` | 否 | 激活事件，默认 `onStartup` |
| `permissions` | 插件 | 所需权限，宿主配置授予 |
| `entry` | 否 | 入口类型 |

## 解析规则

1. 收集：引用的程序集内嵌清单 + 插件目录中的清单文件。
2. 校验：Schema、`primitives` 与当前原语层大版本相同、`id` 唯一。
3. 求解：按能力 ID 与版本范围求解；同一能力多个候选时，`XiHan:Providers:<能力ID>` 指定 → 唯一的 `role: provider` 候选 → 唯一的 `role: default` 候选 → 否则失败并列出候选（`KERNEL_CAPABILITY_004`）。没有被任何人依赖的能力按同一规则选出提供者。
4. 选中结果回填到节点：每个节点知道自己被选中提供哪些能力（`SelectedCapabilities`），只为被选中的能力注册服务与中间件；声明了能力却一个都没被选中的节点（被顶替的 default）不装配、不启动，图里标为 `shadowed`。特性另有与能力选择无关的服务（如对象存储的命名实例 Provider，RFC-0012）时实现 `IShadowAwareFeature`：被顶替时内核在同一位置改调它的 `ConfigureShadowed`，只登记那些服务——不注册能力契约，不贡献中间件、操作与权限码，不校验必需绑定，也不启动。节点还知道每项选中是不是被要求的（`IsRequestedFor`，2026-11-01 修订，ADR-0082）：有清单依赖这项能力（必需或可选，且解析到了提供者），或 `XiHan:Providers:<能力ID>` / 代码的 `UseProvider` 指定了提供者，就是被要求的；没人依赖、也没人指定，只按上一条末句选出的，是兜底选出。兜底选出照样算选中（`SelectedCapabilities`、图谱不变），特性可以据此只为被要求的能力承担跨能力的一致性约束（如 RFC-0012 缺省存储的同一存储规则）。
4. 排序：拓扑排序；循环报 `KERNEL_CAPABILITY_002`，缺失报 `KERNEL_CAPABILITY_001`，版本冲突报 `KERNEL_CAPABILITY_003`，报错必须列出冲突双方。
5. 可选依赖缺失不阻止启动，对应特性进入 `Degraded`。
6. 输出已解析图；`xihan graph` 与启动日志打印同一份。

## 配置节

`contributes.config.section` 的文法（2026-11-01 修订，ADR-0075；此前只许领域节）：

```text
Section    = "XiHan" ":" Domain *( ":" Segment )
Domain     = PascalWord        ; 术语表的领域词（Schema 只校验形式）
Segment    = PascalWord        ; 子节，层数不限
PascalWord = %x41-5A *( ALPHA / DIGIT )
```

正则 `^XiHan:[A-Z][A-Za-z0-9]*(:[A-Z][A-Za-z0-9]*)*$`。`IConfiguration` 的键不区分大小写，清单里按 PascalCase 写。

1. 一份清单只有一个配置节，声明特性在本领域里读取的配置的最近公共节：只读自己子节的写子节（`Caching.Hybrid` 写 `XiHan:Caching:Hybrid`，`Scheduling.Kubernetes` 写 `XiHan:Scheduling:Kubernetes`）；同时读领域节下的公共选项或本领域的几个子节的写领域节（`Caching.Redis` 读 `XiHan:Caching:Redis` 与 `XiHan:Caching:Idempotency`，写 `XiHan:Caching`；`Security.Jwt` 读 `XiHan:Security` 的公共选项与 `XiHan:Security:Jwt`，写 `XiHan:Security`）。
2. 读到别的领域的共享键不进清单，以那个领域的契约为准，由包 README 写明：插件宿主读 `XiHan:Kernel` 的停止超时与 `XiHan:Localization` 的缺省区域，`Security.InMemory` 读 `XiHan:MultiTenancy:TenantClaim`。资源绑定（`XiHan:Bindings`）在 `bindings` 里声明，也不进配置节。
3. 插件宿主在 `Configure` 时把插件清单声明的整节下发给插件（`spec/plugin/lifecycle.md`）：声明子节的插件只收到子节。

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

现状（5.0，2026-11-01 修订，ADR-0075）：每个特性在 `IFeature.Manifest` 属性里手写清单对象（原语层 `Manifest` 及其组成类型），内核在构建期以 `Manifest.Validate()` 校验格式；插件的清单是插件目录里的 `xihan.manifest.json` 文件，由内核 `ManifestReader` 读取。

```csharp
public sealed class EntityFrameworkCoreFeature : IHostedFeature
{
    public Manifest Manifest { get; } = new()
    {
        Id = "xihan.persistence.efcore",
        Version = FrameworkInfo.Version,
        Primitives = 1,
        Stability = Stability.Preview,
        Kind = ManifestKind.Provider,
        Provides = [new CapabilityDeclaration("persistence.relational", 1)],
        Requires = [new CapabilityRequirement("kernel.hosting", 1)],
        Contributes = new Contributions { Config = new ConfigContribution("XiHan:Persistence:EntityFrameworkCore") },
    };
    …
}
```

`[Capability]` 清单生成器顺延：原计划由源生成器从入口类型上的 `[Capability(...)]` 产出 `Manifest` 属性并把 `xihan.manifest.json` 作为嵌入资源写入程序集，`bindings`、`contributes` 由同一特性的可选参数或伴随特性声明。内核保留 `CapabilityAttribute` 类型（运行期不读取），5.0 没有生成器；实现生成器时手写的清单是它的一致性基准，生成结果必须与手写清单逐字段相同。

## 一致性向量

`spec/conformance/vectors/manifest/*.json`：合法清单、每种非法清单的期望错误、求解用例（缺失、循环、冲突、多候选、默认角色、可选依赖）。
