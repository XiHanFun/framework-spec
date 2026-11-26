# 向量与运行器

## 向量文件

- JSON，两空格缩进，按主题放在 `conformance/vectors/`（`primitives/`、`manifest/`、`pipeline/`、`operations/`、`context/`、`plugin/`、`caching/`、`security/`、`provider/`，各子系统档一个目录：`messaging/`、`multitenancy/`、`objectstorage/`、`scheduling/`、`localization/`、`configuration/`、`persistence/`、`auditing/`、`workflow/`、`observability/`、`ai/`）。
- 文件名（不带目录）全仓唯一、要具体（`cache-keys.json`，不叫 `keys.json`）：实现的向量覆盖测试按文件名认回放，一致性模块的测试 `TestVectorFileNamesAreUniqueAcrossDirectories` 守住。
- 顶部 `$comment` 是这个文件的说明书：每个字段的含义、规范出处（RFC 与章节）、ADR，期望值的来源。用例多的文件每条带 `name`（中文短语），照同目录已有文件的形态写。
- 不可见字符一律写成 JSON 转义（`\u00a0`、`\u2028`、`\ufeff` …），现有向量里没有原字符。
- 文件名与 `conformance.Names()` 的写法相同（`plugin/handshake-lines.json`），档表的 `vectors` 也这样写。

## 期望值由 .NET 算出

用 file-based app 调用参考实现，把输出按向量格式写出，脚本放在临时目录，不提交：

```csharp
#:project <XiHan.Framework 克隆>/framework/src/XiHan.Framework.Kernel/XiHan.Framework.Kernel.csproj
#:property TargetFramework=net10.0

// 按向量的输入调用参考实现，把结果按向量格式写到标准输出
```

- `#:property TargetFramework=net10.0` 不能少，否则会跑在本机更高的 .NET 上。
- 需要别的包时 `#:project` 引用对应项目；运行 `dotnet run <脚本>.cs`。
- 含反斜杠或 `\uXXXX` 转义的内容不经 shell heredoc 写（可能吞掉反斜杠），编辑工具也可能把转义写成原字符；写完用脚本查一遍不可见字符，脚本里拼转义用 `chr(92)`。

例外是插件一侧（SDK 认领的 `plugin` 档）没有 .NET 实现的向量：期望值由 Go 一致性模块（`conformance/rules`，不是 Go SDK）的规则函数算出，再用 .NET 能用的部分与各语言的标准库交叉核对，照 `plugin/sdk-envelope-times.json`（ADR-0310）的做法。`$comment` 注明算出期望值的规则函数，以及哪几项由 .NET、哪几项由哪种语言的标准库核对；核对用的程序同样放在临时目录，不提交。

## 规则包

- 一条规范规则一个函数，命名照 `conformance/README.md`「规则」一节（`ParseHandshakeLine`、`MapCallbackStatus`、`ReadProtoError`、`CheckJSONText` …）。
- 每个函数都由向量验证；`plugin-messages.json` 另核对模板占位符与参数一致，`callback-status.json` 的 `details` 成员与所用消息键的参数一致。
- 模板的格式化结果由 .NET 回放 `message-templates.json` 核对，规则包不另实现一份。

## 运行器

- 每个用例单独启动一次插件进程；每一步有截止时间（`--timeout`，默认 15 秒）；调用插件时每次都带会话令牌。
- 传给插件的环境变量只取白名单（`plugin/process.go` 的 `inheritedNames` 与 .NET 运行时配置前缀），测试 `TestEnvironmentKeepsOnlyTheWhitelist` 守住。
- `XIHAN_PLUGIN_HOST_ADDRESS` 与 .NET 宿主一样是两项的列表、回环 TCP 在最后：Windows 上是 `npipe \\.\pipe\xihan-conformance-<随机>;tcp 127.0.0.1:<端口>`，其他系统是 `unix <临时目录>/xihan-conformance-<随机>/host.sock;tcp 127.0.0.1:<端口>`。两项都没人监听，回调由各实现自己的测试覆盖。
- Windows 上清单 `entry` 没有扩展名时补 `.exe`。
- 收尾：断开连接、关闭 stdin，2 秒内不退出才强制终止。
- 退出码：`0` 全部通过（含跳过），`1` 有用例失败，`2` 参数或目录错误。
- 作为库：`plugin.Run(ctx, plugin.Options{Directory: dir})`；`TestAgainstAPluginDirectory` 在设置 `XIHAN_CONFORMANCE_PLUGIN_DIR`（可选 `XIHAN_CONFORMANCE_EXCLUDE`，逗号分隔；可选 `XIHAN_CONFORMANCE_ENVELOPES`，回显信封的操作）时对该目录跑全部用例。

## 用真实插件验证

```bash
cd conformance
go build -o bin/xihan-conformance ./cmd/xihan-conformance
bin/xihan-conformance plugin --dir <插件目录> --exclude echoes.crash --sample 'echoes.create={"text":"x"}'
```

- 可用的插件：Go SDK 的 `examples/echo`（`../framework-sdk-go`），XiHan.Framework 的 `framework/samples/EchoPlugin`（Release 构建后的输出目录）。
- Windows 与 Linux 各跑一遍：两边的回调端点、可执行文件名与进程退出方式不同。本机只有 Windows 时，Linux 一侧交给 CI 或容器。
- `--json` 输出结构化结果，`--case <名>` 只跑指定用例，`--quiet` 不转发插件的 stderr。
