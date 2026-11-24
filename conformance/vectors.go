// Package conformance 内嵌 XiHan Framework Spec 的一致性向量并提供类型化的读取入口；
// 黑盒运行器在子包 plugin，规范性文法在子包 rules。
package conformance

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// Vectors 内嵌全部向量文件（vectors/<主题>/<名字>.json）；SDK 的测试可直接读取。
//
//go:embed vectors
var Vectors embed.FS

// Load 读取并解析一个向量文件；name 形如 "plugin/handshake-lines.json"。
func Load[T any](name string) (T, error) {
	var vector T
	data, err := fs.ReadFile(Vectors, "vectors/"+name)
	if err != nil {
		return vector, fmt.Errorf("conformance: read vector %q: %w", name, err)
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		return vector, fmt.Errorf("conformance: parse vector %q: %w", name, err)
	}
	return vector, nil
}

// Names 列出全部向量文件名（相对 vectors/，正斜杠分隔，升序）。
func Names() ([]string, error) {
	var names []string
	err := fs.WalkDir(Vectors, "vectors", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".json") {
			names = append(names, strings.TrimPrefix(path, "vectors/"))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("conformance: list vectors: %w", err)
	}
	sort.Strings(names)
	return names, nil
}

// HandshakeLine 是一条合法握手行及其解析结果。
type HandshakeLine struct {
	Line    string `json:"line"`
	Network string `json:"network"`
	Address string `json:"address"`
}

// HandshakeLineVectors 对应 plugin/handshake-lines.json。
type HandshakeLineVectors struct {
	Valid   []HandshakeLine `json:"valid"`
	Invalid []string        `json:"invalid"`
}

// HostEndpoint 是宿主回调端点列表里的一项。
type HostEndpoint struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

// HostAddressCase 是一个合法的 XIHAN_PLUGIN_HOST_ADDRESS 值及其各项。
type HostAddressCase struct {
	Value   string         `json:"value"`
	Entries []HostEndpoint `json:"entries"`
}

// HostAddressSelection 是一次回调端点的选择：Value 为 nil 表示变量缺失，Selected 为 nil 表示没有可用的项。
type HostAddressSelection struct {
	Value     *string       `json:"value"`
	Supported []string      `json:"supported"`
	Selected  *HostEndpoint `json:"selected"`
}

// HostAddressVectors 对应 plugin/host-addresses.json（handshake.md §4，ADR-0237）。
type HostAddressVectors struct {
	Valid     []HostAddressCase      `json:"valid"`
	Invalid   []string               `json:"invalid"`
	Selection []HostAddressSelection `json:"selection"`
}

// DeadlineCase 是一个合法的 deadline 文本及其 UTC 时刻（RFC 3339，九位小数）。
type DeadlineCase struct {
	Text string `json:"text"`
	UTC  string `json:"utc"`
}

// EffectiveDeadlineCase 是一次生效截止时间的选取：GRPC 为 nil 表示 gRPC 调用没有截止时间，Expected 为 nil 表示没有截止时间。
type EffectiveDeadlineCase struct {
	Metadata string  `json:"metadata"`
	GRPC     *string `json:"grpc"`
	Expected *string `json:"expected"`
}

// WrittenDeadline 是一次截止时间的写出：UTC 时刻与写出的文本。
type WrittenDeadline struct {
	UTC  string `json:"utc"`
	Text string `json:"text"`
}

// DeadlineVectors 对应 plugin/deadlines.json（invocation.md §2，ADR-0237）。
type DeadlineVectors struct {
	Valid     []DeadlineCase          `json:"valid"`
	Invalid   []string                `json:"invalid"`
	Effective []EffectiveDeadlineCase `json:"effective"`
	Written   []WrittenDeadline       `json:"written"`
}

// HTTPDeadlineCase 是一次从 HTTP 请求头读截止时间：Headers 是头名到各个值的列表（一个头出现几次就有几项），
// Deadline 是期望的截止时间（UTC，九位小数），nil 表示没有截止时间。
type HTTPDeadlineCase struct {
	Name     string              `json:"name"`
	Headers  map[string][]string `json:"headers"`
	Deadline *string             `json:"deadline"`
}

// HTTPDeadlineVectors 对应 context/http-deadline.json（RFC-0003 规则 8，protocol/http.md，protocol/grpc.md，ADR-0276）：Now 是宿主时钟的当前时刻。
type HTTPDeadlineVectors struct {
	Now   string             `json:"now"`
	Cases []HTTPDeadlineCase `json:"cases"`
}

// PluginMessage 是 SDK 运行时与宿主共用的一项插件错误：码、消息键、参数名（也就是 details 的成员）与 en-US 模板。
type PluginMessage struct {
	Code      string   `json:"code"`
	Key       string   `json:"key"`
	Arguments []string `json:"arguments"`
	Template  string   `json:"template"`
	When      string   `json:"when"`
}

