# RFC-0008 认证与授权

> 状态：已采纳（2026-10-27 修订：授予的数据库 Provider；2026-10-28 修订：权限收窄过滤器；2026-10-29 修订：凭据无效是认证失败、授予来源、条件、IP 归属地、绕过管道的端点；2026-11-01 修订：资源名文法、自己产生的租户声明跟随 TenantClaim、权限码文法；授予读不到不当作没有授予、操作引用的权限码装配时核对；插件声明的权限码由宿主登记、按名引用的策略与端点要求启动时核对；同日再修订：契约的策略在宿主的启动核对里核对、不论失败策略都拒绝启动，插件卸载撤回它声明的权限码；2026-11-04 修订：出站地址策略；2026-11-12 修订：WebAuthn；同日再修订：授予缓存在写入提交之后失效，资源名的租户段只用小写、整串匹配，授予的范围拆成平台、每个租户与租户专属；2026-11-14 修订：授予缓存走 `ICache`，失效作用于所有实例；2026-11-24 修订：凭据处理器不另传取消令牌并改名 ICredentialHandler，策略判定的结果改名 AuthorizationDecision） | 日期：2026-10-21 | 对应 ADR：ADR-0027（授予的数据库 Provider）、ADR-0039（权限收窄过滤器）、ADR-0042（凭据无效映射 401）、ADR-0045（授予来源）、ADR-0046（条件）、ADR-0047（IP 归属地）、ADR-0051（绕过管道的端点）、ADR-0071（授予读不到以 503 结束、操作引用的权限码装配时核对）、ADR-0080（插件声明的权限码、策略与端点要求启动时核对）、ADR-0085（启动核对不归失败策略管、卸载撤回登记）、ADR-0117（出站地址策略）、ADR-0170（WebAuthn）、ADR-0194（授予缓存在提交之后失效）、ADR-0196（租户段只用小写）、ADR-0197（授予的范围）、ADR-0229（授予缓存走 ICache）、ADR-0263、ADR-0264 | 参考实现：`XiHan.Framework.Kernel`（出站地址策略）、`XiHan.Framework.Security.Abstractions`、`XiHan.Framework.Hosting.AspNetCore`（绕过管道的端点）、`XiHan.Framework.Security.InMemory`、`XiHan.Framework.Security.Jwt`、`XiHan.Framework.Security.OpenIdConnect`、`XiHan.Framework.Security.Database`、`XiHan.Framework.Security.Ip2Region`、`XiHan.Framework.Security.WebAuthn` | 一致性：`XiHan.Framework.Security.Conformance` | 一致性档：`security`

## 摘要

认证把协议凭据变成主体，授权按策略判定；两者各自是一种能力，可独立换 Provider。内核只定义原语（主体、租户、授权请求、策略接口、权限登记），本 RFC 定义凭据、处理器、授权要求、策略目录与 RBAC 授予。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `security.authentication@1` | 凭据 → 主体 | `Security.InMemory`（`ApiKey` 方案，配置装载） |
| `security.authorization@1` | 策略判定与权限授予 | `Security.InMemory`（RBAC 授予存储）；`Security.Database` 是授予进数据库的显式 Provider |
| `security.webauthn@1` | 通行密钥仪式的选项生成与响应校验（见「WebAuthn」） | `Security.WebAuthn`（可不装） |

`Security.Jwt`（`Bearer`，HS256 对称密钥）与 `Security.OpenIdConnect`（`Bearer`，签发者 JWKS 的非对称密钥）是显式的认证 Provider，同一时刻只装配其一。

## 认证

1. 宿主在 Ingress 阶段把协议凭据（HTTP `Authorization: <scheme> <value>`；没有时，登记过的查询串令牌路径下取 `access_token`，见「绕过管道的端点」）放入上下文，自己不解释它。
2. Authenticate 阶段：没有凭据保持匿名（`Principal.Anonymous`，ID 与认证方案都是 `null`；已认证主体两者都不为空，ADR-0145）；有凭据交给方案匹配（不区分大小写）的 `ICredentialHandler`（`AuthenticateAsync(credential, context)`，取消与截止时间在 `context.CancellationToken` 上，不另传令牌；2026-11-24 修订，ADR-0263、ADR-0264，原名 `IAuthenticationHandler`）。不支持的方案返回 `SECURITY_VALIDATION_001`；凭据无效返回 `SECURITY_DENIED_003`，不区分原因对外暴露；它与 `SECURITY_DENIED_001` 同是认证失败（HTTP 401 并带 `WWW-Authenticate`，gRPC `Unauthenticated`，ADR-0042），客户端据此刷新凭据，其余 DENIED 才是已认证但无权限；成功写入 `SecurityContext.Principal`，保留已有租户。
3. `XiHan:Security:RequireAuthentication` 为真时匿名调用在此被拒绝（`SECURITY_DENIED_001`）；默认由策略决定。
4. 声明按原始类型名保留，不映射为 .NET 长类型；角色声明类型由 `XiHan:Security:RoleClaim`（默认 `role`）决定，租户声明类型由多租户选项 `XiHan:MultiTenancy:TenantClaim`（默认 `tenant`）决定。令牌类方案（JWT、OpenID Connect）原样保留令牌里的声明，签发方要按这个类型写租户；自己产生租户声明的方案（`ApiKey` 把主体记录的租户写成声明）读同一个配置键，改了 `TenantClaim` 也能被 Tenant 阶段的声明解析器认出。

## 绕过管道的端点（2026-10-29 增补，ADR-0051）

