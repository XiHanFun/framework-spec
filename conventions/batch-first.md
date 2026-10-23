# 批量优先

> 规范性。适用于所有子系统的抽象与各语言 SDK；RFC-0005（仓储）、RFC-0006（缓存）、RFC-0009（消息）、RFC-0012（对象存储）、RFC-0013（调度）、RFC-0019（远程 Provider 操作集）据此定义批量方法。

## 规则

1. **集合是一等入参**。对多个对象的写入，以及对已有对象按标识执行的操作（删除、状态变更、撤销等），抽象必须提供以集合为入参、一次调用完成的批量方法；单条是批量的特例（`Add(x)` ≡ `AddRange([x])[0]`）。
2. **实现原生批量**。Provider 用后端的原生批量接口（SQL 多行插入 / `UPDATE … WHERE`、Redis `MGET` / 管道、消息代理的批量发布与确认），不在实现里循环单条，也不并发发出多个单条请求。默认实现（内存）可以逐项处理，但仍是一次调用、一次加锁。
3. **调用方不循环单条**。框架内部、示例与迁移工具处理多个对象时调用批量方法；一致性套件与架构测试检查这一点。
4. **部分失败语义写明**，二选一并在 RFC 里注明：
   - **逐项结果**：返回与入参顺序一一对应的 `Result` 列表（校验类失败：标识重复、不存在、并发冲突、主题不合法）；
   - **汇总异常**：基础设施失败（后端不可用、超时）整批抛出或整批失败，不伪装成部分成功。
   同一方法可以两者并用：先逐项校验，通过校验的项一次性写入，写入阶段的基础设施失败对这些项统一报告。
5. **空集合**：不触后端，返回空结果；仍然要求前置条件成立（如活动的工作单元）。
6. **重复项**：按入参顺序逐项施加语义，后一项看见前一项的效果（新增同一标识两次，第二次是重复；删除同一键两次，第二次是不存在；写入同一键两次，后者覆盖）。
7. **取消时机**：进入后端之前检查取消；写入阶段是原子的一步（或在工作单元事务内），不在中途取消而留下半批。
8. **顺序与上限**：逐项结果保持入参顺序；批次大小由调用方控制，抽象不强加上限，但 Provider 可以按后端限制分片并对调用方透明。
9. **跨语言一致**：操作集与各语言 SDK 的批量方法形态（入参集合、逐项结果）与本文一致；远程 Provider 的操作集为每个批量方法提供对应的操作（如 `cache-entries.get-many`）。

## 命名

| 语义 | .NET | Go | 操作集 |
| --- | --- | --- | --- |
| 批量新增 | `AddRangeAsync(entities)` | `AddRange(ctx, entities)` | `<resource>.add-many` |
| 批量更新 | `UpdateRangeAsync(entities)` | `UpdateRange(ctx, entities)` | `<resource>.update-many` |
| 批量删除 | `RemoveRangeAsync(entities)` / `RemoveManyAsync(keys)` | `RemoveRange` / `RemoveMany` | `<resource>.remove-many` |
| 批量读取 | `FindManyAsync(ids)` / `GetManyAsync(keys)` | `FindMany` / `GetMany` | `<resource>.get-many` |
| 批量写入（键值） | `SetManyAsync(entries)` | `SetMany(ctx, entries)` | `<resource>.set-many` |
| 批量发布 | `PublishManyAsync(topic, envelopes)` | `PublishMany(ctx, topic, envelopes)` | `Host.Publish`（协议已批量） |

实体集合用 `Range` 后缀，键集合用 `Many` 后缀；返回逐项结果时类型是与入参等长的列表。
