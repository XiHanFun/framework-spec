---
name: xihan-spec-testing-release
description: 在 XiHan Spec 仓库跑门禁、核对协议生成物、构建运行器、提交前验证或排查 CI 失败时使用，适用于 buf、gofmt、go vet、go test 与 tools/speccheck；向量与运行器怎么写用 conformance，打规范标签用 version-release。
---

# XiHan Spec 门禁与验证

先读 `references/gates.md`，再以当前 `.github/workflows/ci.yml` 为事实源；本文与 CI 不一致时以 CI 为准并更新本文。

## 标准验证

在仓库根执行：

```bash
buf lint
buf format --diff --exit-code
buf generate && test -z "$(git status --porcelain -- conformance/internal)"
(cd conformance && test -z "$(gofmt -l .)" && go vet ./... && GOOS=windows go vet ./... && go test -race ./...)
(cd tools/speccheck && test -z "$(gofmt -l .)" && go vet ./... && go test ./...)
(cd conformance && go build -o bin/xihan-conformance ./cmd/xihan-conformance && bin/xihan-conformance vectors)
```

改了协议还要跑破坏性变更检查：`buf breaking --against ".git#branch=origin/dev"`。

- 先跑受影响的包（`go test ./rules`、`go test ./plugin -run <用例>`），再跑完整门禁。
- 本机没有 C 编译器时 `-race` 跑不起来（需要 cgo），去掉 `-race` 跑，并说明竞态检查交给 CI。
- 生成物 `conformance/internal/pluginv1` 与 `plugin/plugin.proto` 不一致时重新 `buf generate` 并提交，不手改。
- 不删测试、不删向量用例、不改 CI 门槛制造通过；失败时保留输出作为证据。
- 改运行器或插件协议时，另用真实插件在 Windows 与 Linux 上跑全部用例（见 conformance 技能）。

推送、打标签与触发工作流会改变外部状态，只在用户明确要求时执行；推送后用 `gh run list --branch <分支>` 与 `gh run view <id>` 核对 CI。