长连接（SignalR Hub、Server-Sent Events）一类端点不经内核管道：不受截止时间约束，内核停止时也不被 503 打断。它们由 ASP.NET Core 的认证与授权把关。`Hosting.AspNetCore` 提供认证方案 `XiHan`（`AddAuthentication().AddXiHan(...)`），把它接到内核已装配的认证处理器上，校验项只有一份，JWT、OpenID Connect、ApiKey 都适用。

1. 凭据：先取 `Authorization` 头。没有时，只有请求路径（不含 PathBase）按路径段落在登记的查询串令牌路径前缀（`QueryTokenPaths`）下，才接受查询串 `access_token`（只有一个非空值），作为 `Bearer` 凭据。
   - 没有登记的路径带查询串令牌一律忽略，请求按匿名处理，受保护的端点回 401。
   - 前缀不能为空也不能是 `/`。查询串令牌会进入访问日志与代理日志，只给带不了请求头的客户端（浏览器的 WebSocket 与 EventSource）用。
2. 认证：以请求建调用上下文（与管道同一套载体映射：链路、区域、来源、凭据），交给方案匹配的 `ICredentialHandler`，与 Authenticate 阶段是同一个处理器。
   - 方案只认带绕过元数据的端点：只登记了这一个方案时 ASP.NET Core 会把它当作缺省方案，普通请求仍由管道认证，此处按未认证处理，不重复认证，也不调用挂点。
   - 没有凭据是未认证，不是失败。
   - 方案不支持、凭据无效都是认证失败：401，质询与管道一致（ADR-0042）；方案不支持时按已装配的方案质询。
3. 挂点：认证成功后，依次调用应用登记的 `IInvocationAuthenticationFilter`（如按会话状态核对令牌所属的会话仍然有效），任一返回失败即认证失败。框架不判定会话；管道内的会话门禁仍是应用自己的 Authenticate 阶段中间件。
4. 主体：转成 `ClaimsPrincipal`，身份的认证类型为 `XiHan`。声明按原始类型名保留，另加一条 `ClaimTypes.NameIdentifier` 承载主体 ID 并作为名称声明（SignalR 的用户标识随之可用）。角色声明类型取 `XiHan:Security:RoleClaim`。
5. 在这类端点里发起内核调用：`HttpInvocation.CreateContext(httpContext, services)` 按第 1 条的同一规则带上凭据，调用经管道时照常重新认证与过会话门禁。长连接上的令牌过期后，新的调用会被拒。

## OpenID Connect

1. 配置 `XiHan:Security:OpenIdConnect:Authority`（必填，绝对 URI）。发现文档默认在 `<Authority>/.well-known/openid-configuration`，可用 `MetadataAddress` 覆盖；`RequireHttpsMetadata`（默认真）要求发现文档与 JWKS 都经 HTTPS，只在本地开发时关闭。
2. 元数据按需取得：首次认证时取发现文档的 `issuer` 与 `jwks_uri`，再取 JWKS 的签名密钥；`RefreshInterval`（默认 12 小时）后自动刷新。并发认证只发一次请求，其余等待同一结果。
3. 签名密钥未知（`kid` 不在 JWKS 里）时按需刷新一次再校验；两次按需刷新至少间隔 `MinimumRefreshInterval`（默认 5 分钟），伪造 `kid` 不能让签发者被打爆。
4. 刷新失败沿用最后一份密钥，`MinimumRefreshInterval` 后再试；从未取到元数据时返回 `SECURITY_UNAVAILABLE_003`，5 秒退避后再试。读不出的令牌不访问签发者，直接返回 `SECURITY_DENIED_003`。
5. 校验：签名、过期与有效期强制；签发者取 `Issuer` 覆盖或发现文档的 `issuer`；受众缺省校验，`Audience` 必填，没有即启动失败（2026-11-12 修订，ADR-0180）：共用身份提供方时，发给别的应用的令牌也由同一签发者签名，只有签发者只为本 API 签发令牌时才可以显式设 `ValidateAudience: false` 关掉；主体与声明的映射与 JWT Provider 相同。

## 授权

1. 调用的授权要求是宿主在 Authorize 阶段之前写入上下文的全部 `AuthorizationRequirement { Policy, Resource? }`（端点与路由组元数据上的每一条，以及 Authorize 之前的中间件以 `RequireAuthorization` 追加的），加上操作契约（P3）推导出的那一条（权限码优先于策略名）。Authorize 阶段按顺序逐条判定，全部满足才放行，第一条不满足即以它的结果结束，相同的要求只判定一次；都没有则放行。写入的要求叠加在契约的要求之上，不顶替它：在 `MapXiHanOperations()` 的组上加 `RequireXiHanAuthentication()`，是在每个操作自己的权限之上再要求登录（2026-11-12 修订，ADR-0174）。
2. 权限码文法 `^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`：小写、点分、至少两段，每段以小写字母开头，可含数字与连字符，如 `orders.read`；要带领域前缀时写成三段及以上（`sales.orders.read`）。权限码先经特性登记进内核 `PermissionRegistry`（`FeatureContext.AddPermission`），登记时记下归属（特性的清单 ID；插件声明的码归属插件的清单 ID）。
   - 操作契约引用的权限码（`OperationContract.Permission`）必须已登记（ADR-0071）：全部特性配置完、装配完成的那一刻（`AppBuilder.Build`）一次核对，有未登记的即抛出 `KERNEL_VALIDATION_005`（`FrameworkException`，一次列出全部），启动失败。核对在装配完成时而不是登记操作时：定义权限码的特性可能按拓扑排在引用它的特性之后。
   - 装配完成之后才登记的操作（进程外 / Wasm 插件握手之后：重试成功、运行期 `LoadAsync`）在登记时核对，引用了未登记的权限码即登记失败（`KERNEL_VALIDATION_005`），插件整体被拒、不留下任何代理操作（`plugin/invocation.md` §1）。
   - 插件定义的权限码（ADR-0080）：进程外与 Wasm 宿主登记插件的代理操作时，替它把清单 `contributes.permissions` 声明的码一并登记（显示名取权限码本身），同组的操作可以引用它们；清单没声明、宿主也没登记的码仍按上一条拒绝。声明的码与已登记的码重复（宿主特性定义的，或另一个插件声明的）是冲突（`KERNEL_SECURITY_001`），插件整体登记失败；宿主特性后登记与插件声明的码重复，同样以 `KERNEL_SECURITY_001` 启动失败。配置阶段发现的插件在配置阶段登记，装配完成时的核对看得到它们。
   - 插件卸载时宿主按归属撤回它声明的码，连同它的代理操作一次撤回（`OperationRegistry.RemoveOwnedBy`，ADR-0085），之后可以再加载重新登记。宿主或别的插件的操作以 `permission` 引用了被撤回的码时不随之撤回：重新登记这个码之前，它们的授权判定按策略不存在拒绝（`SECURITY_NOTFOUND_001`），不会放行。
