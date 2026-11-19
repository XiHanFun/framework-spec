<div align="center">

<h1>XiHan Spec</h1>

<h3>The language-neutral specification shared by XiHan Framework and its SDKs</h3>

<p>
  <a href="https://github.com/XiHanFun/framework-spec/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/XiHanFun/framework-spec/ci.yml?branch=main&style=flat-square&label=CI" /></a>
  <a href="./LICENSE"><img alt="MIT License" src="https://img.shields.io/github/license/XiHanFun/framework-spec?style=flat-square&color=green" /></a>
  <a href="https://github.com/XiHanFun/framework-spec/stargazers"><img alt="GitHub Stars" src="https://img.shields.io/github/stars/XiHanFun/framework-spec?style=flat-square&logo=github&label=Stars&color=1f6feb" /></a>
</p>

<p>
  <a href="https://framework.docs.xihanfun.com"><b>Docs</b></a> ·
  <a href="#contents"><b>Contents</b></a> ·
  <a href="#conformance"><b>Conformance</b></a> ·
  <a href="https://github.com/XiHanFun/XiHan.Framework"><b>Reference Implementation</b></a>
</p>

<p><b>English</b> | <a href="./README_cn.md">简体中文</a></p>

</div>

XiHan Spec is the language-neutral specification of XiHan Framework: primitive semantics and wire encoding, the manifest schema, the invocation context and its propagation, pipeline stages and ordering, the out-of-process plugin protocol `xihan.plugin.v1` and the Wasm carrier, protocol mappings, application manifests and packaging, plus data-driven conformance vectors and a black-box runner. The reference implementation is [XiHan.Framework](https://github.com/XiHanFun/XiHan.Framework) for .NET; SDKs for other languages are listed under [Ecosystem](#ecosystem).

## Contents

```text
spec/
├─ rfcs/           # Proposals: RFC first, then ADR, then implementation
├─ conventions/    # Normative: vocabulary, package IDs, capability IDs, operation IDs, error codes, Baggage keys, batch first
├─ manifest/       # Normative: JSON Schema for xihan.manifest.json
├─ plugin/         # Normative: plugin protocol plugin.proto with the handshake, lifecycle and invocation documents; Wasm carrier wasm.md
├─ protocol/       # Normative: HTTP / gRPC / MCP / CloudEvents / command-line mappings
├─ packaging/      # Normative: application manifest xihan.app.json and the artifact mapping of xihan pack
├─ provider/       # Normative: general rules for remote providers and the operation set of each capability (caching.distributed)
├─ migration/      # Normative: general conventions of xihan migrate (command, markers, report format) and the rule set for each source version
└─ conformance/    # Conformance profiles (profiles.json), vectors (vectors/) and the black-box runner (Go module, see conformance/README.md)
```

The documents in this repository are written in Chinese.

## Normative and Informative

| Normative (must match across implementations that claim the profile) | Informative (each language decides) |
| --- | --- |
| Primitive semantics and wire encoding | Internal API shape |
| Manifest schema | Package management and directory layout |
| Context propagation keys and carriers | Logging implementation |
| Pipeline stage names and ordering rules | Concrete middleware |
| Plugin protocol | Isolation mechanism |
| Protocol mappings | Server implementation |
| Vocabulary, error codes, capability IDs, operation IDs, package ID naming | Code style |
| Batch first: collection inputs, native batching, partial-failure semantics (`conventions/batch-first.md`) | Batch size |

Normative content is divided into conformance profiles (RFC-0024), listed in [`conformance/profiles.json`](./conformance/profiles.json): an implementation claims some of the profiles and answers only for those it claims; the status line of every RFC names the profiles it belongs to.

| Profile | Content |
| --- | --- |
| `core` | Claimed by every implementation: primitives and wire encoding, reading and writing manifests, invocation context, pipeline stages and ordering, message templates, operation contracts and payloads, naming conventions |
| `plugin` | Writing out-of-process plugins with the implementation: the plugin side of the handshake, lifecycle, invocations and callbacks, checked by the black-box runner |
| `host` | Hosts: capability resolution, the out-of-process plugin host, protocol mappings, application manifests and packaging, version migration |
| `wasm` | The Wasm plugin ABI (Experimental) |
| Subsystem profiles | One per subsystem RFC, named after its capability ID prefix: `persistence`, `caching`, `multitenancy`, `security`, `messaging`, `localization`, `objectstorage`, `scheduling`, `configuration`, `auditing`, `workflow`, `ai`, `observability` |
| `provider:<operation set>` | Operation sets of remote providers; currently only `provider:caching.distributed@1` |

## Conformance

| Part | Location | Description |
| --- | --- | --- |
| Conformance vectors | `conformance/vectors/` | JSON vectors, one directory per topic (`primitives/`, `manifest/`, `pipeline/`, `operations/`, `plugin/`, `security/`, `provider/`); SDK unit tests in every language read these files directly, and Go code can `go get github.com/xihanfun/framework-spec/conformance` and read them through the embedded `conformance.Vectors` |
| Black-box runner | `conformance/cmd/xihan-conformance` | Language-neutral; acting as the host, it starts an out-of-process plugin written in any language; the runner itself is written in Go |

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
```

| Command | Purpose |
| --- | --- |
| `xihan-conformance plugin --dir <plugin-dir>` | Checks, per `plugin/handshake.md`, `lifecycle.md` and `invocation.md`: the handshake line and version negotiation, refusal when there is no common version, the session token (including two token values), manifest consistency, Configure / Start / Health / WatchHealth / Stop and exit within the grace period, `FailedPrecondition` for calls out of order, `Unimplemented` for unknown operations, a `status` returned by every declared operation, `plugins.process.unavailable` for calls and deliveries before Start, acknowledged subscription delivery, `plugins.event.unsubscribed` and `InvalidArgument` for deliveries nobody subscribes to or with an unknown mode, the first `InvokeStream` frame, `Failed` when the configuration is not a JSON object, exit after a Stop before Start, and exit once stdin is closed; every plugin is started with a two-entry `XIHAN_PLUGIN_HOST_ADDRESS` |
| `xihan-conformance vectors` | Validates the vector files themselves |

What each vector file contains, the rules package, and the runner's cases and options are described in [conformance/README.md](./conformance/README.md) (in Chinese).

## Consuming the Spec

Spec files link to one another by relative path, so the whole repository can be added to an implementation repository as a git submodule:

```bash
git submodule add https://github.com/XiHanFun/framework-spec.git spec
git submodule update --init
```

XiHan.Framework references it at `framework/spec`, and each SDK at `spec`. Each language generates code from `plugin/plugin.proto` with its own protobuf toolchain; `buf.yaml` at the repository root defines the lint and breaking-change checks, and `buf.gen.yaml` generates the Go code for the runner.

## RFCs

Public contracts start as RFCs; see [Contributing](#contributing) for the process. Reference implementation paths in the Status column are relative to `framework/src` in XiHan.Framework.

| RFC | Title | Status |
| --- | --- | --- |
| [0001](./rfcs/0001-primitives.md) | Primitives layer | Adopted (reference implementation `XiHan.Framework.Primitives`) |
| [0002](./rfcs/0002-manifest.md) | Manifest | Adopted (reference implementation `Kernel.Capabilities`) |
| [0003](./rfcs/0003-invocation-context.md) | Invocation context and propagation | Adopted (carrier mappings are implemented with the protocol adapters) |
| [0004](./rfcs/0004-pipeline-stages.md) | Pipeline stages and ordering | Adopted (reference implementation `Kernel.Pipeline`) |
| [0005](./rfcs/0005-persistence.md) | Domain protocols and persistence | Adopted (reference implementation `Domain`, `Persistence.Abstractions`; suite `Persistence.Conformance`) |
| [0006](./rfcs/0006-caching.md) | Caching, distributed locks and idempotency | Adopted (reference implementation `Caching.Abstractions`, `Caching.InMemory`, `Caching.Redis`, `Caching.Hybrid`; suite `Caching.Conformance`) |
| [0007](./rfcs/0007-multitenancy.md) | Multi-tenancy | Adopted (reference implementation `MultiTenancy.Abstractions`, `MultiTenancy.InMemory`; suite `MultiTenancy.Conformance`); provider `MultiTenancy.Database` |
| [0008](./rfcs/0008-security.md) | Authentication and authorization | Adopted (reference implementation `Security.Abstractions`, `Security.InMemory`, `Security.Jwt`, `Security.OpenIdConnect`; suite `Security.Conformance`); providers `Security.Database`, `Security.Ip2Region` (IP geolocation) |
| [0009](./rfcs/0009-messaging.md) | Messaging, the CloudEvents envelope and the outbox | Adopted (reference implementation `Messaging.Abstractions`, `Messaging.InMemory`, `Messaging.RabbitMQ`, `Messaging.Kafka`, `Messaging.Redis`; suite `Messaging.Conformance`) |
| [0010](./rfcs/0010-localization.md) | Localization: English by default and 10 built-in locales | Adopted (reference implementation `Localization.Abstractions`, `Localization.InMemory`); provider `Localization.Database`, suite `Localization.Conformance` |
| [0011](./rfcs/0011-operations.md) | Operation contracts and XGI messages | Adopted (reference implementation `Kernel.Operations`) |
| [0012](./rfcs/0012-objectstorage.md) | Object storage | Adopted (reference implementation `ObjectStorage.Abstractions`, `ObjectStorage.FileSystem`, `ObjectStorage.S3`; suite `ObjectStorage.Conformance`) |
| [0013](./rfcs/0013-scheduling.md) | Scheduling and leader election | Adopted (revised 2026-10-26: job store, dynamic jobs, multiple triggers, misfire, pausing, history, cluster dispatch; reference implementation `Scheduling.Abstractions`, `Scheduling.InMemory`, `Scheduling.Database`; suite `Scheduling.Conformance`) |
| [0014](./rfcs/0014-configuration.md) | Remote configuration and secrets | Adopted (reference implementation `Configuration.Abstractions`, `Configuration.Nacos`; suite `Configuration.Conformance`; the default implementation is .NET's native files and environment variables, with no package) |
| [0015](./rfcs/0015-auditing.md) | Auditing | Adopted (reference implementation `Auditing.Abstractions`, `Auditing.InMemory`; suite `Auditing.Conformance`); provider `Auditing.Database` |
| [0016](./rfcs/0016-packaging.md) | Application manifest and packaging | Adopted (reference implementation `xihan pack` in `Cli`; spec `packaging/`) |
| [0017](./rfcs/0017-plugin-protocol.md) | Out-of-process plugin protocol `xihan.plugin.v1` | Adopted (spec `plugin/`; reference implementation `Plugins.OutOfProcess`; suite `Plugins.Conformance`) |
| [0018](./rfcs/0018-wasm-plugins.md) | Wasm plugin ABI (Experimental): the same messages through exported functions + linear memory | Adopted (spec `plugin/wasm.md`; reference implementation `Plugins.Wasm`; suite `Plugins.Conformance`) |
| [0019](./rfcs/0019-remote-providers.md) | Remote providers: subsystem capabilities provided through plugin operations, with `caching.distributed@1` as the first operation set | Adopted (spec `provider/`; adapter `Caching.Remote`) |
| [0020](./rfcs/0020-migration.md) | Version migration: `xihan migrate <source-version>` performs syntax-level rewrites by rule set, with no runtime compatibility bridge | Adopted (spec `migration/README.md`; implementation `XiHan.Framework.Cli`) |
| [0021](./rfcs/0021-workflow.md) | Workflow: durable execution | Adopted (revised 2026-10-26: trigger-based starts, event gateways, external tasks, HTTP and compensation activities, jumps and migration, claim / send back / expedite, reminders and approval thresholds, retention cleanup; reference implementation `Workflow.Abstractions`, `Workflow.InMemory`, `Workflow.Database`; suite `Workflow.Conformance`) |
| [0022](./rfcs/0022-ai.md) | AI: chat, embeddings, tools and knowledge | Adopted (reference implementation `AI.Abstractions`, `AI.OpenAI`, `AI.VectorData`; suite `AI.Conformance`) |
| [0023](./rfcs/0023-observability.md) | Observability: export and runtime information | Adopted (reference implementation `Observability.Abstractions`, `Observability.Host`, `Observability.OpenTelemetry`; runtime information does not start external commands, and the items it cannot obtain are listed in the RFC) |
| [0024](./rfcs/0024-versions-and-profiles.md) | Spec versions and conformance profiles | Adopted (profile table `conformance/profiles.json`, checked by `tools/speccheck` and the tests of the conformance module) |

## Versioning

The spec is tagged with its own semantic versions `vX.Y.Z` (RFC-0024); the Go conformance module gets `conformance/vX.Y.Z` on the same commit.

- PATCH changes no normative content.
- MINOR only adds, or pins down behavior that was previously unspecified, without breaking interoperation with a peer that implements an earlier MINOR.
- MAJOR changes behavior that was already specified, removes content or breaks the wire; `v2` coexists with `v1`, matching the freeze rule of the primitives layer (RFC-0001).
- Before `v1.0.0` the tags are `v1.0.0-rc.N`, and anything may change between release candidates; `v1.0.0` is tagged together with XiHan.Framework 5.0.0.
- Changes are recorded in [CHANGELOG.md](./CHANGELOG.md).
- Day-to-day spec commits go to `dev`, `main` is fast-forwarded from `dev`, and tags are put on `main`. Day-to-day branches of implementations may point at any pushed spec commit; a release must point its submodule exactly at a spec tag, and a stable release only at a stable tag.

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

SDK repositories are named `framework-sdk-<language>` (ADR-0010). Maturity is judged by the claimed profiles (RFC-0024):

- Reference: .NET.
- Stable: no gaps in the claimed profiles, the runner passes, and a release pins a stable spec tag.
- Experimental: gaps remain, or there is no such release yet.
- Community: maintained by the community.

| Implementation | Claimed profiles | Gaps | Maturity |
| --- | --- | --- | --- |
| XiHan.Framework | `core`, `host`, `wasm`, every subsystem profile, `provider:caching.distributed@1` (adapter) | None | Reference |
| framework-sdk-go | `core`, `plugin`, `provider:caching.distributed@1` (plugin) | None | Experimental (no release pins a spec tag yet) |
| The TypeScript, Rust, Java, Python and PHP SDKs | `core`, `plugin` | None | Experimental (same) |

## Contributing

[Issues](https://github.com/XiHanFun/framework-spec/issues) and pull requests are welcome. Changes to public contracts go through the RFC process:

1. Create `NNNN-<slug>.md` under `rfcs/` with the status `草案` (draft).
2. Once the review passes, write an ADR (in `plans/adr/` of XiHan.Framework), change the RFC status to `已采纳` (adopted), and merge its text into the matching normative directory.
3. The implementation and conformance vectors are merged together with the RFC; public contract changes without an RFC are not merged.
4. A commit that changes normative content adds an entry under "Unreleased" in `CHANGELOG.md`, marked PATCH, MINOR or MAJOR with its ADR (RFC-0024); new normative documents and vectors go into the profile table.

Before committing, run `go test ./...` in `tools/speccheck` as well: every framework error code that appears in the text, the vectors or the protocol must be registered or retired in `conventions/error-codes.md`, and the manifest Schema must agree with the reading steps of RFC-0002 on every manifest vector; the profile table must cover every normative document, and the profiles named on each RFC status line must match it.

## License

[MIT](./LICENSE) © 2021-Present XiHanFun and contributors.
