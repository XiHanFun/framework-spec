# 能力 ID

```text
CapabilityId   = Domain "." Capability
CapabilityRef  = CapabilityId [ "@" VersionRange ] [ "?" ]
Domain         = 术语表领域词的小写形态
Capability     = 小写字母与数字，连字符分隔
VersionRange   = [ "^" ] Version  ; "1"、"^1"
Version        = %x31-39 *DIGIT  ; 不带前导零的正整数，不超过 2147483647
"?"            = 可选依赖（仅在 requires 中）
```

示例：`persistence.relational@1`、`caching.distributed@^1`、`multitenancy.context@^1?`。

## 规则

1. 依赖按能力 ID 声明，不按包名；同一能力可由多个 Provider 提供。
2. 接口版本是整数，变更只加新版本并存（`persistence.relational@2`），不改旧版本语义。版本文本不带前导零、符号与空白，在 32 位有符号整数范围内；整个文本按整串匹配，结尾的换行不例外（2026-11-16 写明，ADR-0236）。
3. 能力 ID 一经发布不改；新能力走 RFC。

## 已登记的能力

| 能力 ID | 含义 |
| --- | --- |
| `kernel.hosting` | 应用构建、特性装配、生命周期 |
| `kernel.capabilities` | 清单解析与能力图谱 |
| `kernel.pipeline` | 执行管道 |
| `kernel.diagnostics` | 诊断薄桥与审计接收器 |
| `kernel.security` | 安全原语 |
| `kernel.plugins` | 插件宿主抽象 |
| `plugins.out-of-process` | 进程外插件宿主（RFC-0017） |
| `plugins.wasm` | Wasm 插件宿主（RFC-0018，Experimental） |
| `domain.model` | 实体协议、值对象、领域事件、规约（保留，尚无实现） |
| `persistence.relational` / `persistence.document` | 仓储、工作单元、迁移（`persistence.document` 保留，尚无实现） |
| `caching.local` / `caching.distributed` | 缓存与分布式锁 |
| `messaging.queue` / `messaging.stream` | 队列与流 |
| `security.authentication` / `security.authorization` | 认证与授权 |
| `security.ip-region` | IP 归属地解析（RFC-0008，离线、可不装） |
| `security.webauthn` | 通行密钥仪式的选项生成与响应校验（RFC-0008，可不装） |
| `multitenancy.context` / `multitenancy.sharding` | 租户上下文与分片 |
| `objectstorage.blob` | 对象存储 |
| `objectstorage.multipart` / `objectstorage.presign` | 分片上传与预签名地址（RFC-0012） |
| `scheduling.jobs` / `scheduling.leader` | 任务与领导选举 |
| `configuration.remote` / `configuration.secrets` | 远程配置与密钥 |
| `observability.export` | 导出器 |
| `observability.host` | 进程与主机的运行信息（RFC-0023） |
| `localization.resources` | 本地化资源 |
| `auditing.trail` | 审计 |
| `workflow.execution` | 持久化执行 |
| `search.fulltext` | 全文检索（保留，尚无实现） |
| `ai.chat` / `ai.embedding` / `ai.tools` / `ai.knowledge` | AI 端口：聊天、嵌入、工具目录、知识库（RFC-0022） |
| `actors.virtual` | 虚拟 Actor（保留，尚无实现） |
| `hosting.aspnetcore` / `hosting.commandline` | 宿主（保留，尚无实现） |
| `protocols.http` / `protocols.grpc` / `protocols.mcp` / `protocols.cloudevents` | 协议适配器 |
| `testing` | 测试基座（保留，尚无实现） |

## 保留、尚无实现的能力

上表标了「保留，尚无实现」的 ID 已经登记，但 .NET 参考实现与各语言 SDK 里都没有哪份清单提供它（2026-11-24 标注，ADR-0276；以前登记表不区分已实现与只占了名字的）。ID 保留：不删、不改、不挪作他用（规则 3）。

| 能力 ID | 现状 |
| --- | --- |
| `domain.model` | `Domain` 是纯契约包，没有特性与清单，领域协议不作为能力提供 |
| `persistence.document` | RFC-0005 预留；没有契约，也没有 Provider，.NET 只有常量 `PersistenceCapabilities.Document` |
| `search.fulltext` | 没有 RFC，没有包 |
| `actors.virtual` | 没有 RFC，没有包 |
| `hosting.aspnetcore` / `hosting.commandline` | 宿主包存在（`Hosting.AspNetCore`、`Hosting.CommandLine`），但没有哪份清单提供这两项能力 |
| `testing` | 测试基座不带清单；这个写法只有一段，不合上面的 `CapabilityId` 文法，写不进清单的 `provides` 与 `requires` |

- 清单依赖保留的能力时求解找不到提供者：必需依赖报 `KERNEL_CAPABILITY_001`，可选依赖缺失不阻止启动、对应特性进入 `Degraded`（RFC-0002 解析规则 5、6）。
- 落地一项保留的能力走 RFC（规则 3）：定下契约与接口版本，有了提供它的清单之后从本节与上表的标注里去掉。
- `configuration.secrets` 不在此列：契约（`ISecretStore`）、接进配置的适配器与一致性套件都已定义（RFC-0014），只是 XiHan.Framework 仓库里还没有提供它的 Provider 包。
