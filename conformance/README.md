# 一致性：向量与运行器

Go 模块 `github.com/xihanfun/framework-spec/conformance`。

## 一致性档（`profiles.json`）

规范性内容按档划分（RFC-0024）：每个档列出规范文件、必须回放的向量与必须通过的运行器子命令，实现认领若干档、只对认领的档负责。Go 经 `conformance.Profiles()` 读取，`RequiredVectors(档…)` 给出认领这些档必须回放的向量；其他语言读 `spec/conformance/profiles.json`。每个向量文件至少属于一个档（本模块的测试核对），每个规范文件至少属于一个档、RFC 状态行写的档与档表一致（`tools/speccheck` 核对）。

## 向量（`vectors/`）

| 文件 | 内容 | 规范 |
| --- | --- | --- |
| `primitives/error-codes.json` | 错误码格式的合法与非法样例 | `conventions/error-codes.md` |
| `primitives/baggage-keys.json` | Baggage 键格式、已登记的键与值能否进入调用上下文（`values`） | `conventions/baggage-keys.md` |
| `primitives/capability-requirements.json` | 能力依赖文本的解析结果 | `conventions/capability-ids.md` |
| `primitives/lifecycle-transitions.json` | 生命周期合法迁移的完整列表 | RFC-0001 |
| `primitives/wire-encoding.json` | Result 与 Error 线上编码的合法写法（连同规范形式）与必须拒绝的写法 | RFC-0001「Result」 |
| `primitives/message-templates.json` | 消息键文法、模板的宽松与严格格式化（含值模型、数组与对象、可移植的格式说明符与舍入、区域的数字符号）、`Describe` | RFC-0010「模板」 |
| `primitives/operation-ids.json` | 操作 ID 文法的合法与非法样例，含 128 个字符的上限两侧 | `conventions/operation-ids.md`、RFC-0011「操作 ID」 |
| `primitives/kernel-messages.json` | SDK 与宿主共用的内核错误：错误码、消息键、参数名与 en-US 模板 | RFC-0011「错误码」 |
| `primitives/trace-context.json` | `traceparent` 的合法与不合法文本、解析结果与写出 | RFC-0003 规则 7 |
| `primitives/json-text.json` | XiHan 读取 JSON 文本的共同检查：合法、语法错误与重复成员（含路径），期望值由 .NET 算出 | RFC-0001「JSON 文本」 |
| `manifest/manifests.json` | 必须读得出的清单与再写出的全文，必须拒绝的清单与逐字的错误（检查顺序、路径、跨字段约束、操作 ID 的 128 个字符上限），期望值由 .NET 算出 | RFC-0002 解析规则 2 与「写出」 |
| `pipeline/ordering.json` | 管道的登记序列、期望的 `Dump` 全文与各种错误报出的时机（期望值由 .NET 算出） | RFC-0004 |
| `operations/payload-reasons.json` | 载荷的解码结果与不合法时 `reason` 的路径（三个各语言都能声明的请求类型，含必填成员与 `null`，期望值由 .NET 算出） | RFC-0011 规则 2 |
| `plugin/handshake-lines.json` | 插件握手行的合法与非法样例：地址的空白字符集（与 .NET 正则的 `\s` 相同）、tcp 回环地址与端口的写法 | `plugin/handshake.md` §2 |
| `plugin/host-addresses.json` | 宿主回调端点列表 `XIHAN_PLUGIN_HOST_ADDRESS` 的合法与非法写法，以及按插件支持的 network 选第一个可用项 | `plugin/handshake.md` §4 |
| `plugin/deadlines.json` | 截止时间文本的严格读法（UTC 时刻，纳秒精度；`InvocationMetadata.deadline` 与 `xihan-deadline` 头共用）、与 gRPC 截止时间取较早者、按 .NET `O` 格式写出 | RFC-0003 规则 8、`plugin/invocation.md` §2 |
| `plugin/plugin-messages.json` | SDK 运行时与宿主共用的插件消息键：错误码、参数（即 `details` 的成员）、en-US 模板与固定的 `reason` 文本 | `plugin/invocation.md` §8 |
| `plugin/callback-status.json` | 插件一侧回调失败的映射：RPC、gRPC 状态与调用上下文是否已取消，到错误码、消息键与 `details` | `plugin/invocation.md` §5 |
| `plugin/error-reading.json` | 读入对方写来的 `Error`：错误码与消息键不合文法、`details` 不合法或溢出、`inner`、`ok` 为假却没有错误 | `plugin/invocation.md` §3 |
| `plugin/protocol-versions.json` | 协议版本协商与原语层版本核对 | `plugin/handshake.md` |
| `plugin/wasm-exports.json` | Wasm ABI v1 的导出与导入表 | `plugin/wasm.md` |
| `caching/cache-keys.json` | 缓存键与键前缀必须接受与必须拒绝的样例：长度按 UTF-8 字节计（正好 512 字节、513 字节、UTF-16 码元数不超而字节数超过）、空白与控制字符 | RFC-0006「存储契约」、`provider/caching-distributed.md` §2 |
| `context/http-deadline.json` | HTTP 载体从请求头读出的截止时间：`xihan-deadline` 合文法与读不出的写法（含 .NET 以前照收的宽松写法、出现多次）、没有它时的 `grpc-timeout`，期望值由 .NET 算出 | RFC-0003 规则 8、`protocol/http.md`、`protocol/grpc.md` |
| `messaging/topics.json` | 消息主题文法的合法与不合法样例（含 128 个字符的上限两侧）、死信后缀的缺省值与死信主题 | RFC-0009「信封」 |
| `messaging/envelope-extensions.json` | CloudEvents 扩展属性名的文法，扩展属性与 Baggage 键的对应，承载追踪与分区键的扩展，不进入消息的 Baggage 键 | RFC-0009「信封」、`conventions/baggage-keys.md` |
| `messaging/instance-ids.json` | 消息子系统实例标识文法的合法与不合法样例（含 64 个字符的上限两侧） | RFC-0009「订阅方式」第 6 条 |
| `messaging/outbox-backoff.json` | 发件箱失败之后的退避间隔（次数很大时截在上限而不溢出）与进死信的时机，期望值由 .NET 算出 | RFC-0009「发件箱」 |
| `messaging/cloudevents-envelopes.json` | 信封的 CloudEvents 结构化 JSON 编码：必须读得出的文档与读出的信封，必须拒绝的文档，期望值由 .NET 算出 | RFC-0009「信封」、`protocol/cloudevents.md` |
| `multitenancy/tenant-ids.json` | 租户 ID 文法的合法与不合法样例（只用小写，含 64 个字符的上限两侧），期望值由 .NET 算出 | RFC-0007「租户 ID」 |
| `multitenancy/tenant-resolution.json` | Tenant 阶段的裁定：声明、Baggage、默认租户与平台调用的先后，声明冲突，文法与存储的核对次序，到租户、无租户或错误码，期望值由 .NET 算出 | RFC-0007「解析」 |
| `multitenancy/tenant-shards.json` | 从租户属性推出分片：连接串、小写的提供者名与去前缀的透传属性，期望值由 .NET 算出 | RFC-0007「分片」 |
| `objectstorage/container-names.json` | 容器名文法的合法与不合法样例（3 到 63 个字符的两侧），期望值由 .NET 算出 | RFC-0012「标识」 |
| `objectstorage/blob-keys.json` | 对象键的文法（分段、`.` 与 `..`、空白、控制字符、反斜线、1024 个字符的上限两侧）与保留前缀 `.xihan`，期望值由 .NET 算出 | RFC-0012「标识」 |
| `objectstorage/blob-metadata.json` | 自定义元数据：名的文法，值里的控制字符，名与值合计 2048 个 UTF-8 字节的上限两侧（含码元数不超而字节数超过的），期望值由 .NET 算出 | RFC-0012「标识」 |
| `objectstorage/blob-content-types.json` | 对象内容类型文法的合法与不合法样例（token 字符、参数、255 个字符的上限两侧），期望值由 .NET 算出 | RFC-0012「标识」 |
| `objectstorage/blob-ranges.json` | 范围读取：起点、可选的长度与总长，到实际范围（超出末尾截断）或不可满足，期望值由 .NET 算出 | RFC-0012「范围读取、分片上传与预签名」 |
| `objectstorage/multipart-parts.json` | 分片编号的范围（1–10000）、分片的最小字节数与完成清单的编号次序，期望值由 .NET 算出 | RFC-0012「范围读取、分片上传与预签名」 |
| `objectstorage/store-definitions.json` | 命名存储实例：实例名的文法、内置的 Provider 名、两个定义是不是同一个定义，期望值由 .NET 算出 | RFC-0012「命名存储实例」 |
| `scheduling/job-names.json` | 作业名、触发器名与领导选举资源名的文法（与主题相同，含 128 个字符的上限两侧），期望值由 .NET 算出 | RFC-0013「作业与触发器」 |
| `scheduling/cron-expressions.json` | cron 表达式：5、6、7 段，列表、范围、步长、名字、`?`、`L`、`L-n`、`nW`、`LW`、`nL`、`n#k`，从起点依次推算的触发时刻（含时区与夏令时的跳过、重复），搜索上限之内没有的，必须拒绝的表达式；期望值由 .NET 算出 | RFC-0013「作业与触发器」 |
| `scheduling/retry-delays.json` | 作业重试前的等待（固定间隔与指数退避、上限、30 天）与重试策略的取值范围，期望值由 .NET 算出 | RFC-0013「作业与触发器」 |
| `scheduling/event-topics.json` | 调度发布的 9 个事件主题与信封的 `source` | RFC-0013「引擎」 |
| `localization/locale-names.json` | 区域名的文法（35 个字符的上限两侧）、规范形式与语言子标签，内置的 10 种区域，期望值由 .NET 算出 | RFC-0010「区域」 |
| `localization/locale-negotiation.json` | 区域的协商（精确命中、按语言命中、默认区域，不合文法的候选忽略）与取文案的回退链，期望值由 .NET 算出 | RFC-0010「协商与宿主」「解析」 |
| `localization/catalog-resolution.json` | 消息目录的合并（同名取并集、同一键同一区域不同即冲突）与按回退链的查找（文案与实际命中的区域），期望值由 .NET 算出 | RFC-0010「语言文件」「解析」 |
| `configuration/secret-names.json` | 密钥名文法的合法与不合法样例（`..`、`//`、结尾的 `/`，256 个字符的上限两侧），期望值由 .NET 算出 | RFC-0014「密钥」 |
| `configuration/remote-documents.json` | 远程配置文档展平成配置键值（JSON 按 `:` 展平、数组按下标、`null` 保留；properties 的 `.` 换成 `:`、注释行）与 Nacos 的内容版本（UTF-8 的 MD5），期望值由 .NET 算出 | RFC-0014「远程配置」「Nacos Provider」 |
| `persistence/index-names.json` | 索引名的推算：前缀、表名、每列的缩写（按词取前两个字符），63 个字符的上限两侧，显式名字的文法；期望值由 .NET 算出 | RFC-0005「索引声明」 |
| `persistence/snowflake-ids.json` | 雪花标识的位布局：由毫秒数、机器位与序列合成标识，由标识还原三段与时刻（含超过 2^53 的值），超出范围的三段；期望值由 .NET 算出 | RFC-0005「雪花标识」 |
| `auditing/exclusions.json` | 审计排除列表的项（操作 ID 或以 `*` 结尾的前缀）合法与不合法的样例，操作是否命中排除列表；期望值由 .NET 算出 | RFC-0015「事件的产生」 |
| `auditing/event-selection.json` | 一次调用是否记成审计事件：选择规则、显式动作、排除、结果的来源与 `IncludeSuccesses`，事件的主体与租户取自哪里；期望值由 .NET 算出 | RFC-0015「事件的产生」 |
| `security/webauthn.json` | WebAuthn 注册与认证的样例响应、存储的凭据与期望结果（成功的凭据与签名计数，失败的错误码与 `reason`） | RFC-0008「WebAuthn」 |
| `provider/caching-distributed.json` | `caching.distributed@1` 远程 Provider 的操作 ID（含批量）、请求 / 响应样例、非法键与选项 | `provider/caching-distributed.md` |

