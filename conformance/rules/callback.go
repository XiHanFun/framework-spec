package rules

import "strconv"

// 回调的两种 RPC（invocation.md §5）。
const (
	// CallbackInvoke 是 Host.Invoke。
	CallbackInvoke = "invoke"
	// CallbackPublish 是 Host.Publish。
	CallbackPublish = "publish"
)

// gRPC 状态码里映射表单独列出的几个（数值与各语言的 gRPC 库相同）。
const (
	grpcCancelled        uint32 = 1
	grpcInvalidArgument  uint32 = 3
	grpcDeadlineExceeded uint32 = 4
	grpcPermissionDenied uint32 = 7
	grpcUnimplemented    uint32 = 12
	grpcUnavailable      uint32 = 14
	grpcUnauthenticated  uint32 = 16
)

// grpcStatusNames 是 gRPC 状态码 0–16 的 PascalCase 名字（invocation.md §3），下标即状态码。
var grpcStatusNames = []string{
	"OK", "Cancelled", "Unknown", "InvalidArgument", "DeadlineExceeded", "NotFound", "AlreadyExists", "PermissionDenied",
	"ResourceExhausted", "FailedPrecondition", "Aborted", "OutOfRange", "Unimplemented", "Internal", "Unavailable", "DataLoss",
	"Unauthenticated",
}

// GRPCStatusName 返回 gRPC 状态码的 PascalCase 名字（invocation.md §3，如 InvalidArgument、Unauthenticated）；
// 0–16 之外的状态码规范没有名字，写成十进制数（与 .NET 的枚举对未定义值的 ToString 相同）。
func GRPCStatusName(code uint32) string {
	if int(code) < len(grpcStatusNames) {
		return grpcStatusNames[code]
	}
	return strconv.FormatUint(uint64(code), 10)
}

// StatusReason 写 reason 参数里的 <状态名>: <说明>（invocation.md §3）：状态名同 GRPCStatusName，说明原样取状态的消息，可以为空。
func StatusReason(code uint32, message string) string {
	return GRPCStatusName(code) + ": " + message
}

// CallbackPermission 返回回调 RPC 所需的权限（invocation.md §5）：Host.Invoke 为 capability:kernel.pipeline，
// Host.Publish 为 capability:messaging.queue；rpc 不是这两种时为空。
func CallbackPermission(rpc string) string {
	switch rpc {
	case CallbackInvoke:
		return "capability:kernel.pipeline"
	case CallbackPublish:
		return "capability:messaging.queue"
	default:
		return ""
	}
}

// CallbackFailure 是回调失败映射出的错误：错误码、消息键与 details。
type CallbackFailure struct {
	// Code 是错误码。
	Code string
	// Key 是消息键。
	Key string
	// Details 是错误的 details，也就是消息模板的参数（invocation.md §8，不另加成员）；nil 表示没有。
	Details map[string]string
}

// MapCallbackStatus 按 invocation.md §5 的表把插件一侧回调的 gRPC 失败映射成错误：rpc 是 CallbackInvoke 或 CallbackPublish，
// code 与 message 是 gRPC 状态码的数值与消息（连接失败按 Unavailable 传入），contextCancelled 表示回调所在的调用上下文的
// 取消信号是否已触发，pluginID 是插件的清单 ID，operation 是 Host.Invoke 的操作 ID（Host.Publish 时不用）。
// Host.Publish 的 RPC 失败时每一项都是这个错误。
func MapCallbackStatus(rpc string, code uint32, message string, contextCancelled bool, pluginID, operation string) CallbackFailure {
	switch {
	case code == grpcUnauthenticated:
		return CallbackFailure{Code: "PLUGINS_DENIED_002", Key: "plugins.callback.session_invalid"}
	case code == grpcPermissionDenied:
		return CallbackFailure{Code: "PLUGINS_DENIED_002", Key: "plugins.callback.denied", Details: map[string]string{"id": pluginID, "permission": CallbackPermission(rpc)}}
	case code == grpcDeadlineExceeded:
		return CallbackFailure{Code: "KERNEL_TIMEOUT_001", Key: "kernel.invocation.deadline_exceeded"}
	// Cancelled 只在调用方确实取消时算取消：超时的归因交给插件自己管道里的截止时间步骤（RFC-0003），
	// 调用方没有取消的 Cancelled 是连接被宿主或网络中断，与 Unavailable 同样处理
	case code == grpcCancelled && contextCancelled:
		return CallbackFailure{Code: "KERNEL_CANCELLED_001", Key: "kernel.invocation.cancelled"}
	case code == grpcCancelled, code == grpcUnavailable:
		return CallbackFailure{Code: "PLUGINS_UNAVAILABLE_001", Key: "plugins.host.unavailable", Details: map[string]string{"id": pluginID, "reason": StatusReason(code, message)}}
	// Unimplemented 与 InvalidArgument 只对 Host.Invoke 有操作层面的含义；Host.Publish 的这两种状态落到最后一项
	case code == grpcUnimplemented && rpc == CallbackInvoke:
		return CallbackFailure{Code: "KERNEL_NOTFOUND_001", Key: "kernel.operation.unknown", Details: map[string]string{"id": operation}}
	case code == grpcInvalidArgument && rpc == CallbackInvoke:
		return CallbackFailure{Code: "KERNEL_VALIDATION_001", Key: "kernel.operation.invalid_payload", Details: map[string]string{"id": operation, "reason": message}}
	default:
		phase := operation
		if rpc == CallbackPublish {
			phase = "publish"
		}
		return CallbackFailure{Code: "PLUGINS_UNEXPECTED_001", Key: "plugins.process.failed", Details: map[string]string{"id": pluginID, "phase": phase, "reason": StatusReason(code, message)}}
	}
}
