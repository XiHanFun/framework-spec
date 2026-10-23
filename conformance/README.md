# 一致性：向量与运行器

Go 模块 `github.com/xihanfun/framework-spec/conformance`。

## 向量（`vectors/`）

| 文件 | 内容 | 规范 |
| --- | --- | --- |
| `primitives/error-codes.json` | 错误码格式的合法与非法样例 | `conventions/error-codes.md` |
| `primitives/baggage-keys.json` | Baggage 键格式与已登记的键 | `conventions/baggage-keys.md` |
| `primitives/capability-requirements.json` | 能力依赖文本的解析结果 | `conventions/capability-ids.md` |
| `primitives/lifecycle-transitions.json` | 生命周期合法迁移的完整列表 | RFC-0001 |
| `plugin/handshake-lines.json` | 插件握手行的合法与非法样例 | `plugin/handshake.md` |
| `plugin/protocol-versions.json` | 协议版本协商与原语层版本核对 | `plugin/handshake.md` |
| `plugin/wasm-exports.json` | Wasm ABI v1 的导出与导入表 | `plugin/wasm.md` |
| `provider/caching-distributed.json` | `caching.distributed@1` 远程 Provider 的操作 ID（含批量）、请求 / 响应样例、非法键与选项 | `provider/caching-distributed.md` |

各语言 SDK 的单元测试直接读取这些文件。Go 里 `go get github.com/xihanfun/framework-spec/conformance` 后经内嵌文件系统 `conformance.Vectors` 或类型化入口（`conformance.HandshakeLines()` 等）读取，不需要 submodule。

## 规则（`rules/`）

规范文法与判定的 Go 实现，供运行器使用，也供 SDK 作者对照：错误码、操作 ID、主题、Baggage 键、清单 ID、能力 ID 与依赖文本、握手行（含 tcp 回环判定）、版本协商、生命周期迁移、管道阶段。每条规则都由向量验证（`rules/rules_test.go`）。

## 黑盒运行器（`cmd/xihan-conformance`）

以宿主身份启动任意语言写的进程外插件，按 `plugin/handshake.md`、`lifecycle.md`、`invocation.md` 核对：

| 用例 | 核对内容 |
| --- | --- |
| `handshake` | 握手行合法、协商出宿主支持的版本、原语层版本相同、握手清单与目录里的文件一致、操作与订阅都在清单声明范围内、目录至少一种区域 |
| `refusal` | 宿主没有共同版本时以 `FailedPrecondition` 拒绝 |
| `lifecycle` | `Configure`、`Start` 成功；`Health` 与 `WatchHealth` 报 Started / Ready / Degraded，健康项都已声明；`Stop` 成功且进程在 5 秒宽限内退出 |
| `unknown-operation` | 未声明的操作以 `Unimplemented` 结束 |
| `declared-operations` | 每个声明的一元操作都返回 `status`：成功时载荷是 JSON，失败时错误码合法 |
| `event` | 向第一个声明的订阅投递消息得到确认；没有订阅时跳过 |
| `stdin` | 关闭 stdin 后进程自行退出 |

每个用例单独启动一次插件进程；运行器不提供宿主回调服务，回调相关的权限与令牌由宿主实现的测试覆盖。

```bash
go install github.com/xihanfun/framework-spec/conformance/cmd/xihan-conformance@latest
xihan-conformance plugin --dir ./bin/echo --exclude echoes.crash --sample 'echoes.create={"text":"hi"}'
xihan-conformance plugin --dir ./bin/echo --json
xihan-conformance vectors
```

| 选项 | 含义 |
| --- | --- |
| `--dir` | 插件目录（含 `xihan.manifest.json` 与 `entry` 指向的可执行文件） |
| `--timeout` | 每一步的截止时间，默认 `15s` |
| `--config` | `Configure` 下发的配置节 JSON，默认 `{}` |
| `--sample op=json` | 某个操作的请求样例；没有的操作用 `{}` |
| `--exclude op` | 不调用的操作（如会故意崩溃的操作） |
| `--case name` | 只跑指定用例，可重复 |
| `--json` | 结果输出为 JSON |
| `--quiet` | 不转发插件的 stderr |

退出码：`0` 全部通过（含跳过），`1` 有用例失败，`2` 参数或目录错误。

作为库使用：`plugin.Run(ctx, plugin.Options{Directory: dir})` 返回每个用例的 `Result`；测试 `TestAgainstAPluginDirectory` 在设置了 `XIHAN_CONFORMANCE_PLUGIN_DIR`（和可选的 `XIHAN_CONFORMANCE_EXCLUDE`）时对该目录跑全部用例。

## 生成代码

`internal/pluginv1` 由仓库根的 `buf generate` 从 `plugin/plugin.proto` 生成并提交；改协议后重新生成，CI 核对生成物与源一致。
