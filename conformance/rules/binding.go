package rules

import (
	"regexp"
	"strconv"
)

// 下发给插件的绑定属性的上限（plugin/lifecycle.md §2，ADR-0285）。
const (
	// MaxBindingProperties 是一个绑定最多带的属性数。
	MaxBindingProperties = 64
	// MaxBindingPropertyNameLength 是属性名的最大字符数（与 Kubernetes Secret 的键相同）。
	MaxBindingPropertyNameLength = 253
	// MaxBindingPropertyBytes 是一个绑定的全部属性名与属性值按 UTF-8 编码后的字节数上限；主值不计入。
	MaxBindingPropertyBytes = 262144
)

// bindingPropertyNamePattern 是下发时属性名的文法：1 到 253 个 ASCII 字母、数字、点、下划线与连字符，不以点开头。
var bindingPropertyNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,252}$`)

// IsBindingPropertyName 判断属性名能不能随 Configure 下发给插件（lifecycle.md §2）。
func IsBindingPropertyName(name string) bool {
	return bindingPropertyNamePattern.MatchString(name)
}

// BindingDeclaration 是清单 bindings 的一项声明：绑定名与清单声明的资源类型。
type BindingDeclaration struct {
	// Name 是绑定名。
	Name string
	// Type 是清单声明的资源类型。
	Type string
}

// BindingSource 是按 RFC-0016 解析出的一个绑定。
type BindingSource struct {
	// Type 是来源声明的类型；空表示来源没有声明。
	Type string
	// Value 是主值；nil 表示来源没有给主值。
	Value *string
	// Properties 是其余属性，属性名到属性值。
	Properties map[string]string
}

// PluginBinding 是 Configure 请求里的一个绑定（plugin.proto 的 Binding）。
type PluginBinding struct {
	// Name 是清单里的绑定名。
	Name string
	// Type 是来源声明的类型，来源没有声明时是清单声明的类型。
	Type string
	// Value 是主值；来源没有给主值时为空。
	Value string
	// Properties 是属性，原样取自解析结果；没有属性时是空表。
	Properties map[string]string
}

// BindingPropertyViolation 给出一个绑定的属性里第一处不能下发的地方，写成错误 reason 的固定英文文本；都合规时为空串。
// 先核对数量、再核对属性名、最后核对字节数：三项都是对整组属性的判断，结果与遍历的先后无关。
// 文本只带绑定名与上限的数字，不带属性名与任何值（lifecycle.md §2）。
func BindingPropertyViolation(binding string, properties map[string]string) string {
	if len(properties) > MaxBindingProperties {
		return "binding '" + binding + "' has more than " + strconv.Itoa(MaxBindingProperties) + " properties"
	}
	for name := range properties {
		if !IsBindingPropertyName(name) {
			return "binding '" + binding + "' has a property name that is not valid"
		}
	}
	bytes := 0
	for name, value := range properties {
		// Go 的字符串长度就是 UTF-8 字节数
		bytes += len(name) + len(value)
	}
	if bytes > MaxBindingPropertyBytes {
		return "the properties of binding '" + binding + "' exceed " + strconv.Itoa(MaxBindingPropertyBytes) + " bytes"
	}
	return ""
}

// DeliverBindings 算出宿主随 Configure 下发的绑定（lifecycle.md §2）：只看清单声明的绑定，按清单的顺序。
// 两种来源都没给的跳过；主值与属性都没有的不下发，绑定名记进 warnings（宿主为它记一条 Warning 日志）；
// 其余的下发，类型取来源声明的、来源没有声明时取清单声明的，主值没有时为空，属性原样。
// 有属性不能下发时 reason 是第一个不合的绑定的 BindingPropertyViolation，这时宿主不发 Configure，bindings 与 warnings 为 nil。
func DeliverBindings(declared []BindingDeclaration, resolved map[string]BindingSource) (bindings []PluginBinding, warnings []string, reason string) {
	bindings = []PluginBinding{}
	warnings = []string{}
	for _, declaration := range declared {
		source, provided := resolved[declaration.Name]
		if !provided {
			continue
		}
		if source.Value == nil && len(source.Properties) == 0 {
			warnings = append(warnings, declaration.Name)
			continue
		}
		if violation := BindingPropertyViolation(declaration.Name, source.Properties); violation != "" {
			return nil, nil, violation
		}
		binding := PluginBinding{Name: declaration.Name, Type: source.Type, Properties: make(map[string]string, len(source.Properties))}
		if binding.Type == "" {
			binding.Type = declaration.Type
		}
		if source.Value != nil {
			binding.Value = *source.Value
		}
		for name, value := range source.Properties {
			binding.Properties[name] = value
		}
		bindings = append(bindings, binding)
	}
	return bindings, warnings, ""
}