// PluginMessageReason 是写进 reason 参数的固定英文文本；Phase 只用于 plugins.process.failed。
type PluginMessageReason struct {
	Key    string `json:"key"`
	Phase  string `json:"phase,omitempty"`
	Reason string `json:"reason"`
	When   string `json:"when"`
}

// PluginMessageVectors 对应 plugin/plugin-messages.json（invocation.md §8，ADR-0237）。
type PluginMessageVectors struct {
	Messages []PluginMessage       `json:"messages"`
	Reasons  []PluginMessageReason `json:"reasons"`
}

// CallbackStatusCase 是一次回调失败：RPC 是 invoke 或 publish，Status 与 StatusCode 是 gRPC 状态的名字与数值；
// 期望的错误是 Code + Key，Details 是错误的 details（JSON 值，null 表示没有）。
type CallbackStatusCase struct {
	RPC              string          `json:"rpc"`
	Status           string          `json:"status"`
	StatusCode       uint32          `json:"statusCode"`
	Message          string          `json:"message"`
	ContextCancelled bool            `json:"contextCancelled"`
	Code             string          `json:"code"`
	Key              string          `json:"key"`
	Details          json.RawMessage `json:"details"`
}

// CallbackStatusVectors 对应 plugin/callback-status.json（invocation.md §5，ADR-0237）：插件 PluginID 回调 Host.Invoke（操作 Operation）或 Host.Publish。
type CallbackStatusVectors struct {
	PluginID  string               `json:"pluginId"`
	Operation string               `json:"operation"`
	Cases     []CallbackStatusCase `json:"cases"`
}

// ProtoError 是线上 Error 的字段（plugin.proto 的 Error）：Details 是 JSON 文本，Inner 为 nil 表示没有内部错误。
type ProtoError struct {
	Code       string      `json:"code"`
	MessageKey string      `json:"message_key"`
	Text       string      `json:"text"`
	Details    string      `json:"details"`
	Inner      *ProtoError `json:"inner,omitempty"`
}

// DecodedErrorExpectation 是读入后期望的错误：Key 为 nil 表示自由文本错误，Text 为 nil 表示没有文案，
// Details 按 JSON 值比较（null 表示没有细节），Inner 为 nil 表示没有内部错误。
type DecodedErrorExpectation struct {
	Code    string                   `json:"code"`
	Key     *string                  `json:"key"`
	Text    *string                  `json:"text"`
	Details json.RawMessage          `json:"details"`
	Inner   *DecodedErrorExpectation `json:"inner"`
}

// ErrorReadingCase 是一次读入：线上的 Error 与读入后期望的错误。
type ErrorReadingCase struct {
	Name     string                  `json:"name"`
	Error    ProtoError              `json:"error"`
	Expected DecodedErrorExpectation `json:"expected"`
}

// StatusWithoutErrorCase 是 Status.ok 为假却没有 error 时的结果：Reader 是 host（宿主读插件，阶段为 Phase）或 plugin
// （插件读宿主，回调的 RPC 与操作 ID；Operation 为 nil 表示 Host.Publish）。
type StatusWithoutErrorCase struct {
	Reader    string                  `json:"reader"`
	Phase     string                  `json:"phase,omitempty"`
	RPC       string                  `json:"rpc,omitempty"`
	Operation *string                 `json:"operation,omitempty"`
	Expected  DecodedErrorExpectation `json:"expected"`
}

// ErrorReadingVectors 对应 plugin/error-reading.json（invocation.md §3，ADR-0237）：插件 PluginID 的错误读入。
type ErrorReadingVectors struct {
	PluginID           string                   `json:"pluginId"`
	Cases              []ErrorReadingCase       `json:"cases"`
	StatusWithoutError []StatusWithoutErrorCase `json:"statusWithoutError"`
}

// BindingPropertyLimits 是下发给插件的绑定属性的上限：一个绑定的属性数、属性名的字符数、全部属性名与属性值的 UTF-8 字节数。
type BindingPropertyLimits struct {
	MaxProperties int `json:"maxProperties"`
	MaxNameLength int `json:"maxNameLength"`
	MaxBytes      int `json:"maxBytes"`
}

// BindingPropertyNames 是下发时属性名文法的样例：Valid 必须接受，Invalid 必须拒绝。
type BindingPropertyNames struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// BindingProperties 是一个绑定的属性（属性名到属性值）。向量里的属性值是字符串，或 {repeat, count}（把 repeat 重复 count 次，
// 用来写上限两侧的长值）；读入后都是展开的文本。
type BindingProperties map[string]string

