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
	// RFC-0001「空白字符」列出的 25 个码位就是 Unicode 的 White_Space，Go 的 unicode.IsSpace 按同一属性判定
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

func TestEnvelopeTimesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.EnvelopeTimes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		got, present, ok := rules.ReadEnvelopeTime(c.Text)
		switch {
		case !ok:
			t.Errorf("%q must be read", c.Text)
		case c.UTC == nil && present:
			t.Errorf("%q means no time, got %s", c.Text, got.Format(time.RFC3339Nano))
		case c.UTC != nil && (!present || !got.Equal(mustTime(t, *c.UTC)) || got.Location() != time.UTC):
			t.Errorf("%q read as %s, want %s", c.Text, got.Format(time.RFC3339Nano), *c.UTC)
		}
	}
	for _, text := range vectors.Invalid {
		if got, _, ok := rules.ReadEnvelopeTime(text); ok {
			t.Errorf("%q must be rejected, got %s", text, got.Format(time.RFC3339Nano))
		}
	}
	if vectors.Error.Code != rules.EnvelopeTimeErrorCode || vectors.Error.Key != rules.EnvelopeTimeErrorKey {
		t.Errorf("error %+v differs from the rules", vectors.Error)
	}
	for _, c := range vectors.Written {
		if c.Instant == nil {
			if c.Text != "" {
				t.Errorf("no time must be written as an empty string, not %q", c.Text)
			}
			continue
		}
		instant := mustTime(t, *c.Instant)
		if got := rules.FormatEnvelopeTime(instant); got != c.Text {
			t.Errorf("%s written as %q, want %q", *c.Instant, got, c.Text)
		}
		// 写给插件的文本按同一文法读回同一时刻
		if read, present, ok := rules.ReadEnvelopeTime(c.Text); !ok || !present || !read.Equal(instant) {
			t.Errorf("%q must read back as %s", c.Text, *c.Instant)
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

func TestHTTPDeadlinesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.HTTPDeadlines()
	if err != nil {
		t.Fatal(err)
	}
	now := mustTime(t, vectors.Now)
	fromHeader, fromTimeout, none := 0, 0, 0
	for _, c := range vectors.Cases {
		for name := range c.Headers {
			if name != rules.DeadlineHeader && name != rules.GRPCTimeoutHeader {
				t.Errorf("%s: unexpected header %q", c.Name, name)
			}
		}
		got, ok := rules.HTTPDeadline(c.Headers[rules.DeadlineHeader], c.Headers[rules.GRPCTimeoutHeader], now)
		switch {
		case c.Deadline == nil && ok:
			t.Errorf("%s: must have no deadline, got %s", c.Name, got.Format(time.RFC3339Nano))
		case c.Deadline != nil && (!ok || !got.Equal(mustTime(t, *c.Deadline)) || got.Location() != time.UTC):
			t.Errorf("%s: deadline %s (ok=%v), want %s", c.Name, got.Format(time.RFC3339Nano), ok, *c.Deadline)
		}
		// 三种结局各数一遍：头读得出、落到 grpc-timeout、没有截止时间
		_, header := rules.ParseDeadline(firstValue(c.Headers[rules.DeadlineHeader]))
		switch {
		case c.Deadline == nil:
			none++
		case header && len(c.Headers[rules.DeadlineHeader]) == 1:
			fromHeader++
		default:
			fromTimeout++
		}
	}
	if fromHeader == 0 || fromTimeout == 0 || none == 0 {
		t.Fatalf("checked %d deadlines from the header, %d from grpc-timeout and %d absent; each must be non-zero", fromHeader, fromTimeout, none)
	}

	// 换算出的时刻超出范围时截到能读出的最晚时刻；八位数的小时装不进 time.Duration
	latest := mustTime(t, "9999-12-31T23:59:59.999999999Z")
	if got, ok := rules.HTTPDeadline(nil, []string{"99999999H"}, now); !ok || !got.Equal(latest) {
		t.Errorf("99999999H from %s = %s (ok=%v), want %s", vectors.Now, got.Format(time.RFC3339Nano), ok, latest.Format(time.RFC3339Nano))
	}
	if got, ok := rules.HTTPDeadline(nil, []string{"1n"}, latest); !ok || !got.Equal(latest) {
		t.Errorf("1n from the latest instant = %s (ok=%v), want the latest instant", got.Format(time.RFC3339Nano), ok)
	}
	for text, want := range map[string]time.Duration{"0S": 0, "5S": 5 * time.Second, "250m": 250 * time.Millisecond, "7u": 7 * time.Microsecond, "9n": 9, "00000002M": 2 * time.Minute, "99999999H": 1<<63 - 1} {
		if got, ok := rules.ParseGRPCTimeout(text); !ok || got != want {
			t.Errorf("grpc-timeout %q = %v (ok=%v), want %v", text, got, ok, want)
		}
	}
	for _, text := range []string{"", "S", "5", "5s", "5 S", " 5S", "5S ", "-5S", "+5S", "5.0S", "123456789S", "5S,5S", "５S"} {
		if got, ok := rules.ParseGRPCTimeout(text); ok {
			t.Errorf("grpc-timeout %q must not parse, got %v", text, got)
		}
	}
}

// firstValue 返回头的第一个值；没有值时为空串。
func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
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

func TestErrorReadingFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.ErrorReading()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsManifestID(vectors.PluginID) {
		t.Fatalf("plugin %q must be a manifest ID", vectors.PluginID)
	}
	for _, c := range vectors.Cases {
		decoded := rules.ReadProtoError(protoError(c.Error), vectors.PluginID)
		compareDecoded(t, c.Name, &decoded, &c.Expected)
	}
	for _, c := range vectors.StatusWithoutError {
		phase := c.Phase
		if c.Reader == rules.ReaderPlugin {
			operation := ""
			if c.Operation != nil {
				operation = *c.Operation
			}
			phase = rules.CallbackPhase(c.RPC, operation)
		}
		name := c.Reader + " " + phase
		compareDecoded(t, name, rules.ReadProtoStatus(false, nil, c.Reader, phase, vectors.PluginID), &c.Expected)
		// 带着 error 时按 error 读，ok 为真时没有错误
		if got := rules.ReadProtoStatus(true, nil, c.Reader, phase, vectors.PluginID); got != nil {
			t.Errorf("%s: an ok status must read as no error, got %+v", name, got)
		}
		withError := rules.ReadProtoStatus(false, &rules.ProtoError{Code: "ORDERS_VALIDATION_001", Text: "boom"}, c.Reader, phase, vectors.PluginID)
		if withError == nil || withError.Code != "ORDERS_VALIDATION_001" || withError.Text != "boom" {
			t.Errorf("%s: a failed status with an error must read the error, got %+v", name, withError)
		}
	}
}

