package rules_test

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestErrorCodesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.ErrorCodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range vectors.Valid {
		if !rules.IsErrorCode(code) {
			t.Errorf("%q must be a valid error code", code)
		}
	}
	for _, code := range vectors.Invalid {
		if rules.IsErrorCode(code) {
			t.Errorf("%q must be an invalid error code", code)
		}
	}
	if kind := rules.ErrorCodeKind("PERSISTENCE_CONFLICT_001"); kind != "CONFLICT" {
		t.Errorf("kind = %q, want CONFLICT", kind)
	}
}

func TestBaggageKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.BaggageKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range vectors.WellFormed {
		if !rules.IsBaggageKey(key) {
			t.Errorf("%q must be well-formed", key)
		}
	}
	for _, key := range vectors.IllFormed {
		if rules.IsBaggageKey(key) {
			t.Errorf("%q must be ill-formed", key)
		}
	}
	if !slices.Equal(vectors.Registered, rules.RegisteredBaggageKeys) {
		t.Errorf("registered keys = %v, want %v", rules.RegisteredBaggageKeys, vectors.Registered)
	}
}

func TestCapabilityRequirementsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.CapabilityRequirements()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		parsed, ok := rules.ParseCapabilityRequirement(c.Input)
		if !ok {
			t.Errorf("%q must parse", c.Input)
			continue
		}
		if parsed.ID != c.ID || parsed.Version != c.Version || parsed.Optional != c.Optional {
			t.Errorf("%q parsed as %+v, want id=%s version=%d optional=%v", c.Input, parsed, c.ID, c.Version, c.Optional)
		}
	}
	for _, text := range vectors.Invalid {
		if _, ok := rules.ParseCapabilityRequirement(text); ok {
			t.Errorf("%q must not parse", text)
		}
	}
}

func TestLifecycleTransitionsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.LifecycleTransitions()
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[[2]string]bool{}
	for _, pair := range vectors.Allowed {
		allowed[pair] = true
		if !rules.CanTransition(pair[0], pair[1]) {
			t.Errorf("%s -> %s must be allowed", pair[0], pair[1])
		}
	}
	for _, from := range rules.LifecycleStates {
		for _, to := range rules.LifecycleStates {
			if rules.CanTransition(from, to) && !allowed[[2]string{from, to}] {
				t.Errorf("%s -> %s is allowed by the rules but absent from the vector", from, to)
			}
		}
	}
}

func TestHandshakeLinesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.HandshakeLines()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Valid {
		endpoint, ok := rules.ParseHandshakeLine(c.Line)
		if !ok {
			t.Errorf("%q must parse", c.Line)
			continue
		}
		if endpoint.Network != c.Network || endpoint.Address != c.Address {
			t.Errorf("%q parsed as %+v, want %s %s", c.Line, endpoint, c.Network, c.Address)
		}
	}
	for _, line := range vectors.Invalid {
		if _, ok := rules.ParseHandshakeLine(line); ok {
			t.Errorf("%q must not parse", line)
		}
	}
}

func TestVersionNegotiationFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.ProtocolVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Cases {
		selected, ok := rules.NegotiateVersion(c.Host, c.Plugin)
		if c.Error != "" {
			if ok {
				t.Errorf("host %v plugin %v must fail, got %d", c.Host, c.Plugin, selected)
			}
			continue
		}
		if !ok || selected != c.Selected {
			t.Errorf("host %v plugin %v selected %d (ok=%v), want %d", c.Host, c.Plugin, selected, ok, c.Selected)
		}
	}
}

func TestOperationIDsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.OperationIDs()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxOperationIDLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxOperationIDLength)
	}
	atLimit, overLimit := false, false
	for _, id := range vectors.Valid {
		if !rules.IsOperationID(id) {
			t.Errorf("%q must be an operation ID", id)
		}
		atLimit = atLimit || len(id) == vectors.MaxLength
	}
	for _, id := range vectors.Invalid {
		if rules.IsOperationID(id) {
			t.Errorf("%q must not be an operation ID", id)
		}
		overLimit = overLimit || len(id) == vectors.MaxLength+1
	}
	// 上限两侧各留一条：只查文法、不查长度的实现在 129 个字符那一条上才会失败
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep an ID of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}
}

