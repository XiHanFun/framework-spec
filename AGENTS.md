# AGENTS.md

本文件约束在 framework-spec（XiHan Spec）仓库内工作的 AI Agent。回答、文档与提交说明使用中文；代码标识、公开 API 和标准技术术语保持英文。

## 仓库概览

XiHan Spec 是 XiHan Framework 的语言中立规范：RFC、规范性文档（术语表与各类 ID、错误码登记表、清单 Schema、插件协议、协议映射、应用清单与打包、远程 Provider、版本迁移）、一致性档表与数据驱动的向量，以及 Go 写的黑盒运行器 `xihan-conformance`。参考实现是 .NET 的 XiHan.Framework，六个 SDK（Go、TypeScript、Rust、Java、Python、PHP）与它都以 git submodule 引用本仓库，并回放同一套向量。本仓库只有规范与一致性工具，没有框架实现。

### 技术栈

| 技术 | 要求 | 用途 |
| --- | --- | --- |
| Go | `conformance/go.mod` 与 `tools/speccheck/go.mod` 的 `go` 指令（CI 按 `go-version-file: conformance/go.mod` 安装） | 一致性模块、黑盒运行器与仓库自身门禁 |
| buf、protoc-gen-go、protoc-gen-go-grpc | `ci.yml`「安装 buf 与生成器」固定的版本 | 插件协议的 lint、格式、破坏性变更检查与 Go 代码生成 |
| gRPC-Go、protobuf-go、go-winio | 以 `conformance/go.mod` 为准 | 运行器以宿主身份连接插件；Windows 命名管道 |
| JSON Schema 2020-12 | `manifest/manifest.schema.json`、`packaging/app.schema.json` 的 `$schema` | 清单与应用清单的 Schema |
| santhosh-tekuri/jsonschema | 以 `tools/speccheck/go.mod` 为准，只在 `tools/speccheck` 引用 | 核对 Schema 能编译、与清单向量一致 |
| GitHub Actions | `.github/workflows/ci.yml` | 门禁 |

## 开始前必须做

1. 按任务读取对应技能：
   - 修订 RFC、规范性文档、术语表、错误码登记表、清单 Schema、插件协议与协议映射，读 `.agents/skills/spec-authoring/SKILL.md`。
   - 改向量、规则包 `conformance/rules`、黑盒运行器或档表，读 `.agents/skills/conformance/SKILL.md`。
   - 跑门禁、核对生成物、排查 CI，读 `.agents/skills/testing-release/SKILL.md`。
   - 打规范标签、让实现钉规范标签，读 `.agents/skills/version-release/SKILL.md`。
   - 整理 `CHANGELOG.md`，读 `.agents/skills/changelog-collect/SKILL.md`。
   - 生成提交信息、准备 PR、回复 Issue、审查测试，分别读 `.agents/skills/commit-msg/SKILL.md`、`.agents/skills/create-pr/SKILL.md`、`.agents/skills/issue-reply/SKILL.md`、`.agents/skills/test-review/SKILL.md`。
2. 同时涉及多个职责时加载对应多个技能，不要一次读取无关资料。
3. 检查当前分支、`git status` 和最近提交；保留用户已有改动。
4. 读相关规范：目标 RFC 的状态行（修订记录、对应 ADR、一致性档）与相关章节；插件一侧读 `plugin/handshake.md`、`plugin/lifecycle.md`、`plugin/invocation.md`；命名与错误码读 `conventions/vocabulary.md`、`conventions/error-codes.md`；档与版本读 `conformance/profiles.json`、`rfcs/0024-versions-and-profiles.md` 与 `CHANGELOG.md` 的「未发布」一节。需要取舍过程时读 XiHan.Framework 的 `plans/adr`（本地克隆在 `../XiHan.Framework`）。
5. 改 Go 代码前读目标包与它的测试：`conformance/rules`、`conformance/plugin`、`conformance/vectors.go`、`conformance/profiles.go`、`tools/speccheck`，以及 `conformance/README.md`。
6. 先找已有的：规范里是否已经写到（`grep -rn` 正文与向量）、规则包里是否已有判定函数、向量里是否已有同类用例，不重复建设。

## 目录结构

