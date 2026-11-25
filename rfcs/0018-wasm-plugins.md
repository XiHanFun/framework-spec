# RFC-0018 Wasm 插件 ABI（Experimental）

> 状态：已采纳（2026-11-01 修订：卸载按归属撤回代理操作与权限码，可直接再加载；同日再修订：贡献的中间件随运行期加载成组登记、随卸载按归属撤回，运行期可以加载贡献中间件的插件；同日第三次修订：回调的超时归因在不再等待与回调已返回两条路径同一判断，运行时不再按时钟改写插件写下的取消；2026-11-04 修订：崩溃细节只写陷阱种类；2026-11-12 修订：中间件不能改写 `xihan.tenant`，回调与发布只能带当前调用的租户；2026-11-24 修订：本 RFC 独有的规则并入 `plugin/wasm.md`，这里改为引用，规则不变；2026-11-26 写明：宿主严格按 UTF-8 解码客体写来的消息） | 日期：2026-10-23 | 对应 ADR：0006（范围）、0009（ABI 形态）、0085（卸载撤回登记）、0088（中间件撤回）、0092（超时归因）、0115、0195（租户）、0276（独有的规则并入 `wasm.md`）、0312（客体消息的严格 UTF-8） | 参考实现：`XiHan.Framework.Plugins.Wasm` | 规范性文本：`spec/plugin/wasm.md`、`spec/plugin/plugin.proto`（中间件消息）、`spec/conformance/vectors/plugin/wasm-exports.json` | 一致性档：`wasm`

## 摘要

Wasm 插件是 `xihan.plugin.v1` 的第二种载体：**同一套消息（`plugin.proto`），调用约定从 gRPC 换成导出函数 + 线性内存**。插件是一个 WASI reactor 形态的核心 Wasm 模块，宿主用 wasmtime 在进程内加载，以沙箱（内存上限、CPU 截止、无环境权限）运行不可信代码；与进程外插件相比，它多了一种贡献——在管道阶段插入中间件（Envoy 过滤器模式）。当前以 Experimental 发布：ABI 只增不改，但不承诺同一大版本内不升版本。

## 范围

- 隔离级别 `wasm`（RFC-0002 清单的 `isolation`）；`entry` 指向插件目录里的 `.wasm` 文件（.NET 参考实现同时接受 `.wat` 文本，便于示例与测试，文本格式不是规范性内容）。
- 能贡献什么（操作、事件处理器与中间件）、不能贡献什么见 `wasm.md` 开头。
- 任意能编译到 `wasm32-wasip1` 的语言：Rust、Go（TinyGo）、C / C++、AssemblyScript、.NET（NativeAOT-LLVM）等；只需 JSON 编解码，不需要 protobuf 运行时。

## 设计

载体的逐条规则在规范性文本 `plugin/wasm.md`，§1–§6 的节号与本节相同；中间件的消息在 `plugin/plugin.proto`。本节每一小节只留要点、取舍的理由与参考实现的选项名、默认值，规则都在 `wasm.md`（2026-11-24 修订，ADR-0276：原先只写在本节的几条规则并入 `wasm.md`，这里改为引用）。同一条规则两处都写到时以 `wasm.md` 为准。

### 1. 模块形态

WASI preview 1 的 reactor 形态核心模块，导出线性内存 `memory`；一个插件一个模块、一个长期存活的实例（`wasm.md` §1，实例见 §5）。

- 模块能用哪些 WebAssembly 提案见 `wasm.md` §1。
- 宿主每次加载时编译，不缓存编译结果（见「非目标与未决」）。

### 2. 编码与内存约定

消息按 proto3 JSON 映射（canonical JSON）编码为 UTF-8 文本，经 `xihan_alloc` / `xihan_free` 与线性内存交换；导出函数以一个 `i64` 给出响应的偏移与长度，返回 `0` 表示给不出响应，按崩溃处理（`wasm.md` §2，崩溃见 §5）。

- 选择 JSON 而非 protobuf 二进制的理由：几乎所有 Wasm 工具链自带 JSON，而 protobuf 运行时在 Wasm 里体积大、语言覆盖不全；载荷本身（`payload`、`data`）已经是 JSON。二进制编码留给 ABI v2 的握手协商。

