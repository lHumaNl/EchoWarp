package auth

import (
	"os"
	"strings"
)

// getMachineID reads /etc/machine-id on Linux.
func getMachineID() (string, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
