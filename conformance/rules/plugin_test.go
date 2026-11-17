package rules_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"
	"unicode"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestWhitespaceIsTheUnicodeWhiteSpaceProperty(t *testing.T) {
	// handshake.md §2 列出的集合就是 Unicode 的 White_Space，Go 的 unicode.IsSpace 按同一属性判定
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if rules.IsWhitespace(r) != unicode.IsSpace(r) {
			t.Errorf("U+%04X: IsWhitespace = %v, unicode.IsSpace = %v", r, rules.IsWhitespace(r), unicode.IsSpace(r))
		}
	}
	if rules.IsWhitespace(0x200B) || rules.IsWhitespace(0x180E) || rules.IsWhitespace(0x1C) {
		t.Error("U+200B, U+180E and U+001C are not whitespace")
	}
}

func TestHostAddressesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.HostAddresses()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		endpoints, ok := rules.ParseHostAddresses(c.Value)
		if !ok {
			t.Errorf("%q must parse", c.Value)
			continue
		}
		if !reflect.DeepEqual(endpoints, endpointsOf(c.Entries)) {
			t.Errorf("%q parsed as %+v, want %+v", c.Value, endpoints, c.Entries)
		}
		// 写出再读回得到同一个值
		if written := rules.FormatHostAddresses(endpoints); written != c.Value {
			t.Errorf("%q written back as %q", c.Value, written)
		}
	}
	for _, value := range vectors.Invalid {
		if _, ok := rules.ParseHostAddresses(value); ok {
			t.Errorf("%q must not parse", value)
		}
	}
	for _, c := range vectors.Selection {
		// 变量缺失与空串一样没有可选的项
		value := ""
		if c.Value != nil {
			value = *c.Value
		}
		selected, ok := rules.SelectHostEndpoint(value, c.Supported)
		switch {
		case c.Selected == nil && ok:
			t.Errorf("%q with %v must select nothing, got %+v", value, c.Supported, selected)
		case c.Selected != nil && (!ok || selected != (rules.Endpoint{Network: c.Selected.Network, Address: c.Selected.Address})):
			t.Errorf("%q with %v selected %+v (ok=%v), want %+v", value, c.Supported, selected, ok, *c.Selected)
		}
	}
}

func TestDeadlinesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.Deadlines()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		want := mustTime(t, c.UTC)
		got, ok := rules.ParseDeadline(c.Text)
		if !ok || !got.Equal(want) || got.Location() != time.UTC {
			t.Errorf("%q parsed as %s (ok=%v), want %s", c.Text, got.Format(time.RFC3339Nano), ok, c.UTC)
		}
	}
	for _, text := range vectors.Invalid {
		if got, ok := rules.ParseDeadline(text); ok {
			t.Errorf("%q must not parse, got %s", text, got.Format(time.RFC3339Nano))
		}
	}
	if _, ok := rules.ParseDeadline(""); ok {
		t.Error("an empty deadline means no deadline")
	}
	for _, c := range vectors.Effective {
		var grpc time.Time
		if c.GRPC != nil {
			grpc = mustTime(t, *c.GRPC)
		}
		got, ok := rules.EffectiveDeadline(c.Metadata, grpc, c.GRPC != nil)
		switch {
		case c.Expected == nil && ok:
			t.Errorf("metadata %q, grpc %v: must have no deadline, got %s", c.Metadata, c.GRPC, got.Format(time.RFC3339Nano))
		case c.Expected != nil && (!ok || !got.Equal(mustTime(t, *c.Expected))):
			t.Errorf("metadata %q, grpc %v: effective %s (ok=%v), want %s", c.Metadata, c.GRPC, got.Format(time.RFC3339Nano), ok, *c.Expected)
		}
	}
	for _, c := range vectors.Written {
		if got := rules.FormatDeadline(mustTime(t, c.UTC)); got != c.Text {
			t.Errorf("%s written as %q, want %q", c.UTC, got, c.Text)
		}
		// 写出的文本按同一文法读回同一时刻
		if parsed, ok := rules.ParseDeadline(c.Text); !ok || !parsed.Equal(mustTime(t, c.UTC)) {
			t.Errorf("%q must read back as %s", c.Text, c.UTC)
		}
	}
	// 写出时比 100 纳秒更细的部分截断、不进位，其他时区先换算成 UTC
	if got := rules.FormatDeadline(time.Date(2026, 11, 18, 11, 25, 0, 999_999_999, time.FixedZone("UTC+8", 8*3600))); got != "2026-11-18T03:25:00.9999999+00:00" {
		t.Errorf("written as %q", got)
	}
}

func TestCallbackStatusFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.CallbackStatus()
	if err != nil {
		t.Fatal(err)
	}
	// details 的 id 是插件的清单 ID（invocation.md §8），操作是合法的操作 ID
	if !rules.IsManifestID(vectors.PluginID) || !rules.IsOperationID(vectors.Operation) {
		t.Fatalf("plugin %q and operation %q must be a manifest ID and an operation ID", vectors.PluginID, vectors.Operation)
	}
	for _, c := range vectors.Cases {
		name := c.RPC + " " + c.Status
		if got := rules.GRPCStatusName(c.StatusCode); got != c.Status {
			t.Errorf("%s: status code %d is named %q", name, c.StatusCode, got)
		}
		failure := rules.MapCallbackStatus(c.RPC, c.StatusCode, c.Message, c.ContextCancelled, vectors.PluginID, vectors.Operation)
		if failure.Code != c.Code || failure.Key != c.Key {
			t.Errorf("%s (cancelled=%v): mapped to %s %s, want %s %s", name, c.ContextCancelled, failure.Code, failure.Key, c.Code, c.Key)
		}
		// details 按 JSON 值比较，null 即没有
		var want map[string]string
		if err := json.Unmarshal(c.Details, &want); err != nil {
			t.Fatalf("%s: details: %v", name, err)
		}
		if !reflect.DeepEqual(failure.Details, want) {
			t.Errorf("%s (cancelled=%v): details %v, want %s", name, c.ContextCancelled, failure.Details, c.Details)
		}
	}
	if rules.GRPCStatusName(17) != "17" || rules.CallbackPermission("other") != "" {
		t.Error("status names and permissions outside the spec")
	}
}

func TestPluginMessagesAreConsistent(t *testing.T) {
	vectors, err := conformance.PluginMessages()
	if err != nil {
		t.Fatal(err)
	}
	arguments := map[string][]string{}
	for _, message := range vectors.Messages {
		if !rules.IsErrorCode(message.Code) {
			t.Errorf("%s: code %q is not valid", message.Key, message.Code)
		}
		if !rules.IsMessageKey(message.Key) || arguments[message.Key] != nil {
			t.Errorf("key %q must be valid and unique", message.Key)
		}
		if message.Template == "" || message.When == "" {
			t.Errorf("%s: template and when must not be empty", message.Key)
		}
		// 参数就是模板的占位符，顺序相同；错误的 details 正好是这些成员（invocation.md §8）
		placeholders := rules.Placeholders(message.Template)
		if !slices.Equal(placeholders, message.Arguments) && !(len(placeholders) == 0 && len(message.Arguments) == 0) {
			t.Errorf("%s: placeholders %v differ from arguments %v", message.Key, placeholders, message.Arguments)
		}
		if message.Arguments == nil {
			t.Errorf("%s: arguments must be listed, even when empty", message.Key)
		}
		arguments[message.Key] = message.Arguments
	}
	for _, reason := range vectors.Reasons {
		if !slices.Contains(arguments[reason.Key], "reason") || reason.Reason == "" || reason.When == "" {
			t.Errorf("reason %q must belong to a message with a reason argument", reason.Reason)
		}
		if (reason.Phase != "") != (reason.Key == "plugins.process.failed") {
			t.Errorf("reason %q: only plugins.process.failed has a phase", reason.Reason)
		}
	}

	// 回调映射出的错误：插件的键在这份向量里，内核的键在 kernel-messages.json 里，details 的成员与参数一致
	kernel, err := conformance.KernelMessages()
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range kernel.Messages {
		if _, exists := arguments[message.Key]; !exists {
			arguments[message.Key] = message.Arguments
		}
	}
	callbacks, err := conformance.CallbackStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range callbacks.Cases {
		expected, known := arguments[c.Key]
		if !known {
			t.Errorf("%s %s: key %q is in neither plugin-messages.json nor kernel-messages.json", c.RPC, c.Status, c.Key)
			continue
		}
		var details map[string]string
		if err := json.Unmarshal(c.Details, &details); err != nil {
			t.Fatalf("%s %s: details: %v", c.RPC, c.Status, err)
		}
		names := make([]string, 0, len(details))
		for name := range details {
			names = append(names, name)
		}
		slices.Sort(names)
		sorted := slices.Sorted(slices.Values(expected))
		if !slices.Equal(names, sorted) && !(len(names) == 0 && len(sorted) == 0) {
			t.Errorf("%s %s: details %v must have exactly the arguments %v of %s", c.RPC, c.Status, names, expected, c.Key)
		}
	}
}

// endpointsOf 把向量里的端点换成规则包的类型。
func endpointsOf(entries []conformance.HostEndpoint) []rules.Endpoint {
	endpoints := make([]rules.Endpoint, 0, len(entries))
	for _, entry := range entries {
		endpoints = append(endpoints, rules.Endpoint{Network: entry.Network, Address: entry.Address})
	}
	return endpoints
}

// mustTime 读向量里 RFC 3339 写法的 UTC 时刻。
func mustTime(t *testing.T, text string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		t.Fatalf("vector time %q: %v", text, err)
	}
	return value
}
