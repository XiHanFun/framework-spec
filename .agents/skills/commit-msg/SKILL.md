---
name: xihan-spec-commit-msg
description: 根据 XiHan Spec 当前暂存区生成一行提交信息时使用，适用于「提交信息」「commit msg」「msg」或提交前概括 staged changes；只生成消息，不负责暂存、解决冲突或执行提交。
---

# XiHan Spec 提交信息

1. 检查 `git status --short --branch`；存在冲突、merge 或 rebase 时先报告，不生成完成态消息。
2. 只读取 `git diff --cached --stat`、完整 staged diff 和最近 20 条提交。暂存区为空时不从未暂存内容猜测。
3. 将全部暂存变化归纳成一个意图，输出一行 Conventional Commit。

格式：`<type>(<scope>): <中文说明>`。type 使用 `feat`、`fix`、`refactor`、`perf`、`docs`、`style`、`test`、`build`、`ci`、`chore`、`revert`；新增规范内容多用 `feat`，门禁或工具的缺陷用 `fix`，澄清与写明多用 `docs`。

scope：

- `spec`：规范正文、Schema、协议、向量、规则包、运行器、`tools/speccheck` 与 `CHANGELOG.md` 的改动，近来的提交都用它。
- 术语表领域词小写（`persistence`、`security`、`caching` …）：只涉及一个子系统 RFC 时可用。
- `docs`：README、AGENTS.md 与仓库技能；`build`、`ci`：工程配置与工作流。

说明写实现者能感知的结果：改了哪份规范的哪一节、写死了什么、以前怎样，向量与规则补了什么；有 ADR 时末尾注 `（ADR-NNNN）`。只要求消息时最终只输出一行标题，不附解释或候选列表。
