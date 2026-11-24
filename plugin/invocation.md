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

- 一元操作：`Plugin.Invoke`。`payload` 是请求类型的 JSON（空等价于 `{}`）；`InvokeResponse.status.ok` 为真时 `payload` 是响应类型的 JSON，否则 `error` 是业务失败。
- 流式操作（`streaming: true`）：`Plugin.InvokeStream`。调用方先发 `start`（`InvokeRequest`，`payload` 为请求对象），再发零个或多个 `chunk`（每帧一个完整 JSON 值），最后发 `end`（`ok: true`；`ok: false` 表示中止，被调方应停止处理）；被调方发零个或多个 `chunk`，最后发 `end`。任一方收到不符合顺序的帧（`start` 不是第一帧、`end` 之后还有帧）以 gRPC `InvalidArgument` 结束流。.NET 参考实现在 RFC-0011 定义流式操作契约之前只把流式操作暴露为内部调用，不经协议适配器发布。
  - 在流式操作契约的 RFC 之前（RFC-0017「非目标」），插件不实现流式调用、也不声明流式操作（`handshake.md` §3 第 4 条），2026-11-18 写明（ADR-0237；以前有的 SDK 直接拒绝、有的读帧、有的实现了流式操作）：`InvokeStream` 核对会话令牌后读第一帧，流在第一帧之前结束或第一帧不是 `start` 时以 gRPC `InvalidArgument` 结束流，第一帧是 `start` 时以 gRPC `Unimplemented` 结束流，不论操作是否声明、插件是否就绪。
- 调用到达时插件未 `Ready`：懒激活按 `lifecycle.md` 等待，否则 `PLUGINS_UNAVAILABLE_001`。
- 插件一侧处理 `Invoke` 依次核对（2026-11-18 写明，ADR-0237；以前有的 SDK 先报未就绪）：会话令牌（`handshake.md` §4）；操作在握手声明里，否则 gRPC `Unimplemented`；插件在可服务状态（`lifecycle.md` §7.3），否则 `Status.error` 为 `PLUGINS_UNAVAILABLE_001` + `plugins.process.unavailable`；然后按 §2 还原上下文，经 SDK 的调用器与完整管道执行（RFC-0011），`payload` 为空等同 `{}`。

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
- 写出 `Error`（插件写给宿主、宿主写给插件，2026-11-18 写明，ADR-0237；以前有的 SDK 按某个区域把带键的错误渲染成文本）：`code`、`message_key` 原样；带消息键的错误 `text` 留空，由宿主按目录翻译成调用方的语言（RFC-0010）；只有自由文本错误（`Error.Literal`）写 `text`；`details` 是模板参数与细节的 JSON 对象文本，没有时为空；`inner` 同样写出。
- 读入 `Error`（宿主读插件、插件读宿主同一规则，参考实现 `PluginErrorMapper.FromProto`，向量 `conformance/vectors/plugin/error-reading.json`）：`code` 不合错误码文法时整个错误换成 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（`phase` 一律为 `invoke`，不论是哪个 RPC；`reason` 为 `invalid error code '<code>'`；原来的 `inner` 丢掉）；`message_key` 合消息键文法的按键处理，否则成为自由文本错误，文案取 `text`，`text` 为空或只有空白（RFC-0001「空白字符」；2026-11-19 写明，ADR-0245）时取 `code`；带键的错误不取 `text`；`details` 为空、只有空白、是空对象 `{}`、通不过 RFC-0001 的共同 JSON 读取检查、不是对象，或任何一层有读成双精度会溢出（±∞）的数时当作没有细节（下溢的数读成 0，超出 64 位的整数读成双精度）；`inner` 同此。`Status.ok` 为假却没有 `error`（整个 `status` 缺失也一样）：`PLUGINS_UNEXPECTED_001` + `plugins.process.failed`（宿主读插件时 `phase` 为这次的阶段，`reason` 为 `the plugin reported a failure without an error`；插件读宿主时 `phase` 为操作 ID 或 `publish`，`reason` 为 `the host reported a failure without an error`）。
- 宿主把插件错误原样放进调用结果；协议适配器再按各自载体映射（HTTP Problem Details、gRPC 状态 + `xihan-error-code`、MCP `isError`）。

## 4. 事件投递（RFC-0009）

