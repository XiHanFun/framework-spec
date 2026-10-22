// Package rules 实现规范里的文法与判定规则（错误码、操作 ID、主题、Baggage 键、能力依赖、
// 握手行、版本协商、生命周期迁移、管道阶段），供运行器与各语言 SDK 的作者对照；
// 每条规则都由 ../vectors 下的向量验证。
package rules

import (
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	errorCodePattern             = regexp.MustCompile(`^[A-Z][A-Z0-9]*_[A-Z][A-Z0-9]*_[0-9]{3}$`)
	operationIDPattern           = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*\.[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	topicPattern                 = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	baggageKeyPattern            = regexp.MustCompile(`^xihan\.[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	manifestIDPattern            = regexp.MustCompile(`^xihan\.[a-z][a-z0-9]*\.[a-z][a-z0-9-]*$`)
	capabilityIDPattern          = regexp.MustCompile(`^[a-z][a-z0-9]*\.[a-z][a-z0-9-]*$`)
	capabilityRequirementPattern = regexp.MustCompile(`^([a-z][a-z0-9]*\.[a-z][a-z0-9-]*)@(\^?)([1-9][0-9]*)(\??)$`)
	handshakeLinePattern         = regexp.MustCompile(`^xihan-plugin/1 (unix|tcp|npipe) (\S+)$`)
)

// RegisteredBaggageKeys 是 conventions/baggage-keys.md 登记的键。
var RegisteredBaggageKeys = []string{
	"xihan.tenant",
	"xihan.principal",
	"xihan.correlation",
	"xihan.idempotency-key",
	"xihan.locale",
}

// PipelineStages 是管道阶段的规范性名称与顺序（RFC-0004）。
var PipelineStages = []string{
	"ingress", "observe", "resilience", "authenticate", "tenant", "authorize",
	"validate", "idempotency", "transaction", "handle", "egress",
}

// LifecycleStates 是生命周期的九个状态，顺序与数值和原语层一致（RFC-0001）。
var LifecycleStates = []string{
	"Declared", "Resolved", "Configured", "Started", "Ready", "Degraded", "Draining", "Stopped", "Failed",
}

// lifecycleTransitions 是合法迁移的完整列表；未列出的组合一律非法。
var lifecycleTransitions = map[string][]string{
	"Declared":   {"Resolved", "Failed"},
	"Resolved":   {"Configured", "Failed"},
	"Configured": {"Started", "Failed"},
	"Started":    {"Ready", "Degraded", "Draining", "Failed"},
	"Ready":      {"Degraded", "Draining", "Failed"},
	"Degraded":   {"Ready", "Draining", "Failed"},
	"Draining":   {"Stopped", "Failed"},
	"Stopped":    {},
	"Failed":     {"Resolved"},
}

// IsErrorCode 判断错误码是否符合 <领域大写>_<KIND>_<NNN>（conventions/error-codes.md）。
func IsErrorCode(code string) bool { return errorCodePattern.MatchString(code) }

// ErrorCodeKind 返回错误码的 KIND 段；不合法时为空。
func ErrorCodeKind(code string) string {
	if !IsErrorCode(code) {
		return ""
	}
	parts := strings.Split(code, "_")
	return parts[1]
}

// IsOperationID 判断操作 ID 是否符合 <resource>.<verb>（conventions/operation-ids.md）。
func IsOperationID(id string) bool { return operationIDPattern.MatchString(id) }

// IsTopic 判断消息主题是否符合 RFC-0009 的文法。
func IsTopic(topic string) bool { return topicPattern.MatchString(topic) }

// IsBaggageKey 判断 Baggage 键是否符合 xihan.<name>。
func IsBaggageKey(key string) bool { return baggageKeyPattern.MatchString(key) }

// IsRegisteredBaggageKey 判断 Baggage 键是否已登记。
func IsRegisteredBaggageKey(key string) bool { return slices.Contains(RegisteredBaggageKeys, key) }

// IsManifestID 判断清单 ID 是否符合 xihan.<领域>.<名称>。
func IsManifestID(id string) bool { return manifestIDPattern.MatchString(id) }

// IsCapabilityID 判断能力 ID 是否符合 <领域>.<能力>。
func IsCapabilityID(id string) bool { return capabilityIDPattern.MatchString(id) }

// CapabilityRequirement 是一条解析后的能力依赖：<能力ID>@[^]<版本>[?]。
type CapabilityRequirement struct {
	ID       string
	Version  int
	Optional bool
}

// ParseCapabilityRequirement 解析能力依赖文本；^N 与 N 等价。
func ParseCapabilityRequirement(text string) (CapabilityRequirement, bool) {
	match := capabilityRequirementPattern.FindStringSubmatch(text)
	if match == nil {
		return CapabilityRequirement{}, false
	}
	version, err := strconv.Atoi(match[3])
	if err != nil {
		return CapabilityRequirement{}, false
	}
	return CapabilityRequirement{ID: match[1], Version: version, Optional: match[4] == "?"}, true
}

// Endpoint 是握手行里的插件端点。
type Endpoint struct {
	Network string
	Address string
}

// ParseHandshakeLine 解析握手行 xihan-plugin/1 <network> <address>；tcp 只接受回环地址。
func ParseHandshakeLine(line string) (Endpoint, bool) {
	match := handshakeLinePattern.FindStringSubmatch(line)
	if match == nil {
		return Endpoint{}, false
	}
	endpoint := Endpoint{Network: match[1], Address: match[2]}
	if endpoint.Network == "tcp" && !IsLoopbackAddress(endpoint.Address) {
		return Endpoint{}, false
	}
	return endpoint, true
}

// IsLoopbackAddress 判断 host:port 是否是回环地址 127.0.0.1 或 [::1]；主机名不算。
func IsLoopbackAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return false
	}
	if _, err := strconv.Atoi(port); err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1"
}

// NegotiateVersion 选宿主列表里最高的插件也支持的协议版本；没有交集返回假。
func NegotiateVersion(host, plugin []uint32) (uint32, bool) {
	var selected uint32
	found := false
	for _, candidate := range host {
		if slices.Contains(plugin, candidate) && (!found || candidate > selected) {
			selected = candidate
			found = true
		}
	}
	return selected, found
}

// CanTransition 判断生命周期迁移是否合法。
func CanTransition(from, to string) bool {
	return slices.Contains(lifecycleTransitions[from], to)
}

// IsServingState 判断状态是否可服务（Ready 或 Degraded）。
func IsServingState(state string) bool { return state == "Ready" || state == "Degraded" }
