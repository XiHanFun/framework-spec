# RFC-0016 应用清单与打包

> 状态：已采纳（2026-11-01 修订：绑定类型词表与 Provider 声明的对照；2026-11-01 修订：挂载配置目录的位置；2026-11-01 修订：资源绑定优先于配置节；2026-11-01 修订：Kafka `stream` 与 Nacos `configuration:remote` 绑定可打包；2026-11-01 修订：工具链的 `xihan mcp` 知识库服务与 `xihan setup`） | 日期：2026-10-23 | 对应 ADR：ADR-0075（挂载配置目录的位置、资源绑定优先）、ADR-0076（Kafka 与 Nacos 绑定可打包、xihan mcp / setup） | 参考实现：`XiHan.Framework.Cli`（`xihan pack`） | 规范性文本：`spec/packaging/app-manifest.md`、`spec/packaging/app.schema.json`

## 摘要

应用在仓库里放一份 `xihan.app.json` 描述自己：名字、项目、Profile、镜像、协议端口、资源绑定、配置挂载、探针、停机宽限、资源与伸缩、领导租约。`xihan pack` 只读这份清单，生成可直接使用的部署产物：Dockerfile、Kustomize 清单、Compose 文件与 .NET Aspire AppHost 登记代码。清单是唯一输入；产物每次整份重新生成，不手改。

## 清单

字段与约束见 `spec/packaging/app-manifest.md`；JSON Schema 见 `spec/packaging/app.schema.json`。要点：

- `name` 是 DNS 标签（`^[a-z][a-z0-9-]{0,62}$`），同时作为镜像仓库名、k8s 资源名与 Compose 项目名的默认值。
- `bindings` 的类型限定为 `connection:relational`、`cache:distributed`、`queue`、`blob`、`stream`、`configuration:remote`，与清单 `bindings` 的资源类型一致（RFC-0002）。每个绑定对应一个 k8s Secret（按 Service Binding 规范挂到 `SERVICE_BINDING_ROOT`）、一个 Compose 依赖服务与一个 Aspire 资源。这六种类型与框架 Provider 声明的绑定一一对应：持久化（EF Core、SqlSugar）`connection:relational`（绑定名 `db`），`Caching.Redis` `cache:distributed`（`cache`），`Messaging.RabbitMQ` `queue`（`queue`），`ObjectStorage.S3` `blob`（`objectstorage`），`Messaging.Kafka` `stream`（`stream`），`Configuration.Nacos` `configuration:remote`（`nacos`）。清单里的绑定名要与 Provider 读取的绑定名相同，Provider 才取得到注入的值。
- `shutdown.graceSeconds` 同时写进 `XiHan:Kernel:StopTimeoutSeconds` 与 `terminationGracePeriodSeconds`（后者多 5 秒，让内核先完成 Draining 再被 kubelet 终止）。
- `probes` 映射为 startup / readiness / liveness 探针，路径固定为宿主的 `/health/startup`、`/health/ready`、`/health/live`。
- `config` 把列出的文件做成 ConfigMap 挂到 `mountPath`，并设环境变量 `XIHAN_CONFIG_PATH`；宿主按该目录装载并监听文件（P3-12 实现）。目录里的文件是部署侧的文件配置，紧接 `appsettings.{环境}.json` 插入，位置见下文「挂载的配置目录」。
- `leader.lease` 为调度器的 Lease 名：生成 ServiceAccount 与只含 `coordination.k8s.io/leases` 的 Role，并设 `XiHan:Scheduling:Kubernetes:Lease`（`Scheduling.Kubernetes` 随 P3-12 提供）。

## 资源绑定与配置节

绑定（`SERVICE_BINDING_ROOT` 目录或 `XiHan:Bindings:<名字>`，内核 `BindingResolver` 解析）来自部署环境，Provider 的配置节来自应用。两者给出同一项连接信息时：

1. 绑定给出的连接信息覆盖配置节里的同类键；绑定没给出的键（前缀、超时、分组、凭据等其余设置）照读配置节。
2. 绑定的主值为空或只有空白时不算给出了连接，照读配置节；绑定与配置节都没有连接时装配失败（`ArgumentException`），不静默退回缺省地址。
3. 绑定整体来自一个来源（目录或配置节），同名时配置节整体覆盖目录来源，不逐属性合并（RFC-0002 的绑定解析）。

框架 Provider 的对照：

| Provider | 绑定（类型） | 主值覆盖 | 属性覆盖 |
| --- | --- | --- | --- |
| `Caching.Redis` | `cache`（`cache:distributed`） | `XiHan:Caching:Redis:Configuration` | — |
| `Messaging.RabbitMQ` | `queue`（`queue`） | `XiHan:Messaging:RabbitMQ:Configuration` | — |
| `Messaging.Kafka` | `stream`（`stream`） | `XiHan:Messaging:Kafka:BootstrapServers` | — |
| `Configuration.Nacos` | `nacos`（`configuration:remote`） | `XiHan:Configuration:Nacos:ServerAddresses` | — |
| `ObjectStorage.S3` | `objectstorage`（`blob`） | `XiHan:ObjectStorage:S3:ServiceUrl` | `region`、`accessKey`、`secretKey`、`forcePathStyle` 覆盖同名键 |

