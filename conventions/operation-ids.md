# 操作 ID

```text
OperationId = Resource "." Verb
Resource    = 小写复数名词，连字符分隔（orders、order-lines）
Verb        = 小写动词（create、get、list、update、delete、cancel、...）
```

示例：`orders.create`、`orders.get`、`order-lines.list`。

## 协议映射（默认推导，可显式覆盖）

| 动词 | HTTP | gRPC | MCP tool |
| --- | --- | --- | --- |
| `create` | `POST /{resource}` | `{Resource}.Create` | `{resource}_create` |
| `list` | `GET /{resource}`；请求带嵌套对象时 `POST /{resource}:list` | `{Resource}.List` | `{resource}_list` |
| `get` | `GET /{resource}/{id}`；请求不含 `id` 时 `GET /{resource}`；请求带嵌套对象时 `POST …:get` | `{Resource}.Get` | `{resource}_get` |
| `update` | `PUT /{resource}/{id}`；请求不含 `id` 时 `PUT /{resource}` | `{Resource}.Update` | `{resource}_update` |
| `delete` | `DELETE /{resource}/{id}`；请求不含 `id` 时 `DELETE /{resource}`；请求带嵌套对象时 `POST …:delete` | `{Resource}.Delete` | `{resource}_delete` |
| 其他 | `POST /{resource}/{id}:{verb}`；请求不含 `id` 时 `POST /{resource}:{verb}` | `{Resource}.{Verb}` | `{resource}_{verb}` |

HTTP 路由按请求对象是否声明 `id` 属性（序列化名，不分大小写）分实例级与集合级：有 `id` 的路径带 `/{id}` 段（`GET /orders/{id}`、`POST /orders/{id}:cancel`），没有的落在集合或单例资源上（`GET /profile`、`PUT /profile`、`POST /orders:export`；Google AIP 的两种形态）。`create` 与 `list` 恒为集合级。GET / DELETE 不读请求体，`get` / `list` / `delete` 的请求带嵌套对象字段（对象、字典、元素是对象的集合，如分页请求的 `page` 与 `conditions`）时进不了查询串，改用 POST 自定义动词（`POST /orders:list`、`POST /orders/{id}:get`）。两个操作推导出同一条路由（如无 `id` 的 `get` 与 `list`）时 HTTP 适配器启动失败（`PROTOCOLS_CONFLICT_002`），用显式覆盖解决。HTTP 适配器从操作描述的请求序列化元数据判定，不用反射；没有类型化元数据的描述（插件代理）看请求 Schema 的 `properties`，两者都没有按集合级处理。

命令行：`<resource> <verb> [--field value ...]`，即 `invoke <resource>.<verb>`（`protocol/commandline.md`）。

一份契约一份 Schema；HTTP 错误用 RFC 9457 Problem Details。
