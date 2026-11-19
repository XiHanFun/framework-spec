package speccheck_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// profileTable 是 conformance/profiles.json 里本测试用到的部分。
type profileTable struct {
	Profiles []struct {
		ID        string   `json:"id"`
		Documents []string `json:"documents"`
	} `json:"profiles"`
}

// roleProfiles 是不按子系统划分的档；其余档要么是子系统（术语表的能力 ID 前缀），要么是 provider:<操作集>。
var roleProfiles = []string{"core", "plugin", "host", "wasm"}

// normativeDirectories 是放规范性文件的目录；conformance 与 tools 不在其中。
var normativeDirectories = []string{"rfcs", "conventions", "manifest", "plugin", "protocol", "provider", "packaging", "migration"}

// rfcProfiles 匹配 RFC 状态行里的「一致性档：」标注，捕获反引号括起的档名列表。
var rfcProfiles = regexp.MustCompile("一致性档：((?:`[^`]+`(?:、)?)+)")

// backticked 取出反引号括起的各项。
var backticked = regexp.MustCompile("`([^`]+)`")

// readProfiles 读取规范仓库里的一致性档表。
func readProfiles(t *testing.T, root string) profileTable {
	t.Helper()
	var table profileTable
	if err := json.Unmarshal(readFile(t, root, "conformance/profiles.json"), &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Profiles) == 0 {
		t.Fatal("conformance/profiles.json defines no profile")
	}
	return table
}

// readCapabilityPrefixes 取术语表第二列的能力 ID 前缀（conventions/vocabulary.md）。
func readCapabilityPrefixes(t *testing.T, root string) map[string]bool {
	t.Helper()
	prefixes := map[string]bool{}
	for _, line := range strings.Split(string(readFile(t, root, "conventions/vocabulary.md")), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		prefix := strings.Trim(strings.TrimSpace(cells[2]), "`")
		if prefix != "" && prefix != "—" {
			prefixes[prefix] = true
		}
	}
	return prefixes
}

func TestEveryNormativeDocumentBelongsToAProfile(t *testing.T) {
	root := repoRoot(t)
	table := readProfiles(t, root)

	listed := map[string]bool{}
	for _, p := range table.Profiles {
		for _, document := range p.Documents {
			listed[document] = true
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(document))); err != nil {
				t.Errorf("profile %q lists the document %q, which does not exist", p.ID, document)
			}
		}
	}

	scanned := 0
	for _, directory := range normativeDirectories {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".md", ".json", ".proto":
			default:
				return nil
			}
			scanned++
			relative := filepath.ToSlash(must(filepath.Rel(root, path)))
			if !listed[relative] {
				t.Errorf("normative document %q belongs to no profile in conformance/profiles.json", relative)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 40 {
		t.Fatalf("only %d normative documents were scanned", scanned)
	}
}

func TestProfileIdsComeFromTheVocabulary(t *testing.T) {
	root := repoRoot(t)
	table := readProfiles(t, root)
	prefixes := readCapabilityPrefixes(t, root)

	for _, p := range table.Profiles {
		switch {
		case slices.Contains(roleProfiles, p.ID):
		case strings.HasPrefix(p.ID, "provider:"):
			// 操作集 caching.distributed@1 的规范文件是 provider/caching-distributed.md
			set, _, _ := strings.Cut(strings.TrimPrefix(p.ID, "provider:"), "@")
			document := "provider/" + strings.ReplaceAll(set, ".", "-") + ".md"
			if !slices.Contains(p.Documents, document) {
				t.Errorf("profile %q must list its operation set document %q", p.ID, document)
			}
			if domain, _, _ := strings.Cut(set, "."); !prefixes[domain] {
				t.Errorf("profile %q names the operation set of %q, which is not a capability prefix of the vocabulary", p.ID, domain)
			}
		default:
			if !prefixes[p.ID] {
				t.Errorf("subsystem profile %q is not a capability prefix of the vocabulary", p.ID)
			}
		}
	}
}

func TestRfcHeadersNameTheirProfiles(t *testing.T) {
	root := repoRoot(t)
	table := readProfiles(t, root)

	expected := map[string][]string{}
	for _, p := range table.Profiles {
		for _, document := range p.Documents {
			if strings.HasPrefix(document, "rfcs/") {
				expected[document] = append(expected[document], p.ID)
			}
		}
	}

	entries, err := os.ReadDir(filepath.Join(root, "rfcs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		document := "rfcs/" + entry.Name()
		header := ""
		for _, line := range strings.Split(string(readFile(t, root, document)), "\n") {
			if strings.HasPrefix(line, "> 状态：") {
				header = line
				break
			}
		}
		m := rfcProfiles.FindStringSubmatch(header)
		if m == nil {
			t.Errorf("%s: the status line must name its profiles as 一致性档：`…`", document)
			continue
		}
		var named []string
		for _, b := range backticked.FindAllStringSubmatch(m[1], -1) {
			named = append(named, b[1])
		}
		// 状态行与档表按同一个先后（档表里档的次序）写，比较时不看次序
		want := slices.Clone(expected[document])
		slices.Sort(named)
		slices.Sort(want)
		if !slices.Equal(named, want) {
			t.Errorf("%s: the status line names the profiles %v, but conformance/profiles.json lists it under %v", document, named, want)
		}
	}
}
