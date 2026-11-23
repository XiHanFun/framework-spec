# 变更记录

按 RFC-0024「版本」：PATCH 不改规范性内容；MINOR 只增或写死以前不作规定的行为；MAJOR 改已经写明的行为、删除或线上不兼容。`v1.0.0` 之前的 rc 之间允许任何改动。每条注明对应的 XiHan.Framework ADR。

## 未发布

- MAJOR（`v1.0.0` 之前允许）：插件的消息目录按归属登记与撤回（ADR-0252）。RFC-0010 新增「归属目录」：本地化器从 `MessageCatalogRegistry` 读目录，`AddMessageCatalog(owner, catalog)` 在配置阶段、`Add(owner, catalogs)` 在运行期按归属登记，`RemoveOwnedBy(owner)` 撤回，同名目录照原规则核对冲突。`plugin/invocation.md` §1、`plugin/handshake.md` §3、`plugin/lifecycle.md`、`plugin/wasm.md`：握手的消息目录以清单 ID 为归属，与代理操作、权限码一次登记、全进或全不进，运行期加载的插件也有目录（以前只有配置阶段发现的才有），卸载随之撤回（以前不撤回，再加载时目录变了要重启宿主）；握手一节原先写的「目录名重复时宿主已有的优先并记警告」改为照 RFC-0010 核对冲突。
- MAJOR（`v1.0.0` 之前允许）：协议适配器跟随操作登记表（ADR-0251）。RFC-0011 新增 `OperationRegistry.GetChangeToken()`（登记表的内容每变一次触发一次）与 `EnforceAdditions(check)`（新增的一组与已登记的放在一起核对）。`protocol/http.md`：`MapXiHanOperations()` 的端点随登记表增减，运行期登记的操作立即有路由、撤回即没有路由（以前在映射时取快照，撤回后的路由答复 `KERNEL_NOTFOUND_001`），此后登记的路由冲突整组被拒（`PROTOCOLS_CONFLICT_002`）；OpenAPI 文档在登记表或分组来源变了之后重新生成，新增 `IOpenApiGroupSource` 提供运行期的分组，分组文档的地址按请求时的分组答复。`protocol/mcp.md`：tool 表按处理消息时的登记表，此后登记的 tool 名不合法或重复整组被拒，`listChanged` 仍为 `false`。`plugin/invocation.md` §1、`plugin/lifecycle.md`：运行期加载的代理操作有 HTTP 路由、进文档、是 tool，卸载随之撤回；gRPC 方法与 AI 工具目录仍只在启动时建成。

## v1.0.0-rc.2（2026-11-20）

- MAJOR（`v1.0.0` 之前允许）：RFC-0005「索引声明」：`[EntityIndex]` 可以用 `Name` 显式命名（字母开头、只含 ASCII 字母数字与 `_`），没给照旧按规则推出；SqlSugar 的 `InitTables` 在建表前核对一起建表的实体之间索引不重名；SqlSugar 在 MySQL（8.0.13 起）与 Oracle 上以每列 `CASE WHEN` 的函数索引建软删除实体的部分唯一索引，这两种库以前启动失败，EF Core 在这两种库上仍不支持（ADR-0249）。

## v1.0.0-rc.1（2026-11-20）

首个标签，内容是到这里为止的全部规范：RFC-0001～0024、22 个向量文件、黑盒运行器的 14 个用例。在此之前的修订记在各 RFC 的状态行与 ADR-0001～0243 里，不在这里逐条重列。

- MINOR：插件回调的截止时间：`plugin/invocation.md` §5 写明，发出之前「截止时间已到」指截止令牌已触发或按时钟已过；回调途中截止时间到了一律按 `DeadlineExceeded`，回调客户端不能只凭取消信号已触发就当作调用方取消（ADR-0246）。
- MAJOR（`v1.0.0` 之前允许）：`Error` 的文案不能只有空白，读线上编码时只有空白的 `message` 不合法，读入插件错误时只有空白的 `text` 与空串一样取 `code`；RFC-0001 新增「空白字符」一节，把 25 个码位写死，各语言不用内建的判断；`wire-encoding.json` 与 `error-reading.json` 补上用例（ADR-0245）。
- PATCH：.NET 的 Result 与 Error 线上编码落在内核的 `ResultJson`，参考实现回放 `primitives/wire-encoding.json`，在 `core` 里不再有缺口；RFC-0001 的 .NET 映射与 README 的生态一节随之修订（ADR-0244）。
- MINOR：规范的版本与一致性档（RFC-0024）；档表 `conformance/profiles.json`，Go 经 `conformance.Profiles()` 读取；每个 RFC 的状态行写明所属的档（ADR-0243）。
- MINOR：清单 Schema 与 RFC-0002 一致：`provides` / `requires` 去掉 `uniqueItems`，接口版本设上界 2147483647；登记 `KERNEL_CAPABILITY_006`，新增「已撤销的错误码」一节；新增 `tools/speccheck`（ADR-0242）。
