// Package plugin 是进程外插件协议 xihan.plugin.v1 的黑盒运行器：以宿主身份启动插件进程，
// 按 plugin/handshake.md、lifecycle.md、invocation.md 核对每个用例；任何语言写的插件都能用它验证。
package plugin

import (
	"io"
	"time"
)

// 用例名；Options.Cases 按这些名字筛选。
const (
	// CaseHandshake 核对握手行、版本协商、原语层版本、清单一致性与声明范围。
	CaseHandshake = "handshake"
	// CaseRefusal 核对没有共同协议版本时握手以 FailedPrecondition 拒绝。
	CaseRefusal = "refusal"
	// CaseLifecycle 核对 Configure / Start / Health / WatchHealth / Stop 与宽限内退出。
	CaseLifecycle = "lifecycle"
	// CaseUnknownOperation 核对未声明的操作以 Unimplemented 结束。
	CaseUnknownOperation = "unknown-operation"
	// CaseDeclaredOperations 核对每个声明的一元操作都返回 status。
	CaseDeclaredOperations = "declared-operations"
	// CaseEvent 核对向声明的订阅投递消息得到确认；没有订阅时跳过。
	CaseEvent = "event"
	// CaseStdin 核对 stdin 关闭后进程自行退出。
	CaseStdin = "stdin"
)

// AllCases 是全部用例，按执行顺序。
var AllCases = []string{CaseHandshake, CaseRefusal, CaseLifecycle, CaseUnknownOperation, CaseDeclaredOperations, CaseEvent, CaseStdin}

// DefaultTimeout 是每一步的默认截止时间。
const DefaultTimeout = 15 * time.Second

// Options 是一次运行的输入。
type Options struct {
	// Directory 是插件目录：含 xihan.manifest.json 与清单 entry 指向的可执行文件。
	Directory string
	// Timeout 是每一步的截止时间；零为 DefaultTimeout。
	Timeout time.Duration
	// Configuration 是 Configure 下发的配置节 JSON；空为 {}。
	Configuration string
	// Samples 是操作 ID 到请求 JSON 的映射；没有样例的操作用 {}。
	Samples map[string]string
	// Exclude 是不调用的操作 ID（如会故意崩溃的操作）。
	Exclude map[string]bool
	// Cases 是只运行的用例名；空为全部。
	Cases []string
	// Stderr 接收插件 stderr 与握手后 stdout 的每一行；nil 丢弃。
	Stderr io.Writer
}

// Result 是一个用例的结果。
type Result struct {
	// Case 是用例名。
	Case string `json:"case"`
	// Passed 为真表示通过。
	Passed bool `json:"passed"`
	// Skipped 为真表示因插件未声明相关能力而跳过（同时 Passed 为真）。
	Skipped bool `json:"skipped"`
	// Message 是失败原因或跳过原因。
	Message string `json:"message,omitempty"`
	// DurationMillis 是用时（毫秒）。
	DurationMillis int64 `json:"durationMs"`
}

// Failed 判断结果里是否有失败的用例。
func Failed(results []Result) bool {
	for _, result := range results {
		if !result.Passed {
			return true
		}
	}
	return false
}
