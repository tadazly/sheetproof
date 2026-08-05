package localization

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]Locale{
		"en": English, "en-US": English, "en-GB": English,
		"zh": SimplifiedChinese, "zh-CN": SimplifiedChinese, "zh-Hans": SimplifiedChinese, "zh-SG": SimplifiedChinese,
		"ja": Japanese, "ja-JP": Japanese,
		"zh-TW": English, "zh-HK": English, "zh-MO": English, "zh-Hant": English,
		"fr-FR": English, "": English,
	}
	for input, expected := range tests {
		if actual := Normalize(input); actual != expected {
			t.Errorf("Normalize(%q) = %q, want %q", input, actual, expected)
		}
	}
}
