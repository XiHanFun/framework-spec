# 插件生命周期、健康与重启

> 规范性。协议 `xihan.plugin.v1`（`plugin.proto`，RFC-0017）。状态词汇与原语层 `LifecycleState`（RFC-0001）一致。

## 1. 状态机

| 状态 | 进入时机 |
| --- | --- |
| `Declared` | 宿主读到清单 |
| `Resolved` | 握手成功，清单与声明校验通过 |
| `Configured` | `Configure` 返回 `ok` |
| `Started` | `Start` 返回 `ok` |
| `Ready` | 首次 `Health` 或 `WatchHealth` 报 `Ready`（插件可直接在 `Start` 后报 `Ready`） |
| `Degraded` | 插件自报 `Degraded`，或任一健康项 `Degraded` / `Unhealthy` |
| `Draining` | 宿主已发 `Stop`，等待在途调用完成 |
| `Stopped` | 进程正常退出 |
| `Failed` | 发现时被拒、握手 / 配置 / 启动失败、进程异常退出、健康报 `Failed`、`Stop` 超时被终止 |

合法迁移同 `conformance/vectors/primitives/lifecycle-transitions.json`；插件自报的状态只能是 `Started`、`Ready`、`Degraded`、`Draining`、`Failed`，其余由宿主裁决；例外是 `Start` 之前的 `Health`，插件照实报 `Declared`、`Resolved` 或 `Configured`（§7.4）。插件一侧的次序、配置、就绪、停止与收尾见 §7。

**发现时被拒**：宿主在启动任何插件代码之前拒绝的插件——清单读不出、不是插件或隔离级别不符、缺 `entry`、清单写了 `contributes.pipeline`（只有 Wasm 插件能贡献中间件，RFC-0017「范围」、RFC-0018；宿主不静默忽略这个字段）（`PLUGINS_VALIDATION_001`），权限未授予（`PLUGINS_DENIED_001`），入口文件不存在（`PLUGINS_NOTFOUND_001`）——不会静默消失：宿主照样以 `Failed` 登记它（带错误、可描述），它是宿主的健康条目 `plugins.<清单 ID>`（清单读不出时没有清单 ID，条目名取 `plugins.<目录名>`），宿主记错误日志，宿主因此进入 `Degraded`（§4）。这些是配置错误，不按 §6 重试：改正后重新加载或重启宿主。按目录发现时如此；宿主在运行期显式加载（`LoadAsync`）时这些失败直接返回给调用方、不登记。

## 2. 配置与启动

