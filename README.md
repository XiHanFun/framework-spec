# XiHan Framework Spec

XiHan Framework 的语言中立规范：原语语义与线上编码、清单 Schema、调用上下文与传播、管道阶段与排序、进程外插件协议 `xihan.plugin.v1` 与 Wasm 载体、协议映射、应用清单与打包，以及数据驱动的一致性向量和黑盒运行器。

| 实现 | 仓库 | 角色 |
| --- | --- | --- |
| .NET | [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework)（`next/`） | 参考实现，以 submodule 引用本仓库到 `next/spec` |
| Go | [framework-sdk-go](https://github.com/XiHanFun/framework-sdk-go) | 第二语言 SDK，以 submodule 引用本仓库到 `spec` |
| TypeScript | [framework-sdk-typescript](https://github.com/XiHanFun/framework-sdk-typescript) | 占位，规划中（ADR-0004：Go 之后的下一个） |
| Rust | [framework-sdk-rust](https://github.com/XiHanFun/framework-sdk-rust) | 占位，规划中（进程外与 Wasm 插件） |
| Java | [framework-sdk-java](https://github.com/XiHanFun/framework-sdk-java) | 占位，规划中 |
| Python | [framework-sdk-python](https://github.com/XiHanFun/framework-sdk-python) | 占位，规划中 |
| PHP | [framework-sdk-php](https://github.com/XiHanFun/framework-sdk-php) | 占位，规划中 |

各仓库按 `framework-sdk-<语言>` 命名（ADR-0010）；SDK 成熟度：Reference（.NET）、Stable（通过全部向量）、Experimental（部分向量）、Community（社区维护）。

## 目录

```text
spec/
├─ rfcs/           # 提案：先 RFC，再 ADR，再实现
├─ conventions/    # 规范性：术语表、包 ID、能力 ID、操作 ID、错误码、Baggage 键
├─ manifest/       # 规范性：xihan.manifest.json 的 JSON Schema
├─ plugin/         # 规范性：插件协议 plugin.proto 与握手、生命周期、调用文本；Wasm 载体 wasm.md
├─ protocol/       # 规范性：HTTP / gRPC / MCP / CloudEvents / 命令行 映射
├─ packaging/      # 规范性：应用清单 xihan.app.json 与 xihan pack 的产物映射
├─ provider/       # 规范性：远程 Provider 的通用规则与各能力的操作集（caching.distributed）
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

## 一致性

- `conformance/vectors/`：JSON 向量，按主题分目录（`primitives/`、`plugin/`、`provider/`）。各语言 SDK 的单元测试直接读取这些文件；Go 可以 `go get github.com/xihanfun/framework-spec/conformance` 后经 `conformance.Vectors` 嵌入读取。
- `conformance/cmd/xihan-conformance`：语言中立的黑盒运行器。`xihan-conformance plugin --dir <插件目录>` 以宿主身份启动任意语言写的进程外插件，按 `plugin/handshake.md`、`lifecycle.md`、`invocation.md` 核对握手行与版本协商、无共同版本时拒绝、清单一致性、Configure / Start / Health / WatchHealth / Stop 与宽限内退出、未知操作 `Unimplemented`、声明的操作都返回 `status`、订阅投递确认、stdin 关闭即退出；`xihan-conformance vectors` 校验向量文件本身。

## 消费方式

规范文件按相对路径互相链接，整个仓库可以作为 git submodule 放进实现仓库：

```bash
git submodule add https://github.com/XiHanFun/framework-spec.git spec
git submodule update --init
```

`plugin/plugin.proto` 用各语言的 protobuf 工具链生成；仓库根的 `buf.yaml` 定义 lint 与破坏性变更检查，`buf.gen.yaml` 为运行器生成 Go 代码。

## RFC 流程

1. 在 `rfcs/` 新建 `NNNN-<slug>.md`，状态 `草案`。
2. 评审通过后写 ADR（XiHan.Framework 的 `nextdocs/adr/`），RFC 状态改为 `已采纳`，正文并入对应规范性目录。
3. 实现与一致性向量随 RFC 一起合并；没有 RFC 的公开契约变更不合并。

参考实现一栏的路径相对 XiHan.Framework 的 `next/src`。

| RFC | 标题 | 状态 |
| --- | --- | --- |
| [0001](rfcs/0001-primitives.md) | 原语层 | 已采纳（参考实现 `XiHan.Framework.Primitives`） |
| [0002](rfcs/0002-manifest.md) | 清单 | 已采纳（参考实现 `Kernel.Capabilities`） |
| [0003](rfcs/0003-invocation-context.md) | 调用上下文与传播 | 已采纳（载体映射随协议适配器实现） |
| [0004](rfcs/0004-pipeline-stages.md) | 管道阶段与排序 | 已采纳（参考实现 `Kernel.Pipeline`） |
| [0005](rfcs/0005-persistence.md) | 领域协议与持久化 | 已采纳（参考实现 `Domain`、`Persistence.Abstractions`，套件 `Persistence.Conformance`） |
| [0006](rfcs/0006-caching.md) | 缓存、分布式锁与幂等 | 已采纳（参考实现 `Caching.Abstractions`、`Caching.InMemory`、`Caching.Redis`，套件 `Caching.Conformance`） |
| [0007](rfcs/0007-multitenancy.md) | 多租户 | 已采纳（参考实现 `MultiTenancy.Abstractions`、`MultiTenancy.InMemory`，套件 `MultiTenancy.Conformance`） |
| [0008](rfcs/0008-security.md) | 认证与授权 | 已采纳（参考实现 `Security.Abstractions`、`Security.InMemory`、`Security.Jwt`，套件 `Security.Conformance`） |
| [0009](rfcs/0009-messaging.md) | 消息、CloudEvents 信封与发件箱 | 已采纳（参考实现 `Messaging.Abstractions`、`Messaging.InMemory`、`Messaging.RabbitMQ`，套件 `Messaging.Conformance`） |
| [0010](rfcs/0010-localization.md) | 本地化：默认英文与 10 种内置区域 | 已采纳（参考实现 `Localization.Abstractions`、`Localization.InMemory`）；Provider `Localization.Database`，套件 `Localization.Conformance` |
| [0011](rfcs/0011-operations.md) | 操作契约与 XGI 消息 | 已采纳（参考实现 `Kernel.Operations`） |
| [0012](rfcs/0012-objectstorage.md) | 对象存储 | 已采纳（参考实现 `ObjectStorage.Abstractions`、`ObjectStorage.FileSystem`，套件 `ObjectStorage.Conformance`） |
| [0013](rfcs/0013-scheduling.md) | 调度与领导选举 | 已采纳（参考实现 `Scheduling.Abstractions`、`Scheduling.InMemory`，套件 `Scheduling.Conformance`） |
| [0014](rfcs/0014-configuration.md) | 远程配置与密钥 | 已采纳（参考实现 `Configuration.Abstractions`，套件 `Configuration.Conformance`；默认实现是 .NET 原生文件与环境变量，无包） |
| [0015](rfcs/0015-auditing.md) | 审计 | 已采纳（参考实现 `Auditing.Abstractions`、`Auditing.InMemory`，套件 `Auditing.Conformance`） |
| [0016](rfcs/0016-packaging.md) | 应用清单与打包 | 已采纳（参考实现 `Cli` 的 `xihan pack`；规范 `packaging/`） |
| [0017](rfcs/0017-plugin-protocol.md) | 进程外插件协议 `xihan.plugin.v1` | 已采纳（规范 `plugin/`；参考实现 `Plugins.OutOfProcess`，套件 `Plugins.Conformance`） |
| [0018](rfcs/0018-wasm-plugins.md) | Wasm 插件 ABI（Experimental）：同一套消息经导出函数 + 线性内存 | 已采纳（规范 `plugin/wasm.md`；参考实现 `Plugins.Wasm`，套件 `Plugins.Conformance`） |
| [0019](rfcs/0019-remote-providers.md) | 远程 Provider：子系统能力经插件操作提供，首个操作集 `caching.distributed@1` | 已采纳（规范 `provider/`；适配器 `Caching.Remote`，插件 `framework-sdk-go` 的 `caching` 包） |

## 版本

规范以语义化版本打标签（`v1.x.y`）：`v1` 期间只增不改，破坏性变更进 `v2` 并与 `v1` 并存，对应原语层的冻结规则（RFC-0001）。实现仓库以 submodule 指向具体提交或标签。

## 许可

[MIT](LICENSE)
