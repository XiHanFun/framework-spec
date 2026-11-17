package rules

import "time"

// deadlineLayout 是写出截止时间的日期与时间部分：.NET 的 O 格式，七位小数（invocation.md §2）；偏移另写 +00:00。
const deadlineLayout = "2006-01-02T15:04:05.0000000"

var (
	// minDeadline 是换算成 UTC 之后（不计小数）允许的最早时刻。
	minDeadline = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	// maxDeadline 是换算成 UTC 之后（不计小数）允许的最晚时刻。
	maxDeadline = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
)

// ParseDeadline 按 invocation.md §2 的文法严格读 InvocationMetadata.deadline，返回 UTC 时刻（纳秒精度）：
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

// FormatDeadline 按 .NET 的 O 格式写出截止时间（invocation.md §2）：换算成 UTC、七位小数、+00:00 偏移；
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
