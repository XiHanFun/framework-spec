package rules

import (
	"slices"
	"strings"
)

// handshakeLinePrefix 是握手行格式版本 1 的前缀，含其后的一个空格（handshake.md §2）。
const handshakeLinePrefix = "xihan-plugin/1 "

// EndpointNetworks 是端点允许的 network（handshake.md §2）。
var EndpointNetworks = []string{"unix", "tcp", "npipe"}

// Endpoint 是握手行里的插件端点，或 XIHAN_PLUGIN_HOST_ADDRESS 里的一个宿主回调端点。
type Endpoint struct {
	Network string
	Address string
}

// IsWhitespace 判断字符是不是规范所说的空白（RFC-0001「空白字符」，握手行、回调端点与 Error 的文案都用它）：U+0009–U+000D、U+0020、U+0085、U+00A0、
// U+1680、U+2000–U+200A、U+2028、U+2029、U+202F、U+205F、U+3000。
//
// 这个集合等于 .NET 正则的 \s 与 Unicode 的 White_Space 属性；RE2 的 \s 只认 ASCII 空白，不能直接用。
func IsWhitespace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r == 0x20, r == 0x85, r == 0xA0, r == 0x1680,
		r >= 0x2000 && r <= 0x200A, r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// ParseHandshakeLine 解析去掉行尾之后的握手行 xihan-plugin/1 <network> <address>（handshake.md §2），
// 前缀之后的部分同 ParseEndpoint。
func ParseHandshakeLine(line string) (Endpoint, bool) {
	rest, ok := strings.CutPrefix(line, handshakeLinePrefix)
	if !ok {
		return Endpoint{}, false
	}
	return ParseEndpoint(rest)
}

// ParseEndpoint 解析 <network> <address>（handshake.md §2）：network 是 unix、tcp 或 npipe，与地址之间恰好一个空格；
// 地址是一个或多个 IsWhitespace 之外的字符（含非 ASCII 字母与 ;），tcp 的地址另须通过 IsLoopbackAddress。
func ParseEndpoint(text string) (Endpoint, bool) {
	network, address, found := strings.Cut(text, " ")
	if !found || !slices.Contains(EndpointNetworks, network) || address == "" || strings.ContainsFunc(address, IsWhitespace) {
		return Endpoint{}, false
	}
	if network == "tcp" && !IsLoopbackAddress(address) {
		return Endpoint{}, false
	}
	return Endpoint{Network: network, Address: address}, true
}

// IsLoopbackAddress 判断 tcp 端点的地址是不是回环地址加端口（handshake.md §2）：IPv4 是 127.0.0.0/8 的点分十进制，
// 四段、每段 0–255、不带前导零；IPv6 只认 [::1]；端口是 1–65535 的十进制，不带正负号与前导零。主机名不算。
func IsLoopbackAddress(address string) bool {
	separator := strings.LastIndexByte(address, ':')
	if separator < 0 {
		return false
	}
	if port, ok := decimal(address[separator+1:], 65535); !ok || port == 0 {
		return false
	}
	host := address[:separator]
	if host == "[::1]" {
		return true
	}
	// 只认四段写法：127.1、0177.0.0.1、2130706433、0x7f.0.0.1 这些操作系统也接受的写法一律不合法
	segments := strings.Split(host, ".")
	if len(segments) != 4 || segments[0] != "127" {
		return false
	}
	for _, segment := range segments[1:] {
		if _, ok := decimal(segment, 255); !ok {
			return false
		}
	}
	return true
}

// ParseHostAddresses 解析 XIHAN_PLUGIN_HOST_ADDRESS（handshake.md §4）：一个或多个 <network> <address>，按宿主的偏好排序、
// 以 ; 分隔，每项同 ParseEndpoint 且地址不含 ;。任何一项不合法整个值就不合法，空串也不合法。
func ParseHostAddresses(value string) ([]Endpoint, bool) {
	parts := strings.Split(value, ";")
	endpoints := make([]Endpoint, 0, len(parts))
	for _, part := range parts {
		// 按 ; 切开后地址里自然没有 ;；空项（开头、结尾或连续的 ;）在 ParseEndpoint 里不合法
		endpoint, ok := ParseEndpoint(part)
		if !ok {
			return nil, false
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, true
}

// SelectHostEndpoint 按 handshake.md §4 选回调端点：值合法时按顺序取第一个 network 在 supported 里的项，之后不换。
// 变量缺失（按空串传入）、不合法或没有可用的项时返回假：插件照常服务，回调以 plugins.host.unavailable 失败、不发请求。
func SelectHostEndpoint(value string, supported []string) (Endpoint, bool) {
	endpoints, ok := ParseHostAddresses(value)
	if !ok {
		return Endpoint{}, false
	}
	for _, endpoint := range endpoints {
		if slices.Contains(supported, endpoint.Network) {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

// FormatHostAddresses 按 XIHAN_PLUGIN_HOST_ADDRESS 的写法写出端点列表：<network> <address>，以 ; 分隔。
func FormatHostAddresses(endpoints []Endpoint) string {
	parts := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		parts = append(parts, endpoint.Network+" "+endpoint.Address)
	}
	return strings.Join(parts, ";")
}

// decimal 读不带正负号与前导零的十进制数（"0" 本身可以），只认 ASCII 数字；不合写法或超过 limit 时返回假。
func decimal(text string, limit int) (int, bool) {
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return 0, false
	}
	value := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		value = value*10 + int(c-'0')
		// 逐位比较上限，过长的数字不会溢出
		if value > limit {
			return 0, false
		}
	}
	return value, true
}
