package app

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func testPipelineLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func defaultPipelineConfig() CapturePipelineConfig {
	return CapturePipelineConfig{
		SampleRate: 48000,
		Channels:   1,
		DeviceID:   0,
		EncoderConfig: EncoderConfig{
			Bitrate:     64000,
			Complexity:  10,
			DTX:         true,
			FEC:         true,
			Application: audio.OpusApplicationVoIP,
		},
	}
}

func TestNewCapturePipeline(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	logger := testPipelineLogger()

	p := NewCapturePipeline(cfg, logger)

	require.NotNil(t, p)
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	assert.Equal(t, cfg.Channels, p.cfg.Channels)
	assert.Equal(t, cfg.DeviceID, p.cfg.DeviceID)
	assert.Equal(t, cfg.EncoderConfig.Bitrate, p.cfg.EncoderConfig.Bitrate)
	assert.Equal(t, cfg.EncoderConfig.Application, p.cfg.EncoderConfig.Application)
	require.NotNil(t, p.configureEncoder, "configureEncoder hook should be set")

	// Verify configureEncoder is the default hook (sets only bitrate)
	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)
	assert.NotPanics(t, func() { p.configureEncoder(enc) })
}

func TestNewServerCapturePipeline(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Complexity = 8
	cfg.EncoderConfig.DTX = true
	cfg.EncoderConfig.FEC = true
	logger := testPipelineLogger()

	p := NewServerCapturePipeline(cfg, logger)

	require.NotNil(t, p)
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	assert.Equal(t, 8, p.cfg.EncoderConfig.Complexity)
	assert.True(t, p.cfg.EncoderConfig.DTX)
	assert.True(t, p.cfg.EncoderConfig.FEC)
	require.NotNil(t, p.configureEncoder, "server configureEncoder hook should be set")

	// Verify server hook applies all settings without panic
	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)
	assert.NotPanics(t, func() { p.configureEncoder(enc) })
}

func TestNewClientCapturePipeline(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	logger := testPipelineLogger()

	p := NewClientCapturePipeline(cfg, logger)

	require.NotNil(t, p)
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	assert.Equal(t, cfg.Channels, p.cfg.Channels)
	assert.Equal(t, cfg.EncoderConfig.Bitrate, p.cfg.EncoderConfig.Bitrate)
	require.NotNil(t, p.configureEncoder)
}

func TestCapturePipeline_ConfigureEncoder(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Bitrate = 96000
	logger := testPipelineLogger()

	p := NewCapturePipeline(cfg, logger)

	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)

	p.configureEncoder(enc)
}

func TestServerCapturePipeline_ConfigureEncoder(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Bitrate = 96000
	cfg.EncoderConfig.Complexity = 5
	cfg.EncoderConfig.DTX = true
	cfg.EncoderConfig.FEC = true
	logger := testPipelineLogger()

	p := NewServerCapturePipeline(cfg, logger)

	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)

	p.configureEncoder(enc)
}

func TestCapturePipeline_Run_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	logger := testPipelineLogger()

	p := NewCapturePipeline(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sendCh := make(chan []byte, 1)

	err := p.Run(ctx, sendCh)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestCapturePipeline_Run_InvalidEncoderApplication(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Application = "invalid"
	logger := testPipelineLogger()

	p := NewCapturePipeline(cfg, logger)

	ctx := context.Background()
	sendCh := make(chan []byte, 1)

	err := p.Run(ctx, sendCh)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create opus encoder")
}

func TestEncoderConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := EncoderConfig{
		Bitrate:     64000,
		Complexity:  10,
		DTX:         false,
		FEC:         false,
		Application: audio.OpusApplicationVoIP,
	}

	assert.Equal(t, 64000, cfg.Bitrate)
	assert.Equal(t, 10, cfg.Complexity)
	assert.False(t, cfg.DTX)
	assert.False(t, cfg.FEC)
	assert.Equal(t, audio.OpusApplicationVoIP, cfg.Application)
}

func TestCapturePipelineConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := CapturePipelineConfig{
		SampleRate: 44100,
		Channels:   2,
		DeviceID:   1,
		EncoderConfig: EncoderConfig{
			Bitrate:     128000,
			Complexity:  8,
			DTX:         true,
			FEC:         true,
			Application: audio.OpusApplicationAudio,
		},
	}

	assert.Equal(t, uint32(44100), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, uint32(1), cfg.DeviceID)
	assert.Equal(t, 128000, cfg.EncoderConfig.Bitrate)
	assert.Equal(t, audio.OpusApplicationAudio, cfg.EncoderConfig.Application)
}

func TestCapturePipeline_Run_CancelledContext(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	logger := testPipelineLogger()

	p := NewCapturePipeline(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sendCh := make(chan []byte, 1)
	err := p.Run(ctx, sendCh)

	assert.Error(t, err)
}

func TestServerCapturePipeline_ConfigureEncoder_AllSettings(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Bitrate = 128000
	cfg.EncoderConfig.Complexity = 10
	cfg.EncoderConfig.DTX = true
	cfg.EncoderConfig.FEC = true
	logger := testPipelineLogger()

	p := NewServerCapturePipeline(cfg, logger)

	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)

	p.configureEncoder(enc)
}

func TestClientCapturePipeline_Creation(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()
	cfg.EncoderConfig.Bitrate = 96000
	logger := testPipelineLogger()

	p := NewClientCapturePipeline(cfg, logger)

	require.NotNil(t, p)
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	assert.Equal(t, 96000, p.cfg.EncoderConfig.Bitrate, "custom bitrate must be preserved")
	assert.Equal(t, cfg.Channels, p.cfg.Channels)
	require.NotNil(t, p.configureEncoder)

	// Verify client encoder hook works
	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.EncoderConfig.Application)
	require.NoError(t, err)
	assert.NotPanics(t, func() { p.configureEncoder(enc) })
}

func TestCapturePipeline_NewWithNilLogger(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()

	p := NewCapturePipeline(cfg, nil)

	require.NotNil(t, p)
	assert.Nil(t, p.logger, "nil logger should be stored as-is")
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	require.NotNil(t, p.configureEncoder)
}

func TestServerCapturePipeline_NewWithNilLogger(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()

	p := NewServerCapturePipeline(cfg, nil)

	require.NotNil(t, p)
	assert.Nil(t, p.logger, "nil logger should be stored as-is")
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	require.NotNil(t, p.configureEncoder)
}

func TestClientCapturePipeline_NewWithNilLogger(t *testing.T) {
	t.Parallel()
	cfg := defaultPipelineConfig()

	p := NewClientCapturePipeline(cfg, nil)

	require.NotNil(t, p)
	assert.Nil(t, p.logger, "nil logger should be stored as-is")
	assert.Equal(t, cfg.SampleRate, p.cfg.SampleRate)
	require.NotNil(t, p.configureEncoder)
}