各语言 SDK 的单元测试直接读取这些文件。Go 里 `go get github.com/xihanfun/framework-spec/conformance` 后经内嵌文件系统 `conformance.Vectors` 或类型化入口（`conformance.HandshakeLines()` 等）读取，不需要 submodule。`conformance.TypedVectors()` 给出向量文件表：每个向量文件一项，带它的类型化读取；`xihan-conformance vectors` 照这张表逐个解析，本模块的测试核对表与内嵌的文件一一对应。

## 规则（`rules/`）

规范文法与判定的 Go 实现，供运行器使用，也供 SDK 作者对照：错误码、操作 ID、主题、Baggage 键与值、清单 ID、清单的激活事件（`IsActivationEvent`）、能力 ID 与依赖文本、缓存键与键前缀（`IsCacheKey`、`IsCacheKeyPrefix`，长度按 UTF-8 字节计）、握手行与端点（`ParseHandshakeLine`、`ParseEndpoint`、`IsLoopbackAddress`，空白字符集 `IsWhitespace`）、宿主回调端点列表（`ParseHostAddresses`、`SelectHostEndpoint`、`FormatHostAddresses`）、截止时间（`ParseDeadline`、`FormatDeadline`、`EffectiveDeadline`，HTTP 载体的 `HTTPDeadline` 与 `ParseGRPCTimeout`）、回调失败的映射（`MapCallbackStatus`、`GRPCStatusName`、`StatusReason`、`CallbackPermission`、`CallbackPhase`）、读入对方写来的 `Error` 与 `Status`（`ReadProtoError`、`ReadProtoStatus`、`ReadDetails`）、版本协商、生命周期迁移、管道阶段、消息键与模板占位符、Result 与 Error 的线上编码（`CanonicalResult` / `CanonicalError` 给出再写出时的规范形式）、JSON 文本的共同读取检查（`CheckJSONText`，与 .NET 独立实现、回放同一份向量）。每条规则都由向量验证（`rules/rules_test.go`、`rules/messages_test.go`、`rules/plugin_test.go`）；`plugin-messages.json` 另核对模板占位符与参数一致，`callback-status.json` 的 `details` 成员与所用消息键的参数一致。模板的格式化结果由参考实现 .NET 回放 `message-templates.json` 核对，规则包不另实现一份。

