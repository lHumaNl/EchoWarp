package preset

import (
	"os"
	"path/filepath"
)

const presetDirectoryMode = 0700

func writeServerPresets(data []byte) error {
	dir := filepath.Dir(FilePath())
	if err := os.MkdirAll(dir, presetDirectoryMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, presetDirectoryMode); err != nil {
		return err
	}
	return replaceServerPresets(dir, data)
}

func replaceServerPresets(dir string, data []byte) error {
	// A private temporary file avoids exposing secrets through an existing permissive file.
	file, err := os.CreateTemp(dir, ".server-presets-*") // CreateTemp uses 0600.
	if err != nil {
		return err
	}
	// Cleanup is best-effort; after a successful rename the temporary path is absent.
	defer func() { _ = os.Remove(file.Name()) }()
	if err := writeSnapshotFile(file, data); err != nil {
		return err
	}
	return os.Rename(file.Name(), FilePath())
}

func writeSnapshotFile(file *os.File, data []byte) error {
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