### 3. 导出（插件实现）

导出表（名字、签名、何时必需、对应的 `plugin.proto` 消息）与加载、握手时的拒绝规则见 `wasm.md` §3；向量 `conformance/vectors/plugin/wasm-exports.json` 给出同一张表。

- `WatchHealth` 与 `InvokeStream` 没有 Wasm 对应：健康只轮询，流式操作不支持。
- 握手请求的 `hostEndpoint` 为空：回调不走网络，走 §4 的导入函数。
- `xihan_middleware` 的消息是为本载体在 `plugin.proto` 里新增的（§6）。

### 4. 导入（宿主提供，模块名 `xihan`）

宿主提供 `log`、`invoke`、`publish` 三个导入函数与 WASI preview 1。导入表，回调的权限、租户、截止时间与取消、嵌套深度、再入，以及 WASI、文件系统与网络的范围见 `wasm.md` §4。

- `invoke` 与 `publish` 的语义同 `Host.Invoke`、`Host.Publish`（`invocation.md` §5）：以插件身份经宿主完整管道。权限不足时不陷阱、以 `Status.error` 返回，与 gRPC 回调的 `PermissionDenied` 对应。
- 再入：wasmtime 允许导入函数内再调导出，所以宿主管道再次进入同一插件（它自己贡献的中间件、它自己的操作或订阅）时，在同一执行线程上、导入函数返回前同步完成；嵌套深度怎样计见 `wasm.md` §4。
- 环境变量里没有会话令牌与宿主地址，它们在 Wasm 里没有意义。
- 文件系统的宿主侧真实目录由宿主配置 `XiHan:Plugins:Wasm:Mounts:<清单 ID>:<客体路径>` 映射（§7）。
- `net.outbound:` 权限宿主永不授予，清单声明了它的结局见 `wasm.md` §4。

### 5. 实例、沙箱与生命周期

每插件一个实例、调用串行，内存、栈与 CPU 都有上限；状态机、崩溃后的退避重建与激活沿用 `lifecycle.md`（`wasm.md` §5）。

- **实例**：每插件一个 `Store` + 一个实例。调用串行是因为 Wasm 线性内存不是线程安全的；多实例池是 v2 的宿主优化，不改变 ABI。
- **内存**：参考实现的选项是 `MemoryLimitBytes`（默认 64 MiB，线性内存上限）、`StackLimitBytes`（默认 1 MiB，Wasm 栈上限）与 `MaxTableElements`（默认 100000，每个表的元素数上限），宿主把超限的陷阱记为崩溃。超限的表现见 `wasm.md` §5，`xihan.log` 单条日志的上限见 `wasm.md` §4（2026-11-12 修订，ADR-0186：以前只限线性内存，表与单条日志都没有上限）。
- **CPU**：宿主启用 epoch 中断，按 `EpochIntervalMilliseconds`（默认 10）递增 epoch，每次调用的截止时间换算成 epoch 配额，到达即陷阱；陷阱后不能保证客体状态一致，所以实例随后重建。epoch 只在执行 Wasm 代码时检查，WASI 里唯一能无限阻塞的 `poll_oneoff`（睡眠）由宿主实现，最多睡到截止时间，到了即把 epoch 截止设为当下，客体一回到 Wasm 代码就被中断（2026-11-12 修订，ADR-0186：以前睡眠中的调用中断不了，宿主要等它睡完）。生命周期调用的时限是 `CallTimeoutSeconds`（默认 30），中间件的时限是 `MiddlewareTimeoutMilliseconds`（默认 100），操作与事件没有截止时间时取 `CallTimeoutSeconds`。
- **健康**：轮询的规则见 `wasm.md` §5；参考实现的间隔是 `HealthIntervalSeconds`，阈值是 `HealthFailures`。
- **崩溃**：什么算陷阱、错误细节 `trap` 写什么见 `wasm.md` §5（2026-11-04，ADR-0115）。重建 = 重新实例化同一模块并重走握手、配置、启动。
- **激活**：Wasm 实例化在毫秒级，懒激活成本远低于子进程，所以 `onOperation:` / `onEvent:` 的插件在加载时实例化、握手取得声明后就丢弃实例。
- **宿主进程**：wasmtime 处理陷阱时改写线程上下文，与 Windows 用户态影子栈（Intel CET）冲突：.NET 9 起 apphost 默认 CET 兼容，影子栈强制时第一次陷阱就让进程 fail-fast（`0xC0000409`），到不了宿主的崩溃处理。参考实现在创建引擎前查询进程的影子栈策略，强制（启用且非审计）时以 `PLUGINS_UNAVAILABLE_003` 抛出配置错误拒绝创建宿主，而不是运行到第一次陷阱才崩溃。Linux / macOS 不受影响。
- 每个插件的健康条目 `plugins.<清单 ID>` 经内核 `IHealthContributor` 给出，语义同进程外插件。

