package rules

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// resourceNamePattern 是资源名去掉 ID 段之后的文法：分区、服务、租户（可为空）与类型；ID 段另判。
var resourceNamePattern = regexp.MustCompile(`^xrn:[a-z0-9-]+:[a-z0-9-]+:(?:[a-z0-9][a-z0-9._-]{0,63})?:[a-z0-9-]+/`)

// IsResourceName 判断文本是不是统一资源名 xrn:<partition>:<service>:<tenant>:<type>/<id>（RFC-0008「资源名」）：
// 分区、服务与类型段是 [a-z0-9-]+，租户段为空或租户 ID，ID 段非空、不含 /、空白（IsWhitespace）与孤立的代理项，
// 以字节表示字符串时即必须是合法的 UTF-8（RFC-0001「孤立的代理项」，ADR-0321）。整串匹配。
func IsResourceName(text string) bool {
	// range 把非法的字节读成 U+FFFD 照收：编成 UTF-8 的代理项就是这样混进来的，先整串核对
	if !utf8.ValidString(text) {
		return false
	}
	prefix := resourceNamePattern.FindString(text)
	if prefix == "" {
		return false
	}
	// 前缀的四段都不含 /，所以匹配到的 / 就是类型与 ID 之间的那一个；RE2 的 \s 只认 ASCII 空白，ID 段逐个字符判
	id := text[len(prefix):]
	return id != "" && !strings.ContainsFunc(id, func(r rune) bool { return r == '/' || IsWhitespace(r) })
}