func TestDetailsFollowTheValueModel(t *testing.T) {
	// 64 位范围内整数写法的数是 int64，其余的数（含带小数点或指数的整数值、超出 64 位的整数、下溢）是 float64
	details := rules.ReadDetails(`{"a":1,"b":1.0,"c":1e2,"d":9223372036854775807,"e":9223372036854775808,"f":-0,"g":1e-400,"h":[1,{"i":2.5}],"j":null,"k":true,"l":"x"}`)
	want := map[string]any{
		"a": int64(1), "b": 1.0, "c": 100.0, "d": int64(9223372036854775807), "e": 9223372036854775808.0, "f": int64(0), "g": 0.0,
		"h": []any{int64(1), map[string]any{"i": 2.5}}, "j": nil, "k": true, "l": "x",
	}
	if !reflect.DeepEqual(details, want) {
		t.Errorf("details = %#v, want %#v", details, want)
	}
	for _, text := range []string{"", " ", "{}", " {} ", "{} x", `{"a":1e309}`, `{"a":[-1e309]}`, "[]", `"x"`} {
		if details := rules.ReadDetails(text); details != nil {
			t.Errorf("%q must read as no details, got %v", text, details)
		}
	}
}

// protoError 把向量里的线上 Error 换成规则包的类型。
func protoError(wire conformance.ProtoError) rules.ProtoError {
	converted := rules.ProtoError{Code: wire.Code, MessageKey: wire.MessageKey, Text: wire.Text, Details: wire.Details}
	if wire.Inner != nil {
		inner := protoError(*wire.Inner)
		converted.Inner = &inner
	}
	return converted
}

// compareDecoded 按 JSON 值比较读入的错误与期望：键与文案的 null 即空，细节的数字按 JSON 数值比较。
func compareDecoded(t *testing.T, name string, got *rules.DecodedError, want *conformance.DecodedErrorExpectation) {
	t.Helper()
	gotText, _ := json.Marshal(decodedJSON(got))
	wantText, _ := json.Marshal(expectedJSON(t, want))
	var gotValue, wantValue any
	_ = json.Unmarshal(gotText, &gotValue)
	_ = json.Unmarshal(wantText, &wantValue)
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("%s: read as %s, want %s", name, gotText, wantText)
	}
}

// decodedJSON 把读入的错误写成与向量同形的 JSON 值。
func decodedJSON(decoded *rules.DecodedError) any {
	if decoded == nil {
		return nil
	}
	value := map[string]any{"code": decoded.Code, "key": nil, "text": nil, "details": nil, "inner": decodedJSON(decoded.Inner)}
	if decoded.Key != "" {
		value["key"] = decoded.Key
	}
	if decoded.Text != "" {
		value["text"] = decoded.Text
	}
	if decoded.Details != nil {
		value["details"] = decoded.Details
	}
	return value
}

// expectedJSON 把向量里期望的错误写成 JSON 值。
func expectedJSON(t *testing.T, expected *conformance.DecodedErrorExpectation) any {
	t.Helper()
	if expected == nil {
		return nil
	}
	var details any
	if err := json.Unmarshal(expected.Details, &details); err != nil {
		t.Fatalf("expected details %s: %v", expected.Details, err)
	}
	value := map[string]any{"code": expected.Code, "key": nil, "text": nil, "details": details, "inner": expectedJSON(t, expected.Inner)}
	if expected.Key != nil {
		value["key"] = *expected.Key
	}
	if expected.Text != nil {
		value["text"] = *expected.Text
	}
	return value
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