子系统档的规则各在一个文件里，由同名的 `rules/*_test.go` 回放对应目录的向量：

| 文件 | 规则 |
| --- | --- |
| `rules/messaging.go` | 死信主题（`DeadLetterTopic`）、扩展属性名与 Baggage 键的对应（`IsEnvelopeExtensionName`、`EnvelopeBaggageMappings`）、实例标识（`IsMessagingInstanceID`）、发件箱的退避与进死信（`OutboxRetryDelay`、`OutboxDeadLettered`）、结构化 JSON 信封的读取（`DecodeCloudEvent`）；主题文法是 `IsTopic` |
| `rules/multitenancy.go` | 租户 ID（`IsTenantID`）、Tenant 阶段的裁定（`ResolveTenant`）、从租户属性推出分片（`TenantShardFromProperties`） |
| `rules/objectstorage.go` | 容器名（`IsContainerName`）、对象键与保留前缀（`IsBlobKey`、`IsReservedBlobKey`）、自定义元数据（`IsBlobMetadataName`、`IsBlobMetadata`，合计按 UTF-8 字节计）、内容类型（`IsBlobContentType`）、范围读取（`ResolveBlobRange`）、分片编号与完成清单（`IsBlobPartNumber`、`IsBlobPartManifest`）、命名存储实例（`IsBlobStoreName`、`IsBlobStoreProvider`、`SameBlobStoreDefinition`） |
| `rules/scheduling.go`、`rules/cron.go` | 调度里的名字（`IsJobName`）、重试的等待与取值范围（`JobRetryWait`、`IsJobRetryPolicy`）、事件主题（`SchedulingEventTopics`）；cron 的解析与下一次触发（`ParseCron`、`CronExpression.Next`、`IsCronExpression`，与 .NET 独立实现、回放同一份向量） |
| `rules/localization.go` | 区域名（`IsLocaleName`、`NormalizeLocaleName`、`LocaleLanguage`）、协商与回退链（`NegotiateLocale`、`LocaleFallbackChain`）、目录的合并与查找（`MergeMessageCatalogs`、`FindMessage`）；消息键与模板在 `rules/messages.go` |
| `rules/configuration.go` | 密钥名（`IsSecretName`）、远程配置文档的展平（`FlattenJSONConfiguration`、`FlattenPropertiesConfiguration`）与 Nacos 的内容版本（`ConfigurationContentVersion`） |
| `rules/persistence.go` | 索引名（`IndexColumnAbbreviation`、`IndexName`、`IsExplicitIndexName`）、雪花标识（`ComposeSnowflakeID`、`ParseSnowflakeID`、`SnowflakeTime`、`SnowflakeWorkerLockResource`） |
| `rules/auditing.go` | 排除列表（`IsAuditExclusion`、`AuditExcludes`）、事件的选择与结果（`SelectAuditEvent`）、主体与租户的归属（`AuditAttribution`） |

