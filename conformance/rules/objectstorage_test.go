package rules_test

import (
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

// replayGrammar 回放一组必须接受与必须拒绝的文本，并核对上限两侧各有一条：正好 max 个字符的合法文本与 max+1 个字符的不合法文本。
func replayGrammar(t *testing.T, what string, max int, valid, invalid []string, accepts func(string) bool) {
	t.Helper()
	atLimit, overLimit := false, false
	for _, text := range valid {
		if !accepts(text) {
			t.Errorf("%q must be a valid %s", text, what)
		}
		atLimit = atLimit || len(text) == max
	}
	for _, text := range invalid {
		if accepts(text) {
			t.Errorf("%q must be an invalid %s", text, what)
		}
		overLimit = overLimit || len(text) == max+1
	}
	if !atLimit || !overLimit {
		t.Errorf("the %s vector must keep a text of exactly %d characters (found: %v) and one of %d (found: %v)", what, max, atLimit, max+1, overLimit)
	}
}

func TestContainerNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.ContainerNames()
	if err != nil {
		t.Fatal(err)
	}
	replayGrammar(t, "container name", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsContainerName)
	// 下限同样两侧各一条
	shortest, tooShort := false, false
	for _, name := range vectors.Valid {
		shortest = shortest || len(name) == vectors.MinLength
	}
	for _, name := range vectors.Invalid {
		tooShort = tooShort || len(name) == vectors.MinLength-1
	}
	if vectors.MinLength != 3 || vectors.MaxLength != 63 || !shortest || !tooShort {
		t.Errorf("container names are 3 to 63 characters; the vector says %d to %d and must keep both sides of the lower bound (%v, %v)", vectors.MinLength, vectors.MaxLength, shortest, tooShort)
	}
}

// lengthCoverage 记下一组样例盖住了长度上限的哪几处：正好到上限的合法样例、超出 1 字节的不合法样例，
// 以及码元数不超而字节数超过的不合法样例（只按 UTF-16 码元计的实现会放过它）。
type lengthCoverage struct {
	atLimit, overByOne, overOnlyInBytes bool
}

// checkLengths 核对向量给一条样例标的两个计数属实（字节数是规范的判据，码元数只用来标出按码元计会判错的用例），并记下它盖住的边界。
func (c *lengthCoverage) checkLengths(t *testing.T, group, name, text string, utf8Bytes, utf16Units, max int, valid bool) {
	t.Helper()
	if bytes := len(text); bytes != utf8Bytes {
		t.Errorf("%s %q: utf8Bytes = %d, but the text is %d bytes", group, name, utf8Bytes, bytes)
	}
	if units := len(utf16.Encode([]rune(text))); units != utf16Units {
		t.Errorf("%s %q: utf16Units = %d, but the text is %d code units", group, name, utf16Units, units)
	}
	c.atLimit = c.atLimit || (valid && utf8Bytes == max)
	c.overByOne = c.overByOne || (!valid && utf8Bytes == max+1)
	c.overOnlyInBytes = c.overOnlyInBytes || (!valid && utf8Bytes > max && utf16Units <= max)
}

// requireAll 要求三处边界都有样例：少了哪一处，改单位的那一类就没有被向量盯住。
func (c *lengthCoverage) requireAll(t *testing.T, what string, max int) {
	t.Helper()
	if !c.atLimit || !c.overByOne || !c.overOnlyInBytes {
		t.Errorf("the %s vector must keep a text of exactly %d bytes (found: %v), one of %d bytes (found: %v) and one over the limit only when counted in bytes (found: %v)",
			what, max, c.atLimit, max+1, c.overByOne, c.overOnlyInBytes)
	}
}

func TestBlobKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BlobKeys()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxBytes != rules.MaxBlobKeyBytes || vectors.Separator != rules.BlobKeySeparator || vectors.ReservedSegment != rules.BlobKeyReservedSegment {
		t.Fatalf("maxBytes, separator and reserved segment = %d, %q, %q; want %d, %q, %q", vectors.MaxBytes, vectors.Separator, vectors.ReservedSegment,
			rules.MaxBlobKeyBytes, rules.BlobKeySeparator, rules.BlobKeyReservedSegment)
	}
	var keys, prefixes lengthCoverage
	for _, c := range vectors.Valid {
		keys.checkLengths(t, "valid", c.Name, c.Key, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, true)
		if !rules.IsBlobKey(c.Key) || rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be a valid key outside the reserved prefix", c.Name, c.Key)
		}
	}
	for _, c := range vectors.Invalid {
		keys.checkLengths(t, "invalid", c.Name, c.Key, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, false)
		if rules.IsBlobKey(c.Key) || rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be an invalid key that is not under the reserved prefix", c.Name, c.Key)
		}
	}
	for _, c := range vectors.Reserved {
		// 保留前缀下的键不参与长度边界的覆盖：它们因保留而不合法，与长度无关
		new(lengthCoverage).checkLengths(t, "reserved", c.Name, c.Key, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, false)
		if rules.IsBlobKey(c.Key) || !rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be rejected as a key under the reserved prefix", c.Name, c.Key)
		}
	}
	for _, c := range vectors.Prefixes.Valid {
		prefixes.checkLengths(t, "prefixes.valid", c.Name, c.Prefix, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, true)
		if !rules.IsBlobKeyPrefix(c.Prefix) {
			t.Errorf("%s: %q must be a valid prefix", c.Name, c.Prefix)
		}
	}
	for _, c := range vectors.Prefixes.Invalid {
		prefixes.checkLengths(t, "prefixes.invalid", c.Name, c.Prefix, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, false)
		if rules.IsBlobKeyPrefix(c.Prefix) {
			t.Errorf("%s: %q must be an invalid prefix", c.Name, c.Prefix)
		}
	}
	keys.requireAll(t, "blob key", vectors.MaxBytes)
	prefixes.requireAll(t, "blob key prefix", vectors.MaxBytes)
	if !rules.IsBlobKeyPrefix("") || rules.IsBlobKey("") {
		t.Error("the empty text is a valid prefix but not a valid key")
	}
}

func TestDownloadFileNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.DownloadFileNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxBytes != rules.MaxBlobDownloadFileNameBytes {
		t.Fatalf("maxBytes = %d, want %d", vectors.MaxBytes, rules.MaxBlobDownloadFileNameBytes)
	}
	var names lengthCoverage
	for _, c := range vectors.Valid {
		names.checkLengths(t, "valid", c.Name, c.Text, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, true)
		if !rules.IsBlobDownloadFileName(c.Text) {
			t.Errorf("%s: %q must be a valid download file name", c.Name, c.Text)
		}
	}
	for _, c := range vectors.Invalid {
		names.checkLengths(t, "invalid", c.Name, c.Text, c.UTF8Bytes, c.UTF16Units, vectors.MaxBytes, false)
		if rules.IsBlobDownloadFileName(c.Text) {
			t.Errorf("%s: %q must be an invalid download file name", c.Name, c.Text)
		}
	}
	names.requireAll(t, "download file name", vectors.MaxBytes)
}

func TestBlobMetadataFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.BlobMetadata()
	if err != nil {
		t.Fatal(err)
	}
	replayGrammar(t, "metadata name", vectors.Names.MaxLength, vectors.Names.Valid, vectors.Names.Invalid, rules.IsBlobMetadataName)
	if vectors.MaxBytes != rules.MaxBlobMetadataBytes {
		t.Fatalf("maxBytes = %d, want %d", vectors.MaxBytes, rules.MaxBlobMetadataBytes)
	}

	atLimit, overOnlyInBytes := false, false
	check := func(group string, cases []conformance.BlobMetadataCase, want bool) {
		for _, c := range cases {
			bytes, units := 0, 0
			for name, value := range c.Metadata {
				bytes += len(name) + len(value)
				units += len(utf16.Encode([]rune(name))) + len(utf16.Encode([]rune(value)))
			}
			if bytes != c.UTF8Bytes {
				t.Errorf("%s %q: utf8Bytes = %d, but the names and values total %d bytes", group, c.Name, c.UTF8Bytes, bytes)
			}
			if got := rules.IsBlobMetadata(c.Metadata); got != want {
				t.Errorf("%s %q: accepted = %v, want %v", group, c.Name, got, want)
			}
			atLimit = atLimit || (want && bytes == vectors.MaxBytes)
			overOnlyInBytes = overOnlyInBytes || (!want && bytes > vectors.MaxBytes && units <= vectors.MaxBytes)
		}
	}
	check("sets.valid", vectors.Sets.Valid, true)
	check("sets.invalid", vectors.Sets.Invalid, false)
	// 两类边界缺一不可：正好到上限的元数据，以及码元数不超、字节数超过的元数据（只按码元计的实现会放过它）
	if !atLimit || !overOnlyInBytes {
		t.Errorf("the vector must keep metadata of exactly %d bytes (found: %v) and metadata over the limit only when counted in bytes (found: %v)", vectors.MaxBytes, atLimit, overOnlyInBytes)
	}
}

func TestBlobContentTypesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BlobContentTypes()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxBlobContentTypeLength || vectors.Default != rules.DefaultBlobContentType {
		t.Fatalf("maxLength and default = %d, %q; want %d, %q", vectors.MaxLength, vectors.Default, rules.MaxBlobContentTypeLength, rules.DefaultBlobContentType)
	}
	replayGrammar(t, "content type", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsBlobContentType)
	if !rules.IsBlobContentType(vectors.Default) {
		t.Errorf("the default content type %q must be valid", vectors.Default)
	}
	// ADR-0282 写明与 ADR-0295 放宽的几类写法各要有样例盯住：; 两侧的空格要收，; 之后为空的要收，制表符在 ; 的哪一侧都要拒
	for _, required := range []struct {
		text  string
		valid bool
	}{
		{"text/plain ; charset=utf-8", true},
		{"text/plain;", true},
		{"text/plain; ", true},
		{"text/plain; charset=utf-8;", true},
		{"text/plain;;charset=utf-8", true},
		{"text/plain;\tcharset=utf-8", false},
		{"text/plain\t;charset=utf-8", false},
		{"text/plain;\t", false},
		{"text/plain ", false},
	} {
		group := vectors.Invalid
		if required.valid {
			group = vectors.Valid
		}
		if !slices.Contains(group, required.text) {
			t.Errorf("the content type vector must keep %q (valid: %v)", required.text, required.valid)
		}
	}
}

func TestBlobRangesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BlobRanges()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) {
		t.Errorf("%q is not an error code", vectors.Code)
	}
	for _, c := range vectors.Cases {
		got, ok := rules.ResolveBlobRange(c.Offset, c.Length, c.Total)
		if ok != (c.Range != nil) {
			t.Errorf("%s: satisfiable = %v, want %v", c.Name, ok, c.Range != nil)
			continue
		}
		if c.Range != nil && (got.Offset != c.Range.Offset || got.Length != c.Range.Length || got.Total != c.Range.Total) {
			t.Errorf("%s: range = %+v, want %+v", c.Name, got, *c.Range)
		}
	}
}

func TestMultipartPartsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.MultipartParts()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxPartNumber != rules.MaxBlobPartNumber || vectors.MinPartBytes != rules.MinBlobPartBytes || !rules.IsErrorCode(vectors.Code) {
		t.Fatalf("maxPartNumber, minPartBytes and code = %d, %d, %q; want %d, %d and an error code", vectors.MaxPartNumber, vectors.MinPartBytes, vectors.Code, rules.MaxBlobPartNumber, rules.MinBlobPartBytes)
	}
	for _, number := range vectors.PartNumbers.Valid {
		if !rules.IsBlobPartNumber(number) {
			t.Errorf("part number %d must be in range", number)
		}
	}
	for _, number := range vectors.PartNumbers.Invalid {
		if rules.IsBlobPartNumber(number) {
			t.Errorf("part number %d must be out of range", number)
		}
	}
	for _, c := range vectors.Manifests {
		if got := rules.IsBlobPartManifest(c.Parts); got != c.Valid {
			t.Errorf("%s: manifest %v valid = %v, want %v", c.Name, c.Parts, got, c.Valid)
		}
	}
}

func TestBlobStoreDefinitionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BlobStoreDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	replayGrammar(t, "store name", vectors.Names.MaxLength, vectors.Names.Valid, vectors.Names.Invalid, rules.IsBlobStoreName)
	if len(vectors.Providers) != len(rules.BlobStoreProviders) {
		t.Errorf("providers = %v, want %v", vectors.Providers, rules.BlobStoreProviders)
	}
	for _, provider := range vectors.Providers {
		if !rules.IsBlobStoreProvider(provider) {
			t.Errorf("%q must be a built-in store provider", provider)
		}
	}
	definition := func(d conformance.BlobStoreDefinitionCase) rules.BlobStoreDefinition {
		return rules.BlobStoreDefinition{Name: d.Name, Provider: d.Provider, Settings: d.Settings}
	}
	for _, c := range vectors.Same {
		// 结论与两个定义的先后无关
		if got := rules.SameBlobStoreDefinition(definition(c.Left), definition(c.Right)); got != c.Same {
			t.Errorf("%s: same = %v, want %v", c.Name, got, c.Same)
		}
		if got := rules.SameBlobStoreDefinition(definition(c.Right), definition(c.Left)); got != c.Same {
			t.Errorf("%s (swapped): same = %v, want %v", c.Name, got, c.Same)
		}
		if !rules.IsBlobStoreName(c.Left.Name) || !rules.IsBlobStoreName(c.Right.Name) {
			t.Errorf("%s: both definitions must have valid store names", c.Name)
		}
	}
}
