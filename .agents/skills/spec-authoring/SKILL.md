---
name: xihan-spec-authoring
description: 新增或修订 XiHan Spec 的 RFC、规范性文档、术语表、错误码登记表、清单 Schema、插件协议与协议映射时使用，适用于规范文字、RFC 状态行、修订注记、CHANGELOG 条目、档表登记与消费方指针的升级；向量、规则包与运行器用 conformance，门禁用 testing-release，打标签用 version-release。
---

# XiHan Spec 规范修订

状态行、修订注记、CHANGELOG 级别、错误码登记与升指针的细则读 `references/revision-rules.md`。

## 先查事实

```bash
git status --short --branch
git log -5 --oneline
grep -rn '<关键词>' rfcs conventions manifest plugin protocol packaging provider migration conformance/vectors
```

按顺序读：目标 RFC 的状态行与相关章节 → 对应的规范性文本（`plugin/*.md`、`protocol/*.md`、`provider/*.md`、`packaging/`、`migration/`）→ `conventions/`（术语表、错误码登记表）→ `conformance/profiles.json` 与 `CHANGELOG.md`「未发布」→ XiHan.Framework 的相关 ADR（`../XiHan.Framework/plans/adr`）与参考实现。只读与任务相关的部分。

## 原则

- 规范先于实现：本仓库的 RFC → XiHan.Framework 的 ADR → 各实现与向量。规范性要求写在正文或向量里，不能只写在 ADR 里。
- 规范没写到、实现之间有分歧时，以 .NET 参考实现实测为准写死；拿不准就停下来报告差异并起草规范文字，不自行发明。
- 「写明」是把以前不作规定的写死（MINOR），「修订」是改已经写明的（改规范性内容是 MAJOR，`v1.0.0` 之前允许；只改措辞、示例或 .NET 映射等非规范性内容是 PATCH）；写死不得让较早 MINOR 的对端互通失败（RFC-0024「版本」第 3 条）。
- 每处修订在正文写注记「（YYYY-MM-DD 写明 / 修订，ADR-NNNN；以前…）」，RFC 状态行追加当天的修订摘要并把 ADR 加进「对应 ADR」。
- 新错误码登记进 `conventions/error-codes.md`，撤销的进「已撤销」且序号不复用；新术语走 RFC；错误码、能力 ID、Baggage 键、操作 ID 一经发布不改。
- 新的规范文件写进档表，RFC 状态行的「一致性档」与档表一致。
- 改 `plugin/plugin.proto` 要过 `buf lint`、`buf format`、`buf breaking`，并 `buf generate` 后把 `conformance/internal/pluginv1` 一起提交；线上不兼容只能新增协议版本并存。

## 完成条件

1. 正文、修订注记、RFC 状态行、`CHANGELOG.md` 与档表一致。
2. 新行为有向量或运行器用例（用 conformance 技能），期望值由 .NET 算出；插件一侧（SDK 认领的 `plugin` 档）没有 .NET 实现的向量例外，期望值由 Go 一致性模块（`conformance/rules`，不是 Go SDK）的规则函数算出，再用 .NET 能用的部分与各语言的标准库交叉核对。
3. testing-release 里的门禁全部通过。
4. 需要时同步 `conformance/README.md` 与 XiHan.Framework 的 `docs/appendix/spec.md`（另一个仓库，单独提交）。
5. 一项规范改动一个提交，说明末尾注 ADR；推送与升消费方指针只在用户要求时做。
