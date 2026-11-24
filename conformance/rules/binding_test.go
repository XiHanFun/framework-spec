package rules_test

import (
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/internal/pluginv1"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestBindingPropertiesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.PluginBindings()
	if err != nil {
		t.Fatal(err)
	}
	limits := vectors.Limits
	if limits.MaxProperties != rules.MaxBindingProperties || limits.MaxNameLength != rules.MaxBindingPropertyNameLength || limits.MaxBytes != rules.MaxBindingPropertyBytes {
		t.Errorf("limits %+v differ from the rules", limits)
	}
	for _, name := range vectors.Names.Valid {
		if !rules.IsBindingPropertyName(name) {
			t.Errorf("property name %q must be accepted", name)
		}
	}
	for _, name := range vectors.Names.Invalid {
		if rules.IsBindingPropertyName(name) {
			t.Errorf("property name %q must be rejected", name)
		}
	}
	// 上限两侧的名字都在样例里
	if !slices.ContainsFunc(vectors.Names.Valid, func(name string) bool { return len(name) == limits.MaxNameLength }) ||
		!slices.ContainsFunc(vectors.Names.Invalid, func(name string) bool { return len(name) == limits.MaxNameLength+1 }) {
		t.Error("the names must include one of exactly the maximum length and one that is one character longer")
	}

	// 固定的 reason 文本登记在 plugin-messages.json 里，{binding} 换成绑定名
	messages, err := conformance.PluginMessages()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Delivery {
		declared := make([]rules.BindingDeclaration, 0, len(c.Declared))
		for _, d := range c.Declared {
			declared = append(declared, rules.BindingDeclaration{Name: d.Name, Type: d.Type})
		}
		resolved := make(map[string]rules.BindingSource, len(c.Resolved))
		for name, r := range c.Resolved {
			resolved[name] = rules.BindingSource{Type: r.Type, Value: r.Value, Properties: r.Properties}
		}
		bindings, warnings, reason := rules.DeliverBindings(declared, resolved)
		if c.Error != nil {
			if c.Bindings != nil || c.Warnings != nil {
				t.Errorf("%s: a case expects either bindings or an error", c.Name)
			}
			want := map[string]string{"id": vectors.PluginID, "phase": "configure", "reason": reason}
			if reason == "" || c.Error.Code != "PLUGINS_UNEXPECTED_001" || c.Error.Key != "plugins.process.failed" || !reflect.DeepEqual(c.Error.Details, want) {
				t.Errorf("%s: failed with reason %q, want %+v", c.Name, reason, c.Error)
			}
			registered := slices.ContainsFunc(messages.Reasons, func(r conformance.PluginMessageReason) bool {
				return r.Key == c.Error.Key && r.Phase == "configure" && slices.ContainsFunc(c.Declared, func(d conformance.DeclaredBinding) bool {
					return strings.ReplaceAll(r.Reason, "{binding}", d.Name) == reason
				})
			})
			if !registered {
				t.Errorf("%s: reason %q is not one of the fixed reasons in plugin-messages.json", c.Name, reason)
			}
			continue
		}
		if c.Bindings == nil || c.Warnings == nil {
			t.Errorf("%s: a delivered case must list its bindings and warnings, even when empty", c.Name)
		}
		if reason != "" || !reflect.DeepEqual(deliveredOf(bindings), c.Bindings) || !slices.Equal(warnings, c.Warnings) {
			t.Errorf("%s: delivered %d bindings, warnings %v, reason %q; want %d bindings, warnings %v", c.Name, len(bindings), warnings, reason, len(c.Bindings), c.Warnings)
		}
	}
}

