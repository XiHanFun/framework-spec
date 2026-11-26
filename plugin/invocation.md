# 插件调用、事件与回调

> 规范性。协议 `xihan.plugin.v1`（`plugin.proto`，RFC-0017）。

## 1. 代理操作

握手声明的每个操作在宿主 `OperationRegistry` 登记为代理操作：契约取 `OperationDeclaration`（`id`、`stability`、`expose`、`permission` / `policy`），Schema 取 `request_schema` / `response_schema`。代理操作与本地操作无差别：协议适配器按 `expose` 发布（HTTP 路由、gRPC 方法、MCP tool、命令行、AI 工具），调用经宿主的完整管道，Authorize 阶段按契约判定；Handle 阶段把载荷与上下文发给插件。HTTP 路由、OpenAPI 文档与 MCP tool 跟随登记表（`protocol/http.md`、`protocol/mcp.md`，ADR-0251）：装配完成之后才登记的代理操作（运行期 `LoadAsync`、首次握手失败后重试成功）登记之后立即有路由、进文档、是 tool；gRPC 方法与 AI 工具目录只在启动时建成，此后登记的代理操作不是 gRPC 方法、也不是 AI 工具，只能经其他协议与内核调用器调用。

- 插件定义的权限码（ADR-0080）：清单 `contributes.permissions` 声明的是插件**定义**的权限码（权限码文法同 RFC-0008，不重复；不合文法或重复的清单读不出，按发现时被拒处理，`PLUGINS_VALIDATION_001`）。宿主登记插件的代理操作时替它把这些码登记进 `PermissionRegistry`（显示名取权限码本身；插件可在握手的消息目录里给 `permissions.<码>.name` / `.description` 的译文）。只登记清单声明了的：操作的 `permission` 既不是宿主已登记的码、也不在本插件清单里，仍按未登记拒绝（`KERNEL_VALIDATION_005`）。要引用宿主或别的插件定义的码，只在操作的 `permission` 里写，不在清单里声明。只进程外与 Wasm 宿主登记；进程内插件宿主（`Plugins.InProcess`）不向应用的登记表贡献操作，也不登记权限码。
- 登记一个插件的贡献是一次批量登记（`OperationRegistry.AddRange(owner, operations, permissions)`，归属是插件的清单 ID），全进或全不进：握手的消息目录最先以同一归属登记进本地化器（RFC-0010「归属目录」，ADR-0252），清单声明的权限码再进 `PermissionRegistry`（同组的操作可以引用它们），最后核对与写入代理操作；任一项不过——同名消息目录里同一键同一区域的模板不同（`LOCALIZATION_CONFLICT_001`）、声明的码与已登记的码重复（宿主特性定义的，或另一个已登记插件声明的，`KERNEL_SECURITY_001`）、超出权限登记表容量（`KERNEL_SECURITY_002`）、操作与已有操作重名（`KERNEL_OPERATION_001`）、超出操作登记表容量（`KERNEL_OPERATION_002`）、装配完成之后操作的 `permission` 未登记（`KERNEL_VALIDATION_005`）、宿主的启动核对之后操作的 `policy` 不存在（`SECURITY_VALIDATION_004`）、映射之后推导出的 HTTP 路由与已有的或同一组里的重复（`PROTOCOLS_CONFLICT_002`）、MCP 服务端建立之后 tool 名不合法或重复（`PROTOCOLS_VALIDATION_004` / `PROTOCOLS_CONFLICT_001`）——整个插件登记失败，已登记的目录与已写进的权限码撤回，不留下任何目录、权限码与代理操作（ADR-0071、ADR-0080）。
- 冲突的处理：两个插件声明同一个码，后登记的那个整体失败；宿主特性定义的码与插件声明的码重复，不论谁先登记都是冲突（插件先登记时，宿主特性的 `AddPermission` 失败）。配置阶段的冲突都是启动失败。
- 宿主配置阶段发现的插件随应用一起装配：声明的权限码在配置阶段登记，冲突与重名启动失败（`KERNEL_SECURITY_001` / `KERNEL_OPERATION_001`），消息目录在配置阶段以归属登记、在建立目录登记表时核对，冲突启动失败（`LOCALIZATION_CONFLICT_001`），权限码未登记在装配完成时一并核对、启动失败（`KERNEL_VALIDATION_005`），`policy` 不存在在宿主的启动核对里一并核对、启动失败（`SECURITY_VALIDATION_004`，RFC-0008：容器建好、任何特性启动之前，不论失败策略）。装配完成之后才登记的（首次握手失败后重试成功、运行期 `LoadAsync`）在登记时被拒：错误同上，插件按发现时被拒处理（`lifecycle.md` §1，`Failed`、健康条目、日志、不再重试），`LoadAsync` 把失败返回给调用方。
- 卸载撤回登记（ADR-0085）：`UnloadAsync` 按归属（清单 ID）一次撤回插件登记的全部代理操作与权限码（`OperationRegistry.RemoveOwnedBy`：先撤回操作再撤回权限码，与登记一样在登记表的锁里一次完成，与并发的登记互斥；读取不取锁，并发的调用按操作 ID 逐个查找，每个操作要么还登记着、要么已撤回），接着按同一归属撤回它的消息目录（RFC-0010「归属目录」，ADR-0252），再停止插件。撤回之后可以直接 `LoadAsync` 同一个插件（同一版本或新版本）重新登记，不用重启宿主；新版本的声明照登记时的规则核对，冲突、重名、引用不存在的权限码或策略照样整组被拒。
  - 卸载后的调用：HTTP 路由、OpenAPI 文档里的路径与 MCP tool 随撤回消失（ADR-0251），之后按地址的请求得到宿主对未匹配地址的答复，`tools/call` 得 `PROTOCOLS_NOTFOUND_002`；按操作 ID 查登记表的入口（内核调用器、启动时发布的 gRPC 方法）返回 `KERNEL_NOTFOUND_001`（操作未登记，不进管道）。撤回之前已经查到操作、正在执行的调用照常完成，或在插件停止后以 `PLUGINS_UNAVAILABLE_001` 结束。
  - 目录撤回之后，插件目录里的键解析不到，按 RFC-0010 退回键与参数的描述；撤回之前已经在执行的调用在目录撤回之后才翻译失败文案的，同样得到描述。
  - 引用了被撤回的权限码的其他操作（宿主或别的插件以 `permission` 引用了本插件声明的码）不随之撤回：重新登记这个码之前，它们的授权判定按策略不存在拒绝（`SECURITY_NOTFOUND_001`），不会放行。
  - Wasm 插件贡献的中间件同样按归属撤回（ADR-0088，`wasm.md` §6）：卸载先把它们撤出管道，之后开始的调用不再经过它们、不因插件已卸载而失败；别的中间件以次序约束引用了它们时卸载被拒（`KERNEL_PIPELINE_003`），什么都不撤。进程外插件不能贡献中间件（RFC-0017），清单写了 `contributes.pipeline` 的在发现时就被拒（`PLUGINS_VALIDATION_001`，`lifecycle.md` §1），没有要撤回的中间件。
  - 不随卸载撤回的：配置阶段接进容器的订阅（卸载后发给该插件的事件不投给任何插件，再加载后恢复；再加载时订阅变了，宿主重启后才生效），启动时发布的 gRPC 方法（按操作 ID 转交调用器，再加载后同一 ID 照常可用；Schema 等描述变了，宿主重启后才重新发布）。消息目录以前也在这里：配置阶段接进容器、不随卸载撤回，再加载时变了要重启宿主（2026-11-23 修订，ADR-0252）。
  - 首次握手失败、正在按重启策略重试的插件被卸载时，重试停止；卸载之后才完成的握手不再登记（只登记宿主里仍是当前运行时的插件）。