// UnmarshalJSON 读入属性并展开 {repeat, count} 形式的值。
func (p *BindingProperties) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("conformance: parse binding properties: %w", err)
	}
	properties := make(BindingProperties, len(raw))
	for name, value := range raw {
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			properties[name] = text
			continue
		}
		// 不是字符串就必须是 {repeat, count}：多余或缺少成员都算写错，免得把笔误读成空值
		var repeated struct {
			Repeat *string `json:"repeat"`
			Count  *int    `json:"count"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(value)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&repeated); err != nil || repeated.Repeat == nil || repeated.Count == nil || *repeated.Count < 0 {
			return fmt.Errorf("conformance: binding property %q must be a string or {repeat, count}", name)
		}
		properties[name] = strings.Repeat(*repeated.Repeat, *repeated.Count)
	}
	*p = properties
	return nil
}

// DeclaredBinding 是清单 bindings 的一项声明。
type DeclaredBinding struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ResolvedBinding 是按 RFC-0016 解析出的一个绑定：Type 为空表示来源没有声明类型，Value 为 nil 表示来源没有给主值。
type ResolvedBinding struct {
	Type       string            `json:"type"`
	Value      *string           `json:"value"`
	Properties BindingProperties `json:"properties"`
}

// DeliveredBinding 是 Configure 请求里的一个绑定，也是 SDK 交给作者的绑定：没有属性时 Properties 是空表。
type DeliveredBinding struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Value      string            `json:"value"`
	Properties BindingProperties `json:"properties"`
}

// BindingDeliveryError 是宿主不发 Configure 时的错误：错误码、消息键与 details（id、phase、reason）。
type BindingDeliveryError struct {
	Code    string            `json:"code"`
	Key     string            `json:"key"`
	Details map[string]string `json:"details"`
}

// BindingDeliveryCase 是宿主一侧的一次组装：Declared 是清单的声明（按清单的顺序），Resolved 是解析出的绑定（键是绑定名）。
// 期望是 Bindings（下发的绑定，连同记了 Warning 的绑定名 Warnings）或 Error 之一，Error 为 nil 表示下发。
type BindingDeliveryCase struct {
	Name     string                     `json:"name"`
	Declared []DeclaredBinding          `json:"declared"`
	Resolved map[string]ResolvedBinding `json:"resolved"`
	Bindings []DeliveredBinding         `json:"bindings"`
	Warnings []string                   `json:"warnings"`
	Error    *BindingDeliveryError      `json:"error"`
}

// BindingReadingCase 是插件一侧的一次读取：Hex 是宿主写出的 ConfigureRequest 的 protobuf 字节（十六进制），
// JSON 是同一个请求在 Wasm 载体上的 proto3 JSON 文本，Bindings 是 SDK 交给作者的绑定。
type BindingReadingCase struct {
	Name     string             `json:"name"`
	Hex      string             `json:"hex"`
	JSON     string             `json:"json"`
	Bindings []DeliveredBinding `json:"bindings"`
}

// BindingPropertyVectors 对应 plugin/binding-properties.json（lifecycle.md §2 与 §7.2，ADR-0285）：插件 PluginID 的绑定连同属性的下发与读取。
type BindingPropertyVectors struct {
	PluginID string                `json:"pluginId"`
	Limits   BindingPropertyLimits `json:"limits"`
	Names    BindingPropertyNames  `json:"names"`
	Delivery []BindingDeliveryCase `json:"delivery"`
	Reading  []BindingReadingCase  `json:"reading"`
}

// ProtocolVersionCase 是一次版本协商：有交集时 Selected 是选定版本，否则 Error 是错误码。
type ProtocolVersionCase struct {
	Host     []uint32 `json:"host"`
	Plugin   []uint32 `json:"plugin"`
	Selected uint32   `json:"selected"`
	Error    string   `json:"error"`
}

// PrimitivesCase 是一次原语层版本核对。
type PrimitivesCase struct {
	Host   uint32 `json:"host"`
	Plugin uint32 `json:"plugin"`
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
}

// ProtocolVersionVectors 对应 plugin/protocol-versions.json。
type ProtocolVersionVectors struct {
	Cases      []ProtocolVersionCase `json:"cases"`
	Primitives []PrimitivesCase      `json:"primitives"`
}

// WasmExport 是 Wasm ABI 的一个导出或导入函数签名。
type WasmExport struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Params       []string `json:"params"`
	Results      []string `json:"results"`
	RequiredWhen string   `json:"requiredWhen"`
	Reason       string   `json:"reason"`
}

// WasmExportVectors 对应 plugin/wasm-exports.json。
type WasmExportVectors struct {
	ABIVersion int `json:"abiVersion"`
	Exports    struct {
		Required []WasmExport `json:"required"`
		Optional []WasmExport `json:"optional"`
		Rejected []WasmExport `json:"rejected"`
	} `json:"exports"`
	Imports struct {
		// Xihan 是模块 xihan 提供的导入函数。
		Xihan []WasmExport `json:"xihan"`
		// WasiSnapshotPreview1 说明 WASI preview 1 由宿主提供。
		WasiSnapshotPreview1 string `json:"wasi_snapshot_preview1"`
	} `json:"imports"`
}

// ErrorCodeVectors 对应 primitives/error-codes.json。
type ErrorCodeVectors struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// OperationIDVectors 对应 primitives/operation-ids.json（conventions/operation-ids.md，ADR-0276）：MaxLength 是整个 ID 的最大字符数。
type OperationIDVectors struct {
	MaxLength int      `json:"maxLength"`
	Valid     []string `json:"valid"`
	Invalid   []string `json:"invalid"`
}

// BaggageKeyVectors 对应 primitives/baggage-keys.json。
type BaggageKeyVectors struct {
	WellFormed []string            `json:"wellFormed"`
	IllFormed  []string            `json:"illFormed"`
	Registered []string            `json:"registered"`
	Values     BaggageValueVectors `json:"values"`
}

// BaggageValueVectors 是已登记键的值能否进入调用上下文的样例（ADR-0204、ADR-0233）。
type BaggageValueVectors struct {
	Acceptable   []string `json:"acceptable"`
	Unacceptable []string `json:"unacceptable"`
}

// WireCase 是一条线上编码样例：Kind 只用于 Result（value / unit / maybe）；
// 合法样例再写出时按 JSON 值等于 Canonical，Canonical 为空时等于 JSON。
type WireCase struct {
	Name      string          `json:"name"`
	Kind      string          `json:"kind,omitempty"`
	JSON      json.RawMessage `json:"json"`
	Canonical json.RawMessage `json:"canonical,omitempty"`
}

// WireCaseSet 是一组必须读得出与必须拒绝的样例。
type WireCaseSet struct {
	Valid   []WireCase `json:"valid"`
	Invalid []WireCase `json:"invalid"`
}

// WireEncodingVectors 对应 primitives/wire-encoding.json（RFC-0001「Result」）。
type WireEncodingVectors struct {
	Results    WireCaseSet     `json:"results"`
	Errors     WireCaseSet     `json:"errors"`
	Duplicates []DuplicateCase `json:"duplicates"`
}

// DuplicateCase 是同一层有重复成员名的原文，必须被拒绝；Target 是 result（带 Kind）或 error。
type DuplicateCase struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Kind   string `json:"kind,omitempty"`
	Text   string `json:"text"`
}

// TraceContextVectors 对应 primitives/trace-context.json（RFC-0003 规则 7）。
type TraceContextVectors struct {
	Parse struct {
		Valid   []ParsedTraceParent `json:"valid"`
		Invalid []string            `json:"invalid"`
	} `json:"parse"`
	Format []TraceParentFormat `json:"format"`
}

// ParsedTraceParent 是一条合法的 traceparent 及其解析结果。
type ParsedTraceParent struct {
	Text    string `json:"text"`
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
	Sampled bool   `json:"sampled"`
}

// TraceParentFormat 是一次写出；Sampled 为 nil 表示上下文没有标志，写 01。
type TraceParentFormat struct {
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
	Sampled *bool  `json:"sampled"`
	Text    string `json:"text"`
}

// ManifestCase 是一份清单：Manifest 是 JSON 值，Text 是原文（表达重复成员）；Reason 只给人看。
type ManifestCase struct {
	Name     string          `json:"name"`
	Manifest json.RawMessage `json:"manifest,omitempty"`
	// Text 是原文，用于 JSON 值表达不了的写法；nil 表示没有给出（空串是合法的原文）。
	Text   *string `json:"text,omitempty"`
	Reason string  `json:"reason,omitempty"`
	// Written 是合法清单读出后再写出的全文（ADR-0236）。
	Written string `json:"written,omitempty"`
	// Error 是不合法清单期望的错误（ADR-0236）。
	Error *ManifestError `json:"error,omitempty"`
}

// ManifestError 是读取清单失败时的错误：码、消息键与参数逐字相等。
type ManifestError struct {
	Code       string            `json:"code"`
	MessageKey string            `json:"messageKey"`
	Arguments  map[string]string `json:"arguments"`
}

// ManifestVectors 对应 manifest/manifests.json（RFC-0002 解析规则 2）。
type ManifestVectors struct {
	Valid   []ManifestCase `json:"valid"`
	Invalid []ManifestCase `json:"invalid"`
}

// PipelineCase 是一组登记与期望的 Dump 全文（{type} 换成实现给按名字登记的实例用的类型名）。
type PipelineCase struct {
	Name  string            `json:"name"`
	Steps []json.RawMessage `json:"steps"`
	Dump  string            `json:"dump"`
}

// PipelineError 是一组登记与期望的错误码；At 是抛出错误的那一步（从 0 起）或 "sort"。
type PipelineError struct {
	Name  string            `json:"name"`
	Steps []json.RawMessage `json:"steps"`
	Error string            `json:"error"`
	At    json.RawMessage   `json:"at"`
	// Message 是配置错误的英文原文，{type} 换成按名字登记的实例的类型名（ADR-0236）。
	Message string `json:"message"`
}

// PipelineOrderingVectors 对应 pipeline/ordering.json（RFC-0004）。
type PipelineOrderingVectors struct {
	Cases  []PipelineCase  `json:"cases"`
	Errors []PipelineError `json:"errors"`
}

// PayloadReasonCase 是一次载荷解码：请求类型（counts / order）、载荷原文与期望的 reason。
type PayloadReasonCase struct {
	Name    string `json:"name"`
	Request string `json:"request"`
	Payload string `json:"payload"`
	// Reason 为 nil 表示解码成功（ADR-0236）。
	Reason *string `json:"reason"`
}

// PayloadReasonVectors 对应 operations/payload-reasons.json（RFC-0011 规则 2）。
type PayloadReasonVectors struct {
	Requests map[string]string   `json:"requests"`
	Cases    []PayloadReasonCase `json:"cases"`
}

// MessageKeyVectors 是消息键的合法与非法样例。
type MessageKeyVectors struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// TemplateCase 是一次模板格式化：Arguments 保留原始 JSON（null 表示没有参数），
// 以便按整数、浮点、布尔、字符串、null 区分取值；Strict 为 nil 表示严格格式化失败。
type TemplateCase struct {
	Name      string          `json:"name"`
	Template  string          `json:"template"`
	Arguments json.RawMessage `json:"arguments"`
	// Locale 是格式化所用的区域，空表示不变文化（ADR-0236）。
	Locale  string  `json:"locale,omitempty"`
	Lenient string  `json:"lenient"`
	Strict  *string `json:"strict"`
}

// DescribeCase 是一次 Describe：Arguments 为 null 表示没有参数。
type DescribeCase struct {
	Key       string          `json:"key"`
	Arguments json.RawMessage `json:"arguments"`
	Text      string          `json:"text"`
}

// MessageTemplateVectors 对应 primitives/message-templates.json（RFC-0010「模板」）。
type MessageTemplateVectors struct {
	Keys     MessageKeyVectors `json:"keys"`
	Format   []TemplateCase    `json:"format"`
	Describe []DescribeCase    `json:"describe"`
}

// KernelMessage 是 SDK 与宿主共用的一项内核错误：码、消息键、参数名与 en-US 模板。
type KernelMessage struct {
	Code      string   `json:"code"`
	Key       string   `json:"key"`
	Arguments []string `json:"arguments"`
	Template  string   `json:"template"`
	When      string   `json:"when"`
}

// KernelMessageVectors 对应 primitives/kernel-messages.json（RFC-0011「错误码」）。
type KernelMessageVectors struct {
	Messages []KernelMessage `json:"messages"`
}

// CapabilityRequirementCase 是一条合法的能力依赖文本及其解析结果。
type CapabilityRequirementCase struct {
	Input    string `json:"input"`
	ID       string `json:"id"`
	Version  int    `json:"version"`
	Optional bool   `json:"optional"`
}

// CapabilityRequirementVectors 对应 primitives/capability-requirements.json。
type CapabilityRequirementVectors struct {
	Valid   []CapabilityRequirementCase `json:"valid"`
	Invalid []string                    `json:"invalid"`
}

// LifecycleTransitionVectors 对应 primitives/lifecycle-transitions.json；Allowed 的每项是 [from, to]。
type LifecycleTransitionVectors struct {
	Allowed [][2]string `json:"allowed"`
}

// ProviderSample 是远程 Provider 操作集的一组请求 / 响应样例。
type ProviderSample struct {
	Operation string          `json:"operation"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response"`
}

