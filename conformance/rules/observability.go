package rules

import "time"

// 运行信息的采样间隔（RFC-0023「采样」）。
const (
	// DefaultHostSampleInterval 是采样间隔的缺省值。
	DefaultHostSampleInterval = 5 * time.Second
	// MaxHostSampleInterval 是采样间隔的上限。
	MaxHostSampleInterval = time.Hour
)

// IsHostSampleInterval 判断采样间隔是否在取值范围内：大于 0，不超过一小时。
func IsHostSampleInterval(interval time.Duration) bool {
	return interval > 0 && interval <= MaxHostSampleInterval
}

// HostCPUTimes 是一次读到的系统 CPU 累计时间：空闲与总计，单位相同即可。
type HostCPUTimes struct {
	Idle  uint64
	Total uint64
}

// hostPercent 把比例写成百分比，限制在 0 到 100 之间，不取整。
func hostPercent(ratio float64) float64 {
	return min(max(ratio*100, 0), 100)
}

// HostCPUUsage 由先后两次采样算出系统 CPU 使用率（RFC-0023「采样」）：1 − 空闲增量 / 总增量。
// 任一次没有读到（nil）、总增量为零或计数变小返回假，该项为空。
func HostCPUUsage(before, after *HostCPUTimes) (float64, bool) {
	if before == nil || after == nil || after.Total <= before.Total || after.Idle < before.Idle {
		return 0, false
	}
	total := after.Total - before.Total
	// 空闲增量不会比总增量大；读到的两个计数不同步时按全闲算
	idle := min(after.Idle-before.Idle, total)
	return hostPercent(1 - float64(idle)/float64(total)), true
}

// HostProcessCPUUsage 由先后两次采样算出进程 CPU 使用率（RFC-0023「采样」）：进程处理器时间增量 /（经过时间 × 逻辑核心数）。
// 处理器时间变小或经过时间不为正返回假。
func HostProcessCPUUsage(cores int, elapsed, before, after time.Duration) (float64, bool) {
	if elapsed <= 0 || after < before {
		return 0, false
	}
	return hostPercent(float64(after-before) / (float64(elapsed) * float64(cores))), true
}

// HostRate 由先后两次读到的累计计数算出每秒的增量（RFC-0023「采样」的网卡速率）：增量 / 经过时间。
// 任一次没有读到（nil）、计数变小或经过时间不为正返回假。
func HostRate(before, after *int64, elapsed time.Duration) (float64, bool) {
	if before == nil || after == nil || *after < *before || elapsed <= 0 {
		return 0, false
	}
	return float64(*after-*before) / elapsed.Seconds(), true
}

// HostUsage 由总量与剩余量算出已用与使用率（RFC-0023「契约」的内存与磁盘）：已用 = 总量 − 剩余，使用率 = 已用 / 总量。
// 任一个没有读到（nil）时已用为空；总量不为正时使用率为空。
func HostUsage(total, remaining *int64) (used *int64, usagePercent *float64) {
	if total == nil || remaining == nil {
		return nil, nil
	}
	difference := *total - *remaining
	if *total <= 0 {
		return &difference, nil
	}
	// 不限制在 0 到 100：总量与剩余来自同一次读取，已用不会为负；这里也不取整
	percent := float64(difference) * 100 / float64(*total)
	return &difference, &percent
}
