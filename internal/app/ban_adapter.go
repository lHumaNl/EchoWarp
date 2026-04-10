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
	ips := s.banMgr.BannedList()
	hwids := s.banMgr.BannedHWIDList()
	nicks := s.banMgr.BannedNicknameList()

	out := make([]echowarp.BanEntry, 0, len(ips)+len(hwids)+len(nicks))
	// TODO(task-019): the file-backed ban.BanManager interface does not
	// expose the BannedAt timestamp for listing, only via the internal
	// banFileData. Until that API is widened, CreatedAt on listed
	// entries is the zero value. The Reason field is likewise lost on
	// round-trip (the internal BanEntry has no Reason column).
	for _, ip := range ips {
		out = append(out, echowarp.BanEntry{
			ID: banIDPrefixIP + ip,
			IP: ip,
		})
	}
	for _, hwid := range hwids {
		out = append(out, echowarp.BanEntry{
			ID:   banIDPrefixHWID + hwid,
			HWID: hwid,
		})
	}
	for _, nick := range nicks {
		out = append(out, echowarp.BanEntry{
			ID:       banIDPrefixNickname + nick,
			Nickname: nick,
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
	switch {
	case entry.IP != "":
		s.banMgr.Ban(entry.IP)
	case entry.HWID != "":
		s.banMgr.BanHWID(entry.HWID)
	case entry.Nickname != "":
		s.banMgr.BanNickname(entry.Nickname)
	default:
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Ban entry requires a subject").
			WithSuggestion("Set exactly one of ip, hwid, or nickname on the ban entry")
	}
	// TODO(task-019): s.banMgr.Ban* methods swallow their persistence
	// error (see pkg/echowarp/ban/manager.go — _ = bm.save()). Once
	// that is widened to return an error, propagate it here so the
	// API caller learns about disk write failures. Similarly, the
	// internal BanEntry has no Reason/CreatedAt columns, so those
	// request fields are dropped on persist.
	_ = entry.CreatedAt
	_ = entry.Reason
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