// CachingDistributedVectors 对应 provider/caching-distributed.json。
type CachingDistributedVectors struct {
	Capability     string            `json:"capability"`
	Operations     []string          `json:"operations"`
	Samples        []ProviderSample  `json:"samples"`
	InvalidKeys    []string          `json:"invalidKeys"`
	InvalidOptions []json.RawMessage `json:"invalidOptions"`
	Errors         map[string]string `json:"errors"`
}

// CacheKeyCase 是一条缓存键或键前缀的样例：Text 是原文，UTF8Bytes 是它的 UTF-8 字节数，
// UTF16Units 是 UTF-16 码元数（不参与判定，只为标出按码元计会判错的用例）。
type CacheKeyCase struct {
	Name       string `json:"name"`
	Text       string `json:"text"`
	UTF8Bytes  int    `json:"utf8Bytes"`
	UTF16Units int    `json:"utf16Units"`
}

// CacheKeyCaseSet 是一组必须接受与必须拒绝的样例。
type CacheKeyCaseSet struct {
	Valid   []CacheKeyCase `json:"valid"`
	Invalid []CacheKeyCase `json:"invalid"`
}

// CacheKeyVectors 对应 caching/cache-keys.json（RFC-0006「存储契约」与「键管理」，ADR-0276）：
// MaxBytes 是键与前缀按 UTF-8 编码后的最大字节数，Keys 是键，Prefixes 是键前缀（与键同一条规则，只是可以为空）。
type CacheKeyVectors struct {
	MaxBytes int             `json:"maxBytes"`
	Keys     CacheKeyCaseSet `json:"keys"`
	Prefixes CacheKeyCaseSet `json:"prefixes"`
}