1. `Configure`：`configuration` 是清单 `contributes.config.section` 节的 JSON 对象（宿主配置源经 `IConfiguration` 展开后再序列化；没有声明配置节为 `{}`）；`granted_permissions` 等于清单 `permissions`；`bindings` 只含清单声明的绑定；`work_directory` 是宿主为该插件创建的可写目录（宿主停止时不删除）。配置变更时宿主不重新调用 `Configure`，而是以 `reason: restart` 停止后重新启动进程。
   - 宿主写出的配置节（参考实现 `PluginJson.SerializeSection`，2026-11-18 写明，ADR-0237）：根总是对象；叶子是 `IConfiguration` 的值，一律写成 JSON 字符串，值为 null 的键写 `null`；子项名恰好是从 `0` 起连续整数的节写成数组，其余写成对象，成员按 `IConfiguration` 列出子项的顺序。插件怎样读见 §7.2。
   - `bindings` 的解析（2026-11-24 写明，ADR-0268；以前只写「只含清单声明的绑定」，.NET 只读配置节 `XiHan:Bindings`，只经 Service Binding 目录给出的绑定到不了插件）：宿主在每次组装 `Configure` 请求时按 RFC-0016「资源绑定与配置节」解析（`SERVICE_BINDING_ROOT` 目录与 `XiHan:Bindings:<名字>`，同名时配置节整体覆盖目录），不用宿主启动时解析的那一份，重启与运行期加载的插件拿到的是那一刻的值；应用指定了绑定根目录时用同一个。只读、只下发清单声明的绑定，按清单的顺序：`name` 是清单里的名字，`value` 是主值，`type` 是来源声明的类型、来源没有声明时取清单声明的类型，`properties` 是其余属性（下一条）。来源既没有给主值、也没有给属性的绑定不下发，宿主记一条不带值的 Warning 日志。清单声明的绑定读失败（目录在、文件读不出来）时不发 `Configure`，按这一步失败处理（`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`，`phase` 为 `configure`，`reason` 为异常类型的全名），插件进入 `Failed` 并按 §6 重试；清单没有声明的绑定读不出来不影响这个插件，根目录不存在只是没有目录来源、不是失败。
   - `bindings` 的属性（2026-11-24 修订，ADR-0285；以前 `Binding` 只有 `name`、`type`、`value` 三个字段，属性不下发，没有主值的绑定整个不下发：只挂属性文件、没有 `uri` 的绑定对隔离运行的插件不可用，带 `uri` 的绑定里另放的凭据也到不了插件）：`Binding.properties` 是解析结果里这个绑定的全部属性，与宿主进程里的特性读到的相同（RFC-0016「资源绑定与配置节」）。
     - 来源：目录来源是绑定目录里 `type` 之外、不以 `.` 开头的每个文件，文件名是属性名，内容去掉首尾空白是属性值；`uri` 文件既是主值也是属性 `uri`，两处都下发。配置来源是 `XiHan:Bindings:<名字>` 下 `type`、`value` 之外有值的子键。同名时配置节整体覆盖目录，属性不逐个合并。
     - 原样下发：属性名区分大小写，宿主不折叠大小写、不改写、不增删；属性值可以为空，空值照发（算不算给出由读它的一方照 RFC-0016「资源绑定与配置节」第 2 条判断）。来源只给了属性、没有主值的绑定照常下发，`value` 为空。`properties` 里各项的先后没有含义。
     - 属性名的文法：1 到 253 个字符，只含 ASCII 字母、数字、`.`、`_`、`-`，且不以 `.` 开头（`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,252}$`，按整串匹配）。字符集与长度同 Kubernetes Secret 的键：Service Binding 目录里的文件名就是 Secret 的键，以 `.` 开头的文件本来就不是属性。
     - 上限：一个绑定最多 64 个属性；一个绑定的全部属性名与属性值按 UTF-8 编码后的字节数合计不超过 262144（256 KiB）。主值不计入。
     - 清单声明的绑定有属性名不合文法、或超过上限时，宿主不发 `Configure`，按这一步失败处理（`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`，`phase` 为 `configure`），插件进入 `Failed` 并按 §6 重试，与读失败相同；宿主不少发、不截断，插件带着缺的凭据启动比启动失败更难查。按清单的顺序逐个绑定核对，一个绑定里先核对数量、再核对属性名、最后核对字节数，报第一处不合的。`reason` 是固定的英文文本，`<绑定名>` 是清单里的名字：属性多于 64 个为 `binding '<绑定名>' has more than 64 properties`，有属性名不合文法为 `binding '<绑定名>' has a property name that is not valid`，字节数合计超过上限为 `the properties of binding '<绑定名>' exceed 262144 bytes`。清单没有声明的绑定不核对。
     - 权限不变：插件仍然只拿得到清单声明并以 `binding:<绑定名>` 获批的绑定（下一条）。属性随绑定一起受这一条管，不单独授权，也没有按属性名的授予。
     - 属性值与主值一样可能是凭据：宿主不得把它们写进日志、错误、健康条目与其他诊断输出。上面的 `reason` 与宿主的 Warning、Error 日志只带绑定名与插件的清单 ID，不带属性名与任何值。插件一侧见 §7.2。
     - 兼容：`properties` 是同一协议版本里的新增字段（规范的 MINOR，RFC-0024「版本」第 3、4 条），不升协议版本，也不在握手里协商。按较早的规范实现的插件忽略它（protobuf 的未知字段；Wasm 载体的 JSON 映射同样忽略未知字段，`wasm.md` §2），照旧读 `name`、`type`、`value`，只是会收到以前不下发的、`value` 为空的绑定；按较早的规范实现的宿主不发它，插件读到的是空的属性表。宿主不按对端的 SDK 版本改变下发的内容。Wasm 载体把 `properties` 写成 JSON 对象（proto3 JSON 映射）。向量：`conformance/vectors/plugin/binding-properties.json`。
   - 隔离运行的插件（进程外、Wasm）读宿主配置与绑定要运营方授予（2026-11-12 修订，ADR-0177）：清单的 `contributes.config.section` 要求宿主授予 `config:<配置节>`（与清单逐字相同），`bindings` 的每一项要求授予 `binding:<绑定名>`；没有授予即在发现时以 `PLUGINS_DENIED_001` 拒绝，与清单声明而未授予的权限同一处理，不启动插件代码。配置节里常有密钥、连接串与签名密钥，插件不能自己点名拿到。进程内插件是受信代码，不受此限。