- 一元操作：`Plugin.Invoke`。`payload` 是请求类型的 JSON（空等价于 `{}`）；`InvokeResponse.status.ok` 为真时 `payload` 是响应类型的 JSON，或者为空，表示没有响应体（见下面「宿主读插件交回的应答」），否则 `error` 是业务失败。
  - 宿主读插件交回的应答（2026-11-26 写明，ADR-0315；以前 .NET 不核对、原样交回，之后 MCP 的 tool 结果与命令行 `invoke` 的输出宽松地解码，gRPC 载体把转义出的孤立代理项写进 protobuf，开了幂等的调用在记录应答时抛出）：`status.ok` 为真而 `payload` 不为空时，宿主先按 RFC-0001「JSON 文本」的共同检查核对它（语法、64 层、合法的 UTF-8 与 Unicode、任何一层的重复成员；RFC-0001「写出与转交」），没过即这次调用失败，载荷不交给协议适配器与调用方：`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`id` 为清单 ID，`phase` 为操作 ID，`reason` 为 `the response payload is not valid JSON text at '<路径>'`，路径照 RFC-0001：语法错误为 `$`，重复成员为第一处重复的路径）。
    - 这是插件写来的内容不合规，与 §3 里错误码不合文法同一类处置：进程外插件的进程与 Wasm 插件的实例照常服务，不按崩溃处理，不计入重启次数（`lifecycle.md` §6）。Wasm 载体里整条消息不是合法的 proto3 JSON 才按崩溃处理（`wasm.md` §2）；`payload` 是消息里的 `bytes` 字段，消息本身合法。错误码的 KIND 是 `UNEXPECTED`，这个操作开了熔断时计入失败（RFC-0004「Resilience 阶段」）。
    - `payload` 为空表示没有响应体，不按 JSON 文本核对（RFC-0001 说空文本不是 JSON，说的是要读成 JSON 值的文本），原样交回：协议适配器按没有响应体处理，开了幂等的调用记下没有响应体、回放出同样的空载荷（RFC-0006「幂等」）。只有空白的载荷不为空，照样核对，没过即上面的失败（2026-11-26 写明，ADR-0318；ADR-0315 时只写了照旧原样交回，算不算插件不合规留作待定，.NET 开了幂等的调用在记录空载荷时抛出）。
    - 幂等回放还原记录的应答（RFC-0011 规则 8）时做同一道检查，没过即解码失败（RFC-0006，`CACHING_CONFLICT_003`）。
    - 进程内插件不向宿主的登记表贡献操作（上文），没有经宿主转交的应答，不适用这一条。
- 流式操作（`streaming: true`）：`Plugin.InvokeStream`。调用方先发 `start`（`InvokeRequest`，`payload` 为请求对象），再发零个或多个 `chunk`（每帧一个完整 JSON 值），最后发 `end`（`ok: true`；`ok: false` 表示中止，被调方应停止处理）；被调方发零个或多个 `chunk`，最后发 `end`。任一方收到不符合顺序的帧（`start` 不是第一帧、`end` 之后还有帧）以 gRPC `InvalidArgument` 结束流。.NET 参考实现在 RFC-0011 定义流式操作契约之前只把流式操作暴露为内部调用，不经协议适配器发布。
  - 在流式操作契约的 RFC 之前（RFC-0017「非目标」），插件不实现流式调用、也不声明流式操作（`handshake.md` §3 第 4 条），2026-11-18 写明（ADR-0237；以前有的 SDK 直接拒绝、有的读帧、有的实现了流式操作）：`InvokeStream` 核对会话令牌后读第一帧，流在第一帧之前结束或第一帧不是 `start` 时以 gRPC `InvalidArgument` 结束流，第一帧是 `start` 时以 gRPC `Unimplemented` 结束流，不论操作是否声明、插件是否就绪。
- 调用到达时插件未 `Ready`：懒激活按 `lifecycle.md` 等待，否则 `PLUGINS_UNAVAILABLE_001`。
- 插件一侧处理 `Invoke` 依次核对（2026-11-18 写明，ADR-0237；以前有的 SDK 先报未就绪）：会话令牌（`handshake.md` §4）；操作在握手声明里，否则 gRPC `Unimplemented`；插件在可服务状态（`lifecycle.md` §7.3），否则 `Status.error` 为 `PLUGINS_UNAVAILABLE_001` + `plugins.process.unavailable`；然后按 §2 还原上下文，经 SDK 的调用器与完整管道执行（RFC-0011），`payload` 为空等同 `{}`。
- 插件一侧写出应答（RFC-0001「写出与转交」）：应答写不出时按 RFC-0011 规则 10 的「编码」处理，`Status.error` 为 `KERNEL_UNEXPECTED_001` + `kernel.invocation.unhandled_exception`（没有参数，没有 `inner`），`payload` 为空。处理器的失败由插件一侧这样报给宿主，宿主按 §3 读入。
  - 类型化的响应由 SDK 的调用器写成 JSON（2026-11-26 写明，ADR-0318；以前各 SDK 不一：Go 写成 U+FFFD 照常交回，TypeScript 与 Python 写成 `\ud800` 这样的转义交回、由宿主按上面「宿主读插件交回的应答」以 `PLUGINS_UNEXPECTED_001` 拒绝，Java 在插件一侧以 `KERNEL_UNEXPECTED_001` 失败）：响应里的字符串（成员名与值，含字典的键）有孤立的代理项，或者在以字节存放字符串的语言（Go、PHP）里不是合法的 UTF-8，即应答写不出，与浮点成员是 NaN 相同；不写成 U+FFFD，也不写成转义交给宿主。写出的整段文本还要过 RFC-0001「JSON 文本」的共同检查：嵌套超过 64 层（原样嵌进去的 JSON 文本，层数从响应的根算起）或任何一层有重复的成员名（`toJSON`、`MarshalJSON` 这类自定义写法与 `RawValue` 一类原样嵌进去的文本都可能写出），同样是应答写不出，在插件一侧本地失败，不交给宿主去按上面「宿主读插件交回的应答」拒绝（2026-11-26 再写明，ADR-0319；以前 Go 与 Java 写完之后已经这样核对，TypeScript、Python、Rust 与 PHP 不查深度（PHP 在 512 层以内照写），前三者也不查重复成员，这样的应答交回之后由宿主以 `PLUGINS_UNEXPECTED_001` 拒绝）。Python 的 `str` 里任何代理项码点（U+D800–U+DFFF）都写不出，成对出现的两个也一样（RFC-0001「写出与转交」，2026-11-26 写明，ADR-0319）。
  - 处理器以字符串交回的应答载荷（SDK 收字符串的接口；2026-11-26 写明，ADR-0315），写进 `InvokeResponse.payload` 之前严格编码成 UTF-8，编码不成即应答写不出（2026-11-26 更正，ADR-0318：ADR-0315 时这里的注记写成「以前 TypeScript 以 `TextEncoder` 编码处理器以字符串交回的应答，孤立的代理项换成 U+FFFD 照发」，与实际不符。TypeScript 的处理器没有以字符串交回应答的接口，应答总经调用器的 `writeJson` 写出，孤立的代理项写成 `\ud800` 这样的转义，再由宿主按上面一条拒绝；运行时随后的 `TextEncoder` 编码的是已经转义过的文本，换不出 U+FFFD）。
  - 以字节交回的应答原样写出，由宿主按上面「宿主读插件交回的应答」核对。

## 2. 上下文映射（RFC-0003）

| 字段 | `InvocationMetadata` | 说明 |
| --- | --- | --- |
| Trace | `trace_parent`、`trace_state` | 插件以其为父 Span |
| Deadline | `deadline` | 截止时间的文本（RFC-0003 规则 8）；宿主同时把它设为 gRPC 调用的截止时间，二者相同 |
| Baggage | `baggage` | 只含已登记的 `xihan.*` 键（`conventions/baggage-keys.md`）；租户、主体、关联 ID、幂等键、区域都在其中 |
| Items / Services | — | 不传播 |

`baggage` 里的 `xihan.tenant` 是这次调用的租户：Tenant 阶段裁定过时取裁定结果，裁定为无租户时不带；还没有裁定（宿主没有 Tenant 阶段，或调用还没走到它）时原样传递调用携带的值（2026-11-12 修订，ADR-0195）。插件回调与发布能带的租户以它为限（§5）。

插件必须按 `deadline` 与 gRPC 取消停止工作；返回的 `Error` 若是超时或取消，用 `KERNEL_TIMEOUT_001` / `KERNEL_CANCELLED_001`。

接收方还原上下文（插件读宿主的调用与投递、宿主读插件的回调，2026-11-18 写明，ADR-0237；以前各实现对不合法或不一致的值各有取舍）：

- `trace_parent` 按 RFC-0003 的文法取，不合文法即没有父 Span；`trace_state` 不进上下文（RFC-0003 的上下文没有它），回调时写空。
- `baggage` 只保留已登记的键（`conventions/baggage-keys.md`，按键名过滤），未登记的丢掉；插件一侧另把 `xihan.principal` 原样交给作者（宿主一侧按 §5 覆盖）。
- `deadline` 按 RFC-0003 规则 8 的文法读，读不出就当作没有；生效的截止时间取它与 gRPC 调用自身截止时间中较早的一个，两者都没有就没有截止时间。
  - 宿主读插件回调的 `deadline`，不为空而读不出时应当记一条告警并限频（2026-11-24 增补，ADR-0279；以前宿主不留痕迹）：告警不带 `deadline` 的原文，可以带插件的清单 ID 与原文的长度；限频的要求同 `protocol/http.md`「上下文映射」对 `xihan-deadline` 的要求，与请求头的告警分开计。`deadline` 为空是没有给，不记。回调照常按没有这个截止时间处理。
- `deadline` 的写法：宿主写 UTC、七位小数、`+00:00` 偏移（.NET 的 `O` 格式，如 `2026-11-18T03:25:00.1234567+00:00`），插件回调时写同样的格式，小数不足七位补零。读法（文法、各段的取值范围、UTC 的范围与小数的截断）见 RFC-0003 规则 8：2026-11-24 起文法写在那里，与 HTTP 载体的 `xihan-deadline` 头共用（ADR-0276），内容与以前写在这里的相同。向量：`conformance/vectors/plugin/deadlines.json`。

## 3. 错误映射

- 业务失败走 `Status.error`，gRPC 状态保持 `OK`；`Error` 字段与 RFC-0001 的 JSON 编码一一对应（`details` 是 JSON 对象文本）。宿主用握手时得到的目录翻译 `message_key`；键不在目录里时文案为"键 (参数)"。
- gRPC 状态只表示传输或协议层失败，宿主映射为：

| gRPC 状态 | 宿主错误 |
| --- | --- |
| `Unavailable`、连接失败、进程已退出 | `PLUGINS_UNAVAILABLE_001` |
| `DeadlineExceeded` | `KERNEL_TIMEOUT_001` |
| `Cancelled`，调用方的令牌（含截止令牌）已触发 | `KERNEL_CANCELLED_001`（宿主不按时钟判断是不是超时：截止时间到了的由 Resilience 阶段的截止时间中间件改报 `KERNEL_TIMEOUT_001`，RFC-0004） |
| `Cancelled`，调用方的令牌没有触发（插件停机或连接释放切断了进行中的调用） | `PLUGINS_UNAVAILABLE_001` |
| `Unimplemented`（插件未实现该 RPC 或操作） | `KERNEL_NOTFOUND_001` |
| `InvalidArgument`（帧序或载荷不合法） | `KERNEL_VALIDATION_001` |
| 其他 | `PLUGINS_UNEXPECTED_001`，细节含状态码与消息 |

- 各项的消息键与参数见 §8 与 `conformance/vectors/plugin/plugin-messages.json`。`reason` 写成 `<状态名>: <说明>` 的地方，状态名用 gRPC 状态码的 PascalCase 名字（`OK`、`Cancelled`、`Unknown`、`InvalidArgument`、`DeadlineExceeded`、`NotFound`、`AlreadyExists`、`PermissionDenied`、`ResourceExhausted`、`FailedPrecondition`、`Aborted`、`OutOfRange`、`Unimplemented`、`Internal`、`Unavailable`、`DataLoss`、`Unauthenticated`），说明原样取状态的消息（各 gRPC 库的文字不同，不要求一致）。
- 写出 `Error`（插件写给宿主、宿主写给插件，2026-11-18 写明，ADR-0237；以前有的 SDK 按某个区域把带键的错误渲染成文本）：`code`、`message_key` 原样；带消息键的错误 `text` 留空，由宿主按目录翻译成调用方的语言（RFC-0010）；只有自由文本错误（`Error.Literal`）写 `text`；`details` 是模板参数与细节的 JSON 对象文本，没有时为空；`inner` 同样写出。`details` 写不出时这个错误不带细节、`details` 为空，其余字段照写，回调、发布与应答照常交出这个错误（RFC-0001「写出与转交」；2026-11-27 写明，ADR-0320；以前 .NET 照 JSON 写出器的缺省把孤立的代理项写成 U+FFFD，对方读到的是另一份细节）：细节里的字符串（成员名与值）有孤立的代理项，或者在以字节存放字符串的语言（Go、PHP）里不是合法的 UTF-8，不写成 U+FFFD 或 `\ud800` 这样的转义；写出的文本没过 RFC-0001「JSON 文本」的共同检查（嵌套超过 64 层、任何一层有重复的成员名）也一样。读入一侧把这样的细节当作没有（下一条），写出一侧不带，两边看到的相同；内部错误的细节各自核对。
- 读入 `Error`（宿主读插件、插件读宿主同一规则，参考实现 `PluginErrorMapper.FromProto`，向量 `conformance/vectors/plugin/error-reading.json`）：`code` 不合错误码文法时整个错误换成 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 一律为 `invoke`，不论是哪个 RPC；`reason` 为 `invalid error code '<code>'`；原来的 `inner` 丢掉）；`message_key` 合消息键文法的按键处理，否则成为自由文本错误，文案取 `text`，`text` 为空或只有空白（RFC-0001「空白字符」；2026-11-19 写明，ADR-0245）时取 `code`；带键的错误不取 `text`；`details` 为空、只有空白、是空对象 `{}`、通不过 RFC-0001 的共同 JSON 读取检查、不是对象，或任何一层有读成双精度会溢出（±∞）的数时当作没有细节（下溢的数读成 0，超出 64 位的整数读成双精度）；`inner` 同此。`Status.ok` 为假却没有 `error`（整个 `status` 缺失也一样）：`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（宿主读插件时 `phase` 为这次的阶段，`reason` 为 `the plugin reported a failure without an error`；插件读宿主时 `phase` 为操作 ID 或 `publish`，`reason` 为 `the host reported a failure without an error`）。
- 宿主把插件错误原样放进调用结果；协议适配器再按各自载体映射（HTTP Problem Details、gRPC 状态 + `xihan-error-code`、MCP `isError`）。

