package conformance_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
)

// profileID 是档名的文法（RFC-0024）：小写词，或 provider: 加操作集 ID 与大版本。
var profileID = regexp.MustCompile(`^(?:[a-z][a-z0-9]*|provider:[a-z][a-z0-9]*\.[a-z][a-z0-9-]*@[1-9][0-9]*)$`)

func TestProfilesCoverEveryVectorFile(t *testing.T) {
	table, err := conformance.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	names, err := conformance.Names()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := table.Profile("core"); !ok {
		t.Fatal("profiles.json must define the profile 'core'")
	}

	ids := map[string]bool{}
	covered := map[string]bool{}
	for _, p := range table.Profiles {
		if !profileID.MatchString(p.ID) {
			t.Errorf("profile id %q does not follow the grammar", p.ID)
		}
		if ids[p.ID] {
			t.Errorf("profile %q is defined twice", p.ID)
		}
		ids[p.ID] = true
		if p.Stability != "stable" && p.Stability != "experimental" {
			t.Errorf("profile %q has stability %q, want stable or experimental", p.ID, p.Stability)
		}
		if p.Title == "" || len(p.Documents) == 0 {
			t.Errorf("profile %q must have a title and at least one document", p.ID)
		}
		for _, v := range p.Vectors {
			if !slices.Contains(names, v) {
				t.Errorf("profile %q lists the vector %q, which does not exist", p.ID, v)
			}
			covered[v] = true
		}
		for _, r := range p.Runner {
			// 运行器目前只有 plugin 子命令按档要求通过；vectors 子命令只校验向量文件本身
			if r != "plugin" {
				t.Errorf("profile %q requires the runner subcommand %q, which is not a conformance subcommand", p.ID, r)
			}
		}
	}

	// 每个向量文件至少属于一个档，否则没有任何实现必须回放它
	for _, name := range names {
		if !covered[name] {
			t.Errorf("vector %q belongs to no profile", name)
		}
	}
}

func TestRequiredVectorsUnionTheProfiles(t *testing.T) {
	table, err := conformance.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	core, _ := table.Profile("core")
	plugin, _ := table.Profile("plugin")

	required, err := table.RequiredVectors("core", "plugin", "core")
	if err != nil {
		t.Fatal(err)
	}
	if len(required) != len(core.Vectors)+len(plugin.Vectors) {
		t.Errorf("core + plugin require %d vectors, want %d without duplicates", len(required), len(core.Vectors)+len(plugin.Vectors))
	}
	if _, err := table.RequiredVectors("core", "bogus"); err == nil {
		t.Error("an unknown profile must be an error")
	}
}
