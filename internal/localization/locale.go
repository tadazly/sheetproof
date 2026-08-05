package localization

import (
	"fmt"
	"os"
	"strings"
)

type Locale string

const (
	English           Locale = "en"
	SimplifiedChinese Locale = "zh-CN"
	Japanese          Locale = "ja"
	DefaultLocale            = English
)

var SupportedLocales = []Locale{English, SimplifiedChinese, Japanese}

func Normalize(value string) Locale {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	if normalized == "zh-tw" || strings.HasPrefix(normalized, "zh-tw-") ||
		normalized == "zh-hk" || strings.HasPrefix(normalized, "zh-hk-") ||
		normalized == "zh-mo" || strings.HasPrefix(normalized, "zh-mo-") ||
		normalized == "zh-hant" || strings.HasPrefix(normalized, "zh-hant-") {
		return DefaultLocale
	}
	switch {
	case normalized == "en" || strings.HasPrefix(normalized, "en-"):
		return English
	case normalized == "ja" || strings.HasPrefix(normalized, "ja-"):
		return Japanese
	case normalized == "zh", normalized == "zh-cn", strings.HasPrefix(normalized, "zh-cn-"),
		normalized == "zh-hans", strings.HasPrefix(normalized, "zh-hans-"),
		normalized == "zh-sg", strings.HasPrefix(normalized, "zh-sg-"):
		return SimplifiedChinese
	default:
		return DefaultLocale
	}
}

func Parse(value string) (Locale, error) {
	trimmed := strings.TrimSpace(value)
	locale := Normalize(trimmed)
	if trimmed == "" || (locale == DefaultLocale && !strings.EqualFold(trimmed, "en") && !strings.HasPrefix(strings.ToLower(trimmed), "en-")) {
		return "", fmt.Errorf("unsupported language %q; expected en, zh-CN, or ja", value)
	}
	return locale, nil
}

func FromEnvironment() Locale {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return Normalize(strings.Split(value, ".")[0])
		}
	}
	return DefaultLocale
}

func Name(locale Locale) string {
	switch locale {
	case SimplifiedChinese:
		return "简体中文"
	case Japanese:
		return "日本語"
	default:
		return "English"
	}
}