// WebAuthnVectors 对应 security/webauthn.json；响应与凭据保持原样，形状见 RFC-0008「WebAuthn」。
type WebAuthnVectors struct {
	RelyingParty   WebAuthnRelyingParty `json:"relyingParty"`
	Registration   []WebAuthnCase       `json:"registration"`
	Authentication []WebAuthnCase       `json:"authentication"`
}

// WebAuthnRelyingParty 是向量所用的依赖方配置。
type WebAuthnRelyingParty struct {
	ID               string   `json:"id"`
	Origins          []string `json:"origins"`
	UserVerification string   `json:"userVerification"`
}

// WebAuthnCase 是一条注册或认证样例：成功时有 Expected，失败时有 Error。
type WebAuthnCase struct {
	Name       string          `json:"name"`
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential,omitempty"`
	// ExpectedUserHandle 是登录前已认出的用户（base64url）；免用户名的流程为空，此时响应必须带 userHandle（ADR-0179）。
	ExpectedUserHandle string          `json:"expectedUserHandle,omitempty"`
	Response           json.RawMessage `json:"response"`
	Expected           json.RawMessage `json:"expected,omitempty"`
	Error              *WebAuthnError  `json:"error,omitempty"`
}

// WebAuthnError 是样例期望的错误：错误码与机读原因（算法不支持时是算法编号）。
type WebAuthnError struct {
	Code      string `json:"code"`
	Reason    string `json:"reason,omitempty"`
	Algorithm *int64 `json:"algorithm,omitempty"`
}

