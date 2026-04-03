// Package metrics provides Prometheus metrics for monitoring EchoWarp connections,
// audio streaming, and authentication. Metrics are automatically registered with
// the default Prometheus registry.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echowarp_connections_total",
		Help: "Total number of connections",
	}, []string{"role", "status"})

	ConnectionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "echowarp_connection_duration_seconds",
		Help:    "Duration of connections",
		Buckets: prometheus.ExponentialBuckets(1, 2, 15),
	})

	AudioBytesSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "echowarp_audio_bytes_sent_total",
		Help: "Total audio bytes sent",
	})

	AudioBytesRecv = promauto.NewCounter(prometheus.CounterOpts{
		Name: "echowarp_audio_bytes_received_total",
		Help: "Total audio bytes received",
	})

	AudioLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "echowarp_audio_latency_seconds",
		Help:    "Audio latency",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
	})

	AuthAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echowarp_auth_attempts_total",
		Help: "Total authentication attempts",
	}, []string{"status"})

	RateLimiterRejections = promauto.NewCounter(prometheus.CounterOpts{
		Name: "echowarp_rate_limiter_rejections_total",
		Help: "Total requests rejected by rate limiter",
	})

	ConnectionPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_pool_size",
		Help: "Total number of connections in pool",
	})

	ConnectionPoolActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_pool_active",
		Help: "Number of active connections",
	})

	EventBusWorkerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_eventbus_worker_pool_size",
		Help: "Maximum number of concurrent event handlers",
	})

	EventBusWorkersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_eventbus_workers_active",
		Help: "Number of currently active event handlers",
	})

	ChannelBufferSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "echowarp_channel_buffer_size",
		Help: "Current number of items in channel buffer",
	}, []string{"channel"})

	ChannelBufferCapacity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "echowarp_channel_buffer_capacity",
		Help: "Channel buffer capacity",
	}, []string{"channel"})

	FramesDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "echowarp_frames_dropped_total",
		Help: "Total frames dropped due to full buffer",
	}, []string{"channel"})

	// Connection quality metrics
	ConnectionLatency = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_latency_milliseconds",
		Help: "Current connection latency (round-trip time) in milliseconds",
	})

	ConnectionJitter = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_jitter_milliseconds",
		Help: "Current connection jitter in milliseconds",
	})

	ConnectionPacketLoss = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_packet_loss_percent",
		Help: "Current packet loss percentage",
	})

	ConnectionBitrate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_bitrate_bits_per_second",
		Help: "Current connection bitrate in bits per second",
	})

	ConnectionQualityLevel = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "echowarp_connection_quality_level",
		Help: "Connection quality level (0=Excellent, 1=Good, 2=Fair, 3=Poor)",
	})
)

// Label values for metrics.
const (
	// StatusSuccess indicates a successful operation.
	StatusSuccess = "success"
	// StatusFailed indicates a failed operation.
	StatusFailed = "failed"
	// RoleServer indicates server-side context.
	RoleServer = "server"
	// RoleClient indicates client-side context.
	RoleClient = "client"
)
