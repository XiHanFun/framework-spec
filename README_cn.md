<div align="center">

<h1>XiHan Spec</h1>

<h3>XiHan Framework 与各语言 SDK 共同遵循的语言中立规范</h3>

<p>
  <a href="https://github.com/XiHanFun/framework-spec/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/XiHanFun/framework-spec/ci.yml?branch=main&style=flat-square&label=CI" /></a>
  <a href="./LICENSE"><img alt="MIT License" src="https://img.shields.io/github/license/XiHanFun/framework-spec?style=flat-square&color=green" /></a>
  <a href="https://github.com/XiHanFun/framework-spec/stargazers"><img alt="GitHub Stars" src="https://img.shields.io/github/stars/XiHanFun/framework-spec?style=flat-square&logo=github&label=Stars&color=1f6feb" /></a>
</p>

<p>
  <a href="https://framework.docs.xihanfun.com"><b>文档</b></a> ·
  <a href="#目录"><b>目录</b></a> ·
  <a href="#一致性"><b>一致性</b></a> ·
  <a href="https://github.com/XiHanFun/XiHan.Framework"><b>参考实现</b></a>
</p>

<p><a href="./README.md">English</a> | <b>简体中文</b></p>

</div>

XiHan Spec 是 XiHan Framework 的语言中立规范：原语语义与线上编码、清单 Schema、调用上下文与传播、管道阶段与排序、进程外插件协议 `xihan.plugin.v1` 与 Wasm 载体、协议映射、应用清单与打包，以及数据驱动的一致性向量和黑盒运行器。参考实现是 .NET 的 [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework)，各语言 SDK 见[生态](#生态)。

## 目录

```text
spec/
├─ rfcs/           # 提案：先 RFC，再 ADR，再实现
├─ conventions/    # 规范性：术语表、包 ID、能力 ID、操作 ID、错误码、Baggage 键、批量优先
├─ manifest/       # 规范性：xihan.manifest.json 的 JSON Schema
├─ plugin/         # 规范性：插件协议 plugin.proto 与握手、生命周期、调用文本；Wasm 载体 wasm.md
├─ protocol/       # 规范性：HTTP / gRPC / MCP / CloudEvents / 命令行 映射
├─ packaging/      # 规范性：应用清单 xihan.app.json 与 xihan pack 的产物映射
├─ provider/       # 规范性：远程 Provider 的通用规则与各能力的操作集（caching.distributed）
├─ migration/      # 规范性：xihan migrate 的通用约定（命令、标记、报告格式）与各来源版本的规则集
└─ conformance/    # 一致性向量（vectors/）与黑盒运行器（Go 模块，见 conformance/README.md）
```

## 规范性与信息性

| 规范性（各语言必须一致） | 信息性（各语言自选） |
| --- | --- |
| 原语语义与线上编码 | 内部 API 形态 |
| 清单 Schema | 包管理与目录布局 |
| 上下文传播键与载体 | 日志实现 |
| 管道阶段名与排序规则 | 具体中间件 |
| 插件协议 | 隔离机制 |
| 协议映射 | 服务器实现 |
| 术语表、错误码、能力 ID、操作 ID、包 ID 命名 | 代码风格 |
| 批量优先：集合入参、原生批量、部分失败语义（`conventions/batch-first.md`） | 批次大小 |

## 一致性

| 组成 | 位置 | 说明 |
| --- | --- | --- |
| 一致性向量 | `conformance/vectors/` | JSON 向量，按主题分目录（`primitives/`、`manifest/`、`pipeline/`、`operations/`、`plugin/`、`security/`、`provider/`）；各语言 SDK 的单元测试直接读取这些文件，Go 可以 `go get github.com/xihanfun/framework-spec/conformance` 后经 `conformance.Vectors` 嵌入读取 |
| 黑盒运行器 | `conformance/cmd/xihan-conformance` | 语言中立，以宿主身份启动任意语言写的进程外插件；运行器本身用 Go 写成 |

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
```

| 命令 | 作用 |
| --- | --- |
| `xihan-conformance plugin --dir <插件目录>` | 按 `plugin/handshake.md`、`lifecycle.md`、`invocation.md` 核对握手行与版本协商、无共同版本时拒绝、会话令牌、清单一致性、Configure / Start / Health / WatchHealth / Stop 与宽限内退出、未知操作 `Unimplemented`、声明的操作都返回 `status`、订阅投递确认、stdin 关闭即退出 |
| `xihan-conformance vectors` | 校验向量文件本身 |

各向量文件的内容、规则包、运行器的用例与选项见 [conformance/README.md](./conformance/README.md)。

## 消费方式

规范文件按相对路径互相链接，整个仓库可以作为 git submodule 放进实现仓库：

```bash
git submodule add https://github.com/XiHanFun/framework-spec.git spec
git submodule update --init
```

XiHan.Framework 引用到 `framework/spec`，各语言 SDK 引用到 `spec`。`plugin/plugin.proto` 用各语言的 protobuf 工具链生成；仓库根的 `buf.yaml` 定义 lint 与破坏性变更检查，`buf.gen.yaml` 为运行器生成 Go 代码。

## RFC

公开契约先进 RFC，流程见[参与](#参与)。状态一栏里参考实现的路径相对 XiHan.Framework 的 `framework/src`。

| RFC | 标题 | 状态 |
| --- | --- | --- |
| [0001](./rfcs/0001-primitives.md) | 原语层 | 已采纳（参考实现 `XiHan.Framework.Primitives`） |
| [0002](./rfcs/0002-manifest.md) | 清单 | 已采纳（参考实现 `Kernel.Capabilities`） |
| [0003](./rfcs/0003-invocation-context.md) | 调用上下文与传播 | 已采纳（载体映射随协议适配器实现） |
| [0004](./rfcs/0004-pipeline-stages.md) | 管道阶段与排序 | 已采纳（参考实现 `Kernel.Pipeline`） |
| [0005](./rfcs/0005-persistence.md) | 领域协议与持久化 | 已采纳（参考实现 `Domain`、`Persistence.Abstractions`，套件 `Persistence.Conformance`） |
| [0006](./rfcs/0006-caching.md) | 缓存、分布式锁与幂等 | 已采纳（参考实现 `Caching.Abstractions`、`Caching.InMemory`、`Caching.Redis`、`Caching.Hybrid`，套件 `Caching.Conformance`） |
| [0007](./rfcs/0007-multitenancy.md) | 多租户 | 已采纳（参考实现 `MultiTenancy.Abstractions`、`MultiTenancy.InMemory`，套件 `MultiTenancy.Conformance`）；Provider `MultiTenancy.Database` |
| [0008](./rfcs/0008-security.md) | 认证与授权 | 已采纳（参考实现 `Security.Abstractions`、`Security.InMemory`、`Security.Jwt`、`Security.OpenIdConnect`，套件 `Security.Conformance`）；Provider `Security.Database`、`Security.Ip2Region`（IP 归属地） |
| [0009](./rfcs/0009-messaging.md) | 消息、CloudEvents 信封与发件箱 | 已采纳（参考实现 `Messaging.Abstractions`、`Messaging.InMemory`、`Messaging.RabbitMQ`、`Messaging.Kafka`、`Messaging.Redis`，套件 `Messaging.Conformance`） |
| [0010](./rfcs/0010-localization.md) | 本地化：默认英文与 10 种内置区域 | 已采纳（参考实现 `Localization.Abstractions`、`Localization.InMemory`）；Provider `Localization.Database`，套件 `Localization.Conformance` |
| [0011](./rfcs/0011-operations.md) | 操作契约与 XGI 消息 | 已采纳（参考实现 `Kernel.Operations`） |
| [0012](./rfcs/0012-objectstorage.md) | 对象存储 | 已采纳（参考实现 `ObjectStorage.Abstractions`、`ObjectStorage.FileSystem`、`ObjectStorage.S3`，套件 `ObjectStorage.Conformance`） |
| [0013](./rfcs/0013-scheduling.md) | 调度与领导选举 | 已采纳（2026-10-26 修订：作业存储、动态作业、多触发器、misfire、暂停、历史、集群分发；参考实现 `Scheduling.Abstractions`、`Scheduling.InMemory`、`Scheduling.Database`，套件 `Scheduling.Conformance`） |
| [0014](./rfcs/0014-configuration.md) | 远程配置与密钥 | 已采纳（参考实现 `Configuration.Abstractions`、`Configuration.Nacos`，套件 `Configuration.Conformance`；默认实现是 .NET 原生文件与环境变量，无包） |
| [0015](./rfcs/0015-auditing.md) | 审计 | 已采纳（参考实现 `Auditing.Abstractions`、`Auditing.InMemory`，套件 `Auditing.Conformance`）；Provider `Auditing.Database` |
| [0016](./rfcs/0016-packaging.md) | 应用清单与打包 | 已采纳（参考实现 `Cli` 的 `xihan pack`；规范 `packaging/`） |
| [0017](./rfcs/0017-plugin-protocol.md) | 进程外插件协议 `xihan.plugin.v1` | 已采纳（规范 `plugin/`；参考实现 `Plugins.OutOfProcess`，套件 `Plugins.Conformance`） |
| [0018](./rfcs/0018-wasm-plugins.md) | Wasm 插件 ABI（Experimental）：同一套消息经导出函数 + 线性内存 | 已采纳（规范 `plugin/wasm.md`；参考实现 `Plugins.Wasm`，套件 `Plugins.Conformance`） |
| [0019](./rfcs/0019-remote-providers.md) | 远程 Provider：子系统能力经插件操作提供，首个操作集 `caching.distributed@1` | 已采纳（规范 `provider/`；适配器 `Caching.Remote`） |
| [0020](./rfcs/0020-migration.md) | 版本迁移：`xihan migrate <来源版本>` 按规则集做语法级重写，不做运行期兼容桥 | 已采纳（规范 `migration/README.md`；实现 `XiHan.Framework.Cli`） |
| [0021](./rfcs/0021-workflow.md) | 工作流：持久化执行 | 已采纳（2026-10-26 修订：触发式启动、事件网关、外部任务、HTTP 与补偿活动、跳转与迁移、认领 / 退回 / 催办、提醒与审批阈值、保留期清理；参考实现 `Workflow.Abstractions`、`Workflow.InMemory`、`Workflow.Database`，套件 `Workflow.Conformance`） |
| [0022](./rfcs/0022-ai.md) | AI：聊天、嵌入、工具与知识 | 已采纳（参考实现 `AI.Abstractions`、`AI.OpenAI`、`AI.VectorData`，套件 `AI.Conformance`） |
| [0023](./rfcs/0023-observability.md) | 可观测性：导出与运行信息 | 已采纳（参考实现 `Observability.Abstractions`、`Observability.Host`、`Observability.OpenTelemetry`；运行信息不启动外部命令，取不到的项见正文） |

## 版本

规范以语义化版本打标签（`v1.x.y`）：`v1` 期间只增不改，破坏性变更进 `v2` 并与 `v1` 并存，对应原语层的冻结规则（RFC-0001）。实现仓库以 submodule 指向具体提交或标签。

## 生态

| 项目 | 说明 |
| --- | --- |
| [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework) | .NET 参考实现 |
| [framework-sdk-go](https://github.com/XiHanFun/framework-sdk-go) | Go SDK |
| [framework-sdk-typescript](https://github.com/XiHanFun/framework-sdk-typescript) | TypeScript SDK |
| [framework-sdk-rust](https://github.com/XiHanFun/framework-sdk-rust) | Rust SDK |
| [framework-sdk-java](https://github.com/XiHanFun/framework-sdk-java) | Java SDK |
| [framework-sdk-python](https://github.com/XiHanFun/framework-sdk-python) | Python SDK |
| [framework-sdk-php](https://github.com/XiHanFun/framework-sdk-php) | PHP SDK |

SDK 仓库按 `framework-sdk-<语言>` 命名（ADR-0010）。SDK 成熟度：Reference（.NET）、Stable（通过全部向量）、Experimental（部分向量）、Community（社区维护）。

## 参与

欢迎提交 [Issue](https://github.com/XiHanFun/framework-spec/issues) 与 Pull Request。公开契约的变更走 RFC 流程：

1. 在 `rfcs/` 新建 `NNNN-<slug>.md`，状态 `草案`。
2. 评审通过后写 ADR（XiHan.Framework 的 `plans/adr/`），RFC 状态改为 `已采纳`，正文并入对应规范性目录。
3. 实现与一致性向量随 RFC 一起合并；没有 RFC 的公开契约变更不合并。

## 许可证

[MIT](./LICENSE) © 2021-Present XiHanFun and contributors。