// TypedVector 是向量文件表的一项：一个向量文件与它的类型化读取。
type TypedVector struct {
	// Name 是文件名，相对 vectors/，与 Names 的写法相同。
	Name string
	// Load 按文件的类型读取并解析，返回的值与对应的类型化入口（如 HandshakeLines）给出的相同。
	Load func() (any, error)
	// kind 是文件内容的类型，类型化入口按它在表里找文件。
	kind reflect.Type
}

// vectorFile 把一个向量文件写成表里的一项，T 是文件内容的类型。
func vectorFile[T any](name string) TypedVector {
	return TypedVector{
		Name: name,
		Load: func() (any, error) { return Load[T](name) },
		kind: reflect.TypeFor[T](),
	}
}

// typedVectors 是向量文件表：每个向量文件一项，按 Names 的顺序。文件名只写在这里，类型化入口与 TypedVectors
// （vectors 子命令与测试用它）都从这张表取；新增向量文件在这里加一行，再加一个类型化入口。
var typedVectors = []TypedVector{
	vectorFile[AINameVectors]("ai/ai-names.json"),
	vectorFile[KnowledgeChunkVectors]("ai/knowledge-chunks.json"),
	vectorFile[AIPromptTemplateVectors]("ai/prompt-templates.json"),
	vectorFile[AIProviderDefaultVectors]("ai/provider-defaults.json"),
	vectorFile[AIToolNameVectors]("ai/tool-names.json"),
	vectorFile[AuditEventSelectionVectors]("auditing/event-selection.json"),
	vectorFile[AuditExclusionVectors]("auditing/exclusions.json"),
	vectorFile[CacheKeyVectors]("caching/cache-keys.json"),
	vectorFile[RemoteDocumentVectors]("configuration/remote-documents.json"),
	vectorFile[SecretNameVectors]("configuration/secret-names.json"),
	vectorFile[HTTPDeadlineVectors]("context/http-deadline.json"),
	vectorFile[CatalogResolutionVectors]("localization/catalog-resolution.json"),
	vectorFile[LocaleNameVectors]("localization/locale-names.json"),
	vectorFile[LocaleNegotiationVectors]("localization/locale-negotiation.json"),
	vectorFile[ManifestVectors]("manifest/manifests.json"),
	vectorFile[CloudEventEnvelopeVectors]("messaging/cloudevents-envelopes.json"),
	vectorFile[EnvelopeExtensionVectors]("messaging/envelope-extensions.json"),
	vectorFile[MessagingInstanceIDVectors]("messaging/instance-ids.json"),
	vectorFile[OutboxBackoffVectors]("messaging/outbox-backoff.json"),
	vectorFile[TopicVectors]("messaging/topics.json"),
	vectorFile[TenantIDVectors]("multitenancy/tenant-ids.json"),
	vectorFile[TenantResolutionVectors]("multitenancy/tenant-resolution.json"),
	vectorFile[TenantShardVectors]("multitenancy/tenant-shards.json"),
	vectorFile[BlobContentTypeVectors]("objectstorage/blob-content-types.json"),
	vectorFile[BlobKeyVectors]("objectstorage/blob-keys.json"),
	vectorFile[BlobMetadataVectors]("objectstorage/blob-metadata.json"),
	vectorFile[BlobRangeVectors]("objectstorage/blob-ranges.json"),
	vectorFile[ContainerNameVectors]("objectstorage/container-names.json"),
	vectorFile[MultipartPartVectors]("objectstorage/multipart-parts.json"),
	vectorFile[BlobStoreDefinitionVectors]("objectstorage/store-definitions.json"),
	vectorFile[HostUsageVectors]("observability/host-usage.json"),
	vectorFile[PayloadReasonVectors]("operations/payload-reasons.json"),
	vectorFile[IndexNameVectors]("persistence/index-names.json"),
	vectorFile[SnowflakeIDVectors]("persistence/snowflake-ids.json"),
	vectorFile[PipelineOrderingVectors]("pipeline/ordering.json"),
	vectorFile[BindingPropertyVectors]("plugin/binding-properties.json"),
	vectorFile[CallbackStatusVectors]("plugin/callback-status.json"),
	vectorFile[DeadlineVectors]("plugin/deadlines.json"),
	vectorFile[ErrorReadingVectors]("plugin/error-reading.json"),
	vectorFile[HandshakeLineVectors]("plugin/handshake-lines.json"),
	vectorFile[HostAddressVectors]("plugin/host-addresses.json"),
	vectorFile[PluginMessageVectors]("plugin/plugin-messages.json"),
	vectorFile[ProtocolVersionVectors]("plugin/protocol-versions.json"),
	vectorFile[WasmExportVectors]("plugin/wasm-exports.json"),
	vectorFile[BaggageKeyVectors]("primitives/baggage-keys.json"),
	vectorFile[CapabilityRequirementVectors]("primitives/capability-requirements.json"),
	vectorFile[ErrorCodeVectors]("primitives/error-codes.json"),
	vectorFile[JSONTextVectors]("primitives/json-text.json"),
	vectorFile[KernelMessageVectors]("primitives/kernel-messages.json"),
	vectorFile[LifecycleTransitionVectors]("primitives/lifecycle-transitions.json"),
	vectorFile[MessageTemplateVectors]("primitives/message-templates.json"),
	vectorFile[OperationIDVectors]("primitives/operation-ids.json"),
	vectorFile[TraceContextVectors]("primitives/trace-context.json"),
	vectorFile[WireEncodingVectors]("primitives/wire-encoding.json"),
	vectorFile[CachingDistributedVectors]("provider/caching-distributed.json"),
	vectorFile[CronExpressionVectors]("scheduling/cron-expressions.json"),
	vectorFile[SchedulingEventTopicVectors]("scheduling/event-topics.json"),
	vectorFile[JobNameVectors]("scheduling/job-names.json"),
	vectorFile[JobRetryDelayVectors]("scheduling/retry-delays.json"),
	vectorFile[WebAuthnVectors]("security/webauthn.json"),
	vectorFile[WorkflowKeywordVectors]("workflow/keyword-matching.json"),
	vectorFile[WorkflowStatusOrderVectors]("workflow/status-order.json"),
	vectorFile[WorkflowEventTopicVectors]("workflow/workflow-event-topics.json"),
	vectorFile[WorkflowNameVectors]("workflow/workflow-names.json"),
}

