package auth

// Version is the application version used in auth handshakes.
// Set once at startup from internal/version.Version. No other place defines the version.
var Version string

// checkVersionCompat verifies exact version match between peers.
func checkVersionCompat(peerVersion string) error {
	if Version == "" || peerVersion == "" {
		return nil
	}
	if Version != peerVersion {
		return NewVersionMismatchError(peerVersion, Version)
	}
	return nil
}