## 4. 事件投递（RFC-0009）

- 宿主为握手声明的每个 `Subscription` 按它的 `mode` 向消息代理订阅主题：`competing`（缺省）时订阅了主题的宿主实例里只有一个把消息交给插件，`broadcast` 时每个在线的宿主实例都把消息交给自己的插件实例（尽力而为、不进死信，RFC-0009「订阅方式」）。消息到达时派发器按 RFC-0009 开新作用域、构造上下文、经完整管道，在 Handle 阶段调用 `Plugin.HandleEvent`，只投给以投递的方式订阅了该主题的插件。`message_type` 非空时只投递 `type` 相同的消息，其余在宿主侧按成功处理（不进插件）。
- `HandleEventRequest.mode` 是这次投递的订阅方式（`competing` 或 `broadcast`），宿主总是填写，空同 `competing`。插件按（主题，方式）找处理器，再看类型：只交给以这种方式声明、`message_type` 为空或等于信封 `type` 的订阅。同一主题的不同类型可以各用一种方式（握手第 5 条只禁止同一（主题，类型）对重复），例如（`t`，空）竞争、（`t`，`x`）广播时，类型 `x` 的消息以两种方式各到一次，插件靠 `mode` 把竞争投递交给前者、广播投递交给后者。插件找不到以这种方式声明的订阅时返回 `error`，不改交给另一种方式的处理器。
- `Envelope` 是 CloudEvents 1.0 属性的平铺；`attempt` 从 1 起。插件返回 `ok` 表示已处理；返回 `error` 由代理按重试与死信策略处理；插件不可用期间的投递同样按该策略处理。
- `Envelope.time`（2026-11-25 修订，ADR-0293；以前宿主读插件写来的 `time` 用通用的日期解析：没有偏移的按 UTC 读，空格代替 `T`、没有秒、只有日期、偏移不带冒号、首尾带空白、超过九位的小数与 RFC 1123 的日期也读得出）：空串表示没有这个属性；不是空串时文法与 RFC-0009「信封」第 3 条相同，也就是 RFC-0003 规则 8 的读法（`^[0-9]{4}-[0-9]{2}-[0-9]{2}[Tt][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?([Zz]|[+-][0-9]{2}:[0-9]{2})\z`，按整串匹配、不裁剪空白，各段的取值范围与换算成 UTC 之后的范围照规则 8），读成 UTC 时刻，比实现的时钟精度更细的小数截断、不进位。
  - 宿主写给插件的（`HandleEventRequest.envelope`）一律合这条文法：信封没有时间写空串，有则换算成 UTC、七位小数、偏移写 `+00:00`（.NET 的 `O` 格式，与截止时间的写法相同）。SDK 怎样读见下面「SDK 读宿主写来的」。
  - 插件写给宿主的（`Host.Publish` 的信封、Wasm 的 `publish` 导入）不合文法或越界时，那一项不发布，结果是 `MESSAGING_VALIDATION_002` + `messaging.envelope.invalid_time`（参数 `time` 是原文），其余各项照常；这是 §5「逐项的核对」里「信封能否转换」那一步，宿主不换别的格式再试，也不当作没有时间。SDK 怎样写见下面「SDK 写给宿主的」。
  - SDK 读宿主写来的（2026-11-25 修订，ADR-0306；以前 SDK 把宿主写来的文本原样交给作者，不另解析、不改写）：SDK 把 `time` 按这条文法（RFC-0003 规则 8）读成该语言的时间类型交给作者，空串是没有时间。比该时间类型的精度更细的小数截去、不进位，得到的时刻不晚于原文，纪元之前的时刻也一样。读不出（不合文法、越界，或者合文法却超出该语言时间类型能表示的范围）时以 gRPC `InvalidArgument` 拒绝这次投递，不交给处理器：这是下面「插件一侧处理 `HandleEvent`」第 2 步「宿主写来的内容不合法」，排在找订阅之前。宿主照「宿主写给插件的」那一条写出时，只会遇到最后一种情形（如 Windows 上 Rust 的 `SystemTime` 表示不了 1601 年之前的时刻）。
  - SDK 写给宿主的（2026-11-25 修订，ADR-0306；以前作者给的是文本，SDK 原样写出、不在本地核对，另提供以时刻类型写入的接口时要求写出的文本合这条文法、宜与宿主的写法相同）：作者发布时给该语言的时间类型，或者不给。SDK 把它换算成 UTC，写成 `YYYY-MM-DDTHH:MM:SS`、可选的秒的小数与 `Z`（`T` 与 `Z` 大写）：小数去掉末尾的 0，全为 0 时不写小数点，最多写到该时间类型的精度（纳秒九位、100 纳秒七位、微秒六位、毫秒三位）；不给时写空串。例如 `2026-11-25T03:25:00Z`、`2026-11-25T03:25:00.5Z`、`2026-11-25T03:25:00.12Z`，`2026-11-25T11:25:00.25+08:00` 这个时刻写成 `2026-11-25T03:25:00.25Z`。这与宿主的写法（七位小数、`+00:00`）不同，两种都合这条文法。
    - SDK 不在本地核对年份范围：换算成 UTC 之后年份不在 `0001`–`9999` 之间的时刻照写（不改成范围之内的时刻，也不写成空串），由宿主按上面「插件写给宿主的」那一条以 `messaging.envelope.invalid_time` 拒绝那一项，其余各项照常。理由同 SDK 不在本地核对 `xihantenant`：本地先判会改变与宿主其余逐项核对的先后（§5）。这样的时刻写成什么文本规范不作规定（各 SDK 现在的写法不一，有五位数的年份、带正负号的年份与 `0000` 年），时间类型里不是有效时刻的值（如 JavaScript 的 `Invalid Date`）同样照写、写法不作规定。
  - 向量（2026-11-26 补充，ADR-0310；以前 `envelope-times.json` 只在档 `host`，SDK 写给宿主的文本没有收向量）：
    - `conformance/vectors/plugin/envelope-times.json`（档 `host` 与 `plugin`）：宿主读插件写来的文本与宿主写给插件的文本。SDK 读宿主写来的文本用的是同一条文法，它的 `valid` 与 `invalid` 对 SDK 的读入同样适用（`valid` 的时刻截到该时间类型的精度后比较）；`written` 是宿主的写法（七位小数、`+00:00`），SDK 读得出，但不是 SDK 写出的样子。
    - `conformance/vectors/plugin/sdk-envelope-times.json`（档 `plugin`）：SDK 一侧，按纳秒、100 纳秒、微秒与毫秒四种精度各给期望值，SDK 取自己时间类型精度的那一项回放。`writing` 是作者给的时刻（合这条文法的文本，精确到纳秒）与写给宿主的文本，`reading` 是宿主写来的文本与读出的时刻，`unreadable` 是 `envelope-times.json` 的 `invalid` 之外 SDK 同样必须读不出的文本。年份越界的时刻写成什么不作规定，不收用例。
