package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// PredefinedReasons is the list of built-in kick/ban reason choices.
var PredefinedReasons = []string{
	"Spam / flooding",
	"Inappropriate behavior",
	"AFK / idle too long",
	"Audio issues",
	"Server maintenance",
}

// RecentReason represents a recently used kick/ban reason.
type RecentReason struct {
	Text     string    `json:"text"`
	LastUsed time.Time `json:"last_used"`
	UseCount int       `json:"use_count"`
}

// RecentReasonsFile is the on-disk format for recent reasons.
type RecentReasonsFile struct {
	Reasons []RecentReason `json:"reasons"`
}

const maxRecentReasons = 10

// reasonsFilePathFunc is the function that returns the path to the reasons file.
// Overridden in tests.
var reasonsFilePathFunc = defaultReasonsFilePath

func defaultReasonsFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "echowarp", "kick_ban_reasons.json")
}

// LoadRecentReasons loads recent reasons from disk. Returns nil on any error.
func LoadRecentReasons() ([]RecentReason, error) {
	path := reasonsFilePathFunc()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil //nolint:nilerr // fault-tolerant
	}
	var file RecentReasonsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, nil //nolint:nilerr // fault-tolerant
	}
	return file.Reasons, nil
}

// SaveRecentReason adds or updates a reason in the recent reasons file (LRU, max 10).
func SaveRecentReason(text string) error {
	if text == "" {
		return nil
	}
	path := reasonsFilePathFunc()
	if path == "" {
		return nil
	}

	reasons, _ := LoadRecentReasons()

	// Check if already exists
	found := false
	for i, r := range reasons {
		if r.Text == text {
			reasons[i].UseCount++
			reasons[i].LastUsed = time.Now()
			found = true
			break
		}
	}
	if !found {
		reasons = append(reasons, RecentReason{
			Text:     text,
			LastUsed: time.Now(),
			UseCount: 1,
		})
	}

	// Sort by LastUsed descending (most recent first)
	sort.Slice(reasons, func(i, j int) bool {
		return reasons[i].LastUsed.After(reasons[j].LastUsed)
	})

	// LRU eviction
	if len(reasons) > maxRecentReasons {
		reasons = reasons[:maxRecentReasons]
	}

	file := RecentReasonsFile{Reasons: reasons}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
