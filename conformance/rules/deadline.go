package rules

import "time"

// deadlineLayout 是写出截止时间的日期与时间部分：.NET 的 O 格式，七位小数（RFC-0003 规则 8）；偏移另写 +00:00。
const deadlineLayout = "2006-01-02T15:04:05.0000000"

var (
	// minDeadline 是换算成 UTC 之后（不计小数）允许的最早时刻。
	minDeadline = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	// maxDeadline 是换算成 UTC 之后（不计小数）允许的最晚时刻。
	maxDeadline = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)

// ParseDeadline 按 RFC-0003 规则 8 的文法严格读截止时间的文本，返回 UTC 时刻（纳秒精度）；
// 插件协议的 InvocationMetadata.deadline（invocation.md §2）与 HTTP 载体的 xihan-deadline 头（protocol/http.md）是同一条文法：
// ^[0-9]{4}-[0-9]{2}-[0-9]{2}[Tt][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?([Zz]|[+-][0-9]{2}:[0-9]{2})\z，
// 年 0001–9999、月日按公历（含闰年）、时 00–23、分 00–59、秒 00–59（不认闰秒）、偏移的时 00–23 与分 00–59，
// 换算成 UTC 之后（不计小数）在 0001-01-01T00:00:00 到 9999-12-31T23:59:59 之间。
// 读不出（含空串）返回假，调用方当作没有截止时间；时钟精度比纳秒粗的实现把多出的小数截断、不进位。
func ParseDeadline(text string) (time.Time, bool) {
	// 固定部分 yyyy-MM-ddTHH:mm:ss 是 19 个字节，之后至少还有一个字节的偏移
	if len(text) < 20 || text[4] != '-' || text[7] != '-' || (text[10] != 'T' && text[10] != 't') || text[13] != ':' || text[16] != ':' {
		return time.Time{}, false
	}
	year, okYear := fixedDigits(text[0:4])
	month, okMonth := fixedDigits(text[5:7])
	day, okDay := fixedDigits(text[8:10])
	hour, okHour := fixedDigits(text[11:13])
	minute, okMinute := fixedDigits(text[14:16])
	second, okSecond := fixedDigits(text[17:19])
	if !okYear || !okMonth || !okDay || !okHour || !okMinute || !okSecond {
		return time.Time{}, false
	}
	// time.Date 会把越界的日期进位成别的日子，所以每一段先按公历核对
	if year < 1 || month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) || hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, false
	}

	rest := text[19:]
	nanos := 0
	if rest != "" && rest[0] == '.' {
		end := 1
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		count := end - 1
		if count < 1 || count > 9 {
			return time.Time{}, false
		}
		nanos, _ = fixedDigits(rest[1:end])
		// 不足九位的小数按右补零换算成纳秒
		for i := count; i < 9; i++ {
			nanos *= 10
		}
		rest = rest[end:]
	}

	offset := 0
	switch {
	case rest == "Z" || rest == "z":
	case len(rest) == 6 && (rest[0] == '+' || rest[0] == '-') && rest[3] == ':':
		offsetHours, okHours := fixedDigits(rest[1:3])
		offsetMinutes, okMinutes := fixedDigits(rest[4:6])
		if !okHours || !okMinutes || offsetHours > 23 || offsetMinutes > 59 {
			return time.Time{}, false
		}
		offset = offsetHours*3600 + offsetMinutes*60
		if rest[0] == '-' {
			offset = -offset
		}
	default:
		return time.Time{}, false
	}

	// 范围按换算成 UTC 之后、不计小数的时刻判定，所以 9999-12-31T23:59:59.999999999Z 仍然合法
	utc := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC).Add(-time.Duration(offset) * time.Second)
	if utc.Before(minDeadline) || utc.After(maxDeadline) {
		return time.Time{}, false
	}
	return utc.Add(time.Duration(nanos)), true
}

// FormatDeadline 按 .NET 的 O 格式写出截止时间（RFC-0003 规则 8）：换算成 UTC、七位小数、+00:00 偏移；
// 比 100 纳秒更细的部分截断、不进位，小数不足七位补零。
func FormatDeadline(deadline time.Time) string {
	return deadline.UTC().Format(deadlineLayout) + "+00:00"
}

// EffectiveDeadline 取生效的截止时间（invocation.md §2）：metadata 按 ParseDeadline 读、读不出当作没有，
// 与 gRPC 调用自身的截止时间（hasGRPC 为假表示没有，与 context.Context.Deadline 的返回值对应）取较早的一个；
// 两者都没有时返回假，即没有截止时间。
func EffectiveDeadline(metadata string, grpc time.Time, hasGRPC bool) (time.Time, bool) {
	parsed, hasMetadata := ParseDeadline(metadata)
	switch {
	case hasMetadata && hasGRPC:
		if grpc.Before(parsed) {
			return grpc.UTC(), true
		}
		return parsed, true
	case hasMetadata:
		return parsed, true
	case hasGRPC:
		return grpc.UTC(), true
	default:
		return time.Time{}, false
	}
}

