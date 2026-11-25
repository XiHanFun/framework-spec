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

说明一句话写实现者能感知的结果，即哪份 RFC 新增或改了什么；有 ADR 时末尾注 `（ADR-NNNN）`。写死了什么、以前怎样、向量与规则补了什么写进 RFC 正文与 `CHANGELOG.md`。

## 写短

- 只写一行标题，不写正文。
- 说明一句话讲这次改动的结果，不超过 40 个字：汉字、字母、数字、标点各算一个，末尾的 `（ADR-NNNN）` 不计。
- 说明里不再出现「：」「；」，除末尾的 ADR 注外不用括号或破折号补充；不罗列文件、成员、取值与改前改后，不写向量与测试的数量、门禁结果，这些写进 RFC 正文、ADR 与 `CHANGELOG.md`。
- 40 个字写不下时，多半是暂存区混了几件事（先建议拆成几笔），或细节写多了（删到只剩结果），不要加长标题。

不要：`feat(spec): RFC-0013 新增「计划的文本形式」，把存储里持久化的计划文本写成文法：各部分之间正好一个空格，别的空白与连着的空格都不合法；种类 manual、every……`

要：`feat(spec): RFC-0013 新增计划的文本形式（ADR-0297）`

只要求消息时最终只输出一行标题，不附解释或候选列表。
