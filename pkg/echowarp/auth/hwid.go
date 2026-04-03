package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// hwidFilePathFn returns the path for storing the HWID file.
// It is a variable so tests can override it.
var hwidFilePathFn = defaultHWIDFilePath

func defaultHWIDFilePath() (string, error) {
	return filepath.Join(echoWarpDir(), "hwid"), nil
}

// echoWarpDir returns the base directory for EchoWarp files.
// Uses ECHOWARP_CONFIG_DIR env var if set, otherwise ~/.config/echowarp.
func echoWarpDir() string {
	if dir := os.Getenv("ECHOWARP_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".echowarp")
	}
	return filepath.Join(home, ".config", "echowarp")
}

// GenerateHWID generates a SHA-256 based hardware identifier from machine characteristics.
// Returns a 32-character hex string (first 16 bytes of the SHA-256 hash).
func GenerateHWID() (string, error) {
	machineID, _ := getMachineID() //nolint:errcheck
	macAddr := getPrimaryMAC()
	hostname, _ := os.Hostname() //nolint:errcheck
	osName := runtime.GOOS

	input := hostname + macAddr + osName + machineID
	if input == "" {
		return "", fmt.Errorf("cannot generate HWID: no hardware identifiers available")
	}

	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:16]), nil
}

// LoadOrGenerateHWID reads an existing HWID from ~/.config/echowarp/hwid,
// or generates a new one and saves it to that path.
func LoadOrGenerateHWID() (string, error) {
	path, err := hwidFilePathFn()
	if err != nil {
		return GenerateHWID()
	}

	data, err := os.ReadFile(path)
	if err == nil {
		hwid := strings.TrimSpace(string(data))
		if hwid != "" {
			return hwid, nil
		}
	}

	hwid, err := GenerateHWID()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return hwid, nil // return HWID even if we can't persist it
	}
	_ = os.WriteFile(path, []byte(hwid+"\n"), 0o600) //nolint:errcheck

	return hwid, nil
}

// getPrimaryMAC returns the MAC address of the first non-loopback network interface.
func getPrimaryMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}
