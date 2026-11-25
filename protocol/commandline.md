# 命令行载体

> 规范性。参考实现：`XiHan.Framework.Hosting.CommandLine`。命令行参数翻译为 XGI 调用，一次进程一条命令。

## 上下文映射（RFC-0003）

| 字段 | 选项 | 说明 |
| --- | --- | --- |
| Baggage `xihan.tenant` | `--tenant <id>` | |
| Baggage `xihan.correlation` | `--correlation <id>` | |
| Baggage `xihan.locale` | `--locale <locale>` | 规范化后写入；失败文案据此翻译 |
| Deadline | `--timeout <seconds>` | 正数秒，按宿主时钟换算为绝对截止时间 |
| Credential | `--authorization '<scheme> <value>'` 或环境变量 `XIHAN_AUTHORIZATION` | 原样进入凭据，由 Authenticate 阶段解释；选项优先 |
| Trace | — | 每次调用生成新的 trace-id |

## 操作映射（RFC-0011）

- `invoke <operation-id> [...]` 调用操作；`<resource> <verb> [...]` 等价于 `invoke <resource>.<verb> [...]`（命令名不是已注册命令、且能拼成合法操作 ID 时）。
- 发布范围：`Expose` 含 `CommandLine` 且非 `Stability.Internal` 的操作；其他 → `HOSTING_NOTFOUND_001`，退出码 3。
- 载荷（JSON 对象）：`--json <object>` > `--data <file>`（`@file` 亦可）> 其余 `--<field> <value>` 选项（没有字段时为 `{}`）。`--json` 与 `--data` 的载荷原样进入。`--json` 的文本严格编码成 UTF-8（RFC-0001「JSON 文本」）：编码不成（孤立的代理项）与不是 JSON 对象同样是用法错误，不换成 U+FFFD 照常调用（2026-11-26 写明，ADR-0312）。保留选项：`json`、`data`、`authorization`、`tenant`、`correlation`、`locale`、`timeout`、`help`。
- 字段选项的类型（ADR-0128）：操作带 `OperationSchema` 时按请求 Schema 写入。属性取根的 `properties`，没有声明的取 `additionalProperties`；`$ref`、`allOf`、`anyOf`、`oneOf` 展开后取并集。允许 `string` 或不约束类型的写字符串；否则 `boolean`、`integer`、`number` 写 JSON 字面量；`array` 写数组（单值也是一元数组，元素按 `items`）。重复的选项总是数组，目标不是数组时元素写字符串。`object` 不从文本解析，嵌套对象用 `--json` 或 `--data`。字段选项的名字与值有孤立的代理项时与 `--json` 编码不成同样是用法错误（同一个错误码与消息键），不发起调用（RFC-0001「写出与转交」；2026-11-26 写明，ADR-0315；以前 .NET 照 JSON 写出器的缺省写成 U+FFFD 照常调用）。
- 转换不了的值按字符串写入，不算用法错误：进入管道后由 Validate 阶段报 `KERNEL_VALIDATION_003`，退出码 2，违规落在字段路径上。
- 操作没有 `OperationSchema` 时值都写字符串，重复的选项成字符串数组，由请求类型的序列化上下文决定怎么读。
- 成功：响应 JSON 一行写到 stdout，退出码 0。
- 失败：stderr 一行 `{"code":"<错误码>","detail":"<文案>","traceId":"<trace-id>"}`；退出码按 KIND：VALIDATION 2、NOTFOUND 3、DENIED 4、CONFLICT 5、UNAVAILABLE 6、TIMEOUT 7、CANCELLED 8、其他 1。用法错误（无命令、未知命令、选项或载荷不合法）→ 64，stderr 为同样的 JSON（`HOSTING_VALIDATION_001` / `HOSTING_NOTFOUND_002`）。
- 弃用（2026-11-24 增补，ADR-0280；以前命令行只在 `operations` 的稳定性一列看得到弃用，调用时没有任何提示）：稳定性是 `Deprecated` 的操作照常调用，stdout 的输出与退出码不变。
  - 调用进入管道之前，在 stderr 写一行弃用提示：面向人的一句话，按调用方区域（`--locale`，没有时是默认区域）翻译，带操作 ID；每次调用写一行。调用失败时这一行在失败的那一行 JSON 之前。
  - 用法错误（退出码 64）没有发起调用，不写这一行；其余级别的操作不写。
  - `operations` 里弃用的操作，稳定性一列是 `Deprecated`。
  - 带 `--help` 的命令行指向一个弃用的操作（`invoke <operation-id>` 或简写）时，帮助之后空一行，再写同一句提示（按默认区域，与帮助的其余文案一样）。

## 内置命令

| 命令 | 输出 |
| --- | --- |
| `help`（或无命令、`--help`） | 用法与命令列表；无命令时写到 stderr 并返回 64 |
| `graph` | `App.DescribeGraph()` |
| `pipeline` | `App.DescribePipeline()` |
| `operations` | 每行 `ID<TAB>稳定性<TAB>暴露<TAB>permission=…/policy=…/-` |
| `invoke` | 见上 |
| `mcp`（`Protocols.Mcp` 注册） | stdio 传输：stdin / stdout 每行一条 JSON-RPC 消息；接受与 `invoke` 相同的上下文选项（`--authorization` 或 `XIHAN_AUTHORIZATION`、`--tenant`、`--correlation`、`--locale`、`--timeout`），作用于会话里的每个 `tools/call`，见 `protocol/mcp.md` |

## 进程约定

- 日志只写 stderr；stdout 只有命令输出。
- Ctrl+C 取消当前命令（退出码 8），宿主仍按顺序停止。
