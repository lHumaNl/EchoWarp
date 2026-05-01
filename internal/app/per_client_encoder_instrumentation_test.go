package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

const blockedSendSchedulerYields = 10

func TestPerClientEncoder_InstrumentationActiveMarkerLogsOnStart(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	logs := &lockedLogBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		cfg := lagInstrumentationEncoderConfig(&PerClientEncoderStats{})
		cfg.HeartbeatInterval = time.Hour
		done <- runPerClientEncoder(ctx, sub, cfg, make(chan []byte, 1), testJSONLogger(logs, slog.LevelInfo))
	}()

	record := waitForSlogRecord(t, logs, "Per-client audio instrumentation active")
	assert.Equal(t, "client-1", record["clientID"])
	assert.Equal(t, "Client One", record["nickname"])
	assert.Equal(t, float64(4), record["sub_queue_cap"])
	assert.Equal(t, float64(3600000), record["heartbeat_interval_ms"])
	assert.Equal(t, float64(3600000), record["encode_warn_ms"])
	assert.Equal(t, float64(3600000), record["send_wait_warn_ms"])

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, done), context.Canceled)
}

func TestPerClientEncoder_HeartbeatLogsZeroCounters(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	logs := &lockedLogBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		cfg := lagInstrumentationEncoderConfig(&PerClientEncoderStats{})
		cfg.HeartbeatInterval = time.Millisecond
		done <- runPerClientEncoder(ctx, sub, cfg, make(chan []byte, 1), testJSONLogger(logs, slog.LevelDebug))
	}()

	record := waitForSlogRecord(t, logs, "Per-client audio pipeline stats")
	assert.Equal(t, "client-1", record["clientID"])
	assert.Equal(t, false, record["muted"])
	assert.Equal(t, float64(0), record["subscriber_drops_total"])
	assert.Equal(t, float64(0), record["sub_queue_len"])
	assert.Equal(t, float64(4), record["sub_queue_cap"])
	assert.Equal(t, float64(0), record["sub_queue_max"])
	assert.Equal(t, float64(0), record["encode_count"])
	assert.Equal(t, float64(0), record["send_count"])

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, done), context.Canceled)
}

func TestPerClientEncoder_HeartbeatIncludesMutedState(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	logs := &lockedLogBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		cfg := lagInstrumentationEncoderConfig(&PerClientEncoderStats{})
		cfg.HeartbeatInterval = time.Millisecond
		cfg.Muted = func() bool { return true }
		done <- runPerClientEncoder(ctx, sub, cfg, make(chan []byte, 1), testJSONLogger(logs, slog.LevelDebug))
	}()

	record := waitForSlogRecord(t, logs, "Per-client audio pipeline stats")
	assert.Equal(t, true, record["muted"])

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, done), context.Canceled)
}

func TestPerClientEncoder_AudioPipelineStatsIncludesPausedState(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	defer sub.Close()
	logs := &lockedLogBuffer{}
	cfg := lagInstrumentationEncoderConfig(&PerClientEncoderStats{})
	cfg.Paused = func() bool { return true }
	monitor := newPerClientEncoderMonitor(cfg, sub, testJSONLogger(logs, slog.LevelDebug))

	monitor.LogHeartbeat()
	record := decodeFirstSlogJSONRecord(t, logs.Bytes())
	assert.Equal(t, "Per-client audio pipeline stats", record["msg"])
	assert.Equal(t, true, record["paused"])
}

