//go:build !windows

package virtualstate

import "os"

func atomicRename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
