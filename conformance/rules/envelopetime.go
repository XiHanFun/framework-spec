package rules

import "time"

// EnvelopeTimeErrorCode 是宿主拒绝插件写来的 Envelope.time 时那一项的错误码（invocation.md §4，ADR-0293）。
const EnvelopeTimeErrorCode = "MESSAGING_VALIDATION_002"

// EnvelopeTimeErrorKey 是同一个错误的消息键，参数 time 是原文。
const EnvelopeTimeErrorKey = "messaging.envelope.invalid_time"

// ReadEnvelopeTime 按插件协议读 Envelope.time（invocation.md §4，ADR-0293）：空串表示没有这个属性，present 为假、ok 为真；
// 否则与 RFC-0009「信封」第 3 条同一条文法，也就是 ParseDeadline 的读法，读成 UTC 时刻。
// 读不出时 ok 为假：宿主不发布这一项，结果是 EnvelopeTimeErrorCode + EnvelopeTimeErrorKey，不当作没有时间。
func ReadEnvelopeTime(text string) (instant time.Time, present bool, ok bool) {
	if text == "" {
		return time.Time{}, false, true
	}
	instant, ok = ParseDeadline(text)
	return instant, ok, ok
}

// FormatEnvelopeTime 写出宿主交给插件的 Envelope.time：与截止时间的写法相同（FormatDeadline），换算成 UTC、七位小数、+00:00。
func FormatEnvelopeTime(instant time.Time) string {
	return FormatDeadline(instant)
}

// TruncateEnvelopeTime 把 Envelope.time 的时刻截到 SDK 时间类型的精度（invocation.md §4「SDK 读宿主写来的」，ADR-0306、ADR-0310）：
// precision 是 1 秒的约数（规范列出纳秒、100 纳秒、微秒与毫秒四种），比它更细的小数截去、不进位，
// 得到的时刻不晚于原来的时刻，纪元之前也一样；结果是 UTC。precision 不大于 0 时只换算成 UTC。
func TruncateEnvelopeTime(instant time.Time, precision time.Duration) time.Time {
	// Truncate 按距零时刻（0001-01-01 UTC）的整倍数向下取，零时刻是整秒，所以 precision 整除 1 秒时只截秒的小数
	return instant.UTC().Truncate(precision)
}

// FormatSDKEnvelopeTime 写出 SDK 交给宿主的 Envelope.time（invocation.md §4「SDK 写给宿主的」，ADR-0306、ADR-0310）：
// 先按 TruncateEnvelopeTime 截到时间类型的精度，再写成 UTC 的 YYYY-MM-DDTHH:MM:SS、秒的小数与大写的 Z，
// 小数去掉末尾的 0、全为 0 时不写小数点，所以最多写到精度的位数。它与宿主的写法（FormatEnvelopeTime）不同，两种都合 ReadEnvelopeTime 的文法。
// 作者没有给时间时 SDK 写空串，不经这里；不核对年份范围，0001–9999 之外的时刻写成什么规范不作规定。
func FormatSDKEnvelopeTime(instant time.Time, precision time.Duration) string {
	// RFC3339Nano 的小数是 .999999999：去掉末尾的 0，全为 0 时连小数点也不写；UTC 的偏移写成 Z
	return TruncateEnvelopeTime(instant, precision).Format(time.RFC3339Nano)
}
