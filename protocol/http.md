# HTTP 载体

> 规范性。参考实现：`XiHan.Framework.Hosting.AspNetCore`（上下文与错误信封）、`XiHan.Framework.Protocols.Http`（操作映射）。

## 上下文映射（RFC-0003）

| 字段 | 请求头 | 说明 |
| --- | --- | --- |
| Trace | `traceparent` | W3C；缺失或不合法时生成新的 trace-id |
| Baggage | `baggage` | W3C；只保留已登记的 `xihan.` 键 |
| Deadline | `xihan-deadline` | RFC 3339，UTC |
| Locale | `Accept-Language` | 没有显式 `xihan.locale` Baggage 时按权重协商写入 |
| Credential | `Authorization` | 原样进入凭据，由 Authenticate 阶段解释 |

## 操作映射（RFC-0011）

- 路由推导见 `conventions/operation-ids.md`；显式覆盖写在 `XiHan:Protocols:Http:Routes:<操作 ID>`，统一前缀 `XiHan:Protocols:Http:RoutePrefix`。
- 请求载荷 = 路由值 ∪ JSON 请求体 ∪ 查询串，合并成一个 JSON 对象；同名时路由值 > 请求体 > 查询串。路由值与查询串按请求类型序列化元数据里的属性类型写入：布尔、整数、浮点与数值枚举写成 JSON 字面量，集合属性写成数组（单值也是一元数组），其余写字符串；属性名不分大小写匹配并改写成序列化名。没有类型化元数据（插件代理）或找不到属性时按字符串写入，多值写成字符串数组；解析不了的值按字符串写入，由反序列化报出载荷不合法。
- 成功：`create` 201，其余 200；正文为响应类型的 JSON，`Content-Type: application/json; charset=utf-8`。
- 失败：RFC 9457 Problem Details（`application/problem+json`）：`type` = `urn:xihan:error:<错误码>`，`title` = KIND 段，`status` 由 KIND 映射（VALIDATION 400、NOTFOUND 404、CONFLICT 409、DENIED 403 / 认证失败 401——`SECURITY_DENIED_001` 未认证与 `SECURITY_DENIED_003` 凭据无效 / 过期 / 签名不符（RFC 6750 §3.1 的 `invalid_token`）、UNAVAILABLE 503、TIMEOUT 504、CANCELLED 408、其他 500；401 带 `WWW-Authenticate`：凭据无效时只质询被拒的方案，Bearer 为 `Bearer error="invalid_token"`、其他方案只写方案名，没有凭据时逐个质询已装配认证处理器的方案、不带错误码，ADR-0042），`detail` 为本地化文案，`code`、`traceId` 为扩展字段；`Content-Language` 为文案命中的区域。VALIDATION 种类的错误另带扩展成员 `errors`：对象，键是请求属性的序列化名（camelCase，嵌套用点分路径与下标，如 `lines[0].sku`；根级为空串），值是该字段的本地化消息数组（内核校验器的违规经 `kernel.validation.*` 消息键按调用方区域翻译，与 `detail` 同一通道；应用自校验的文案原样）。所有错误按需带扩展成员 `details`：对象，只收 `Error.Details` 里的字符串、布尔、数值、UUID 与时间等标量（如 `id`、`capacity`、`retryAfter`），集合与对象不输出。
- 不发布：`Expose` 不含 `Http` 的操作、`Stability.Internal` 的操作。

## OpenAPI

