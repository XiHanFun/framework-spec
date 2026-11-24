<div align="center">

<h1>XiHan Spec</h1>

<h3>XiHan Framework 与各语言 SDK 共同遵循的语言中立规范</h3>

<p>
  <a href="https://github.com/XiHanFun/framework-spec/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/XiHanFun/framework-spec/ci.yml?branch=main&style=flat-square&label=CI" /></a>
  <a href="./LICENSE"><img alt="MIT License" src="https://img.shields.io/github/license/XiHanFun/framework-spec?style=flat-square&color=green" /></a>
  <a href="https://github.com/XiHanFun/framework-spec/stargazers"><img alt="GitHub Stars" src="https://img.shields.io/github/stars/XiHanFun/framework-spec?style=flat-square&logo=github&label=Stars&color=1f6feb" /></a>
</p>

<p>
  <a href="https://framework.docs.xihanfun.com/appendix/spec"><b>文档</b></a> ·
  <a href="#目录"><b>目录</b></a> ·
  <a href="./conformance/README.md"><b>一致性</b></a> ·
  <a href="https://github.com/XiHanFun/XiHan.Framework"><b>参考实现</b></a>
</p>

<p><a href="./README.md">English</a> | <b>简体中文</b></p>

</div>

XiHan Spec 是 XiHan Framework 的语言中立规范：原语语义与线上编码、清单 Schema、调用上下文与传播、管道阶段与排序、进程外插件协议 `xihan.plugin.v1` 与 Wasm 载体、协议映射、应用清单与打包，以及数据驱动的一致性向量和黑盒运行器。参考实现是 .NET 的 [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework)，各语言 SDK 见[生态](#生态)。

## 目录

| 目录 | 内容 |
| --- | --- |
| `rfcs/` | 提案：先 RFC，再 ADR，再实现 |
| `conventions/` | 规范性：规范用语、术语表、包 ID、能力 ID、操作 ID、错误码、Baggage 键、批量优先 |
| `manifest/` | 规范性：`xihan.manifest.json` 的 JSON Schema |
| `plugin/` | 规范性：插件协议 `plugin.proto` 与握手、生命周期、调用文本；Wasm 载体 `wasm.md` |
| `protocol/` | 规范性：HTTP / gRPC / MCP / CloudEvents / 命令行 映射 |
| `packaging/` | 规范性：应用清单 `xihan.app.json` 与 `xihan pack` 的产物映射 |
| `provider/` | 规范性：远程 Provider 的通用规则与各能力的操作集（`caching.distributed`） |
| `migration/` | 规范性：`xihan migrate` 的通用约定（命令、标记、报告格式）与各来源版本的规则集 |
| `conformance/` | 一致性档表（`profiles.json`）、向量（`vectors/`）与黑盒运行器（Go 模块，见 [conformance/README.md](./conformance/README.md)） |

正文里「必须」「不得」「应当」「可以」各有多强的约束力，见 [conventions/normative-language.md](./conventions/normative-language.md)。

## 安装

规范文件按相对路径互相链接，整个仓库作为 git submodule 放进实现仓库（XiHan.Framework 放在 `framework/spec`，各语言 SDK 放在 `spec`）：

```bash
git submodule add https://github.com/XiHanFun/framework-spec.git spec
```

## 快速开始

安装黑盒运行器，然后核对任意语言写的进程外插件，或校验向量文件本身：

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
xihan-conformance plugin --dir <插件目录>
xihan-conformance vectors
```

## 文档

| 去处 | 内容 |
| --- | --- |
| [规范与多语言](https://framework.docs.xihanfun.com/appendix/spec) | 规范性与信息性、一致性档、向量与运行器、引用规范、版本、各语言 SDK、变更流程与 RFC 列表 |
| [进程外插件](https://framework.docs.xihanfun.com/plugins/out-of-process) | 用任意语言写插件并由 .NET 宿主承载 |
| SDK 文档页：[Go](https://framework.docs.xihanfun.com/sdks/go) · [TypeScript](https://framework.docs.xihanfun.com/sdks/typescript) · [Rust](https://framework.docs.xihanfun.com/sdks/rust) · [Java](https://framework.docs.xihanfun.com/sdks/java) · [Python](https://framework.docs.xihanfun.com/sdks/python) · [PHP](https://framework.docs.xihanfun.com/sdks/php) | 各 SDK 的安装与用法、与宿主共用的行为、语言映射 |
| [conformance/README.md](./conformance/README.md) | 各向量文件的内容、规则包、运行器的用例与选项 |
| [CHANGELOG.md](./CHANGELOG.md) | 规范的改动，每条标明 PATCH、MINOR 或 MAJOR 与 ADR |

## 状态

规范以独立的语义化版本 `vX.Y.Z` 打标签（RFC-0024），`v1.0.0` 之前打 `v1.0.0-rc.N`。规范性内容按 [`conformance/profiles.json`](./conformance/profiles.json) 的一致性档划分：XiHan.Framework 认领 `plugin` 之外的全部档，六个 SDK 认领 `core` 与 `plugin`（Go 另有 `provider:caching.distributed@1`），都没有缺口。档、成熟度与各实现的现状见[文档页](https://framework.docs.xihanfun.com/appendix/spec)。

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

SDK 仓库按 `framework-sdk-<语言>` 命名（ADR-0010）。

## 参与

欢迎提交 [Issue](https://github.com/XiHanFun/framework-spec/issues) 与 Pull Request。公开契约的变更先在本仓库走 RFC，再在 XiHan.Framework 写 ADR，然后才进各实现（见[变更流程](https://framework.docs.xihanfun.com/appendix/spec#变更流程)）。开发约定、门禁与发布流程见 [AGENTS.md](./AGENTS.md)。

## 许可证

[MIT](./LICENSE) © 2021-Present XiHanFun and contributors。
