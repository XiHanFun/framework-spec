# 变更记录

按 RFC-0024「版本」：PATCH 不改规范性内容；MINOR 只增或写死以前不作规定的行为；MAJOR 改已经写明的行为、删除或线上不兼容。`v1.0.0` 之前的 rc 之间允许任何改动。每条注明对应的 XiHan.Framework ADR。

## 未发布（v1.0.0-rc.1）

首个标签，内容是到这里为止的全部规范：RFC-0001～0024、22 个向量文件、黑盒运行器的 14 个用例。在此之前的修订记在各 RFC 的状态行与 ADR-0001～0243 里，不在这里逐条重列。

- MAJOR（`v1.0.0` 之前允许）：`Error` 的文案不能只有空白，读线上编码时只有空白的 `message` 不合法，读入插件错误时只有空白的 `text` 与空串一样取 `code`；RFC-0001 新增「空白字符」一节，把 25 个码位写死，各语言不用内建的判断；`wire-encoding.json` 与 `error-reading.json` 补上用例（ADR-0245）。
- PATCH：.NET 的 Result 与 Error 线上编码落在内核的 `ResultJson`，参考实现回放 `primitives/wire-encoding.json`，在 `core` 里不再有缺口；RFC-0001 的 .NET 映射与 README 的生态一节随之修订（ADR-0244）。
- MINOR：规范的版本与一致性档（RFC-0024）；档表 `conformance/profiles.json`，Go 经 `conformance.Profiles()` 读取；每个 RFC 的状态行写明所属的档（ADR-0243）。
- MINOR：清单 Schema 与 RFC-0002 一致：`provides` / `requires` 去掉 `uniqueItems`，接口版本设上界 2147483647；登记 `KERNEL_CAPABILITY_006`，新增「已撤销的错误码」一节；新增 `tools/speccheck`（ADR-0242）。
