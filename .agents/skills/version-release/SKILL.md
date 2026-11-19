---
name: xihan-spec-version-release
description: 为 XiHan Spec 打规范标签 vX.Y.Z 与 conformance/vX.Y.Z、把 CHANGELOG.md 的「未发布」定为版本，或让实现钉规范标签时使用；只有用户明确要求时才快进 main、打标签与推送，不负责实现仓库自己的发版。
---

# XiHan Spec 版本发布

规则出处是 RFC-0024「版本」与「实现钉规范」；本仓库没有发布工作流，版本只体现在标签与 `CHANGELOG.md` 上。

## 前置条件

- 版本号与是否预发布先问用户，不预选。`v1.0.0` 之前打 `v1.0.0-rc.N`，`v1.0.0` 与 XiHan.Framework 的 5.0.0 正式版一起打；此后按 PATCH / MINOR / MAJOR 的界定选号。
- 工作区干净，无 merge / rebase、冲突或 detached HEAD；`dev` 的 CI 全绿。
- `CHANGELOG.md`「未发布」一节覆盖上一个标签以来全部规范性改动（用 changelog-collect 核对），每条有级别与 ADR，所选版本号与其中最高的级别相符。

## 步骤

每一步都在用户要求之后才做：

1. 在 `dev` 上把 `CHANGELOG.md` 的「未发布」一节改成版本号（RFC-0024「版本」第 6 条），单独一个提交。
2. 快进 `main`：`git fetch . dev:main`，不是快进就停下查；推送 `dev` 与 `main`，等 `ci.yml` 在 `main` 上通过。
3. 在 `main` 的同一提交上打两个标签：`git tag -a vX.Y.Z -m "vX.Y.Z" main` 与 `git tag -a conformance/vX.Y.Z -m "conformance/vX.Y.Z" main`；Go 要求子目录模块的标签带目录前缀，所以第二个不能省。
4. 推送标签：`git push origin vX.Y.Z conformance/vX.Y.Z`，再用 `go list -m github.com/xihanfun/framework-spec/conformance@vX.Y.Z` 确认模块代理取得到。

推送用的是用户自己的 GitHub 凭据。已推送的标签不移动、不删了重打：Go 的校验和数据库记下了 conformance 模块这个版本的哈希，改了会让消费方校验失败；发错只能用下一个号。

## 实现钉规范

- 日常分支可以指向规范已推送的任意提交；实现发预发布或正式版时，submodule 必须正好在规范标签上，正式版只钉不带预发布后缀的标签。
- Go SDK 的 `go.mod` 在发版时必须依赖 conformance 模块的标签版本（`go get github.com/xihanfun/framework-spec/conformance@vX.Y.Z`），不能是伪版本。
- XiHan.Framework 的 `release.yml` 核对 `framework/spec` 在规范标签上；规范没有对应标签时它会拒发，所以框架发版前先在这里打标签。
- 各实现在 README 的「状态」一节写明钉的规范版本、认领的档与缺口。升指针的步骤见 spec-authoring 的 `references/revision-rules.md`。