持久化 Provider（EF Core、SqlSugar）不从配置节读连接，绑定原样交给应用传入特性的配置委托。命名存储实例不读绑定（RFC-0012）。

## 挂载的配置目录

宿主（`Hosting.AspNetCore`、`Hosting.CommandLine`）的 `AddXiHan` 在装配内核之前处理 `XIHAN_CONFIG_PATH` 指向的目录：

1. 只取顶层 `*.json`，跳过以 `.` 开头的条目（ConfigMap 的 `..data` 等）；`appsettings.json` 最先，其余按文件名序数随后，后者覆盖前者。目录不存在或环境变量为空时什么都不加。
2. 这些文件以 `reloadOnChange` 装载，紧接构建器里最后一个 appsettings 文件源（`appsettings.json` 或 `appsettings.{环境}.json`，按文件名认，不区分大小写）插入。默认构建器的配置源因此从低到高是：`appsettings.json` → `appsettings.{环境}.json` → 挂载目录里的文件 → 用户机密（仅开发环境）→ 环境变量 → 命令行参数 → 应用自己添加的配置源（远程配置、密钥等）；后者覆盖前者。挂载文件覆盖 appsettings，环境变量与命令行覆盖挂载文件——Dockerfile 与 Deployment 以环境变量写的 `XiHan__Kernel__StopTimeoutSeconds` 不会被 ConfigMap 里的同名键改掉。
3. 构建器里没有 appsettings 文件源（应用关掉了默认配置）时追加在末尾，顺序由应用自己组织。
4. 文件列表在启动时确定，运行中新增的文件要重启才装载；已装载文件的内容变化触发重载令牌。

## 产物

| 产物 | 路径 | 说明 |
| --- | --- | --- |
| Dockerfile | `Dockerfile`、`Dockerfile.dockerignore`（BuildKit 读与 Dockerfile 同目录的同名忽略文件） | 多阶段：`sdk:10.0` 发布 → `aspnet:10.0-noble-chiseled`（AOT 时 `runtime-deps:10.0-noble-chiseled` 并在构建阶段装 clang）；构建阶段另外编译就绪探针 `/xihan/healthprobe`，供 Compose 健康检查探测 `/health/ready`（chiseled 镜像里没有 shell 与 curl，见 `app-manifest.md`「就绪探针」）；非 root（`USER $APP_UID`）；Kestrel 端点与停机超时以环境变量写入 |
| Kustomize | `kustomize/*.yaml` | Deployment（探针、宽限、安全上下文、只读根文件系统、绑定与配置挂载）、Service、ConfigMap、HPA（有 `scaling` 时）、PDB（副本大于 1 时）、ServiceAccount + Role + RoleBinding（有 `leader` 时）、`bindings.example.yaml`（Secret 示例，不在 kustomization 里） |
| Compose | `compose.yaml` | 应用服务（`healthcheck` 以就绪探针请求 `/health/ready`）+ 每个绑定一个依赖服务（PostgreSQL、Redis、RabbitMQ、MinIO、单节点 KRaft 的 Kafka、standalone 的 Nacos），以 `XiHan__Bindings__<name>__type/value` 注入，依赖服务健康后应用才启动 |
| Aspire | `aspire/XiHanAppHost.g.cs` | `Add<Name>(this IDistributedApplicationBuilder)`：按绑定添加资源并把连接串注入 `XiHan__Bindings__<name>__value` |

## 绑定的依赖服务

每种绑定类型在 Compose 与 Aspire 里生成什么、注入的主值是什么，见 `app-manifest.md`「映射」的类型表；规则：

1. 注入的主值与对应 Provider 读取绑定主值的形式一致：关系库是连接串，Redis 与 Kafka 是 `host:port`（Kafka 即引导地址），RabbitMQ 是 `amqp://` URI，MinIO 与 Nacos 是 `http://` URL（Nacos 地址没有路径时用缺省上下文路径 `/nacos`）。
2. 新增的依赖服务镜像固定版本（不写 `latest`），每个依赖服务都有 `healthcheck`，应用以 `depends_on: condition: service_healthy` 等它们健康。JVM 服务（Kafka、Nacos）另设 `start_period`，启动期内的探测失败不计入重试。
3. Kafka 为单节点 KRaft：同一进程兼任 broker 与 controller，对外监听地址写依赖服务名（`PLAINTEXT://<绑定名>:9092`），Compose 网络里的应用拿到的元数据才指向可达的主机；内部主题的副本数为 1。Nacos 为 standalone（内嵌存储），数据目录挂命名卷。
4. Kustomize 不生成依赖服务：集群里的数据库、缓存、代理与配置中心由部署方提供，每个绑定只生成 Secret 示例（`type` 与 `uri`）与挂载。
5. Aspire 有官方集成的类型用集成（PostgreSQL、Redis、RabbitMQ、Kafka），注入资源的连接串并 `WithReference`；没有集成的（MinIO、Nacos）以固定镜像的容器资源运行，注入它的 HTTP 端点。

