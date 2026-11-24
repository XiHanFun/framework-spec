package rules

import "time"

// MaxJobNameLength 是调度里名字的最大长度（RFC-0013「作业与触发器」）；与消息主题同一条文法。
const MaxJobNameLength = 128

// MaxJobRetries 是重试策略里重试次数的上限（RFC-0013「作业与触发器」的重试）。
const MaxJobRetries = 100

// JobRetryDelayLimit 是重试的等待时间与上限的上限：30 天。
const JobRetryDelayLimit = 30 * 24 * time.Hour

// SchedulingEventSource 是调度事件信封的 source（RFC-0013「引擎」）。
const SchedulingEventSource = "/scheduling"

// SchedulingEventTopics 是调度发布的事件的主题，也是信封的 type（RFC-0013「引擎」）。
var SchedulingEventTopics = []string{
	"scheduling.job.scheduled",
	"scheduling.job.unscheduled",
	"scheduling.job.paused",
	"scheduling.job.resumed",
	"scheduling.run.started",
	"scheduling.run.succeeded",
	"scheduling.run.failed",
	"scheduling.run.canceled",
	"scheduling.run.lost",
}

// IsJobName 判断作业名、触发器名或领导选举的资源名是否合文法（RFC-0013「作业与触发器」）：与主题文法相同。
func IsJobName(name string) bool { return topicPattern.MatchString(name) }

// IsJobRetryPolicy 判断重试策略的取值是否合法（RFC-0013「作业与触发器」的重试）：重试次数在 1 到 MaxJobRetries 之间，
// 等待大于零且不超过 30 天；maxDelay 为 nil 表示不设上限，给了的话同样大于零且不超过 30 天。
func IsJobRetryPolicy(maxRetries int, delay time.Duration, maxDelay *time.Duration) bool {
	inRange := func(wait time.Duration) bool { return wait > 0 && wait <= JobRetryDelayLimit }
	return maxRetries >= 1 && maxRetries <= MaxJobRetries && inRange(delay) && (maxDelay == nil || inRange(*maxDelay))
}

// JobRetryWait 给出第 retry 次重试前的等待（RFC-0013「作业与触发器」的重试）：固定间隔每次都等 delay；
// 指数退避第 n 次等 delay 乘以 2 的 n − 1 次方，超过上限取上限，没设上限（maxDelay 为 nil）时超过 30 天按 30 天。retry 从 1 起。
func JobRetryWait(exponential bool, delay time.Duration, maxDelay *time.Duration, retry int) time.Duration {
	if !exponential {
		return delay
	}
	limit := JobRetryDelayLimit
	if maxDelay != nil {
		limit = *maxDelay
	}
	wait := delay
	// 逐次翻倍并在到达上限时停下：次数再多也不会溢出
	for i := 1; i < retry && wait < limit; i++ {
		if wait > limit/2 {
			return limit
		}
		wait *= 2
	}
	return min(wait, limit)
}
