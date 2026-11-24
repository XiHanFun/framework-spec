package speccheck_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// capabilityID 是能力 ID 的文法（conventions/capability-ids.md）：<领域>.<能力>。
var capabilityID = regexp.MustCompile(`^([a-z][a-z0-9]*)\.[a-z][a-z0-9-]*$`)

// reservedMark 是登记表里保留项的标注。
const reservedMark = "保留，尚无实现"

// capabilityRegistry 是 conventions/capability-ids.md 里门禁用到的两张表。
type capabilityRegistry struct {
	// registered 是「已登记的能力」里的全部 ID，按出现的顺序。
	registered []string
	// marked 是「已登记的能力」里标了保留的 ID。
	marked []string
	// reserved 是「保留、尚无实现的能力」一节列出的 ID。
	reserved []string
}

// readCapabilityRegistry 解析能力登记表：一行可以登记几个 ID（以 / 分隔），含义一栏写了「保留，尚无实现」的是保留项；
// 一行几个 ID 而只保留其中一个时，标注写成「（`<ID>` 保留，尚无实现）」。
func readCapabilityRegistry(t *testing.T, root string) capabilityRegistry {
	t.Helper()
	var registry capabilityRegistry
	section := ""
	for _, line := range strings.Split(string(readFile(t, root, "conventions/capability-ids.md")), "\n") {
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimPrefix(line, "## ")
			continue
		}
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		var ids []string
		for _, id := range backticked.FindAllStringSubmatch(cells[1], -1) {
			ids = append(ids, id[1])
		}
		meaning := cells[2]
		switch section {
		case "已登记的能力":
			registry.registered = append(registry.registered, ids...)
			if !strings.Contains(meaning, reservedMark) {
				continue
			}
			// 标注点了名的只算点到的那几个，否则这一行的 ID 都是保留项
			named := 0
			for _, id := range ids {
				if strings.Contains(meaning, "`"+id+"` 保留") {
					registry.marked = append(registry.marked, id)
					named++
				}
			}
			if named == 0 {
				registry.marked = append(registry.marked, ids...)
			}
		case "保留、尚无实现的能力":
			registry.reserved = append(registry.reserved, ids...)
		}
	}
	if len(registry.registered) < 40 {
		t.Fatalf("only %d capability IDs were read from conventions/capability-ids.md; the table probably failed to parse", len(registry.registered))
	}
	return registry
}

func TestCapabilityRegistryIsConsistent(t *testing.T) {
	root := repoRoot(t)
	registry := readCapabilityRegistry(t, root)
	prefixes := readVocabulary(t, root).capabilityPrefixes

	seen := map[string]bool{}
	for _, id := range registry.registered {
		if seen[id] {
			t.Errorf("capability %q is registered twice", id)
		}
		seen[id] = true

		m := capabilityID.FindStringSubmatch(id)
		switch {
		case m == nil:
			// 不合文法的 ID 写不进清单，只能作为保留项留着（ID 一经登记不删）
			if !slices.Contains(registry.marked, id) {
				t.Errorf("capability %q does not follow the grammar <domain>.<capability>; only a reserved ID may be malformed", id)
			}
		case !prefixes[m[1]]:
			t.Errorf("capability %q uses the domain %q, which is not a capability prefix of the vocabulary", id, m[1])
		}
	}

	// 表里的标注与「保留」一节一一对应：只改了一处的时候在这里失败
	marked, reserved := slices.Clone(registry.marked), slices.Clone(registry.reserved)
	slices.Sort(marked)
	slices.Sort(reserved)
	if !slices.Equal(marked, reserved) {
		t.Errorf("the registry marks %v as reserved, but the reserved section lists %v", marked, reserved)
	}
	if len(reserved) == 0 {
		t.Error("the reserved section lists no capability; drop the section and this check together when every capability is implemented")
	}
}