### 6. 中间件贡献

清单 `contributes.pipeline[]` 的每一项在宿主管道的对应阶段插入一个中间件，每次调用宿主在进入下游前后各调用一次 `xihan_middleware`。消息 `MiddlewareRequest`、`MiddlewareResponse` 与 `MiddlewareAction` 在 `plugin.proto`（只有 wasm 隔离使用）；两个阶段的语义、中间件的命名与次序约束、失败即关闭、按归属登记与撤回见 `wasm.md` §6。

- 短路必须给出明确的失败，所以 `SHORT_CIRCUIT` 的 `outcome` 不是失败或缺失时按 `KERNEL_VALIDATION_001` 处理。
- 中间件看不到请求与响应载荷是 v1 的范围：只有元数据、操作 ID 与结果。需要载荷的逻辑写成操作而不是中间件。
- 失败即关闭的理由：安全相关阶段不能失败即放行。中间件失败后宿主 `Degraded` 直到插件重建成功；是否允许按贡献项配置「失败即放行」列入未决。

### 7. 发现与宿主配置

- 插件目录含 `xihan.manifest.json` 与 `.wasm`；宿主按 `XiHan:Plugins:Wasm:Directories` 发现，`Grants:<清单 ID>` 授予权限，`Mounts:<清单 ID>:<客体路径>` 映射文件系统；握手应答的核对见 `wasm.md` §3。
- 发现在配置阶段同步完成（与进程外插件一致），代理操作连同清单声明的权限码（`plugin/invocation.md` §1）、以清单 ID 为归属的消息目录（RFC-0010「归属目录」）、订阅与中间件据此登记。运行期 `LoadAsync` 也可以加载贡献中间件的插件：中间件经 `PipelineBuilder.UseOwned` 与代理操作、权限码一次成组登记、立即核对次序约束，全进或全不进（RFC-0004「运行期改动」，ADR-0088）。
- 卸载（`UnloadAsync`）按归属撤回插件贡献的中间件、登记的代理操作与权限码，之后可直接再加载（`plugin/invocation.md` §1，ADR-0085、ADR-0088）：撤回之后开始的调用不再经过它的中间件；撤回之前已进入管道的调用按进入时的管道走完，插件停止后它的钩子失败即关闭；别的中间件以次序约束引用了它的中间件时卸载被拒（`KERNEL_PIPELINE_003`）（`plugin/wasm.md` §6）。

## 错误码

见 `conventions/error-codes.md` 中 `PLUGINS_*`，含义以登记表为准，各码在什么情形产生见 `plugin/wasm.md`。

- 本载体新增：`PLUGINS_VALIDATION_005`（模块不合法）、`PLUGINS_UNAVAILABLE_002`（沙箱资源不足）、`PLUGINS_UNAVAILABLE_003`（宿主进程强制影子栈；配置错误，以 `FrameworkException` 抛出，不本地化）。
- 沿用：陷阱与 `0` 返回是 `PLUGINS_UNEXPECTED_002`，超时是 `KERNEL_TIMEOUT_001`，权限是 `PLUGINS_DENIED_001` / `PLUGINS_DENIED_002`。

## 一致性

