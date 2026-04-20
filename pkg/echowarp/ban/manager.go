package ban

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// BanEntry represents the ban state for a single address.
type BanEntry struct {
	Address        string    `json:"address" yaml:"address"`
	FailedAttempts int       `json:"failed_attempts" yaml:"failed_attempts"`
	Banned         bool      `json:"banned" yaml:"banned"`
	LastAttempt    time.Time `json:"last_attempt" yaml:"last_attempt"`
	BannedAt       time.Time `json:"banned_at,omitempty" yaml:"banned_at,omitempty"`
	// Reason is an optional human-readable note attached to an API-initiated
	// ban. Older entries saved before this field was added have it empty.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// banFileData is the structure for persistent storage (YAML, with JSON fallback).
type banFileData struct {
	Entries      map[string]*BanEntry `json:"entries" yaml:"entries"`
	HWIDBans     map[string]*BanEntry `json:"hwid_bans,omitempty" yaml:"hwid_bans,omitempty"`
	NicknameBans map[string]*BanEntry `json:"nickname_bans,omitempty" yaml:"nickname_bans,omitempty"`
}

// FileBanManager implements BanManager with file-based persistence.
// Thread-safe: All operations protected by sync.RWMutex.
type FileBanManager struct {
	maxFailedAttempts int
	filePath          string
	mu                sync.RWMutex
	entries           map[string]*BanEntry
	hwidBans          map[string]*BanEntry
	nickBans          map[string]*BanEntry
}

// NewFileBanManager creates a new file-based ban manager.
// maxFailed: number of failures before ban (0 disables banning).
// filePath: path to the YAML file for persistence (legacy JSON files are auto-detected).
func NewFileBanManager(maxFailed int, filePath string) (*FileBanManager, error) {
	bm := &FileBanManager{
		maxFailedAttempts: maxFailed,
		filePath:          filePath,
		entries:           make(map[string]*BanEntry),
		hwidBans:          make(map[string]*BanEntry),
		nickBans:          make(map[string]*BanEntry),
	}

	if err := bm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("ban manager: load: %w", err)
	}

	return bm, nil
}

// IsBanned returns true if the address is currently banned.
func (bm *FileBanManager) IsBanned(addr string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	entry, ok := bm.entries[addr]
	return ok && entry.Banned
}

// RecordFailure increments the failure counter for an address.
// Returns true if the address became banned as a result.
func (bm *FileBanManager) RecordFailure(addr string) bool {
	if bm.maxFailedAttempts <= 0 {
		return false
	}

	bm.mu.Lock()

	entry, ok := bm.entries[addr]
	if !ok {
		entry = &BanEntry{Address: addr}
		bm.entries[addr] = entry
	}

	if entry.Banned {
		bm.mu.Unlock()
		return false
	}

	entry.FailedAttempts++
	entry.LastAttempt = time.Now()

	banned := entry.FailedAttempts >= bm.maxFailedAttempts
	if banned {
		entry.Banned = true
		entry.BannedAt = time.Now()
	}

	bm.mu.Unlock()

	if banned {
		_ = bm.save() //nolint:errcheck
	}
	return banned
}

// RecordSuccess resets the failure counter for a non-banned address.
func (bm *FileBanManager) RecordSuccess(addr string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	entry, ok := bm.entries[addr]
	if ok && !entry.Banned {
		entry.FailedAttempts = 0
	}
}

// Ban immediately bans an address and persists to disk.
// Persistence errors are swallowed — use BanWithReason when the caller needs
// to learn about disk-write failures (e.g. to return a 500 from an API).
func (bm *FileBanManager) Ban(addr string) {
	_ = bm.BanWithReason(addr, "") //nolint:errcheck
}

