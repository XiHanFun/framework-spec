package rules

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// WriteErrorText 把以字节存放的 Error 文案里不是合法 UTF-8 的部分换成 U+FFFD（invocation.md §3「写出 Error 的文案」，ADR-0339）：
// 按 Unicode 标准 §3.9 的最大子部分，从不能解码的位置起，能成为某个合法 UTF-8 序列开头的最长前缀是一段，不能的一个字节是一段，
// 每一段换一个 U+FFFD；合法的字符（含本来就是 U+FFFD 的）照抄。结果与 .NET 的 Encoding.UTF8.GetString 相同（向量 error-text-writing.json 的 bytes）。
func WriteErrorText(text []byte) string {
	if utf8.Valid(text) {
		return string(text)
	}
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		// 解码出 RuneError 且只占一个字节才是不能解码；占三个字节的是本来就写着的 U+FFFD
		if r != utf8.RuneError || size > 1 {
			b.Write(text[i : i+size])
			i += size
			continue
		}
		b.WriteRune(utf8.RuneError)
		i += maximalSubpart(text[i:])
	}
	return b.String()
}

// maximalSubpart 返回 text 开头那一段不能解码的字节的长度：首字节不能开头时是 1，否则是之后还接得上的字节数加 1。
// 序列的长度与第二个字节的范围按 Unicode 标准表 3-7：E0、ED、F0、F4 的第二个字节比 80–BF 窄，其余的延续字节都是 80–BF。
func maximalSubpart(text []byte) int {
	size, low, high := 0, byte(0x80), byte(0xBF)
	switch lead := text[0]; {
	case lead >= 0xC2 && lead <= 0xDF:
		size = 2
	case lead == 0xE0:
		size, low = 3, 0xA0
	case lead == 0xED:
		size, high = 3, 0x9F
	case lead >= 0xE1 && lead <= 0xEF:
		size = 3
	case lead == 0xF0:
		size, low = 4, 0x90
	case lead >= 0xF1 && lead <= 0xF3:
		size = 4
	case lead == 0xF4:
		size, high = 4, 0x8F
	default:
		// 80–C1、F5–FF 不能开头
		return 1
	}
	// 调用方已经确认这里解不出一个完整的字符，所以接上的字节总比 size 少：接不上或到了末尾即停，已接上的前缀是一段
	taken := 1
	for taken < size && taken < len(text) {
		next := text[taken]
		if taken == 1 && (next < low || next > high) || taken > 1 && (next < 0x80 || next > 0xBF) {
			break
		}
		taken++
	}
	return taken
}

// WriteErrorTextUTF16 把以 UTF-16 存放的 Error 文案编成 UTF-8（invocation.md §3「写出 Error 的文案」，ADR-0339）：每个孤立的代理项
// （RFC-0001「孤立的代理项」）换一个 U+FFFD，高代理项紧跟低代理项照常是平面以外的一个字符。与 .NET 的 Encoding.UTF8、
// JavaScript 的 String.prototype.toWellFormed 相同（向量 error-text-writing.json 的 utf16）。
func WriteErrorTextUTF16(units []uint16) string {
	// utf16.Decode 正好把每个不成对的代理项换成一个 U+FFFD
	return string(utf16.Decode(units))
}

// WriteErrorTextCodePoints 把 Python 的 str 那样以码点存放的 Error 文案编成 UTF-8（invocation.md §3「写出 Error 的文案」，ADR-0339）：
// 每个代理项码点（U+D800–U+DFFF）换一个 U+FFFD，成对出现的两个也各换一个（向量 error-text-writing.json 的 codePoints）。
func WriteErrorTextCodePoints(points []rune) string {
	var b strings.Builder
	for _, point := range points {
		if utf16.IsSurrogate(point) {
			b.WriteRune(utf8.RuneError)
			continue
		}
		b.WriteRune(point)
	}
	return b.String()
}