func TestBindingPropertiesAreReadFromTheWire(t *testing.T) {
	vectors, err := conformance.PluginBindings()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Reading {
		raw, err := hex.DecodeString(c.Hex)
		if err != nil {
			t.Fatalf("%s: hex: %v", c.Name, err)
		}
		// 生成的消息类型读出绑定连同属性：生成物没有跟上 plugin.proto 时这里读不到 properties
		var binary pluginv1.ConfigureRequest
		if err := proto.Unmarshal(raw, &binary); err != nil {
			t.Fatalf("%s: unmarshal: %v", c.Name, err)
		}
		if got := readBindings(&binary); !reflect.DeepEqual(got, c.Bindings) {
			t.Errorf("%s: the bytes read as %+v, want %+v", c.Name, got, c.Bindings)
		}
		// Wasm 载体的 proto3 JSON 文本读出同一个请求
		var text pluginv1.ConfigureRequest
		if err := protojson.Unmarshal([]byte(c.JSON), &text); err != nil {
			t.Fatalf("%s: json: %v", c.Name, err)
		}
		if !proto.Equal(&binary, &text) {
			t.Errorf("%s: the JSON text and the bytes are different requests", c.Name)
		}
		// 按较早的规范实现的插件只认 name、type、value，properties 是未知字段：跳过它，其余三项读到的不变
		legacy, err := readLegacyBindings(raw)
		if err != nil {
			t.Fatalf("%s: legacy read: %v", c.Name, err)
		}
		want := make([]conformance.DeliveredBinding, 0, len(c.Bindings))
		for _, binding := range c.Bindings {
			want = append(want, conformance.DeliveredBinding{Name: binding.Name, Type: binding.Type, Value: binding.Value})
		}
		if !reflect.DeepEqual(legacy, want) {
			t.Errorf("%s: a plugin without the properties field reads %+v, want %+v", c.Name, legacy, want)
		}
	}
}

// deliveredOf 把规则算出的绑定换成向量的形态。
func deliveredOf(bindings []rules.PluginBinding) []conformance.DeliveredBinding {
	delivered := make([]conformance.DeliveredBinding, 0, len(bindings))
	for _, binding := range bindings {
		delivered = append(delivered, conformance.DeliveredBinding{Name: binding.Name, Type: binding.Type, Value: binding.Value, Properties: binding.Properties})
	}
	return delivered
}

// readBindings 按 lifecycle.md §7.2 把请求里的绑定交给作者：没有属性时是空表，不是 nil。
func readBindings(request *pluginv1.ConfigureRequest) []conformance.DeliveredBinding {
	bindings := make([]conformance.DeliveredBinding, 0, len(request.GetBindings()))
	for _, binding := range request.GetBindings() {
		properties := conformance.BindingProperties{}
		for name, value := range binding.GetProperties() {
			properties[name] = value
		}
		bindings = append(bindings, conformance.DeliveredBinding{Name: binding.GetName(), Type: binding.GetType(), Value: binding.GetValue(), Properties: properties})
	}
	return bindings
}

// readLegacyBindings 以只认 Binding 的 name、type、value（字段 1 到 3）的读法读 ConfigureRequest 的字节，其余字段跳过。
func readLegacyBindings(request []byte) ([]conformance.DeliveredBinding, error) {
	bindings := []conformance.DeliveredBinding{}
	for len(request) > 0 {
		number, kind, n := protowire.ConsumeTag(request)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		request = request[n:]
		// ConfigureRequest.bindings 是字段 3
		if number != 3 || kind != protowire.BytesType {
			if n = protowire.ConsumeFieldValue(number, kind, request); n < 0 {
				return nil, protowire.ParseError(n)
			}
			request = request[n:]
			continue
		}
		message, n := protowire.ConsumeBytes(request)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		request = request[n:]
		var binding conformance.DeliveredBinding
		for len(message) > 0 {
			number, kind, n := protowire.ConsumeTag(message)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			message = message[n:]
			if number > 3 || kind != protowire.BytesType {
				if n = protowire.ConsumeFieldValue(number, kind, message); n < 0 {
					return nil, protowire.ParseError(n)
				}
				message = message[n:]
				continue
			}
			value, n := protowire.ConsumeBytes(message)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			message = message[n:]
			switch number {
			case 1:
				binding.Name = string(value)
			case 2:
				binding.Type = string(value)
			case 3:
				binding.Value = string(value)
			}
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}
