package speccheck_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// codePattern 匹配错误码文法（conventions/error-codes.md）：域、种类、三位序号；第一组是域。
var codePattern = regexp.MustCompile(`\b([A-Z]+)_[A-Z]+_[0-9]{3}\b`)

// tableCode 匹配登记表一行的首列错误码。
var tableCode = regexp.MustCompile("^\\|\\s*`([A-Z]+_[A-Z]+_[0-9]{3})`\\s*\\|")

// registry 是登记表的两部分：已登记与已撤销，值为在表里出现的次数。
type registry struct {
	registered map[string]int
	retired    map[string]int
}

// readRegistry 按二级标题把 conventions/error-codes.md 的表行分到已登记与已撤销。
func readRegistry(t *testing.T, root string) registry {
	t.Helper()
	r := registry{registered: map[string]int{}, retired: map[string]int{}}
	var section map[string]int
	for _, line := range strings.Split(string(readFile(t, root, "conventions/error-codes.md")), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "## ") {
			// 只有这两节的表是登记表，「规则」等其他节里的码不算登记
			switch strings.TrimSpace(strings.TrimPrefix(line, "## ")) {
			case "已登记的框架错误码":
				section = r.registered
			case "已撤销的错误码":
				section = r.retired
			default:
				section = nil
			}
			continue
		}
		if m := tableCode.FindStringSubmatch(line); m != nil && section != nil {
			section[m[1]]++
		}
	}
	return r
}

// readErrorDomains 取术语表第四列的错误码域（conventions/vocabulary.md）；「—」表示没有。
func readErrorDomains(t *testing.T, root string) map[string]bool {
	t.Helper()
	domains := map[string]bool{}
	for _, line := range strings.Split(string(readFile(t, root, "conventions/vocabulary.md")), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		domain := strings.Trim(strings.TrimSpace(cells[4]), "`")
		if domain != "" && domain != "—" {
			domains[domain] = true
		}
	}
	if len(domains) < 20 {
		t.Fatalf("only %d error code domains were read from conventions/vocabulary.md; the table probably failed to parse", len(domains))
	}
	return domains
}

func TestRegistryIsConsistent(t *testing.T) {
	root := repoRoot(t)
	r := readRegistry(t, root)
	domains := readErrorDomains(t, root)
	if len(r.registered) < 100 {
		t.Fatalf("only %d registered error codes were read; the table probably failed to parse", len(r.registered))
	}
	for _, part := range []struct {
		name  string
		codes map[string]int
	}{{"registered", r.registered}, {"retired", r.retired}} {
		for _, code := range sortedKeys(part.codes) {
			if part.codes[code] > 1 {
				t.Errorf("%s is %s %d times", code, part.name, part.codes[code])
			}
			if domain := codePattern.FindStringSubmatch(code)[1]; !domains[domain] {
				t.Errorf("%s is %s, but its domain %q is not an error code domain of the vocabulary", code, part.name, domain)
			}
		}
	}
	// 撤销的序号不复用：同一个码不能既撤销又登记
	for _, code := range sortedKeys(r.retired) {
		if r.registered[code] > 0 {
			t.Errorf("%s is both registered and retired", code)
		}
	}
}

func TestEveryFrameworkCodeIsRegistered(t *testing.T) {
	root := repoRoot(t)
	r := readRegistry(t, root)
	domains := readErrorDomains(t, root)
	var failures []string
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative := filepath.ToSlash(must(filepath.Rel(root, path)))
		if d.IsDir() {
			// 生成代码与构建产物不是规范的来源；本模块的测试数据也不算
			switch relative {
			case ".git", "tools", "conformance/internal", "conformance/bin":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".md", ".json", ".proto", ".go":
		default:
			return nil
		}
		if relative == "conventions/error-codes.md" {
			return nil
		}
		scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for number, line := range bytes.Split(data, []byte("\n")) {
			for _, m := range codePattern.FindAllSubmatch(line, -1) {
				code, domain := string(m[0]), string(m[1])
				// 域不在术语表里的是应用自定义的码（示例里的 ORDERS 等），不归登记表管
				if !domains[domain] || r.registered[code] > 0 || r.retired[code] > 0 {
					continue
				}
				failures = append(failures, fmt.Sprintf("%s:%d: %s", relative, number+1, code))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 50 {
		t.Fatalf("only %d files were scanned under %s", scanned, root)
	}
	if len(failures) > 0 {
		t.Errorf("framework error codes that conventions/error-codes.md neither registers nor retires:\n%s", strings.Join(failures, "\n"))
	}
}

// sortedKeys 按字典序返回映射的键，让失败信息的顺序固定。
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// must 取出不会失败的调用结果；WalkDir 给出的路径总在根之下。
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
