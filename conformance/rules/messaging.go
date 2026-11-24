package rules

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"time"
)

var (
	envelopeExtensionNamePattern = regexp.MustCompile(`^[a-z0-9]{1,20}$`)
	messagingInstanceIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// MaxTopicLength 是消息主题的最大长度（RFC-0009「信封」）；文法只含 ASCII，字符数就是字节数。
const MaxTopicLength = 128

// DefaultDeadLetterSuffix 是死信主题后缀的缺省值（RFC-0009「信封」）。
const DefaultDeadLetterSuffix = ".dlq"

// MaxEnvelopeExtensionNameLength 是 CloudEvents 扩展属性名的最大长度（RFC-0009「信封」）。
const MaxEnvelopeExtensionNameLength = 20

// MaxMessagingInstanceIDLength 是消息子系统实例标识的最大长度（RFC-0009「订阅方式」第 6 条）。
const MaxMessagingInstanceIDLength = 64

// EnvelopeTraceExtension 是承载追踪的扩展属性，值是 W3C traceparent（RFC-0003 规则 7），不进 Baggage。
const EnvelopeTraceExtension = "traceparent"

// EnvelopePartitionKeyExtension 是分区键的扩展属性（RFC-0009「流」），不进 Baggage。
const EnvelopePartitionKeyExtension = "xihankey"

// CloudEventsSpecVersion 是信封的 CloudEvents 规范版本。
const CloudEventsSpecVersion = "1.0"

// CloudEventsContentType 是结构化 JSON 编码的内容类型。
const CloudEventsContentType = "application/cloudevents+json"

// CloudEventsDefaultDataContentType 是文档不写 datacontenttype 时的数据内容类型。
const CloudEventsDefaultDataContentType = "application/json"

// 发件箱三个选项的缺省值（RFC-0009「发件箱」）。
const (
	// DefaultOutboxRetryDelay 是第一次失败之后的重试间隔。
	DefaultOutboxRetryDelay = 5 * time.Second
	// DefaultOutboxMaxRetryDelay 是重试间隔的上限。
	DefaultOutboxMaxRetryDelay = 5 * time.Minute
	// DefaultOutboxMaxAttempts 是一条消息最多尝试投递的次数（含首次）。
	DefaultOutboxMaxAttempts = 10
)

// EnvelopeBaggageMapping 是一个扩展属性与它对应的 Baggage 键。
type EnvelopeBaggageMapping struct {
	Extension  string
	BaggageKey string
}

// EnvelopeBaggageMappings 是扩展属性与 Baggage 键的对应（conventions/baggage-keys.md 的 CloudEvents 载体）：
// 发布侧按它补齐、派发侧按它放进新调用的 Baggage；xihan.idempotency-key 不在其中（RFC-0009 投递第 4 条）。
var EnvelopeBaggageMappings = []EnvelopeBaggageMapping{
	{Extension: "xihantenant", BaggageKey: "xihan.tenant"},
	{Extension: "xihanprincipal", BaggageKey: "xihan.principal"},
	{Extension: "xihancorrelation", BaggageKey: "xihan.correlation"},
	{Extension: "xihanlocale", BaggageKey: "xihan.locale"},
}

// DeadLetterTopic 给出主题的死信主题：主题加后缀，后缀为空时用缺省值（RFC-0009「信封」）。
// 结果是代理上的名字，不再过主题文法。
func DeadLetterTopic(topic, suffix string) string {
	if suffix == "" {
		suffix = DefaultDeadLetterSuffix
	}
	return topic + suffix
}

// IsEnvelopeExtensionName 判断 CloudEvents 扩展属性名是否合文法（RFC-0009「信封」）：小写字母与数字，1 到 20 个字符。
func IsEnvelopeExtensionName(name string) bool { return envelopeExtensionNamePattern.MatchString(name) }

// IsMessagingInstanceID 判断消息子系统的实例标识是否合文法（RFC-0009「订阅方式」第 6 条）：
// 1 到 64 个字符，字母或数字开头，其余是字母、数字、.、_、-。
func IsMessagingInstanceID(id string) bool { return messagingInstanceIDPattern.MatchString(id) }

// OutboxRetryDelay 给出发件箱里一条消息第 attempts 次失败之后推后的间隔（RFC-0009「发件箱」）：
// retryDelay 乘以 2 的（attempts − 1）次方，不超过 maxRetryDelay。attempts 从 1 起。
func OutboxRetryDelay(retryDelay, maxRetryDelay time.Duration, attempts int) time.Duration {
	delay := retryDelay
	// 逐次翻倍并在到达上限时停下：次数再多也不会溢出成负数或零
	for i := 1; i < attempts && delay < maxRetryDelay; i++ {
		if delay > maxRetryDelay/2 {
			return maxRetryDelay
		}
		delay *= 2
	}
	return min(delay, maxRetryDelay)
}

// OutboxDeadLettered 判断发件箱里已经尝试 attempts 次的消息在这次失败之后是否进死信（RFC-0009「发件箱」）：
// 尝试次数达到 maxAttempts 即进死信，不再排下一次。
func OutboxDeadLettered(attempts, maxAttempts int) bool { return attempts >= maxAttempts }

// CloudEvent 是从结构化 JSON 文档读出的信封：Time 为 nil 表示没有时间，Subject 为 nil 表示没有主题标识；
// Data 是内联的 JSON 数据（原文），Binary 是 data_base64 解出的字节，两者都为空表示没有数据。
type CloudEvent struct {
	ID              string
	Source          string
	Type            string
	Time            *time.Time
	Subject         *string
	DataContentType string
	Data            json.RawMessage
	Binary          []byte
	Extensions      map[string]string
}

// DecodeCloudEvent 按 CloudEvents 1.0 结构化 JSON 读出信封（RFC-0009「信封」，protocol/cloudevents.md「入站」）；
// 规范版本不是 1.0、缺必填属性、时间不合 RFC 3339、扩展属性名不合文法或 data_base64 不是 Base64 时返回假。
func DecodeCloudEvent(document []byte) (CloudEvent, bool) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil || members == nil {
		return CloudEvent{}, false
	}

	// 规范属性都是字符串；必填的三个还不能为空
	text := func(name string) (*string, bool) {
		raw, ok := members[name]
		delete(members, name)
		if !ok || bytes.Equal(raw, []byte("null")) {
			return nil, true
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, false
		}
		return &value, true
	}
	required := func(name string) (string, bool) {
		value, ok := text(name)
		if !ok || value == nil || *value == "" {
			return "", false
		}
		return *value, true
	}

	version, ok := text("specversion")
	if !ok || version == nil || *version != CloudEventsSpecVersion {
		return CloudEvent{}, false
	}
	event := CloudEvent{DataContentType: CloudEventsDefaultDataContentType, Extensions: map[string]string{}}
	if event.ID, ok = required("id"); !ok {
		return CloudEvent{}, false
	}
	if event.Source, ok = required("source"); !ok {
		return CloudEvent{}, false
	}
	if event.Type, ok = required("type"); !ok {
		return CloudEvent{}, false
	}

	occurred, ok := text("time")
	if !ok {
		return CloudEvent{}, false
	}
	if occurred != nil {
		parsed, err := time.Parse(time.RFC3339Nano, *occurred)
		if err != nil {
			return CloudEvent{}, false
		}
		event.Time = &parsed
	}
	if event.Subject, ok = text("subject"); !ok {
		return CloudEvent{}, false
	}
	contentType, ok := text("datacontenttype")
	if !ok {
		return CloudEvent{}, false
	}
	if contentType != nil {
		event.DataContentType = *contentType
	}

	// JSON 数据内联在 data，其他数据在 data_base64
	if raw, has := members["data"]; has && !bytes.Equal(raw, []byte("null")) {
		event.Data = raw
	}
	delete(members, "data")
	encoded, ok := text("data_base64")
	if !ok {
		return CloudEvent{}, false
	}
	if encoded != nil && event.Data == nil {
		binary, err := base64.StdEncoding.DecodeString(*encoded)
		if err != nil {
			return CloudEvent{}, false
		}
		event.Binary = binary
	}

	// 余下的顶层成员都是扩展属性；名字不合文法整个信封不合法
	for name, raw := range members {
		if !IsEnvelopeExtensionName(name) {
			return CloudEvent{}, false
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			// 非字符串的扩展值规范不作规定，向量也不收；这里照参考实现：数字与布尔保留原文，其余读成空字符串
			value = ""
			if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] != '{' && trimmed[0] != '[' && !bytes.Equal(trimmed, []byte("null")) {
				value = string(trimmed)
			}
		}
		event.Extensions[name] = value
	}
	return event, true
}
