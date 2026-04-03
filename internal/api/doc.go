// Package api provides HTTP REST API and WebSocket for daemon control.
//
// The API allows external control of the EchoWarp daemon:
//   - GET /api/v1/status - Current status
//   - GET /api/v1/stats - Connection statistics
//   - GET /api/v1/devices - Audio devices
//   - POST /api/v1/start - Start streaming
//   - POST /api/v1/stop - Stop streaming
//   - GET /ws/v1/events - WebSocket event stream
//   - GET /metrics - Prometheus metrics
//
// All endpoints require Bearer token authentication.
package api