- `Envelope.data_schema`（字段 11，2026-11-25 增补，ADR-0305；以前协议没有这个字段，信封也不承载 `dataschema`）是 RFC-0009「信封」的 `dataschema`：空串表示没有这个属性，不是空串时照原样就是它的值，不裁剪、不规范化，取值规则与 RFC-0009 相同（与 `source` 一样只要求非空，不核对 URI 语法）。Wasm 载体的 proto3 JSON 里字段名是 `dataSchema`。
  - 宿主写给插件的（`HandleEventRequest.envelope`）：信封有 `dataschema` 时照原样写，没有写空串。SDK 原样交给作者，不另解析。
  - 插件写给宿主的（`Host.Publish` 的信封、Wasm 的 `publish` 导入）：空串是没有；不是空串时照原样作为 `dataschema` 发布，经代理到达处理器时不变。SDK 不在本地核对、原样写出作者给的文本，理由同 `time`。只有空白的值规范不作规定（同 RFC-0009「一致性」里不收进向量的写法）：.NET 宿主与 `source` 只有空白时一样拒绝那一项，`MESSAGING_VALIDATION_002` + `messaging.envelope.invalid`，这是 §5「逐项的核对」里「信封能否转换」那一步。
  - 兼容：同一协议版本里的新增字段（规范的 MINOR，RFC-0024「版本」第 3、4 条），不升协议版本，也不在握手里协商。按较早的规范实现的插件忽略它（protobuf 的未知字段；Wasm 载体的 JSON 映射同样忽略未知字段，`wasm.md` §2），读到的信封没有 `dataschema`；按较早的规范实现的宿主不认它，插件发布到那样的宿主上的信封丢掉 `dataschema`、其余照常发布。宿主不按对端的 SDK 版本改变写出的内容。