```text
/
├─ AGENTS.md
├─ .agents/skills/                     # 按规范修订、一致性、门禁、发版与协作分类的仓库技能
├─ .claude/skills -> ../.agents/skills # Claude Code 技能兼容入口
├─ CLAUDE.md -> AGENTS.md              # Claude Code 仓库指令入口
├─ CHANGELOG.md                        # 规范的变更记录（RFC-0024）
├─ buf.yaml                            # 插件协议的 lint 与破坏性变更检查
├─ buf.gen.yaml                        # 为运行器生成协议的 Go 代码
├─ rfcs/                               # 提案：先 RFC，再 ADR，再实现
├─ conventions/                        # 规范性：术语表、包 ID、能力 ID、操作 ID、错误码、Baggage 键、批量优先
├─ manifest/                           # 规范性：xihan.manifest.json 的 JSON Schema
├─ plugin/                             # 规范性：plugin.proto 与握手、生命周期、调用；Wasm 载体 wasm.md
├─ protocol/                           # 规范性：HTTP / gRPC / MCP / CloudEvents / 命令行映射
├─ packaging/                          # 规范性：应用清单 xihan.app.json 与 xihan pack 的产物映射
├─ provider/                           # 规范性：远程 Provider 的通用规则与各能力的操作集
├─ migration/                          # 规范性：xihan migrate 的通用约定与各来源版本的规则集
├─ conformance/                        # Go 模块 github.com/xihanfun/framework-spec/conformance
│  ├─ profiles.json                    # 一致性档表，唯一事实源
│  ├─ vectors/                         # 一致性向量，按主题分目录，经 conformance.Vectors 内嵌
│  ├─ rules/                           # 规范文法与判定的 Go 实现，每条规则由向量验证
│  ├─ plugin/                          # 黑盒运行器的用例、插件进程与会话
│  ├─ cmd/xihan-conformance/           # 运行器命令行
│  └─ internal/pluginv1/               # buf generate 的生成物，提交进仓库
├─ tools/speccheck/                    # 仓库自身的门禁：独立 Go 模块，只有测试
└─ .github/workflows/ci.yml
```

消费方把整个仓库作为 submodule 放在 `spec/`（XiHan.Framework 是 `framework/spec`），规范文件之间按相对路径互相链接，目录不能随意改名或移动。

## 常用命令

在仓库根执行（Go 命令在所示目录）：

| 任务 | 命令 |
| --- | --- |
| 安装 buf 与生成器 | `go install github.com/bufbuild/buf/cmd/buf@<版本>` 等三条，版本照 `ci.yml` |
| 协议 lint | `buf lint` |
| 协议格式 | `buf format --diff --exit-code`（就地修正用 `buf format -w`） |
| 破坏性变更 | `buf breaking --against ".git#branch=origin/dev"`（CI：PR 相对目标分支，推送相对推送前的提交） |
| 生成协议代码 | `buf generate`，之后 `git status --porcelain -- conformance/internal` 必须为空 |
| 格式与静态检查 | `conformance/`：`test -z "$(gofmt -l .)" && go vet ./... && GOOS=windows go vet ./...`（后一条盖住只在 Windows 上编译的文件） |
| 向量、规则与运行器测试 | `conformance/`：`go test -race ./...` |
| 登记表、Schema 与档表 | `tools/speccheck/`：`test -z "$(gofmt -l .)" && go vet ./... && go test ./...` |
| 构建运行器 | `conformance/`：`go build -o bin/xihan-conformance ./cmd/xihan-conformance` |
| 校验向量文件 | `conformance/bin/xihan-conformance vectors` |
| 对插件跑黑盒运行器 | `conformance/bin/xihan-conformance plugin --dir <插件目录> --exclude echoes.crash --sample 'echoes.create={"text":"x"}'` |
| 在消费仓库初始化规范 | XiHan.Framework：`git submodule update --init framework/spec`；SDK：`git submodule update --init spec` |

本机环境（Go 不在 PATH、国内代理、没有 C 编译器时的 `-race`）见 `.agents/skills/testing-release/references/gates.md`。

## 规范的组织与修订规则

