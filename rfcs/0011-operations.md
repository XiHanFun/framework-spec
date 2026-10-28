# RFC-0011 操作契约与 XGI

> 状态：已采纳（2026-10-28 修订：Schema 命名——封闭泛型带实参名、字符串枚举进 `$defs`；2026-10-28 修订：程序集级 Expose 缺省与只读声明） | 日期：2026-10-22 | 对应 ADR：0034、0041 | 参考实现：`XiHan.Framework.Kernel.Operations` | 一致性：`conformance/vectors/operations/`（随协议适配器补）

## 摘要

业务以操作为单位声明：一个类实现 `IOperation<TRequest, TResponse>`，带操作 ID、稳定性、暴露的协议与授权要求。协议适配器把线上消息翻译成 XGI（XiHan Gateway Interface）请求交给调用器，调用器让它经过完整管道并在 Handle 阶段执行，再把结果翻译回各协议。一份契约，多协议暴露；内核零反射，请求与响应用源生成的 JSON 元数据编解码。

## 操作 ID

见 `conventions/operation-ids.md`：`<resource>.<verb>`，资源是小写复数名词（连字符分隔），动词小写；文法 `^[a-z][a-z0-9]*(-[a-z0-9]+)*\.[a-z][a-z0-9]*(-[a-z0-9]+)*$`（`OperationId`）。

## 契约

```csharp
[Operation("orders.create", Stability = Stability.Stable, Expose = Expose.Http | Expose.Mcp, Permission = "orders.write")]
public sealed class CreateOrder : IOperation<CreateOrderRequest, OrderResponse>
{
    public ValueTask<Result<OrderResponse>> HandleAsync(CreateOrderRequest request, InvocationContext context) { … }
}
```

- `IOperation<TRequest, TResponse>.HandleAsync` 返回 `Result<TResponse>`；可恢复失败走 `Result`，不抛异常。成功值不能为 `null`。
- `[Operation]` 是源生成器与工具链的输入，运行期不读；运行期形态是 `OperationContract(id, stability, expose, policy, permission, readOnly)`，`policy` 与 `permission` 二选一。
- 暴露协议的缺省（ADR-0041）：操作没写 `Expose` 时取程序集级 `[assembly: OperationDefaults(Expose = …)]`，程序集也没声明时为 `Expose.All`；生成器把落定的值写进契约，运行期登记表与之一致。生成器诊断 `XHG004`（缺省关闭）在操作依赖隐式的 `Expose.All` 时报告，应用以 `dotnet_diagnostic.XHG004.severity = error` 打开即可强制写明。
- 只读（ADR-0041）：`OperationContract.IsReadOnly` 为真当且仅当动词是 `get` / `list`，或操作以 `ReadOnly = true` 声明了只读的非标准动词（`tree`、`list-online` 等查询变体）；`create` / `update` / `delete` 不能声明只读（生成器 `XHG005`，契约构造抛出 `ArgumentException`）。协议据此推导行为：MCP 的只读与幂等提示（`protocol/mcp.md`）、HTTP 方法（`conventions/operation-ids.md`）；没有约定的 `get-*` / `list-*` 前缀规则，只认显式声明。
- 登记：特性在 `Configure` 里 `context.AddOperation<CreateOrder, CreateOrderRequest, OrderResponse>(contract, jsonContext.CreateOrderRequest, jsonContext.OrderResponse)`；操作按作用域注册到容器，描述进入 `OperationRegistry`（ID 唯一，容量 10000）。`Kernel.Generators` 从 `[Operation]` 生成这段登记代码（`GeneratedOperations.Add<类名>` 与 `AddGeneratedOperations`）与 Schema，特性只写 `context.AddGeneratedOperations()`。
- `OperationDescriptor`：契约、归属特性、实现/请求/响应类型、可选的 `OperationSchema`，以及不经反射的 `DecodeRequest`、`EncodeResponse`、`ExecuteAsync`。
- `OperationSchema`：请求与响应的 JSON Schema（draft 2020-12）文本，两段都必须是 JSON 对象。根 `title` 与 `$defs` 定义名取类型名，封闭泛型为 `<外层名>Of<实参名>`（多个实参以 `And` 连接、嵌套递归，如 `PageResultOfUserResponse`、`EnvelopeOfListOfUserResponseAndInt32`；数组加 `Array`，可空值类型加 `Nullable` 前缀，基元类型用 CLR 名），不同实参的同一泛型是不同的 Schema；属性名按序列化上下文的命名策略，`required` 为不可空且无默认值的属性，嵌套对象进 `$defs`，枚举按整数（`JsonStringEnumConverter` 时按字符串：进 `$defs/<枚举类型名>` 为 `type: string` + `enum`，属性以 `$ref` 引用，可空为 `anyOf` 带 `null`）。请求类型属性（含位置记录的主构造参数）上的 DataAnnotations 折进 Schema：`[Required]` 进 `required` 并去掉可空（字符串还加 `minLength: 1`，`AllowEmptyStrings = true` 除外），`[Range]` → `minimum` / `maximum`，`[StringLength]` / `[MinLength]` / `[MaxLength]` / `[Length]` → 字符串的 `minLength` / `maxLength` 或集合的 `minItems` / `maxItems`，`[RegularExpression]` → `pattern`；其他注解（`[EmailAddress]` 等）不折。生成器产出，手写登记可省略；协议适配器据此发布描述（HTTP 的 OpenAPI、MCP 的 tool 描述）。

