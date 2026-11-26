package conformance

import (
	"fmt"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// CodeUnits 是以四位十六进制写出的一串 UTF-16 码元。向量用它写出 JSON 字符串写不出的文本，即含孤立代理项的标识
// （RFC-0012「一致性」，ADR-0316）：转义出的孤立代理项各语言的 JSON 读取器读法不同，Go 的 encoding/json 把它换成 U+FFFD。
type CodeUnits []string

// Units 解析出码元；有一项不是四位十六进制时返回错误。
func (c CodeUnits) Units() ([]uint16, error) {
	units := make([]uint16, len(c))
	for i, unit := range c {
		value, err := strconv.ParseUint(unit, 16, 16)
		if len(unit) != 4 || err != nil {
			return nil, fmt.Errorf("conformance: code unit %q must be four hexadecimal digits", unit)
		}
		units[i] = uint16(value)
	}
	return units, nil
}

// Text 把码元组成 Go 的字符串：按 WTF-8 编成字节，成对的代理项合成平面以外的字符、编成 4 个字节，
// 孤立的代理项各照 UTF-8 三字节的写法编成 ED A0 80–ED BF BF。合法的文本得到的就是它的 UTF-8；
// 有孤立的代理项时得到的不是合法的 UTF-8，这就是以字节表示字符串的语言里收到这样的文本的样子。
func (c CodeUnits) Text() (string, error) {
	units, err := c.Units()
	if err != nil {
		return "", err
	}
	text := make([]byte, 0, len(units)*3)
	for i := 0; i < len(units); i++ {
		unit := rune(units[i])
		if isHighSurrogate(unit) && i+1 < len(units) && isLowSurrogate(rune(units[i+1])) {
			text = utf8.AppendRune(text, utf16.DecodeRune(unit, rune(units[i+1])))
			i++
			continue
		}
		if utf16.IsSurrogate(unit) {
			// utf8.AppendRune 会把代理项换成 U+FFFD，那样组出的文本是合法的，测不到拒收，所以照三字节的写法自己编
			text = append(text, 0xE0|byte(unit>>12), 0x80|byte(unit>>6)&0x3F, 0x80|byte(unit)&0x3F)
			continue
		}
		text = utf8.AppendRune(text, unit)
	}
	return string(text), nil
}

// isHighSurrogate 判断码元是否是高代理项（U+D800–U+DBFF）。
func isHighSurrogate(unit rune) bool { return unit >= 0xD800 && unit <= 0xDBFF }

// isLowSurrogate 判断码元是否是低代理项（U+DC00–U+DFFF）。
func isLowSurrogate(unit rune) bool { return unit >= 0xDC00 && unit <= 0xDFFF }

// caseText 返回一条用例的文本：写了 utf16 的按码元组成，否则是 JSON 字符串里的原文；两样都写了是向量写错了。
func caseText(name, text string, units CodeUnits) (string, error) {
	if units == nil {
		return text, nil
	}
	if text != "" {
		return "", fmt.Errorf("conformance: case %q gives both the text and its utf16 code units", name)
	}
	return units.Text()
}
