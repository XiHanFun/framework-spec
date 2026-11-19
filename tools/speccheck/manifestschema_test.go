package speccheck_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// manifestCase 是 manifests.json 的一个用例；text 用例是 JSON 值表达不了的写法，在 Schema 之前就被拒绝。
type manifestCase struct {
	Name     string          `json:"name"`
	Manifest json.RawMessage `json:"manifest"`
	Text     *string         `json:"text"`
	Error    struct {
		MessageKey string `json:"messageKey"`
	} `json:"error"`
}

// manifestVectors 是 conformance/vectors/manifest/manifests.json 里与 Schema 有关的部分。
type manifestVectors struct {
	Valid   []manifestCase `json:"valid"`
	Invalid []manifestCase `json:"invalid"`
}

// compileSchema 按 2020-12 编译规范里的一个 Schema；编译即按元 Schema 核对它本身。
func compileSchema(t *testing.T, root, relative string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(readFile(t, root, relative)))
	if err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	location := filepath.Join(root, filepath.FromSlash(relative))
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(location, doc); err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	return schema
}

// validate 按 Schema 校验一份 JSON 值，返回校验错误；值本身读不出时测试失败。
func validate(t *testing.T, schema *jsonschema.Schema, name string, raw json.RawMessage) error {
	t.Helper()
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s: the manifest of the vector is not JSON: %v", name, err)
	}
	return schema.Validate(instance)
}

func TestSchemasCompile(t *testing.T) {
	root := repoRoot(t)
	for _, relative := range []string{"manifest/manifest.schema.json", "packaging/app.schema.json"} {
		compileSchema(t, root, relative)
	}
}

func TestManifestVectorsAgreeWithTheSchema(t *testing.T) {
	root := repoRoot(t)
	schema := compileSchema(t, root, "manifest/manifest.schema.json")
	var vectors manifestVectors
	if err := json.Unmarshal(readFile(t, root, "conformance/vectors/manifest/manifests.json"), &vectors); err != nil {
		t.Fatal(err)
	}

	accepted := 0
	for _, c := range vectors.Valid {
		if c.Text != nil {
			continue
		}
		accepted++
		if err := validate(t, schema, c.Name, c.Manifest); err != nil {
			t.Errorf("valid manifest %q must satisfy the schema: %v", c.Name, err)
		}
	}

	// RFC-0002 解析规则 2：第 3 步按 Schema 核对形状（kernel.manifest.invalid），
	// 第 4 步的跨字段约束（primitives.manifest.*）在形状合格之后才查，所以那些清单必须过得了 Schema
	rejected, crossField := 0, 0
	for _, c := range vectors.Invalid {
		if c.Text != nil {
			continue
		}
		key := c.Error.MessageKey
		switch {
		case key == "kernel.manifest.invalid":
			rejected++
			if err := validate(t, schema, c.Name, c.Manifest); err == nil {
				t.Errorf("manifest %q is rejected with %s, so the schema must reject it too", c.Name, key)
			}
		case strings.HasPrefix(key, "primitives.manifest."):
			crossField++
			if err := validate(t, schema, c.Name, c.Manifest); err != nil {
				t.Errorf("manifest %q is rejected by the cross-field step with %s, so it must satisfy the schema: %v", c.Name, key, err)
			}
		}
	}

	// 防止向量换了写法后这里一条都没核对还照样通过
	if accepted == 0 || rejected == 0 || crossField == 0 {
		t.Fatalf("checked %d valid, %d shape-invalid and %d cross-field manifests; each must be non-zero", accepted, rejected, crossField)
	}
}
