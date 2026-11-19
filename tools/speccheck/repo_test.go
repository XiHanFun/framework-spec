package speccheck_test

import (
	"os"
	"path/filepath"
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
