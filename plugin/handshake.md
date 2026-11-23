# 插件启动与握手

> 规范性。协议 `xihan.plugin.v1`（`plugin.proto`，RFC-0017）。参考实现：`XiHan.Framework.Plugins.OutOfProcess`（P4-02）。

## 1. 启动

宿主从插件目录读取 `xihan.manifest.json`（RFC-0002；`kind: plugin`、`isolation: out-of-process`），校验 Schema、原语层大版本与权限授予后启动进程：

- `entry` 是相对插件目录的可执行文件路径（Windows 上没有扩展名时补 `.exe`）；工作目录是插件目录。
- 宿主只继承必要的环境变量（`PATH`、`HOME` / `USERPROFILE`、临时目录、区域与时区、操作系统运行所需的系统变量、`DOTNET_*`，以及 Go 运行时的 `GOMAXPROCS`、`GOGC`、`GOMEMLIMIT`、`GODEBUG`、`GOTRACEBACK`），并追加下表；插件不得假定其他变量存在。Go 只列这几个名字、不按 `GO` 前缀继承（2026-11-12 修订，ADR-0208；以前按 `GO*` 继承，`GOOGLE_APPLICATION_CREDENTIALS` 等 Google 云凭据随之交给了插件）。
- 宿主把插件的 stderr 逐行转为自己的日志（来源 `plugins.<清单 ID>`，级别见 `invocation.md` §6）；stdout 只用于握手行，握手完成后的 stdout 输出按 Information 记录。
- 一行最多 16384 个字符（2026-11-12 修订，ADR-0185）：超过的部分宿主读掉丢弃、不进内存，行照常记录并另记一条告警；握手行超过即握手不合法（`PLUGINS_VALIDATION_003`），宿主读到上限就判定，不等行尾。
- 宿主保持插件的 stdin 打开直到停止；插件必须在 stdin 关闭（宿主退出或崩溃）时自行退出，不留孤儿进程（收尾的时限见 `lifecycle.md` §7）。

| 环境变量 | 含义 |
| --- | --- |
| `XIHAN_PLUGIN_PROTOCOL_VERSIONS` | 宿主支持的协议版本，逗号分隔、升序，如 `1` |
| `XIHAN_PLUGIN_PRIMITIVES` | 宿主的原语层大版本，如 `1` |
| `XIHAN_PLUGIN_SESSION` | 本次启动的会话令牌（随机，至少 32 字节，base64url） |
| `XIHAN_PLUGIN_HOST_ADDRESS` | 宿主回调端点，一个或多个，按宿主的偏好排序，以 `;` 分隔：`<network> <address>[;<network> <address>]…`，如 `npipe \\.\pipe\xihan-host-1f3a;tcp 127.0.0.1:50123`（§4） |
| `XIHAN_PLUGIN_INSTANCE` | 实例 ID |
| `XIHAN_PLUGIN_LOCALE` | 宿主默认区域（BCP 47） |

插件 SDK 在监听与写握手行之前核对自己的清单与声明（2026-11-18 写明，ADR-0237）：`kind` 是 `plugin`、`isolation` 是 `out-of-process`、没有 `contributes.pipeline`、`primitives` 等于 SDK 支持的原语层大版本、声明的操作都在 `contributes.operations` 里且不重复、订阅的主题合文法且（主题，类型）不重复、静态声明的健康项名在 `contributes.health` 里（健康项由健康回调在运行期给出的 SDK 在上报时按 `lifecycle.md` §7.4 丢掉未声明的名字）、消息目录含 `en-US`。不合即拒绝启动：不监听、不写握手行，在 stderr 记一条 `error` 日志；宿主看到的是进程在握手之前退出（§5）。

## 2. 握手行

插件启动 gRPC 服务端后向 stdout 写一行（`\n` 结尾），之后不得再向 stdout 写任何东西直到握手完成：

```text
xihan-plugin/1 <network> <address>
```

