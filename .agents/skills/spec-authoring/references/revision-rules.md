# 修订细则

## RFC 文件

- 文件名 `rfcs/NNNN-<slug>.md`，首行 `# RFC-NNNN <标题>`，第三行是引用块状态行，字段以 ` | ` 分隔：

  ```text
  > 状态：已采纳（<修订记录>） | 日期：YYYY-MM-DD | 对应 ADR：NNNN、NNNN（说明） | 参考实现：… | 一致性：… | 规范性文本：… | 一致性档：`core`、`host`
  ```

- 「状态」是 `草案` 或 `已采纳`；「日期」是 RFC 首次提出的日期，不随修订改；「参考实现」「一致性」「规范性文本」按需写；「一致性档」必填，`tools/speccheck` 核对它与档表一致。
- 常见章节：摘要、动机、正文各节、错误码、一致性、与 .NET 的映射、非目标与未决。规范里的 ADR 编号指 XiHan.Framework 的 `plans/adr/`。

## 状态行的修订记录

- 括号里按时间先后追加，用 `；` 分隔：`2026-11-16 修订：<要点>`；同一天第二次起写「同日再修订：…」「同日第三次修订：…」。
- 新 ADR 追加进「对应 ADR」，需要时带括号说明，如 `0244（.NET 的线上编码）`。

## 正文注记

- 写在被修订的那一句或那一条旁边：「（YYYY-MM-DD 写明，ADR-NNNN；以前…）」或「（YYYY-MM-DD 修订，ADR-NNNN）」。「以前…」写清旧行为或各实现的旧做法，没有可说的就省略。
- 例：`plugin/invocation.md` 里「插件一侧处理 `Invoke` 依次核对（2026-11-18 写明，ADR-0237；以前有的 SDK 先报未就绪）：…」。
- 规范性的要求写进正文本身，注记只记来历；不要让读者必须读 ADR 才知道该怎么实现。

## CHANGELOG.md

- 记在「未发布」一节，最新的在最上面，一条一行：`- <级别>：<主题>：<要点>（ADR-NNNN）。`
- 级别（RFC-0024「版本」第 3、4 条，「一致性档」第 4 条）：
  - PATCH：不改任何规范性内容。措辞、示例、理由、非规范性文档，以及运行器、`tools/speccheck` 的缺陷修复，前提是不改任何期望值。
  - MINOR：只增，或把以前不作规定的行为写死。新 RFC 与新章节、可选字段与可选操作、新档、新向量文件或新用例、运行器新用例；实现之间有分歧而规范没写的地方选定一种写法。写死不得让按较早 MINOR 实现的对端互通失败；读取一侧对较早 MINOR 可能写出的形式要么照收，要么算 MAJOR。新增档、把文件或向量加进已有的档也是 MINOR。
  - MAJOR：改已经写明的行为、删除内容、线上不兼容、把向量移出档。新的协议版本与旧版本并存算 MINOR，去掉对旧版本的支持才算 MAJOR。`v1.0.0` 之前写「MAJOR（`v1.0.0` 之前允许）」。
- 只改仓库工程（README、AGENTS.md、技能、CI）不是规范的变化。

## 档表 conformance/profiles.json

- 每档有 `id`、`title`、`stability`（`stable` 或 `experimental`）、`documents`（相对仓库根）、`vectors`（相对 `conformance/vectors`）、`runner`（必须通过的运行器子命令）。
- 每个向量文件至少属于一档（`conformance` 模块的测试核对）；每个规范文件至少属于一档、子系统档名取术语表的能力 ID 前缀、RFC 状态行的档与档表一致（`tools/speccheck` 核对）。

## 错误码与术语

- 文法 `<域>_<KIND>_<NNN>`；框架错误码登记在 `conventions/error-codes.md`「已登记的框架错误码」，撤销的移到「已撤销的错误码」，序号不复用。正文、向量与协议里出现的框架错误码都必须登记或撤销（规则 5，`tools/speccheck` 核对）。
- 新的领域词、能力 ID 前缀、Baggage 键走 RFC，登记到 `conventions/` 对应文件。

## 升消费方的规范指针

只在用户要求推送后做，推送顺序是本仓库在先，消费仓库在后；XiHan.Framework 的 Go 示例（`framework/samples/EchoGoPlugin`、`CacheGoPlugin`）依赖 Go SDK，Go SDK 要先于它推送。

1. XiHan.Framework：`git -C framework/spec fetch origin`，`git -C framework/spec checkout --detach <提交>`，在 XiHan.Framework 提交指针（子模块检出是游离 HEAD，不用它的本地分支）。它的架构测试 `SpecVectorCoverageTests` 读档表，新向量要回放或列为缺口。
2. 各 SDK：同样升 `spec/` 指针，跑各自的向量覆盖测试。
3. Go SDK 另把 conformance 模块升到同一提交：`go get github.com/xihanfun/framework-spec/conformance@<提交>`，`go.mod` 里是伪版本 `v0.0.0-<时间>-<12 位哈希>`。Go SDK 的测试按 `go.mod` 里的版本 `go install` 运行器，运行器的新用例要靠这一步才跑得到。
4. 规范提交还没推送时，只在当前命令的环境里让 Go 从本地克隆解析，不写进 git 配置：

   ```bash
   GOPRIVATE=github.com/xihanfun/framework-spec \
   GIT_CONFIG_COUNT=2 \
   GIT_CONFIG_KEY_0="url.file:///<本地克隆的绝对路径>.insteadOf" \
   GIT_CONFIG_VALUE_0="https://github.com/xihanfun/framework-spec" \
   GIT_CONFIG_KEY_1="protocol.file.allow" GIT_CONFIG_VALUE_1="always" \
   go get github.com/xihanfun/framework-spec/conformance@<提交>
   ```

   这样算出的 `go.sum` 哈希与模块代理一致，前提是这个规范提交推送时不被改写。
5. 走国内代理（`GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=sum.golang.google.cn`）时，校验和库对仓库根路径 `github.com/xihanfun/framework-spec` 可能返回 404，再加 `GOPRIVATE=github.com/xihanfun/framework-spec` 直接从 GitHub 取。
