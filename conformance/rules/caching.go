package rules

import (
	"unicode"
	"unicode/utf8"
)

// MaxCacheKeyBytes 是缓存键与键前缀按 UTF-8 编码后的最大字节数（RFC-0006「存储契约」，ADR-0276）。
const MaxCacheKeyBytes = 512

// IsCacheKey 判断缓存键是否合法（RFC-0006「存储契约」，provider/caching-distributed.md §2）：
// 非空、不含空白、控制字符与孤立的代理项、按 UTF-8 编码后不超过 MaxCacheKeyBytes 字节。
func IsCacheKey(key string) bool {
	return key != "" && IsCacheKeyPrefix(key)
}

// IsCacheKeyPrefix 判断缓存键前缀是否合法（RFC-0006「键管理」）：与键同一条规则，只是可以为空；
// 空前缀在列出键时表示全部，按前缀删除不接受，那一条由操作自己判断。
func IsCacheKeyPrefix(prefix string) bool {
	// Go 的字符串就是 UTF-8 字节序列，len 给出的正是规范要的字节数；按 UTF-16 码元计（.NET 的 string.Length、
	// Java 与 JavaScript 的 length）会把含非 ASCII 字符的超长键放过去。
	// 孤立的代理项在 Go 里只能以编成 UTF-8 的代理项或别的非法字节出现，range 把它们读成 utf8.RuneError 照收，
	// 所以先整串核对 UTF-8（ADR-0316）：宽松编码把它们换成 U+FFFD，两个不同的键会撞成同一个条目
	if len(prefix) > MaxCacheKeyBytes || !utf8.ValidString(prefix) {
		return false
	}
	for _, char := range prefix {
		// 空白按 RFC-0001「空白字符」的 25 个码位，不用 unicode.IsSpace；控制字符是 Unicode 的 Cc 类，
		// 与 .NET 的 char.IsControl 相同（U+0000–U+001F、U+007F–U+009F）
		if IsWhitespace(char) || unicode.IsControl(char) {
			return false
		}
	}
	return true
}
