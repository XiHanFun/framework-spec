package rules_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestTopicsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.Topics()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxTopicLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxTopicLength)
	}
	atLimit, overLimit := false, false
	for _, topic := range vectors.Valid {
		if !rules.IsTopic(topic) {
			t.Errorf("%q must be a valid topic", topic)
		}
		atLimit = atLimit || len(topic) == vectors.MaxLength
	}
	for _, topic := range vectors.Invalid {
		if rules.IsTopic(topic) {
			t.Errorf("%q must be an invalid topic", topic)
		}
		overLimit = overLimit || len(topic) == vectors.MaxLength+1
	}
	// 上限两侧各要有一条：正好到上限的合法主题与只多一个字符的不合法主题
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep a topic of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}

	if vectors.DeadLetter.DefaultSuffix != rules.DefaultDeadLetterSuffix {
		t.Errorf("default dead-letter suffix = %q, want %q", vectors.DeadLetter.DefaultSuffix, rules.DefaultDeadLetterSuffix)
	}
	for _, c := range vectors.DeadLetter.Cases {
		suffix := ""
		if c.Suffix != nil {
			suffix = *c.Suffix
		}
		if got := rules.DeadLetterTopic(c.Topic, suffix); got != c.DeadLetter {
			t.Errorf("dead-letter topic of %q with suffix %q = %q, want %q", c.Topic, suffix, got, c.DeadLetter)
		}
	}
}

func TestEnvelopeExtensionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.EnvelopeExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Names.MaxLength != rules.MaxEnvelopeExtensionNameLength {
		t.Fatalf("names.maxLength = %d, want %d", vectors.Names.MaxLength, rules.MaxEnvelopeExtensionNameLength)
	}
	for _, name := range vectors.Names.Valid {
		if !rules.IsEnvelopeExtensionName(name) {
			t.Errorf("%q must be a valid extension attribute name", name)
		}
	}
	for _, name := range vectors.Names.Invalid {
		if rules.IsEnvelopeExtensionName(name) {
			t.Errorf("%q must be an invalid extension attribute name", name)
		}
	}
	// 保留名与规则包的清单一致，且每一个都在必须拒绝的样例里
	if !slices.Equal(vectors.Names.Reserved, rules.EnvelopeExtensionReservedNames) {
		t.Errorf("names.reserved = %v, want %v", vectors.Names.Reserved, rules.EnvelopeExtensionReservedNames)
	}
	for _, name := range vectors.Names.Reserved {
		if !slices.Contains(vectors.Names.Invalid, name) {
			t.Errorf("reserved name %q must also be listed as invalid", name)
		}
	}

	var mappings []rules.EnvelopeBaggageMapping
	for _, mapping := range vectors.Baggage {
		mappings = append(mappings, rules.EnvelopeBaggageMapping{Extension: mapping.Extension, BaggageKey: mapping.BaggageKey})
		// 对应表两边都得是合法的名字，Baggage 键还得已登记
		if !rules.IsEnvelopeExtensionName(mapping.Extension) || !rules.IsRegisteredBaggageKey(mapping.BaggageKey) {
			t.Errorf("mapping %s -> %s must pair an extension attribute name with a registered baggage key", mapping.Extension, mapping.BaggageKey)
		}
	}
	if !slices.Equal(mappings, rules.EnvelopeBaggageMappings) {
		t.Errorf("baggage mappings = %v, want %v", rules.EnvelopeBaggageMappings, mappings)
	}
	if vectors.Trace != rules.EnvelopeTraceExtension || vectors.PartitionKey != rules.EnvelopePartitionKeyExtension {
		t.Errorf("trace and partition key extensions = %q, %q; want %q, %q", rules.EnvelopeTraceExtension, rules.EnvelopePartitionKeyExtension, vectors.Trace, vectors.PartitionKey)
	}

	// 已登记的 Baggage 键要么有对应的扩展，要么列在 notCarried 里，两者不相交
	for _, key := range rules.RegisteredBaggageKeys {
		mapped := slices.ContainsFunc(mappings, func(m rules.EnvelopeBaggageMapping) bool { return m.BaggageKey == key })
		if mapped == slices.Contains(vectors.NotCarried, key) {
			t.Errorf("baggage key %s must either map to an extension attribute or be listed as not carried", key)
		}
	}
}

func TestMessagingInstanceIDsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.MessagingInstanceIDs()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxMessagingInstanceIDLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxMessagingInstanceIDLength)
	}
	atLimit, overLimit := false, false
	for _, id := range vectors.Valid {
		if !rules.IsMessagingInstanceID(id) {
			t.Errorf("%q must be a valid instance ID", id)
		}
		atLimit = atLimit || len(id) == vectors.MaxLength
	}
	for _, id := range vectors.Invalid {
		if rules.IsMessagingInstanceID(id) {
			t.Errorf("%q must be an invalid instance ID", id)
		}
		overLimit = overLimit || len(id) == vectors.MaxLength+1
	}
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep an instance ID of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}
}

func TestOutboxBackoffFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.OutboxBackoff()
	if err != nil {
		t.Fatal(err)
	}
	millis := func(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }
	if millis(vectors.Defaults.RetryDelayMs) != rules.DefaultOutboxRetryDelay || millis(vectors.Defaults.MaxRetryDelayMs) != rules.DefaultOutboxMaxRetryDelay || vectors.Defaults.MaxAttempts != rules.DefaultOutboxMaxAttempts {
		t.Errorf("defaults = %+v, want %v, %v and %d attempts", vectors.Defaults, rules.DefaultOutboxRetryDelay, rules.DefaultOutboxMaxRetryDelay, rules.DefaultOutboxMaxAttempts)
	}
	for _, c := range vectors.Cases {
		if got := rules.OutboxRetryDelay(millis(c.RetryDelayMs), millis(c.MaxRetryDelayMs), c.Attempts); got != millis(c.DelayMs) {
			t.Errorf("%s: delay = %v, want %v", c.Name, got, millis(c.DelayMs))
		}
	}
	for _, c := range vectors.DeadLetter {
		if got := rules.OutboxDeadLettered(c.Attempts, c.MaxAttempts); got != c.DeadLettered {
			t.Errorf("attempt %d of %d: dead-lettered = %v, want %v", c.Attempts, c.MaxAttempts, got, c.DeadLettered)
		}
	}

	// 缺省参数下的整段时间：前 9 次失败之后的间隔之和，第 10 次进死信（RFC-0009「发件箱」的「约 20 分钟」）
	var total time.Duration
	for attempts := 1; !rules.OutboxDeadLettered(attempts, rules.DefaultOutboxMaxAttempts); attempts++ {
		total += rules.OutboxRetryDelay(rules.DefaultOutboxRetryDelay, rules.DefaultOutboxMaxRetryDelay, attempts)
	}
	if total != 1215*time.Second {
		t.Errorf("a message that keeps failing waits %v in total before it is dead-lettered, want 1215s", total)
	}
}

func TestCloudEventEnvelopesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.CloudEventEnvelopes()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.ContentType != rules.CloudEventsContentType || !rules.IsErrorCode(vectors.Code) {
		t.Fatalf("content type %q and code %q, want %q and a valid error code", vectors.ContentType, vectors.Code, rules.CloudEventsContentType)
	}
	for _, c := range vectors.Valid {
		event, ok := rules.DecodeCloudEvent([]byte(c.Text))
		if !ok {
			t.Errorf("%s: the document must decode", c.Name)
			continue
		}
		want := c.Expected
		if event.ID != want.ID || event.Source != want.Source || event.Type != want.Type || event.DataContentType != want.DataContentType {
			t.Errorf("%s: attributes = %q %q %q %q, want %q %q %q %q", c.Name, event.ID, event.Source, event.Type, event.DataContentType, want.ID, want.Source, want.Type, want.DataContentType)
		}
		if !reflect.DeepEqual(event.Subject, want.Subject) {
			t.Errorf("%s: subject differs", c.Name)
		}
		if !reflect.DeepEqual(event.DataSchema, want.DataSchema) {
			t.Errorf("%s: dataschema differs", c.Name)
		}
		// 时间按时刻比较：文档里可以带偏移，期望值是 UTC
		switch {
		case want.Time == nil && event.Time != nil, want.Time != nil && event.Time == nil:
			t.Errorf("%s: time = %v, want %v", c.Name, event.Time, want.Time)
		case want.Time != nil:
			instant, err := time.Parse(time.RFC3339Nano, *want.Time)
			if err != nil || !instant.Equal(*event.Time) {
				t.Errorf("%s: time = %v, want %s (%v)", c.Name, event.Time, *want.Time, err)
			}
		}
		// 内联数据按 JSON 值比较，二进制数据按字节比较，两者至多一个
		switch {
		case want.Data != nil:
			if event.Binary != nil || !sameJSON(t, event.Data, want.Data) {
				t.Errorf("%s: data = %s, want %s", c.Name, event.Data, want.Data)
			}
		case want.DataBase64 != nil:
			binary, err := base64.StdEncoding.DecodeString(*want.DataBase64)
			if err != nil || event.Data != nil || !bytes.Equal(event.Binary, binary) {
				t.Errorf("%s: binary data = %x, want %x (%v)", c.Name, event.Binary, binary, err)
			}
		default:
			if event.Data != nil || event.Binary != nil {
				t.Errorf("%s: the envelope must carry no data", c.Name)
			}
		}
		if !reflect.DeepEqual(event.Extensions, want.Extensions) {
			t.Errorf("%s: extensions = %v, want %v", c.Name, event.Extensions, want.Extensions)
		}
	}
	for _, c := range vectors.Invalid {
		if _, ok := rules.DecodeCloudEvent([]byte(c.Text)); ok {
			t.Errorf("%s: the document must be rejected", c.Name)
		}
	}
}

// sameJSON 判断两段 JSON 是否是同一个值：不比较空白与成员顺序，数字保留原文。
func sameJSON(t *testing.T, left, right json.RawMessage) bool {
	t.Helper()
	a, errA := rules.Exact(left)
	b, errB := rules.Exact(right)
	return errA == nil && errB == nil && reflect.DeepEqual(a, b)
}