## 黑盒运行器（`cmd/xihan-conformance`）

以宿主身份启动任意语言写的进程外插件，按 `plugin/handshake.md`、`lifecycle.md`、`invocation.md` 核对：

| 用例 | 核对内容 |
| --- | --- |
| `handshake` | 握手行合法、协商出宿主支持的版本、原语层版本相同、握手清单与目录里的文件一致且不以 BOM 开头、操作与订阅都在清单声明范围内、目录至少一种区域；插件写的 `primitives` 等于清单的 `primitives`，没有 `streaming: true` 的操作，每个订阅的 `mode` 是 `competing` 或 `broadcast`（不写空）（`handshake.md` §3） |
| `refusal` | 宿主没有共同版本时以 `FailedPrecondition` 拒绝 |
| `session` | 不带会话令牌、带错令牌，或带两个 `xihan-plugin-session` 值（一对一错，两种先后；两个都对）的调用都以 `Unauthenticated` 拒绝，带对了照常握手（`handshake.md` §4，ADR-0202、ADR-0237） |
| `lifecycle` | `Configure`、`Start` 成功；`Health` 与 `WatchHealth` 报 `Ready` 或 `Degraded`（`Start` 应答之前已按健康回调的结果定下状态，不会再报 `Started`，`lifecycle.md` §7.3），健康项都已声明；`Stop` 成功且进程在 5 秒宽限内退出 |
| `order` | 调用次序（`lifecycle.md` §7.1）：握手前的 `Configure` 与 `Stop`、再次 `Handshake`、没 `Configure` 就 `Start`、再次 `Configure`、再次 `Start` 都以 `FailedPrecondition` 拒绝；握手后 `Health` 报 `Resolved`、`Configure` 后报 `Configured`，都没有健康项；`Start` 之后报 `Ready` 或 `Degraded`（`Failed` 不通过，§7.3） |
| `unknown-operation` | 未声明的操作以 `Unimplemented` 结束 |
| `declared-operations` | 每个声明的一元操作都返回 `status`：成功时载荷是 JSON，失败时错误码合法 |
| `unavailable` | `Configure` 之后、`Start` 之前：未声明的操作仍以 `Unimplemented` 结束；每个声明的操作（`--exclude` 的除外）与对第一个订阅的投递都以 `PLUGINS_UNAVAILABLE_001` + `plugins.process.unavailable` 失败，`text` 为空，`details` 为 `{"id":<清单 ID>,"state":"Configured"}`（`invocation.md` §1、§4，`lifecycle.md` §7.3） |
| `event` | 向第一个声明的订阅投递消息得到确认；没有订阅时跳过 |
| `unsubscribed` | `Start` 之后向没人订阅的随机主题投递：`mode` 为 `competing` 或空都以 `KERNEL_NOTFOUND_001` + `plugins.event.unsubscribed` 失败，`text` 为空，`details` 为 `{"id","topic","type","mode":"competing"}`；`mode` 为 `bogus` 与没有 `envelope` 都以 `InvalidArgument` 拒绝（`invocation.md` §4） |
| `stream` | `InvokeStream` 第一帧是 `chunk`、不发帧就关闭发送方向都以 `InvalidArgument` 结束；第一帧是 `start`（未声明的操作，以及第一个声明的操作）以 `Unimplemented` 结束（`invocation.md` §1） |
| `configuration` | 配置节为 `[]` 与语法错误的 `{"a":` 时各开一个进程：`Configure` 以 `PLUGINS_UNEXPECTED_001` + `plugins.process.failed` 失败，`text` 为空，`details` 为 `{"id","phase":"configure","reason":"the configuration is not a JSON object"}`；之后 `Health` 报 `Failed`，`Stop` 应答 `ok`、进程在宽限内以退出码 0 退出（`lifecycle.md` §7.2、§7.5） |
| `stop-early` | 握手后直接 `Stop`（`grace_seconds` 为 0）应答 `ok`，进程在 5 秒内以退出码 0 退出（`lifecycle.md` §7.5） |
| `stdin` | 关闭 stdin 后进程自行退出 |