func TestPerClientEncoder_AudioLagInstrumentationLogsQueueDepth(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	for i := 0; i < 3; i++ {
		hub.dispatch(fullPCMFrame(48000, 1, 0.25))
	}

	stats := &PerClientEncoderStats{}
	logs := &lockedLogBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	sendCh := make(chan []byte, 8)
	go func() {
		cfg := lagInstrumentationEncoderConfig(stats)
		cfg.Muted = func() bool { return true }
		done <- runPerClientEncoder(ctx, sub, cfg, sendCh, testJSONLogger(logs, slog.LevelInfo))
	}()

	packet := waitForPerClientPacket(t, done, sendCh)
	require.NotEmpty(t, packet)
	require.Eventually(t, func() bool { return stats.Snapshot().QueueMaxDepth > 0 }, time.Second, time.Millisecond)

	record := waitForSlogRecord(t, logs, "Per-client audio pipeline lag")
	assert.Equal(t, "client-1", record["clientID"])
	assert.Equal(t, "Client One", record["nickname"])
	assert.Equal(t, true, record["muted"])
	assert.Greater(t, record["sub_queue_max"].(float64), float64(0))
	assert.Equal(t, float64(4), record["sub_queue_cap"])
	assert.GreaterOrEqual(t, stats.Snapshot().SendCount, uint64(1))

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, done), context.Canceled)
}

func TestPerClientEncoder_BackpressureCancellationRecordsSendWait(t *testing.T) {
	t.Parallel()

	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	sub := hub.Subscribe("client-1")
	stats := &PerClientEncoderStats{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	sendCh := make(chan []byte, 1)
	sendCh <- []byte("occupied")
	cfg := lagInstrumentationEncoderConfig(stats)
	cfg.SendWaitWarnThreshold = time.Nanosecond

	go func() {
		done <- runPerClientEncoder(ctx, sub, cfg, sendCh, testHubLogger())
	}()
	hub.dispatch(fullPCMFrame(48000, 1, 0.25))
	waitForBlockedPerClientSend(t, done, stats)

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, done), context.Canceled)
	snapshot := stats.Snapshot()
	assert.Equal(t, uint64(1), snapshot.SendCount)
	assert.Equal(t, uint64(1), snapshot.SendSlow)
	assert.Greater(t, snapshot.SendWaitMax, time.Duration(0))
	assert.Equal(t, 1, len(sendCh))
}

func TestPerClientEncoder_BackpressureIsolation_BlockedClientDropsWhileOtherSends(t *testing.T) {
	hub := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())
	slowSub := hub.Subscribe("slow")
	fastSub := hub.Subscribe("fast")

	slowStats := &PerClientEncoderStats{}
	fastStats := &PerClientEncoderStats{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slowDone := make(chan error, 1)
	fastDone := make(chan error, 1)
	slowSendCh := make(chan []byte, 1)
	fastSendCh := make(chan []byte, 16)
	slowSendCh <- []byte("occupied")

	slowCfg := lagInstrumentationEncoderConfig(slowStats)
	slowCfg.ClientID = "slow"
	slowCfg.Nickname = "Slow Client"
	slowCfg.SendWaitWarnThreshold = time.Nanosecond

	fastCfg := lagInstrumentationEncoderConfig(fastStats)
	fastCfg.ClientID = "fast"
	fastCfg.Nickname = "Fast Client"

	go func() {
		slowDone <- runPerClientEncoder(ctx, slowSub, slowCfg, slowSendCh, testHubLogger())
	}()
	go func() {
		fastDone <- runPerClientEncoder(ctx, fastSub, fastCfg, fastSendCh, testHubLogger())
	}()

	hub.dispatch(fullPCMFrame(48000, 1, 0.25))
	waitForBlockedPerClientSend(t, slowDone, slowStats)

	packets := [][]byte{waitForPerClientPacket(t, fastDone, fastSendCh)}
	for i := 0; i < 5; i++ {
		hub.dispatch(fullPCMFrame(48000, 1, 0.25+float32(i)*0.01))
		packets = append(packets, waitForPerClientPacket(t, fastDone, fastSendCh))
	}

	for i, packet := range packets {
		require.NotEmptyf(t, packet, "fast client packet %d should not be empty", i)
	}

	require.Eventually(t, func() bool {
		return fastStats.Snapshot().SendCount == 6
	}, time.Second, time.Millisecond)

	assert.Equal(t, uint64(1), slowSub.DropCount())
	assert.Equal(t, uint64(0), fastSub.DropCount())
	assert.Equal(t, 4, slowSub.QueueDepth())
	assert.Equal(t, 0, fastSub.QueueDepth())
	assert.Equal(t, uint64(0), slowStats.Snapshot().SendCount)

	cancel()
	require.ErrorIs(t, waitForPerClientDone(t, slowDone), context.Canceled)
	require.ErrorIs(t, waitForPerClientDone(t, fastDone), context.Canceled)

	slowSnapshot := slowStats.Snapshot()
	assert.Equal(t, uint64(1), slowSnapshot.SendCount)
	assert.Equal(t, uint64(1), slowSnapshot.SendSlow)
	assert.Greater(t, slowSnapshot.SendWaitMax, time.Duration(0))

	fastSnapshot := fastStats.Snapshot()
	assert.Equal(t, uint64(6), fastSnapshot.SendCount)
	assert.Equal(t, uint64(0), fastSnapshot.SendSlow)
	assert.Equal(t, 1, len(slowSendCh))
}

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *lockedLogBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func testJSONLogger(logs io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: level}))
}

