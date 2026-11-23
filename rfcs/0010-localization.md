# RFC-0010 本地化

> 状态：已采纳（2026-10-28 修订：消息键首段放开下划线；2026-11-01 修订：区域名长度上限与存储列长、调用链作用域用协商后的区域、目录名冲突与区域名错误的报告方式；2026-11-14 修订：数据库翻译的跨实例重新加载；2026-11-16 修订：模板文法、可移植的格式说明符与值的文本形式；同日再修订：浮点的全部写法、说明符的大小写与精度、`Describe` 按名字排序；同日第三次修订：值模型、数组与对象的文本、可移植说明符的文法、区域的数字符号；2026-11-23 修订：按归属登记与撤回的目录） | 日期：2026-10-21 | 对应 ADR：0038、0070、0233（模板与值的文本形式）、0235、0236、0252（归属目录） | 参考实现：`XiHan.Framework.Localization.Abstractions`、`XiHan.Framework.Localization.InMemory`；原语在 `XiHan.Framework.Primitives` | 一致性：`conformance/vectors/primitives/message-templates.json` | 一致性档：`core`、`localization`

## 摘要

文案不写在代码里。代码只引用消息键（专门的代码，如 `security.token.subject_missing`），每种语言一个文件（`Localization/<区域>.json`，含默认语言 `en-US`），任何地方用 `key.L()` 取当前语言的文案，`Error` 只携带键与参数并由宿主在 Egress 按调用方语言翻译。默认语言英文，内置与 XiHan.UI 相同的 10 种区域。

## 区域

| 区域 | 语言 |
| --- | --- |
| `en-US` | 英语（默认，也是语言文件的基准） |
| `zh-CN` | 简体中文 |
| `zh-TW` | 繁体中文 |
| `ja-JP` | 日语 |
| `ko-KR` | 韩语 |
| `de-DE` | 德语 |
| `es-ES` | 西班牙语 |
| `fr-FR` | 法语 |
| `pt-BR` | 葡萄牙语（巴西） |
| `ru-RU` | 俄语 |

区域名文法 `^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`，最长 35 个字符（`LocaleName.MaxLength`），比较不区分大小写，规范形式为语言小写、地区大写、文字首字母大写（`LocaleName`）。存放区域名的列（如数据库 Provider 的 `MessageTranslation.Locale`）长度取这个上限，合法的区域名都能写入。

## 消息键

```text
MessageKey = Domain ("." Segment)+
Domain     = Segment；术语表领域词的小写（kernel、persistence、security …）或应用自定义的小写标识，操作 ID 的资源段把连字符换成下划线后即是（tenant_members）
Segment    = [a-z][a-z0-9_]*
```

整个键最长 128 个字符（`MessageTemplate.MaxKeyLength`，2026-11-16 写明，ADR-0233）。

每个包一个 `<领域>Messages` 静态常量类（`KernelMessages`、`SecurityMessages`、应用的 `OrdersMessages`），只定义键，不含文案；键的说明与占位符名写在 XML 文档里。原语层按主题拆分（`ManifestMessages`、`ResultMessages`）。

## 语言文件

