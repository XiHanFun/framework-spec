package rules

import "regexp"

// MaxMessageKeyLength 是消息键的最大长度（RFC-0010「消息键」，与 .NET 的 MessageTemplate.MaxKeyLength 相同）。
const MaxMessageKeyLength = 128

// MaxBaggageValueLength 是已登记 Baggage 键的值按 UTF-16 码元计的最大长度（conventions/baggage-keys.md）。
const MaxBaggageValueLength = 256

var (
	messageKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	placeholderNamePattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*|[0-9]+)$`)
)

// IsMessageKey 判断消息键是否符合文法且不超过 MaxMessageKeyLength（RFC-0010）。
func IsMessageKey(key string) bool {
	return len(key) <= MaxMessageKeyLength && messageKeyPattern.MatchString(key)
}

// IsBaggageValueAcceptable 判断已登记键的值能否进入调用上下文：非空、按 UTF-16 码元计不超过
// MaxBaggageValueLength、不含 U+0000–U+001F 与 U+007F–U+009F（ADR-0204、ADR-0233）。
func IsBaggageValueAcceptable(value string) bool {
	if value == "" {
		return false
	}
	units := 0
	for _, r := range value {
		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) {
			return false
		}
		// 基本多文种平面以外的字符在 UTF-16 里是一对代理项，按两个码元计，与 .NET、JavaScript、Java 的长度一致
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
	}
	return units <= MaxBaggageValueLength
}

// Placeholders 按出现顺序列出模板里的占位符名（去重、不含格式说明符），规则同 RFC-0010「模板」：
// {{ 与 }} 是转义，花括号里不是合法名字的与没有闭合的 { 都是字面量。
func Placeholders(template string) []string {
	var names []string
	seen := map[string]bool{}
	for index := 0; index < len(template); {
		c := template[index]
		if (c == '{' || c == '}') && index+1 < len(template) && template[index+1] == c {
			index += 2
			continue
		}
		if c != '{' {
			index++
			continue
		}
		end := indexFrom(template, '}', index+1)
		if end < 0 {
			// 没有闭合的左花括号连同其后的文本都是字面量
			break
		}
		body := template[index+1 : end]
		name := body
		if separator := indexFrom(body, ':', 0); separator >= 0 {
			name = body[:separator]
		}
		if placeholderNamePattern.MatchString(name) && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
		index = end + 1
	}
	return names
}

// indexFrom 返回 c 在 text[from:] 里第一次出现的位置（相对 text），没有时为 -1。
func indexFrom(text string, c byte, from int) int {
	for i := from; i < len(text); i++ {
		if text[i] == c {
			return i
		}
	}
	return -1
}

// traceParentPattern 是 W3C traceparent 版本 00 的文法（RFC-0003 规则 7）。
var traceParentPattern = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)

// TraceParent 是解析出的 traceparent：trace-id、parent-id 与采样位。
type TraceParent struct {
	TraceID string
	SpanID  string
	Sampled bool
}

// ParseTraceParent 按 RFC-0003 规则 7 解析 traceparent：只认版本 00 与小写十六进制，
// trace-id 或 parent-id 全零不合法；是否采样只看标志的最低位。
func ParseTraceParent(text string) (TraceParent, bool) {
	match := traceParentPattern.FindStringSubmatch(text)
	if match == nil || isZeros(match[1]) || isZeros(match[2]) {
		return TraceParent{}, false
	}
	flags := match[3]
	// 最低位在第二个十六进制数字里：奇数即采样
	last := flags[1]
	sampled := (last >= '0' && last <= '9' && (last-'0')%2 == 1) || (last >= 'a' && last <= 'f' && (last-'a'+10)%2 == 1)
	return TraceParent{TraceID: match[1], SpanID: match[2], Sampled: sampled}, true
}

// FormatTraceParent 写出 00-<trace-id>-<parent-id>-<00|01>。
func FormatTraceParent(value TraceParent) string {
	flags := "00"
	if value.Sampled {
		flags = "01"
	}
	return "00-" + value.TraceID + "-" + value.SpanID + "-" + flags
}

// isZeros 判断文本是否全为字符 0。
func isZeros(text string) bool {
	for _, c := range text {
		if c != '0' {
			return false
		}
	}
	return true
}
