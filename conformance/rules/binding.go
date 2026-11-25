package rules

import (
	"regexp"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

// 下发给插件的绑定属性的上限（plugin/lifecycle.md §2，ADR-0285）。
const (
	// MaxBindingProperties 是一个绑定最多带的属性数。
	MaxBindingProperties = 64
	// MaxBindingPropertyNameLength 是属性名的最大字符数（与 Kubernetes Secret 的键相同）。
	MaxBindingPropertyNameLength = 253
	// MaxBindingPropertyBytes 是一个绑定的全部属性名与属性值按 UTF-8 编码后的字节数上限；主值不计入。
	MaxBindingPropertyBytes = 262144
	// MaxBindingFileBytes 是绑定目录里一个文件的字节数上限（RFC-0016「资源绑定与配置节」第 9 条，ADR-0292）。
	MaxBindingFileBytes = 524288
	// MaxBindingDirectoryBytes 是一个绑定目录里读的文件合计的字节数上限（同上）。
	MaxBindingDirectoryBytes = 1048576
	// MaxBindingDirectoryFiles 是一个绑定目录里读的文件数上限，以点开头的不计（RFC-0016「资源绑定与配置节」第 10 条，ADR-0301）。
	MaxBindingDirectoryFiles = 128
	// MaxBindingDirectories 是解析全部绑定时根目录下的绑定数上限（同上）。
	MaxBindingDirectories = 128
	// MaxConfigureRequestBytes 是整个 ConfigureRequest 按 protobuf 二进制编码的字节数上限（lifecycle.md §2，ADR-0292）。
	MaxConfigureRequestBytes = 2097152
)

// bindingPropertyNamePattern 是下发时属性名的文法：1 到 253 个 ASCII 字母、数字、点、下划线与连字符，不以点开头。
var bindingPropertyNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,252}$`)

// IsBindingPropertyName 判断属性名能不能随 Configure 下发给插件（lifecycle.md §2）。
func IsBindingPropertyName(name string) bool {
	return bindingPropertyNamePattern.MatchString(name)
}

// BindingDeclaration 是清单 bindings 的一项声明：绑定名、清单声明的资源类型与是否必需。
type BindingDeclaration struct {
	// Name 是绑定名。
	Name string
	// Type 是清单声明的资源类型。
	Type string
	// Required 是清单的 required；清单没写时是 true。
	Required bool
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

// BindingDeclarationViolation 核对一个声明的绑定与解析结果（lifecycle.md §2「bindings 的核对」，ADR-0292），给出 reason 的固定英文文本；合格时为空串。
// provided 为假表示两种来源都没给；先核对有没有（必需的绑定没有能下发的内容即不合），再核对类型（来源声明了类型而与清单的不同即不合，必需与可选都核对）。
func BindingDeclarationViolation(declaration BindingDeclaration, source BindingSource, provided bool) string {
	if !provided || !source.HasContent() {
		if declaration.Required {
			return "binding '" + declaration.Name + "' is required but not provided"
		}
		return ""
	}
	if source.Type != "" && source.Type != declaration.Type {
		return "binding '" + declaration.Name + "' is of type '" + source.Type + "' but the manifest declares '" + declaration.Type + "'"
	}
	return ""
}

// HasContent 返回解析出的绑定有没有能交给插件的内容：主值或至少一个属性。只给了类型（或一个空目录）的没有。
func (s BindingSource) HasContent() bool {
	return s.Value != nil || len(s.Properties) > 0
}

// DeliverBindings 算出宿主随 Configure 下发的绑定（lifecycle.md §2）：只看清单声明的绑定，按清单的顺序逐个核对，报第一处不合的。
// 一个绑定里依次是 BindingDeclarationViolation（有没有、类型）与 BindingPropertyViolation（数量、属性名、字节数）；
// 可选的绑定两种来源都没给就跳过，只给了类型的不下发，绑定名记进 warnings（宿主为它记一条 Warning 日志）；
// 其余的下发，类型是清单声明的（来源声明了类型时与它相同），主值没有时为空，属性原样。
// 有不合的地方时 reason 是那一处的固定文本，这时宿主不发 Configure，bindings 与 warnings 为 nil。整个请求的大小另由 ConfigureRequestViolation 核对。
func DeliverBindings(declared []BindingDeclaration, resolved map[string]BindingSource) (bindings []PluginBinding, warnings []string, reason string) {
	bindings = []PluginBinding{}
	warnings = []string{}
	for _, declaration := range declared {
		source, provided := resolved[declaration.Name]
		if violation := BindingDeclarationViolation(declaration, source, provided); violation != "" {
			return nil, nil, violation
		}
		if !provided {
			continue
		}
		if !source.HasContent() {
			warnings = append(warnings, declaration.Name)
			continue
		}
		if violation := BindingPropertyViolation(declaration.Name, source.Properties); violation != "" {
			return nil, nil, violation
		}
		binding := PluginBinding{Name: declaration.Name, Type: declaration.Type, Properties: make(map[string]string, len(source.Properties))}
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

// ConfigureRequestSize 算出 ConfigureRequest 按 protobuf 二进制编码（plugin.proto）的字节数，也就是 gRPC 载体发出的那条消息的长度。
// proto3 不写取缺省值（空串）的标量字段；map 的每一项是一条带键与值两个字段的子消息，键与值总是写出。
func ConfigureRequestSize(configuration string, grantedPermissions []string, bindings []PluginBinding, workDirectory string) int {
	size := stringFieldSize(configuration)
	for _, permission := range grantedPermissions {
		size += repeatedStringSize(permission)
	}
	for _, binding := range bindings {
		message := stringFieldSize(binding.Name) + stringFieldSize(binding.Type) + stringFieldSize(binding.Value)
		for name, value := range binding.Properties {
			message += embeddedSize(repeatedStringSize(name) + repeatedStringSize(value))
		}
		size += embeddedSize(message)
	}
	return size + stringFieldSize(workDirectory)
}

// ConfigureRequestViolation 核对整个请求的大小（lifecycle.md §2，ADR-0292）：size 超过 MaxConfigureRequestBytes 时给出 reason 的固定英文文本，否则为空串。
func ConfigureRequestViolation(size int) string {
	if size > MaxConfigureRequestBytes {
		return "the configure request exceeds " + strconv.Itoa(MaxConfigureRequestBytes) + " bytes"
	}
	return ""
}

// stringFieldSize 是一个单值字符串字段的编码长度：空串不写。字段号都小于 16，标签占一个字节。
func stringFieldSize(value string) int {
	if value == "" {
		return 0
	}
	return repeatedStringSize(value)
}

// repeatedStringSize 是一个总要写出的字符串字段（repeated 的一项、map 的键与值）的编码长度：标签、长度与内容。
func repeatedStringSize(value string) int {
	return 1 + protowire.SizeBytes(len(value))
}

// embeddedSize 是一条长 length 字节的子消息作为字段的编码长度：标签、长度与内容。
func embeddedSize(length int) int {
	return 1 + protowire.SizeBytes(length)
}
