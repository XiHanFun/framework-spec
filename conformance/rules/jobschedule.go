package rules

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// 计划文本里的间隔与时刻（RFC-0013「计划的文本形式」）。
const (
	// jobTicksPerSecond 是一秒里 100 纳秒的个数：间隔与时刻都精确到 100 纳秒。
	jobTicksPerSecond = 10_000_000
	// jobTicksPerDay 是一天里 100 纳秒的个数。
	jobTicksPerDay = 86_400 * jobTicksPerSecond
	// jobMaxIntervalTicks 是间隔的上限（以 100 纳秒计），即 10675199.02:48:05.4775807。
	jobMaxIntervalTicks = math.MaxInt64
	// jobMaxDayDigits 是间隔里天数的最多位数：上限是 10675199 天多。
	jobMaxDayDigits = 8
	// jobFractionDigits 是间隔里不足一秒的部分的位数。
	jobFractionDigits = 7
	// jobTick 是计划文本的时刻精度。
	jobTick = 100 * time.Nanosecond
)

// JobSchedule 是读出的计划文本（RFC-0013「计划的文本形式」）：不适用或没写的成员是零值或 nil。
type JobSchedule struct {
	// Kind 是种类：manual、every、cron 或 once。
	Kind string
	// IntervalTicks 是 every 的间隔，以 100 纳秒为单位。
	IntervalTicks int64
	// RepeatCount 是 every 的总运行次数，nil 为无限。
	RepeatCount *int
	// Cron 是 cron 的表达式，照原样（各段之间一个空格）。
	Cron string
	// TimeZone 是 cron 的时区名，nil 为 UTC。
	TimeZone *string
	// At 是 once 的时刻（UTC，精确到 100 纳秒）。
	At *time.Time
	// StartAt 是起始时刻（UTC），之前不运行。
	StartAt *time.Time
	// EndAt 是结束时刻（UTC），之后没有下一次。
	EndAt *time.Time
	// Calendar 是排除日历的名字。
	Calendar *string
}

// ParseJobSchedule 按 RFC-0013「计划的文本形式」读计划文本；不合文法、时区名在时区库里找不到都返回假。
// 各部分之间正好一个空格，种类之后是种类自己的部分，再按 tz=（只跟在 cron 后面）、start=、end=、calendar= 的次序接选项，
// 每个至多一次；时刻按 RFC-0003 规则 8 的读法（ParseDeadline），截到 100 纳秒。
func ParseJobSchedule(text string) (JobSchedule, bool) {
	// 制表符是 cron 自己认的分隔符，放进去会被当成两段，先挡掉；别的空白落在某一部分里，由那一部分的文法拒绝
	if text == "" || strings.ContainsRune(text, '\t') {
		return JobSchedule{}, false
	}
	parts := strings.Split(text, " ")
	for _, part := range parts {
		// 开头、结尾与连着的空格分出空的部分
		if part == "" {
			return JobSchedule{}, false
		}
	}
	schedule := JobSchedule{Kind: parts[0]}
	index := 1
	switch parts[0] {
	case "manual":
	case "every":
		if index >= len(parts) {
			return JobSchedule{}, false
		}
		ticks, ok := ParseJobInterval(parts[index])
		if !ok {
			return JobSchedule{}, false
		}
		schedule.IntervalTicks = ticks
		index++
		if index < len(parts) && strings.HasPrefix(parts[index], "x") {
			count, ok := parseJobRepeatCount(parts[index])
			if !ok {
				return JobSchedule{}, false
			}
			schedule.RepeatCount = &count
			index++
		}
	case "cron":
		// cron 的各段不含 '='，选项都含：表达式到第一个含 '=' 的部分为止
		first := index
		for index < len(parts) && !strings.Contains(parts[index], "=") {
			index++
		}
		expression := strings.Join(parts[first:index], " ")
		if _, ok := ParseCron(expression); !ok {
			return JobSchedule{}, false
		}
		schedule.Cron = expression
		if index < len(parts) && strings.HasPrefix(parts[index], "tz=") {
			zone := strings.TrimPrefix(parts[index], "tz=")
			if !IsKnownTimeZone(zone) {
				return JobSchedule{}, false
			}
			schedule.TimeZone = &zone
			index++
		}
	case "once":
		if index >= len(parts) {
			return JobSchedule{}, false
		}
		at, ok := parseJobInstant(parts[index])
		if !ok {
			return JobSchedule{}, false
		}
		schedule.At = &at
		index++
	default:
		return JobSchedule{}, false
	}

	// 选项按固定次序、每个至多一次：次序不对、重复与不认得的选项都会剩下没读完的部分
	for _, option := range []struct {
		prefix string
		target **time.Time
	}{{"start=", &schedule.StartAt}, {"end=", &schedule.EndAt}} {
		if index < len(parts) && strings.HasPrefix(parts[index], option.prefix) {
			instant, ok := parseJobInstant(strings.TrimPrefix(parts[index], option.prefix))
			if !ok {
				return JobSchedule{}, false
			}
			*option.target = &instant
			index++
		}
	}
	if index < len(parts) && strings.HasPrefix(parts[index], "calendar=") {
		calendar := strings.TrimPrefix(parts[index], "calendar=")
		if !IsJobName(calendar) {
			return JobSchedule{}, false
		}
		schedule.Calendar = &calendar
		index++
	}
	if index != len(parts) {
		return JobSchedule{}, false
	}
	return schedule, true
}

