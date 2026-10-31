# RFC-0014 远程配置与密钥

> 状态：已采纳（2026-10-26 修订：Nacos Provider；2026-11-01 修订：加载与重读的失败语义） | 日期：2026-10-23 | 对应 ADR：ADR-0019（Nacos）、ADR-0068（失败语义） | 参考实现：`XiHan.Framework.Configuration.Abstractions`、`XiHan.Framework.Configuration.Nacos` | 一致性：`XiHan.Framework.Configuration.Conformance`

## 摘要

配置子系统不另造配置模型：应用仍然读 `Microsoft.Extensions.Configuration` 的 `IConfiguration`。本 RFC 只定义两个 Provider 契约与把它们接进 `IConfigurationBuilder` 的适配器：远程配置客户端（Nacos、Apollo、Consul 等）与密钥库（Vault、Key Vault、KMS 等）。默认实现是 .NET 原生的文件与环境变量，不设默认包。

## 能力

| 能力 | 含义 | 契约 |
| --- | --- | --- |
| `configuration.remote@1` | 远程配置：按命名空间取整份文档，带版本，可轮询或长轮询 | `IRemoteConfigurationClient` |
| `configuration.secrets@1` | 密钥：按名字批量读取，只读 | `ISecretStore` |

## 远程配置

- 文档 `ConfigurationDocument(Version, Values)`：`Values` 的键已是 `IConfiguration` 的键形式（`A:B:C`），Provider 负责把自己的格式（JSON、YAML、properties）展平；`Version` 非空，每次变化都不同（ETag、修订号或内容哈希）。
- `FetchAsync(namespace, knownVersion)`：`knownVersion` 为空时总是返回文档；非空且远端未变化时返回无值。实现可以长轮询：阻塞到变化或自己的窗口到期，收到取消时返回无值而不是抛出。命名空间不存在 → `CONFIGURATION_NOTFOUND_001`；不可达 → `CONFIGURATION_UNAVAILABLE_001`；远端文档不是声明的格式（JSON 不合法等）→ `CONFIGURATION_VALIDATION_001`。
- 适配器 `AddXiHanRemoteConfiguration(client, options)`：构建配置时同步加载一次（`IConfigurationProvider.Load` 是同步契约，受 `LoadTimeout` 约束）。首次加载的失败有三种：客户端返回失败结果（错误码照用）、超时（`CONFIGURATION_UNAVAILABLE_001`，原因 `load timeout`）、客户端抛出任何异常（`CONFIGURATION_UNAVAILABLE_001`，原因取异常消息）；三种一样处理——先回调 `OnError`，`Optional` 为假则抛出带该错误码的 `FrameworkException` 让启动失败（客户端抛出的异常作为内部异常），为真则该源为空。不论首次加载成败，`PollInterval`（默认 30 秒，`null` 关闭）非空时都按周期带已知版本轮询（首次失败时已知版本为空，第一次成功的轮询装入整份文档；计时用 `Clock`，默认系统时钟，测试换成可推进的时钟由用例触发轮询），拿到新版本就替换整份数据，键值有变化才触发重载令牌；轮询中客户端返回失败或抛出异常都回调 `OnError` 并保留最后一份成功的数据。`Prefix` 把文档挂到某个节下。
- `OnError` 是唯一的报告渠道（配置在日志之前构建），它自己抛出的异常没有更上一层可报告，被丢弃：不改变首次加载按 `Optional` 得出的结果，也不终止轮询或重读。密钥配置同此。

## 密钥

- 名字 `SecretName`：`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`，不含 `..`、`//`，不以 `/` 结尾。
- `GetAsync(names[])` 批量逐项返回 `SecretLookup(Name, Result<SecretValue>)`，顺序与入参一致，重复项各自作答，空集合返回空；不存在 → `CONFIGURATION_NOTFOUND_002`，无权限 → `CONFIGURATION_DENIED_001`，不可达 → `CONFIGURATION_UNAVAILABLE_002`。
- `SecretValue.ToString()` 固定返回遮蔽文本，防止进入日志。
- 适配器 `AddXiHanSecrets(store, options)`：`Mappings` 把配置键映射到密钥名，构建时一次批量读取。失败分两类：逐项失败（不存在、无权限等）每项回调一次 `OnError`；整体失败——超时（原因 `load timeout`）、密钥库抛出任何异常（原因取异常消息）、返回的条数与请求不一致——是一个 `CONFIGURATION_UNAVAILABLE_002`，回调一次。首次读取有失败时，`Optional` 为假则随后抛出 `FrameworkException`（逐项失败列出失败的配置键，整体失败带密钥库抛出的异常作为内部异常），为真则跳过失败的项（整体失败时该源为空）。不论首次读取成败，`RefreshInterval` 非空时都周期性重读（计时用 `Clock`，同上）：逐项失败的项保留旧值，整体失败（含条数不一致）回调 `OnError` 并保留全部旧值；重读到的值有变化才触发重载，同样的内容不触发（订阅者不会因轮询反复重绑）。

## Nacos Provider

`Configuration.Nacos` 实现 `IRemoteConfigurationClient`，直接用 Nacos 的 HTTP 配置接口（v1，Nacos 2.x 兼容），不依赖 SDK：

- 命名空间参数即 Nacos 的 `dataId`；`Group`（默认 `DEFAULT_GROUP`）与 `Tenant`（Nacos 命名空间 ID，默认公共）来自选项。
- 版本是内容的 MD5（小写十六进制），与 Nacos 监听接口的约定一致：监听时服务端拿报来的值与它存的内容 MD5（按 UTF-8 编码计算）逐字比较。取配置响应的 `Content-MD5` 头（HTTP 上它是内容头）原文；没有这个头时按同样的算法自己算，不得换成别的摘要，否则监听永远报告变化、退化成普通轮询。`knownVersion` 非空时先以该 MD5 长轮询监听（`LongPollTimeout`，默认 30 秒），服务端报告变化才取整份文档，窗口到期或取消返回无值。
- 文档格式 `Format`：`Json`（默认，对象按 `A:B` 展平、数组按下标、`null` 保留为空值）或 `Properties`（`a.b=c` → `a:b`，`#` / `!` 行是注释）。
- 配置了 `Username` / `Password` 时先登录取 `accessToken`，到期前自动续；`ServiceUrl` 可以是多个地址，按序尝试。

## 一致性

Provider 继承 `RemoteConfigurationClientConformance` 或 `SecretStoreConformance`，实现夹具（客户端或密钥库加写入钩子）并通过全部用例。

## 错误码

见 `conventions/error-codes.md` 中 `CONFIGURATION_*`。
