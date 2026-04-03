//go:build linux

package cli

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// systemMemoryMB returns total physical RAM in megabytes on Linux via /proc/meminfo.
func systemMemoryMB() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb / 1024
		}
	}
	return 0
}
