package rules_test

import (
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

func TestBlobKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BlobKeys()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxBlobKeyLength || vectors.Separator != rules.BlobKeySeparator || vectors.ReservedSegment != rules.BlobKeyReservedSegment {
		t.Fatalf("maxLength, separator and reserved segment = %d, %q, %q; want %d, %q, %q", vectors.MaxLength, vectors.Separator, vectors.ReservedSegment,
			rules.MaxBlobKeyLength, rules.BlobKeySeparator, rules.BlobKeyReservedSegment)
	}
	atLimit, overLimit := false, false
	for _, c := range vectors.Valid {
		if !rules.IsBlobKey(c.Key) || rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be a valid key outside the reserved prefix", c.Name, c.Key)
		}
		atLimit = atLimit || len(c.Key) == vectors.MaxLength
	}
	for _, c := range vectors.Invalid {
		if rules.IsBlobKey(c.Key) || rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be an invalid key that is not under the reserved prefix", c.Name, c.Key)
		}
		// 上限两侧的键只用 ASCII：字节数、码元数与字符数相同，不依赖长度的单位
		units := len(utf16.Encode([]rune(c.Key)))
		overLimit = overLimit || (len(c.Key) == vectors.MaxLength+1 && units == len(c.Key))
	}
	for _, c := range vectors.Reserved {
		if rules.IsBlobKey(c.Key) || !rules.IsReservedBlobKey(c.Key) {
			t.Errorf("%s: %q must be rejected as a key under the reserved prefix", c.Name, c.Key)
		}
	}
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep an ASCII key of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}
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