- 宿主为握手声明的每个 `Subscription` 按它的 `mode` 向消息代理订阅主题：`competing`（缺省）时订阅了主题的宿主实例里只有一个把消息交给插件，`broadcast` 时每个在线的宿主实例都把消息交给自己的插件实例（尽力而为、不进死信，RFC-0009「订阅方式」）。消息到达时派发器按 RFC-0009 开新作用域、构造上下文、经完整管道，在 Handle 阶段调用 `Plugin.HandleEvent`，只投给以投递的方式订阅了该主题的插件。`message_type` 非空时只投递 `type` 相同的消息，其余在宿主侧按成功处理（不进插件）。
- `HandleEventRequest.mode` 是这次投递的订阅方式（`competing` 或 `broadcast`），宿主总是填写，空同 `competing`。插件按（主题，方式）找处理器，再看类型：只交给以这种方式声明、`message_type` 为空或等于信封 `type` 的订阅。同一主题的不同类型可以各用一种方式（握手第 5 条只禁止同一（主题，类型）对重复），例如（`t`，空）竞争、（`t`，`x`）广播时，类型 `x` 的消息以两种方式各到一次，插件靠 `mode` 把竞争投递交给前者、广播投递交给后者。插件找不到以这种方式声明的订阅时返回 `error`，不改交给另一种方式的处理器。
- `Envelope` 是 CloudEvents 1.0 属性的平铺；`attempt` 从 1 起。插件返回 `ok` 表示已处理；返回 `error` 由代理按重试与死信策略处理；插件不可用期间的投递同样按该策略处理。
- 至少一次投递：插件处理器必须幂等（可用 `baggage` 里的 `xihan.idempotency-key` 或信封 `id`）。
- 插件一侧处理 `HandleEvent` 依次（2026-11-18 写明，ADR-0237；以前找不到订阅时各 SDK 的错误码不同，匹配多个订阅时有的只交给第一个）：
  1. 会话令牌（`handshake.md` §4）。
  2. 没有 `envelope`，或 `mode` 不是空、`competing`、`broadcast`：gRPC `InvalidArgument`。
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

gRPC 黑盒套件 `XiHan.Framework.Plugins.Conformance`（`PluginProtocolConformance`，任何语言的插件以其构建产物目录继承运行）以本文件、`handshake.md` 与 `lifecycle.md` 为用例来源：握手行与版本协商、无共同版本时拒绝、不带或带错会话令牌的调用以 `Unauthenticated` 拒绝（`handshake.md` §4，ADR-0202）、清单一致性（含清单不写 `contributes.pipeline`）、Configure / Start / Health / WatchHealth / Stop 与宽限内退出、未知操作 `Unimplemented`、声明的操作都返回 `status`、订阅投递确认、stdin 关闭即退出，以及插件一侧的规则（2026-11-18 补充，ADR-0237）：调用次序不对以 `FailedPrecondition` 拒绝、`Start` 之前的 `Health` 报本地状态、未就绪时的调用与投递报 `plugins.process.unavailable`（未声明的操作仍先报 `Unimplemented`）、投递找不到订阅报 `plugins.event.unsubscribed`、未知的投递方式 `InvalidArgument`、`InvokeStream` 的第一帧、配置节不是对象时进入 `Failed`、`Start` 之前的 `Stop` 应答后退出、两个会话令牌值也以 `Unauthenticated` 拒绝；每个用例都以多项的 `XIHAN_PLUGIN_HOST_ADDRESS` 启动插件。framework-spec 的黑盒运行器 `xihan-conformance plugin` 有同样的用例。回调的令牌与权限、回调的本地核对与状态映射、崩溃重启由宿主与 SDK 实现的测试覆盖（回放 `callback-status.json`）；流式帧序随流式操作契约补充。数据向量见 `conformance/vectors/plugin/`。

## 8. 运行时的消息键

SDK 运行时与宿主共用下面的消息键（2026-11-18 写明，ADR-0237；以前规范只列了错误码，键与参数只在 .NET 的目录里，各 SDK 的参数与之不一致）。宿主必须有这些键的目录（.NET 在 `Plugins.Protocol`，10 种区域），SDK 不随握手发送它们（`handshake.md` §3 第 6 条）；SDK 自己带同样的目录（参考实现的 10 种区域）供本地渲染。码、键、参数、en-US 模板与固定的 `reason` 文本以向量 `conformance/vectors/plugin/plugin-messages.json` 为准；`reason` 是英文诊断文本，不本地化。

| 键 | 参数 | 错误码 | 场合 |
| --- | --- | --- | --- |
| `plugins.process.unavailable` | `id`、`state` | `PLUGINS_UNAVAILABLE_001` | 不在可服务状态时到达的调用与投递（`state` 是状态名） |
| `plugins.process.failed` | `id`、`phase`、`reason` | `PLUGINS_UNEXPECTED_001` | 回调抛出、配置节不是对象、对方写来的错误不合法、没有对应的 gRPC 状态 |
| `plugins.event.unsubscribed` | `id`、`topic`、`type`、`mode` | `KERNEL_NOTFOUND_001` | 投递找不到订阅 |
| `plugins.host.unavailable` | `id`、`reason` | `PLUGINS_UNAVAILABLE_001` | 回调连不上宿主、没有可用的回调端点 |
| `plugins.callback.denied` | `id`、`permission` | `PLUGINS_DENIED_002` | 回调所需的权限未声明或未授予 |
| `plugins.callback.tenant_denied` | `id`、`tenant` | `PLUGINS_DENIED_002` | 回调带了不属于在途调用的租户 |
| `plugins.callback.session_invalid` | 无 | `PLUGINS_DENIED_002` | 宿主拒绝了会话令牌 |

`id` 一律是插件的清单 ID。错误的 `details` 就是这些参数，不另加成员。
