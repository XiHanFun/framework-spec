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
