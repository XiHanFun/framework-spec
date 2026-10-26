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
- 失败：RFC 9457 Problem Details（`application/problem+json`）：`type` = `urn:xihan:error:<错误码>`，`title` = KIND 段，`status` 由 KIND 映射（VALIDATION 400、NOTFOUND 404、CONFLICT 409、DENIED 403 / 未认证 401、UNAVAILABLE 503、TIMEOUT 504、CANCELLED 408、其他 500），`detail` 为本地化文案，`code`、`traceId` 为扩展字段；`Content-Language` 为文案命中的区域。
- 不发布：`Expose` 不含 `Http` 的操作、`Stability.Internal` 的操作。

## OpenAPI

- `MapXiHanOpenApi()` 发布 OpenAPI 3.1 文档（默认 `/openapi.json`，`application/json`），在第一次请求时从 `OperationRegistry` 生成并缓存；端点绕过内核管道，不鉴权。
- `paths` 按路由模板分组，方法小写；`operationId` = 操作 ID，`tags` = 资源段，非 `Stable` 的操作带 `x-xihan-stability`。
- 授权扩展（ADR-0031）：按权限码授权的操作带 `x-xihan-permission: "<权限码>"`，按策略授权的带 `x-xihan-policy: "<策略名>"`（二选一，都没有不写，与 `x-xihan-stability` 并列）；`components["x-xihan-permissions"]` 列权限定义，键为权限码、序数排序，值为 `{ "code", "displayName", "description" }`（`description` 为空省略），收录范围 = 文档里已发布操作引用的权限码 ∪ 内核 `PermissionRegistry` 已登记的权限；被引用但未登记的只有 `code`。只给 gRPC / MCP 的操作与 `Stability.Internal` 的操作不进文档，它们引用的权限码不算被引用。前端据此隐藏没有权限的入口、按权限定义生成授权界面。
- 参数：路由模板里的 `{name}` 为必填 `path` 参数；GET / DELETE 的请求 Schema 里其余属性为 `query` 参数，`required` 按 Schema；POST / PUT / PATCH 的请求 Schema 整体作为 `requestBody`（`application/json`，必填）。
- 响应：成功状态码同操作映射，正文 Schema 为响应 Schema；`default` 为 `application/problem+json`，引用 `components/schemas/ProblemDetails`（`type`、`title`、`status`、`detail`、`instance`、`code`、`traceId`）。
- Schema 里的 `$defs` 提升为 `components/schemas/<名>`，`#/$defs/<名>` 改写为 `#/components/schemas/<名>`；同名定义以先到者为准；`$schema` 不进文档。没有 `OperationSchema` 的操作只发布路径、方法与 `operationId`。