- 本仓库是事实源。规范性要求必须写在正文或向量里，不能只写在 ADR 里；规范里的「ADR-NNNN」指 XiHan.Framework 仓库 `plans/adr/` 下以该编号开头的文件（RFC-0024「规范与决策记录」）。
- .NET（XiHan.Framework）是参考实现。规范没写到、各实现之间有分歧的行为，以 .NET 实测为准写进规范；向量的期望值由 .NET 程序算出，不手算，也不取某个 SDK 的输出。例外是插件一侧（SDK 认领的 `plugin` 档）没有 .NET 实现的向量：期望值由 Go 一致性模块（`conformance/rules`，不是 Go SDK）的规则函数算出，再用 .NET 能用的部分与各语言的标准库交叉核对，照 `plugin/sdk-envelope-times.json`（ADR-0310）的做法。拿不准 .NET 的行为或要改 .NET 时，停下来报告差异并起草规范文字，不自行发明。
- 公开契约的变化顺序：本仓库的 RFC → XiHan.Framework 的 ADR（`plans/adr`）→ 各实现与一致性向量。新 RFC 是 `rfcs/NNNN-<slug>.md`，状态 `草案`；评审通过后写 ADR、状态改为 `已采纳`、正文并入对应规范性目录；没有 RFC 的公开契约变更不合并。新增术语表条目同样走 RFC。
- 修订已采纳的内容：正文就地写注记「（YYYY-MM-DD 写明 / 修订，ADR-NNNN；以前…）」，「写明」是把以前不作规定的写死，「修订」是改已经写明的；RFC 状态行追加「YYYY-MM-DD 修订：…」（同一天多次用「同日再修订」「同日第三次修订」），并把 ADR 加进「对应 ADR」。
- 每个改动规范性内容的提交在 `CHANGELOG.md`「未发布」一节记一条，标明 PATCH、MINOR 或 MAJOR 与 ADR；三级的界定照 RFC-0024「版本」第 3 条，`v1.0.0` 之前的 MAJOR 注明「（`v1.0.0` 之前允许）」。
- 新的规范文件与向量文件写进 `conformance/profiles.json`（至少一档），RFC 状态行的「一致性档」与档表一致；把向量移出档是 MAJOR。
- 框架错误码在 `conventions/error-codes.md` 登记，撤销的进「已撤销」且序号不复用；错误码、能力 ID、Baggage 键、操作 ID 一经发布不改。
- 消费方：XiHan.Framework 的 `framework/spec` 与各 SDK 的 `spec/` 是 submodule。先在本仓库提交并推送，再到各消费仓库升指针并提交；Go SDK 还要把 `go.mod` 里 conformance 模块升到同一提交的伪版本，规范提交还没推送时可从本地克隆解析。步骤见 `.agents/skills/spec-authoring/references/revision-rules.md`。不在消费仓库的 submodule 里直接改规范文件。

## 代码规范

- 文档、RFC、注释与提交说明用中文；`README.md` 是英文、`README_cn.md` 是中文，两份同构同步。
- 编辑器约定照 `.editorconfig` 与 `.gitattributes`：UTF-8、LF、末尾换行、两空格缩进，Go 文件用 tab；Markdown 不裁行尾空白。
- Go 代码过 `gofmt` 与 `go vet`（CI 必过）。每个类型、函数、变量与常量，导出与否，都有以名字开头的中文 `//` 注释（`// repoRoot 返回…`；`main` 与 `Test*` 函数除外），包有 `// Package <名> …` 注释；函数内部写中文 `//` 说明为什么，不复述代码。
- 运行时文本用英文：`fmt.Errorf` 的消息小写开头、以包名作前缀并用 `%w` 包装（`conformance: parse vector %q: %w`），测试断言与命令行输出同样用英文。
- 规则包 `conformance/rules` 实现规范的文法与判定，每条规则都由向量验证；正则按整串匹配（`^…$`）；规范所说的空白（RFC-0001「空白字符」）用 `rules.IsWhitespace` 判断，RE2 的 `\s` 只认 ASCII 空白，不能用在这类判定里。
- 运行器每一步都有截止时间（`--timeout`，默认 15 秒），`context.Context` 一路传下去；每个用例单独启动插件进程，结束时关 stdin、超时才强制终止；传给插件的环境变量只取白名单。
- `conformance` 模块会进入 Go SDK 用户的模块图，依赖保持最少：Schema 校验等只为门禁用的库放在独立模块 `tools/speccheck`。
- `conformance/internal/pluginv1` 是生成物，不手改；改 `plugin/plugin.proto` 后 `buf generate` 并与源同一提交。`buf.yaml` 的 lint 例外每条都写理由。
- 向量是 JSON，顶部 `$comment` 写清字段含义、规范出处与 ADR，期望值由 .NET 算出时注明；属于上面例外的，注明算出期望值的规则函数，以及哪几项由 .NET、哪几项由哪种语言的标准库核对。

## 测试

- 改规则补向量与 `rules/*_test.go` 的回放；改向量文件或档表跑 `conformance` 的 `go test`（每个向量文件至少属于一个档）与 `tools/speccheck`（每个规范文件至少属于一个档、RFC 状态行与档表一致、错误码已登记、清单向量与 Schema 一致）。
- 运行器的新用例同时补 `plugin/fake_test.go`：合规的假插件必须通过，按以前做法应答的假插件必须在新用例上失败。
- 改插件协议或运行器时，用真实插件在 Windows 与 Linux 上各跑一遍全部用例（Windows 上回调端点是命名管道 + 回环 TCP，其他系统是 Unix 套接字 + 回环 TCP），例如 Go SDK 的 `examples/echo` 与 XiHan.Framework 的 `framework/samples/EchoPlugin`。
- 新增向量文件会让认领该档的实现的覆盖测试失败，向用户报告受影响的仓库，由各实现回放或登记缺口。
- 修复缺陷先写能复现问题的测试或用例，再改实现；不删测试、不删向量用例、不放宽门禁来通过 CI。

