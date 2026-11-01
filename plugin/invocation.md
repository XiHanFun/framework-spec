# 插件调用、事件与回调

> 规范性。协议 `xihan.plugin.v1`（`plugin.proto`，RFC-0017）。

## 1. 代理操作

握手声明的每个操作在宿主 `OperationRegistry` 登记为代理操作：契约取 `OperationDeclaration`（`id`、`stability`、`expose`、`permission` / `policy`），Schema 取 `request_schema` / `response_schema`。代理操作与本地操作无差别：协议适配器按 `expose` 发布（HTTP 路由、gRPC 方法、MCP tool、命令行、AI 工具），调用经宿主的完整管道，Authorize 阶段按契约判定；Handle 阶段把载荷与上下文发给插件。

- 插件定义的权限码（ADR-0080）：清单 `contributes.permissions` 声明的是插件**定义**的权限码（权限码文法同 RFC-0008，不重复；不合文法或重复的清单读不出，按发现时被拒处理，`PLUGINS_VALIDATION_001`）。宿主登记插件的代理操作时替它把这些码登记进 `PermissionRegistry`（显示名取权限码本身；插件可在握手的消息目录里给 `permissions.<码>.name` / `.description` 的译文）。只登记清单声明了的：操作的 `permission` 既不是宿主已登记的码、也不在本插件清单里，仍按未登记拒绝（`KERNEL_VALIDATION_005`）。要引用宿主或别的插件定义的码，只在操作的 `permission` 里写，不在清单里声明。只进程外与 Wasm 宿主登记；进程内插件宿主（`Plugins.InProcess`）不向应用的登记表贡献操作，也不登记权限码。
- 登记一个插件的贡献是一次批量登记（`OperationRegistry.AddRange(operations, permissions)`），全进或全不进：清单声明的权限码先进 `PermissionRegistry`（同组的操作可以引用它们），再核对与写入代理操作；任一项不过——声明的码与已登记的码重复（宿主特性定义的，或另一个已登记插件声明的，`KERNEL_SECURITY_001`）、超出权限登记表容量（`KERNEL_SECURITY_002`）、操作与已有操作重名（`KERNEL_OPERATION_001`）、超出操作登记表容量（`KERNEL_OPERATION_002`）、装配完成之后操作的 `permission` 未登记（`KERNEL_VALIDATION_005`）——整个插件登记失败，已写进的权限码撤回，不留下任何权限码与代理操作（ADR-0071、ADR-0080）。
- 冲突的处理：两个插件声明同一个码，后登记的那个整体失败；宿主特性定义的码与插件声明的码重复，不论谁先登记都是冲突（插件先登记时，宿主特性的 `AddPermission` 失败）。配置阶段的冲突都是启动失败。
- 宿主配置阶段发现的插件随应用一起装配：声明的权限码在配置阶段登记，冲突与重名启动失败（`KERNEL_SECURITY_001` / `KERNEL_OPERATION_001`），权限码未登记在装配完成时一并核对、启动失败（`KERNEL_VALIDATION_005`）。装配完成之后才登记的（首次握手失败后重试成功、运行期 `LoadAsync`）在登记时被拒：错误同上，插件按发现时被拒处理（`lifecycle.md` §1，`Failed`、健康条目、日志、不再重试），`LoadAsync` 把失败返回给调用方。
- 登记表只增不减：卸载（`UnloadAsync`）不撤回插件的权限码与代理操作，卸载后调用这些操作返回 `PLUGINS_UNAVAILABLE_001`。登记过的插件卸载后再 `LoadAsync`，与自己留下的登记重复而失败（`KERNEL_SECURITY_001` / `KERNEL_OPERATION_001`），换版本须重启宿主。
- `policy` 是策略名，调用时在宿主的策略目录里查找；不存在按 `SECURITY_NOTFOUND_001` 拒绝（RFC-0008）。

- 一元操作：`Plugin.Invoke`。`payload` 是请求类型的 JSON（空等价于 `{}`）；`InvokeResponse.status.ok` 为真时 `payload` 是响应类型的 JSON，否则 `error` 是业务失败。
- 流式操作（`streaming: true`）：`Plugin.InvokeStream`。调用方先发 `start`（`InvokeRequest`，`payload` 为请求对象），再发零个或多个 `chunk`（每帧一个完整 JSON 值），最后发 `end`（`ok: true`；`ok: false` 表示中止，被调方应停止处理）；被调方发零个或多个 `chunk`，最后发 `end`。任一方收到不符合顺序的帧（`start` 不是第一帧、`end` 之后还有帧）以 gRPC `InvalidArgument` 结束流。.NET 参考实现在 RFC-0011 定义流式操作契约之前只把流式操作暴露为内部调用，不经协议适配器发布。
- 调用到达时插件未 `Ready`：懒激活按 `lifecycle.md` 等待，否则 `PLUGINS_UNAVAILABLE_001`。

## 2. 上下文映射（RFC-0003）

| 字段 | `InvocationMetadata` | 说明 |
| --- | --- | --- |
| Trace | `trace_parent`、`trace_state` | 插件以其为父 Span |
| Deadline | `deadline` | RFC 3339 UTC；宿主同时把它设为 gRPC 调用的截止时间，二者相同 |
| Baggage | `baggage` | 只含已登记的 `xihan.*` 键（`conventions/baggage-keys.md`）；租户、主体、关联 ID、幂等键、区域都在其中 |
| Items / Services | — | 不传播 |

