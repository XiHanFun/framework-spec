---
name: xihan-spec-conformance
description: 新增或修改 XiHan Spec 的一致性向量、规则包 conformance/rules、黑盒运行器 xihan-conformance 与一致性档表 profiles.json 时使用，适用于补用例、由 .NET 算期望值、给运行器加用例、用真实插件验证运行器；规范正文的修订用 spec-authoring，门禁命令用 testing-release。
---

# XiHan Spec 一致性

向量格式、期望值的算法、运行器内部与验证方法读 `references/vectors-and-runner.md`；用户可见的说明在 `conformance/README.md`。

## 位置

| 部分 | 位置 |
| --- | --- |
| 档表 | `conformance/profiles.json`，Go 经 `conformance.Profiles()` 与 `RequiredVectors(档…)` 读取 |
| 向量 | `conformance/vectors/<主题>/<名字>.json`，经 `conformance.Vectors` 内嵌，向量文件表与类型化入口在 `conformance/vectors.go`；各子系统档向量的类型与入口在 `conformance/vectors_subsystems.go`，非空核对在 `vectors_subsystems_test.go` |
| 规则 | `conformance/rules`：文法与判定的 Go 实现，`rules_test.go`、`messages_test.go`、`plugin_test.go` 回放向量；子系统档一档一个文件（`messaging.go`、`scheduling.go` 与 `cron.go` 等），由同名的 `*_test.go` 回放 |
| 运行器 | `conformance/plugin`（用例 `cases.go`、名单 `options.go`、调度 `runner.go`、进程与会话）与 `conformance/cmd/xihan-conformance` |
| 运行器自测 | `conformance/plugin/fake_test.go`：测试程序扮演合规与 `legacy` 两种假插件 |

## 规则

- 期望值由 .NET 参考实现算出，不手算，不取某个 SDK 的输出；.NET 与规范不一致时停下来报告，不改向量迁就。
- 向量文件顶部 `$comment` 写清字段含义、规范出处、ADR，以及「期望值由 .NET 参考实现算出」（如是）。
- 新向量文件：写进档表至少一档；有类型化入口时在 `vectors.go` 的 `typedVectors` 表加一行并加入口，在 `vectors_test.go` 的核对表里核对非空（`TestTypedVectorsCoverEveryVectorFile` 要求表覆盖全部向量文件）；规则包回放它；`conformance/README.md` 的向量表加一行。
- 向量里出现的框架错误码必须已登记；清单向量必须与 Schema 一致（`tools/speccheck` 核对）。
- 加用例、改期望值、加向量文件或运行器用例都要记 `CHANGELOG.md`（级别见 spec-authoring）。
- 运行器新用例：`options.go` 加常量，`runner.go` 的 `caseTable` 登记（`AllCases` 由它导出，`TestCaseTableCoversEveryCase` 要求常量、表与实现一一对上），`cases.go` 实现；`fake_test.go` 里合规假插件必须通过，`legacy` 假插件必须在新用例上失败；`conformance/README.md` 的用例表加一行。各 SDK 的测试从运行器 `help` 输出的 `cases:` 行读用例全集。
- 每个实现都有「认领的档 + 缺口」的向量覆盖测试，新增向量文件会让认领该档的实现失败：向用户列出受影响的仓库。

## 完成条件

1. `conformance` 与 `tools/speccheck` 的门禁通过（testing-release）。
2. 改运行器或插件协议时，用真实插件在 Windows 与 Linux 上跑过全部用例。
3. 档表、`conformance/README.md`、`CHANGELOG.md` 与规范正文一致。
