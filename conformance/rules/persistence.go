package rules

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var explicitIndexNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// 索引名的前缀与上限（RFC-0005「索引声明」）。
const (
	// MaxIndexNameLength 是索引名的最大长度（PostgreSQL 标识符的上限）；超过是声明错误，不截断。
	MaxIndexNameLength = 63
	// IndexNamePrefix 是普通索引名的前缀。
	IndexNamePrefix = "IX_"
	// UniqueIndexNamePrefix 是唯一索引名的前缀。
	UniqueIndexNamePrefix = "UX_"
)

// 雪花标识的位布局（RFC-0005「雪花标识」）。
const (
	// SnowflakeTimestampBits 是毫秒数占的位数。
	SnowflakeTimestampBits = 41
	// SnowflakeWorkerBits 是机器位占的位数。
	SnowflakeWorkerBits = 10
	// SnowflakeSequenceBits 是序列占的位数。
	SnowflakeSequenceBits = 12
	// MaxSnowflakeWorker 是机器位的最大值。
	MaxSnowflakeWorker = 1<<SnowflakeWorkerBits - 1
	// MaxSnowflakeSequence 是序列的最大值。
	MaxSnowflakeSequence = 1<<SnowflakeSequenceBits - 1
	// MaxSnowflakeTimestamp 是毫秒数的最大值。
	MaxSnowflakeTimestamp = 1<<SnowflakeTimestampBits - 1
	// SnowflakeWorkerLockPrefix 加机器位的十进制写法，是租约分配机器位时锁的资源名。
	SnowflakeWorkerLockPrefix = "persistence:snowflake:worker:"
)

// DefaultSnowflakeEpoch 是雪花标识 epoch 的缺省值：2026-01-01T00:00:00Z。
var DefaultSnowflakeEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// IndexColumnAbbreviation 给出属性名在索引名里的缩写（RFC-0005「索引声明」）：每个词的前两个字符；
// 词从首字符、小写或数字之后的大写字母、连续大写里后跟小写的那个大写字母开始，_ 只分词不计入。
func IndexColumnAbbreviation(property string) string {
	chars := []rune(property)
	var builder strings.Builder
	// taken 是当前词已经读过的字符数，到 2 之后的字符不再取
	taken := 0
	for index, char := range chars {
		if char == '_' {
			taken = 0
			continue
		}
		if index > 0 && unicode.IsUpper(char) &&
			(!unicode.IsUpper(chars[index-1]) || (index+1 < len(chars) && unicode.IsLower(chars[index+1]))) {
			taken = 0
		}
		if taken < 2 {
			builder.WriteRune(char)
		}
		taken++
	}
	return builder.String()
}

// IndexName 推出索引的名字（RFC-0005「索引声明」）：前缀加表名（字母、数字、_ 以外的字符换成 _），再加每列一段缩写，
// 以 _ 连接；超过 MaxIndexNameLength 个字符返回假（声明错误，不截断）。
func IndexName(table string, properties []string, unique bool) (string, bool) {
	var builder strings.Builder
	if unique {
		builder.WriteString(UniqueIndexNamePrefix)
	} else {
		builder.WriteString(IndexNamePrefix)
	}
	for _, char := range table {
		// 只认 ASCII 的字母与数字：别的字符在各数据库的标识符里不一定能用
		if char == '_' || (char < unicode.MaxASCII && (unicode.IsLetter(char) || unicode.IsDigit(char))) {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	for _, property := range properties {
		builder.WriteByte('_')
		builder.WriteString(IndexColumnAbbreviation(property))
	}
	name := builder.String()
	return name, len(name) <= MaxIndexNameLength
}

// IsExplicitIndexName 判断声明里给出的索引名是否合法（RFC-0005「索引声明」）：字母开头，
// 只含 ASCII 字母、数字与 _，不超过 MaxIndexNameLength 个字符。
func IsExplicitIndexName(name string) bool {
	return len(name) <= MaxIndexNameLength && explicitIndexNamePattern.MatchString(name)
}

// ComposeSnowflakeID 由三段合成雪花标识（RFC-0005「雪花标识」）：毫秒数在第 62–22 位，机器位在第 21–12 位，
// 序列在第 11–0 位；任何一段超出范围返回假。
func ComposeSnowflakeID(milliseconds int64, worker, sequence int) (int64, bool) {
	if milliseconds < 0 || milliseconds > MaxSnowflakeTimestamp || worker < 0 || worker > MaxSnowflakeWorker || sequence < 0 || sequence > MaxSnowflakeSequence {
		return 0, false
	}
	return milliseconds<<(SnowflakeWorkerBits+SnowflakeSequenceBits) | int64(worker)<<SnowflakeSequenceBits | int64(sequence), true
}

// ParseSnowflakeID 把雪花标识还原成三段：自 epoch 起的毫秒数、机器位与序列。value 不为负。
func ParseSnowflakeID(value int64) (milliseconds int64, worker, sequence int) {
	return value >> (SnowflakeWorkerBits + SnowflakeSequenceBits), int(value >> SnowflakeSequenceBits & MaxSnowflakeWorker), int(value & MaxSnowflakeSequence)
}

// SnowflakeTime 给出雪花标识的时刻：epoch 加它的毫秒数。
func SnowflakeTime(value int64, epoch time.Time) time.Time {
	milliseconds, _, _ := ParseSnowflakeID(value)
	return epoch.UTC().Add(time.Duration(milliseconds) * time.Millisecond)
}

// SnowflakeWorkerLockResource 给出租约分配机器位时锁的资源名。
func SnowflakeWorkerLockResource(worker int) string {
	return SnowflakeWorkerLockPrefix + strconv.Itoa(worker)
}
