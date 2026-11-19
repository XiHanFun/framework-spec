package conformance

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// profilesJSON 是一致性档表 profiles.json（RFC-0024）的原文。
//
//go:embed profiles.json
var profilesJSON []byte

// Profile 是一个一致性档：认领它的实现必须遵守 Documents、回放 Vectors、通过 Runner 列出的运行器子命令。
type Profile struct {
	// ID 是档名：core、plugin、host、wasm、子系统的能力 ID 前缀，或 provider:<操作集>。
	ID string `json:"id"`
	// Title 是给人看的说明。
	Title string `json:"title"`
	// Stability 是 stable 或 experimental；认领 experimental 的档不影响实现的 Stable 判定。
	Stability string `json:"stability"`
	// Documents 是该档的规范文件，相对规范仓库根。
	Documents []string `json:"documents"`
	// Vectors 是该档必须回放的向量文件，相对 vectors/（与 Names 的写法相同）。
	Vectors []string `json:"vectors"`
	// Runner 是该档必须通过的黑盒运行器子命令，如 plugin。
	Runner []string `json:"runner"`
}

// ProfileTable 对应 profiles.json。
type ProfileTable struct {
	Profiles []Profile `json:"profiles"`
}

// Profiles 读取内嵌的一致性档表。
func Profiles() (ProfileTable, error) {
	var table ProfileTable
	if err := json.Unmarshal(profilesJSON, &table); err != nil {
		return table, fmt.Errorf("conformance: parse profiles.json: %w", err)
	}
	return table, nil
}

// Profile 按档名取一个档；没有这个档时 ok 为 false。
func (t ProfileTable) Profile(id string) (profile Profile, ok bool) {
	for _, p := range t.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// RequiredVectors 返回认领了给定这些档的实现必须回放的向量文件（去重、保持表里的先后）；有未知的档名时报错。
func (t ProfileTable) RequiredVectors(ids ...string) ([]string, error) {
	seen := map[string]bool{}
	var vectors []string
	for _, id := range ids {
		profile, ok := t.Profile(id)
		if !ok {
			return nil, fmt.Errorf("conformance: unknown profile %q", id)
		}
		for _, v := range profile.Vectors {
			if !seen[v] {
				seen[v] = true
				vectors = append(vectors, v)
			}
		}
	}
	return vectors, nil
}
