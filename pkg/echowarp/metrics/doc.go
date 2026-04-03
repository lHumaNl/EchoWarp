// Package metrics provides Prometheus metrics for monitoring EchoWarp.
//
// Available metrics:
//
// Connections:
//   - echowarp_connections_total (counter) - Total connections by role/status
//   - echowarp_connection_duration_seconds (histogram) - Connection duration
//   - echowarp_connection_pool_size (gauge) - Total connections in pool
//   - echowarp_connection_pool_active (gauge) - Active connections
//
// Audio:
//   - echowarp_audio_bytes_sent_total (counter) - Audio bytes sent
//   - echowarp_audio_bytes_received_total (counter) - Audio bytes received
//   - echowarp_audio_latency_seconds (histogram) - Audio latency
//
// Authentication:
//   - echowarp_auth_attempts_total (counter) - Auth attempts by status
//
// Rate Limiting:
//   - echowarp_rate_limiter_rejections_total (counter) - Rejected requests
//
// EventBus:
//   - echowarp_eventbus_worker_pool_size (gauge) - Max concurrent handlers
//   - echowarp_eventbus_workers_active (gauge) - Active handlers
//
// Backpressure:
//   - echowarp_channel_buffer_size (gauge) - Items in channel buffer
//   - echowarp_channel_buffer_capacity (gauge) - Buffer capacity
//   - echowarp_frames_dropped_total (counter) - Dropped frames
//
// All metrics are automatically registered with the default Prometheus registry.
// Access via GET /metrics endpoint when API server is running.
package metrics