- 至少一次投递：插件处理器必须幂等（可用 `baggage` 里的 `xihan.idempotency-key` 或信封 `id`）。
- 插件一侧处理 `HandleEvent` 依次（2026-11-18 写明，ADR-0237；以前找不到订阅时各 SDK 的错误码不同，匹配多个订阅时有的只交给第一个）：
  1. 会话令牌（`handshake.md` §4）。
  2. 宿主写来的内容不合法：没有 `envelope`，`mode` 不是空、`competing`、`broadcast`，或者信封的 `time` 不是空串却读不出（§4「SDK 读宿主写来的」；2026-11-25 修订，ADR-0306，以前 SDK 不读 `time`）：gRPC `InvalidArgument`。
  3. 按（主题，方式，类型）找订阅，一个也没有：`Status.error` 为 `KERNEL_NOTFOUND_001` + `plugins.event.unsubscribed`（`id` 为清单 ID，`topic`、`type` 取信封，`mode` 为这次投递的方式，空写 `competing`）。
  4. 插件在可服务状态（`lifecycle.md` §7.3），否则 `PLUGINS_UNAVAILABLE_001` + `plugins.process.unavailable`。
  5. 按 §2 从 `metadata` 还原上下文；匹配的订阅按握手里的声明顺序依次交给各自的处理器，第一个失败即终止并作为结果（与 RFC-0009「投递」第 2 条的派发器相同）。例如（`t`，空）与（`t`，`x`）都是竞争订阅时，类型 `x` 的一次竞争投递先后交给两者。处理器抛出为 `KERNEL_UNEXPECTED_001` + `kernel.invocation.unhandled_exception`（同 RFC-0011 规则 5）。全部成功为 `ok`。

## 5. 回调（`Host` 服务）