2. `Start`：返回 `ok` 后宿主才登记代理操作与事件订阅；在此之前到达的调用按 `PLUGINS_UNAVAILABLE_001` 失败。
3. 每一步都带 `StartTimeout`（默认 30 秒）的截止时间；超时为 `PLUGINS_TIMEOUT_001`，宿主终止进程。

## 3. 激活

清单 `activation` 决定进程何时启动：

| 事件 | 行为 |
| --- | --- |
| `onStartup` | 宿主启动时启动，与特性启动并行；插件失败不阻止宿主进入 `Ready`，但宿主进入 `Degraded` |
| `onOperation:<操作 ID 或 * 通配>` | 首次调用匹配的操作时启动；调用在截止时间内等待插件 `Ready`，超时为 `KERNEL_TIMEOUT_001` |
| `onEvent:<主题或 * 通配>` | 首次有匹配主题的消息要投递时启动；投递在截止时间内等待 |

模式的字符（清单 Schema 的 `activation`，RFC-0002 解析规则 2 第 3 步）：`onOperation:` 是操作 ID 的字符加 `*`（`[a-z0-9.*-]`），`onEvent:` 是主题的字符加 `*`（`[a-z0-9._*-]`）。`onEvent:` 可以带下划线（2026-11-24 修订，ADR-0276；以前不行，带下划线的主题只能订阅、不能写成激活事件）。

懒激活的插件在 `Ready` 之前，其代理操作与订阅已经登记（握手信息来自清单与上次成功握手的缓存；没有缓存时宿主在启动阶段做一次"探测启动"取得声明后立即 `Stop`）。当前不定义空闲停止。

## 4. 健康

- 宿主优先用 `WatchHealth` 订阅推送；连接断开时回退为每 `HealthInterval`（默认 10 秒）一次 `Health`，连续 `HealthFailures`（默认 3）次失败视为 `Failed`。
- 健康项名字必须在清单 `contributes.health` 里声明；宿主把每个健康项映射为宿主健康项 `plugins.<清单 ID>.<项名>`，插件整体映射为 `plugins.<清单 ID>`。
- 任一插件 `Failed` 或 `Degraded` → 宿主 `Degraded`；宿主永远不因插件而退出。`/health/ready` 的应答按宿主的就绪规则，不受懒激活插件影响。

## 5. 停止

1. 宿主调用 `Stop`（`grace_seconds` 默认等于宿主的 `XiHan:Kernel:StopTimeoutSeconds`，`reason` 为 `shutdown` / `unload` / `restart`），插件进入 `Draining`：拒绝新调用（`PLUGINS_UNAVAILABLE_001`）、完成在途调用、释放资源、返回、退出进程。
2. 宿主在宽限期内等待进程退出；超时先发终止信号：关闭插件的 stdin（`handshake.md` §1 要求插件随之退出），Unix 上同时发 SIGTERM；Windows 没有 SIGTERM，关闭 stdin 就是等价的终止信号。发出信号后再等 5 秒，仍未退出则强制终止整个进程树。两种结局都记为 `Failed`（`PLUGINS_TIMEOUT_001`）：插件没有在宽限内完成 `Stop`。插件应把 SIGTERM 与 stdin 关闭都当作立即收尾退出的请求（时限见 §7.7）。插件一侧收到 `Stop` 的做法见 §7.5。
3. 宿主停止时按与启动相反的顺序停止插件，并等待全部结束再停止特性。

## 6. 崩溃与重启

