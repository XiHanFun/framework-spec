// Package conformance 内嵌 XiHan Framework Spec 的一致性向量并提供类型化的读取入口；
// 黑盒运行器在子包 plugin，规范性文法在子包 rules。
package conformance

import (
	"embed"
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
	WellFormed []string `json:"wellFormed"`
	IllFormed  []string `json:"illFormed"`
	Registered []string `json:"registered"`
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
