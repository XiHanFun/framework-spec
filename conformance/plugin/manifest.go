package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ManifestFileName 是插件目录里的清单文件名。
const ManifestFileName = "xihan.manifest.json"

// Manifest 是运行器用到的清单字段（RFC-0002）；完整结构按 manifest/manifest.schema.json。
type Manifest struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Primitives  string `json:"primitives"`
	Kind        string `json:"kind"`
	Isolation   string `json:"isolation"`
	Contributes struct {
		Operations []string `json:"operations"`
		Health     []string `json:"health"`
	} `json:"contributes"`
	Permissions []string `json:"permissions"`
	Activation  []string `json:"activation"`
	Entry       string   `json:"entry"`
}

// comparedFields 是 handshake.md §3 要求握手清单与文件逐字段一致的字段。
var comparedFields = []string{"id", "version", "primitives", "kind", "isolation", "provides", "requires", "contributes", "activation", "permissions"}

// readManifest 读取插件目录里的清单，同时给出类型化字段与原始对象。
func readManifest(directory string) (Manifest, map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(directory, ManifestFileName))
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("read %s: %w", ManifestFileName, err)
	}
	return parseManifest(data)
}

// parseManifest 解析清单 JSON。
func parseManifest(data []byte) (Manifest, map[string]any, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, nil, fmt.Errorf("parse manifest: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Manifest{}, nil, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.ID == "" || manifest.Version == "" || manifest.Primitives == "" || manifest.Kind == "" {
		return Manifest{}, nil, fmt.Errorf("manifest is missing id, version, primitives or kind")
	}
	return manifest, raw, nil
}

// sameManifest 按 comparedFields 比较两份清单；返回第一个不一致的字段名。
// 缺省的数组与对象按空处理，activation 缺省为 onStartup，所以两边写法不同但语义相同时仍视为一致。
func sameManifest(file, handshake map[string]any) (string, bool) {
	for _, field := range comparedFields {
		left := canonical(field, file[field])
		right := canonical(field, handshake[field])
		if !bytes.Equal(left, right) {
			return field, false
		}
	}
	return "", true
}

// canonical 给出一个字段的规范 JSON：剪掉空值后序列化（encoding/json 按键排序）。
func canonical(field string, value any) []byte {
	pruned := prune(value)
	if pruned == nil && field == "activation" {
		pruned = []any{"onStartup"}
	}
	data, err := json.Marshal(pruned)
	if err != nil {
		return nil
	}
	return data
}

// prune 递归删除 nil、空字符串、空数组与空对象，返回 nil 表示整个值为空。
func prune(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case []any:
		var kept []any
		for _, item := range typed {
			if pruned := prune(item); pruned != nil {
				kept = append(kept, pruned)
			}
		}
		if len(kept) == 0 {
			return nil
		}
		return kept
	case map[string]any:
		kept := map[string]any{}
		for key, item := range typed {
			if pruned := prune(item); pruned != nil {
				kept[key] = pruned
			}
		}
		if len(kept) == 0 {
			return nil
		}
		return kept
	default:
		return value
	}
}