- 进程异常退出（非 `Stop` 引起）记为 `Failed`（`PLUGINS_UNEXPECTED_002`，细节含退出码与 stderr 末尾若干行），宿主进入 `Degraded`；在途调用以 `PLUGINS_UNAVAILABLE_001` 失败。
- 重启退避：首次 1 秒，之后翻倍，上限 60 秒；插件连续 `Ready` 满 10 分钟后退避归零。`RestartLimit`（默认 5，按 10 分钟窗口计）内重启失败后停止重试，保持 `Failed`，直到宿主显式 `UnloadAsync` + `LoadAsync`（卸载按归属撤回插件登记的代理操作、权限码与 Wasm 插件贡献的中间件，再加载重新登记，不用重启宿主，见 `invocation.md` §1、`wasm.md` §6）。
- 重启策略同样覆盖启动阶段的失败（`handshake.md` §5、本文 §2）：握手（读握手行、`Handshake` 调用与核对）、`Configure`、`Start` 失败都终止进程、记为 `Failed`，宿主进入服务状态后按上面的退避重试，计入 `RestartLimit`；运行期显式加载（`LoadAsync`）的同类失败同样返回给调用方，插件保持 `Failed` 并按策略重试，直到 `UnloadAsync`。发现时被拒（§1）不重试。首次握手就失败的插件重试成功后才有声明可用：宿主把它的代理操作登记进操作登记表、消息目录以归属登记进本地化器（与运行期加载一致：HTTP 路由、OpenAPI 文档与 MCP tool 随之出现，gRPC 方法、AI 工具与事件订阅可以不补登记，`invocation.md` §1）；连同清单声明的权限码一次登记（`invocation.md` §1）；登记不进去（声明的权限码与已登记的重复、与已有操作重名、引用了未登记的权限码或不存在的策略、HTTP 路由或 tool 名冲突）按发现时被拒处理，一个权限码与代理操作都不留下。懒激活插件在首次调用时启动失败：该次调用失败，下一次调用再尝试启动。
- 重启后是新的进程、新的会话令牌、重新握手；宿主校验新握手的清单与上次一致，不一致按 `PLUGINS_VALIDATION_004` 失败。
- 事件订阅在插件不可用期间由宿主的消息代理按其重试与死信策略处理，宿主不缓冲消息。

## 7. 插件一侧（SDK）

插件运行时（各语言 SDK）对宿主调用的处理，2026-11-18 写明（ADR-0237）；以前规范只写了宿主一侧，各 SDK 在这些点上各有做法。错误的消息键与参数见 `invocation.md` §8，`phase` 一律小写。

### 7.1 调用次序

| RPC | 接受的状态 | 之后 |
| --- | --- | --- |
| `Handshake` | `Declared` | `Resolved` |
| `Configure` | `Resolved` | `Configured`，失败即 `Failed`（§7.2） |
| `Start` | `Configured` | `Started`，再按 §7.3 进入 `Ready`、`Degraded` 或 `Failed` |
| `Stop` | `Declared`、`Draining`、`Stopped` 之外的任何状态 | §7.5 |
| `Health`、`WatchHealth` | 任何状态 | 不变（§7.4） |
| `Invoke`、`InvokeStream`、`HandleEvent` | 任何状态 | 不变；没有就绪时的结果见 `invocation.md` §1、§4 |

不在表里的组合以 gRPC `FailedPrecondition` 拒绝、什么都不做、状态不变：重复 `Handshake`、没握手就 `Configure`、`Configure` 两次、没 `Configure` 就 `Start`、`Start` 两次、没握手就 `Stop`、停止中再 `Stop`。会话令牌先于次序核对（`handshake.md` §4）。

### 7.2 配置

