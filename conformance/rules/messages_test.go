package rules_test

import (
	"encoding/json"
	"reflect"
	"slices"
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
		if len(arguments) > 1 {
			t.Errorf("describe %q has %d arguments; at most one keeps the order out of the vectors", c.Key, len(arguments))
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
			var want any
			if err := json.Unmarshal(expected, &want); err != nil {
				t.Fatalf("%s: canonical: %v", c.Name, err)
			}
			// 规范形式按 JSON 值比较：先过一遍编码，消掉 Go 类型与 JSON 解码结果之间的差别
			var got any
			text, _ := json.Marshal(canonical)
			_ = json.Unmarshal(text, &got)
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
