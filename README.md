<div align="center">

<h1>XiHan Spec</h1>

<h3>The language-neutral specification shared by XiHan Framework and its SDKs</h3>

<p>
  <a href="https://github.com/XiHanFun/framework-spec/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/XiHanFun/framework-spec/ci.yml?branch=main&style=flat-square&label=CI" /></a>
  <a href="./LICENSE"><img alt="MIT License" src="https://img.shields.io/github/license/XiHanFun/framework-spec?style=flat-square&color=green" /></a>
  <a href="https://github.com/XiHanFun/framework-spec/stargazers"><img alt="GitHub Stars" src="https://img.shields.io/github/stars/XiHanFun/framework-spec?style=flat-square&logo=github&label=Stars&color=1f6feb" /></a>
</p>

<p>
  <a href="https://framework.docs.xihanfun.com/appendix/spec"><b>Docs</b></a> ·
  <a href="#contents"><b>Contents</b></a> ·
  <a href="./conformance/README.md"><b>Conformance</b></a> ·
  <a href="https://github.com/XiHanFun/XiHan.Framework"><b>Reference Implementation</b></a>
</p>

<p><b>English</b> | <a href="./README_cn.md">简体中文</a></p>

</div>

XiHan Spec is the language-neutral specification of XiHan Framework: primitive semantics and wire encoding, the manifest schema, the invocation context and its propagation, pipeline stages and ordering, the out-of-process plugin protocol `xihan.plugin.v1` and the Wasm carrier, protocol mappings, application manifests and packaging, plus data-driven conformance vectors and a black-box runner. The reference implementation is [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework) for .NET; SDKs for other languages are listed under [Ecosystem](#ecosystem).

## Contents

The documents in this repository are written in Chinese.

| Directory | Content |
| --- | --- |
| `rfcs/` | Proposals: RFC first, then ADR, then implementation |
| `conventions/` | Normative: requirement words, vocabulary, package IDs, capability IDs, operation IDs, error codes, Baggage keys, batch first |
| `manifest/` | Normative: JSON Schema for `xihan.manifest.json` |
| `plugin/` | Normative: plugin protocol `plugin.proto` with the handshake, lifecycle and invocation documents; Wasm carrier `wasm.md` |
| `protocol/` | Normative: HTTP / gRPC / MCP / CloudEvents / command-line mappings |
| `packaging/` | Normative: application manifest `xihan.app.json` and the artifact mapping of `xihan pack` |
| `provider/` | Normative: general rules for remote providers and the operation set of each capability (`caching.distributed`) |
| `migration/` | Normative: general conventions of `xihan migrate` (command, markers, report format) and the rule set for each source version |
| `conformance/` | Conformance profiles (`profiles.json`), vectors (`vectors/`) and the black-box runner (Go module, see [conformance/README.md](./conformance/README.md)) |

How binding the words 必须 (MUST), 不得 (MUST NOT), 应当 (SHOULD) and 可以 (MAY) are in the spec text is defined in [conventions/normative-language.md](./conventions/normative-language.md).

## Installation

Spec files link to one another by relative path, so the whole repository is added to an implementation repository as a git submodule (XiHan.Framework uses `framework/spec`, each SDK uses `spec`):

```bash
git submodule add https://github.com/XiHanFun/framework-spec.git spec
```

## Quick Start

Install the black-box runner, then check an out-of-process plugin written in any language, or validate the vector files themselves:

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
xihan-conformance plugin --dir <plugin-dir>
xihan-conformance vectors
```

## Documentation

The documentation site is written in Chinese.

| Where | Content |
| --- | --- |
| [Spec and languages](https://framework.docs.xihanfun.com/appendix/spec) | Normative and informative content, conformance profiles, vectors and the runner, consuming the spec, versioning, the SDK of each language, the change process and the RFC list |
| [Out-of-process plugins](https://framework.docs.xihanfun.com/plugins/out-of-process) | Writing a plugin in any language and hosting it in .NET |
| SDK pages: [Go](https://framework.docs.xihanfun.com/sdks/go) · [TypeScript](https://framework.docs.xihanfun.com/sdks/typescript) · [Rust](https://framework.docs.xihanfun.com/sdks/rust) · [Java](https://framework.docs.xihanfun.com/sdks/java) · [Python](https://framework.docs.xihanfun.com/sdks/python) · [PHP](https://framework.docs.xihanfun.com/sdks/php) | Installing and using each SDK, the behavior it shares with the host, its language mapping |
| [conformance/README.md](./conformance/README.md) | What each vector file contains, the rules package, the runner's cases and options |
| [CHANGELOG.md](./CHANGELOG.md) | Changes to the spec, each marked PATCH, MINOR or MAJOR with its ADR |

## Status

The spec is tagged with its own semantic versions `vX.Y.Z` (RFC-0024); before `v1.0.0` the tags are `v1.0.0-rc.N`. Normative content is divided into the conformance profiles of [`conformance/profiles.json`](./conformance/profiles.json): XiHan.Framework claims every profile except `plugin`, the six SDKs claim `core` and `plugin` (Go also `provider:caching.distributed@1`), and none has gaps at the spec version it pins; vector files added to the spec are replayed, or listed as gaps, when an implementation moves its pin. Profiles, maturity levels and the state of each implementation are on the [docs page](https://framework.docs.xihanfun.com/appendix/spec).

## Ecosystem

| Project | Description |
| --- | --- |
| [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework) | The .NET reference implementation |
| [framework-sdk-go](https://github.com/XiHanFun/framework-sdk-go) | Go SDK |
| [framework-sdk-typescript](https://github.com/XiHanFun/framework-sdk-typescript) | TypeScript SDK |
| [framework-sdk-rust](https://github.com/XiHanFun/framework-sdk-rust) | Rust SDK |
| [framework-sdk-java](https://github.com/XiHanFun/framework-sdk-java) | Java SDK |
| [framework-sdk-python](https://github.com/XiHanFun/framework-sdk-python) | Python SDK |
| [framework-sdk-php](https://github.com/XiHanFun/framework-sdk-php) | PHP SDK |

SDK repositories are named `framework-sdk-<language>` (ADR-0010).

## Contributing

[Issues](https://github.com/XiHanFun/framework-spec/issues) and pull requests are welcome. Changes to public contracts start as an RFC in this repository, then get an ADR in XiHan.Framework, then reach the implementations (see the [change process](https://framework.docs.xihanfun.com/appendix/spec#变更流程)). Development conventions, gates and the release process are in [AGENTS.md](./AGENTS.md).

## License

[MIT](./LICENSE) © 2021-Present XiHanFun and contributors.