每个用例单独启动一次插件进程；运行器调用插件时每次都带会话令牌，`details` 按 JSON 值比较。运行器给插件的 `XIHAN_PLUGIN_HOST_ADDRESS` 与 .NET 宿主一样是两项的列表、回环 TCP 在最后（Windows 上是 `npipe \\.\pipe\xihan-conformance-<随机>;tcp 127.0.0.1:<端口>`，其他系统是 `unix <临时目录>/xihan-conformance-<随机>/host.sock;tcp 127.0.0.1:<端口>`），每个用例都考到插件解析列表；两项都没人监听：运行器不提供宿主回调服务，回调的令牌、权限、本地核对与状态映射由宿主与 SDK 实现的测试覆盖（回放 `callback-status.json`），读入对方写来的错误同样由实现回放 `error-reading.json`。

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
xihan-conformance plugin --dir ./bin/echo --exclude echoes.crash --sample 'echoes.create={"text":"hi"}'
xihan-conformance plugin --dir ./bin/echo --json
xihan-conformance vectors
```

| 选项 | 含义 |
| --- | --- |
| `--dir` | 插件目录（含 `xihan.manifest.json` 与 `entry` 指向的可执行文件） |
| `--timeout` | 每一步的截止时间，默认 `15s` |
| `--config` | `Configure` 下发的配置节 JSON，默认 `{}` |
| `--sample op=json` | 某个操作的请求样例；没有的操作用 `{}` |
| `--exclude op` | 不调用的操作（如会故意崩溃的操作） |
| `--case name` | 只跑指定用例，可重复 |
| `--json` | 结果输出为 JSON |
| `--quiet` | 不转发插件的 stderr |

退出码：`0` 全部通过（含跳过），`1` 有用例失败，`2` 参数或目录错误。

`xihan-conformance version` 打印运行器的版本，握手时作为宿主 SDK 版本上报的也是它（`plugin.Version`）：取自构建信息里本模块的版本，不带前缀 `v`，`go install …@<版本>` 装的就是那个版本；在仓库里直接构建或测试时取不到，是 `devel`。

作为库使用：`plugin.Run(ctx, plugin.Options{Directory: dir})` 返回每个用例的 `Result`；测试 `TestAgainstAPluginDirectory` 在设置了 `XIHAN_CONFORMANCE_PLUGIN_DIR`（和可选的 `XIHAN_CONFORMANCE_EXCLUDE`）时对该目录跑全部用例。运行器自己的测试（`plugin/fake_test.go`）让测试程序扮演插件：合规的假插件必须通过全部用例，按以前各 SDK 的做法应答的假插件必须在新用例上失败。

## 生成代码

`internal/pluginv1` 由仓库根的 `buf generate` 从 `plugin/plugin.proto` 生成并提交；改协议后重新生成，CI 核对生成物与源一致。