// FormatJobSchedule 按 RFC-0013「计划的文本形式」第 7 条写出计划文本：种类与它的部分，cron 有时区时接 tz=，
// 再依次接有值的 start=、end=、calendar=；时刻换算成 UTC、七位小数、+00:00（FormatDeadline）。
func FormatJobSchedule(schedule JobSchedule) string {
	var builder strings.Builder
	builder.WriteString(schedule.Kind)
	switch schedule.Kind {
	case "every":
		builder.WriteString(" " + FormatJobInterval(schedule.IntervalTicks))
		if schedule.RepeatCount != nil {
			builder.WriteString(" x" + strconv.Itoa(*schedule.RepeatCount))
		}
	case "cron":
		builder.WriteString(" " + schedule.Cron)
		if schedule.TimeZone != nil {
			builder.WriteString(" tz=" + *schedule.TimeZone)
		}
	case "once":
		builder.WriteString(" " + FormatDeadline(*schedule.At))
	}
	if schedule.StartAt != nil {
		builder.WriteString(" start=" + FormatDeadline(*schedule.StartAt))
	}
	if schedule.EndAt != nil {
		builder.WriteString(" end=" + FormatDeadline(*schedule.EndAt))
	}
	if schedule.Calendar != nil {
		builder.WriteString(" calendar=" + *schedule.Calendar)
	}
	return builder.String()
}

// ParseJobInterval 读计划文本里的间隔 [d.]hh:mm:ss[.fffffff]，返回以 100 纳秒计的长度：天数不带前导 0、不足一天不写，
// 时 00–23、分秒 00–59 各两位，不足一秒的部分正好七位、为 0 时不写；间隔大于 0、不超过 jobMaxIntervalTicks。
func ParseJobInterval(text string) (int64, bool) {
	var days int64
	colon := strings.IndexByte(text, ':')
	if dot := strings.IndexByte(text, '.'); dot >= 0 && (colon < 0 || dot < colon) {
		dayText := text[:dot]
		value, ok := jobDigits(dayText)
		if !ok || len(dayText) > jobMaxDayDigits || dayText[0] == '0' {
			return 0, false
		}
		days = value
		text = text[dot+1:]
	}
	if len(text) < 8 || text[2] != ':' || text[5] != ':' {
		return 0, false
	}
	hours, okHours := jobDigits(text[0:2])
	minutes, okMinutes := jobDigits(text[3:5])
	seconds, okSeconds := jobDigits(text[6:8])
	if !okHours || !okMinutes || !okSeconds || hours > 23 || minutes > 59 || seconds > 59 {
		return 0, false
	}
	var fraction int64
	if rest := text[8:]; rest != "" {
		// 不足一秒的部分：正好七位，为 0 时写法里不出现
		value, ok := jobDigits(strings.TrimPrefix(rest, "."))
		if len(rest) != jobFractionDigits+1 || rest[0] != '.' || !ok || value == 0 {
			return 0, false
		}
		fraction = value
	}
	// 天数最多八位，乘出来装得下 int64；与不足一天的部分相加之前先核对上限
	dayTicks := days * jobTicksPerDay
	rest := ((hours*60+minutes)*60+seconds)*jobTicksPerSecond + fraction
	if dayTicks > jobMaxIntervalTicks-rest {
		return 0, false
	}
	total := dayTicks + rest
	return total, total > 0
}

// FormatJobInterval 写出间隔（以 100 纳秒计，大于 0）：.NET TimeSpan 的 c 格式对正值的写法。
func FormatJobInterval(ticks int64) string {
	days := ticks / jobTicksPerDay
	rest := ticks % jobTicksPerDay
	seconds := rest / jobTicksPerSecond
	fraction := rest % jobTicksPerSecond
	text := fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	if days > 0 {
		text = strconv.FormatInt(days, 10) + "." + text
	}
	if fraction > 0 {
		text += fmt.Sprintf(".%07d", fraction)
	}
	return text
}

// IsTimeZoneName 判断计划文本里的时区名是否合文法（RFC-0013「计划的文本形式」第 6 条）：
// ^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$。
func IsTimeZoneName(name string) bool {
	if name == "" {
		return false
	}
	for component := range strings.SplitSeq(name, "/") {
		if component == "" {
			return false
		}
		for _, char := range component {
			if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '+' && char != '-' {
				return false
			}
		}
	}
	return true
}

// IsKnownTimeZone 判断时区名合文法并且在时区库里找得到（RFC-0013「计划的文本形式」第 6 条）。
// time.LoadLocation 把 Local 解释成本机时区，它不是 IANA 的名字，不收；UTC 不依赖时区库。
func IsKnownTimeZone(name string) bool {
	if !IsTimeZoneName(name) || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// parseJobRepeatCount 读总运行次数：x 接 1 到 2147483647、不带前导 0 的十进制整数。
func parseJobRepeatCount(text string) (int, bool) {
	digits := strings.TrimPrefix(text, "x")
	value, ok := jobDigits(digits)
	if !ok || len(digits) > 10 || digits[0] == '0' || value > math.MaxInt32 {
		return 0, false
	}
	return int(value), true
}

// parseJobInstant 读计划文本里的时刻：RFC-0003 规则 8 的读法，截到 100 纳秒。
func parseJobInstant(text string) (time.Time, bool) {
	instant, ok := ParseDeadline(text)
	if !ok {
		return time.Time{}, false
	}
	return instant.Truncate(jobTick), true
}

// jobDigits 读只由 ASCII 数字组成的非空十进制数；调用方先限定位数，不会溢出。
func jobDigits(text string) (int64, bool) {
	if text == "" || len(text) > 18 {
		return 0, false
	}
	var value int64
	for _, char := range text {
		if char < '0' || char > '9' {
			return 0, false
		}
		value = value*10 + int64(char-'0')
	}
	return value, true
}