- 位置 `Localization/<区域>.json`，内容是键 → 模板的平面对象；以 `<EmbeddedResource Include="Localization/*.json" LogicalName="Localization/%(Filename)%(Extension)" />` 嵌入。
- 10 种内置区域每种一个文件，键集完全一致，各区域模板的占位符与 `en-US` 一致（架构测试 `LocalizationTests`）。
- 模板语法：`{name}` 按名字、`{0}` 按位置，可带 `:format` 格式说明符（`{amount:N2}`、`{when:yyyy-MM-dd}`），按命中区域的文化格式化；`{{`、`}}` 转义。逐字的规则见下节「模板」。
- `MessageCatalog.FromEmbeddedResources(name, typeof(AnyTypeInThatAssembly))` 装载一个程序集的全部语言文件。`AddMessageCatalog` 把同名目录合并为一份，留在先注册的位置：内容相同（如多个 Provider 都带同一个抽象包的目录）时什么都不变；互为补充（如插件握手带来两种语言、宿主包带全部十种）时区域与键取并集；同一键同一区域的模板不同则抛出携带 `LOCALIZATION_CONFLICT_001` 的 `FrameworkException`，应用启动失败。任何一份都不会被静默丢弃。原语层与内核的目录由 `Localization.InMemory` 代为注册；子系统目录由其 Provider 在 `Configure` 里注册；`Plugins.InProcess` 与应用的目录由应用注册。
- 归属目录（2026-11-23 修订，ADR-0252；以前目录只能在配置阶段注册，运行期加载的插件没有目录，卸载也撤不回）：本地化器从 `MessageCatalogRegistry` 读目录，它是容器里的单例，`AddMessageCatalog` 一并登记。它先放上面注册的目录，再放按归属登记的：`AddMessageCatalog(owner, catalog)` 在配置阶段登记，`Add(owner, catalogs)` 在运行期登记一组，`RemoveOwnedBy(owner)` 撤回一个归属的全部目录。插件宿主以插件的清单 ID 为归属，加载时登记、卸载时撤回（`plugin/invocation.md` §1）。
  - 按归属登记的目录不与同名目录合并成一份，但照同样的规则核对：与已有的同名目录或同一组里的同名目录在同一键同一区域的模板不同，`Add` 整组不登记，抛出携带 `LOCALIZATION_CONFLICT_001` 的 `FrameworkException`；配置阶段登记的在建立登记表时核对，同样抛出、应用启动失败。
  - 同一归属已有目录时再 `Add` 是编程错误（`InvalidOperationException`），要先撤回。空的一组什么都不登记。
  - 查找顺序：先注册的目录，再按归属登记的、按登记顺序；同一键先找到的优先，插件的目录不覆盖宿主已有的键。撤回之后，只在这个归属的目录里有的键解析不到，退回键与参数的描述。

## 模板

各语言 SDK 与宿主按同一套规则格式化（2026-11-16 写明，ADR-0233，取自 .NET `MessageTemplate` 的行为）。以前只有上面一句语法说明，Go SDK 把 `:format` 当成 printf 动词，插件把目录交给 .NET 宿主后会被误读。

- 占位符是 `{name}` 或 `{name:format}`：
  - `name` 符合 `^([A-Za-z][A-Za-z0-9_]*|[0-9]+)$`，按名字或按位置（位置参数的名字是 `0`、`1` …）。
  - 第一个 `:` 之后到 `}` 是格式说明符。
- 字面量：
  - `{{` 与 `}}` 是转义的单个花括号。
  - 花括号里不是合法名字的（如 JSON 片段 `{"a":1}`），连同花括号原样输出。
  - 没有闭合的 `{` 及其后的全部文本原样输出。
  - 参数值里的花括号不再解释。
- 缺值：
  - 宽松格式化（`MessageTemplate.Format`）把缺值的占位符连同格式说明符原样留在文本里，如 `{amount:N2}`，让遗漏可见。
  - 严格格式化（`TryFormat`，本地化器解析时用）任一占位符缺值即失败，调用方退回 `Describe`。
  - 参数存在而值为 `null` 不算缺值。
- 值模型（2026-11-16 写明，ADR-0236）：格式化之前先把参数值归到可移植的值模型，只有空、布尔、字符串、64 位有符号整数、IEEE 754 双精度浮点、数组与对象七种。
  - 各宽度的整数在 64 位有符号范围内是整数，超出时（如 .NET 的 `ulong.MaxValue`、`Int128`、`BigInteger`，JavaScript 里超出安全整数的 `number`）是双精度浮点。
  - 单精度浮点、`Half` 与十进制小数（`decimal`、`BigDecimal`、`Decimal`）先换成最接近的双精度浮点：单精度的 0.1 写 `0.10000000149011612`，`2.50m` 写 `2.5`。
  - 字符是一个字符的字符串；字符串键的映射是对象，其余序列（列表、数组、元组）是数组。
  - JSON 里读到的数字：整数写法（没有小数点与指数）且在 64 位范围内是整数，其余是双精度浮点；`-0` 是整数 0，`-0.0` 是负零。
  - 模型之外的值（日期时间、枚举、UUID 等语言自己的类型）是语言的文本：.NET 取 `IFormattable.ToString(null, 不变文化)`，其余取 `ToString()`；各 SDK 在 README 里写明。它们不跨语言，插件要跨语言格式化时先转成字符串。
