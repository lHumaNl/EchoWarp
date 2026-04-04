package audio

// EstimateConferenceBandwidth calculates the estimated bandwidth in bits/sec
// for a conference with n participants at the given Opus bitrate.
//
// Each participant sends 1 stream and receives 1 personal mix.
// Server sends N personal mixes and receives N streams.
//
// Returns (perClientBps, serverBps).
func EstimateConferenceBandwidth(n int, opusBitrate int) (perClientBps int, serverBps int) {
	if n <= 0 {
		return 0, 0
	}

	// Opus overhead: ~40 bytes per 20ms packet = 16 kbit/s per stream
	overheadBps := 16000

	streamBps := opusBitrate + overheadBps

	// Client: sends 1 stream + receives 1 personal mix
	perClientBps = 2 * streamBps

	// Server: receives N streams + sends N personal mixes
	serverBps = 2 * n * streamBps

	return perClientBps, serverBps
}