- `xihan-plugin/1` 是握手行格式版本（与协议版本无关，只在行格式变化时递增）。
- `network` 为 `unix`、`tcp` 或 `npipe`。
- `address` 是一个或多个非空白字符。空白（2026-11-18 写明，ADR-0237；与 .NET 正则的 `\s` 相同）指 U+0009–U+000D、U+0020、U+0085、U+00A0、U+1680、U+2000–U+200A、U+2028、U+2029、U+202F、U+205F、U+3000，即 RFC-0001「空白字符」；其余字符（含非 ASCII 字母，如路径里的中文用户名）都可以出现在地址里。
- `unix` 与 `npipe` 的地址只要求非空；`tcp` 的地址必须是回环地址加端口，写法固定（2026-11-18 写明，ADR-0237）：
  - IPv4 是 `127.0.0.0/8` 里的点分十进制，四段，每段 `0`–`255`，不带前导零（`127.0.0.1`、`127.1.2.3`；`127.1`、`0177.0.0.1`、`127.000.0.1` 都不合法）；
  - IPv6 只认 `[::1]`；
  - 端口 `1`–`65535` 的十进制，不带正负号与前导零。
- 文法：`^xihan-plugin/1 (unix|tcp|npipe) (<地址>)\z`，`<地址>` 是一个或多个上面空白之外的字符；行按 UTF-8 解码，不合法的字节序列替换成 U+FFFD；宿主按 `\n`、`\r\n` 或单独的 `\r` 分行（.NET `StreamReader.ReadLine` 的行为），行尾不属于行，插件写 `\n` 结尾（Windows 上文本模式自动写成 `\r\n` 的也合法）。一致性向量 `conformance/vectors/plugin/handshake-lines.json`。

宿主在 `HandshakeTimeout`（默认 10 秒）内读到握手行并连上端点，否则终止进程并以 `PLUGINS_TIMEOUT_001` 失败；行不合法以 `PLUGINS_VALIDATION_003` 失败。

## 3. Handshake 调用

连接后宿主调用 `Plugin.Handshake`：

1. 版本协商：插件选宿主 `protocol_versions` 里最高的自己也支持的版本写入 `protocol_version`；没有共同版本时返回 gRPC `FailedPrecondition`。宿主收到不在列表里的版本同样视为失败（`PLUGINS_VALIDATION_003`）。向量：`conformance/vectors/plugin/protocol-versions.json`。协议版本只在线上不兼容时新增一个整数，与旧版本并存；同一协议版本里的写死走规范的 MINOR，不在握手里协商（RFC-0024「版本」第 4 条）。
2. 原语层：`primitives` 必须与宿主相同，否则 `PLUGINS_VALIDATION_004`。插件写清单的 `primitives`（§1 已核对它等于 SDK 支持的版本）。
3. 清单：`manifest` 必须与目录里的文件逐字段一致（`id`、`version`、`primitives`、`kind`、`isolation`、`provides`、`requires`、`contributes`、`activation`、`permissions`），否则 `PLUGINS_VALIDATION_004`。插件写清单文件的原文，去掉开头的 UTF-8 BOM（若有）。
4. 操作：`operations[].id` 必须都出现在清单 `contributes.operations`；ID 文法与 Schema 规则同 RFC-0011；重复即 `PLUGINS_VALIDATION_004`。宿主把它们登记为代理操作（见 `invocation.md`）。插件只声明一元操作（`streaming: false`），流式操作等流式契约的 RFC（`invocation.md` §1）。
5. 订阅：`subscriptions[].topic` 必须符合主题文法；`mode` 为空、`competing` 或 `broadcast`（RFC-0009「订阅方式」），其他值即 `PLUGINS_VALIDATION_004`；重复的（主题，类型）对即 `PLUGINS_VALIDATION_004`，同一对不能以两种方式各声明一次。插件按声明的顺序写订阅，`mode` 总是写 `competing` 或 `broadcast`，不写空（2026-11-18 写明，ADR-0237）。
6. 目录：`catalogs` 以插件的清单 ID 为归属登记进宿主的本地化器（RFC-0010「归属目录」），与代理操作一起登记、随卸载撤回（`invocation.md` §1）。同名目录照 RFC-0010 的规则核对：区域与键取并集，同一键同一区域的模板不同即冲突（`LOCALIZATION_CONFLICT_001`），插件整组登记失败；不同名目录里的同一键，宿主已有的优先（2026-11-23 修订，ADR-0252；以前只有配置阶段发现的插件接进本地化器、卸载不撤回，这里写的「目录名重复时宿主已有的优先并记警告」与实现不符）。至少应含 `en-US`。插件只发自己的目录：SDK 运行时自己的消息键（`invocation.md` §8）由宿主提供，SDK 不随握手发送（2026-11-18 写明，ADR-0237；以前有的 SDK 另发一份，与宿主的同名键重复）。

