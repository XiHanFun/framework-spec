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

// sameOptional 判断两个可空的值是否相同：都为 nil，或都不为 nil 且值相等。
func sameOptional[T comparable](got, want *T) bool {
	return (got == nil) == (want == nil) && (got == nil || *got == *want)
}

func TestLinuxHostReadingsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.HostUsage()
	if err != nil {
		t.Fatal(err)
	}
	linux := vectors.Linux

	for _, c := range linux.CPUTimes {
		times, ok := rules.LinuxCPUTimes(c.Stat)
		if ok != (c.Times != nil) || (ok && (times.Idle != c.Times.Idle || times.Total != c.Times.Total)) {
			t.Errorf("cpu times %s: read %+v (present %v), want %+v", c.Name, times, ok, c.Times)
		}
	}
	for _, c := range linux.Memory {
		memory := rules.LinuxMemory(c.Meminfo)
		if !sameOptional(memory.Total, c.Memory.TotalBytes) || !sameOptional(memory.Available, c.Memory.AvailableBytes) ||
			!sameOptional(memory.Free, c.Memory.FreeBytes) || !sameOptional(memory.Cached, c.Memory.CachedBytes) {
			t.Errorf("memory %s: the readings differ from the vector", c.Name)
		}
	}
	for _, c := range linux.Uptime {
		uptime, ok := rules.LinuxUptime(c.Uptime)
		if ok != (c.Milliseconds != nil) || (ok && (uptime%time.Millisecond != 0 || uptime.Milliseconds() != *c.Milliseconds)) {
			t.Errorf("uptime %s: read %v (present %v), want %v ms", c.Name, uptime, ok, c.Milliseconds)
		}
	}
	for _, c := range linux.Mounts {
		mounts := rules.LinuxBlockDeviceMounts(c.Mounts)
		same := len(mounts) == len(c.Disks)
		for index := 0; same && index < len(mounts); index++ {
			same = mounts[index] == rules.HostMount{Device: c.Disks[index].Device, MountPoint: c.Disks[index].MountPoint, FileSystem: c.Disks[index].FileSystem}
		}
		if !same {
			t.Errorf("mounts %s: read %+v, want %+v", c.Name, mounts, c.Disks)
		}
	}
	for _, c := range linux.Processors {
		processor := rules.LinuxProcessor(c.Files)
		if !sameOptional(processor.Name, c.Processor.Name) || !sameOptional(processor.Vendor, c.Processor.Vendor) ||
			!sameOptional(processor.PhysicalCores, c.Processor.PhysicalCores) || !sameOptional(processor.FrequencyMHz, c.Processor.FrequencyMhz) ||
			!sameOptional(processor.CacheBytes, c.Processor.CacheBytes) {
			t.Errorf("processor %s: the readings differ from the vector", c.Name)
		}
	}
	for _, c := range linux.GPUs {
		gpus := rules.LinuxGPUs(c.Files)
		same := len(gpus) == len(c.GPUs)
		for index := 0; same && index < len(gpus); index++ {
			got, want := gpus[index], c.GPUs[index]
			same = sameOptional(got.Name, want.Name) && sameOptional(got.Vendor, want.Vendor) && sameOptional(got.VendorID, want.VendorID) &&
				sameOptional(got.DeviceID, want.DeviceID) && sameOptional(got.Driver, want.Driver) && sameOptional(got.DriverVersion, want.DriverVersion) &&
				sameOptional(got.MemoryBytes, want.MemoryBytes) && sameOptional(got.TemperatureCelsius, want.TemperatureCelsius)
		}
		if !same {
			t.Errorf("gpus %s: the readings differ from the vector", c.Name)
		}
	}
}
