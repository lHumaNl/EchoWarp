// Package audio provides audio capture and playback functionality.
//
// This package handles:
//   - Audio device enumeration via malgo (miniaudio wrapper)
//   - PCM audio capture from input devices
//   - PCM audio playback to output devices
//   - Opus encoding/decoding for network transmission
//   - Audio mixing for multi-client scenarios
//   - Virtual microphone creation (platform-specific)
//
// The recommended sample rate is 48000 Hz with mono channel for voice
// communication, which is optimal for the Opus codec.
package audio