握手成功后插件状态为 `Resolved`；宿主接着 `Configure` 与 `Start`（`lifecycle.md`）。整个握手阶段的 gRPC 调用都带 `HandshakeTimeout` 的截止时间。`HandshakeRequest.host_endpoint` 是 `XIHAN_PLUGIN_HOST_ADDRESS` 的第一项，只供诊断；插件选回调端点以环境变量为准（§4）。

## 4. 会话令牌与回调

- 插件回调 `Host` 服务时必须在元数据 `xihan-plugin-session` 里带 `XIHAN_PLUGIN_SESSION`；缺失或不匹配返回 gRPC `Unauthenticated` + `PLUGINS_DENIED_002`。
- 宿主调用 `Plugin` 服务的每一次 RPC（含 `Handshake`）同样在元数据 `xihan-plugin-session` 里带这个令牌。插件必须校验（按常数时间比较），缺失或不匹配以 gRPC `Unauthenticated` 拒绝、不执行（2026-11-12 修订，ADR-0202；以前只有回调带令牌，本机其他用户或同一 Pod 的边车容器直接连插件端点，就能冒充宿主、以任意主体与租户调用插件）。元数据里必须恰好有一个 `xihan-plugin-session` 值：没有、或有两个及以上（即使其中一个正确）都按不匹配拒绝（2026-11-18 写明，ADR-0237）。会话令牌的校验先于其他一切处理，包括流式调用读第一帧。
- 令牌只对本次进程有效；重启后换新令牌。
- 宿主的回调端点（2026-11-18 修订，ADR-0237；以前只有一个，Windows 上是命名管道，而 Python、Java、PHP 的 gRPC 库连不上命名管道，这些插件在 Windows 上回调不了宿主）：
  - 宿主提供一个或多个端点，按偏好写进 `XIHAN_PLUGIN_HOST_ADDRESS`，每项的 `network` 与 `address` 同 §2，地址另不得含 `;`；宿主的临时目录路径含空白或 `;` 时不提供 Unix 域套接字这一项。整个值的文法：`^(unix|tcp|npipe) <地址>(;(unix|tcp|npipe) <地址>)*\z`，任何一项不合法整个值就不合法。向量：`conformance/vectors/plugin/host-addresses.json`。
  - 宿主至少提供一个回环 TCP 端点，放在最后：.NET 宿主在 Windows 上提供命名管道与回环 TCP，在其他系统上提供 Unix 域套接字与回环 TCP。各端点服务同一个 `Host` 服务、校验同一个会话令牌。
  - 插件按顺序取第一个自己的 gRPC 库能连的端点（`network` 是自己支持的）回调，之后不换。变量缺失、不合法或没有一项能连时，插件照常服务，启动时在 stderr 记一条 `warning`，每次回调都以 `PLUGINS_UNAVAILABLE_001` + `plugins.host.unavailable`（`reason` 为 `no supported callback endpoint`）失败，不发出请求（`invocation.md` §5）。
- 宿主端点与插件端点都只接受本机连接：
  - Unix 域套接字放在本进程新建的私有目录里：目录名随机、权限 `0700`，已经存在就不用（可能是别人抢先建的、指向别处的链接），套接字文件 `0600`（2026-11-12 修订，ADR-0202；以前宿主把回调套接字直接放在共享的临时目录下）。
  - 命名管道：宿主的命名管道只允许当前用户并拒绝远程客户端。插件的命名管道应同样只允许当前用户、拒绝远程客户端；运行时给不了访问控制表或拒绝远程客户端的标志的（Node 的 libuv 两样都给不了，不用 unsafe 的 Rust 只能拒绝远程客户端）用系统缺省的安全描述符：其他用户只能以只读打开、发不出请求，隔离与 TCP 一样靠令牌，SDK 在文档里写明，需要只接受本机连接的改用回环 TCP（2026-11-18 修订，ADR-0237）。
  - TCP 只绑定回环地址，同机的其他用户连得上，隔离只靠令牌。

## 5. 失败与重试

握手阶段任何失败都终止进程、记为 `Failed`，并按 `lifecycle.md` 的重启策略处理；握手失败不会让宿主进程退出。
