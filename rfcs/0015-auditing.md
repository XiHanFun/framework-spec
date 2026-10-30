# RFC-0015 审计

> 状态：已采纳（2026-10-27 修订：数据库 Provider；2026-10-28 修订：主体与租户取自安全上下文、记录列长） | 日期：2026-10-23 | 对应 ADR：ADR-0028（数据库 Provider）、ADR-0036 | 参考实现：`XiHan.Framework.Auditing.Abstractions`、`XiHan.Framework.Auditing.InMemory`、`XiHan.Framework.Auditing.Database` | 一致性：`XiHan.Framework.Auditing.Conformance`

## 摘要

内核已定义审计事件 `AuditEvent` 与接收器 `IAuditSink`（RFC-0001 附属），默认丢弃。审计子系统补上三件事：在 Observe 阶段把每次被选中的调用变成一条事件写进接收器的中间件、调用期间补充动作 / 资源 / 细节的上下文扩展、以及查询契约 `IAuditTrail`。能力 `auditing.trail@1` 的 Provider 同时实现 `IAuditSink` 与 `IAuditTrail`；默认实现是有界的内存环形缓冲，并把每条事件写进日志。

## 事件的产生

- `AuditMiddleware` 挂在 Observe 阶段末尾（`ObserveMiddleware` 之内），调用结束后决定是否记录：动作来自 `context.SetAuditAction(...)` 的显式声明，否则来自当前操作（`AuditSelection.Protected`：只记声明了权限的操作，默认；`All`：所有操作；`None`：只记显式声明的）。没有动作就不记。事件的主体与租户优先取安全上下文（认证后的 `Principal.Id`、解析后的 `TenantContext.Id`），没有时回退到 Baggage 的 `xihan.principal` / `xihan.tenant`（插件协议、消息派发、命令行写入的）；Observe 在 Authenticate 之外、事件在调用结束时构造，安全上下文此时已就绪。
- 结果优先取管道里已写入的 outcome；没有写入而抛出了异常，按异常记失败（`KERNEL_UNEXPECTED_001`，取消为 `KERNEL_CANCELLED_001`）；正常返回却没写结果视为成功。`IncludeSuccesses` 为假时只记失败。
- 主体与租户来自 baggage（`xihan.principal`、`xihan.tenant`），trace-id 来自上下文；资源与细节来自 `SetAuditResource` / `AddAuditDetail`。
- 接收器写入失败只记日志，不影响业务调用的结果；写入不带调用的取消令牌，被取消的调用同样记录。

## 查询

- `IAuditTrail.QueryAsync(AuditQuery)` → `Result<AuditPage>`：按写入顺序升序；过滤项 `From` / `To`（含边界）、`Action`、`PrincipalId`、`TenantId`、`TraceId`、`Succeeded`；`PageSize` 1 到 1000（默认 100）；`ContinuationToken` 不透明，页未满时为空。
- 不合法的查询（页大小越界、起止颠倒、令牌不可识别）→ `AUDITING_VALIDATION_001`；存储不可达 → `AUDITING_UNAVAILABLE_001`。

## 默认实现

`Auditing.InMemory`：容量 `XiHan:Auditing:InMemory:Capacity`（默认 10000）的环形缓冲，满了丢最旧的；`LogEvents`（默认开）时每条事件以 Information 级写日志（内核日志薄桥）。配置节 `XiHan:Auditing`：`Selection`、`IncludeSuccesses`。

## 数据库 Provider

`Auditing.Database`（ADR-0028）：事件先进有界队列（`XiHan:Auditing:Database`：`QueueCapacity` 默认 10000；满了丢新到的并计数、节流告警，调用路径不阻塞），后台按批（`BatchSize` 默认 100）或按周期（`FlushIntervalSeconds` 默认 1）在新作用域直接开工作单元批量落库（不经管道），落库失败只记日志；停止时先停循环、停止接收，再刷完剩余事件。行只追加（动作、结果、错误码 / 消息键 / 文案 / 细节、trace-id、资源、主体、租户、细节 JSON、进程内写入序号），不实现 `IMultiTenant`；查询按写入序号升序分页，续页令牌是序号；`Error.Inner` 不持久化。审计记录是平台级数据：落库与查询都在根提供者的新作用域里进行、不绑定租户分片，独立库租户的记录也落应用默认连接，靠租户列区分（ADR-0057 核对）。`LogEvents`（默认开）同内存实现。清单依赖 `persistence.relational@1`。记录的字符串列显式声明长度（`[StringLength(n)]`，动作 256、错误码 64、消息键 256、trace-id 64、资源 1024、主体 256、租户 128）；错误文案、错误细节 JSON 与细节 JSON 是长文本列（`Persistence.Abstractions` 的 `[LongText]`），SqlSugar 的 CodeFirst 与 EF Core 都据此建表，不再落成 `varchar(255)`。

## 一致性

Provider 继承 `AuditTrailConformance`，实现返回接收器与查询的夹具，并通过全部用例。

## 错误码

见 `conventions/error-codes.md` 中 `AUDITING_*`。
