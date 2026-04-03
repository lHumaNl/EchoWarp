// Package ban provides IP-based banning functionality for EchoWarp.
// It implements a file-based ban list that persists across restarts.
//
// The BanManager interface defines operations for checking, adding, and removing bans.
// FileBanManager is the concrete implementation that stores bans in a JSON file.
//
// Ban entries include:
//   - IP address
//   - Timestamp of ban
//   - Reason for ban
//   - Expiration time (optional)
//
// Thread-safe: All operations are protected by sync.RWMutex.
package ban
