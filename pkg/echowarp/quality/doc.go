// Package quality provides real-time connection quality monitoring for WebRTC connections.
//
// This package monitors WebRTC peer connections and calculates quality metrics based on
// network statistics including latency, jitter, packet loss, and throughput. It provides
// both programmatic access to quality data and visual representations suitable for TUIs.
//
// # Features
//
//   - Real-time stats collection from WebRTC peer connections
//   - Quality level calculation (Excellent, Good, Fair, Poor)
//   - Visual representations (emoji, ASCII) for TUI integration
//   - Prometheus metrics export
//   - Non-blocking updates via channels
//   - Thread-safe access to current quality
//
// # Quality Levels
//
// Quality is assessed based on latency and packet loss:
//
//   - Excellent: latency < 50ms, packet loss < 1%
//   - Good: latency < 100ms, packet loss < 3%
//   - Fair: latency < 200ms, packet loss < 5%
//   - Poor: latency >= 200ms or packet loss >= 5%
//
// # Basic Usage
//
//	indicator := quality.NewQualityIndicator(peerConnection)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	updateChan := indicator.Start(ctx)
//	defer indicator.Stop()
//
//	for quality := range updateChan {
//	    fmt.Println(indicator.FormatForTUI(quality))
//	}
//
// # Integration with TUI
//
// The package provides helper methods for visual representation:
//
//	quality := indicator.GetCurrentQuality()
//	emoji := quality.Quality.Emoji()        // "🟢" for excellent
//	ascii := quality.Quality.ASCII()        // "[====]" for excellent
//	formatted := indicator.FormatForTUI(quality)
//
// # Prometheus Metrics
//
// The package automatically exports the following metrics:
//   - echowarp_connection_latency_milliseconds
//   - echowarp_connection_jitter_milliseconds
//   - echowarp_connection_packet_loss_percent
//   - echowarp_connection_bitrate_bits_per_second
//   - echowarp_connection_quality_level (0=Excellent, 1=Good, 2=Fair, 3=Poor)
//
// # WebRTC Stats Collected
//
//   - RTT (round-trip time) from ICECandidatePairStats
//   - Jitter from InboundRTPStreamStats
//   - Packet loss from InboundRTPStreamStats
//   - Bytes sent/received for throughput calculation
package quality