3. 策略按名在 `PolicyCatalog` 查找：注册的 `IPolicy`；`permission:<code>` 在权限码已登记于内核 `PermissionRegistry` 时动态映射为权限策略，未登记的权限码按策略不存在处理。
4. 按名引用的策略在启动时核对（ADR-0080），同第 2 条：引用了不存在的策略是编程错误，不等到调用时。
   - 操作契约的 `policy`：授权能力的 Provider 被选中时在 `Configure` 里经 `FeatureContext.AddStartupCheck` 登记启动核对 `PolicyCatalog.EnforceOn(OperationRegistry)`（ADR-0085，修订 ADR-0080 的「在 `StartAsync` 里调用」）。内核在宿主容器建好之后、任何特性启动之前执行它（RFC-0011「启动核对」）：一次核对已登记的全部操作，有不存在的策略（含 `permission:<code>` 的权限码未登记）即抛出 `SECURITY_VALIDATION_004`（`FrameworkException`，一次列出全部，不包成 `KERNEL_HOSTING_002`），宿主启动失败。
     - 不论内核的失败策略是 `TerminateStartup` 还是 `IsolateFeature`，都拒绝启动：契约引用了不存在的策略是装配错误，隔离授权特性修不好它，只会让应用带着坏契约降级运行、调用时才 404。
     - 此后登记的操作（插件握手之后）在登记时逐个核对，不存在即登记失败（同一错误码），插件整体被拒。
     - 核对放在启动核对而不是配置阶段：策略是容器里的 `IPolicy`，容器建好才知道全部策略名；配置阶段发现的插件的操作与声明的权限码此时都已登记；在任何特性启动之前核对并挂上，插件宿主启动之后的重试登记都经过挂上的核对。
     - `security.authorization@1` 的 Provider 都必须这样做（`Security.InMemory`、`Security.Database` 已实现）；没有装配授权能力时不核对。
   - 端点元数据上的要求（`RequireXiHanPermission`、`RequireXiHanPolicy`、`RequireXiHanAuthentication`）：`UseXiHan()` 在宿主建请求管道时（路由已全部映射）核对全部端点的 `AuthorizationRequirement`，有不存在的策略或未登记的权限码即抛出 `SECURITY_VALIDATION_004`（一次列出全部），宿主启动失败。端点上的要求不得被静默跳过（2026-11-12 修订，ADR-0175）：
     - 绕过宿主管道的端点（`PipelineBypassMetadata`：探针、OpenAPI、`.proto`、自己标注绕过的端点）不会判定要求，挂了要求即抛出 `SECURITY_VALIDATION_008`，宿主启动失败；处理器自己以调用上下文走完整管道并带上端点要求的端点（`PipelineBypassMetadata.SelfInvoking`，gRPC 操作方法）除外，`MapXiHanGrpc()` 上的约定在每次调用里与契约的要求叠加判定。
     - `RequireXiHan*` 给端点包一层守卫：`UseXiHan()` 没有把端点上的要求写进调用上下文（没调 `UseXiHan()`，或它排在路由之前读不到端点）时，端点不执行、抛出 `InvalidOperationException`，宁可失败不可放行。
   - 没有装配授权能力时要求不得静默失效（2026-11-13 修订，ADR-0217；以前两者都不核对、Authorize 阶段不判定任何要求，契约上的权限与端点上的要求等于不存在）：
     - 内核在装配完成时核对：有操作的契约声明了权限码或策略，而 Authorize 阶段没有任何中间件，即抛出 `KERNEL_VALIDATION_006`（一次列出全部），启动失败。此后登记的操作（插件握手之后）声明了权限码或策略即登记失败（同一错误码），插件整体被拒。
     - `UseXiHan()` 在宿主建请求管道时核对：容器里没有 `PolicyCatalog`（没有装配授权能力），而有端点挂了 `AuthorizationRequirement`，即抛出 `KERNEL_VALIDATION_006`，宿主启动失败。
     - 启动之后才写进上下文的要求（`RequireAuthorization`）：没有装配授权能力时抛出 `InvalidOperationException`，宁可失败不可放行。
