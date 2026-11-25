package rules

import (
	"strconv"
	"strings"
	"time"
)

// cron 年份段的取值范围与搜索上限（RFC-0013「作业与触发器」的 cron）；步长的上限是各段能写的最大值，见 cronField。
const (
	// cronMinYear 是年份段能写的最小年份。
	cronMinYear = 1970
	// cronMaxYear 是年份段能写的最大年份。
	cronMaxYear = 2099
	// cronSearchLimit 是推算下一次触发时向后搜索的上限：下一次的墙上时间与起点（去掉秒以下）相差不超过 1830 天。
	cronSearchLimit = 5 * 366 * 24 * time.Hour
)

var (
	// cronMonthNames 是月的英文缩写名，下标 0 是一月。
	cronMonthNames = []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
	// cronWeekdayNames 是星期的英文缩写名，下标 0 是周日。
	cronWeekdayNames = []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}
)

// cronField 是一个段的取值范围与可用的名字；max 同时是这一段步长的上限（星期在 0 到 7 里数，上限是 7）。
type cronField struct {
	min, max int
	names    []string
}

var (
	cronSecondField     = cronField{min: 0, max: 59}
	cronMinuteField     = cronField{min: 0, max: 59}
	cronHourField       = cronField{min: 0, max: 23}
	cronDayOfMonthField = cronField{min: 1, max: 31}
	cronMonthField      = cronField{min: 1, max: 12, names: cronMonthNames}
	cronDayOfWeekField  = cronField{min: 0, max: 7, names: cronWeekdayNames}
	cronYearField       = cronField{min: cronMinYear, max: cronMaxYear}
)

// CronExpression 是解析后的 cron 表达式（RFC-0013「作业与触发器」）：5 段（分 时 日 月 星期，秒为 0）、
// 6 段（秒 分 时 日 月 星期）或再加第 7 段年份。
type CronExpression struct {
	seconds [60]bool
	minutes [60]bool
	hours   [24]bool
	months  [12]bool
	// years 为 nil 表示没有年份段。
	years []bool

	// monthDaysRestricted 与 weekdaysRestricted 记日与星期是否有限定（不是 * 或 ?）；两者都限定时取并集。
	monthDaysRestricted bool
	weekdaysRestricted  bool
	// days 是按日期写的日；lastDayOffsets[n] 是 L-n（L 即 n = 0）；nearestWeekdays[d-1] 是 dW；lastWeekday 是 LW。
	days            [31]bool
	lastDayOffsets  [31]bool
	nearestWeekdays [31]bool
	lastWeekday     bool
	// weekdays 是按星期写的日，下标 0 是周日；lastWeekdays[w] 是 wL；nthWeekdays[(k-1)*7+w] 是 w#k。
	weekdays     [7]bool
	lastWeekdays [7]bool
	nthWeekdays  [35]bool
}

// IsCronExpression 判断 cron 表达式是否合文法。
func IsCronExpression(expression string) bool {
	_, ok := ParseCron(expression)
	return ok
}

// ParseCron 解析 cron 表达式；不合文法返回假。各段以空格与制表符分隔，别的空白不是分隔符。
func ParseCron(expression string) (*CronExpression, bool) {
	fields := strings.FieldsFunc(expression, func(char rune) bool { return char == ' ' || char == '\t' })
	if len(fields) < 5 || len(fields) > 7 {
		return nil, false
	}
	parsed := &CronExpression{}
	// 6 段起第一段是秒；5 段写法等价于秒为 0
	offset := 0
	if len(fields) >= 6 {
		offset = 1
		if !parseCronList(fields[0], cronSecondField, parsed.seconds[:]) {
			return nil, false
		}
	} else {
		parsed.seconds[0] = true
	}
	if !parseCronList(fields[offset], cronMinuteField, parsed.minutes[:]) ||
		!parseCronList(fields[offset+1], cronHourField, parsed.hours[:]) ||
		!parsed.parseMonthDays(fields[offset+2]) ||
		!parseCronList(fields[offset+3], cronMonthField, parsed.months[:]) ||
		!parsed.parseWeekdays(fields[offset+4]) {
		return nil, false
	}
	if len(fields) == 7 {
		parsed.years = make([]bool, cronMaxYear-cronMinYear+1)
		if !parseCronList(fields[6], cronYearField, parsed.years) {
			return nil, false
		}
	}
	return parsed, true
}