每次回调带元数据 `xihan-plugin-session`（`handshake.md`）；宿主以插件身份执行：主体 ID 为清单 ID、认证方案 `Plugin`、无声明；回调元数据里的 `xihan.principal` 被忽略并覆盖，其余已登记 Baggage 采用，追踪以回调的 `trace_parent` 为父。

租户只能是宿主随调用交给插件的那一个（2026-11-12 修订，ADR-0195；以前 `xihan.tenant` 原样采用，插件身份没有租户声明可比对，插件能在任意启用的租户里调用宿主操作、发布消息）：

- `Host.Invoke` 的元数据带 `xihan.tenant` 时，它必须是宿主此刻交给该插件处理的某个调用（操作、事件或中间件）的租户（§2），否则不执行，`Status.error` 为 `PLUGINS_DENIED_002`。不带时是无租户的调用。
- `Host.Publish` 以 `metadata` 按 §2 建调用上下文（主体同上被覆盖），在这个上下文里发布：发布侧按 RFC-0009 从上下文补齐信封缺的扩展属性（租户、主体、关联 ID、区域、追踪）。`metadata` 带 `xihan.tenant` 时先按同一规则判定，越界即整批不发布，每一项都是 `PLUGINS_DENIED_002` + `plugins.callback.tenant_denied`（2026-11-18 修订，ADR-0237；以前宿主不读 `metadata`，插件在处理某个租户的调用时发布的消息不带租户，与进程内的发布不同）。
- `Host.Publish` 的信封带 `xihantenant` 扩展时逐项按同一规则判定，越界的那一项为 `PLUGINS_DENIED_002`，其余照常发布。逐项的核对依次是：宿主有没有消息发布器（`plugins.callback.messaging_unavailable`）、主题文法（`MESSAGING_VALIDATION_002`）、`xihantenant`、信封能否转换。
- 宿主没有在处理该插件的调用时（调用已结束、插件在后台回调），回调不能带租户。

| RPC | 权限 | 语义 |
| --- | --- | --- |
| `Host.Invoke` | `capability:kernel.pipeline` | 以新的调用经宿主完整管道执行操作（含 Authorize：插件主体须被授予操作要求的权限）；未登记的操作 `KERNEL_NOTFOUND_001` |
| `Host.Publish` | `capability:messaging.queue` | 经宿主消息代理发布，逐项结果；`topic` 为空的信封 `MESSAGING_VALIDATION_002` |

回调所需的权限必须**在清单 `permissions` 里声明并且已被宿主授予**才生效：清单是插件权限的唯一事实源（RFC-0017），宿主授予而清单没有声明的权限不放行回调。未声明或未授予都返回 gRPC `PermissionDenied` + `PLUGINS_DENIED_002`。`net.outbound:` 与 `fs.*:` 权限不经协议表达，由宿主通过操作系统沙箱（若启用）强制。宿主一侧依次核对：会话令牌、权限、`metadata` 的租户，然后执行（`Host.Publish` 再逐项核对）。

插件一侧（SDK 的回调客户端，2026-11-18 写明，ADR-0237；以前有的 SDK 在本地核对信封的租户、有的不核对，回调失败的映射也各不相同）：

- 回调端点按 `handshake.md` §4 选；没有可用端点时不发请求，`Host.Invoke` 以 `PLUGINS_UNAVAILABLE_001` + `plugins.host.unavailable`（`reason` 为 `no supported callback endpoint`）失败，`Host.Publish` 的每一项都是这个错误。
- 发出之前依次核对，不合即不发请求（2026-11-18 写明先后）：先是有没有可用的回调端点（上一条），再按宿主的顺序核对，与宿主给出同样的错误：
  1. 权限：RPC 所需的权限既在清单 `permissions` 里、也在 `Configure` 的 `granted_permissions` 里，否则 `PLUGINS_DENIED_002` + `plugins.callback.denied`（`permission` 为所需的权限）。
  2. 租户：回调上下文的 `xihan.tenant` 必须是插件此刻正在处理的某个调用或投递的租户（同一租户可以同时有多个），否则 `PLUGINS_DENIED_002` + `plugins.callback.tenant_denied`。
  - `Host.Publish` 两项都是整批的结果，每一项都是那个错误。信封的 `xihantenant` 不在本地核对，由宿主逐项判定：宿主在它之前还核对发布器与主题，本地先判它会改变错误的先后。
  - 最后，回调上下文的截止时间已到或已被取消时也不发请求，结果与下表相应的一行相同：截止时间已到按 `DeadlineExceeded`，否则已被取消按「`Cancelled`，回调上下文的取消信号已触发」。截止时间已到指截止令牌（RFC-0003 第 1 条）已触发，或者按时钟已过截止时间，两者任一成立即是（2026-11-19 写明，ADR-0246；以前没说按什么判断，有的 SDK 只读时钟）：截止令牌由定时器触发，可能早于时钟读数到达截止时间，也可能晚于它；只读时钟时，令牌先到会让截止时间落到「已被取消」那一行。这是发出之前的核对，与调用器进入时的核对（RFC-0011 规则 10）一样可以读时钟；管道里截止时间步骤的归因仍只看令牌（RFC-0004）。
  - 截止时间之后还有一项：`Host.Invoke` 的载荷以字符串给出时（SDK 收字符串的接口），发出之前严格编码成 UTF-8（RFC-0001「JSON 文本」；2026-11-26 写明，ADR-0312；以前 TypeScript 的 `TextEncoder` 与 Java 的 `getBytes(UTF_8)` 把孤立的代理项换成 U+FFFD 或 `?` 照发，宿主收到的是另一份合法的 JSON）：孤立的代理项编码不成，即不发请求，`KERNEL_VALIDATION_001` + `kernel.operation.invalid_payload`（`id` 为操作 ID，`reason` 为 `$`），与宿主读到不是 JSON 的载荷时相同（RFC-0011 规则 2）。这一项排在上面各项之后：宿主在管道的 Validate 阶段才解码载荷，权限、租户与截止时间的错误先于它。载荷以字节给出时原样发出，由宿主核对。
  - `Host.Publish` 的信封 `data` 以字符串给出时（SDK 收字符串的接口；2026-11-26 写明，ADR-0315；以前 Java 的 `Envelope.Builder.data(String)` 以 `getBytes(UTF_8)` 把孤立的代理项换成 `?`，TypeScript 的 `TextEncoder` 换成 U+FFFD，宿主收到的是另一份数据），发出之前同样严格编码成 UTF-8（RFC-0001「写出与转交」）。编码不成的那一项不发出，结果是 `MESSAGING_VALIDATION_002` + `messaging.envelope.invalid`（`reason` 为 `the data has a lone surrogate and cannot be encoded as UTF-8`），与宿主转换不了信封时同一个码与键（上文「逐项的核对」的最后一步）；其余各项照常组成一次请求发出，结果放回各自原来的位置，每一项都编码不成时不发请求。宿主那几步逐项核对（发布器、主题、`xihantenant`）对不发出的那一项不再进行。这一项排在上面整批的各项（端点、权限、租户、截止时间）之后：整批不合时每一项都是整批的那个错误，与 `Host.Invoke` 的载荷同一个先后。`data` 以字节给出时原样发出。
  - 回调客户端已关闭（2026-11-27 写明，ADR-0328；以前没说它与上面各项的先后，有本地关闭状态的 TypeScript 与 Java 把它排在编码之后，其余 SDK 在发出时才失败）：SDK 释放了回调客户端（插件停止、运行时收尾）之后的回调不发请求，也不重新连接，结果与下表「`Cancelled`，回调上下文的取消信号没有触发；`Unavailable`；连接失败」那一行相同，`PLUGINS_UNAVAILABLE_001` + `plugins.host.unavailable`，`reason` 照那一行写成 `<状态名>: <说明>`，用哪个状态名与说明不作规定。它不是发出之前的核对，而是发出这一步本身，排在上面各项之后，载荷与 `data` 的编码也在它之前：`Host.Invoke` 的载荷编码不成时结果是 `kernel.operation.invalid_payload`，与客户端是否已关闭无关；`Host.Publish` 编码不成的那一项是 `messaging.envelope.invalid`、不发出，其余各项都是已关闭的那个错误，每一项都编码不成时不发请求，每一项都是编码的错误。整批的核对不合时每一项仍是整批的那个错误。回调途中客户端被释放、切断了进行中的回调，结果同样是那一行（截止时间已到或调用方取消时照下表相应的行）。
