//go:build !darwin && !linux

package audio

// stubPlatformAEC is returned on platforms without a native AEC backend (Windows, etc.).
type stubPlatformAEC struct{}

func getPlatformAEC() PlatformAEC {
	return nil
}
