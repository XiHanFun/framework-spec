package speccheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot 返回规范仓库的根目录：本模块在 tools/speccheck，根在上两级。
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// 以 README 与 conventions 目录确认真的是规范仓库，避免在别处跑出空结果却通过
	for _, marker := range []string{"README.md", filepath.Join("conventions", "error-codes.md")} {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			t.Fatalf("%s is not the spec repository root: %v", root, err)
		}
	}
	return root
}

// readFile 读取相对规范仓库根的文件。
func readFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// vocabulary 是术语表（conventions/vocabulary.md）里门禁用到的两列。
type vocabulary struct {
	// capabilityPrefixes 是第二列的能力 ID 前缀：子系统档的档名取自它。
	capabilityPrefixes map[string]bool
	// errorDomains 是第四列的错误码域：域在其中的错误码归登记表管。
	errorDomains map[string]bool
}

// readVocabulary 解析术语表：每个领域词一行，某一列写「—」表示没有。
func readVocabulary(t *testing.T, root string) vocabulary {
	t.Helper()
	v := vocabulary{capabilityPrefixes: map[string]bool{}, errorDomains: map[string]bool{}}
	for _, line := range strings.Split(string(readFile(t, root, "conventions/vocabulary.md")), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		// 行以竖线开头，cells[0] 是空串：领域词在 cells[1]，能力 ID 前缀在 cells[2]，错误码域在 cells[4]
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		for column, values := range map[int]map[string]bool{2: v.capabilityPrefixes, 4: v.errorDomains} {
			if value := strings.Trim(strings.TrimSpace(cells[column]), "`"); value != "" && value != "—" {
				values[value] = true
			}
		}
	}
	if len(v.errorDomains) < 20 {
		t.Fatalf("only %d error code domains were read from conventions/vocabulary.md; the table probably failed to parse", len(v.errorDomains))
	}
	return v
}
