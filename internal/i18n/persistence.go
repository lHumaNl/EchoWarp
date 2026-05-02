package i18n

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings represents the persisted user settings.
type Settings struct {
	Language string `yaml:"language,omitempty"`
}

// settingsPathFn can be overridden in tests.
var settingsPathFn = defaultSettingsPath

// settingsPath returns the path to the settings file.
func settingsPath() string {
	return settingsPathFn()
}

func defaultSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "echowarp", "settings.yaml")
}

// LoadLanguage reads the saved language preference from disk.
// Returns English if no file exists or the language field is empty.
func LoadLanguage() Language {
	path := settingsPath()
	if path == "" {
		return English
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return English
	}
	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil || s.Language == "" {
		return English
	}
	return Language(s.Language)
}

// SaveLanguage persists the language preference to disk.
// If the language is English (default), the file is removed to keep defaults clean.
func SaveLanguage(lang Language) error {
	path := settingsPath()
	if path == "" {
		return nil
	}
	if lang == English {
		// Remove file if it exists; English is the default.
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	s := Settings{Language: string(lang)}
	data, err := yaml.Marshal(&s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