// parseCronList 解析一个普通的段：逗号分隔的各项，每项是值、范围或带步长的范围；? 只能写在日与星期上。
func parseCronList(text string, field cronField, set []bool) bool {
	if text == "?" {
		return false
	}
	for item := range strings.SplitSeq(text, ",") {
		if !parseCronItem(item, field, set) {
			return false
		}
	}
	return true
}

// parseCronItem 解析列表里的一项并把取到的值记进 set（下标是值减去段的最小值）。
func parseCronItem(item string, field cronField, set []bool) bool {
	step := 1
	body, stepText, hasStep := strings.Cut(item, "/")
	if hasStep {
		// 步长不超过这一段能写的最大值（与 Quartz、cronie 一致，ADR-0297）
		value, ok := parseCronNumber(stepText)
		if !ok || value < 1 || value > field.max {
			return false
		}
		step = value
	}
	start, end, ok := parseCronRange(body, field, hasStep)
	if !ok {
		return false
	}
	for value := start; value <= end; value += step {
		set[value-field.min] = true
	}
	return true
}

// parseCronRange 解析 *、单个值或 起-止；单个值带步长时一直取到段的最大值。
func parseCronRange(text string, field cronField, hasStep bool) (start, end int, ok bool) {
	if text == "*" {
		return field.min, field.max, true
	}
	first, last, isRange := strings.Cut(text, "-")
	if !isRange {
		value, ok := parseCronValue(text, field)
		if !ok {
			return 0, 0, false
		}
		if hasStep {
			return value, field.max, true
		}
		return value, value, true
	}
	if start, ok = parseCronValue(first, field); !ok {
		return 0, 0, false
	}
	if end, ok = parseCronValue(last, field); !ok || start > end {
		return 0, 0, false
	}
	return start, end, true
}

// parseCronValue 解析一个值：范围内的数字，或这个段的名字（不区分 ASCII 字母的大小写）。
func parseCronValue(text string, field cronField) (int, bool) {
	if value, ok := parseCronNumber(text); ok {
		return value, value >= field.min && value <= field.max
	}
	for index, name := range field.names {
		if cronUpper(text) == name {
			return field.min + index, true
		}
	}
	return 0, false
}

// cronUpper 把 ASCII 小写字母换成大写，别的字符不动：名字与 L、W 只认 ASCII 字母，
// 不随 Unicode 的大小写折叠把长 s（U+017F）之类的字符认成 S。
func cronUpper(text string) string {
	return strings.Map(func(char rune) rune {
		if char >= 'a' && char <= 'z' {
			return char - 'a' + 'A'
		}
		return char
	}, text)
}

// parseCronNumber 解析只由 ASCII 数字组成的非负整数（可以有前导 0）；符号、小数点与空白都不认。
func parseCronNumber(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(text)
	return value, err == nil
}

// parseMonthDays 解析日的段：*、?，或 L、LW、L-n、dW 与普通的值、范围、步长组成的列表。
func (c *CronExpression) parseMonthDays(text string) bool {
	if text == "*" || text == "?" {
		return true
	}
	c.monthDaysRestricted = true
	for item := range strings.SplitSeq(text, ",") {
		upper := cronUpper(item)
		switch {
		case upper == "L":
			c.lastDayOffsets[0] = true
		case upper == "LW":
			c.lastWeekday = true
		case strings.HasPrefix(upper, "L-"):
			back, ok := parseCronNumber(item[2:])
			if !ok || back > 30 {
				return false
			}
			c.lastDayOffsets[back] = true
		case len(item) > 1 && strings.HasSuffix(upper, "W"):
			day, ok := parseCronValue(item[:len(item)-1], cronDayOfMonthField)
			if !ok {
				return false
			}
			c.nearestWeekdays[day-1] = true
		default:
			if !parseCronItem(item, cronDayOfMonthField, c.days[:]) {
				return false
			}
		}
	}
	return true
}