- 回调途中（2026-11-19 写明，ADR-0246；以前没说，有的 SDK 在回调上下文的取消信号触发时一律在本地中止回调，截止时间到了也报成取消）：截止时间到了，结果是 `DeadlineExceeded` 那一行。gRPC 调用带着同一个截止时间，可以交给 gRPC 库在本端中止；在本地中止时，同样按上一条判断截止时间已到。调用方取消、而截止时间未到时，在本地中止回调，结果是「`Cancelled`，回调上下文的取消信号已触发」那一行。插件管道里截止时间步骤派生给处理器的取消信号，在截止时间也会触发：回调客户端不能只凭这个信号已触发，就把回调当作调用方取消。
- 回调的 `metadata` 取自回调所在的调用上下文：`trace_parent` 是这个上下文按 RFC-0003 写出的 traceparent（与其他载体同一取法），`deadline` 按 §2 的写法，`baggage` 是上下文里已登记的键，`trace_state` 为空；gRPC 调用的截止时间与 `deadline` 相同。
- gRPC 状态的映射与宿主一侧（§3）对称（向量 `conformance/vectors/plugin/callback-status.json`）。gRPC 库自己的叫法先换成标准状态：库按截止时间在本端中止调用时报的状态（如 tonic 以 `Cancelled` 报 `Timeout expired`）按 `DeadlineExceeded`；回调客户端只按下表映射，不另按时钟或令牌改写结果（超时的归因交给插件自己的管道，RFC-0003 规则 1）：