func lagInstrumentationEncoderConfig(stats *PerClientEncoderStats) PerClientEncoderConfig {
	return PerClientEncoderConfig{
		ClientID:                "client-1",
		Nickname:                "Client One",
		SampleRate:              48000,
		Channels:                1,
		Encoder:                 EncoderConfig{Bitrate: 64000, Application: audio.OpusApplicationVoIP},
		Stats:                   stats,
		InstrumentationInterval: time.Nanosecond,
		EncodeWarnThreshold:     time.Hour,
		SendWaitWarnThreshold:   time.Hour,
	}
}

func waitForPerClientPacket(t *testing.T, done <-chan error, encoded <-chan []byte) []byte {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("per-client encoder exited before producing audio: %v", err)
	case packet := <-encoded:
		return packet
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for per-client encoded audio")
	}
	return nil
}

func waitForPerClientPackets(t *testing.T, done <-chan error, encoded <-chan []byte, count int) [][]byte {
	t.Helper()
	packets := make([][]byte, 0, count)
	for len(packets) < count {
		select {
		case err := <-done:
			t.Fatalf("per-client encoder exited before producing audio: %v", err)
		case packet := <-encoded:
			packets = append(packets, packet)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for per-client encoded audio packet %d/%d", len(packets)+1, count)
		}
	}
	return packets
}

func waitForBlockedPerClientSend(t *testing.T, done <-chan error, stats *PerClientEncoderStats) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("per-client encoder exited before blocking on send: %v", err)
		case <-deadline:
			t.Fatal("timed out waiting for blocked per-client send")
		default:
		}
		if snapshot := stats.Snapshot(); snapshot.EncodeCount > 0 && snapshot.SendCount == 0 {
			assertEncoderRemainsBlocked(t, done, stats)
			return
		}
		runtime.Gosched()
	}
}

func assertEncoderRemainsBlocked(t *testing.T, done <-chan error, stats *PerClientEncoderStats) {
	t.Helper()
	for i := 0; i < blockedSendSchedulerYields; i++ {
		runtime.Gosched()
		select {
		case err := <-done:
			t.Fatalf("per-client encoder exited before cancellation: %v", err)
		default:
		}
		assert.Zero(t, stats.Snapshot().SendCount)
	}
}

func waitForPerClientDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for per-client encoder exit")
	}
	return nil
}

func waitForSlogRecord(t *testing.T, logs *lockedLogBuffer, msg string) map[string]any {
	t.Helper()
	var record map[string]any
	require.Eventually(t, func() bool {
		for _, candidate := range decodeSlogJSONRecords(t, logs.Bytes()) {
			if candidate["msg"] == msg {
				record = candidate
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
	return record
}

func decodeFirstSlogJSONRecord(t *testing.T, data []byte) map[string]any {
	t.Helper()
	records := decodeSlogJSONRecords(t, data)
	require.NotEmpty(t, records)
	return records[0]
}

func decodeSlogJSONRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	records := make([]map[string]any, 0)
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			return records
		}
		require.NoError(t, err)
		records = append(records, record)
	}
}