// BanWithReason immediately bans an address, records an optional human-readable
// reason, and returns any persistence error to the caller.
func (bm *FileBanManager) BanWithReason(addr, reason string) error {
	bm.mu.Lock()
	entry, ok := bm.entries[addr]
	if !ok {
		entry = &BanEntry{Address: addr}
		bm.entries[addr] = entry
	}
	entry.Banned = true
	entry.BannedAt = time.Now()
	if reason != "" {
		entry.Reason = reason
	}
	bm.mu.Unlock()
	return bm.save()
}

// Unban removes the ban status from an address and persists to disk.
func (bm *FileBanManager) Unban(addr string) {
	bm.mu.Lock()

	if entry, ok := bm.entries[addr]; ok {
		entry.Banned = false
		entry.FailedAttempts = 0
		entry.BannedAt = time.Time{}
	}

	bm.mu.Unlock()
	_ = bm.save() //nolint:errcheck
}

// BannedList returns all currently banned addresses.
func (bm *FileBanManager) BannedList() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var list []string
	for addr, entry := range bm.entries {
		if entry.Banned {
			list = append(list, addr)
		}
	}
	return list
}

// BannedEntries returns full ban entries (with BannedAt timestamp and Reason)
// for all currently banned IP addresses. Prefer this over BannedList when the
// caller needs metadata.
func (bm *FileBanManager) BannedEntries() []BanEntry {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	var list []BanEntry
	for _, entry := range bm.entries {
		if entry.Banned {
			list = append(list, *entry)
		}
	}
	return list
}

// IsHWIDBanned returns true if the HWID is currently banned.
func (bm *FileBanManager) IsHWIDBanned(hwid string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	entry, ok := bm.hwidBans[hwid]
	return ok && entry.Banned
}

// BanHWID immediately bans a hardware identifier and persists to disk.
// Persistence errors are swallowed — use BanHWIDWithReason when the caller
// needs to learn about disk-write failures.
func (bm *FileBanManager) BanHWID(hwid string) {
	_ = bm.BanHWIDWithReason(hwid, "") //nolint:errcheck
}

// BanHWIDWithReason bans a hardware identifier with an optional reason and
// returns any persistence error to the caller.
func (bm *FileBanManager) BanHWIDWithReason(hwid, reason string) error {
	bm.mu.Lock()
	entry, ok := bm.hwidBans[hwid]
	if !ok {
		entry = &BanEntry{Address: hwid}
		bm.hwidBans[hwid] = entry
	}
	entry.Banned = true
	entry.BannedAt = time.Now()
	if reason != "" {
		entry.Reason = reason
	}
	bm.mu.Unlock()
	return bm.save()
}

// UnbanHWID removes the ban for a hardware identifier and persists to disk.
func (bm *FileBanManager) UnbanHWID(hwid string) {
	bm.mu.Lock()

	if entry, ok := bm.hwidBans[hwid]; ok {
		entry.Banned = false
		entry.BannedAt = time.Time{}
	}

	bm.mu.Unlock()
	_ = bm.save() //nolint:errcheck
}

// BannedHWIDList returns all currently banned hardware identifiers.
func (bm *FileBanManager) BannedHWIDList() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var list []string
	for hwid, entry := range bm.hwidBans {
		if entry.Banned {
			list = append(list, hwid)
		}
	}
	return list
}

// BannedHWIDEntries returns full ban entries for all banned HWIDs.
func (bm *FileBanManager) BannedHWIDEntries() []BanEntry {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	var list []BanEntry
	for _, entry := range bm.hwidBans {
		if entry.Banned {
			list = append(list, *entry)
		}
	}
	return list
}

// IsNicknameBanned returns true if the nickname is currently banned.
func (bm *FileBanManager) IsNicknameBanned(nickname string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	entry, ok := bm.nickBans[nickname]
	return ok && entry.Banned
}

// BanNickname immediately bans a nickname and persists to disk.
// Persistence errors are swallowed — use BanNicknameWithReason when the caller
// needs to learn about disk-write failures.
func (bm *FileBanManager) BanNickname(nickname string) {
	_ = bm.BanNicknameWithReason(nickname, "") //nolint:errcheck
}