// parseWeekdays 解析星期的段：*、?，或 w#k、wL 与普通的值、范围、步长组成的列表；7 与 0 都是周日。
func (c *CronExpression) parseWeekdays(text string) bool {
	if text == "*" || text == "?" {
		return true
	}
	c.weekdaysRestricted = true
	var raw [8]bool
	for item := range strings.SplitSeq(text, ",") {
		if weekdayText, ordinalText, isNth := strings.Cut(item, "#"); isNth {
			weekday, ok := parseCronValue(weekdayText, cronDayOfWeekField)
			ordinal, okOrdinal := parseCronNumber(ordinalText)
			if !ok || !okOrdinal || ordinal < 1 || ordinal > 5 {
				return false
			}
			c.nthWeekdays[(ordinal-1)*7+weekday%7] = true
			continue
		}
		if len(item) > 1 && strings.HasSuffix(cronUpper(item), "L") {
			weekday, ok := parseCronValue(item[:len(item)-1], cronDayOfWeekField)
			if !ok {
				return false
			}
			c.lastWeekdays[weekday%7] = true
			continue
		}
		if !parseCronItem(item, cronDayOfWeekField, raw[:]) {
			return false
		}
	}
	copy(c.weekdays[:], raw[:7])
	c.weekdays[0] = c.weekdays[0] || raw[7]
	return true
}