5. 策略不存在返回 `SECURITY_NOTFOUND_001`（宁可拒绝，不可放行）；拒绝返回策略给出的原因，未给出时为 `SECURITY_DENIED_002`。契约的权限码与策略、端点元数据都已在启动时核对（第 2、4 条），这条路径只剩启动时看不到的要求：应用代码以 `RequireAuthorization` 显式写入的要求、启动之后才加进来的端点。
6. 内置策略：`authenticated`（已认证）；`permission:<code>`（匿名先按 `SECURITY_DENIED_001` 拒绝，缺权限返回 `SECURITY_DENIED_004`）。
7. 策略判定（内核 `IPolicy.EvaluateAsync`）返回 `Result<AuthorizationDecision>`（ADR-0071；2026-11-24 改名，ADR-0264，原名 `AuthorizationResult`）：成功是允许或拒绝；失败是无法判定——判定依赖的授予存储、授予来源或收窄过滤器读不到数据（`SECURITY_UNAVAILABLE_001`）。Authorize 阶段以该失败结束调用（HTTP 503，gRPC `Unavailable`），既不放行，也不改写成拒绝（403）。权限策略把检查器的失败原样作为判定失败返回。

## 资源名（2026-11-01 增补）

授权要求与审计里的资源用统一资源名 `xrn:<partition>:<service>:<tenant>:<type>/<id>`（内核 `ResourceName`）：

- `partition`、`service`、`type`：`[a-z0-9-]+`。
- `tenant`：空段（全局资源；.NET 的 `ResourceName.Tenant` 为 `null`，构造时传空字符串被拒绝，2026-11-11 修订，ADR-0145），或与 RFC-0007「租户 ID」同一文法的租户 ID（`[a-z0-9][a-z0-9._-]{0,63}`，只用小写，2026-11-12 修订，ADR-0196）。任何合法的租户 ID 拼进资源名都能原样解析回来，`acme`、`acme.cn`、`t_42` 都是合法的租户段，`Acme` 不是。
- `id`：不含 `/` 与空白的非空串。

解析不合法的资源名失败（`Parse` 抛出参数异常，`TryParse` 返回假），不做大小写或字符替换；整串匹配，结尾带换行的同样不合法（2026-11-12 修订，ADR-0196）。

## RBAC 授予

- 主体是用户（`user:<id>`）或角色（`role:<name>`）；授予是（主体，权限码，范围）。范围有三种（2026-11-12 修订，ADR-0197；以前租户为空的授予对任何租户生效，平台授给 `role:admin` 的权限也落到每个租户里带同名角色、同一用户 ID 的人身上）：
  - 平台（租户为空）：只对不带租户的调用生效。
  - 每个租户（租户为 `*`，.NET 的 `PermissionGrant.AnyTenant`）：对带任何租户的调用生效，对不带租户的调用不生效。`*` 不合租户 ID 文法（RFC-0007），不会与哪个租户撞名。
  - 租户专属（租户为租户 ID）：只对该租户生效。

  不带租户的调用只看到平台授予；带租户的调用只看到每个租户的授予与本租户的授予。平台与租户两边的授予互不渗透：平台的人要在租户里持有权限，要另有一条每个租户或该租户的授予。
