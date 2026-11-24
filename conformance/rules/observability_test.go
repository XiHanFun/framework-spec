package rules_test

import (
	"testing"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// sameReading 判断算出的读数与期望是否相同：期望为 nil 表示该项为空。
func sameReading(value float64, present bool, want *float64) bool {
	if want == nil {
		return !present
	}
	return present && value == *want
}

func TestHostUsageFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.HostUsage()
	if err != nil {
		t.Fatal(err)
	}
	millis := func(ms float64) time.Duration { return time.Duration(ms * float64(time.Millisecond)) }
	if millis(float64(vectors.SampleInterval.DefaultMs)) != rules.DefaultHostSampleInterval || millis(float64(vectors.SampleInterval.MaxMs)) != rules.MaxHostSampleInterval {
		t.Errorf("sample interval = %+v, want %v and at most %v", vectors.SampleInterval, rules.DefaultHostSampleInterval, rules.MaxHostSampleInterval)
	}
	for _, ms := range vectors.SampleInterval.Valid {
		if !rules.IsHostSampleInterval(millis(float64(ms))) {
			t.Errorf("a sample interval of %d ms must be in range", ms)
		}
	}
	for _, ms := range vectors.SampleInterval.Invalid {
		if rules.IsHostSampleInterval(millis(float64(ms))) {
			t.Errorf("a sample interval of %d ms must be out of range", ms)
		}
	}

	times := func(sample *conformance.HostCPUSample) *rules.HostCPUTimes {
		if sample == nil {
			return nil
		}
		return &rules.HostCPUTimes{Idle: sample.Idle, Total: sample.Total}
	}
	for _, c := range vectors.CPU {
		if usage, ok := rules.HostCPUUsage(times(c.Before), times(c.After)); !sameReading(usage, ok, c.UsagePercent) {
			t.Errorf("cpu %s: usage = %v (present %v), want %v", c.Name, usage, ok, c.UsagePercent)
		}
	}
	for _, c := range vectors.Process {
		usage, ok := rules.HostProcessCPUUsage(c.Cores, millis(c.ElapsedMs), millis(c.ProcessCPUBeforeMs), millis(c.ProcessCPUAfterMs))
		if !sameReading(usage, ok, c.UsagePercent) {
			t.Errorf("process %s: usage = %v (present %v), want %v", c.Name, usage, ok, c.UsagePercent)
		}
	}
	for _, c := range vectors.Network {
		if rate, ok := rules.HostRate(c.Before, c.After, millis(c.ElapsedMs)); !sameReading(rate, ok, c.BytesPerSecond) {
			t.Errorf("network %s: rate = %v (present %v), want %v", c.Name, rate, ok, c.BytesPerSecond)
		}
	}

	sameBytes := func(got, want *int64) bool { return (got == nil) == (want == nil) && (got == nil || *got == *want) }
	samePercent := func(got, want *float64) bool { return (got == nil) == (want == nil) && (got == nil || *got == *want) }
	for _, c := range vectors.Memory {
		if used, usage := rules.HostUsage(c.TotalBytes, c.AvailableBytes); !sameBytes(used, c.UsedBytes) || !samePercent(usage, c.UsagePercent) {
			t.Errorf("memory %s: used and usage differ from the vector", c.Name)
		}
	}
	for _, c := range vectors.Disks {
		if used, usage := rules.HostUsage(&c.TotalBytes, &c.FreeBytes); used == nil || *used != c.UsedBytes || !samePercent(usage, c.UsagePercent) {
			t.Errorf("disk %s: used and usage differ from the vector", c.Name)
		}
	}
}
