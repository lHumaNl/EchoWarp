package app

import (
	"strings"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Ban ID prefixes. The public echowarp.BanEntry.ID uses the form
// "<prefix>:<subject>" so every ID is self-describing and can be routed
// back to the correct internal map (addresses, HWIDs, or nicknames)
// without a parallel lookup table. The prefixes are deliberately short
// to keep IDs URL-safe for DELETE /api/v1/bans/{id}.
const (
	banIDPrefixIP       = "ip:"
	banIDPrefixHWID     = "hwid:"
	banIDPrefixNickname = "nick:"
)

// BanList implements echowarp.BanManager. It flattens the three
// separate ban maps kept by ban.BanManager into a single slice of
// public BanEntry values. Each entry is annotated with its kind via
// the ID prefix so RemoveBan can route the delete without re-parsing
// the subject string.
//
// Returns an empty non-nil slice when s.banMgr is nil (happens in
// tests that build a ServerApp without a ban manager). This matches
// the Node-level contract: the caller must not see nil.
func (s *ServerApp) BanList() []echowarp.BanEntry {
	if s.banMgr == nil {
		return []echowarp.BanEntry{}
	}
	ips := s.banMgr.BannedEntries()
	hwids := s.banMgr.BannedHWIDEntries()
	nicks := s.banMgr.BannedNicknameEntries()

	out := make([]echowarp.BanEntry, 0, len(ips)+len(hwids)+len(nicks))
	for _, e := range ips {
		out = append(out, echowarp.BanEntry{
			ID:        banIDPrefixIP + e.Address,
			IP:        e.Address,
			Reason:    e.Reason,
			CreatedAt: e.BannedAt,
		})
	}
	for _, e := range hwids {
		out = append(out, echowarp.BanEntry{
			ID:        banIDPrefixHWID + e.Address,
			HWID:      e.Address,
			Reason:    e.Reason,
			CreatedAt: e.BannedAt,
		})
	}
	for _, e := range nicks {
		out = append(out, echowarp.BanEntry{
			ID:        banIDPrefixNickname + e.Address,
			Nickname:  e.Address,
			Reason:    e.Reason,
			CreatedAt: e.BannedAt,
		})
	}
	return out
}

// AddBan implements echowarp.BanManager. The Node wrapper has already
// validated that exactly one of IP/HWID/Nickname is set, but we guard
// again here as a defense-in-depth check and to keep the adapter
// self-contained for direct-call tests. A nil s.banMgr is reported as
// a structured ErrInternalState so the API layer returns 500 rather
// than silently accepting writes that nothing will persist.
func (s *ServerApp) AddBan(entry echowarp.BanEntry) error {
	if s.banMgr == nil {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Server has no ban manager configured").
			WithSuggestion("Configure a ban manager via WithBanManager on the Node before adding bans")
	}
	var persistErr error
	switch {
	case entry.IP != "":
		persistErr = s.banMgr.BanWithReason(entry.IP, entry.Reason)
	case entry.HWID != "":
		persistErr = s.banMgr.BanHWIDWithReason(entry.HWID, entry.Reason)
	case entry.Nickname != "":
		persistErr = s.banMgr.BanNicknameWithReason(entry.Nickname, entry.Reason)
	default:
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban entry requires a subject").
			WithSuggestion("Set exactly one of ip, hwid, or nickname on the ban entry")
	}
	if persistErr != nil {
		return ewerrors.Wrap(persistErr, ewerrors.ErrInternalState, "Failed to persist ban")
	}
	// entry.CreatedAt is not threaded through — the ban manager uses
	// wall-clock time at the moment of persistence (BannedAt field). Same
	// value is returned from subsequent GET /bans via BannedEntries().
	return nil
}

// RemoveBan implements echowarp.BanManager. Parses the kind prefix
// off the id and dispatches to the matching Unban* method. An
// unrecognized prefix is reported as ErrConfigValidation so the API
// layer returns 400 rather than 500 — the client sent a malformed ID.
func (s *ServerApp) RemoveBan(id string) error {
	if s.banMgr == nil {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Server has no ban manager configured").
			WithSuggestion("Configure a ban manager via WithBanManager on the Node before removing bans")
	}
	switch {
	case strings.HasPrefix(id, banIDPrefixIP):
		subject := strings.TrimPrefix(id, banIDPrefixIP)
		if subject == "" {
			return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban id has empty subject").
				WithSuggestion("Use the id returned by GET /api/v1/bans")
		}
		s.banMgr.Unban(subject)
	case strings.HasPrefix(id, banIDPrefixHWID):
		subject := strings.TrimPrefix(id, banIDPrefixHWID)
		if subject == "" {
			return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban id has empty subject").
				WithSuggestion("Use the id returned by GET /api/v1/bans")
		}
		s.banMgr.UnbanHWID(subject)
	case strings.HasPrefix(id, banIDPrefixNickname):
		subject := strings.TrimPrefix(id, banIDPrefixNickname)
		if subject == "" {
			return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban id has empty subject").
				WithSuggestion("Use the id returned by GET /api/v1/bans")
		}
		s.banMgr.UnbanNickname(subject)
	default:
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban id has unknown kind").
			WithContext("id", id).
			WithSuggestion("Ban id must start with ip:, hwid:, or nick:")
	}
	return nil
}
