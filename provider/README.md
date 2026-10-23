# 远程 Provider

> 规范性。RFC-0019。

一个子系统能力由进程外插件提供时，宿主侧的 `<领域>.Remote` 适配器声明能力并实现子系统抽象，插件实现本目录为该能力规定的操作集；二者之间只有插件协议 `xihan.plugin.v1`（`../plugin/`）与 XGI 的 JSON 载荷。

## 规则

1. 适配器的清单 `kind: provider`、`role: provider`，`provides` 列出能力；插件清单不写 `provides`，`contributes.operations` 列出操作集的全部操作。
2. 操作 ID 固定，按 `conventions/operation-ids.md` 的 `<resource>.<verb>` 文法；握手声明里 `stability` 为 `internal`、`expose` 为空，协议适配器不对外发布。
3. 适配器按操作 ID 从宿主操作登记表取代理操作并直接执行，不经过宿主管道。
4. 插件的业务失败原样成为子系统错误；传输层失败（插件未加载、操作未登记、不可达、超时）由适配器映射为子系统的不可用错误码，细节带原错误码。
5. 输入校验两侧都做：适配器先按子系统规则校验，插件也必须校验。
6. 宿主会并发调用；原子操作的原子性由插件保证。每次调用带截止时间。
7. 适配器 + 插件必须通过该子系统的一致性套件；`../conformance/vectors/provider/` 给出各操作集的向量。

## 操作集

| 能力 | 文件 | 操作 |
| --- | --- | --- |
| `caching.distributed@1` | [caching-distributed.md](caching-distributed.md) | `cache-entries.get` / `set` / `try-add` / `remove` / `try-remove` / `get-many` / `set-many` / `remove-many` |
