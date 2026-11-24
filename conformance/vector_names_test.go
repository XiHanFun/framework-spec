package conformance_test

import (
	"path"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
)

func TestVectorFileNamesAreUniqueAcrossDirectories(t *testing.T) {
	names, err := conformance.Names()
	if err != nil {
		t.Fatal(err)
	}
	// 实现的向量覆盖测试常按文件名（不带目录）认回放：两个目录里有同名文件就分不出回放的是哪一个
	directories := map[string]string{}
	for _, name := range names {
		base := path.Base(name)
		if other, seen := directories[base]; seen {
			t.Errorf("the vector file name %q appears in both %q and %q; file names must be unique across directories", base, other, path.Dir(name))
		}
		directories[base] = path.Dir(name)
	}
}
