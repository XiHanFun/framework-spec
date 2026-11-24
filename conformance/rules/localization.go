package rules

import (
	"regexp"
	"slices"
	"strings"
)

var localeNamePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// MaxLocaleNameLength 是区域名的最大长度（RFC-0010「区域」）。
const MaxLocaleNameLength = 35

// DefaultLocale 是缺省的默认区域，也是语言文件的基准与回退链的最后一道（RFC-0010「区域」）。
const DefaultLocale = "en-US"

// BuiltInLocales 是内置的 10 种区域，也是缺省的启用列表（RFC-0010「区域」）。
var BuiltInLocales = []string{"en-US", "zh-CN", "zh-TW", "ja-JP", "ko-KR", "de-DE", "es-ES", "fr-FR", "pt-BR", "ru-RU"}

// IsLocaleName 判断区域名是否合文法（RFC-0010「区域」）：语言子标签两到三个字母，其后每个子标签 2 到 8 个字母或数字，最长 35 个字符。
func IsLocaleName(locale string) bool {
	return len(locale) <= MaxLocaleNameLength && localeNamePattern.MatchString(locale)
}

// NormalizeLocaleName 把合文法的区域名写成规范形式（RFC-0010「区域」）：语言小写，地区（两个字母）大写，
// 文字（四个字母）首字母大写其余小写；其余子标签规范不作规定，这里照参考实现写成小写。
func NormalizeLocaleName(locale string) string {
	parts := strings.Split(locale, "-")
	for index, part := range parts {
		switch {
		case index == 0:
			parts[index] = strings.ToLower(part)
		case len(part) == 4:
			parts[index] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		case len(part) == 2:
			parts[index] = strings.ToUpper(part)
		default:
			parts[index] = strings.ToLower(part)
		}
	}
	return strings.Join(parts, "-")
}

// LocaleLanguage 给出区域名的语言子标签（小写）。
func LocaleLanguage(locale string) string {
	language, _, _ := strings.Cut(locale, "-")
	return strings.ToLower(language)
}

// NegotiateLocale 把候选区域协商成启用列表里的区域（RFC-0010「协商与宿主」）：候选里第一个精确命中启用列表的
// （不区分大小写）；都没有时按候选的次序取启用列表里第一个同语言的区域；再没有取默认区域。不合文法的候选忽略。
func NegotiateLocale(enabled []string, defaultLocale string, candidates []string) string {
	valid := slices.DeleteFunc(slices.Clone(candidates), func(candidate string) bool { return !IsLocaleName(candidate) })
	// 两轮：精确命中先于按语言命中，后面的候选精确命中时不取前面候选的同语言区域
	for _, candidate := range valid {
		for _, locale := range enabled {
			if strings.EqualFold(locale, candidate) {
				return locale
			}
		}
	}
	for _, candidate := range valid {
		for _, locale := range enabled {
			if LocaleLanguage(locale) == LocaleLanguage(candidate) {
				return locale
			}
		}
	}
	return defaultLocale
}

// LocaleFallbackChain 给出取文案时依次查找的区域（RFC-0010「解析」）：请求区域（规范形式）、
// 启用列表里同语言的区域、默认区域、en-US，重合的只出现一次。locale 必须合文法。
func LocaleFallbackChain(enabled []string, defaultLocale, locale string) []string {
	var chain []string
	add := func(candidate string) {
		if !slices.ContainsFunc(chain, func(seen string) bool { return strings.EqualFold(seen, candidate) }) {
			chain = append(chain, candidate)
		}
	}
	requested := NormalizeLocaleName(locale)
	add(requested)
	for _, candidate := range enabled {
		if LocaleLanguage(candidate) == LocaleLanguage(requested) {
			add(candidate)
		}
	}
	add(defaultLocale)
	add(DefaultLocale)
	return chain
}

// MessageCatalog 是一份消息目录：名字，以及区域到「键 → 模板」的表。
type MessageCatalog struct {
	Name     string
	Messages map[string]map[string]string
}

// MergeMessageCatalogs 把按次序注册的目录合并（RFC-0010「语言文件」）：同名的目录并成一份、留在先注册的位置，
// 区域与键取并集；同一键同一区域的模板不同即冲突，返回假（LOCALIZATION_CONFLICT_001）。不同名的目录不合并。
func MergeMessageCatalogs(catalogs []MessageCatalog) ([]MessageCatalog, bool) {
	var merged []MessageCatalog
	for _, catalog := range catalogs {
		at := slices.IndexFunc(merged, func(existing MessageCatalog) bool { return existing.Name == catalog.Name })
		if at < 0 {
			merged = append(merged, MessageCatalog{Name: catalog.Name, Messages: map[string]map[string]string{}})
			at = len(merged) - 1
		}
		for locale, templates := range catalog.Messages {
			// 区域名不区分大小写：按规范形式归到同一个区域
			locale = NormalizeLocaleName(locale)
			target := merged[at].Messages[locale]
			if target == nil {
				target = map[string]string{}
				merged[at].Messages[locale] = target
			}
			for key, template := range templates {
				if existing, ok := target[key]; ok && existing != template {
					return nil, false
				}
				target[key] = template
			}
		}
	}
	return merged, true
}

// FindMessage 按回退链在目录里找一个键的模板，返回模板与实际命中的区域；哪一级都没有返回假。同一键先注册的目录优先。
func FindMessage(catalogs []MessageCatalog, chain []string, key string) (template, locale string, found bool) {
	for _, candidate := range chain {
		for _, catalog := range catalogs {
			if template, ok := catalog.Messages[NormalizeLocaleName(candidate)][key]; ok {
				return template, candidate, true
			}
		}
	}
	return "", "", false
}
