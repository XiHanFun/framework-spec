# 门禁

## CI（`.github/workflows/ci.yml`）

在 `main` 与 `dev` 的 push 和 pull request 上跑，可手动触发；同一引用的新运行取消旧运行。单个作业「协议与运行器」在 `ubuntu-latest` 上依次执行：

| 步骤 | 命令 | 失败时 |
| --- | --- | --- |
| 安装 Go | `actions/setup-go`，`go-version-file: conformance/go.mod` | 升 Go 版本改 `go.mod` 的 `go` 指令 |
| 安装 buf 与生成器 | `go install` 固定版本的 buf、protoc-gen-go、protoc-gen-go-grpc | 升版本改 `ci.yml`，本机同步安装 |
| 协议 lint | `buf lint` | 改 `plugin/plugin.proto`；确需例外时写进 `buf.yaml` 并注明理由 |
| 协议格式 | `buf format --diff --exit-code` | `buf format -w` |
| 破坏性变更（PR） | `buf breaking --against ".git#branch=origin/<目标分支>"` | 线上不兼容只能新增协议版本并存（RFC-0024） |
| 破坏性变更（推送） | `buf breaking --against ".git#ref=<推送前的提交>"`，新建分支时跳过 | 同上 |
| 生成物与源一致 | `buf generate` 后 `git status --porcelain -- conformance/internal` 为空 | 重新生成并提交 |
| 格式与静态检查 | `conformance/`：`test -z "$(gofmt -l .)"`、`go vet ./...`、`GOOS=windows go vet ./...` | `gofmt -w .` |
| 测试 | `conformance/`：`go test -race ./...`（向量、规则、档表、运行器与假插件） | 修规则、向量或运行器，不删用例 |
| 登记表与清单 Schema | `tools/speccheck/`：`gofmt`、`go vet`、`go test ./...` | 见下文 |
| 构建运行器 | `conformance/`：`go build -o bin/xihan-conformance ./cmd/xihan-conformance` | 修编译错误 |

## tools/speccheck 核对什么

- 错误码登记表自洽；正文、向量与协议里出现的框架错误码都在 `conventions/error-codes.md` 登记或撤销。
- `manifest/manifest.schema.json` 与 `packaging/app.schema.json` 能编译；清单 Schema 对每个清单向量都与 RFC-0002 的读取步骤一致；应用清单 Schema 拒绝 `packaging/app-manifest.md` 字段表禁止的写法（`appschema_test.go` 里逐行备了例子，Schema 表达不了的跨字段约束另有一张表）。
- 能力登记表自洽：登记的 ID 合文法、领域在术语表里，「保留，尚无实现」的标注与「保留、尚无实现的能力」一节一一对应。
- 档表覆盖每个规范文件；子系统档名来自术语表；每个 RFC 状态行写的档与档表一致。

它是独立模块（只有测试），在 `tools/speccheck` 目录里跑；测试把上两级目录当作仓库根，并先确认那里有 `README.md` 与 `conventions/error-codes.md`。

## 本机环境

- Go 与 `go install` 装的工具不在 PATH 时先加上：Windows 上 winget 装的 Go 在 `C:\Program Files\Go\bin`，buf 与两个生成器在 `~/go/bin`（Git Bash：`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`）。
- 国内网络拉依赖用 `GOPROXY=https://goproxy.cn,direct` 与 `GOSUMDB=sum.golang.google.cn`，只设在当前命令的环境里。
- `-race` 需要 cgo 与 C 编译器；Windows 上没有时去掉 `-race`，CI 的 Linux 作业仍会跑竞态检查。
- 运行器的 Windows 路径（命名管道、`.exe` 补全）只在 Windows 上测得到，CI 只有 Linux：改这些代码时在本机 Windows 上跑 `go test ./plugin` 与真实插件。
- 构建出的 `conformance/bin/` 已被 `.gitignore` 排除，不要提交。
