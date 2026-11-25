package rules_test

import (
	"slices"
	"testing"
	"time"

	// 时区用例要 IANA 时区库：Windows 上没有系统自带的，测试程序内嵌一份
	_ "time/tzdata"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestJobNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.JobNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxJobNameLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxJobNameLength)
	}
	replayGrammar(t, "job name", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsJobName)
	// 与主题同一条文法：两边的判定必须一致
	for _, name := range append(slices.Clone(vectors.Valid), vectors.Invalid...) {
		if rules.IsJobName(name) != rules.IsTopic(name) {
			t.Errorf("%q: job names and topics share one grammar, but the two checks disagree", name)
		}
	}
}

func TestCronExpressionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.CronExpressions()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Cases {
		expression, ok := rules.ParseCron(c.Expression)
		if !ok {
			t.Errorf("%s: %q must parse", c.Name, c.Expression)
			continue
		}
		location := time.UTC
		if c.TimeZone != nil {
			location, err = time.LoadLocation(*c.TimeZone)
			if err != nil {
				t.Errorf("%s: %v", c.Name, err)
				continue
			}
		}
		after, err := time.Parse(time.RFC3339Nano, c.After)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		// 每一个都从前一个推算：严格晚于它
		for index, want := range c.Next {
			next, found := expression.Next(after, location)
			if !found {
				t.Errorf("%s: occurrence %d after %s is missing, want %s", c.Name, index+1, after.Format(time.RFC3339), want)
				break
			}
			if got := next.UTC().Format("2006-01-02T15:04:05Z"); got != want {
				t.Errorf("%s: occurrence %d after %s = %s, want %s", c.Name, index+1, after.Format(time.RFC3339), got, want)
				break
			}
			after = next
		}
		if c.Exhausted {
			if next, found := expression.Next(after, location); found {
				t.Errorf("%s: there must be no occurrence after %s, got %s", c.Name, after.Format(time.RFC3339), next.UTC().Format(time.RFC3339))
			}
		}
	}
	for _, c := range vectors.Invalid {
		if rules.IsCronExpression(c.Expression) {
			t.Errorf("%s: %q must be rejected", c.Name, c.Expression)
		}
	}
}

func TestJobRetryDelaysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.JobRetryDelays()
	if err != nil {
		t.Fatal(err)
	}
	millis := func(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }
	optional := func(ms *int64) *time.Duration {
		if ms == nil {
			return nil
		}
		wait := millis(*ms)
		return &wait
	}
	if vectors.MaxRetries != rules.MaxJobRetries || millis(vectors.LimitMs) != rules.JobRetryDelayLimit {
		t.Fatalf("maxRetries and limit = %d, %v; want %d, %v", vectors.MaxRetries, millis(vectors.LimitMs), rules.MaxJobRetries, rules.JobRetryDelayLimit)
	}
	for _, c := range vectors.Waits {
		if c.Backoff != "Fixed" && c.Backoff != "Exponential" {
			t.Errorf("%s: backoff %q must be Fixed or Exponential", c.Name, c.Backoff)
			continue
		}
		if got := rules.JobRetryWait(c.Backoff == "Exponential", millis(c.DelayMs), optional(c.MaxDelayMs), c.Retry); got != millis(c.WaitMs) {
			t.Errorf("%s: wait = %v, want %v", c.Name, got, millis(c.WaitMs))
		}
		// 用到的取值本身必须合法
		if !rules.IsJobRetryPolicy(1, millis(c.DelayMs), optional(c.MaxDelayMs)) {
			t.Errorf("%s: the delay and the cap must be valid policy values", c.Name)
		}
	}
	for _, c := range vectors.Policies {
		if got := rules.IsJobRetryPolicy(c.MaxRetries, millis(c.DelayMs), optional(c.MaxDelayMs)); got != c.Valid {
			t.Errorf("%s: valid = %v, want %v", c.Name, got, c.Valid)
		}
	}
}

func TestSchedulingEventTopicsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.SchedulingEventTopics()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Source != rules.SchedulingEventSource || !slices.Equal(vectors.Topics, rules.SchedulingEventTopics) {
		t.Errorf("source and topics = %q, %v; want %q, %v", vectors.Source, vectors.Topics, rules.SchedulingEventSource, rules.SchedulingEventTopics)
	}
	for _, topic := range vectors.Topics {
		if !rules.IsTopic(topic) {
			t.Errorf("%q must be a valid topic", topic)
		}
	}
}

func TestJobScheduleTextsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.JobScheduleTexts()
	if err != nil {
		t.Fatal(err)
	}
	instant := func(text *string) *time.Time {
		if text == nil {
			return nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, *text)
		if err != nil {
			t.Fatalf("instant %q: %v", *text, err)
		}
		return &parsed
	}
	sameInstant := func(got, want *time.Time) bool {
		return (got == nil) == (want == nil) && (got == nil || got.Equal(*want))
	}
	sameString := func(got, want *string) bool {
		return (got == nil) == (want == nil) && (got == nil || *got == *want)
	}
	sameInt := func(got, want *int) bool {
		return (got == nil) == (want == nil) && (got == nil || *got == *want)
	}
	for _, c := range vectors.Valid {
		schedule, ok := rules.ParseJobSchedule(c.Text)
		if !ok {
			t.Errorf("%s: %q must parse", c.Name, c.Text)
			continue
		}
		var cron *string
		if schedule.Kind == "cron" {
			cron = &schedule.Cron
		}
		var interval *int64
		if schedule.Kind == "every" {
			interval = &schedule.IntervalTicks
		}
		if schedule.Kind != c.Kind || (interval == nil) != (c.IntervalTicks == nil) || (interval != nil && *interval != *c.IntervalTicks) ||
			!sameInt(schedule.RepeatCount, c.RepeatCount) || !sameString(cron, c.Cron) || !sameString(schedule.TimeZone, c.TimeZone) ||
			!sameInstant(schedule.At, instant(c.At)) || !sameInstant(schedule.StartAt, instant(c.StartAt)) || !sameInstant(schedule.EndAt, instant(c.EndAt)) ||
			!sameString(schedule.Calendar, c.Calendar) {
			t.Errorf("%s: %q parsed into %+v, which does not match the vector", c.Name, c.Text, schedule)
			continue
		}
		if got := rules.FormatJobSchedule(schedule); got != c.Canonical {
			t.Errorf("%s: written back as %q, want %q", c.Name, got, c.Canonical)
		}
		// 写回的文本本身合法，读回再写出不变
		again, ok := rules.ParseJobSchedule(c.Canonical)
		if !ok || rules.FormatJobSchedule(again) != c.Canonical {
			t.Errorf("%s: the text written back %q must read back to itself", c.Name, c.Canonical)
		}
	}
	for _, c := range vectors.Invalid {
		if _, ok := rules.ParseJobSchedule(c.Text); ok {
			t.Errorf("%s: %q must be rejected", c.Name, c.Text)
		}
	}
}

func TestJobScheduleTimeZonesComeFromTheTimeZoneDatabase(t *testing.T) {
	// 向量只用不依赖时区库的 UTC；这里另核对查得到与查不到的 IANA 名、Local 与 Windows 的时区 ID
	for name, want := range map[string]bool{"UTC": true, "Asia/Shanghai": true, "Etc/GMT-8": true, "America/Argentina/Buenos_Aires": true, "Mars/Olympus": false, "Local": false, "China Standard Time": false} {
		if got := rules.IsKnownTimeZone(name); got != want {
			t.Errorf("IsKnownTimeZone(%q) = %v, want %v", name, got, want)
		}
	}
}
