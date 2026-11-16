package rules_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestMessageKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.MessageTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range vectors.Keys.Valid {
		if !rules.IsMessageKey(key) {
			t.Errorf("%q must be a valid message key", key)
		}
	}
	for _, key := range vectors.Keys.Invalid {
		if rules.IsMessageKey(key) {
			t.Errorf("%q must be an invalid message key", key)
		}
	}
	for _, c := range vectors.Describe {
		if !rules.IsMessageKey(c.Key) {
			t.Errorf("describe key %q must be valid", c.Key)
		}
		var arguments map[string]any
		if err := json.Unmarshal(c.Arguments, &arguments); err != nil {
			t.Errorf("describe %q arguments: %v", c.Key, err)
		}
	}
}

func TestTemplateCasesAreWellFormed(t *testing.T) {
	vectors, err := conformance.MessageTemplates()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range vectors.Format {
		if c.Name == "" || names[c.Name] {
			t.Errorf("format case %q must have a unique name", c.Name)
		}
		names[c.Name] = true
		var arguments map[string]any
		if err := json.Unmarshal(c.Arguments, &arguments); err != nil {
			t.Errorf("%s: arguments: %v", c.Name, err)
		}
		missing := false
		for _, name := range rules.Placeholders(c.Template) {
			if _, ok := arguments[name]; !ok {
				missing = true
			}
		}
		// 有占位符缺值时严格格式化必然失败，没有缺值时必然成功
		if missing != (c.Strict == nil) {
			t.Errorf("%s: strict must be null exactly when a placeholder has no value", c.Name)
		}
		if c.Strict != nil && *c.Strict != c.Lenient {
			t.Errorf("%s: with every value present, strict and lenient must agree", c.Name)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	cases := map[string][]string{
		"Operation '{id}' of '{owner}'": {"id", "owner"},
		"{0} of {1} {0}":                {"0", "1"},
		"{{literal}} {x:N2}":            {"x"},
		"{\"a\":1} {b}":                 {"b"},
		"a {b":                          nil,
		"{1a} {_x} {ok}":                {"ok"},
	}
	for template, want := range cases {
		if got := rules.Placeholders(template); !slices.Equal(got, want) {
			t.Errorf("Placeholders(%q) = %v, want %v", template, got, want)
		}
	}
}

func TestKernelMessagesAreConsistent(t *testing.T) {
	vectors, err := conformance.KernelMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors.Messages) == 0 {
		t.Fatal("no kernel messages")
	}
	keys := map[string]bool{}
	for _, message := range vectors.Messages {
		if !rules.IsErrorCode(message.Code) {
			t.Errorf("%s: code %q is not valid", message.Key, message.Code)
		}
		if !rules.IsMessageKey(message.Key) || keys[message.Key] {
			t.Errorf("key %q must be valid and unique", message.Key)
		}
		keys[message.Key] = true
		if message.Template == "" || message.When == "" {
			t.Errorf("%s: template and when must not be empty", message.Key)
		}
		// 参数名与模板的占位符一一对应、顺序相同，SDK 可以据此核对自己的默认模板
		if placeholders := rules.Placeholders(message.Template); !slices.Equal(placeholders, message.Arguments) && !(len(placeholders) == 0 && len(message.Arguments) == 0) {
			t.Errorf("%s: placeholders %v differ from arguments %v", message.Key, placeholders, message.Arguments)
		}
	}
}

func TestBaggageValuesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BaggageKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors.Values.Acceptable) == 0 || len(vectors.Values.Unacceptable) == 0 {
		t.Fatal("baggage values are missing")
	}
	for _, value := range vectors.Values.Acceptable {
		if !rules.IsBaggageValueAcceptable(value) {
			t.Errorf("%q must be acceptable", value)
		}
	}
	for _, value := range vectors.Values.Unacceptable {
		if rules.IsBaggageValueAcceptable(value) {
			t.Errorf("%q must be unacceptable", value)
		}
	}
}

func TestWireEncodingFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.WireEncoding()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors.Results.Valid) == 0 || len(vectors.Results.Invalid) == 0 || len(vectors.Errors.Valid) == 0 || len(vectors.Errors.Invalid) == 0 {
		t.Fatal("wire encoding vectors are incomplete")
	}
	check := func(set conformance.WireCaseSet, read func(conformance.WireCase) (map[string]any, error)) {
		for _, c := range set.Valid {
			canonical, err := read(c)
			if err != nil {
				t.Errorf("%s: must be readable: %v", c.Name, err)
				continue
			}
			expected := c.Canonical
			if len(expected) == 0 {
				expected = c.JSON
			}
			want, err := decodeExact(expected)
			if err != nil {
				t.Fatalf("%s: canonical: %v", c.Name, err)
			}
			// 规范形式按 JSON 值比较：先过一遍编码，消掉 Go 类型与 JSON 解码结果之间的差别；数字按原文比较，超出双精度的整数也要相等
			text, _ := json.Marshal(canonical)
			got, _ := decodeExact(text)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: canonical = %s, want %s", c.Name, text, expected)
			}
		}
		for _, c := range set.Invalid {
			if _, err := read(c); err == nil {
				t.Errorf("%s: must be rejected", c.Name)
			}
		}
	}
	check(vectors.Results, func(c conformance.WireCase) (map[string]any, error) { return rules.CanonicalResult(c.Kind, c.JSON) })
	check(vectors.Errors, func(c conformance.WireCase) (map[string]any, error) { return rules.CanonicalError(c.JSON) })
}

func TestTraceContextFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.TraceContext()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Parse.Valid {
		parsed, ok := rules.ParseTraceParent(c.Text)
		if !ok || parsed.TraceID != c.TraceID || parsed.SpanID != c.SpanID || parsed.Sampled != c.Sampled {
			t.Errorf("%q parsed as %+v (ok=%v)", c.Text, parsed, ok)
		}
	}
	for _, text := range vectors.Parse.Invalid {
		if _, ok := rules.ParseTraceParent(text); ok {
			t.Errorf("%q must be invalid", text)
		}
	}
	for _, c := range vectors.Format {
		// 没有标志的上下文按已采样写出
		sampled := c.Sampled == nil || *c.Sampled
		if got := rules.FormatTraceParent(rules.TraceParent{TraceID: c.TraceID, SpanID: c.SpanID, Sampled: sampled}); got != c.Text {
			t.Errorf("format = %q, want %q", got, c.Text)
		}
	}
}

func TestDuplicateMembersAreRejected(t *testing.T) {
	vectors, err := conformance.WireEncoding()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors.Duplicates) == 0 {
		t.Fatal("no duplicate cases")
	}
	for _, c := range vectors.Duplicates {
		var err error
		if c.Target == "result" {
			_, err = rules.CanonicalResult(c.Kind, []byte(c.Text))
		} else {
			_, err = rules.CanonicalError([]byte(c.Text))
		}
		if err == nil {
			t.Errorf("%s: must be rejected", c.Name)
		}
	}
	if err := rules.HasNoDuplicateMembers([]byte(`{"a":{"b":1},"c":[{"b":2},{"b":3}],"b":4}`)); err != nil {
		t.Errorf("same names in different objects are fine: %v", err)
	}
}

func TestJSONTextFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.JSONText()
	if err != nil || len(vectors.Cases) == 0 {
		t.Fatalf("json text: %v", err)
	}
	for _, c := range vectors.Cases {
		data, err := c.Bytes()
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		problem := rules.CheckJSONText(data)
		switch c.Expect {
		case "valid":
			if problem != nil {
				t.Errorf("%s: must pass, got %+v", c.Name, *problem)
			}
		case "syntax":
			if problem == nil || !problem.Syntax || problem.Path != "$" {
				t.Errorf("%s: must be a syntax error, got %+v", c.Name, problem)
			}
		case "duplicate":
			if problem == nil || problem.Syntax || problem.Path != c.Path {
				t.Errorf("%s: must report the duplicate at %q, got %+v", c.Name, c.Path, problem)
			}
		default:
			t.Errorf("%s: unknown expectation %q", c.Name, c.Expect)
		}
	}
}

// decodeExact 把 JSON 解码成可比较的值，数字保留原文
func decodeExact(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	err := decoder.Decode(&value)
	return value, err
}

func TestNewVectorFilesAreWellFormed(t *testing.T) {
	manifests, err := conformance.Manifests()
	if err != nil || len(manifests.Valid) == 0 || len(manifests.Invalid) == 0 {
		t.Fatalf("manifests: %v", err)
	}
	for _, c := range append(append([]conformance.ManifestCase{}, manifests.Valid...), manifests.Invalid...) {
		if (len(c.Manifest) == 0) == (c.Text == nil) {
			t.Errorf("manifest %q must have exactly one of manifest and text", c.Name)
		}
	}
	for _, c := range manifests.Valid {
		if c.Written == "" || strings.Contains(c.Written, "\r") || strings.HasSuffix(c.Written, "\n") {
			t.Errorf("valid manifest %q needs the written text with LF and no trailing newline", c.Name)
		}
	}
	for _, c := range manifests.Invalid {
		if c.Error == nil || c.Error.Code != "KERNEL_CAPABILITY_006" || !rules.IsMessageKey(c.Error.MessageKey) {
			t.Errorf("invalid manifest %q needs the expected error", c.Name)
		}
	}
	pipeline, err := conformance.PipelineOrdering()
	if err != nil || len(pipeline.Cases) == 0 || len(pipeline.Errors) == 0 {
		t.Fatalf("pipeline: %v", err)
	}
	for _, c := range pipeline.Errors {
		if !rules.IsErrorCode(c.Error) || !strings.HasPrefix(c.Error, "KERNEL_PIPELINE_") || c.Message == "" {
			t.Errorf("pipeline error %q has code %q", c.Name, c.Error)
		}
	}
	reasons, err := conformance.PayloadReasons()
	if err != nil || len(reasons.Cases) == 0 {
		t.Fatalf("payload reasons: %v", err)
	}
	for _, c := range reasons.Cases {
		if _, ok := reasons.Requests[c.Request]; !ok || (c.Reason != nil && !strings.HasPrefix(*c.Reason, "$")) {
			t.Errorf("payload case %q is malformed", c.Name)
		}
		// 共同读取检查不通过的载荷，reason 就是检查给出的路径
		if problem := rules.CheckJSONText([]byte(c.Payload)); problem != nil && c.Payload != "" && (c.Reason == nil || *c.Reason != problem.Path) {
			t.Errorf("payload case %q: the JSON text check gives %q", c.Name, problem.Path)
		}
	}
}