// Next 给出严格晚于 after 的下一次触发时刻（UTC）：按 location 的墙上时间匹配；夏令时开始时被跳过的墙上时间不运行，
// 结束时重复的墙上时间只运行第一次；下一次的墙上时间与起点（去掉秒以下）相差超过搜索上限（1830 天）返回假。
func (c *CronExpression) Next(after time.Time, location *time.Location) (time.Time, bool) {
	// 游标是墙上时间，借 UTC 的 time.Time 做不带时区的日历运算；从 after 的下一个整秒开始
	local := after.In(location)
	cursor := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), 0, time.UTC).Add(time.Second)
	limit := cursor.Add(cronSearchLimit)
	// 逐级跳过：年不匹配跳到下一个匹配年的年初，月不匹配跳到下一个匹配月的月初，以此类推，不逐秒遍历
	for cursor.Before(limit) {
		year, month, day := cursor.Year(), int(cursor.Month()), cursor.Day()
		matchingYear := c.nextYear(year)
		if matchingYear < 0 {
			return time.Time{}, false
		}
		if matchingYear != year {
			cursor = time.Date(matchingYear, 1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		matchingMonth := nextCronSet(c.months[:], month-1) + 1
		if matchingMonth == 0 {
			cursor = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if matchingMonth != month {
			cursor = time.Date(year, time.Month(matchingMonth), 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		matchingDay := nextCronDay(c.dayMask(year, month), day)
		if matchingDay < 0 {
			cursor = time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if matchingDay != day {
			cursor = time.Date(year, time.Month(month), matchingDay, 0, 0, 0, 0, time.UTC)
			day = matchingDay
		}

		hour := nextCronSet(c.hours[:], cursor.Hour())
		if hour < 0 {
			cursor = time.Date(year, time.Month(month), day+1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if hour != cursor.Hour() {
			cursor = time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC)
		}
		minute := nextCronSet(c.minutes[:], cursor.Minute())
		if minute < 0 {
			cursor = time.Date(year, time.Month(month), day, hour+1, 0, 0, 0, time.UTC)
			continue
		}
		if minute != cursor.Minute() {
			cursor = time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
		}
		second := nextCronSet(c.seconds[:], cursor.Second())
		if second < 0 {
			cursor = time.Date(year, time.Month(month), day, hour, minute+1, 0, 0, time.UTC)
			continue
		}
		cursor = time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
		// 上限按候选的墙上时间判定：游标按年、月、日跳着走，跳到的那一天可能已经越过上限
		if !cursor.Before(limit) {
			return time.Time{}, false
		}

		// 被跳过的墙上时间不存在；after 落在重复时刻的第二次出现时，第一次出现并不晚于 after
		instant, exists := resolveWallTime(cursor, location)
		if !exists || !instant.After(after) {
			cursor = cursor.Add(time.Second)
			continue
		}
		return instant, true
	}
	return time.Time{}, false
}

// nextYear 给出不早于 from 的下一个匹配的年份；没有年份段时就是 from，没有匹配的返回 -1。
func (c *CronExpression) nextYear(from int) int {
	if c.years == nil {
		return from
	}
	for year := max(from, cronMinYear); year <= cronMaxYear; year++ {
		if c.years[year-cronMinYear] {
			return year
		}
	}
	return -1
}

// dayMask 给出这个月里匹配的日，第 d 位是 d 日：日与星期都有限定时取并集，都没有限定时是整个月。
func (c *CronExpression) dayMask(year, month int) uint32 {
	daysInMonth := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	firstWeekday := int(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Weekday())
	if !c.monthDaysRestricted && !c.weekdaysRestricted {
		return uint32((uint64(1) << (daysInMonth + 1)) - 2)
	}

	var mask uint32
	if c.monthDaysRestricted {
		// 离 day 最近的工作日，不跨月：周六取前一天（1 日是周六取 3 日），周日取后一天（月末是周日取前两天）
		nearestWeekday := func(day int) int {
			switch (firstWeekday + day - 1) % 7 {
			case int(time.Saturday):
				if day == 1 {
					return 3
				}
				return day - 1
			case int(time.Sunday):
				if day == daysInMonth {
					return day - 2
				}
				return day + 1
			default:
				return day
			}
		}
		for day := 1; day <= daysInMonth; day++ {
			if c.days[day-1] {
				mask |= 1 << day
			}
			if c.nearestWeekdays[day-1] {
				mask |= 1 << nearestWeekday(day)
			}
			if c.lastDayOffsets[daysInMonth-day] {
				mask |= 1 << day
			}
		}
		if c.lastWeekday {
			mask |= 1 << nearestWeekday(daysInMonth)
		}
	}
	if c.weekdaysRestricted {
		for day := 1; day <= daysInMonth; day++ {
			weekday := (firstWeekday + day - 1) % 7
			// 这个星期在本月的最后一次出现：再过七天就出了月
			if c.weekdays[weekday] || (c.lastWeekdays[weekday] && day+7 > daysInMonth) || c.nthWeekdays[(day-1)/7*7+weekday] {
				mask |= 1 << day
			}
		}
	}
	return mask
}

// nextCronSet 给出 set 里不小于 from 的第一个为真的下标，没有返回 -1。
func nextCronSet(set []bool, from int) int {
	for index := from; index < len(set); index++ {
		if set[index] {
			return index
		}
	}
	return -1
}

// nextCronDay 给出 mask 里不早于 from 的第一个日，没有返回 -1。
func nextCronDay(mask uint32, from int) int {
	for day := from; day <= 31; day++ {
		if mask&(1<<day) != 0 {
			return day
		}
	}
	return -1
}

// resolveWallTime 把墙上时间换算成时刻：wall 是借 UTC 表示的墙上时间。夏令时开始时被跳过的墙上时间不存在，返回假；
// 结束时重复的墙上时间取第一次出现（偏移较大、时刻较早的那个）。
func resolveWallTime(wall time.Time, location *time.Location) (time.Time, bool) {
	guess := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, location)
	var resolved time.Time
	found := false
	// 候选偏移取这一刻与前后各一天的偏移：转换点两侧的偏移都在其中；偏移对得上的时刻才真的显示成这个墙上时间
	for _, probe := range []time.Time{guess.Add(-24 * time.Hour), guess, guess.Add(24 * time.Hour)} {
		_, offset := probe.Zone()
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		if _, actual := candidate.In(location).Zone(); actual != offset {
			continue
		}
		if !found || candidate.Before(resolved) {
			resolved, found = candidate, true
		}
	}
	return resolved, found
}