插件必须按 `deadline` 与 gRPC 取消停止工作；返回的 `Error` 若是超时或取消，用 `KERNEL_TIMEOUT_001` / `KERNEL_CANCELLED_001`。

## 3. 错误映射

- 业务失败走 `Status.error`，gRPC 状态保持 `OK`；`Error` 字段与 RFC-0001 的 JSON 编码一一对应（`details` 是 JSON 对象文本）。宿主用握手时得到的目录翻译 `message_key`；键不在目录里时文案为"键 (参数)"。
- gRPC 状态只表示传输或协议层失败，宿主映射为：

| gRPC 状态 | 宿主错误 |
| --- | --- |
| `Unavailable`、连接失败、进程已退出 | `PLUGINS_UNAVAILABLE_001` |
| `DeadlineExceeded` | `KERNEL_TIMEOUT_001` |
| `Cancelled` | `KERNEL_CANCELLED_001` |
| `Unimplemented`（插件未实现该 RPC 或操作） | `KERNEL_NOTFOUND_001` |
| `InvalidArgument`（帧序或载荷不合法） | `KERNEL_VALIDATION_001` |
| 其他 | `PLUGINS_UNEXPECTED_001`，细节含状态码与消息 |

- 宿主把插件错误原样放进调用结果；协议适配器再按各自载体映射（HTTP Problem Details、gRPC 状态 + `xihan-error-code`、MCP `isError`）。

## 4. 事件投递（RFC-0009）

- 宿主为握手声明的每个 `Subscription` 按它的 `mode` 向消息代理订阅主题：`competing`（缺省）时订阅了主题的宿主实例里只有一个把消息交给插件，`broadcast` 时每个在线的宿主实例都把消息交给自己的插件实例（尽力而为、不进死信，RFC-0009「订阅方式」）。消息到达时派发器按 RFC-0009 开新作用域、构造上下文、经完整管道，在 Handle 阶段调用 `Plugin.HandleEvent`，只投给以投递的方式订阅了该主题的插件。`message_type` 非空时只投递 `type` 相同的消息，其余在宿主侧按成功处理（不进插件）。
- `Envelope` 是 CloudEvents 1.0 属性的平铺；`attempt` 从 1 起。插件返回 `ok` 表示已处理；返回 `error` 由代理按重试与死信策略处理；插件不可用期间的投递同样按该策略处理。
- 至少一次投递：插件处理器必须幂等（可用 `baggage` 里的 `xihan.idempotency-key` 或信封 `id`）。

## 5. 回调（`Host` 服务）

每次回调带元数据 `xihan-plugin-session`（`handshake.md`）；宿主以插件身份执行：主体 ID 为清单 ID、认证方案 `Plugin`、无声明；回调元数据里的 `xihan.principal` 被忽略并覆盖，`xihan.tenant` 与其余已登记 Baggage 采用，追踪以回调的 `trace_parent` 为父。

| RPC | 权限 | 语义 |
| --- | --- | --- |
| `Host.Invoke` | `capability:kernel.pipeline` | 以新的调用经宿主完整管道执行操作（含 Authorize：插件主体须被授予操作要求的权限）；未登记的操作 `KERNEL_NOTFOUND_001` |
| `Host.Publish` | `capability:messaging.queue` | 经宿主消息代理发布，逐项结果；`topic` 为空的信封 `MESSAGING_VALIDATION_002` |

回调所需的权限必须**在清单 `permissions` 里声明并且已被宿主授予**才生效：清单是插件权限的唯一事实源（RFC-0017），宿主授予而清单没有声明的权限不放行回调。未声明或未授予都返回 gRPC `PermissionDenied` + `PLUGINS_DENIED_002`。`net.outbound:` 与 `fs.*:` 权限不经协议表达，由宿主通过操作系统沙箱（若启用）强制。

## 6. 日志与诊断

- 插件日志写 stderr，一行一条；宿主逐行转为日志（来源 `plugins.<清单 ID>`）。以 `{` 开头且能解析为 JSON 对象的行按结构化日志处理：`level`（`trace` / `debug` / `information` / `warning` / `error` / `critical`）、`message`、其余字段为作用域属性。
- 宿主为每次 `Invoke` / `InvokeStream` / `HandleEvent` 记录内核的 `xihan.invoke` Span 子 Span `xihan.plugin.invoke`，标签 `xihan.plugin.id`、`xihan.operation`。

## 7. 一致性

gRPC 黑盒套件 `XiHan.Framework.Plugins.Conformance`（`PluginProtocolConformance`，任何语言的插件以其构建产物目录继承运行）以本文件、`handshake.md` 与 `lifecycle.md` 为用例来源：握手行与版本协商、无共同版本时拒绝、清单一致性、Configure / Start / Health / WatchHealth / Stop 与宽限内退出、未知操作 `Unimplemented`、声明的操作都返回 `status`、订阅投递确认、stdin 关闭即退出。回调的令牌与权限、崩溃重启由宿主实现的测试覆盖；流式帧序随流式操作契约补充。数据向量见 `conformance/vectors/plugin/`。
