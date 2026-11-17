package rules

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
)

// 读入 Status 的两种读者（invocation.md §3）。
const (
	// ReaderHost 是宿主读插件写来的 Status。
	ReaderHost = "host"
	// ReaderPlugin 是插件读宿主写来的 Status（回调的结果）。
	ReaderPlugin = "plugin"
)

// ProtoError 是线上 Error 的字段（plugin.proto 的 xihan.plugin.v1.Error）：details 是 JSON 文本，可以为空。
type ProtoError struct {
	Code       string
	MessageKey string
	Text       string
	Details    string
	Inner      *ProtoError
}

// DecodedError 是读入后的错误（invocation.md §3）。
type DecodedError struct {
	// Code 是错误码。
	Code string
	// Key 是消息键；为空表示自由文本错误。
	Key string
	// Text 是自由文本错误的文案；带键的错误为空，由读者按目录翻译。
	Text string
	// Details 是细节：值按 RFC-0010 的值模型，64 位范围内整数写法的数为 int64、其余的数为 float64，
	// 另有 string、bool、nil、[]any 与 map[string]any；nil 表示没有细节。
	Details map[string]any
	// Inner 是内部错误；nil 表示没有。
	Inner *DecodedError
}

// ReadProtoError 按 invocation.md §3 读入对方写来的 Error（宿主读插件、插件读宿主同一规则），pluginID 是插件的清单 ID：
//   - code 不合错误码文法时整个错误换成 PLUGINS_UNEXPECTED_001 + plugins.process.failed（phase 一律为 invoke，
//     reason 为 invalid error code '<code>'），原来的 inner 丢掉；
//   - message_key 合消息键文法的按键处理、不取 text，否则成为自由文本错误，文案取 text，text 为空时取 code；
//   - details 按 ReadDetails 读，inner 按同一规则读。
func ReadProtoError(proto ProtoError, pluginID string) DecodedError {
	if !IsErrorCode(proto.Code) {
		return DecodedError{
			Code:    "PLUGINS_UNEXPECTED_001",
			Key:     "plugins.process.failed",
			Details: map[string]any{"id": pluginID, "phase": "invoke", "reason": "invalid error code '" + proto.Code + "'"},
		}
	}
	decoded := DecodedError{Code: proto.Code, Details: ReadDetails(proto.Details)}
	if proto.Inner != nil {
		inner := ReadProtoError(*proto.Inner, pluginID)
		decoded.Inner = &inner
	}
	if IsMessageKey(proto.MessageKey) {
		decoded.Key = proto.MessageKey
		return decoded
	}
	decoded.Text = proto.Text
	if decoded.Text == "" {
		decoded.Text = proto.Code
	}
	return decoded
}

// ReadProtoStatus 按 invocation.md §3 读入对方写来的 Status：ok 为真时返回 nil；有 failure 时同 ReadProtoError；
// ok 为假却没有 failure 时是 PLUGINS_UNEXPECTED_001 + plugins.process.failed，reason 随 reader 而定（ReaderHost 为
// the plugin reported a failure without an error，ReaderPlugin 为 the host reported a failure without an error）。
// phase 是这次的阶段：宿主读插件时是操作 ID、event、configure、start、stop，插件读宿主时见 CallbackPhase。
func ReadProtoStatus(ok bool, failure *ProtoError, reader, phase, pluginID string) *DecodedError {
	if ok {
		return nil
	}
	if failure != nil {
		decoded := ReadProtoError(*failure, pluginID)
		return &decoded
	}
	reason := "the plugin reported a failure without an error"
	if reader == ReaderPlugin {
		reason = "the host reported a failure without an error"
	}
	return &DecodedError{
		Code:    "PLUGINS_UNEXPECTED_001",
		Key:     "plugins.process.failed",
		Details: map[string]any{"id": pluginID, "phase": phase, "reason": reason},
	}
}

// CallbackPhase 返回插件一侧回调失败时 plugins.process.failed 的 phase（invocation.md §3、§5）：
// Host.Invoke 是操作 ID，Host.Publish 是 publish。
func CallbackPhase(rpc, operation string) string {
	if rpc == CallbackPublish {
		return "publish"
	}
	return operation
}

// ReadDetails 按 invocation.md §3 读 Error.details 的 JSON 文本：为空、只有空白、通不过 RFC-0001 的共同 JSON 读取检查、
// 不是对象，或任何一层有读成双精度会溢出（±∞）的数时返回 nil，即没有细节；下溢的数读成 0，超出 64 位的整数读成双精度。
func ReadDetails(text string) map[string]any {
	// 只有空白的文本在共同读取检查里是语法错误
	if text == "" || CheckJSONText([]byte(text)) != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(text)))
	// 先保留数字原文，再按值模型逐个换算，才能分出整数、双精度与溢出
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil
	}
	members, isObject := root.(map[string]any)
	if !isObject {
		return nil
	}
	value, ok := detailValue(members)
	if !ok {
		return nil
	}
	return value.(map[string]any)
}

// detailValue 把解码出的值换成值模型：数字换成 int64 或 float64，数组与对象逐项换算；有溢出的数时返回假。
func detailValue(value any) (any, bool) {
	switch typed := value.(type) {
	case json.Number:
		text := typed.String()
		// 64 位范围内整数写法的数是整数，与 .NET 的 JsonElement.TryGetInt64 相同：带小数点或指数的写法不算
		if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
			return integer, true
		}
		number, err := strconv.ParseFloat(text, 64)
		// 文本已过共同读取检查，ParseFloat 只可能报超出范围：溢出得到 ±∞，下溢得到 0 且不报错
		if err != nil || math.IsInf(number, 0) {
			return nil, false
		}
		return number, true
	case []any:
		items := make([]any, len(typed))
		for index, item := range typed {
			converted, ok := detailValue(item)
			if !ok {
				return nil, false
			}
			items[index] = converted
		}
		return items, true
	case map[string]any:
		members := make(map[string]any, len(typed))
		for name, item := range typed {
			converted, ok := detailValue(item)
			if !ok {
				return nil, false
			}
			members[name] = converted
		}
		return members, true
	default:
		return value, true
	}
}