- 检查器取用户本身的授予与其角色授予之并集；`*` 匹配任何权限。
- 批量读取（2026-11-11 修订，ADR-0139）：`IPermissionGrantStore` 只有 `GetManyAsync(subjects, tenantId)`，返回与入参等长、顺序一致的 `Result<IReadOnlyList<IReadOnlyCollection<string>>>`，每项是该主体对这次调用生效的授予（租户为空时是平台授予，否则是每个租户的授予加该租户的授予；调用的租户不能是 `*`，传入即参数异常），重复的主体各得一份相同的结果，空集合不访问后端；检查器把用户与它的全部角色一次读出，不再一个角色一次往返。单个主体的 `GetGrantsAsync(subject, tenantId)` 是扩展方法，等于只含它的 `GetManyAsync`。
- 授予管理（2026-11-11 修订，ADR-0158）：可选接口 `IPermissionGrantAdministration`，两个实现都提供。`GrantAsync(grants[])` / `RevokeAsync(grants[])` 返回与入参同序的逐项结果：已存在的授予幂等成功，撤销不存在的是 `SECURITY_NOTFOUND_002`，同一批里后一项看见前一项的效果。`ListAsync(PermissionGrantQuery)` 按主体与租户过滤并偏移分页（`Skip` / `Take`，最多 1000，结果页 `Items` 与 `Total`），顺序是主体种类、主体 ID、权限码、范围（平台在前、每个租户其次，然后按租户 ID），越界返回 `SECURITY_VALIDATION_005`。范围过滤是 `GrantTenantFilter`：`Any` 不过滤、`Platform` 只要平台授予、`Of(tenantId)` 只要该范围（`Of("*")` 是每个租户的授予），不用 `null` 表示「不过滤」（2026-11-12 修订，ADR-0197：`Global` 改名 `Platform`）。
- 读不到不是没有（ADR-0071）：授予读取返回 `Result`，没有授予是成功的空集合，存储不可达（连接、超时、事务失败）是 `SECURITY_UNAVAILABLE_001` 失败，不得以空集合代替；`IPermissionChecker.HasPermissionAsync` 返回 `Result<bool>`，存储、来源或过滤器的失败原样返回，不当作不持有。检查器遇到第一个失败即返回，不再读来源或过滤器（用户与角色在同一次批量读取里，存储故障时整批失败；授予只能加、过滤器只能拿走，部分读取得不出可靠结论）。匿名主体不访问存储，存储故障时仍是不持有。
- 收窄（ADR-0039）：授予命中之后，检查器按登记顺序逐个调用全部 `IPermissionFilter`（单例），任一返回假即不持有、后面的不再调用；匿名主体与授予未命中时不调用。`IPermissionFilter.AllowsAsync` 返回 `Result<bool>`：过滤器依赖的数据（套餐、拒绝规则、委托）读不到时返回失败，检查器原样返回（跳过它等于放行，按拒绝处理又会把故障变成 403）。过滤器收到的是具体权限码（通配命中也一样），只能拿走权限、不能凭空给出；用于套餐白名单、显式拒绝、委托到期等授予之外的约束。被收窄的权限在权限策略上同样返回 `SECURITY_DENIED_004`。替换 `IPermissionChecker` 的实现必须同样在授予命中之后施加全部过滤器。
- 授予来源（ADR-0045）：与收窄对称，`IPermissionGrantSource`（单例）给出主体在授予存储之外的额外授予（权限码或 `*`），用于角色继承、限时委托等不落在主体自己授予行上的授予。检查器在用户与角色的存储授予都没命中时按登记顺序逐个查询，任一命中即算授予命中，之后照常经全部过滤器收窄；匿名主体不查询。`IPermissionGrantSource.GetGrantsAsync` 返回 `Result<IReadOnlyCollection<string>>`：来源依赖的数据（角色继承、委托）读不到时返回失败，检查器原样返回，不当作没有授予。来源只能加、不能拿走；替换 `IPermissionChecker` 的实现必须同样并上全部来源。
- `IPermissionGrantStore` 由 Provider 实现；内存实现从 `XiHan:Security:InMemory:Grants` 装载平台授予、从 `XiHan:Security:InMemory:AnyTenantGrants` 装载每个租户的授予（2026-11-12 修订，ADR-0197），有界。
- `Security.Database`（ADR-0027）：授予是实体行（主体种类、主体 ID、权限码、租户列；平台授予以空串存储，每个租户的授予以 `*` 存储），`IPermissionGrantStore` 单例在新作用域直接开只读工作单元读仓储（不经管道），一次查出主体对这次调用生效的授予（平台调用是平台授予，租户调用是每个租户的授予与当前租户的授予）；数据库故障（开启、读取、提交只读工作单元失败，含连接与命令超时）返回 `SECURITY_UNAVAILABLE_001`（消息键 `security.grant_store.unavailable`，不带参数：底层原因只记日志，不进响应），失败不进缓存；调用方取消照常抛出。结果经 `ICache` 缓存（代数缓存区，区 `security:grants`，范围是主体、条目是租户，RFC-0006），装了两级缓存时判定在本机 L1 命中、不出进程；条目的存活时间 `XiHan:Security:Database:CacheTimeToLiveSeconds`（正数，默认 30），只兜住失效丢失，容量由提供 `caching.local` 的 Provider 管；缓存后端故障时判定直接查库、不写缓存，结果仍然正确，不当作没有授予（2026-11-14 修订，ADR-0229；以前是本实例内存里的有界表，`CacheSize` 默认 10000，存活时间 0 为不过期）；`PermissionGrantRecordStore` 在调用方的工作单元里批量授予 / 撤销并逐项返回结果，撤销不存在的授予返回 `SECURITY_NOTFOUND_002`，写入后失效该主体的缓存，作用于所有共用缓存的实例（2026-11-14 修订，ADR-0229；以前跨实例靠 TTL 收敛）。写入在调用方的事务提交后才对别的调用可见，所以失效两次：写入时一次，作用域释放时（已提交或回滚）再一次；缓存按代数丢弃失效之前开始的读取，提交之前读回的旧行不留在缓存里（2026-11-12 修订，ADR-0194；以前只在写入时失效一次，撤销提交之前的判定能把已撤销的授予写回缓存，保留到 TTL 结束，TTL 为 0 时永久保留）。写入时的失效遇到缓存后端故障就抛出、调用失败回滚，撤销传不到别的实例时不报成功；释放时的失效失败只记错误日志，靠存活时间收敛。存活时间不再接受 0（2026-11-14 修订，ADR-0229）。清单依赖 `persistence.relational@1` 与 `caching.local@1`。

## 条件

条件（ADR-0046）是访问策略、权限条件与约束规则共用的属性判定，引擎在 `Security.Abstractions.Conditions`，与存储无关：应用提供属性来源（主体、资源、环境）与条件的存储，引擎负责登记、编译、求值。

