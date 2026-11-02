# 错误码

```text
ErrorCode = Domain "_" Kind "_" Sequence
Domain    = 术语表错误码域（大写）或应用自定义的大写标识
Kind      = VALIDATION | NOTFOUND | CONFLICT | DENIED | UNAVAILABLE | TIMEOUT | CANCELLED | UNEXPECTED | 领域自定义
Sequence  = 三位数字，域内唯一，一经发布不复用
```

示例：`PERSISTENCE_CONFLICT_001`、`SECURITY_DENIED_003`、`KERNEL_CAPABILITY_001`。

## 规则

1. 框架错误码在本文登记，新增走 RFC 附件。
2. 错误码进入 `Error.Code`，HTTP 适配器把它放进 RFC 9457 Problem Details 的 `type` 或扩展字段，gRPC 放进 trailing metadata，插件协议放进 `Status.error.code`（`spec/plugin/plugin.proto`）。
3. 消息面向人，错误码面向程序；客户端只应依赖错误码。
4. 文案不写在代码里：错误携带消息键（`<领域>Messages` 常量）与参数（`Error.Details`），文案在所属包的 `Localization/<区域>.json`（10 种区域各一个文件，RFC-0010）；一个错误码可以对应多个消息键。

## 已登记的框架错误码

| 错误码 | 含义 |
| --- | --- |
| `PRIMITIVES_VALIDATION_001` | 原语参数非法（空 Code、空 Message、清单跨字段约束等） |
| `PRIMITIVES_VALIDATION_002` | 使用了未初始化的 Result（default 值） |
| `KERNEL_CAPABILITY_001` | 能力解析失败：缺失依赖 |
| `KERNEL_CAPABILITY_002` | 能力解析失败：循环依赖 |
| `KERNEL_CAPABILITY_003` | 能力解析失败：版本约束冲突 |
| `KERNEL_CAPABILITY_004` | 同一能力有多个候选且未指定 |
| `KERNEL_CAPABILITY_005` | 清单 ID 重复 |
| `KERNEL_PIPELINE_001` | 中间件顺序约束成环 |
| `KERNEL_PIPELINE_002` | 阶段名未知 |
| `KERNEL_PIPELINE_003` | 中间件重复注册，或顺序约束引用了未注册或跨阶段的中间件；运行期改动（RFC-0004）：成组登记混了别的归属或把约束挂到组外、按归属撤回会留下指向被撤回中间件的约束、应用构建之后单项改动或按类型登记 |
| `KERNEL_HOSTING_001` | 必需绑定缺失或类型不匹配 |
| `KERNEL_HOSTING_002` | 特性启动或停止失败 |
| `KERNEL_HOSTING_003` | 特性停止超时 |
| `KERNEL_HOSTING_004` | 应用状态不允许该操作（重复构建、未就绪时调用） |
| `KERNEL_UNAVAILABLE_001` | 应用未就绪或正在停止，暂不接收调用（HTTP 503） |
| `KERNEL_UNAVAILABLE_002` | 操作被限流：固定窗口的许可用尽且队列已满；Resilience 阶段拒绝，`retryAfter` 秒后重试 |
| `KERNEL_UNAVAILABLE_003` | 操作已达并发上限且队列已满；Resilience 阶段拒绝 |
| `KERNEL_UNAVAILABLE_004` | 操作的熔断器打开，暂不执行；Resilience 阶段拒绝，`retryAfter` 秒后重试 |
| `KERNEL_UNAVAILABLE_005` | 健康探针失败或超时（`PeriodicHealthContributor`：条目 Degraded，`name`、`reason`） |
| `KERNEL_TIMEOUT_001` | 调用超过截止时间；截止令牌已触发时下游抛出或写下的取消都报为它，调用方同时取消也一样（RFC-0004，2026-11-01 起） |
| `KERNEL_CANCELLED_001` | 调用在截止时间到达之前被调用方取消（客户端断开、调用方的取消令牌） |
| `KERNEL_UNEXPECTED_001` | 调用期间发生未处理异常 |
| `KERNEL_SECURITY_001` | 权限码重复：特性重复登记，或插件清单声明的权限码与已登记的重复（宿主特性定义的或另一个插件声明的；插件整体登记失败，ADR-0080） |
| `KERNEL_SECURITY_002` | 权限注册表已满 |
| `KERNEL_NOTFOUND_001` | 操作未登记（含插件卸载时按归属撤回了的代理操作，ADR-0085） |
| `KERNEL_VALIDATION_001` | 操作载荷不合法（非 JSON、null 或类型不匹配） |
| `KERNEL_VALIDATION_002` | 类型化调用的请求或响应类型与登记不符 |
| `KERNEL_VALIDATION_003` | 请求不满足操作契约的 JSON Schema（类型、必填、枚举、范围、长度、模式、元素数）；Validate 阶段拒绝，不进 Handle |
| `KERNEL_VALIDATION_004` | 请求对象的自校验（`IValidatableObject`）失败；Validate 阶段拒绝，不进 Handle |
| `KERNEL_VALIDATION_005` | 操作契约引用了未登记的权限码：装配完成时以 `FrameworkException` 抛出、启动失败（一次列出全部）；装配之后登记的操作（插件握手之后）在登记时返回，插件整体登记失败（RFC-0008，ADR-0071） |
| `KERNEL_OPERATION_001` | 操作 ID 重复登记 |
| `KERNEL_OPERATION_002` | 操作登记表已满 |
| `PLUGINS_VALIDATION_001` | 插件清单、种类或隔离级别不合法，或清单声明了该隔离级别不支持的贡献（进程外与进程内插件写了 `contributes.pipeline`：只有 Wasm 插件能贡献中间件） |
| `PLUGINS_VALIDATION_002` | 插件入口类型未实现 IPlugin，或实例清单与文件清单不一致 |
| `PLUGINS_NOTFOUND_001` | 插件清单文件、程序集或入口类型不存在 |
| `PLUGINS_DENIED_001` | 插件声明的权限未被宿主授予 |
| `PLUGINS_CONFLICT_001` | 同一清单 ID 的插件已加载 |
| `PLUGINS_UNEXPECTED_001` | 插件加载、配置、启动或停止时发生未处理异常 |
| `PLUGINS_VALIDATION_003` | 进程外：握手行不合法，或协议版本没有交集 |
| `PLUGINS_VALIDATION_004` | 进程外：握手响应与清单不一致（原语层版本、清单字段、未声明或重复的操作 / 健康项 / 订阅） |
| `PLUGINS_DENIED_002` | 插件回调被拒：会话令牌无效（进程外），或所需权限未在清单声明、未被宿主授予（进程外与 Wasm） |
| `PLUGINS_UNAVAILABLE_001` | 进程外：插件进程未就绪、正在停止、已退出或不可达 |
| `PLUGINS_TIMEOUT_001` | 进程外：握手、配置、启动或停止超时 |
| `PLUGINS_UNEXPECTED_002` | 进程外：插件进程异常退出；Wasm：陷阱或导出函数返回 0 |
| `PLUGINS_VALIDATION_005` | Wasm：模块不合法（不是 reactor、ABI 版本不支持、缺少必需导出或签名不符、导入了宿主不提供的函数） |
| `PLUGINS_UNAVAILABLE_002` | Wasm：沙箱资源不足（回调嵌套超限、客体内存分配失败） |
| `PLUGINS_UNAVAILABLE_003` | Wasm：宿主进程不满足载体要求（Windows 强制用户态影子栈 / CET 时陷阱会终止进程，宿主可执行项目须设 `CETCompat=false`） |
| `PERSISTENCE_VALIDATION_001` | 多租户实体在没有当前租户的调用中新增 |
| `PERSISTENCE_VALIDATION_002` | 工作单元状态不允许该操作：重复开始、未开始就提交（以异常抛出）；EF Core 作用域里的 DbContext 已按别的连接建出后再绑定命名连接（`BeginAsync` 以异常抛出）；同一服务作用域里再经管道发起的嵌套调用（Transaction 阶段以结果结束内层调用，不执行处理器） |
| `PERSISTENCE_VALIDATION_003` | 工作单元里的聚合有未派发的领域事件，但没有注册领域事件派发器 |
| `PERSISTENCE_VALIDATION_004` | 实体落点不成立（装配错误，以异常抛出）：SqlSugar 实体的 `[Tenant(configId)]` 或登记时指定的命名连接不在客户端里、两者指向不同的连接（缺省落点规则下启动时校验）；工作单元绑定的连接不在客户端里（`BeginAsync`），或绑定连接的工作单元里操作的实体不落在那个连接上；EF Core 实体登记到、或工作单元绑定没有在 `XiHan:Persistence:EntityFrameworkCore:Connections` 声明的连接（装配时、`BeginAsync`），绑定连接的工作单元里取用落在别处的实体 |
| `PERSISTENCE_VALIDATION_005` | 查询的取出条数超过 `EntityQuery.DefaultMaxTake`（1000）：构造查询时以异常抛出，调用里由 Transaction 阶段转为结果（HTTP 400，参数 `take`、`max`）；库结构的表列表（`TableListQuery.Take`）超过上限由读结构的 Provider 以结果返回（不在构造时抛出）；一次读结构的表数超过 `SchemaLimits.MaxTablesPerRead`（100）以结果返回（参数 `count`、`max`）；仓储查询的 `Skip` 为负、`Take` 小于 1 是编程错误，抛 `ArgumentOutOfRangeException`，不产生错误码（表列表的见 `PERSISTENCE_VALIDATION_007`） |
| `PERSISTENCE_VALIDATION_006` | 运行时数据源的定义不合法：数据库类型 Provider 不认识、连接串格式错（`DataSourceRegistry` 的取用与测试连接；同名的已登记实例不受影响） |
| `PERSISTENCE_VALIDATION_007` | 库结构表列表的分页越界：`TableListQuery.Skip` 为负或 `Take` 小于 1（参数 `skip`、`take`；不在构造时抛出，读结构的 Provider 以结果返回，HTTP 400） |
| `PERSISTENCE_NOTFOUND_001` | 更新或删除的实体不存在（含已软删除、不属于当前租户） |
| `PERSISTENCE_NOTFOUND_002` | 运行时数据源没有在本进程登记（`DataSourceRegistry.GetAsync`） |
| `PERSISTENCE_NOTFOUND_003` | 库里没有要读结构的表（`ISchemaReader.GetTablesAsync` 的逐项结果，参数 `dataSource`、`table`） |
| `PERSISTENCE_CONFLICT_001` | 乐观并发冲突：并发戳不匹配；内存实现提交时发现更新或删除的行已被别的工作单元删除或改过 |
| `PERSISTENCE_CONFLICT_002` | 新增的标识已存在；含并发新增：检查之后另一个工作单元提交了同一标识（关系型在写入时、内存实现在提交时返回） |
| `PERSISTENCE_CONFLICT_003` | 唯一键重复：写入撞上实体声明的唯一索引（`[EntityIndex(IsUnique = true)]` 等）——同一次写入里两项同键，或库里另一行（标识不同）同键；含更新把键改成已有的值。软删除实体的唯一索引只约束未删除的行（部分唯一索引，`WHERE IsDeleted = false`）：删除不会撞上，新增或更新的未删除行撞上另一未删除的同键行才返回；唯一索引的列里显式写了删除标记时按普通唯一索引约束（同一键最多一行已删除，软删除也会撞上）。关系型 Provider 在写入失败后核对，这次写入的项统一报告，工作单元随后只能回滚；内存实现不建索引、不返回它 |
| `PERSISTENCE_UNAVAILABLE_001` | 没有活动的工作单元（仓储在 Transaction 阶段之外使用；以异常抛出） |
| `PERSISTENCE_UNAVAILABLE_002` | 存储容量已满（内存实现的每类实体上限） |
| `PERSISTENCE_UNAVAILABLE_003` | 存储不可达 |
| `PERSISTENCE_UNAVAILABLE_004` | 雪花标识生成器的时钟回拨超过容忍阈值（以异常抛出；调用里由 `SnowflakeIdFeature` 的中间件转为结果，HTTP 503） |
| `PERSISTENCE_UNAVAILABLE_005` | 雪花标识生成器没有可用的机器位：未配置且容器里没有分布式锁、1024 个机器位都被占用、租约尚未取得或已丢失（启动期与生成时以异常抛出，调用里由 `SnowflakeIdFeature` 的中间件转为结果、HTTP 503，健康项以结果上报） |
| `PERSISTENCE_UNAVAILABLE_006` | 数据源连不上或读取失败：运行时数据源的测试连接、健康探测（条目 Degraded），缺省连接与运行时数据源的库结构读取（含 Oracle 11g 及更早版本上的读列：数据字典没有 `IDENTITY_COLUMN`，不支持）；参数 `name`、`reason`（驱动消息，连接串与口令类键的值已换成 `***`） |
| `PERSISTENCE_UNAVAILABLE_007` | 运行时数据源登记已满（`XiHan:Persistence:DataSources:MaxCount`）；不淘汰已有的，替换同名的不占名额 |
| `PERSISTENCE_UNAVAILABLE_008` | 当前的持久化 Provider 不支持该能力：运行时数据源与库结构读取（EF Core 与内存实现）；命名连接（内存实现：实体登记到命名连接在装配时、绑定连接的工作单元在 `BeginAsync` 以异常抛出；EF Core：普通工作单元里取用登记到命名连接的实体，以异常抛出——一个 DbContext 一个连接，那样的实体只能在绑定那个连接的工作单元里读写；已定，不计划支持，ADR-0091）；软删除实体声明了唯一索引（要建成部分唯一索引）而所连数据库不支持部分索引——SqlSugar 的 PostgreSQL、SQLite、SQL Server 之外，EF Core 的 Npgsql、SQLite、SQL Server 之外的关系型 Provider——启动时以异常抛出，不退化成普通唯一索引 |
| `PERSISTENCE_UNEXPECTED_001` | 只读工作单元拒绝写入，是操作实现的缺陷：只读操作（动词 `get` / `list` 或声明了 `ReadOnly`）或以 `IsReadOnly` 开启的工作单元里新增、更新、删除（含批量），以及工作流在只读调用里的写入操作（`IWorkflowTransaction.EnsureWritable`，取锁、执行活动之前）。仓储写入返回结果并把工作单元记为失败（提交返回同一错误）；直接写入入口 `EnsureTransactionAsync` 抛出、由 Transaction 阶段转为同码结果；HTTP 500、gRPC `Internal`、命令行按 UNEXPECTED 的退出码，按操作计入熔断。同一服务作用域里再经管道的嵌套调用仍是 `PERSISTENCE_VALIDATION_002` |
| `CACHING_VALIDATION_001` | 缓存键或幂等键不合法 |
| `CACHING_VALIDATION_002` | 条目选项不合法（绝对与滑动过期同时设置等）：远程缓存插件按操作集校验 `ttlMs` / `slidingMs` 时返回（`provider/caching-distributed.md`），适配器原样成为异常的错误码；.NET 侧构造或使用不合法的 `CacheEntryOptions` 时直接抛 `ArgumentException`（编程错误），不产生错误码 |
| `CACHING_VALIDATION_003` | 键列表的每页上限越界：`CacheKeyQuery.Limit` 不在 1 到 1000 之间（参数 `limit`、`max`；不在构造时抛出，`ICacheStoreInspector` / `ICacheInspector` 的 `ListKeysAsync` 以结果返回，HTTP 400） |
| `CACHING_UNAVAILABLE_001` | 缓存存储不可达或操作失败 |
| `CACHING_UNAVAILABLE_002` | 缓存存储已达容量上限（健康项 Degraded 的信号；LRU 不拒绝写入） |
| `CACHING_CONFLICT_001` | 分布式锁被其他持有者占用：结果式获取 `AcquireAsync` 返回（RFC-0006）；`TryAcquireAsync` 被占用时只是无值 |
| `CACHING_CONFLICT_002` | 同一幂等键的调用仍在处理中 |
| `CACHING_CONFLICT_003` | 幂等记录不能回放给本次调用：同一幂等键已被另一个调用（另一个操作，或操作与非操作调用之间）使用，或记录里的操作响应无法还原 |
| `MESSAGING_VALIDATION_001` | 主题不合法 |
| `MESSAGING_VALIDATION_002` | 信封不合法或无法解码 |
| `MESSAGING_VALIDATION_003` | 领域事件类型未登记（`AddDomainEvent`） |
| `MESSAGING_VALIDATION_004` | 领域事件类型重复登记或登记表已满（以异常抛出） |
| `MESSAGING_UNAVAILABLE_001` | 消息代理不可达或操作失败 |
| `MESSAGING_UNAVAILABLE_002` | 队列已满 |
| `MESSAGING_UNEXPECTED_001` | 处理器抛出未处理异常（代理侧记录） |
| `LOCALIZATION_VALIDATION_002` | 消息目录不合法（JSON、键或区域名错误） |
| `LOCALIZATION_CONFLICT_001` | 同名消息目录里同一键同一区域的模板不一致（注册时抛出，启动失败；同名目录的其余情形合并为一份） |
| `LOCALIZATION_NOTFOUND_001` | 数据库里没有这条翻译 |
| `SECURITY_DENIED_001` | 未认证 |
| `SECURITY_DENIED_002` | 策略拒绝 |
| `SECURITY_DENIED_003` | 凭据无效、过期或签名不符 |
| `SECURITY_DENIED_004` | 缺少权限 |
| `SECURITY_VALIDATION_001` | 不支持的认证方案或凭据格式不合法 |
| `SECURITY_VALIDATION_002` | 条件无效：子句编译出错（属性未登记、运算符与值种类不搭配、条件值解析不了） |
| `SECURITY_VALIDATION_003` | 条件属性的值不是登记的值种类 |
| `SECURITY_VALIDATION_004` | 按名引用的策略不存在：操作契约的 `policy`（授权能力登记的启动核对里核对：容器建好、任何特性启动之前，不论失败策略都终止启动，ADR-0085；之后登记的操作在登记时返回，插件整体登记失败）、端点元数据的 `RequireXiHanPolicy` / `RequireXiHanPermission` / `RequireXiHanAuthentication`（`UseXiHan()` 建请求管道时核对）引用了未注册的策略，或 `permission:<code>` 的权限码未登记；启动时以 `FrameworkException` 抛出、一次列出全部（RFC-0008，ADR-0080、ADR-0085） |
| `SECURITY_NOTFOUND_001` | 策略不存在：调用时才看到的要求（应用代码以 `RequireAuthorization` 显式写入的、启动之后才加进来的端点）引用的策略未登记，或其中 `permission:<code>` 的权限码未登记（含插件卸载时撤回、尚未重新登记的权限码）；操作契约的权限码在装配时核对（`KERNEL_VALIDATION_005`），契约的策略与端点元数据在启动时核对（`SECURITY_VALIDATION_004`） |
| `SECURITY_NOTFOUND_002` | 要撤销的授予不存在 |
| `SECURITY_NOTFOUND_003` | 取不到条件属性的值 |
| `PROTOCOLS_VALIDATION_001` | MCP：消息不是合法的 JSON 或不是合法的 JSON-RPC 2.0 请求 |
| `PROTOCOLS_VALIDATION_002` | MCP：协议版本不受支持 |
| `PROTOCOLS_VALIDATION_003` | MCP：tool 参数不是 JSON 对象 |
| `PROTOCOLS_VALIDATION_004` | MCP：推导或配置的 tool 名不合法（启动失败） |
| `PROTOCOLS_NOTFOUND_001` | MCP：JSON-RPC 方法不受支持 |
| `PROTOCOLS_NOTFOUND_002` | MCP：tool 未登记 |
| `PROTOCOLS_CONFLICT_001` | MCP：两个操作得到同一个 tool 名（启动失败） |
| `PROTOCOLS_CONFLICT_002` | HTTP：两个操作推导出同一条路由（方法与模板都相同，启动失败），或普通端点与操作共用一条路由（生成 OpenAPI 文档失败） |
| `PROTOCOLS_VALIDATION_010` | HTTP：普通端点的 JSON 请求 / 响应类型既没有声明同哪个操作的 Schema，也不与任何已登记操作的请求 / 响应类型匹配（生成 OpenAPI 文档失败） |
| `PROTOCOLS_VALIDATION_005` | gRPC：请求不是合法的 protobuf 消息或与 Schema 不符 |
| `PROTOCOLS_VALIDATION_006` | gRPC：暴露给 gRPC 的操作没有 Schema，或 Schema 无法映射为 protobuf（启动失败） |
| `PROTOCOLS_VALIDATION_007` | gRPC：推导或配置的服务 / 方法名不合法（启动失败） |
| `PROTOCOLS_CONFLICT_004` | gRPC：两个操作得到同一个服务 / 方法名（启动失败） |
| `PROTOCOLS_CONFLICT_003` | gRPC：同名 protobuf 消息在不同操作里形状不同（启动失败） |
| `PROTOCOLS_UNEXPECTED_001` | gRPC：操作的响应无法按 Schema 编码为 protobuf |
| `PROTOCOLS_VALIDATION_008` | CloudEvents：请求不是 CloudEvent（内容类型既非结构化 / 批量格式，也没有 `ce-specversion` 头） |
| `PROTOCOLS_VALIDATION_009` | CloudEvents：事件类型没有映射到主题，类型本身也不是合法主题 |
| `PROTOCOLS_NOTFOUND_003` | CloudEvents：派发模式下主题没有处理器 |
| `PROTOCOLS_NOTFOUND_004` | CloudEvents：主题没有配置出站 Webhook |
| `PROTOCOLS_UNAVAILABLE_001` | CloudEvents：Webhook 返回非 2xx |
| `PROTOCOLS_UNAVAILABLE_002` | CloudEvents：Webhook 无法连接 |
| `HOSTING_VALIDATION_001` | 命令行：用法不合法（选项值、载荷或数据文件有误） |
| `HOSTING_NOTFOUND_001` | 命令行：操作未登记或未暴露给命令行 |
| `HOSTING_NOTFOUND_002` | 命令行：命令不存在 |
| `OBJECTSTORAGE_NOTFOUND_001` | 对象不存在（S3：桶不存在同此，含条件写入探测的桶——启动或建实例时探测桶不存在，缺省实例启动失败（`KERNEL_HOSTING_002` 的内层）、命名实例不建；启动时连不上、之后才发现缺桶的，后台照常重试，缺省实例的健康项 Failed 带此码，RFC-0012「S3 的条件写入探测」） |
| `OBJECTSTORAGE_NOTFOUND_002` | 分片上传不存在或已结束 |
| `OBJECTSTORAGE_NOTFOUND_003` | 命名存储实例不存在 |
| `OBJECTSTORAGE_VALIDATION_001` | 读取范围的起点不在对象内 |
| `OBJECTSTORAGE_VALIDATION_002` | 完成分片上传时的分片清单不合法 |
| `OBJECTSTORAGE_VALIDATION_003` | 命名存储实例的定义不合法（Provider 未登记或设置不合法） |
| `OBJECTSTORAGE_VALIDATION_004` | 缺省存储的能力选择不一致：存储包被要求提供 `objectstorage.multipart` 或 `objectstorage.presign`（有清单依赖该能力，或 `XiHan:Providers` / `UseProvider` 指定了它），却没有被选为 `objectstorage.blob`（装配错误，以异常抛出，启动失败）；只是兜底选出的不报错，存储包按被顶替处理 |
| `OBJECTSTORAGE_VALIDATION_005` | 列举的页大小（`BlobQuery.PageSize`）不在 1 到 1000 之间（参数 `pageSize`、`max`；不在构造时抛出，各 Provider 列举时以结果返回，HTTP 400） |
| `OBJECTSTORAGE_DENIED_001` | 预签名地址无效或已过期 |
| `OBJECTSTORAGE_DENIED_002` | 条件写入探测被存储拒绝：凭据无效或无权写探测键、建桶或放弃探测用的分片上传（HTTP 403；启动或建实例时被拒，缺省实例启动失败（`KERNEL_HOSTING_002` 的内层）、命名实例不建；启动时连不上、之后才被拒的，后台照常重试，缺省实例的健康项 Failed 带此码，RFC-0012「S3 的条件写入探测」） |
| `OBJECTSTORAGE_CONFLICT_001` | 对象已存在且不允许覆盖 |
| `OBJECTSTORAGE_UNAVAILABLE_001` | 对象存储不可达或 IO 失败 |
| `OBJECTSTORAGE_UNAVAILABLE_002` | 配额或单对象大小超限 |
| `OBJECTSTORAGE_UNAVAILABLE_003` | 不覆盖写入在本存储上无法保证：S3 兼容服务不认条件写入（`If-None-Match: *`；启动或建实例时探明的，缺省实例启动失败、命名实例不建；之后在后台或写入时才探明的，不覆盖写入一律以它失败、缺省实例的健康项 Failed），或存储以 `ConditionalWrites = false` 关闭了条件写入（`Overwrite = false` 的写入与分片上传的发起、完成一律以它失败，不访问存储） |
| `OBJECTSTORAGE_UNAVAILABLE_004` | 不覆盖写入暂不可用：S3 兼容服务是否认条件写入尚未确认（启动或建实例时连不上，后台在重试；之后的探测被拒绝或探测桶不存在时同样以它失败；HTTP 503，参数 `container`、`key`、`reason` 为最近一次探测失败的错误码；不访问写入目标，探测通过后自动恢复，RFC-0012「S3 的条件写入探测」） |
| `SCHEDULING_NOTFOUND_001` | 作业未登记 |
| `SCHEDULING_NOTFOUND_002` | 作业类型未登记 |
| `SCHEDULING_NOTFOUND_003` | 触发器不存在 |
| `SCHEDULING_NOTFOUND_004` | 运行不存在或已结束 |
| `SCHEDULING_NOTFOUND_005` | 作业没有代码定义（不能恢复为代码定义） |
| `SCHEDULING_VALIDATION_001` | 作业定义不合法（触发器名重复、日历未登记、计划不合法） |
| `SCHEDULING_VALIDATION_002` | 运行历史查询的分页越界：`Skip` 为负或 `Limit` 不在 1 到 1000 之间（参数 `skip`、`limit`、`max`；不在构造时抛出，`RunsAsync` 以结果返回，HTTP 400） |
| `SCHEDULING_CONFLICT_001` | 作业正在运行且不允许并发 |
| `SCHEDULING_CONFLICT_002` | 领导租约被其他持有者占用 |
| `SCHEDULING_CONFLICT_003` | 运行被取消（`CancelRunsAsync` 或宿主停止） |
| `SCHEDULING_UNAVAILABLE_001` | 调度器未运行：本实例的调度器尚未启动或已开始停止，手动触发不起新的运行 |
| `SCHEDULING_UNAVAILABLE_002` | 领导选举的存储（如 Kubernetes API Server）不可达或超时 |
| `SCHEDULING_UNAVAILABLE_003` | 作业存储不可达 |
| `SCHEDULING_UNAVAILABLE_004` | 作业存储容量已满（内存实现的运行历史） |
| `SCHEDULING_TIMEOUT_001` | 运行超过作业的执行时限（`JobDefinition.Timeout`） |
| `SCHEDULING_UNEXPECTED_001` | 作业抛出未处理异常 |
| `CONFIGURATION_NOTFOUND_001` | 远程配置的命名空间不存在 |
| `CONFIGURATION_NOTFOUND_002` | 密钥不存在 |
| `CONFIGURATION_UNAVAILABLE_001` | 远程配置不可达或超时 |
| `CONFIGURATION_UNAVAILABLE_002` | 密钥库不可达或超时 |
| `CONFIGURATION_DENIED_001` | 没有读取该密钥的权限 |
| `CONFIGURATION_VALIDATION_001` | 远程配置文档不是声明的格式 |
| `AUDITING_VALIDATION_001` | 审计查询不合法（页大小越界、起止颠倒、续页令牌不可识别） |
| `AUDITING_UNAVAILABLE_001` | 审计存储不可达 |
| `SECURITY_UNAVAILABLE_001` | 身份或授予存储不可达：授予存储、授予来源或收窄过滤器读不到数据；Authorize 阶段以它结束调用（HTTP 503），不当作不持有（RFC-0008，ADR-0071） |
| `SECURITY_UNAVAILABLE_002` | 身份或授予存储容量已满 |
| `SECURITY_UNAVAILABLE_003` | 签发者的发现文档或 JWKS 不可达且没有缓存 |
| `SECURITY_UNAVAILABLE_004` | 条件属性暂不可得，补齐后再求值 |
| `MULTITENANCY_VALIDATION_001` | 租户 ID 不合法 |
| `MULTITENANCY_VALIDATION_002` | 调用必须带租户但没有解析到 |
| `MULTITENANCY_NOTFOUND_001` | 租户不存在 |
| `MULTITENANCY_DENIED_001` | 租户已禁用 |
| `MULTITENANCY_UNAVAILABLE_001` | 租户存储容量已满 |
| `MULTITENANCY_UNAVAILABLE_002` | 租户分片无法解析（租户存储不可达） |
| `WORKFLOW_VALIDATION_001` | 工作流定义不合法（`details.errors` 列出全部问题） |
| `WORKFLOW_VALIDATION_002` | 启动或恢复的输入不合法（缺必填变量、子流程深度超限、任务结果不在允许范围、跳转目标不存在、迁移目标不兼容、外部任务领取的锁时长或条数越界、待办查询缺条件或分页越界、实例查询的分页或清理的条数越界） |
| `WORKFLOW_VALIDATION_003` | 表达式语法不合法或求值失败 |
| `WORKFLOW_NOTFOUND_001` | 工作流定义不存在、没有已发布版本，或在调用方之外的分区（平台与各租户互不可见），或建定义时写了别的租户 |
| `WORKFLOW_NOTFOUND_002` | 工作流实例不存在，或在调用方之外的分区 |
| `WORKFLOW_NOTFOUND_003` | 书签（用户任务、外部任务）不存在，或在调用方之外的分区 |
| `WORKFLOW_NOTFOUND_004` | 活动类型或指派规则类型未登记 |
| `WORKFLOW_CONFLICT_001` | 实例状态不允许该操作 |
| `WORKFLOW_CONFLICT_002` | 定义状态不允许该操作 |
| `WORKFLOW_CONFLICT_003` | 书签不可恢复（实例不在运行中、键不匹配） |
| `WORKFLOW_CONFLICT_004` | 指派人已在任务上或任务已被认领 |
| `WORKFLOW_CONFLICT_005` | 同一关联 ID 下已有未结束的实例（`ReusePolicy.RejectActive`） |
| `WORKFLOW_DENIED_001` | 操作者不是任务的指派人 |
| `WORKFLOW_DENIED_002` | 工作者没有持有外部任务的锁或锁已过期 |
| `WORKFLOW_TIMEOUT_001` | 实例锁等待超时 |
| `WORKFLOW_UNAVAILABLE_001` | 工作流存储不可达；或引擎要自己开调用时应用不在服务状态、自己开的调用里没有可加入的事务 |
| `WORKFLOW_UNAVAILABLE_002` | 工作流存储容量已满 |
| `WORKFLOW_UNEXPECTED_001` | 活动抛出未处理异常或引擎内部失败 |
| `AI_VALIDATION_001` | AI 请求不合法（Provider 缺必填项、提示词参数缺失、工具名不合法或重复、工具参数无法序列化；知识库提示词引用了 `{context}`、`{question}` 以外的占位符时为启动失败） |
| `AI_VALIDATION_002` | 嵌入维度与知识库配置的维度不一致 |
| `AI_NOTFOUND_001` | AI Provider 未配置，或其种类没有登记工厂 |
| `AI_NOTFOUND_002` | 提示词模板不存在（`IAIChat` 请求的提示词名；知识库 `PromptName` 引用的模板不存在时为启动失败） |
| `AI_NOTFOUND_003` | AI 工具不存在 |
| `AI_DENIED_001` | 请求被护栏拦截 |
| `AI_TIMEOUT_001` | 模型调用超时：Provider 的超时令牌（`TimeoutSeconds`）已触发即是，调用方同时取消、Provider 接住取消正常返回也一样；Provider 报告了自己的超时同样是它（RFC-0022「超时与取消」，2026-11-02 起） |
| `AI_UNAVAILABLE_001` | 模型端点不可达或返回错误 |
| `AI_UNAVAILABLE_002` | 知识库存储未装配或不可达（向量库连接器抛出的异常，含其自身的容量限制） |
| `AI_UNEXPECTED_001` | 护栏、工具或 Provider 抛出未处理异常 |
| `CLI_NOTFOUND_001` | 应用清单文件不存在 |
| `CLI_VALIDATION_001` | 应用清单不合法，或 `--only` 的产物种类不认识 |
| `CLI_UNAVAILABLE_001` | 无法启动 dotnet 命令行 |
| `CLI_NOTFOUND_002` | `migrate` 的 `--path` 不存在 |
| `CLI_VALIDATION_002` | `migrate` 的来源版本没有对应的规则集 |
| `CLI_VALIDATION_003` | `migrate --fixers` 里有不认识的规则编号 |
| `CLI_NOTFOUND_003` | `mcp` 的知识库里查不到包、能力、配置节或文档页（tool 结果 `isError`，文案带候选） |
| `CLI_VALIDATION_004` | `mcp` 的文档检索既没给检索词也没给页面，或条数超出 1–50 |
| `CLI_NOTFOUND_004` | `setup` 在 `--path` 及其上级找不到项目根（git 根、解决方案或项目文件） |
| `CLI_VALIDATION_005` | `setup` 的 `--client` 或 `--mode` 不认识 |
| `CLI_VALIDATION_006` | `setup` 要合并的编码助手配置不是合法的 JSON 对象；不改动任何文件 |
| `CLI_CONFLICT_001` | `setup --check`：配置或技能未就绪 |