## XGI 消息

```text
OperationRequest  = Id (操作 ID) + Payload (请求类型的 UTF-8 JSON；空等价于 {})
OperationResponse = Outcome (Result) + Payload (成功时响应类型的 UTF-8 JSON；失败为空)
```

JSON 是 XGI 的规范编码；gRPC 等二进制协议在适配器里转换（`Protocols.Grpc` 以 `OperationSchema` 为 protobuf 描述，在线格式与 JSON 之间转换，`.proto` 由同一份描述生成；见 `protocol/grpc.md`），或走类型化入口避免中间编码。

## 调用器

`IOperationInvoker`：

| 方法 | 语义 |
| --- | --- |
| `InvokeAsync(context, OperationRequest)` | 未登记返回 `KERNEL_NOTFOUND_001`（不进管道）；否则把描述与载荷放进上下文，经 `App.InvokeAsync` 走完整管道，Validate 阶段校验并解码，Handle 阶段执行、编码 |
| `ExecuteAsync(context, OperationRequest)` | 调用方已在管道里（宿主端点）：上下文里已有 Validate 阶段解码的请求就直接执行，否则解码后执行，编码并写入上下文结果 |
| `InvokeAsync<TRequest, TResponse>(context, id, request)` / `ExecuteAsync<…>` | 类型化入口，不经 JSON；类型与登记不符返回 `KERNEL_VALIDATION_002`；`InvokeAsync` 把请求对象放进上下文，Validate 阶段按序列化元数据编码一次再校验 Schema |

规则：

1. 描述在管道开始前进入上下文（`context.SetOperation`）。Authorize 阶段没有显式授权要求时按契约的 `permission` / `policy` 判定（`Security.Abstractions.AuthorizationMiddleware`）。
2. 载荷不合法（非 JSON、`null`、类型不匹配）返回 `KERNEL_VALIDATION_001`，不执行操作。
3. 解码后的请求对象与执行成功的响应对象记录在上下文（`GetOperationRequest` / `GetOperationResponse`），供 Validate、Egress 与适配器使用；原始载荷记录在 `GetOperationPayload`。
4. 宿主端点在进入管道前就绑定载荷：`Protocols.Http` 的端点元数据携带 `OperationPayloadBinder`，`UseXiHan()` 据此在构造上下文时把路由值、请求体与查询串绑定为载荷放进上下文（绑定失败直接写 Problem Details，不进管道），端点内 `ExecuteAsync` 用 Validate 阶段解码好的请求。
5. 操作抛出的异常由 Observe 阶段翻译为 `KERNEL_UNEXPECTED_001`，与普通调用一致。
6. 宿主端点可用 `OperationDescriptor` 作为端点元数据，`Hosting.AspNetCore` 的 `UseXiHan()` 在构造上下文时把它放入上下文，端点内再 `ExecuteAsync`。

