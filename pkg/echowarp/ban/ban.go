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
	Ban(addr string)

	// Unban removes the ban for an address.
	Unban(addr string)

	// BannedList returns all currently banned addresses.
	BannedList() []string

	// IsHWIDBanned checks if a hardware identifier is banned.
	IsHWIDBanned(hwid string) bool

	// BanHWID bans a hardware identifier.
	BanHWID(hwid string)

	// UnbanHWID removes the ban for a hardware identifier.
	UnbanHWID(hwid string)

	// BannedHWIDList returns all currently banned hardware identifiers.
	BannedHWIDList() []string

	// IsNicknameBanned checks if a nickname is banned.
	IsNicknameBanned(nickname string) bool

	// BanNickname bans a nickname.
	BanNickname(nickname string)

	// UnbanNickname removes the ban for a nickname.
	UnbanNickname(nickname string)

	// BannedNicknameList returns all currently banned nicknames.
	BannedNicknameList() []string

	// Close persists ban state and releases resources.
	Close() error
}
