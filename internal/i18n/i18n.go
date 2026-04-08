package i18n

import (
	"fmt"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// Language represents a locale identifier such as "en" or "ru".
type Language string

const (
	English Language = "en"
	Russian Language = "ru"
)

type translations struct {
	lang    Language
	entries map[string]string
}

var current atomic.Pointer[translations]

// englishFallback holds the English translations for fallback lookups.
var englishFallback atomic.Pointer[translations]

func init() {
	// Load English as the default language.
	en := loadTranslations(English)
	englishFallback.Store(en)
	current.Store(en)
}

// T returns the translated string for the given key.
// Falls back to English if the key is missing in the current language,
// and falls back to the key itself if English also lacks the key.
func T(key string) string {
	if cur := current.Load(); cur != nil {
		if v, ok := cur.entries[key]; ok {
			return v
		}
	}
	if en := englishFallback.Load(); en != nil {
		if v, ok := en.entries[key]; ok {
			return v
		}
	}
	return key
}

// Tf returns the translated format string for the given key, with fmt.Sprintf arguments applied.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// SetLanguage loads translations for the given language and sets it as current.
func SetLanguage(lang Language) {
	t := loadTranslations(lang)
	current.Store(t)
}

// CurrentLanguage returns the currently active language.
func CurrentLanguage() Language {
	if cur := current.Load(); cur != nil {
		return cur.lang
	}
	return English
}

// AvailableLanguages returns the list of available languages.
func AvailableLanguages() []Language {
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return []Language{English}
	}
	var langs []Language
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".yaml" {
			langs = append(langs, Language(name[:len(name)-5]))
		}
	}
	if len(langs) == 0 {
		return []Language{English}
	}
	return langs
}

func loadTranslations(lang Language) *translations {
	t := &translations{
		lang:    lang,
		entries: make(map[string]string),
	}
	data, err := localesFS.ReadFile("locales/" + string(lang) + ".yaml")
	if err != nil {
		return t
	}
	_ = yaml.Unmarshal(data, &t.entries)
	return t
}