## 命令

`xihan pack [--manifest <path>] [--output <dir>] [--only dockerfile,kustomize,compose,aspire]`。默认清单 `./xihan.app.json`，默认输出清单所在目录下的 `deploy/`。成功时逐行输出写出的文件（相对输出目录），退出码 0；清单不存在 → `CLI_NOTFOUND_001`（3），不合法 → `CLI_VALIDATION_001`（2，`detail` 列出全部问题）。

`xihan graph` / `pipeline` / `operations` / `invoke` 转发给应用自己的命令行宿主（`dotnet run --project <project> -- <command> …`），项目取 `--project`，否则取当前目录 `xihan.app.json` 的 `project`，否则交给 `dotnet run` 自己找。

## 工具链：编码助手的知识库服务

`xihan mcp [--path <项目目录>] [--locale <区域>]` 以 stdio 提供框架离线知识库的 MCP 服务（服务名 `xihan-framework`，协议与传输同 `spec/protocol/mcp.md` 的 stdio 传输），供编码助手按工具版本查证框架。

- 数据随工具构建、与工具版本一致：包（包 ID、环层、说明、框架包 / 第三方包 / 共享框架依赖）与全部公开特性的清单（RFC-0002 的字段）、各包 Options 读取的配置键（完整路径、类型、缺省值、必填、上下限或枚举取值），以及文档站、RFC 与约定、各包 README 的全文。参考实现以测试核对清单与配置键与代码一致。
- 三个只读 tool，名字沿用 4.x：`xihan_framework_info`（输入 `name`：包 ID、短名、清单 ID、特性类型名或能力 ID；不给时列出全部包；能力 ID 给出提供者与依赖方）、`xihan_framework_config`（输入 `query`：配置节含子节、完整键路径、包、Options 类名或文字；不给时列出全部配置节；输出含按缺省值写出的 appsettings 片段）、`xihan_framework_search`（输入 `query` 与 `limit` 1–50，或 `page` 取一页全文）。输出是结构化 JSON。
- 查不到为 tool 结果 `isError`，文本 `CLI_NOTFOUND_003: <文案>`，文案带最接近的候选；检索参数不合法为 `CLI_VALIDATION_004`。tool 的标题与描述按 `--locale` 本地化。
- `--path`（缺省当前目录）所在项目引用的 `XiHan.Framework.*` 版本与工具不同时，在 stderr 与服务端 `instructions` 里提示改用 `dnx XiHan.Framework.Cli@<项目版本> --yes -- mcp`；stdout 只有协议消息。

`xihan setup --client <claude|cursor|vscode|codex> [--mode mcp|skill|both] [--pin <版本>] [--path <目录>] [--dry-run] [--check]` 把上述服务写进编码助手的项目配置：服务名 `xihan-framework`，命令 `dnx XiHan.Framework.Cli@<版本> --yes -- mcp`（Windows 上的 Claude Code 写成 `cmd /c dnx …`）。

| `--client` | 写入 | 技能（`--mode` 为 `skill` / `both`） |
| --- | --- | --- |
| `claude` | `.mcp.json` 的 `mcpServers` | `.claude/skills/xihan-framework/SKILL.md` |
| `cursor` | `.cursor/mcp.json` 的 `mcpServers` | `.agents/skills/xihan-framework/SKILL.md` |
| `vscode` | `.vscode/mcp.json` 的 `servers`，项带 `"type": "stdio"` | 同上 |
| `codex` | 不写文件，输出 `~/.codex/config.toml` 的 `[mcp_servers.xihan-framework]` 片段 | 同上 |

- 版本取 `--pin`，否则取项目引用的 `XiHan.Framework.*` 版本（项目文件与 `.props` 里的 `PackageReference` / `PackageVersion`，取引用最多的），再否则取工具版本；项目根是 `--path`（缺省当前目录）向上的 git 根，否则第一个含解决方案或项目文件的目录，找不到为 `CLI_NOTFOUND_004`。
- 幂等：JSON 配置只替换本服务一项，其余服务与顶层键原样保留；要合并的任何一份配置不是合法 JSON 对象（含注释的 JSONC 也算）时一个文件都不写，`CLI_VALIDATION_006`。`--client` / `--mode` 不认识为 `CLI_VALIDATION_005`（用法错误）。
- `--dry-run` 只输出要写的内容；`--check` 核对配置与技能是否与本次规划一致，不一致为 `CLI_CONFLICT_001`，`detail` 列出各项。

## 错误码

见 `conventions/error-codes.md` 中 `CLI_*`。
