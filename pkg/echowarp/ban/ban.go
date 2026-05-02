package ban

// BanManager defines the interface for managing bans by IP, HWID, and nickname.
// Implementations track failed authentication attempts and ban addresses
// that exceed a threshold.
type BanManager interface {
	// IsBanned checks if an address is currently banned.
	IsBanned(addr string) bool

	// RecordFailure records a failed authentication attempt.
	// Returns true if this failure caused the address to become banned.
	RecordFailure(addr string) bool

	// RecordSuccess resets the failure counter for an address.
	RecordSuccess(addr string)

	// Ban immediately bans an address regardless of failure count.
	// Persistence errors are swallowed; use BanWithReason when the caller
	// needs to learn about disk-write failures.
	Ban(addr string)

	// BanWithReason bans an address with an optional human-readable reason
	// and returns any persistence error.
	BanWithReason(addr, reason string) error

	// Unban removes the ban for an address.
	Unban(addr string)

	// BannedList returns all currently banned addresses (subjects only).
	BannedList() []string

	// BannedEntries returns full ban entries (with BannedAt and Reason) for
	// all currently banned addresses.
	BannedEntries() []BanEntry

	// IsHWIDBanned checks if a hardware identifier is banned.
	IsHWIDBanned(hwid string) bool

	// BanHWID bans a hardware identifier. Swallows persistence errors;
	// use BanHWIDWithReason to get them back.
	BanHWID(hwid string)

	// BanHWIDWithReason bans an HWID with an optional reason and returns
	// any persistence error.
	BanHWIDWithReason(hwid, reason string) error

	// UnbanHWID removes the ban for a hardware identifier.
	UnbanHWID(hwid string)

	// BannedHWIDList returns all currently banned hardware identifiers.
	BannedHWIDList() []string

	// BannedHWIDEntries returns full ban entries for all banned HWIDs.
	BannedHWIDEntries() []BanEntry

	// IsNicknameBanned checks if a nickname is banned.
	IsNicknameBanned(nickname string) bool

	// BanNickname bans a nickname. Swallows persistence errors; use
	// BanNicknameWithReason to get them back.
	BanNickname(nickname string)

	// BanNicknameWithReason bans a nickname with an optional reason and
	// returns any persistence error.
	BanNicknameWithReason(nickname, reason string) error

	// UnbanNickname removes the ban for a nickname.
	UnbanNickname(nickname string)

	// BannedNicknameList returns all currently banned nicknames.
	BannedNicknameList() []string

	// BannedNicknameEntries returns full ban entries for all banned nicknames.
	BannedNicknameEntries() []BanEntry

	// Close persists ban state and releases resources.
	Close() error
}