## 校验

Validate 阶段的内置中间件 `ValidationMiddleware`（内核，`KernelOptions.UseBuiltInMiddleware` 控制）对带操作描述的调用按序执行，任一步失败即以 VALIDATION 种类的结果结束调用、不进 Idempotency / Transaction / Handle：

1. 取 JSON 载荷：原始调用用上下文里的载荷（空载荷视为 `{}`）；类型化调用只有请求对象，按描述的 `RequestInfo` 编码一次；都没有（插件代理的类型化调用）则跳过 Schema 校验。
2. 描述带 `OperationSchema` 时按请求 Schema 校验载荷：支持 `type`（含 `[type, "null"]`）、`properties`、`required`、`additionalProperties`、`items`、`enum`、`const`、`anyOf` / `oneOf` / `allOf`、`$ref`（`#/$defs/…`）、`minimum` / `maximum` / `exclusiveMinimum` / `exclusiveMaximum`、`minLength` / `maxLength`、`pattern`、`minItems` / `maxItems`；`format` 等其他关键字不断言。违规以 `$.path: reason` 列在 `KERNEL_VALIDATION_003` 的 `violations` 细节里（最多 32 条），`reason` 是第一条；载荷不是 JSON 返回 `KERNEL_VALIDATION_001`。
3. 解码（原始调用）并记录请求对象；请求实现 `IValidatableObject` 时调用 `Validate(ValidationContext)`（上下文带请求类型名与调用作用域的服务提供器），每条 `ValidationResult` 按成员名列为违规，返回 `KERNEL_VALIDATION_004`。`ErrorMessage` 是应用文案，应用按 RFC-0010 用消息键取（`key.L()`）。

`ExecuteAsync` 不经管道，不校验；需要校验的宿主路径按规则 4 先绑定载荷再进管道。

## 协议映射

见 `conventions/operation-ids.md` 的默认推导与显式覆盖；`Expose` 决定适配器是否发布该操作（`Ai` 让它成为 `ai.tools@1` 目录里的模型工具，RFC-0022）；`Stability.Internal` 的操作不对外发布。适配器发布描述文档时（如 `Protocols.Http` 的 `MapXiHanOpenApi()`、`Protocols.Mcp` 的 `tools/list`）只读登记表与 `OperationSchema`，不读源码、不反射类型。

面向人的操作标题与描述不写在契约里：放在消息目录，键为 `<操作 ID>.title` 与 `<操作 ID>.description`（ID 里的 `-` 换成 `_`，如 `order_lines.list.description`），由适配器按调用方区域取出（MCP tool 的 `title` / `description`）；没有就不给。

## 源生成器

`Kernel.Generators` 以分析器形式引用（`OutputItemType="Analyzer" ReferenceOutputAssembly="false"`），不进入运行时依赖；输入是 `[Operation]` 类与项目里标注 `[JsonSerializable]` 的 `JsonSerializerContext`，输出在 `RootNamespace` 下的 `GeneratedOperations`。诊断：`XHG001` 未实现 `IOperation<TRequest, TResponse>`、`XHG002` 请求或响应类型未被任何序列化上下文登记、`XHG003` 操作 ID 不合文法；任一诊断成立时不生成代码。

## 错误码

| 错误码 | 含义 |
| --- | --- |
| `KERNEL_NOTFOUND_001` | 操作未登记 |
| `KERNEL_VALIDATION_001` | 操作载荷不合法 |
| `KERNEL_VALIDATION_002` | 类型化调用的请求或响应类型与登记不符 |
| `KERNEL_OPERATION_001` | 操作 ID 重复登记（配置期异常） |
| `KERNEL_OPERATION_002` | 操作登记表已满（配置期异常） |