- 结构：条件集由组构成，组由子句构成；组内且、组间或。子句是（属性名，运算符，条件值原文，是否取反，标识）。没有非空组的条件集是"无条件"，求值恒为命中；需要拒绝空条件的场景由调用方在保存时拦下。
- 运算符（整数值是持久化契约，不得调整）：`Equals` 0、`NotEquals` 1、`GreaterThan` 2、`GreaterThanOrEquals` 3、`LessThan` 4、`LessThanOrEquals` 5、`Contains` 6、`NotContains` 7、`In` 8、`NotIn` 9、`Between` 10、`StartsWith` 11、`EndsWith` 12、`IsNull` 13、`IsNotNull` 14。带 Not 的是对应运算符的否定，子句取反再作用于结果。
- 值种类（整数值同样是契约）：`String` 0（不区分大小写的序数比较）、`Number` 1（十进制）、`Boolean` 2（`true` / `false` / `1` / `0`）、`DateTime` 3（ISO 8601，不带偏移的按编译时区的墙钟解释）、`Date` 4、`TimeOfDay` 5（`HH:mm[:ss]`）、`IpAddress` 6（IPv4 只收四段点分，IPv4 映射的 IPv6 还原为 IPv4，不收区域标识）。
- 搭配：等于、不等于、在集合中、不在集合中、为空、不为空对任何属性可用；比大小与区间只用于数字、日期时间、日期、时刻；包含 / 不包含用于多值属性与字符串（单值字符串为子串）；开头 / 结尾只用于字符串。编辑界面按同一规则列运算符。
- 属性登记：登记名不区分大小写，可带旧名别名；以 `.*` 结尾的是一族属性（如 `resource.route.*`），按最长前缀匹配，精确登记优先；查找顺序精确名 → 别名 → 族。登记项带值种类与是否多值；重名、别名冲突在登记表建立时报错。编译选项可带只对这一次生效的额外登记项，优先于登记表。
- 值语法：集合写 JSON 数组（元素只能是字符串、数字、布尔）或以逗号、全角逗号、换行分隔，不能为空；IP 集合可写 CIDR 网段，主机位必须为零、前缀不超过地址位数。区间写 `下限,上限`、`[下限,上限]` 或 `{"min":下限,"max":上限}`（键不区分大小写），正好两个值，下限不得大于上限——时刻例外，下限大于上限表示跨零点。标量可写成 JSON 字符串（去掉引号）；为空 / 不为空不需要条件值。
- 编译：属性未登记、运算符与值种类不搭配、条件值解析不了都是 `SECURITY_VALIDATION_002`，消息键说明具体原因（RFC-0010 本地化）。出错的子句保留在编译结果里，求值时恒为无法判定，存量脏数据不会让整个条件集失效；保存前据编译错误拒绝。
- 求值：子句结果是命中、不命中、无法判定、待定。属性取不到（`SECURITY_NOTFOUND_003`）或值不是登记的种类（`SECURITY_VALIDATION_003`）为无法判定，按不命中计且取反不翻转；属性暂不可得（`SECURITY_UNAVAILABLE_004`，如模型绑定前的资源行属性）为待定。为空 / 不为空把取不到、空列表、全是空白的字符串当空，可以取反。多值属性按任一元素判定，否定运算符按没有元素判定。组：任一子句不命中或无法判定即不命中，否则任一待定即待定，否则命中。条件集：任一组命中即命中，否则任一组待定即待定，否则不命中。时间类属性按编译时区折算成日期、时刻。
- 说明模式：试算时逐组逐子句给出结果、取到的属性值（不变文化文本）与无法判定 / 待定的原因（带错误码与消息键的错误）；不要说明时短路求值，只取被引用到的属性。

## IP 归属地

IP 归属地（ADR-0047）是能力 `security.ip-region@1`：`IIpRegionResolver.Resolve(IPAddress)` 返回可能缺失的归属地（国家、省份、城市、运营商、两位国家代码，各段可缺）。条件属性 `environment.geo.*`、登录日志与命中记录的归属地列用它。

- 实现不访问网络、线程安全；IPv4 映射的 IPv6 地址按 IPv4 查；回环、私有网段、保留地址、数据不可用、查不到都返回无值，不抛出。
- 宿主可以不装：没有本能力时调用方把归属地当作取不到（条件子句为无法判定，为空 / 不为空照常判定）。
- 参考实现 `Security.Ip2Region`：ip2region 离线库，IPv4 与 IPv6 各一个由宿主提供的 xdb 文件，配置节 `XiHan:Security:Ip2Region`；某个地址族的文件缺失或加载失败只记警告，该族一律无值。

## 出站地址策略（2026-11-04 增补，ADR-0117）

框架或应用向用户给的地址发请求时，要防止被指向内网与云元数据地址（服务端请求伪造）。机制在内核：`Kernel.Security.OutboundAddressPolicy`，宿主总是登记为单例；放行项 `OutboundAddressOptions` 读配置节 `XiHan:Security:Outbound`，装配时读一次，不热更新，值不合法即启动失败。

一个 IP 能否连接，依次判定（`IsAllowed(address)`；IPv4 映射地址 `::ffff:0:0/96` 先还原成 IPv4）：

1. 落在 `AllowedNetworks`（CIDR，主机位必须为零）里 → 放行。这是唯一能放行第 3、6、7 步拒绝的地址的配置。
2. 回环 `127.0.0.0/8`、`::1` → 看 `AllowLoopback`，缺省拒绝。
3. 云的 IPv6 元数据地址 → 拒绝：AWS `fd00:ec2::254`、GCP `fd20:ce::254`、Oracle `fd00:c1::a9fe:a9fe`（2026-11-12 修订，ADR-0206；以前只有 AWS 的，另两个在开了私网时能连）。它们在私网段里，开私网不等于开元数据。
4. 私网 `10.0.0.0/8`、`172.16.0.0/12`、`192.168.0.0/16`、`fc00::/7` → 看 `AllowPrivateNetwork`，缺省拒绝。
5. 内嵌 IPv4 的 IPv6 地址（NAT64 `64:ff9b::/96`、6to4 `2002::/16`、IPv4 兼容 `::/96`）→ 取出内嵌的 IPv4，从第 1 步重新判定。
6. 其余 IPv4：`0.0.0.0/8`、`100.64.0.0/10`、`169.254.0.0/16`、`192.0.0.0/24`、`192.0.2.0/24`、`192.88.99.0/24`、`198.18.0.0/15`、`198.51.100.0/24`、`203.0.113.0/24`、`224.0.0.0/4`、`240.0.0.0/4` → 拒绝，此外放行。
7. 其余 IPv6：只放行全球单播 `2000::/3`，其中 `2001::/23`、`2001:db8::/32`、`3fff::/20` 拒绝。