func TestGrammars(t *testing.T) {
	for _, id := range []string{"orders.create", "order-lines.list", "echoes.relay"} {
		if !rules.IsOperationID(id) {
			t.Errorf("%q must be an operation ID", id)
		}
	}
	for _, id := range []string{"", "Orders.Create", "orders", "orders.", "orders.create.now", "-orders.create"} {
		if rules.IsOperationID(id) {
			t.Errorf("%q must not be an operation ID", id)
		}
	}
	if !rules.IsTopic("echo.events") || rules.IsTopic("") || rules.IsTopic("Echo") {
		t.Error("topic grammar")
	}
	if !rules.IsManifestID("xihan.samples.echo") || rules.IsManifestID("samples.echo") {
		t.Error("manifest ID grammar")
	}
	if len(rules.PipelineStages) != 11 || rules.PipelineStages[0] != "ingress" || rules.PipelineStages[10] != "egress" {
		t.Error("pipeline stages")
	}
}

func TestCacheKeysFollowTheVectors(t *testing.T) {
	vectors, err := conformance.CacheKeys()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxBytes != rules.MaxCacheKeyBytes {
		t.Fatalf("maxBytes = %d, want %d", vectors.MaxBytes, rules.MaxCacheKeyBytes)
	}

	// 向量里写的两个计数先核对：字节数是规范的判据，码元数只用来标出按码元计会判错的用例
	atLimit, overOnlyInBytes := false, false
	check := func(group string, cases []conformance.CacheKeyCase, accepts func(string) bool, want bool) {
		for _, c := range cases {
			if bytes := len(c.Text); bytes != c.UTF8Bytes {
				t.Errorf("%s %q: utf8Bytes = %d, but the text is %d bytes", group, c.Name, c.UTF8Bytes, bytes)
			}
			if units := len(utf16.Encode([]rune(c.Text))); units != c.UTF16Units {
				t.Errorf("%s %q: utf16Units = %d, but the text is %d code units", group, c.Name, c.UTF16Units, units)
			}
			if got := accepts(c.Text); got != want {
				t.Errorf("%s %q: accepted = %v, want %v", group, c.Name, got, want)
			}
			atLimit = atLimit || (want && c.UTF8Bytes == vectors.MaxBytes)
			overOnlyInBytes = overOnlyInBytes || (!want && c.UTF8Bytes > vectors.MaxBytes && c.UTF16Units <= vectors.MaxBytes)
		}
	}
	check("keys.valid", vectors.Keys.Valid, rules.IsCacheKey, true)
	check("keys.invalid", vectors.Keys.Invalid, rules.IsCacheKey, false)
	check("prefixes.valid", vectors.Prefixes.Valid, rules.IsCacheKeyPrefix, true)
	check("prefixes.invalid", vectors.Prefixes.Invalid, rules.IsCacheKeyPrefix, false)

	// 两类边界缺一不可：正好到上限的键，以及码元数不超、字节数超过的键（只按码元计的实现会放过它）
	if !atLimit || !overOnlyInBytes {
		t.Errorf("the vector must keep a key of exactly %d bytes (found: %v) and a key over the limit only when counted in bytes (found: %v)", vectors.MaxBytes, atLimit, overOnlyInBytes)
	}
	if !rules.IsCacheKeyPrefix("") || rules.IsCacheKey("") {
		t.Error("the empty text is a valid prefix but not a valid key")
	}
}

func TestProviderCacheKeysFollowTheKeyGrammar(t *testing.T) {
	vectors, err := conformance.CachingDistributed()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range vectors.InvalidKeys {
		if rules.IsCacheKey(key) {
			t.Errorf("invalid key %q of the operation set must not be a cache key", key)
		}
	}

	// 样例请求里的键都必须合法：单键操作在 key，批量在 keys 与 entries[].key
	checked := 0
	for _, sample := range vectors.Samples {
		var request struct {
			Key     *string  `json:"key"`
			Keys    []string `json:"keys"`
			Entries []struct {
				Key string `json:"key"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(sample.Request, &request); err != nil {
			t.Fatalf("%s: %v", sample.Operation, err)
		}
		keys := request.Keys
		if request.Key != nil {
			keys = append(keys, *request.Key)
		}
		for _, entry := range request.Entries {
			keys = append(keys, entry.Key)
		}
		for _, key := range keys {
			checked++
			if !rules.IsCacheKey(key) {
				t.Errorf("%s: sample key %q must be a cache key", sample.Operation, key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no sample key was checked")
	}
}
