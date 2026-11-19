---
name: xihan-spec-create-pr
description: 为 XiHan Spec 准备、审查或创建 Pull Request 时使用，适用于生成 PR 标题与正文、比较分支或执行 gh pr create；普通规范修订不使用，只要求草稿时不写远程。
---

# XiHan Spec 创建 PR

## 前置检查

- 检查分支、上游、工作区、merge / rebase 和冲突；非正常分支状态不创建 PR。
- base 默认 `dev`；`main` 只由 `dev` 快进，不作为 PR 的目标，除非用户指定。
- 用 `git log <base>..HEAD`、`git diff --stat <base>...HEAD` 和完整 diff 审查整条分支。
- 核对规范正文、修订注记、RFC 状态行、`CHANGELOG.md`、档表、向量与生成物是否一致，门禁是否跑过。

## PR 内容

仓库没有 PR 模板，正文按以下几段写：

- **关联 Issue**：`Closes #<编号>` 或「无」。
- **变更说明**：改了哪些规范、写死或修订了什么、以前怎样，对应的 RFC 与 ADR。
- **影响范围**：涉及的规范文件、向量文件、一致性档与 CHANGELOG 级别；需要升规范指针或补回放的实现仓库。
- **自测**：只列实际执行过的门禁（buf、gofmt、go vet、两个模块的 go test、真实插件跑运行器）及结果。
- **破坏性变更**：MAJOR 级改动、线上不兼容或移出档的向量，以及实现方的迁移做法；没有就写「无」。

标题使用 Conventional Commit 形态（见 commit-msg）。只要草稿时不写远程；明确要求创建 PR 时才推送当前分支并执行 `gh pr create --base dev`，禁止 force push、合并或改写其他远端分支。创建后返回 PR 链接。