// BanNicknameWithReason bans a nickname with an optional reason and returns
// any persistence error to the caller.
func (bm *FileBanManager) BanNicknameWithReason(nickname, reason string) error {
	bm.mu.Lock()
	entry, ok := bm.nickBans[nickname]
	if !ok {
		entry = &BanEntry{Address: nickname}
		bm.nickBans[nickname] = entry
	}
	entry.Banned = true
	entry.BannedAt = time.Now()
	if reason != "" {
		entry.Reason = reason
	}
	bm.mu.Unlock()
	return bm.save()
}

// UnbanNickname removes the ban for a nickname and persists to disk.
func (bm *FileBanManager) UnbanNickname(nickname string) {
	bm.mu.Lock()

	if entry, ok := bm.nickBans[nickname]; ok {
		entry.Banned = false
		entry.BannedAt = time.Time{}
	}

	bm.mu.Unlock()
	_ = bm.save() //nolint:errcheck
}

// BannedNicknameList returns all currently banned nicknames.
func (bm *FileBanManager) BannedNicknameList() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var list []string
	for nick, entry := range bm.nickBans {
		if entry.Banned {
			list = append(list, nick)
		}
	}
	return list
}

// BannedNicknameEntries returns full ban entries for all banned nicknames.
func (bm *FileBanManager) BannedNicknameEntries() []BanEntry {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	var list []BanEntry
	for _, entry := range bm.nickBans {
		if entry.Banned {
			list = append(list, *entry)
		}
	}
	return list
}

// Close saves the ban state to file and releases resources.
func (bm *FileBanManager) Close() error {
	return bm.save()
}

// jsonFallbackPath returns the legacy .json path corresponding to bm.filePath.
// If filePath already ends in .json, it returns it as-is.
// If filePath ends in .yml/.yaml, it replaces the extension with .json.
func (bm *FileBanManager) jsonFallbackPath() string {
	ext := filepath.Ext(bm.filePath)
	if ext == ".json" {
		return bm.filePath
	}
	return strings.TrimSuffix(bm.filePath, ext) + ".json"
}

func (bm *FileBanManager) load() error {
	// Try primary path first
	data, err := os.ReadFile(bm.filePath)
	if err != nil {
		// Fall back to legacy JSON path if primary is YAML
		fallback := bm.jsonFallbackPath()
		if fallback != bm.filePath {
			data, err = os.ReadFile(fallback)
			if err != nil {
				return err
			}
			// Legacy JSON fallback
			return bm.unmarshalFileData(data, true)
		}
		return err
	}

	// Detect format: if file looks like JSON, parse as JSON; otherwise YAML.
	isJSON := false
	trimmed := strings.TrimSpace(string(data))
	if trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') {
		isJSON = true
	}
	return bm.unmarshalFileData(data, isJSON)
}

func (bm *FileBanManager) unmarshalFileData(data []byte, isJSON bool) error {
	var fileData banFileData
	var err error
	if isJSON {
		err = json.Unmarshal(data, &fileData)
	} else {
		err = yaml.Unmarshal(data, &fileData)
	}
	if err != nil {
		return fmt.Errorf("ban manager: unmarshal: %w", err)
	}

	if fileData.Entries != nil {
		bm.entries = fileData.Entries
	}
	if fileData.HWIDBans != nil {
		bm.hwidBans = fileData.HWIDBans
	}
	if fileData.NicknameBans != nil {
		bm.nickBans = fileData.NicknameBans
	}

	return nil
}

func (bm *FileBanManager) save() error {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	fileData := banFileData{
		Entries:      bm.entries,
		HWIDBans:     bm.hwidBans,
		NicknameBans: bm.nickBans,
	}
	data, err := yaml.Marshal(fileData)
	if err != nil {
		return fmt.Errorf("ban manager: marshal: %w", err)
	}

	if err := os.WriteFile(bm.filePath, data, 0600); err != nil {
		return fmt.Errorf("ban manager: write: %w", err)
	}

	return nil
}