- 向量 `conformance/vectors/plugin/wasm-exports.json`：必需 / 可选导出的名字与签名、导入表、ABI 版本。
- 套件 `WasmPluginConformance`（`Plugins.Conformance` 新增抽象类）：以模块文件为输入，核对导出表、`xihan_abi_version`、握手一致性、生命周期、未知操作返回 `KERNEL_NOTFOUND_001` 失败状态（Wasm 没有 gRPC `Unimplemented`，统一为 `Status.error`）、声明的操作都返回 `status`、中间件两阶段调用与短路、陷阱后可重建。
- 参考客体：`samples/EchoWasmPlugin`，Rust（`wasm32-wasip1`，cdylib）实现 `echoes.create` / `echoes.relay` / `echoes.trap` / `echoes.spin` / `events.list`、订阅 `echo.events` 与一个 `observe` 阶段的中间件；构建产物 `plugin/echo.wasm` 随仓库提交，测试与一致性套件直接加载它，仓库 CI 不要求 Rust 工具链。

## 与 .NET 的映射

- 运行时：NuGet `Wasmtime`（wasmtime-dotnet，含各平台原生库；版本见 XiHan.Framework 的 `framework/Directory.Packages.props`）提供核心模块、`Module.FromText`、WASI 配置（参数、环境、stdio、预开放目录）、`Store.SetLimits`（内存、表、实例上限）、epoch 中断（`Config.WithEpochInterruption` + `Store.SetEpochDeadline`）与燃料；它没有组件模型的宿主侧绑定 API，这是 ABI v1 选核心模块的直接原因。
- 包拆分：把进程外插件宿主里与载体无关的部分从 `Plugins.OutOfProcess` 移到新包 `XiHan.Framework.Plugins.Protocol`（Ring 3）——从 `plugin.proto` 生成的消息类型（internal）、`PluginEndpoint`、`PluginProtocol`、`PluginErrors` / `PluginMessages` 与 10 种区域文案、`HandshakeValidator`、`PluginMetadata` / `PluginErrorMapper` / `PluginEnvelopeMapper` / `PluginJson`、`ProxyOperationDescriptor`、`PluginSubscription` / `ProxyOperation` / `HandshakeInfo`；`Plugins.OutOfProcess`、`Plugins.Wasm`、`Plugins.Conformance` 都依赖它（internal 部分经 `InternalsVisibleTo`）。包 ID 文法改为 `"Plugins." ( Isolation | "Protocol" | "Conformance" )`，`Plugins.Protocol` 在 Ring 3。
- `Plugins.Wasm`（Ring 5，`IsAotCompatible=false`）：`WasmPluginsFeature`（能力 `plugins.wasm@1`，配置 `XiHan:Plugins:Wasm`）、`WasmPluginHost : IPluginHost, IHealthContributor`、`WasmPluginRuntime`（模块、实例、串行调用队列、epoch 计时器、健康轮询、重建退避）、`WasmHostFunctions`（`xihan.log` / `invoke` / `publish` 与权限判定）、`WasmMiddleware : IPipelineMiddleware`（每个贡献项一个实例，`InvokeAsync` 做 before → next → after），`WasmPluginOptions`。
- 错误码常量与文案加进 `Plugins.Protocol` 的 `PluginErrors` / `PluginMessages`。

## 非目标与未决

- **非目标（v1）**：组件模型 / WIT（待 wasmtime-dotnet 提供宿主侧组件绑定后以 ABI v2 引入，届时 `xihan_abi_version` 返回 2 并以 WIT 世界 `xihan:plugin` 替代导出表）；流式操作；多实例并发；`net.outbound`（WASI 套接字属 preview 2）；中间件访问或改写载荷；热重载；燃料计量（epoch 足够）；编译缓存（宿主每次加载都重新编译模块，不提供 `CacheDirectory` 之类的落盘缓存；原稿 §7 的 `CacheDirectory` 从未实现，已删去，需要时另起 RFC，连同失效策略一并规定）。
- **未决**：中间件是否允许按贡献项配置失败即放行；是否允许插件声明所需的最小内存上限（清单字段）；`.wat` 是否进入规范（倾向不进入）。