| gRPC 状态 | 错误 |
| --- | --- |
| `Unauthenticated` | `PLUGINS_DENIED_002` + `plugins.callback.session_invalid` |
| `PermissionDenied` | `PLUGINS_DENIED_002` + `plugins.callback.denied`（`permission` 为所需的权限） |
| `DeadlineExceeded` | `KERNEL_TIMEOUT_001` + `kernel.invocation.deadline_exceeded` |
| `Cancelled`，回调上下文的取消信号已触发 | `KERNEL_CANCELLED_001` + `kernel.invocation.cancelled`（超时的归因交给插件自己管道里的截止时间步骤，RFC-0003） |
| `Cancelled`，回调上下文的取消信号没有触发；`Unavailable`；连接失败 | `PLUGINS_UNAVAILABLE_001` + `plugins.host.unavailable`（`reason` 为 `<状态名>: <说明>`） |
| `Unimplemented`，`Host.Invoke` | `KERNEL_NOTFOUND_001` + `kernel.operation.unknown`（`id` 为操作 ID） |
| `InvalidArgument`，`Host.Invoke` | `KERNEL_VALIDATION_001` + `kernel.operation.invalid_payload`（`id` 为操作 ID，`reason` 为状态的说明） |
| 其他，含 `Host.Publish` 的 `Unimplemented` 与 `InvalidArgument` | `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为操作 ID 或 `publish`，`reason` 为 `<状态名>: <说明>`） |

- `Host.Publish` 的 RPC 失败时每一项都是同一个错误；宿主返回的结果数与信封数不同时，每一项都是 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 为 `publish`，`reason` 为 `the host returned <结果数> results for <信封数> envelopes`）。`Status` 里的错误按 §3 的读入规则处理。

## 6. 日志与诊断

- 插件日志写 stderr，一行一条；宿主逐行转为日志（来源 `plugins.<清单 ID>`）。以 `{` 开头且能解析为 JSON 对象的行按结构化日志处理：`level`（`trace` / `debug` / `information` / `warning` / `error` / `critical`）、`message`、其余字段为作用域属性。
- 宿主为每次 `Invoke` / `InvokeStream` / `HandleEvent` 记录一条 Span `xihan.plugin.invoke`（种类 Client，与 `xihan.invoke` 同一个源，RFC-0023「内核的遥测」；2026-11-24 修订，ADR-0269，以前只写它是 `xihan.invoke` 的子 Span、带 `xihan.plugin.id` 与 `xihan.operation` 两个标签，.NET 没有记这条 Span）：
  - 父级按 RFC-0003 规则 7 取：管道里有 Observe 阶段的 `xihan.invoke` 时是它的子 Span；没有时以上下文的上游为远端父级，或作为这条链路的根，上游「不采样」时不开。
  - 标签在创建时给出：`xihan.plugin.id`（插件的清单 ID）；`Invoke` / `InvokeStream` 另带 `xihan.operation`（操作 ID），`HandleEvent` 不带，主题不进标签。
  - 状态：宿主一侧的结果是失败（插件报的业务失败、插件不可用、传输失败）时为 Error，说明是错误码；调用抛出（取消、载体的故障）时为 Error，说明只写异常类型的全名；成功时留空。
  - 它覆盖等插件可用与调用本身。宿主在它之内组装 `InvocationMetadata`，`trace_parent` 因此以它为父级（RFC-0003 规则 7「写出」），插件一侧的 Span 与插件原样带回的回调都挂在它下面；没有开这条 Span（没有监听器、不采样）时 `trace_parent` 照规则 7 取 `xihan.invoke` 或上下文的。

## 7. 一致性

gRPC 黑盒套件 `XiHan.Framework.Plugins.Conformance`（`PluginProtocolConformance`，任何语言的插件以其构建产物目录继承运行）以本文件、`handshake.md` 与 `lifecycle.md` 为用例来源：握手行与版本协商、无共同版本时拒绝、不带或带错会话令牌的调用以 `Unauthenticated` 拒绝（`handshake.md` §4，ADR-0202）、清单一致性（含清单不写 `contributes.pipeline`）、Configure / Start / Health / WatchHealth / Stop 与宽限内退出、未知操作 `Unimplemented`、声明的操作都返回 `status`、订阅投递确认、stdin 关闭即退出，以及插件一侧的规则（2026-11-18 补充，ADR-0237）：调用次序不对以 `FailedPrecondition` 拒绝、`Start` 之前的 `Health` 报本地状态、未就绪时的调用与投递报 `plugins.process.unavailable`（未声明的操作仍先报 `Unimplemented`）、投递找不到订阅报 `plugins.event.unsubscribed`、未知的投递方式 `InvalidArgument`、`InvokeStream` 的第一帧、配置节不是对象时进入 `Failed`、`Start` 之前的 `Stop` 应答后退出、两个会话令牌值也以 `Unauthenticated` 拒绝，信封的 `time` 读不出的投递以 `InvalidArgument` 拒绝、`data_schema` 原样交给作者（2026-11-27 补充，ADR-0327，见下）；每个用例都以多项的 `XIHAN_PLUGIN_HOST_ADDRESS` 启动插件。framework-spec 的黑盒运行器 `xihan-conformance plugin` 有同样的用例。回调的令牌与权限、回调的本地核对与状态映射、崩溃重启由宿主与 SDK 实现的测试覆盖（回放 `callback-status.json`），SDK 读写信封 `time` 的取值（§4，ADR-0306）由 SDK 实现的测试覆盖（回放 `envelope-times.json` 与 `sdk-envelope-times.json`，2026-11-26 补充，ADR-0310）；流式帧序随流式操作契约补充。数据向量见 `conformance/vectors/plugin/`。

套件与运行器这样核对插件一侧处理信封的两条（2026-11-27 补充，ADR-0327；以前两者都不核对，只靠各 SDK 的单元测试，ADR-0305、ADR-0306）：

- 读不出的 `time`（运行器的用例 `unsubscribed`）：`Start` 之后向没人订阅的随机主题投递，信封的其余属性合法、方式为 `competing`，`time` 逐条取 `envelope-times.json` 的 `invalid` 与 `sdk-envelope-times.json` 的 `unreadable`，每一条都必须以 gRPC `InvalidArgument` 结束。主题没人订阅也是 `InvalidArgument`：这是 §4「插件一侧处理 `HandleEvent`」第 2 步，排在找订阅之前，答 `plugins.event.unsubscribed` 即不通过。合文法却超出某种语言时间类型范围的时刻（Windows 上 Rust 的 `SystemTime` 表示不了 1601 年之前）不在这两组里，不核对。
- `data_schema` 交给作者（运行器的用例 `event-envelope`）：处理器不回显信封，被测插件要声明一个回显信封的操作，运行器以 `--envelopes <操作 ID>`、套件以 `EnvelopeOperation` 指明它；没有指明，或插件没有订阅时，这个用例跳过。指明的操作不在握手声明的操作里即不通过。`Start` 之后向第一个订阅按它声明的方式与类型投递三条信封：`id` 各不相同，`time` 是 `envelope-times.json` 的 `written` 里第一个有时刻的文本（宿主的写法），`data_schema` 依次是 `HTTPS://Schemas.Example.COM/conformance/./v1/../order.json?v=1#Frag`、`urn:xihan:conformance:数据模式` 与空串，三次都必须答 `ok`。随后以 `{}` 调用那个操作：必须答 `ok`，载荷是 JSON 对象，成员 `envelopes` 是数组，每一项是一个 JSON 对象，对应作者在一次投递里拿到的信封，`id` 是信封的 `id`（字符串），`dataSchema` 是作者拿到的 `dataschema`（字符串；没有时为 `null`、空串或不写），其余成员不作规定。三条信封都必须列在里面，同一个 `id` 的每一项 `dataSchema` 都与投递时逐字符相同（不裁剪、不规范化），空串那一条是没有。数组按收到的先后，可以只留最近的若干项，但至少留得下这三项；插件进程每个用例单独启动，列出的只有这个进程收到的。

## 8. 运行时的消息键

SDK 运行时与宿主共用下面的消息键（2026-11-18 写明，ADR-0237；以前规范只列了错误码，键与参数只在 .NET 的目录里，各 SDK 的参数与之不一致）。宿主必须有这些键的目录（.NET 在 `Plugins.Protocol`，10 种区域），SDK 不随握手发送它们（`handshake.md` §3 第 6 条）；SDK 自己带同样的目录（参考实现的 10 种区域）供本地渲染。码、键、参数、en-US 模板与固定的 `reason` 文本以向量 `conformance/vectors/plugin/plugin-messages.json` 为准；`reason` 是英文诊断文本，不本地化。

| 键 | 参数 | 错误码 | 场合 |
| --- | --- | --- | --- |
| `plugins.process.unavailable` | `id`、`state` | `PLUGINS_UNAVAILABLE_001` | 不在可服务状态时到达的调用与投递（`state` 是状态名） |
| `plugins.process.failed` | `id`、`phase`、`reason` | `PLUGINS_UNEXPECTED_001` | 回调抛出、配置节不是对象、对方写来的错误不合法、没有对应的 gRPC 状态；宿主组装 `Configure` 时清单声明的绑定读不出来、属性不合文法或超过上限、配置节有孤立的代理项（`lifecycle.md` §2）；插件交回的应答载荷没过 JSON 文本的检查（§1） |
| `plugins.event.unsubscribed` | `id`、`topic`、`type`、`mode` | `KERNEL_NOTFOUND_001` | 投递找不到订阅 |
| `plugins.host.unavailable` | `id`、`reason` | `PLUGINS_UNAVAILABLE_001` | 回调连不上宿主、没有可用的回调端点 |
| `plugins.callback.denied` | `id`、`permission` | `PLUGINS_DENIED_002` | 回调所需的权限未声明或未授予 |
| `plugins.callback.tenant_denied` | `id`、`tenant` | `PLUGINS_DENIED_002` | 回调带了不属于在途调用的租户 |
| `plugins.callback.session_invalid` | 无 | `PLUGINS_DENIED_002` | 宿主拒绝了会话令牌 |

`id` 一律是插件的清单 ID。错误的 `details` 就是这些参数，不另加成员。
