# RFC-0008 认证与授权

> 状态：已采纳（2026-10-27 修订：授予的数据库 Provider；2026-10-28 修订：权限收窄过滤器；2026-10-29 修订：凭据无效是认证失败、授予来源、条件、IP 归属地） | 日期：2026-10-21 | 对应 ADR：ADR-0027（授予的数据库 Provider）、ADR-0039（权限收窄过滤器）、ADR-0042（凭据无效映射 401）、ADR-0045（授予来源）、ADR-0046（条件）、ADR-0047（IP 归属地） | 参考实现：`XiHan.Framework.Security.Abstractions`、`XiHan.Framework.Security.InMemory`、`XiHan.Framework.Security.Jwt`、`XiHan.Framework.Security.OpenIdConnect`、`XiHan.Framework.Security.Database`、`XiHan.Framework.Security.Ip2Region` | 一致性：`XiHan.Framework.Security.Conformance`

## 摘要

认证把协议凭据变成主体，授权按策略判定；两者各自是一种能力，可独立换 Provider。内核只定义原语（主体、租户、授权请求、策略接口、权限登记），本 RFC 定义凭据、处理器、授权要求、策略目录与 RBAC 授予。

## 能力

| 能力 | 含义 | 默认实现 |
| --- | --- | --- |
| `security.authentication@1` | 凭据 → 主体 | `Security.InMemory`（`ApiKey` 方案，配置装载） |
| `security.authorization@1` | 策略判定与权限授予 | `Security.InMemory`（RBAC 授予存储）；`Security.Database` 是授予进数据库的显式 Provider |

`Security.Jwt`（`Bearer`，HS256 对称密钥）与 `Security.OpenIdConnect`（`Bearer`，签发者 JWKS 的非对称密钥）是显式的认证 Provider，同一时刻只装配其一。

## 认证

1. 宿主在 Ingress 阶段把协议凭据（HTTP `Authorization: <scheme> <value>`）放入上下文，自己不解释它。
2. Authenticate 阶段：没有凭据保持匿名；有凭据交给方案匹配（不区分大小写）的 `IAuthenticationHandler`。不支持的方案返回 `SECURITY_VALIDATION_001`；凭据无效返回 `SECURITY_DENIED_003`，不区分原因对外暴露；它与 `SECURITY_DENIED_001` 同是认证失败（HTTP 401 并带 `WWW-Authenticate`，gRPC `Unauthenticated`，ADR-0042），客户端据此刷新凭据，其余 DENIED 才是已认证但无权限；成功写入 `SecurityContext.Principal`，保留已有租户。
3. `XiHan:Security:RequireAuthentication` 为真时匿名调用在此被拒绝（`SECURITY_DENIED_001`）；默认由策略决定。
4. 声明按原始类型名保留，不映射为 .NET 长类型；角色声明类型由 `XiHan:Security:RoleClaim`（默认 `role`）决定，租户声明类型由多租户选项决定。

## OpenID Connect

1. 配置 `XiHan:Security:OpenIdConnect:Authority`（必填，绝对 URI）。发现文档默认在 `<Authority>/.well-known/openid-configuration`，可用 `MetadataAddress` 覆盖；`RequireHttpsMetadata`（默认真）要求发现文档与 JWKS 都经 HTTPS，只在本地开发时关闭。
2. 元数据按需取得：首次认证时取发现文档的 `issuer` 与 `jwks_uri`，再取 JWKS 的签名密钥；`RefreshInterval`（默认 12 小时）后自动刷新。并发认证只发一次请求，其余等待同一结果。
3. 签名密钥未知（`kid` 不在 JWKS 里）时按需刷新一次再校验；两次按需刷新至少间隔 `MinimumRefreshInterval`（默认 5 分钟），伪造 `kid` 不能让签发者被打爆。
4. 刷新失败沿用最后一份密钥，`MinimumRefreshInterval` 后再试；从未取到元数据时返回 `SECURITY_UNAVAILABLE_003`，5 秒退避后再试。读不出的令牌不访问签发者，直接返回 `SECURITY_DENIED_003`。
5. 校验：签名、过期与有效期强制；签发者取 `Issuer` 覆盖或发现文档的 `issuer`；受众只在配置了 `Audience` 才校验；主体与声明的映射与 JWT Provider 相同。

## 授权

1. 调用的授权要求 `AuthorizationRequirement(policy, resource?)` 由宿主端点元数据或操作契约（P3）在 Authorize 阶段之前写入上下文；没有要求则放行。
2. 策略按名在 `PolicyCatalog` 查找：注册的 `IPolicy`；`permission:<code>` 在权限码已登记于内核 `PermissionRegistry` 时动态映射为权限策略，未登记的权限码按策略不存在处理。
3. 策略不存在返回 `SECURITY_NOTFOUND_001`（宁可拒绝，不可放行）；拒绝返回策略给出的原因，未给出时为 `SECURITY_DENIED_002`。
4. 内置策略：`authenticated`（已认证）；`permission:<code>`（匿名先按 `SECURITY_DENIED_001` 拒绝，缺权限返回 `SECURITY_DENIED_004`）。

## RBAC 授予

- 主体是用户（`user:<id>`）或角色（`role:<name>`）；授予是（主体，权限码，租户?）。租户为空的授予对任何租户生效，租户专属授予只对该租户生效。
- 检查器取用户本身的授予与其角色授予之并集；`*` 匹配任何权限。
- 收窄（ADR-0039）：授予命中之后，检查器按登记顺序逐个调用全部 `IPermissionFilter`（单例），任一返回假即不持有、后面的不再调用；匿名主体与授予未命中时不调用。过滤器收到的是具体权限码（通配命中也一样），只能拿走权限、不能凭空给出；用于套餐白名单、显式拒绝、委托到期等授予之外的约束。被收窄的权限在权限策略上同样返回 `SECURITY_DENIED_004`。替换 `IPermissionChecker` 的实现必须同样在授予命中之后施加全部过滤器。
- 授予来源（ADR-0045）：与收窄对称，`IPermissionGrantSource`（单例）给出主体在授予存储之外的额外授予（权限码或 `*`），用于角色继承、限时委托等不落在主体自己授予行上的授予。检查器在用户与角色的存储授予都没命中时按登记顺序逐个查询，任一命中即算授予命中，之后照常经全部过滤器收窄；匿名主体不查询。来源只能加、不能拿走；替换 `IPermissionChecker` 的实现必须同样并上全部来源。
- `IPermissionGrantStore` 由 Provider 实现；内存实现从 `XiHan:Security:InMemory:Grants` 装载全局授予，有界。
- `Security.Database`（ADR-0027）：授予是实体行（主体种类、主体 ID、权限码、租户列；全局授予以空串存储），`IPermissionGrantStore` 单例在新作用域直接开只读工作单元读仓储（不经管道），一次查出主体的全局授予与当前租户的授予，按（主体，租户）有界缓存并按 TTL 失效（`XiHan:Security:Database`：`CacheSize` 默认 10000、`CacheTimeToLiveSeconds` 默认 30，0 为不过期）；`PermissionGrantRecordStore` 在调用方的工作单元里批量授予 / 撤销并逐项返回结果，撤销不存在的授予返回 `SECURITY_NOTFOUND_002`，写入后失效该主体的缓存，跨实例的失效靠 TTL 收敛。清单依赖 `persistence.relational@1`。

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

## 一致性

授权 Provider 必须让其 `IPermissionGrantStore` 通过 `PermissionGrantStoreConformance`。

## 错误码

见 `conventions/error-codes.md` 中 `SECURITY_*`。
