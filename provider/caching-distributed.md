# `caching.distributed@1` 的操作集

> 规范性。RFC-0019、RFC-0006。适配器参考实现：`XiHan.Framework.Caching.Remote`；插件参考实现：`framework-sdk-go` 的 `caching` 包。向量：`../conformance/vectors/provider/caching-distributed.json`。

## 1. 操作

资源名 `cache-entries`；五个操作与 RFC-0006 的存储契约一一对应。请求与响应都是 JSON 对象。

| 操作 | 请求字段 | 响应字段 |
| --- | --- | --- |
| `cache-entries.get` | `key` | `value`：条目的值（base64）；不存在或已过期为 `null` |
| `cache-entries.set` | `key`、`value`、`ttlMs`?、`slidingMs`? | 无（`{}`） |
| `cache-entries.try-add` | `key`、`value`、`ttlMs`?、`slidingMs`? | `added`：不存在（或已过期）时写入为 `true`，否则 `false` |
| `cache-entries.remove` | `key` | `removed`：删除前存在且未过期为 `true` |
| `cache-entries.try-remove` | `key`、`expected` | `removed`：当前值等于 `expected` 时删除为 `true`，不存在、已过期或值不同为 `false` |

## 2. 字段

- `key`：字符串，非空、不含空白与控制字符、不超过 512 字符、区分大小写。
- `value` / `expected`：标准 base64（RFC 4648 §4，带填充）编码的字节；空字节串编码为 `""`。字段缺失按空字节串。
- `ttlMs`：绝对存活毫秒数，正整数。`slidingMs`：滑动过期毫秒数，正整数。二者至多给一个；都不给时用插件的默认存活时间（插件自行决定，须大于零）。
- 响应里没有列出的字段必须忽略；请求里未知字段必须忽略。

## 3. 语义

- 过期在读取时判定：`get`、`try-add`、`remove`、`try-remove` 都把已过期的条目视为不存在。
- 滑动过期的条目在 `get` 命中时把到期时间延长为"现在 + `slidingMs`"；`try-remove` 与 `remove` 不延长。
- `set` 覆盖已有条目及其过期设置；`try-add` 原子：并发的多次 `try-add` 恰有一次 `added: true`。
- `try-remove` 原子：比较与删除之间不得插入其他写入。
- 返回的值是条目写入时的字节，插件不得改写。
- 插件只在自身串行化范围内保证原子性；跨插件实例的一致性（如多个宿主共用一个 Redis）由插件的后端保证。

## 4. 错误

| 情况 | `Status.error.code` |
| --- | --- |
| `key` 不合法 | `CACHING_VALIDATION_001` |
| `ttlMs` / `slidingMs` 不是正整数，或同时给出 | `CACHING_VALIDATION_002` |
| 后端不可用或失败 | `CACHING_UNAVAILABLE_001` |

`value` / `expected` 不是合法 base64 按 `KERNEL_VALIDATION_001`（载荷不合法）。适配器把传输层失败（插件未加载、操作未登记、进程不可达、超时）映射为 `CACHING_UNAVAILABLE_001`，细节 `cause` 带原错误码。

## 5. 清单与握手

插件清单示例（节选）：

```json
{
  "kind": "plugin",
  "isolation": "out-of-process",
  "contributes": {
    "operations": [ "cache-entries.get", "cache-entries.set", "cache-entries.try-add", "cache-entries.remove", "cache-entries.try-remove" ]
  }
}
```

握手声明的每个操作 `stability` 为 `internal`、`expose` 为空、`streaming` 为假。

## 6. 一致性

- 适配器 + 插件通过 `CacheStoreConformance` 全部用例（含 32 路并发 `try-add`、256 KiB 值、真实时间的过期与滑动过期、基于存储的分布式锁）。
- 向量 `caching-distributed.json`：操作 ID、各操作的请求 / 响应样例、非法键与非法选项样例；各语言实现的单元测试以它核对编码。