// fixedDigits 读一段全是 ASCII 数字的文本（全角数字等其他数字字符不算）；有别的字符时返回假。
func fixedDigits(text string) (int, bool) {
	value := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		value = value*10 + int(c-'0')
	}
	return value, text != ""
}

// daysInMonth 返回公历某年某月的天数；闰年按四年一闰、百年不闰、四百年再闰。
func daysInMonth(year, month int) int {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// DeadlineHeader 是 HTTP 载体带截止时间的请求头（RFC-0003 规则 8，protocol/http.md）；gRPC 的元数据同名。
const DeadlineHeader = "xihan-deadline"

// GRPCTimeoutHeader 是 gRPC 标准的相对超时头（protocol/grpc.md）。
const GRPCTimeoutHeader = "grpc-timeout"

// latestDeadline 是换算出的截止时间超出范围时截到的时刻：ParseDeadline 能读出的最晚时刻。
var latestDeadline = maxDeadline.Add(time.Second - time.Nanosecond)

// HTTPDeadline 取 HTTP 载体上的截止时间（RFC-0003 规则 8，protocol/http.md，protocol/grpc.md）：
// deadline 是 xihan-deadline 头的各个值，timeout 是 grpc-timeout 头的各个值（一个头出现几次就有几项，没有这个头为空），now 是宿主时钟的当前时刻。
// xihan-deadline 的值按 ParseDeadline 整串读，读不出（含空串、头出现多次）当作没有这个头；
// 没有它时 grpc-timeout 生效，相对 now；两个都没有（或都读不出）返回假，即没有截止时间。
func HTTPDeadline(deadline, timeout []string, now time.Time) (time.Time, bool) {
	// 一个头出现多次时各个值以逗号并成一个文本（RFC 9110 §5.3），逗号不在两条文法里，所以并出来的文本一定读不出
	if parsed, ok := ParseDeadline(joinFieldValues(deadline)); ok {
		return parsed, true
	}
	seconds, nanos, ok := parseGRPCTimeout(joinFieldValues(timeout))
	if !ok {
		return time.Time{}, false
	}
	// 八位数的小时有一万多年，time.Duration 装不下：按秒与纳秒分开算，超出能读出的最晚时刻就截到那里（protocol/grpc.md）
	utc := now.UTC()
	if seconds > latestDeadline.Unix()-utc.Unix() {
		return latestDeadline, true
	}
	result := time.Unix(utc.Unix()+seconds, int64(utc.Nanosecond())+nanos).UTC()
	if result.After(latestDeadline) {
		return latestDeadline, true
	}
	return result, true
}

// ParseGRPCTimeout 读 grpc-timeout 的值（protocol/grpc.md）：1 到 8 位 ASCII 十进制数字加一个单位，
// H 时、M 分、S 秒、m 毫秒、u 微秒、n 纳秒；整串匹配。读不出返回假。
// 八位数的小时超出 time.Duration 的范围时返回能表示的最大时长，换算成时刻用 HTTPDeadline。
func ParseGRPCTimeout(text string) (time.Duration, bool) {
	seconds, nanos, ok := parseGRPCTimeout(text)
	if !ok {
		return 0, false
	}
	const maxSeconds = int64(1<<63-1) / int64(time.Second)
	if seconds >= maxSeconds {
		return time.Duration(1<<63 - 1), true
	}
	return time.Duration(seconds)*time.Second + time.Duration(nanos), true
}

// parseGRPCTimeout 把 grpc-timeout 的值拆成整秒与不足一秒的纳秒；读不出返回假。
func parseGRPCTimeout(text string) (seconds, nanos int64, ok bool) {
	if len(text) < 2 || len(text) > 9 {
		return 0, 0, false
	}
	value, digits := fixedDigits(text[:len(text)-1])
	if !digits {
		return 0, 0, false
	}
	amount := int64(value)
	switch text[len(text)-1] {
	case 'H':
		return amount * 3600, 0, true
	case 'M':
		return amount * 60, 0, true
	case 'S':
		return amount, 0, true
	case 'm':
		return amount / 1_000, amount % 1_000 * 1_000_000, true
	case 'u':
		return amount / 1_000_000, amount % 1_000_000 * 1_000, true
	case 'n':
		return amount / 1_000_000_000, amount % 1_000_000_000, true
	default:
		return 0, 0, false
	}
}

// joinFieldValues 把一个头的各个值并成一个文本：没有值是空串，一个值原样，多个值以逗号相连。
func joinFieldValues(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	}
	joined := values[0]
	for _, value := range values[1:] {
		joined += "," + value
	}
	return joined
}
