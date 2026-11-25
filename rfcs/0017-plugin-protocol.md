# RFC-0017 进程外插件协议

> 状态：已采纳（2026-11-01 修订：卸载按归属撤回代理操作与权限码，可直接再加载；同日再修订：清单写了 `contributes.pipeline` 的插件在发现时被拒，不再静默忽略；同日第三次修订：gRPC `Cancelled` 不再按时钟改报超时，交给截止时间中间件归因；2026-11-12 修订：回调与发布只能带宿主随调用交给插件的租户；2026-11-13 修订：`HandleEventRequest` 带这次投递的订阅方式；2026-11-18 修订：宿主提供多个回调端点并至少有一个回环 TCP，插件一侧的次序、配置、就绪、停止、收尾、投递、回调与错误读写写明，运行时消息键进规范，`Host.Publish` 按元数据建上下文；2026-11-24 修订：`Configure` 的 `bindings` 在组装请求时与内核同源解析、只带主值；同日再修订：Span `xihan.plugin.invoke` 的种类、父级、标签、状态与范围，`trace_parent` 以它为父级；同日第三次修订：`Binding` 消息新增 `properties`，绑定的属性随 `Configure` 下发；2026-11-25 修订：下发之前核对来源的类型与清单的必需，`Configure` 请求有总上限，绑定目录的文件有大小上限） | 日期：2026-10-23 | 对应 ADR：0006、0085、0092、0195、0226、0237、0268、0269、0285、0292 | 参考实现：`XiHan.Framework.Plugins.OutOfProcess`；一致性：`XiHan.Framework.Plugins.Conformance` | 规范性文本：`spec/plugin/plugin.proto`、`spec/plugin/handshake.md`、`spec/plugin/lifecycle.md`、`spec/plugin/invocation.md` | 一致性档：`plugin`、`host`

## 摘要

进程外插件是宿主启动的子进程，以 gRPC 协议 `xihan.plugin.v1` 与宿主通信：宿主调用插件的握手、配置、启停、健康、操作调用与事件投递；插件回调宿主的操作调用与消息发布。协议只搬运 XGI 的 JSON 载荷与 RFC-0001 的错误编码，不定义业务消息，因而同时是跨语言插件与远程 Provider 的基础（ADR-0006），也是 Go SDK 的第一个真实用例（ADR-0004）。

## 范围

- 隔离级别 `out-of-process`（RFC-0002 清单的 `isolation`）；`in-process` 已由 `Plugins.InProcess` 提供，`wasm` 为 Experimental，不在本文（RFC-0018）。
- 插件可贡献操作与事件处理器；不可贡献中间件（中间件在宿主进程内，跨进程调用的延迟与失败语义不适合管道阶段）。只有 Wasm 插件能贡献中间件（RFC-0018）：清单写了 `contributes.pipeline`（非空）的进程外插件在发现时被拒，`PLUGINS_VALIDATION_001`，按 `lifecycle.md` §1 的发现时被拒处理（不启动进程、以 `Failed` 登记、是健康条目、记错误日志），宿主照常启动；运行期 `LoadAsync` 把同一错误返回给调用方、不登记。宿主不静默忽略这个字段。
- 任意语言：只需 protobuf、gRPC 与 JSON。

## 设计

### 传输与启动

子进程 + 本机 gRPC（Unix 域套接字、Windows 命名管道或回环 TCP）。启动参数经环境变量下发，插件以一行 stdout 握手行公布端点（go-plugin 模式），宿主随后连接并调用 `Handshake`。stderr 是插件日志。见 `handshake.md`。

### 握手

协商协议版本（整数，`1`）、核对原语层大版本、交换清单（JSON 文本，必须与目录里的文件一致）、操作声明（契约 + JSON Schema）、事件订阅与消息目录。清单仍是装配的唯一事实源：握手只能细化清单里声明过的东西（操作 ID 必须在 `contributes.operations`，健康项必须在 `contributes.health`）。

### 生命周期

状态词汇与 `LifecycleState` 相同；`Configure` → `Start` → `Ready`，`Stop` 带宽限期；健康以 `WatchHealth` 推送为主、`Health` 轮询为备；崩溃进入 `Failed` 并按退避重启，宿主进入 `Degraded` 但不退出；激活事件 `onStartup` / `onOperation:` / `onEvent:` 决定进程何时启动。见 `lifecycle.md`。

`Configure` 下发配置节、已授予的权限、工作目录与清单声明并获批的资源绑定。每个绑定带名字、类型、主值与它的全部属性（`Binding.properties`；2026-11-24 修订，ADR-0285，以前只带主值，只挂属性文件的绑定对插件不可用）：属性名区分大小写、原样下发，下发时属性名有文法、数量与字节数有上限，不合即配置失败；属性随绑定一起受 `binding:<绑定名>` 的授予约束，不单独授权；属性值可能是凭据，宿主与插件都不得写进日志。`properties` 是同一协议版本里的新增字段，不升协议版本：较早的插件忽略它，较早的宿主不发它。下发之前宿主按清单核对（2026-11-25 修订，ADR-0292）：来源声明的类型与清单不同、清单标为必需的绑定解析不出，即配置失败；整个 `ConfigureRequest` 按 protobuf 二进制编码不超过 2097152 字节，超过即不发。细则见 `lifecycle.md` §2 与 §7.2。

### 调用