// TypedVectors 返回向量文件表：每个向量文件一项，按 Names 的顺序。
func TypedVectors() []TypedVector {
	return slices.Clone(typedVectors)
}

// typed 按内容的类型在向量文件表里找到文件并读取；表里没有这个类型时报错。
func typed[T any]() (T, error) {
	kind := reflect.TypeFor[T]()
	for _, file := range typedVectors {
		if file.kind == kind {
			return Load[T](file.Name)
		}
	}
	var vector T
	return vector, fmt.Errorf("conformance: no vector file is registered for %s", kind)
}

// WebAuthn 读取 WebAuthn 注册与认证的样例。
func WebAuthn() (WebAuthnVectors, error) {
	return typed[WebAuthnVectors]()
}

// CachingDistributed 读取 caching.distributed 远程 Provider 的操作集向量。
func CachingDistributed() (CachingDistributedVectors, error) {
	return typed[CachingDistributedVectors]()
}

// CacheKeys 读取缓存键与键前缀的文法向量。
func CacheKeys() (CacheKeyVectors, error) {
	return typed[CacheKeyVectors]()
}

// HandshakeLines 读取握手行向量。
func HandshakeLines() (HandshakeLineVectors, error) {
	return typed[HandshakeLineVectors]()
}

// ProtocolVersions 读取版本协商向量。
func ProtocolVersions() (ProtocolVersionVectors, error) {
	return typed[ProtocolVersionVectors]()
}

