package rules

import (
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxJSONDepth 是共同读取检查允许的最大嵌套层数（RFC-0001「JSON 文本」）。
const MaxJSONDepth = 64

// JSONTextProblem 是 JSON 文本没有通过共同读取检查的原因。
type JSONTextProblem struct {
	// Syntax 为真表示不是 JSON（RFC 8259 之外的写法、BOM、嵌套过深、非法 UTF-8、孤立的代理项转义）；否则是重复成员。
	Syntax bool
	// Path 在语法错误时为 $，重复成员时为第一处重复（第二次出现的成员）的路径。
	Path string
}

// CheckJSONText 按 RFC-0001「JSON 文本」检查 UTF-8 文本，通过时返回 nil（ADR-0236）。
// 语法错误不论出现在哪里都优先于重复成员，所以遇到重复成员也读到末尾；成员名按转义还原后比较。
func CheckJSONText(data []byte) *JSONTextProblem {
	reader := jsonReader{data: data}
	reader.whitespace()
	if !reader.value(0, "$") {
		return &JSONTextProblem{Syntax: true, Path: "$"}
	}
	reader.whitespace()
	if reader.at != len(data) {
		return &JSONTextProblem{Syntax: true, Path: "$"}
	}
	if reader.duplicate != "" {
		return &JSONTextProblem{Path: reader.duplicate}
	}
	return nil
}

// MemberPath 按 RFC-0011 规则 2 写成员的路径：名字含特殊字符时写 ['名字']，名字原样不转义。
func MemberPath(parent, name string) string {
	if strings.ContainsFunc(name, isBracketed) {
		return parent + "['" + name + "']"
	}
	return parent + "." + name
}

// IndexPath 写数组项的路径。
func IndexPath(parent string, index int) string {
	return parent + "[" + strconv.Itoa(index) + "]"
}

// isBracketed 判断成员名里的字符是否要求写成方括号形式（照 System.Text.Json 的实测）。
func isBracketed(r rune) bool {
	switch r {
	case '.', ' ', '\'', '/', '"', '[', ']', '(', ')', '\\', '\t', '\n', '\r', '\f', '\b', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// jsonReader 是按 RFC 8259 逐字节读取的检查器。
type jsonReader struct {
	data      []byte
	at        int
	duplicate string
}

// whitespace 跳过 JSON 的四种空白。
func (r *jsonReader) whitespace() {
	for r.at < len(r.data) {
		switch r.data[r.at] {
		case ' ', '\t', '\n', '\r':
			r.at++
		default:
			return
		}
	}
}

// value 读一个值；depth 是外层容器的层数。
func (r *jsonReader) value(depth int, path string) bool {
	if r.at >= len(r.data) {
		return false
	}
	switch c := r.data[r.at]; {
	case c == '{':
		return r.object(depth+1, path)
	case c == '[':
		return r.array(depth+1, path)
	case c == '"':
		_, ok := r.text()
		return ok
	case c == '-' || (c >= '0' && c <= '9'):
		return r.number()
	default:
		return r.literal("true") || r.literal("false") || r.literal("null")
	}
}

// object 读一个对象；重复的成员名只记第一处。
func (r *jsonReader) object(depth int, path string) bool {
	if depth > MaxJSONDepth {
		return false
	}
	r.at++
	r.whitespace()
	if r.at < len(r.data) && r.data[r.at] == '}' {
		r.at++
		return true
	}
	names := map[string]bool{}
	for {
		if r.at >= len(r.data) || r.data[r.at] != '"' {
			return false
		}
		name, ok := r.text()
		if !ok {
			return false
		}
		member := MemberPath(path, name)
		if names[name] && r.duplicate == "" {
			r.duplicate = member
		}
		names[name] = true
		r.whitespace()
		if r.at >= len(r.data) || r.data[r.at] != ':' {
			return false
		}
		r.at++
		r.whitespace()
		if !r.value(depth, member) {
			return false
		}
		r.whitespace()
		if r.at >= len(r.data) {
			return false
		}
		switch r.data[r.at] {
		case ',':
			r.at++
			r.whitespace()
		case '}':
			r.at++
			return true
		default:
			return false
		}
	}
}

// array 读一个数组。
func (r *jsonReader) array(depth int, path string) bool {
	if depth > MaxJSONDepth {
		return false
	}
	r.at++
	r.whitespace()
	if r.at < len(r.data) && r.data[r.at] == ']' {
		r.at++
		return true
	}
	for index := 0; ; index++ {
		if !r.value(depth, IndexPath(path, index)) {
			return false
		}
		r.whitespace()
		if r.at >= len(r.data) {
			return false
		}
		switch r.data[r.at] {
		case ',':
			r.at++
			r.whitespace()
		case ']':
			r.at++
			return true
		default:
			return false
		}
	}
}

// text 读一个字符串并还原转义；非法 UTF-8、原始控制字符、非法转义与孤立的代理项都不合法。
func (r *jsonReader) text() (string, bool) {
	r.at++
	var builder strings.Builder
	for r.at < len(r.data) {
		c := r.data[r.at]
		switch {
		case c == '"':
			r.at++
			return builder.String(), true
		case c < 0x20:
			return "", false
		case c == '\\':
			if !r.escape(&builder) {
				return "", false
			}
		case c < 0x80:
			builder.WriteByte(c)
			r.at++
		default:
			// utf8.DecodeRune 对非法序列、过长编码与编码成 UTF-8 的代理项都返回长度 1 的 RuneError
			decoded, size := utf8.DecodeRune(r.data[r.at:])
			if decoded == utf8.RuneError && size <= 1 {
				return "", false
			}
			builder.WriteRune(decoded)
			r.at += size
		}
	}
	return "", false
}

// escape 读一个转义序列；高代理项后必须紧跟低代理项。
func (r *jsonReader) escape(builder *strings.Builder) bool {
	if r.at+1 >= len(r.data) {
		return false
	}
	switch r.data[r.at+1] {
	case '"', '\\', '/':
		builder.WriteByte(r.data[r.at+1])
	case 'b':
		builder.WriteByte('\b')
	case 'f':
		builder.WriteByte('\f')
	case 'n':
		builder.WriteByte('\n')
	case 'r':
		builder.WriteByte('\r')
	case 't':
		builder.WriteByte('\t')
	case 'u':
		unit, ok := r.hex4(r.at + 2)
		if !ok {
			return false
		}
		switch {
		case utf16.IsSurrogate(rune(unit)) && unit < 0xDC00:
			// 高代理项：下一个必须是低代理项的转义
			low, ok := r.hex4(r.at + 8)
			if !ok || r.data[r.at+6] != '\\' || r.data[r.at+7] != 'u' || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			builder.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
			r.at += 12
			return true
		case utf16.IsSurrogate(rune(unit)):
			return false
		default:
			builder.WriteRune(rune(unit))
		}
		r.at += 6
		return true
	default:
		return false
	}
	r.at += 2
	return true
}

// hex4 读 from 起的四位十六进制数。
func (r *jsonReader) hex4(from int) (uint16, bool) {
	if from+4 > len(r.data) {
		return 0, false
	}
	value, err := strconv.ParseUint(string(r.data[from:from+4]), 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(value), true
}

// number 读一个数：-? (0 | [1-9][0-9]*) (.[0-9]+)? ([eE][+-]?[0-9]+)?
func (r *jsonReader) number() bool {
	if r.data[r.at] == '-' {
		r.at++
	}
	if r.at >= len(r.data) {
		return false
	}
	if r.data[r.at] == '0' {
		r.at++
	} else if !r.digits() {
		return false
	}
	if r.at < len(r.data) && r.data[r.at] == '.' {
		r.at++
		if !r.digits() {
			return false
		}
	}
	if r.at < len(r.data) && (r.data[r.at] == 'e' || r.data[r.at] == 'E') {
		r.at++
		if r.at < len(r.data) && (r.data[r.at] == '+' || r.data[r.at] == '-') {
			r.at++
		}
		if !r.digits() {
			return false
		}
	}
	return true
}

// digits 读至少一位十进制数字。
func (r *jsonReader) digits() bool {
	start := r.at
	for r.at < len(r.data) && r.data[r.at] >= '0' && r.data[r.at] <= '9' {
		r.at++
	}
	return r.at > start
}

// literal 读一个字面量。
func (r *jsonReader) literal(word string) bool {
	if strings.HasPrefix(string(r.data[r.at:]), word) {
		r.at += len(word)
		return true
	}
	return false
}
