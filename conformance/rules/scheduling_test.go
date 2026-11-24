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
