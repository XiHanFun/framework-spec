package rules

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var secretNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)

// MaxSecretNameLength 是密钥名的最大长度（RFC-0014「密钥」）；文法只含 ASCII，字符数就是字节数。
const MaxSecretNameLength = 256

// IsSecretName 判断密钥名是否合文法（RFC-0014「密钥」）：字母或数字开头，其余是字母、数字、.、_、/、-，
// 最长 256 个字符；不含 ..、//，不以 / 结尾。
func IsSecretName(name string) bool {
	return secretNamePattern.MatchString(name) && !strings.Contains(name, "..") && !strings.Contains(name, "//") && !strings.HasSuffix(name, "/")
}

// ConfigurationContentVersion 给出 Nacos 远程配置文档的版本（RFC-0014「Nacos Provider」）：内容按 UTF-8 编码的 MD5，小写十六进制。
// 这是 Nacos 监听接口的约定，只用来与服务端比对内容是否变化，不用于任何安全目的。
func ConfigurationContentVersion(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

// MaxJSONConfigurationDepth 是 JSON 配置文档里对象与数组最多嵌套的层数（RFC-0014「Nacos Provider」的「文档格式」）。
const MaxJSONConfigurationDepth = 64

// configurationValues 收集展平后的键值：键不能为空、不区分大小写，同一个键出现两次即文档不合法。
type configurationValues struct {
	values map[string]*string
	// seen 记已经出现过的键的大写形式。
	seen map[string]bool
}

// add 记下一个键值；键为空、或（不区分大小写）已经出现过返回假。
func (v *configurationValues) add(key string, value *string) bool {
	folded := strings.ToUpper(key)
	if key == "" || v.seen[folded] {
		return false
	}
	v.seen[folded] = true
	v.values[key] = value
	return true
}

// newConfigurationValues 给出一份空的键值。
func newConfigurationValues() *configurationValues {
	return &configurationValues{values: map[string]*string{}, seen: map[string]bool{}}
}

// FlattenJSONConfiguration 把 JSON 文档展平成配置的键值（RFC-0014「Nacos Provider」的 Json 格式）：对象按 : 展平，
// 数组按下标，字符串取值，数字取原文，布尔写成 True / False，null 保留为空值（nil），空对象与空数组不产生键；
// 照收 // 与 /* */ 注释和尾逗号，嵌套最多 64 层。文档不合法、顶层不是对象、展平后的键为空或（不区分大小写）重复返回假
// （CONFIGURATION_VALIDATION_001）。
func FlattenJSONConfiguration(content string) (map[string]*string, bool) {
	reader := &jsonConfigurationReader{text: content, values: newConfigurationValues()}
	if !reader.skipInsignificant() || reader.peek() != '{' || !reader.readValue("", 1) {
		return nil, false
	}
	// 根值之后只能是空白与注释
	if !reader.skipInsignificant() || reader.position != len(reader.text) {
		return nil, false
	}
	return reader.values.values, true
}

// jsonConfigurationReader 是照收注释与尾逗号、其余按 JSON 文法读的读取器，边读边把叶子记进 values。
type jsonConfigurationReader struct {
	text     string
	position int
	values   *configurationValues
}

// peek 给出当前位置的字节，读完了返回 0。
func (r *jsonConfigurationReader) peek() byte {
	if r.position >= len(r.text) {
		return 0
	}
	return r.text[r.position]
}

// skipInsignificant 跳过空白与注释；块注释没有结束返回假。
func (r *jsonConfigurationReader) skipInsignificant() bool {
	for r.position < len(r.text) {
		switch char := r.text[r.position]; {
		case char == ' ' || char == '\t' || char == '\n' || char == '\r':
			r.position++
		case strings.HasPrefix(r.text[r.position:], "//"):
			end := strings.IndexAny(r.text[r.position:], "\r\n")
			if end < 0 {
				r.position = len(r.text)
			} else {
				r.position += end
			}
		case strings.HasPrefix(r.text[r.position:], "/*"):
			end := strings.Index(r.text[r.position+2:], "*/")
			if end < 0 {
				return false
			}
			r.position += end + 4
		default:
			return true
		}
	}
	return true
}

// readValue 读一个值并记到 prefix 之下；depth 是这个值若是容器时所在的层数。
func (r *jsonConfigurationReader) readValue(prefix string, depth int) bool {
	switch char := r.peek(); {
	case char == '{':
		return depth <= MaxJSONConfigurationDepth && r.readObject(prefix, depth)
	case char == '[':
		return depth <= MaxJSONConfigurationDepth && r.readArray(prefix, depth)
	case char == '"':
		text, ok := r.readString()
		return ok && r.values.add(prefix, &text)
	case char == '-' || (char >= '0' && char <= '9'):
		text, ok := r.readNumber()
		return ok && r.values.add(prefix, &text)
	case r.readLiteral("true"):
		text := "True"
		return r.values.add(prefix, &text)
	case r.readLiteral("false"):
		text := "False"
		return r.values.add(prefix, &text)
	case r.readLiteral("null"):
		return r.values.add(prefix, nil)
	default:
		return false
	}
}

// readLiteral 读一个字面量（true、false、null）；不是它返回假、位置不动。
func (r *jsonConfigurationReader) readLiteral(literal string) bool {
	if !strings.HasPrefix(r.text[r.position:], literal) {
		return false
	}
	r.position += len(literal)
	return true
}

// readObject 读一个对象：成员名接在 prefix 之后（以 : 连接，成员名原样、不转义）；最后一个成员之后可以有一个逗号。
func (r *jsonConfigurationReader) readObject(prefix string, depth int) bool {
	r.position++
	for first := true; ; first = false {
		if !r.skipInsignificant() {
			return false
		}
		if r.peek() == '}' {
			r.position++
			return true
		}
		if !first {
			// 成员之间要有逗号；逗号之后可以直接结束（尾逗号）
			if r.peek() != ',' {
				return false
			}
			r.position++
			if !r.skipInsignificant() {
				return false
			}
			if r.peek() == '}' {
				r.position++
				return true
			}
		}
		if r.peek() != '"' {
			return false
		}
		name, ok := r.readString()
		if !ok || !r.skipInsignificant() || r.peek() != ':' {
			return false
		}
		r.position++
		if !r.skipInsignificant() {
			return false
		}
		child := name
		if prefix != "" {
			child = prefix + ":" + name
		}
		if !r.readValue(child, depth+1) {
			return false
		}
	}
}

// readArray 读一个数组：元素的下标接在 prefix 之后；最后一个元素之后可以有一个逗号。
func (r *jsonConfigurationReader) readArray(prefix string, depth int) bool {
	r.position++
	for index := 0; ; index++ {
		if !r.skipInsignificant() {
			return false
		}
		if r.peek() == ']' {
			r.position++
			return true
		}
		if index > 0 {
			if r.peek() != ',' {
				return false
			}
			r.position++
			if !r.skipInsignificant() {
				return false
			}
			if r.peek() == ']' {
				r.position++
				return true
			}
		}
		if !r.readValue(prefix+":"+strconv.Itoa(index), depth+1) {
			return false
		}
	}
}

// readNumber 读一个数字并给出它的原文：-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?。
func (r *jsonConfigurationReader) readNumber() (string, bool) {
	start := r.position
	digits := func() int {
		from := r.position
		for r.peek() >= '0' && r.peek() <= '9' {
			r.position++
		}
		return r.position - from
	}
	if r.peek() == '-' {
		r.position++
	}
	// 整数部分：单独的 0，或不以 0 开头的若干位
	if r.peek() == '0' {
		r.position++
	} else if digits() == 0 {
		return "", false
	}
	if r.peek() == '.' {
		r.position++
		if digits() == 0 {
			return "", false
		}
	}
	if r.peek() == 'e' || r.peek() == 'E' {
		r.position++
		if r.peek() == '+' || r.peek() == '-' {
			r.position++
		}
		if digits() == 0 {
			return "", false
		}
	}
	return r.text[start:r.position], true
}

// readString 读一个字符串并还原转义；没有转义的控制字符、不认识的转义与落单的代理项转义都不合法。
func (r *jsonConfigurationReader) readString() (string, bool) {
	r.position++
	var builder strings.Builder
	for r.position < len(r.text) {
		char := r.text[r.position]
		switch {
		case char == '"':
			r.position++
			return builder.String(), true
		case char < 0x20:
			return "", false
		case char != '\\':
			_, size := utf8.DecodeRuneInString(r.text[r.position:])
			builder.WriteString(r.text[r.position : r.position+size])
			r.position += size
		default:
			if !r.readEscape(&builder) {
				return "", false
			}
		}
	}
	return "", false
}

// readEscape 读一个反斜杠转义并把它表示的字符写进 builder。
func (r *jsonConfigurationReader) readEscape(builder *strings.Builder) bool {
	if r.position+1 >= len(r.text) {
		return false
	}
	escaped := r.text[r.position+1]
	r.position += 2
	switch escaped {
	case '"', '\\', '/':
		builder.WriteByte(escaped)
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
		unit, ok := r.readHexUnit()
		if !ok {
			return false
		}
		if !utf16.IsSurrogate(unit) {
			builder.WriteRune(unit)
			return true
		}
		// 代理项必须成对：高位之后紧跟一个 \u 写的低位
		if unit >= 0xDC00 || !strings.HasPrefix(r.text[r.position:], `\u`) {
			return false
		}
		r.position += 2
		low, ok := r.readHexUnit()
		if !ok || low < 0xDC00 || low > 0xDFFF {
			return false
		}
		builder.WriteRune(utf16.DecodeRune(unit, low))
	default:
		return false
	}
	return true
}

// readHexUnit 读 \u 之后的四位十六进制数字，给出一个 UTF-16 码元。
func (r *jsonConfigurationReader) readHexUnit() (rune, bool) {
	if r.position+4 > len(r.text) {
		return 0, false
	}
	var unit rune
	for _, char := range []byte(r.text[r.position : r.position+4]) {
		switch {
		case char >= '0' && char <= '9':
			unit = unit<<4 | rune(char-'0')
		case char >= 'a' && char <= 'f':
			unit = unit<<4 | rune(char-'a'+10)
		case char >= 'A' && char <= 'F':
			unit = unit<<4 | rune(char-'A'+10)
		default:
			return 0, false
		}
	}
	r.position += 4
	return unit, true
}

// FlattenPropertiesConfiguration 把 properties 文档展平成配置的键值（RFC-0014「Nacos Provider」的 Properties 格式）：
// 按 LF 分行，行、键、值两侧的空白去掉；空行与 # / ! 起头的行跳过；第一个 = 或 : 分开键与值，没有分隔符的行值为空串；
// 键里的 . 换成 :；不认转义与续行。有一行的键为空、展平后的键（不区分大小写）重复返回假（CONFIGURATION_VALIDATION_001）。
func FlattenPropertiesConfiguration(content string) (map[string]*string, bool) {
	values := newConfigurationValues()
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimFunc(line, IsWhitespace)
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		key, value := line, ""
		if separator := strings.IndexAny(line, "=:"); separator >= 0 {
			key, value = line[:separator], strings.TrimFunc(line[separator+1:], IsWhitespace)
		}
		key = strings.TrimFunc(key, IsWhitespace)
		if key == "" || !values.add(strings.ReplaceAll(key, ".", ":"), &value) {
			return nil, false
		}
	}
	return values.values, true
}