## 文档

- README 只放概览、目录、安装、快速开始、文档链接、状态、生态与参与，两份同步；规范内容本身在规范文件里，向量与运行器的细则在 `conformance/README.md`。
- 面向读者的详细说明在 XiHan.Framework 仓库的 `docs/appendix/spec.md`（本地克隆在 `../XiHan.Framework`）：规范性与信息性、一致性档、向量与运行器、版本、各语言 SDK、变更流程与 RFC 表。它是另一个仓库，单独提交，构建用 `pnpm --dir docs build`。
- 新增或修订 RFC、档、向量文件、运行器用例或成熟度时，同步那一页与 `conformance/README.md`。
- 文档正文不写包版本、规范提交哈希或测试数量，用「见 `CHANGELOG.md`」「见 `profiles.json`」之类指向真源。

## 发布

- 规范没有版本文件：版本就是 `main` 上的标签 `vX.Y.Z`，Go 一致性模块在同一提交另打 `conformance/vX.Y.Z`（RFC-0024）；`CHANGELOG.md` 的「未发布」一节在打标签时改成版本号。`v1.0.0` 之前打 `v1.0.0-rc.N`，`v1.0.0` 与 XiHan.Framework 的 5.0.0 正式版一起打。
- 没有发布工作流，也不发到包注册表：Go 用户经模块代理按标签取 `github.com/xihanfun/framework-spec/conformance`，运行器用 `go install …/cmd/xihan-conformance@<版本>` 安装。
- 打标签与推送是外部状态变更，要用户自己的凭据，只在用户明确要求时执行；已推送的标签不移动。步骤见 `.agents/skills/version-release/SKILL.md`。
- 实现发版时 submodule 必须正好在规范标签上，正式版只钉正式版标签；XiHan.Framework 的 `release.yml` 核对这一条。

## 分支

- 日常提交在 `dev`；`main` 只在用户明确要求（如「推送并快进 main」）时由 `dev` 快进（`git fetch . dev:main`，不是快进就停下查），标签打在 `main` 上。
- `ci.yml` 在 `main` 与 `dev` 的 push 和 pull request 上都跑，也可以手动触发（`workflow_dispatch`）。
- 实现的日常分支可以指向规范已推送的任意提交（`dev` 或 `main`）。

## Git 与提交

- 使用 Conventional Commits：`<type>(<scope>): <中文说明>`。scope 默认 `spec`（规范正文、向量、规则包、运行器与 speccheck 的改动都用它）；只涉及一个子系统 RFC 时也可用术语表领域词小写；README、AGENTS.md 与技能用 `docs`，工程配置用 `build`、`ci`。有 ADR 的提交在说明末尾注明（`（ADR-NNNN）`）。
- 提交信息只写一行标题，不写正文。说明一句话讲这次改动的结果，不超过 40 个字（汉字、字母、数字、标点各算一个，末尾的 `（ADR-NNNN）` 不计）；不再用「：」「；」往下展开，不罗列文件、成员、取值与改前改后，不写测试数量与门禁结果，这些细节写进 RFC 正文、ADR 与 `CHANGELOG.md`。细则见 `commit-msg` 技能。
- 保留用户已有和无关改动；不要 reset、checkout 或清理它们。
- 一项规范改动一个提交：正文、向量、规则、运行器、档表与 `CHANGELOG.md` 属于同一闭环时放在同一提交。
- 提交前检查 `git diff --check` 与相关门禁（至少 `buf lint`、`gofmt`、`go vet`、两个模块的 `go test`）。
- 不推送、不打标签、不创建远程 PR，除非用户明确要求。

## 禁止事项

- 不凭空发明规范行为，不手算或从某个 SDK 抄向量期望值。
- 不把规范性要求只写在 ADR、提交说明或文档站里。
- 不改已发布的错误码、能力 ID、Baggage 键、操作 ID，不复用撤销的错误码序号，不移动已推送的标签。
- 不手改 `conformance/internal/pluginv1` 等生成物，不提交 `conformance/bin/`。
- 不删向量用例或运行器用例、不放宽门禁来让某个实现通过。
- 不往 `conformance` 模块加只为门禁用的依赖。
- 不在消费仓库的 submodule 检出里改规范文件。
- 不推送、不打标签、不快进 `main`，除非用户明确要求。