// WasmExports 读取 Wasm ABI 导出表向量。
func WasmExports() (WasmExportVectors, error) {
	return typed[WasmExportVectors]()
}

// HostAddresses 读取宿主回调端点列表的解析与选择向量。
func HostAddresses() (HostAddressVectors, error) {
	return typed[HostAddressVectors]()
}

// Deadlines 读取 InvocationMetadata.deadline 的读法、生效的截止时间与写出向量。
func Deadlines() (DeadlineVectors, error) {
	return typed[DeadlineVectors]()
}

// HTTPDeadlines 读取 HTTP 载体上截止时间的读法向量。
func HTTPDeadlines() (HTTPDeadlineVectors, error) {
	return typed[HTTPDeadlineVectors]()
}

// PluginMessages 读取 SDK 运行时与宿主共用的插件消息键向量。
func PluginMessages() (PluginMessageVectors, error) {
	return typed[PluginMessageVectors]()
}

// CallbackStatus 读取插件一侧回调失败的映射向量。
func CallbackStatus() (CallbackStatusVectors, error) {
	return typed[CallbackStatusVectors]()
}

// ErrorReading 读取读入对方写来的 Error 与 Status 的向量。
func ErrorReading() (ErrorReadingVectors, error) {
	return typed[ErrorReadingVectors]()
}

// PluginBindings 读取下发给插件的绑定连同属性的向量：属性名的文法与上限、宿主下发什么、插件读到什么。
func PluginBindings() (BindingPropertyVectors, error) {
	return typed[BindingPropertyVectors]()
}

// ErrorCodes 读取错误码格式向量。
func ErrorCodes() (ErrorCodeVectors, error) {
	return typed[ErrorCodeVectors]()
}

// OperationIDs 读取操作 ID 文法向量。
func OperationIDs() (OperationIDVectors, error) {
	return typed[OperationIDVectors]()
}

// BaggageKeys 读取 Baggage 键向量。
func BaggageKeys() (BaggageKeyVectors, error) {
	return typed[BaggageKeyVectors]()
}

// CapabilityRequirements 读取能力依赖文本向量。
func CapabilityRequirements() (CapabilityRequirementVectors, error) {
	return typed[CapabilityRequirementVectors]()
}

// LifecycleTransitions 读取生命周期迁移向量。
func LifecycleTransitions() (LifecycleTransitionVectors, error) {
	return typed[LifecycleTransitionVectors]()
}

// WireEncoding 读取 Result 与 Error 的线上编码向量。
func WireEncoding() (WireEncodingVectors, error) {
	return typed[WireEncodingVectors]()
}

// MessageTemplates 读取消息键文法、模板格式化与 Describe 向量。
func MessageTemplates() (MessageTemplateVectors, error) {
	return typed[MessageTemplateVectors]()
}

// KernelMessages 读取 SDK 与宿主共用的内核错误向量。
func KernelMessages() (KernelMessageVectors, error) {
	return typed[KernelMessageVectors]()
}

// TraceContext 读取 traceparent 的解析与写出向量。
func TraceContext() (TraceContextVectors, error) {
	return typed[TraceContextVectors]()
}

// Manifests 读取清单读取的向量。
func Manifests() (ManifestVectors, error) {
	return typed[ManifestVectors]()
}

// PipelineOrdering 读取管道排序、打印与报错时机的向量。
func PipelineOrdering() (PipelineOrderingVectors, error) {
	return typed[PipelineOrderingVectors]()
}

// JSONTextCase 是共同 JSON 读取检查的一条样例：Text 是原文（按 UTF-8 编码），Hex 是原始字节。
type JSONTextCase struct {
	Name   string  `json:"name"`
	Text   *string `json:"text,omitempty"`
	Hex    string  `json:"hex,omitempty"`
	Expect string  `json:"expect"`
	Path   string  `json:"path,omitempty"`
}

// Bytes 返回样例的原始字节。
func (c JSONTextCase) Bytes() ([]byte, error) {
	if c.Text != nil {
		return []byte(*c.Text), nil
	}
	return hex.DecodeString(c.Hex)
}

// JSONTextVectors 对应 primitives/json-text.json（RFC-0001「JSON 文本」）。
type JSONTextVectors struct {
	Cases []JSONTextCase `json:"cases"`
}

// JSONText 读取共同 JSON 读取检查的向量。
func JSONText() (JSONTextVectors, error) {
	return typed[JSONTextVectors]()
}

// PayloadReasons 读取载荷错误路径的向量。
func PayloadReasons() (PayloadReasonVectors, error) {
	return typed[PayloadReasonVectors]()
}
