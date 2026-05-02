package app

import (
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	perClientLagLogInterval      = 2 * time.Second
	perClientHeartbeatInterval   = 5 * time.Second
	perClientEncodeWarnThreshold = 10 * time.Millisecond
	perClientSendWaitThreshold   = 5 * time.Millisecond
)

// PerClientEncoderStatsSnapshot is a point-in-time view of encoder lag counters.
type PerClientEncoderStatsSnapshot struct {
	EncodeCount   uint64
	EncodeSlow    uint64
	EncodeMax     time.Duration
	SendCount     uint64
	SendSlow      uint64
	SendWaitMax   time.Duration
	QueueDepth    int
	QueueCapacity int
	QueueMaxDepth int
}

// PerClientEncoderStats stores low-overhead per-client audio send-path counters.
type PerClientEncoderStats struct {
	encodeCount   atomic.Uint64
	encodeSlow    atomic.Uint64
	encodeMaxNs   atomic.Int64
	sendCount     atomic.Uint64
	sendSlow      atomic.Uint64
	sendMaxNs     atomic.Int64
	queueDepth    atomic.Int64
	queueCapacity atomic.Int64
	queueMaxDepth atomic.Int64
}

func (s *PerClientEncoderStats) Snapshot() PerClientEncoderStatsSnapshot {
	if s == nil {
		return PerClientEncoderStatsSnapshot{}
	}
	return PerClientEncoderStatsSnapshot{
		EncodeCount:   s.encodeCount.Load(),
		EncodeSlow:    s.encodeSlow.Load(),
		EncodeMax:     time.Duration(s.encodeMaxNs.Load()),
		SendCount:     s.sendCount.Load(),
		SendSlow:      s.sendSlow.Load(),
		SendWaitMax:   time.Duration(s.sendMaxNs.Load()),
		QueueDepth:    int(s.queueDepth.Load()),
		QueueCapacity: int(s.queueCapacity.Load()),
		QueueMaxDepth: int(s.queueMaxDepth.Load()),
	}
}

func (s *PerClientEncoderStats) recordEncode(duration, threshold time.Duration) {
	s.encodeCount.Add(1)
	if duration >= threshold {
		s.encodeSlow.Add(1)
	}
	updateMaxInt64(&s.encodeMaxNs, duration.Nanoseconds())
}

func (s *PerClientEncoderStats) recordSendWait(duration, threshold time.Duration) {
	s.sendCount.Add(1)
	if duration >= threshold {
		s.sendSlow.Add(1)
	}
	updateMaxInt64(&s.sendMaxNs, duration.Nanoseconds())
}

func (s *PerClientEncoderStats) recordQueue(depth, capacity int) {
	s.queueDepth.Store(int64(depth))
	s.queueCapacity.Store(int64(capacity))
	updateMaxInt64(&s.queueMaxDepth, int64(depth))
}

type perClientEncoderMonitor struct {
	clientID      string
	nickname      string
	logger        *slog.Logger
	sub           *CaptureSubscription
	stats         *PerClientEncoderStats
	interval      time.Duration
	heartbeat     time.Duration
	encodeWarn    time.Duration
	sendWaitWarn  time.Duration
	muted         func() bool
	paused        func() bool
	nextLog       time.Time
	encodeCount   uint64
	encodeSlow    uint64
	encodeMax     time.Duration
	sendCount     uint64
	sendSlow      uint64
	sendWaitMax   time.Duration
	queueMaxDepth int
	dropsLastLog  uint64
}

func newPerClientEncoderMonitor(cfg PerClientEncoderConfig, sub *CaptureSubscription, logger *slog.Logger) *perClientEncoderMonitor {
	stats := cfg.Stats
	if stats == nil {
		stats = &PerClientEncoderStats{}
	}
	clientID := nonEmptyString(cfg.ClientID, sub.clientID)
	nickname := nonEmptyString(cfg.Nickname, clientID)
	interval := durationOrDefault(cfg.InstrumentationInterval, perClientLagLogInterval)
	return &perClientEncoderMonitor{
		clientID: clientID, nickname: nickname, logger: logger, sub: sub, stats: stats,
		interval:     interval,
		heartbeat:    durationOrDefault(cfg.HeartbeatInterval, perClientHeartbeatInterval),
		encodeWarn:   durationOrDefault(cfg.EncodeWarnThreshold, perClientEncodeWarnThreshold),
		sendWaitWarn: durationOrDefault(cfg.SendWaitWarnThreshold, perClientSendWaitThreshold),
		muted:        cfg.Muted,
		paused:       cfg.Paused,
		nextLog:      initialPerClientLagLogTime(interval),
	}
}

