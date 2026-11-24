package rules_test

import (
	"slices"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestLocaleNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.LocaleNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxLocaleNameLength || vectors.Default != rules.DefaultLocale || !slices.Equal(vectors.BuiltIn, rules.BuiltInLocales) {
		t.Fatalf("maxLength, default and built-in locales = %d, %q, %v; want %d, %q, %v", vectors.MaxLength, vectors.Default, vectors.BuiltIn,
			rules.MaxLocaleNameLength, rules.DefaultLocale, rules.BuiltInLocales)
	}
	for _, c := range vectors.Valid {
		if !rules.IsLocaleName(c.Text) {
			t.Errorf("%q must be a valid locale name", c.Text)
			continue
		}
		if got := rules.NormalizeLocaleName(c.Text); got != c.Canonical {
			t.Errorf("%q: canonical form = %q, want %q", c.Text, got, c.Canonical)
		}
		if got := rules.LocaleLanguage(c.Text); got != c.Language {
			t.Errorf("%q: language = %q, want %q", c.Text, got, c.Language)
		}
		// 规范形式本身合文法，再规范一次不变
		if !rules.IsLocaleName(c.Canonical) || rules.NormalizeLocaleName(c.Canonical) != c.Canonical {
			t.Errorf("%q: the canonical form %q must be valid and stable", c.Text, c.Canonical)
		}
	}
	if len(vectors.Longest) != vectors.MaxLength || !rules.IsLocaleName(vectors.Longest) {
		t.Errorf("longest %q must be a valid locale name of exactly %d characters", vectors.Longest, vectors.MaxLength)
	}
	overLimit := false
	for _, text := range vectors.Invalid {
		if rules.IsLocaleName(text) {
			t.Errorf("%q must be an invalid locale name", text)
		}
		overLimit = overLimit || len(text) == vectors.MaxLength+1
	}
	if !overLimit {
		t.Errorf("the vector must keep an invalid locale name of %d characters", vectors.MaxLength+1)
	}
	// 内置区域都已经是规范形式
	for _, locale := range vectors.BuiltIn {
		if !rules.IsLocaleName(locale) || rules.NormalizeLocaleName(locale) != locale {
			t.Errorf("built-in locale %q must be valid and canonical", locale)
		}
	}
}

func TestLocaleNegotiationFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.LocaleNegotiation()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vectors.Negotiation {
		got := rules.NegotiateLocale(c.Enabled, c.Default, c.Candidates)
		if got != c.Locale {
			t.Errorf("%s: negotiated %q, want %q", c.Name, got, c.Locale)
		}
		// 结果总是启用列表里的写法
		if !slices.Contains(c.Enabled, c.Locale) || !slices.Contains(c.Enabled, c.Default) {
			t.Errorf("%s: the negotiated locale and the default must be in the enabled list", c.Name)
		}
	}
	for _, c := range vectors.Fallback {
		if got := rules.LocaleFallbackChain(c.Enabled, c.Default, c.Locale); !slices.Equal(got, c.Chain) {
			t.Errorf("%s: chain = %v, want %v", c.Name, got, c.Chain)
		}
		if len(c.Chain) == 0 || c.Chain[len(c.Chain)-1] != rules.DefaultLocale {
			t.Errorf("%s: the chain must end with %s", c.Name, rules.DefaultLocale)
		}
	}
}

func TestCatalogResolutionFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.CatalogResolution()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) {
		t.Errorf("%q is not an error code", vectors.Code)
	}
	for _, c := range vectors.Cases {
		var catalogs []rules.MessageCatalog
		for _, catalog := range c.Catalogs {
			catalogs = append(catalogs, rules.MessageCatalog{Name: catalog.Name, Messages: catalog.Messages})
		}
		merged, ok := rules.MergeMessageCatalogs(catalogs)
		if ok == c.Conflict {
			t.Errorf("%s: merged without conflict = %v, want conflict = %v", c.Name, ok, c.Conflict)
			continue
		}
		for _, request := range c.Requests {
			chain := rules.LocaleFallbackChain(rules.BuiltInLocales, c.Default, request.Locale)
			template, locale, found := rules.FindMessage(merged, chain, request.Key)
			switch {
			case request.Expected == nil && found:
				t.Errorf("%s: %s in %s resolved to %q (%s), want nothing", c.Name, request.Key, request.Locale, template, locale)
			case request.Expected != nil && (!found || template != request.Expected.Text || locale != request.Expected.Locale):
				t.Errorf("%s: %s in %s resolved to %q (%s, found %v), want %q (%s)", c.Name, request.Key, request.Locale, template, locale, found, request.Expected.Text, request.Expected.Locale)
			}
			// 向量里的键都得合消息键的文法
			if !rules.IsMessageKey(request.Key) {
				t.Errorf("%s: %q is not a message key", c.Name, request.Key)
			}
		}
	}
}
