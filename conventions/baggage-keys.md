# Baggage 键

调用上下文随请求跨进程传播的键值。键名由本文登记，格式 `xihan.<name>`，小写，连字符分隔。

| 键 | 含义 | 值格式 |
| --- | --- | --- |
| `xihan.tenant` | 当前租户 | 租户 ID 字符串 |
| `xihan.principal` | 当前主体 | 主体 ID 字符串 |
| `xihan.correlation` | 业务关联 ID | 不透明字符串 |
| `xihan.idempotency-key` | 幂等键 | 不透明字符串 |
| `xihan.locale` | 请求文化 | BCP 47 |

## 载体

| 载体 | 映射 |
| --- | --- |
| HTTP | W3C `baggage` 头；Trace 走 `traceparent`；截止时间走 `xihan-deadline`（RFC 3339，UTC） |
| gRPC | 同名元数据键 |
| CloudEvents | 扩展属性，去掉点与连字符：`xihantenant`、`xihanprincipal`、`xihancorrelation`、`xihanlocale`；Trace 走 `traceparent`（CloudEvents 分布式追踪扩展）。`xihan.idempotency-key` 不进入消息，见下文 |
| 插件协议 | `metadata` 映射，键名原样 |

敏感信息不得进入 Baggage；主体只传 ID，不传声明集。

消息不携带幂等键：派发为每条消息开新调用且不写幂等键（RFC-0009 投递第 4 条）。幂等键随消息传下去，同一请求发出的几条消息与同一条消息的重试会被 Idempotency 阶段当成同一次调用，第一次之后只回放结果、不再执行处理器。CloudEvents 载体因此只映射上表四个键；发布侧与派发侧的规则见 RFC-0009「信封」。