插件操作在宿主登记为代理操作，与本地操作一样经完整管道并由协议适配器发布；Handle 阶段经 `Invoke`（一元）或 `InvokeStream`（双向帧流）执行。上下文按 RFC-0003 平铺为 `InvocationMetadata`（traceparent、截止时间、已登记 Baggage）；业务失败走 `Status.error`，gRPC 状态只表示传输失败并映射为 `PLUGINS_*` / `KERNEL_*` 错误码。事件按 RFC-0009 至少一次投递。见 `invocation.md`。

### 回调与权限

插件以会话令牌回调宿主的 `Host.Invoke` 与 `Host.Publish`，分别需要清单权限 `capability:kernel.pipeline` 与 `capability:messaging.queue`——权限必须在清单声明并被宿主授予才生效，授予多于声明的部分不放行；宿主以插件身份（主体 ID = 清单 ID，方案 `Plugin`）执行，授权仍按操作契约判定。`net.outbound:` 与 `fs.*:` 权限由宿主沙箱强制，不经协议。

## 错误码

见 `conventions/error-codes.md` 中 `PLUGINS_*`，含义以登记表为准。

- 本协议用到 `PLUGINS_VALIDATION_001`、`PLUGINS_VALIDATION_003`、`PLUGINS_VALIDATION_004`、`PLUGINS_DENIED_002`、`PLUGINS_UNAVAILABLE_001`、`PLUGINS_UNEXPECTED_001`、`PLUGINS_UNEXPECTED_002` 与 `PLUGINS_TIMEOUT_001`；各自在什么情形产生见 `handshake.md`、`lifecycle.md` 与 `invocation.md`。
- gRPC 状态与错误码的映射见 `invocation.md` §3（宿主一侧）与 §5（插件一侧的回调），运行时的消息键与参数见 `invocation.md` §8。

## 一致性

- 数据向量：`conformance/vectors/plugin/handshake-lines.json`（握手行文法）、`protocol-versions.json`（版本协商）、`host-addresses.json`（回调端点列表与选择）、`deadlines.json`（`deadline` 的读法与生效的截止时间）、`plugin-messages.json`（运行时的消息键）、`callback-status.json`（插件一侧回调失败的映射）、`error-reading.json`（读入对方写来的 `Error`）、`binding-properties.json`（下发给插件的绑定：属性名的文法与上限、宿主下发什么、插件读到什么）。
- gRPC 黑盒套件：`XiHan.Framework.Plugins.Conformance` 的 `PluginProtocolConformance`，以本 RFC 的三份规范性文本为用例来源，对任何语言的插件实现做一致性测试；远程 Provider 的套件随各子系统的 Conformance 包。

## 与 .NET 的映射

- `Plugins.OutOfProcess`（Ring 5）：`OutOfProcessPluginsFeature`（能力 `plugins.out-of-process@1`，配置 `XiHan:Plugins:OutOfProcess`）在配置阶段发现并握手、登记代理操作（`OperationDescriptor` 直通 JSON，`ExecuteAsync` 发 `Invoke`）连同清单声明的权限码（`plugin/invocation.md` §1，ADR-0080）、以清单 ID 为归属的消息目录（RFC-0010「归属目录」，ADR-0252）与事件订阅（`AddMessageHandler`）；`OutOfProcessPluginHost : IPluginHost, IHealthContributor` 负责进程管理、握手核对、健康、重启退避、回调服务器与运行期加载卸载；卸载按归属（清单 ID）撤回插件登记的代理操作、权限码与消息目录，之后可直接再加载（`plugin/invocation.md` §1，ADR-0085）。宿主回调端点在 Windows 上是命名管道加回环 TCP、其他系统是 Unix 域套接字加回环 TCP（`handshake.md` §4，ADR-0237）。进程外与 Wasm 两个宿主共用 `Plugins.Protocol` 的 `PluginConfigureRequests` 组装 `Configure` 请求：绑定经内核的 `BindingResolver` 解析（ADR-0268），`BindingValue.Properties` 原样写进 `Binding.properties`，文法与上限由 `PluginBindingProperties` 核对（ADR-0285）；必需与类型由内核的 `PluginBindingRules` 核对（进程内宿主加载插件之前用同一处），整个请求的大小在组装之后核对，绑定目录的文件大小由内核的 `BindingResolver` 在读的时候限制（ADR-0292）。
- 内核新增 `IHealthContributor` / `ContributorStatus`，`HealthSnapshot.Contributions` 与 `App.State` 采集它们：条目 Degraded / Failed 只把 Ready 压成 Degraded（`lifecycle.md` §4 的"宿主进入 Degraded 但不退出"由此实现）。
- 协议消息的 .NET 类型由 `Grpc.Tools` 从同一份 `.proto` 生成（运行时 `Google.Protobuf`，Ring 5 允许第三方依赖）；`Protocols.Grpc` 的动态编解码只服务于没有编译期 Schema 的操作消息，不用于本协议。其他语言用各自的 protobuf 工具链从同一份 `.proto` 生成。
- 错误码常量 `PluginErrors` 与消息键 `PluginMessages` 随包实现，文案 10 种区域各一个文件（RFC-0010）。

## 非目标

- 流式操作在 .NET 侧的契约类型（`IOperation` 的流式形态）另起 RFC；本协议先定义帧格式。
- 插件热重载、空闲停止、插件市场、资源配额（CPU / 内存）由宿主沙箱与后续 RFC 处理。
- Wasm 插件的 ABI 不在本文，见 RFC-0018 与 `plugin/wasm.md`。