- `configuration` 为空等同 `{}`。文本必须通过 RFC-0001 的共同 JSON 读取检查（`conformance/vectors/primitives/json-text.json`）且是对象，否则不调配置回调，`ConfigureResponse.status` 为 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为 `configure`，`reason` 为 `the configuration is not a JSON object`），插件进入 `Failed`。
- SDK 把配置节原样交给作者，另提供类型化读取：成员名按 ASCII 不区分大小写匹配（与 `IConfiguration` 一致）；字符串叶子读成数字时按 JSON 数字的文法，读成布尔时只认 `true`、`false`（ASCII 不区分大小写），其余照目标类型的常规读法。
- `granted_permissions`、`bindings`、`work_directory` 原样交给作者；回调前的权限核对以 `granted_permissions` 为准（`invocation.md` §5）。
- 绑定的属性（2026-11-24 写明，ADR-0285；以前协议不带属性）：SDK 交给作者的每个绑定连同 `properties`（属性名到属性值）一起交出。属性名区分大小写，SDK 不折叠大小写、不改写；没有属性时（含按较早的规范实现的宿主不发这个字段）是空表，不是空值；值为空的属性照样在表里；各项的先后没有含义。`value` 为空表示来源没有给主值，SDK 不当成错误，也不拿属性拼出一个主值。SDK 不核对属性名的文法与上限，那是宿主的事（§2）。主值与属性值可能是凭据：SDK 不得把它们写进日志、错误与其他诊断输出；交给作者的绑定类型的调试输出（各语言的 `toString`、`Debug`、`repr` 之类）应当只带绑定名、类型与属性名，不带主值与属性值。向量 `conformance/vectors/plugin/binding-properties.json` 的 `reading`。
- 配置回调返回失败：`status` 带那个错误，插件进入 `Failed`。回调抛出：`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为 `configure`，`reason` 按 §7.6），插件进入 `Failed`。

### 7.3 启动与就绪

启动回调返回成功后插件进入 `Started`；有健康回调时立即求一次（§7.4），按结果进入 `Ready`、`Degraded` 或 `Failed`（回调报其他状态的按 `Ready`，回调抛出的按 `Degraded`），没有健康回调即进入 `Ready`；然后才应答 `StartResponse`（`ok`）。所以 `Start` 应答之后的状态是 `Ready`、`Degraded` 或 `Failed`。启动回调返回失败或抛出：规则同 §7.2（`phase` 为 `start`），插件进入 `Failed`。

只有 `Ready` 与 `Degraded` 是可服务的状态：其余状态下到达的调用与投递以 `PLUGINS_UNAVAILABLE_001` + `plugins.process.unavailable` 失败（`state` 是插件此刻的状态名，如 `Configured`、`Draining`、`Failed`）。

### 7.4 健康

- `Health` 与 `WatchHealth` 任何状态都可以调用。`Start` 之前报本地状态（`Declared`、`Resolved` 或 `Configured`），没有健康项、不调健康回调；`Started` 之后照实报，`Draining`、`Stopped`、`Failed` 不再调健康回调。
- `Ready` 与 `Degraded` 期间，`Health` 每次都求一次健康回调；健康回调报 `Ready`、`Degraded` 或 `Failed` 的照办（迁移按 `lifecycle-transitions.json`，`Failed` 之后不再回来），报其他状态的不改变状态。健康项名不在清单 `contributes.health` 里的丢掉、不发给宿主，同一个名字在进程里只记一条 `warning`。健康回调抛出：这一次的结果是 `Degraded`，`error` 为 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为 `health`），没有健康项；状态与回调报了 `Degraded` 一样迁到 `Degraded`，之后的结果照常决定状态。
- `WatchHealth` 先推一次当前状态，之后状态、健康项或 `error` 变化时推送；可服务期间 SDK 每 10 秒（宿主 `HealthInterval` 的缺省值）重新求一次健康回调。插件进入 `Draining` 时推一次 `Draining`，应答 `Stop` 后结束流。

### 7.5 停止

- 启动回调没有成功过（`Resolved`、`Configured`，或在那之前进入 `Failed`）：不调停止回调，应答 `ok`，以退出码 0 退出。宿主在"探测启动"（§3）与配置、启动失败后都会这样停止插件。
- `Started`、`Ready`、`Degraded`：进入 `Draining`，新到的调用与投递以 `PLUGINS_UNAVAILABLE_001` 拒绝；等在途调用结束，最长到宽限期（`grace_seconds`，`0` 按 5 秒）；宽限到了仍没结束的调用触发它们的取消信号（RFC-0003，只经令牌通知，不强行中止）、不再等。然后调停止回调（`reason` 原样交给作者），应答，进入 `Stopped`，以退出码 0 退出。
- 启动回调成功过、之后进入 `Failed`（健康回调报 `Failed`）：不迁移状态，在途调用的处理同上，然后调停止回调、应答、退出。
- 停止回调在进程里最多调一次。它返回失败时 `StopResponse.status` 带那个错误，抛出时为 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为 `stop`）；两种情况都照常退出（宿主只记录，§5）。

### 7.6 回调抛出时的 reason

生命周期与健康回调抛出异常时，错误不带异常的文本（文本可能含路径、连接串等，只写进 stderr 日志）：`reason` 是异常的简单类型名，不带包名、命名空间与指针标记（如 `ValueError`、`IllegalStateException`、`TypeError`、`InvalidOperationException`；Go 返回的不是 `Error` 的错误取动态类型名，如 `*fs.PathError` 写 `PathError`）。没有异常的语言里回调以 panic 结束的写 `panic`。事件处理器与操作处理器抛出的按 `invocation.md` §4、RFC-0011 处理，不在此列。

### 7.7 stdin 关闭与 SIGTERM

插件把 stdin 读到结尾（或读出错）与收到 SIGTERM 都当作收尾请求：可服务的插件进入 `Draining`（其他状态不迁移），立即触发全部在途调用的取消信号并等它们结束；启动回调成功过而停止回调还没调过的，调一次停止回调，`reason` 为 `shutdown`。从收到请求起最多 3 秒（短于宿主发出终止信号后再等的 5 秒，§5），到时不论是否完成都以退出码 0 退出；正在处理的 `Stop` 照常进行，但同样受这 3 秒限制。