- 值的文本形式（无格式说明符时，`Describe` 同样适用）：
  - 字符串原样，整数十进制，负号在前。
  - 布尔写 `true` / `false`。
  - `null` 写空串。
  - 浮点在 1e-4 ≤ |x| < 1e17 或 x = 0 时，写成能往返的最短十进制，不用指数；整数值的浮点不带小数点，`2.0` 写 `2`，1e16 写 `10000000000000000`。
  - 范围之外的浮点写成最短往返的有效数字加指数：`E`、符号（总写）、至少两位的十进制指数，如 `1E+17`、`1.2345678901234568E+17`、`9.9E-05`、`1.5E-07`、`5E-324`（2026-11-16 写明，ADR-0235，取自 .NET 10 的实测；ADR-0233 起草时把上界写成 1e15，并且没有规定范围外的写法）。
  - 负零写 `-0`；非数与无穷写 `NaN`、`Infinity`、`-Infinity`。
  - 数组与对象写成紧凑的 JSON 文本（2026-11-16 写明，ADR-0236；以前不作规定，.NET 写出 `System.Int32[]` 这样的类型名）：没有空白；对象的成员按名字的序数顺序（逐个 UTF-16 码元比较）排列，同名成员保持原顺序；嵌套的值里 `null` 写 `null`，数字与上面的写法相同（不随区域换小数点），字符串按下面的 JSON 字符串写法。超过 64 层的容器写成 `null`。
  - JSON 字符串写法：`"` 与 `\` 加反斜杠；退格、换页、换行、回车、制表符写 `\b`、`\f`、`\n`、`\r`、`\t`；其余 U+0000–U+001F 与孤立的代理项写成小写十六进制的 `\u001f`、`\ud800`；别的字符（含 `/`、U+007F–U+009F、U+2028 与非 ASCII）原样。清单写出用同一种写法。
- 可移植的格式说明符：说明符文本符合 `^[DdFfNnXx][0-9]{0,2}$` 时才认（2026-11-16 写明，ADR-0236），其余一律按无格式写出，宿主也一样，不再交给 .NET 的格式串（以前 .NET 认 `E2`、`0.00`、`yyyy-MM-dd` 等，SDK 不认，同一份目录两边写出不同的文本）。精度是字母后的 0–2 位数字，可以有前导零（`F02` 即 `F2`）；`D100`、` F2`、`F2 `、`D-1` 都不认。
- 区域的数字符号（2026-11-16 写明，ADR-0236）：格式化时给出的区域只改变小数点与分组符，取自下表（.NET 10 的 ICU 实测），区域名不区分大小写；表外的区域与不给区域一样按不变文化（`.` 与 `,`）。负号、`NaN`、`Infinity` 不随区域变化。小数点用在无格式的浮点（含指数写法的尾数，de-DE 的 1.5e20 写 `1,5E+20`）与 `F`、`N` 上，分组符只用在 `N` 上；整数无格式时不分组，`D`、`X` 与数组、对象里的数字不受区域影响。以前宿主按平台的区域数据解释，随 ICU 版本与全球化不变模式变化，en-US 的无穷写成 `∞`，SDK 一律按不变文化。

  | 区域 | 小数点 | 分组符 |
  | --- | --- | --- |
  | en-US、zh-CN、zh-TW、ja-JP、ko-KR | `.` | `,` |
  | de-DE、es-ES、pt-BR | `,` | `.` |
  | fr-FR | `,` | U+202F（窄不换行空格） |
  | ru-RU | `,` | U+00A0（不换行空格） |

  - `D` / `Dn`：整数，至少 n 位，不足补零，负号在前。如 `{n:D5}` 取 42 得 `00042`，`{n:D3}` 取 -7 得 `-007`。
  - `X` / `Xn` 与 `x` / `xn`：非负整数的十六进制，大写或小写，至少 n 位。如 `{n:x4}` 取 255 得 `00ff`。
  - `F` / `Fn`：定点小数，n 位小数，缺省 2。如 `{n:F}` 取 2 得 `2.00`。
  - `N` / `Nn`：带千分位的定点小数，不变文化下分组符是 `,`、小数点是 `.`，n 位小数，缺省 2，负号在前。如 `{n:N2}` 取 1234567.891 得 `1,234,567.89`。
  - `F` 与 `N` 的舍入：按值的精确十进制展开舍入，正好一半时取偶数（四舍六入五成双），与 .NET 的格式化一致。0.125 写 `F2` 得 `0.12`，0.375 写 `F2` 得 `0.38`；2.5 写 `F0` 得 `2`，3.5 得 `4`，-2.5 得 `-2`。2.675 的二进制值略小于 2.675，不是正好一半，写 `F2` 得 `2.67`。JavaScript 的 `toFixed`、Java 的 `String.format`、PHP 的 `number_format` 都是一半时进位，要另行处理。
  - `F` 与 `N` 不用指数，写出全部整数位：1e20 写 `F2` 得 `100000000000000000000.00`，写 `N2` 得 `100,000,000,000,000,000,000.00`。负数舍入成零时保留负号：-0.001 写 `F2` 得 `-0.00`，-0.4 写 `N0` 得 `-0`；负零写 `F` 得 `-0.00`。
  - 不适用时按无格式写出（2026-11-16 写明，ADR-0236；以前不作规定，.NET 对浮点的 `D` 抛出异常、对负数的 `X` 按类型宽度写补码）：`D` 与 `X` 只用于整数，`X` 只用于非负整数；`F` 与 `N` 用于整数与浮点；四个说明符用在字符串、布尔、`null`、数组与对象上都按无格式。超出 64 位、按模型成为浮点的整数也不认 `D` 与 `X`。
  - 字母的大小写（2026-11-16 写明，ADR-0235，照 .NET）：`D`、`F`、`N` 不区分大小写，`{n:d3}` 等同 `{n:D3}`；`X` 与 `x` 分别给出大写与小写的十六进制数字。
  - 精度 0–99 都按精确的十进制展开：0.1 写 `F99` 得到它的二进制值的全部有效数字后补零。
  - 无穷与非数写 `F`、`N` 时与无格式相同：`NaN`、`Infinity`、`-Infinity`。
- `Describe(key, arguments)`：
  - 没有参数（空或 `null`）时就是键本身。
  - 有参数时写成 `key (name=value, name=value)`，值按上面的文本形式。
  - 参数按名字的序数顺序（逐个 UTF-16 码元比较）排列（2026-11-16 修订，ADR-0235）。以前 .NET 按参数给出的顺序，无序映射的语言排不出同样的文本。
  - 它只给人看，不参与程序判断。

向量 `conformance/vectors/primitives/message-templates.json`：消息键的合法与非法样例、宽松与严格格式化的期望结果、`Describe` 的期望结果（含多个参数时的排序）。

## 解析

- `IMessageResolver`（原语层）：`TryResolve(key, locale, arguments, out text)`，找不到键或占位符缺值即失败；`Negotiate(locale)` 把调用方给出的区域协商成实现启用的区域（本地化 Provider 按下文「协商与宿主」的规则，缺省原样返回，`null` 仍是 `null`）。
- `MessageScope`（原语层）：环境作用域（解析器 + 区域），`AsyncLocal` 传播。内核在 `App.InvokeAsync` 入口把 Baggage `xihan.locale` 经解析器的 `Negotiate` 协商后安装一个作用域，调用链里 `key.L()` 与 `context.Localize`、Problem Details、gRPC 与 MCP 的失败文案用的是同一个协商后的区域：Baggage 里的区域不在启用列表时，即使有它的语言文件也不用；`Localization.InMemory` 启动时用默认区域设置进程默认。
- `key.L()`、`key.L(("id", id))`、`key.L(count, amount)`（原语层扩展方法）：按 `MessageScope.Current` 解析，调用链里是调用方语言，其他地方是默认语言；没有作用域或找不到键时返回 `MessageTemplate.Describe(key, arguments)`，即 `kernel.feature.start_failed (id=…, reason=…)`。
- `IMessageLocalizer`（Ring 2）：显式区域的入口，只有 `Options`、`Find(key, locale)` 与 `Resolve(key, locale, arguments)` 三个成员，`Resolve` 给出文案与实际命中的区域。按上下文区域取、翻译错误、退回描述这些形式是扩展方法（`MessageLocalizerExtensions`），由 `Resolve` 推出，各实现行为一致（2026-11-11 修订，ADR-0146）；`context.Localize(key, arguments)` 扩展方法按上下文区域解析。
- 回退链：请求区域 → 启用列表里同语言的区域 → 默认区域 → `en-US`；同一键先注册的目录优先。
- 数据库翻译（`Localization.Database`）：查找是同步的，翻译装在进程内的有界快照里，库里的翻译优先于目录。`ICache` 里放翻译版本戳 `localization:translations:version`：`MessageTranslationStore` 写入提交后（作用域释放时）经 `MessageTranslationLoader.PublishChangeAsync` 换版本戳，并唤醒本实例立即重新加载；每个实例按 `ChangeCheckIntervalSeconds`（缺省 1，0 关闭）核对版本戳，变了就重新加载；`RefreshIntervalSeconds`（缺省 300，0 关闭）的周期刷新兜住直接改表与版本戳丢失。重新加载彼此串行。读版本戳遇到缓存后端故障时跳过这次核对；写入后换版本戳失败只记错误日志，本实例照常重新加载。清单依赖 `persistence.relational@1` 与 `caching.local@1`（2026-11-14 修订，ADR-0229；以前只按周期刷新，写入的实例自己也要显式重新加载）。

## 错误

- `Error(code, messageKey, details)`：`MessageKey` 必须合法，`Details` 同时是模板参数；`Message` 按进程默认区域解析（日志与异常用），没有默认作用域时是键与参数的描述。
- 线上还原与异常转换用 `Error.Literal(code, text)`（无键，不翻译）；`WithCode` 换码保留键与参数，`WithText` 填入已解析文案。
- 幂等记录保存 `ErrorMessageKey` 与字符串化的 `ErrorDetails`，回放时仍可按调用方区域翻译。
- 架构测试 `ErrorCallSites_UseMessageKeyConstants`：正式包里的 `Error.*`/`new Error(` 第二个参数必须是 `<领域>Messages` 常量，`Error.Literal` 只允许在 `FrameworkException.ToError` 与 `IdempotencyRecord`。
- `FrameworkException`（配置与编程错误）不走消息键，消息为英文。
- 区域名不合法不单独占用错误码：传给 API 或写在选项里的区域名不合法是编程或配置错误，由 `LocaleName.Validate` 与各构造函数、`FromConfiguration` 在调用处抛出 `ArgumentException`；语言文件名或目录里的区域名不合法归目录不合法（`LOCALIZATION_VALIDATION_002`）；外部输入里的区域名（Baggage `xihan.locale`、`Accept-Language`）不合法时在协商中忽略，不报错。原先登记的 `LOCALIZATION_VALIDATION_001`（区域名不合法）没有任何路径会产生，已撤销登记，序号不复用。

## 协商与宿主

- 调用的区域来自 Baggage `xihan.locale`；HTTP 宿主把 `Accept-Language` 按权重取候选，经 `LocalizationOptions.Negotiate` 写入该 Baggage（已有 Baggage 时不覆盖，没有装配本地化时取第一个合法值）。协商：候选精确命中启用列表 → 按语言命中第一个同语言区域 → 默认区域。
- `XiHan:Localization:DefaultLocale`（默认 `en-US`）与 `XiHan:Localization:Locales`（逗号分隔，默认内置 10 种）可改默认与启用列表，默认区域必须在列表里。
- Problem Details：`detail` 是 `IMessageLocalizer.Resolve(error, locale)` 的文案，`Content-Language` 是实际命中的区域；没有装配本地化或自由文本错误时 `detail` 是 `Error.Message`。
- `Profiles.Minimal` 与 `Profiles.Standard` 都装配 `InMemoryLocalizationFeature`（`role: default`，`localization.resources@1`）。

## 错误码

见 `conventions/error-codes.md` 中 `LOCALIZATION_*`。