func initialPerClientLagLogTime(interval time.Duration) time.Time {
	now := time.Now()
	if interval <= time.Nanosecond {
		return now
	}
	return now.Add(interval)
}

func (m *perClientEncoderMonitor) HeartbeatInterval() time.Duration {
	return m.heartbeat
}

func (m *perClientEncoderMonitor) LogActive() {
	m.logger.Info("Per-client audio instrumentation active",
		"clientID", m.clientID,
		"nickname", m.nickname,
		"sub_queue_cap", m.sub.QueueCapacity(),
		"heartbeat_interval_ms", durationMillis(m.heartbeat),
		"encode_warn_ms", durationMillis(m.encodeWarn),
		"send_wait_warn_ms", durationMillis(m.sendWaitWarn))
}

func (m *perClientEncoderMonitor) ObserveQueue() {
	depth, capacity := m.sub.QueueDepth(), m.sub.QueueCapacity()
	m.stats.recordQueue(depth, capacity)
	if depth > m.queueMaxDepth {
		m.queueMaxDepth = depth
	}
}

func (m *perClientEncoderMonitor) RecordEncode(duration time.Duration) {
	m.encodeCount++
	if duration >= m.encodeWarn {
		m.encodeSlow++
	}
	if duration > m.encodeMax {
		m.encodeMax = duration
	}
	m.stats.recordEncode(duration, m.encodeWarn)
}

func (m *perClientEncoderMonitor) RecordSendWait(duration time.Duration) {
	m.sendCount++
	if duration >= m.sendWaitWarn {
		m.sendSlow++
	}
	if duration > m.sendWaitMax {
		m.sendWaitMax = duration
	}
	m.stats.recordSendWait(duration, m.sendWaitWarn)
}

func (m *perClientEncoderMonitor) LogIfDue(now time.Time) {
	if now.Before(m.nextLog) {
		return
	}
	drops := m.sub.DropCount() - m.dropsLastLog
	if m.shouldLog(drops) {
		m.logLag(drops)
	}
	m.resetInterval(now)
}

func (m *perClientEncoderMonitor) LogCurrentIfUseful() {
	drops := m.sub.DropCount() - m.dropsLastLog
	if !m.shouldLog(drops) {
		return
	}
	m.logLag(drops)
	m.resetInterval(time.Now())
}

func (m *perClientEncoderMonitor) shouldLog(drops uint64) bool {
	return drops > 0 || m.queueMaxDepth > 0 || m.encodeMax >= m.encodeWarn || m.sendWaitMax >= m.sendWaitWarn
}

func (m *perClientEncoderMonitor) LogHeartbeat() {
	m.logger.Debug("Per-client audio pipeline stats", m.pipelineStatsFields(0, false)...)
}

func (m *perClientEncoderMonitor) logLag(drops uint64) {
	m.logger.Warn("Per-client audio pipeline lag", m.pipelineStatsFields(drops, true)...)
}

func (m *perClientEncoderMonitor) pipelineStatsFields(drops uint64, includeDrops bool) []any {
	fields := []any{
		"clientID", m.clientID,
		"nickname", m.nickname,
		"muted", m.isMuted(),
	}
	if m.paused != nil {
		fields = append(fields, "paused", m.paused())
	}
	if includeDrops {
		fields = append(fields, "subscriber_drops", drops)
	}
	return append(fields,
		"subscriber_drops_total", m.sub.DropCount(),
		"sub_queue_len", m.sub.QueueDepth(),
		"sub_queue_cap", m.sub.QueueCapacity(),
		"sub_queue_max", m.queueMaxDepth,
		"encode_count", m.encodeCount,
		"encode_slow", m.encodeSlow,
		"encode_max_ms", durationMillis(m.encodeMax),
		"send_count", m.sendCount,
		"send_wait_slow", m.sendSlow,
		"send_wait_max_ms", durationMillis(m.sendWaitMax))
}

func (m *perClientEncoderMonitor) isMuted() bool {
	return m.muted != nil && m.muted()
}

func (m *perClientEncoderMonitor) resetInterval(now time.Time) {
	m.nextLog = now.Add(m.interval)
	m.encodeCount, m.encodeSlow, m.encodeMax = 0, 0, 0
	m.sendCount, m.sendSlow, m.sendWaitMax = 0, 0, 0
	m.queueMaxDepth = 0
	m.dropsLastLog = m.sub.DropCount()
}

func updateMaxInt64(value *atomic.Int64, candidate int64) {
	for {
		current := value.Load()
		if candidate <= current || value.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func nonEmptyString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