- `MapXiHanOpenApi(options?, pattern)` 发布 OpenAPI 3.1 文档（默认 `/openapi.json`，`application/json`），在第一次请求时从 `OperationRegistry`、权限码注册表、已装配的认证处理器与宿主的全部端点生成并缓存；端点绕过内核管道，不鉴权。`info.title` / `info.version` / `servers` 由应用的 `OpenApiOptions` 指定，缺省标题取应用名、版本取框架版本、不写 `servers`。
- 认证方案：装配了以 `Bearer` 方案工作的 `security.authentication@1` Provider（`Security.Jwt`、`Security.OpenIdConnect`）时发布 `components.securitySchemes.bearer`（`type: http`、`scheme: bearer`、`bearerFormat: JWT`）并挂全局 `security: [{ bearer: [] }]`；没有权限也没有策略要求的操作与端点标 `security: []`。权限码与策略名只走 `x-xihan-*`，不进 scope。其他方案（ApiKey）不发布。
- 操作摘要：每个操作的 `summary` 取消息目录里 `<操作 ID>.description`（ID 的 `-` 换 `_`，与 MCP `tools/list` 同一个键）在调用方区域的文案，没有就不写；装配了本地化时按 `Accept-Language` 协商出的区域分别生成与缓存，响应带 `Content-Language`，缺省区域是本地化选项的默认区域（`en-US`）；没有装配本地化时只有一份文档、没有摘要。
- 普通端点：宿主 `EndpointDataSource` 里没有 `OperationDescriptor` 元数据、有 `IHttpMethodMetadata`、带 `IAcceptsMetadata` 或 `IProducesResponseTypeMetadata` 之一、没有 `IExcludeFromDescriptionMetadata` 的路由端点进文档（Hub、健康探针、重定向、静态资源没有描述元数据，自然不进，不做名单）。路由模板参数作必填 `path` 参数（`string`，约束、可选标记与通配前缀去掉）；`IEndpointNameMetadata` 作 `operationId`，`IEndpointSummaryMetadata` / `IEndpointDescriptionMetadata` / `ITagsMetadata` 照搬；`RequireXiHanPermission` / `RequireXiHanPolicy` / `RequireXiHanAuthentication` 的要求出 `x-xihan-permission` / `x-xihan-policy`，权限码并入 `components.x-xihan-permissions` 的收录。`Accepts` / `Produces` 按内容类型写：JSON（`application/json` 或 `+json`）只引用已登记操作的根 Schema（`$ref`）；`multipart/form-data` 是含 `format: binary` 属性（`file`，多文件为 `files` 数组）的对象；表单是对象；`text/*` 与 `string` 是字符串；其余二进制按内容类型写成 `format: binary` 的字符串；有类型没写内容类型的响应按 JSON；没有类型也没有内容类型的响应没有正文；失败响应同操作一样引用 `ProblemDetails`。JSON 类型不做第二套 Schema 推导：端点以 `WithXiHanOperationSchema(request:, response:)` 显式声明同哪个操作的请求 / 响应 Schema，没有声明时按 `Accepts<T>` / `Produces<T>` 的 `T` 与已登记操作的请求 / 响应类型自动匹配（取 ID 最小且有 Schema 的操作）；被引用的根 Schema 提升到 `components/schemas/<title>`（没有 `title` 按操作 ID 拼），操作自己也改为引用它；引用不到以 `PROTOCOLS_VALIDATION_010` 生成失败，不做兜底推导。普通端点与操作共用一条路由以 `PROTOCOLS_CONFLICT_002` 生成失败。
- `paths` 按路由模板分组，方法小写；`operationId` = 操作 ID，`tags` = 资源段，非 `Stable` 的操作带 `x-xihan-stability`。
- 授权扩展（ADR-0031）：按权限码授权的操作带 `x-xihan-permission: "<权限码>"`，按策略授权的带 `x-xihan-policy: "<策略名>"`（二选一，都没有不写，与 `x-xihan-stability` 并列）；`components["x-xihan-permissions"]` 列权限定义，键为权限码、序数排序，值为 `{ "code", "displayName", "description" }`（`description` 为空省略），收录范围 = 文档里已发布操作引用的权限码 ∪ 内核 `PermissionRegistry` 已登记的权限；被引用但未登记的只有 `code`。只给 gRPC / MCP 的操作与 `Stability.Internal` 的操作不进文档，它们引用的权限码不算被引用。前端据此隐藏没有权限的入口、按权限定义生成授权界面。
- 参数：路由模板里的 `{name}` 为必填 `path` 参数；GET / DELETE 的请求 Schema 里其余属性为 `query` 参数，`required` 按 Schema；POST / PUT / PATCH 的请求 Schema 整体作为 `requestBody`（`application/json`，必填）。
- 响应：成功状态码同操作映射，正文 Schema 为响应 Schema；`default` 为 `application/problem+json`，引用 `components/schemas/ProblemDetails`（`type`、`title`、`status`、`detail`、`instance`、`code`、`traceId`，以及 `errors`：字段键 → 字符串数组；`details`：名字 → 标量）。
- Schema 里的 `$defs` 提升为 `components/schemas/<名>`（字符串枚举定义也是一个组件，属性与查询参数以 `$ref` 引用），`#/$defs/<名>` 改写为 `#/components/schemas/<名>`；同名定义以先到者为准（封闭泛型的名字带实参名，不同实参不同名）；`$schema` 不进文档。没有 `OperationSchema` 的操作只发布路径、方法与 `operationId`。
