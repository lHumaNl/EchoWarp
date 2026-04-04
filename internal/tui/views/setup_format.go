package views

import (
	"fmt"
	"strconv"
	"strings"
)

// formatSampleRate returns a human-readable sample rate string like "48 kHz".
func formatSampleRate(sr uint32) string {
	if sr >= 1000 {
		khz := float64(sr) / 1000.0
		if khz == float64(int(khz)) {
			return fmt.Sprintf("%d kHz", int(khz))
		}
		return fmt.Sprintf("%.1f kHz", khz)
	}
	return fmt.Sprintf("%d Hz", sr)
}

// FormatSampleRate returns a human-readable sample rate string like "48 kHz".
func FormatSampleRate(sr uint32) string {
	if sr%1000 == 0 {
		return fmt.Sprintf("%d kHz", sr/1000)
	}
	return fmt.Sprintf("%.1f kHz", float64(sr)/1000)
}

// FormatBitrate returns a human-readable bitrate string like "64 kbps".
func FormatBitrate(br int) string {
	if br%1000 == 0 {
		return fmt.Sprintf("%d kbps", br/1000)
	}
	return fmt.Sprintf("%.1f kbps", float64(br)/1000)
}

// parseSampleRate parses "48 kHz" → 48000, "44.1 kHz" → 44100.
func parseSampleRate(s string) uint32 {
	s = strings.TrimSuffix(strings.TrimSpace(s), " kHz")
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return uint32(v * 1000)
	}
	return 0
}

// parseBitrate parses "64 kbps" → 64000.
func parseBitrate(s string) int {
	s = strings.TrimSuffix(strings.TrimSpace(s), " kbps")
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return int(v * 1000)
	}
	return 0
}

func FormatChannels(ch uint32) string {
	switch ch {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		return fmt.Sprintf("%dch", ch)
	}
}

// formatBitDepth returns a human-readable bit depth string like "16-bit".
func formatBitDepth(bd uint32) string {
	if bd > 0 {
		return fmt.Sprintf("%d-bit", bd)
	}
	return ""
}

// formatDeviceInfo builds the info string with fixed-width columns for alignment.
// Layout: "stereo  32-bit  44.1 kHz" with each field padded to a consistent width.
func formatDeviceInfo(dev deviceRow) string {
	ch := ""
	if dev.Channels > 0 {
		ch = FormatChannels(dev.Channels)
	}
	bd := ""
	if dev.BitDepth > 0 {
		bd = formatBitDepth(dev.BitDepth)
	}
	sr := ""
	if dev.IsVirtual {
		sr = "adaptive"
	} else if dev.SampleRate > 0 {
		sr = formatSampleRate(dev.SampleRate)
	}
	// Fixed column widths: channels=7, bit depth=6, sample rate=8
	return fmt.Sprintf("%-7s %-6s %8s", ch, bd, sr)
}