检查放在建立连接时，按解析出的实际 IP 判。只查 URL 或事先解析一次都会被 DNS 重绑定绕过：

- `Apply(SocketsHttpHandler)` 设置 `ConnectCallback`。每条新连接都走它，重定向的目标也一样。回调解析主机名后只连策略允许的地址，连接用的就是检查过的地址，不再解析第二次。处理器已有别的连接回调时抛 `InvalidOperationException`，同一策略重复 `Apply` 没有副作用。
- 主机名在 `AllowedHosts` 里（不区分大小写，整名相等，不支持通配符）时不看解析结果。值不是主机名（带协议或端口）即启动失败。
- 解析结果里一部分被拒绝时只连其余的，记一条 Debug；全部被拒绝时记一条 Warning（主机、端口与解析出的地址），抛 `OutboundAddressDeniedException`，不发起连接。异常只带请求里的主机名与端口，不带解析出的地址：解析结果是内网信息，只进日志。经 `HttpClient` 发请求时它是 `HttpRequestException` 的 `InnerException`。
- `Apply` 同时关掉 `UseProxy`：经代理时回调看到的是代理的地址，查不到真正的目标。应用之后再打开代理就绕过了策略，要用代理就在代理上做这层限制。
- 只管 `SocketsHttpHandler` 经连接回调建立的 TCP 连接。HTTP/3 走 QUIC，不经过连接回调；请求缺省是 HTTP/1.1，把 `Version` 设成 3.0 的请求不受保护。不是 HTTP 但地址来自用户的连接（SMTP）用 `ConnectAsync(host, port, ct)`：规则与上面相同，HTTP 的连接回调就是调用它，返回连好的 `Socket`，由调用方释放；主机名空白、端口不在 1～65535 抛 `ArgumentException`（2026-11-04 修订，ADR-0126）。自己连接的驱动（数据库、消息代理等）不归它管。

框架的出站路径是否接入，取决于地址是不是用户给的：

| 路径 | 地址来源 | 接入 |
| --- | --- | --- |
| 工作流 `Http` 活动的缺省客户端 | 流程定义，租户管理员可编辑 | 接入（RFC-0021） |
| CloudEvents Webhook | 运维配置，但接收方是第三方，它的 3xx 能把请求引到内网 | 接入（`protocol/cloudevents.md`） |
| S3 命名实例 | `BlobStoreDefinition`，可能来自应用的库（RFC-0012） | 接入 |
| S3 缺省实例 | 运维配置 | 不接入 |
| OpenAI Provider | 运维配置 | 不接入。应用的提供商目录让用户填端点时，给命名客户端 `OpenAIFeature.HttpClientName` 接上（RFC-0022） |
| OpenID Connect 发现与 JWKS | 运维配置，签发者常在内网，且它本身就是令牌的信任根 | 不接入。`OpenIdConnectMetadataCache` 接受应用传入的 `HttpClient` |
| Nacos、Kubernetes API Server、OTLP 导出 | 运维配置，都在内网 | 不接入 |
| 进程外插件通道、CLI 健康探测 | 本机，由框架自己决定 | 不接入 |

应用替换了主处理器或 `WorkflowHttpClient` 时，由替换后的处理器决定能连哪里。

## WebAuthn（2026-11-12 增补，ADR-0170）

能力 `security.webauthn@1`：通行密钥（WebAuthn Level 3）注册与认证两个仪式的选项生成与响应校验。参考实现 `Security.WebAuthn`（`WebAuthnCeremonies`）是无状态的：挑战由应用保存到仪式结束，凭据由应用存储，框架只生成与核对。

1. 依赖方配置 `XiHan:Security:WebAuthn`，装配时读一次，值不合法即启动失败：
   - `RelyingPartyId`：必填，可注册的域名（如 `example.com`、本地开发的 `localhost`），不带协议、端口与路径，按小写比较。
   - `RelyingPartyName`：认证器上显示的名称，缺省同 `RelyingPartyId`。
   - `Origins`：必填，至少一个，形如 `scheme://host[:port]`、不带路径；主机必须等于 `RelyingPartyId` 或是它的子域，`https` 之外只允许主机为 `localhost` 的 `http`。
   - `UserVerification` 与 `ResidentKey`：`Required`、`Preferred`、`Discouraged`，缺省都是 `Required`（免用户名、免密码登录要可发现凭据与本机验证）。
   - `TimeoutSeconds`：仪式的超时，缺省 300。`ChallengeSize`：挑战字节数，16 到 64，缺省 32。
2. 选项：`CreateRegistrationOptions(user, excludeCredentials)` 与 `CreateAuthenticationOptions(allowCredentials)` 生成与 `PublicKeyCredentialCreationOptionsJSON` / `PublicKeyCredentialRequestOptionsJSON` 同形的对象，二进制一律 base64url（无填充），浏览器用 `PublicKeyCredential.parseCreationOptionsFromJSON` / `parseRequestOptionsFromJSON` 直接还原。
   - 挑战取自 `RandomNumberGenerator`；证明一律请求 `none`；算法依次为 ES256（-7）、PS256（-37）、RS256（-257）。
   - 用户句柄 1 到 64 字节，不能含个人信息；`allowCredentials` 为空即请求可发现凭据（免用户名登录）。
