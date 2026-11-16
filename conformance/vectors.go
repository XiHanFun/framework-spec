// Package conformance 内嵌 XiHan Framework Spec 的一致性向量并提供类型化的读取入口；
// 黑盒运行器在子包 plugin，规范性文法在子包 rules。
package conformance

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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

// WebAuthn 读取 WebAuthn 注册与认证的样例。
func WebAuthn() (WebAuthnVectors, error) {
	return Load[WebAuthnVectors]("security/webauthn.json")
}

// CachingDistributed 读取 caching.distributed 远程 Provider 的操作集向量。
func CachingDistributed() (CachingDistributedVectors, error) {
	return Load[CachingDistributedVectors]("provider/caching-distributed.json")
}

// HandshakeLines 读取握手行向量。
func HandshakeLines() (HandshakeLineVectors, error) {
	return Load[HandshakeLineVectors]("plugin/handshake-lines.json")
}

// ProtocolVersions 读取版本协商向量。
func ProtocolVersions() (ProtocolVersionVectors, error) {
	return Load[ProtocolVersionVectors]("plugin/protocol-versions.json")
}

// WasmExports 读取 Wasm ABI 导出表向量。
func WasmExports() (WasmExportVectors, error) {
	return Load[WasmExportVectors]("plugin/wasm-exports.json")
}

// ErrorCodes 读取错误码格式向量。
func ErrorCodes() (ErrorCodeVectors, error) {
	return Load[ErrorCodeVectors]("primitives/error-codes.json")
}

// BaggageKeys 读取 Baggage 键向量。
func BaggageKeys() (BaggageKeyVectors, error) {
	return Load[BaggageKeyVectors]("primitives/baggage-keys.json")
}

// CapabilityRequirements 读取能力依赖文本向量。
func CapabilityRequirements() (CapabilityRequirementVectors, error) {
	return Load[CapabilityRequirementVectors]("primitives/capability-requirements.json")
}

// LifecycleTransitions 读取生命周期迁移向量。
func LifecycleTransitions() (LifecycleTransitionVectors, error) {
	return Load[LifecycleTransitionVectors]("primitives/lifecycle-transitions.json")
}

// WireEncoding 读取 Result 与 Error 的线上编码向量。
func WireEncoding() (WireEncodingVectors, error) {
	return Load[WireEncodingVectors]("primitives/wire-encoding.json")
}

// MessageTemplates 读取消息键文法、模板格式化与 Describe 向量。
func MessageTemplates() (MessageTemplateVectors, error) {
	return Load[MessageTemplateVectors]("primitives/message-templates.json")
}

// KernelMessages 读取 SDK 与宿主共用的内核错误向量。
func KernelMessages() (KernelMessageVectors, error) {
	return Load[KernelMessageVectors]("primitives/kernel-messages.json")
}

// TraceContext 读取 traceparent 的解析与写出向量。
func TraceContext() (TraceContextVectors, error) {
	return Load[TraceContextVectors]("primitives/trace-context.json")
}

// Manifests 读取清单读取的向量。
func Manifests() (ManifestVectors, error) {
	return Load[ManifestVectors]("manifest/manifests.json")
}

// PipelineOrdering 读取管道排序、打印与报错时机的向量。
func PipelineOrdering() (PipelineOrderingVectors, error) {
	return Load[PipelineOrderingVectors]("pipeline/ordering.json")
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
	return Load[JSONTextVectors]("primitives/json-text.json")
}

// PayloadReasons 读取载荷错误路径的向量。
func PayloadReasons() (PayloadReasonVectors, error) {
	return Load[PayloadReasonVectors]("operations/payload-reasons.json")
}