3. 注册校验 `VerifyRegistration(response, expectedChallenge)` 依次核对，第一处不符即返回：
   1. `type` 为 `public-key`，`id` 与 `rawId` 相同（`id`）。
   2. `clientDataJSON` 是 JSON 对象（`client_data`），`type` 为 `webauthn.create`（`type`），`challenge` 与期望值逐字节相同、定长比较（`challenge`），`origin` 在 `Origins` 里（`origin`），`crossOrigin` 不为真（`cross_origin`）。
   3. 证明对象是 CBOR 映射，带 `fmt`、`attStmt` 与 `authData`（`attestation_object`）；`authData` 的长度与结构合法（`authenticator_data`）。
   4. RP ID 哈希等于 `SHA-256(RelyingPartyId)`（`rp_id`）；用户在场（UP，`user_present`）；`UserVerification` 为 `Required` 时用户已验证（UV，`user_verified`）；不可备份（BE 为 0）时备份状态（BS）不能置位（`flags`）；带证明凭据数据（AT，`attested_credential`）。
   5. 凭据 ID 1 到 1023 字节且与 `rawId` 相同（`credential_id`）；公钥是 COSE 键，算法在上面三种里，ES256 只认 P-256，RSA 模长 2048 到 4096 位（`public_key`；2026-11-13 修订，ADR-0216：以前不设上限，几万位的模数让每次验签都很慢；算法不在其中为 `SECURITY_VALIDATION_007`）。
   6. 证明声明不校验，任何格式都按 `none` 对待：依赖方不要求证明，公钥是否真由认证器持有，由之后每次认证的签名证明；只有 `fmt` 为 `none` 时要求 `attStmt` 为空映射（`attestation`）。
   成功返回凭据 ID、COSE 公钥（base64url）、算法、签名计数、AAGUID、BE、BS、UV 与 `transports`。
4. 认证校验 `VerifyAuthentication(response, expectedChallenge, credential)`：
   1. 同 3.1，且 `id` 等于存储的凭据 ID（`credential`）。
   2. `clientDataJSON` 的 `type` 为 `webauthn.get`，其余同 3.2。
   3. `authenticatorData` 的长度与结构、RP ID、UP、UV、BE 与 BS 同 3.3、3.4（不要求 AT）。
   4. 签名覆盖 `authenticatorData ‖ SHA-256(clientDataJSON)`，算法取存储的公钥：ES256 是 DER 编码的 ECDSA，PS256 是 PSS，RS256 是 PKCS#1 v1.5，都用 SHA-256（`signature`）。
   5. 凭据属于谁（2026-11-12 修订，ADR-0179；WebAuthn §7.2 第 6 步；2026-11-13 修订，ADR-0216：移到验签之后，以前在验签之前，没有私钥的人拿任意凭据 ID 与句柄试，就能从 `user_handle` 与 `signature` 两种原因分辨凭据属于谁）：存储的凭据必须记下所属用户的句柄（注册选项的 `user.id`）。调用方传入登录前已认出的用户（先填账号、第二因素）时，凭据的所属用户必须是他；没有认出用户（免用户名）时，响应必须带 `userHandle`。响应带了 `userHandle` 就必须等于所属用户。任一不满足为 `user_handle`。结果交回所属用户的句柄，免用户名的流程据此确定是谁登录，不取响应里的句柄。
   6. 签名计数：新旧值有一个不为 0 时新值必须大于旧值，否则 `SECURITY_DENIED_006`（认证器可能被复制，应用应提醒本人或停用凭据）；都为 0 是不计数的认证器，放行。
   成功返回新的签名计数、UV、BS 与所属用户的句柄，应用据此更新存储的凭据。
5. 失败以 `Result` 返回、不抛出，参数 `reason` 是上面括号里的机读原因：
   - 响应解析不了是 `SECURITY_VALIDATION_006`：`id`、`client_data`、`attestation_object`、`authenticator_data`、`flags`、`attested_credential`、`credential_id`、`public_key`、`attestation`。
   - 公钥算法不支持是 `SECURITY_VALIDATION_007`，参数 `algorithm`。
   - 核对不通过是 `SECURITY_DENIED_005`：`type`、`challenge`、`origin`、`cross_origin`、`rp_id`、`user_present`、`user_verified`、`credential`、`user_handle`、`signature`。
   - 参数为 `null` 抛 `ArgumentNullException`；应用给的期望挑战或存储的凭据 ID 不是 base64url 抛 `ArgumentException`（调用方的错误，不是响应的错误）；存储的公钥解不开按 `public_key` 返回。
6. 不做：证明的信任链与元数据服务（MDS；AAGUID 在 `none` 证明下是认证器自报的型号，没有经过证明，不能拿来限制型号，2026-11-13，ADR-0216），扩展（客户端扩展结果忽略），挑战与凭据的存储。

## 一致性

授权 Provider 必须让其 `IPermissionGrantStore` 通过 `PermissionGrantStoreConformance`。套件的「存储不可达」用例要求 Provider 给出一个背后存储不可达的实例（覆写 `CreateUnavailableStoreAsync`），读取必须返回 `SECURITY_UNAVAILABLE_001` 而不是空集合；只有进程内、读取不会失败的存储（内存实现）可以不给，该用例跳过。

WebAuthn 的实现要逐条回放 `conformance/vectors/security/webauthn.json`：注册与认证的样例响应及期望结果（成功时的凭据与签名计数，失败时的错误码与 `reason`）。

## 错误码

见 `conventions/error-codes.md` 中 `SECURITY_*`。
