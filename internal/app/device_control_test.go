package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func newTestMixer() *audio.AudioMixer {
	return audio.NewAudioMixer(audio.MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    960,
		BufferFrames: 3,
	})
}

func sendCmd(t *testing.T, ch chan<- DeviceCommand, cmd DeviceCommand) {
	t.Helper()
	select {
	case ch <- cmd:
	case <-time.After(time.Second):
		t.Fatal("timed out sending command")
	}
}

// waitDrain gives the consumer goroutine a moment to process the command.
func waitDrain() {
	time.Sleep(50 * time.Millisecond)
}

func TestDeviceMuteAppliesToMixer(t *testing.T) {
	mixer := newTestMixer()
	dummyCh := make(chan []float32, 1)
	mixer.AddSourceWithVolume("device-1", dummyCh, 1.0)

	cmdCh := make(chan DeviceCommand, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, mixer, nil, nil, slog.Default())

	// Initially not muted.
	require.False(t, mixer.IsSourceMuted("device-1"))

	// Toggle mute on.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleMute, DeviceID: 1})
	waitDrain()
	assert.True(t, mixer.IsSourceMuted("device-1"), "device should be muted after toggle")

	// Toggle mute off.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleMute, DeviceID: 1})
	waitDrain()
	assert.False(t, mixer.IsSourceMuted("device-1"), "device should be unmuted after second toggle")
}

func TestDeviceVolumeAppliesToMixer(t *testing.T) {
	mixer := newTestMixer()
	dummyCh := make(chan []float32, 1)
	mixer.AddSourceWithVolume("device-5", dummyCh, 1.0)

	cmdCh := make(chan DeviceCommand, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, mixer, nil, nil, slog.Default())

	// Volume up.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeUp, DeviceID: 5})
	waitDrain()
	assert.InDelta(t, 1.1, float64(mixer.GetSourceVolume("device-5")), 0.01)

	// Volume down twice → back to 0.9.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 5})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 5})
	waitDrain()
	assert.InDelta(t, 0.9, float64(mixer.GetSourceVolume("device-5")), 0.01)

	// Volume cannot exceed 1.5.
	for i := 0; i < 20; i++ {
		sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeUp, DeviceID: 5})
	}
	waitDrain()
	assert.InDelta(t, 1.5, float64(mixer.GetSourceVolume("device-5")), 0.01)

	// Volume cannot go below 0.
	for i := 0; i < 30; i++ {
		sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 5})
	}
	waitDrain()
	assert.InDelta(t, 0.0, float64(mixer.GetSourceVolume("device-5")), 0.01)
}

func TestAGCToggleAppliesToProcessor(t *testing.T) {
	agcProc := audio.NewAGCProcessor(audio.AGCConfig{SampleRate: 48000})
	require.True(t, agcProc.IsEnabled(), "AGC should be enabled by default")

	agcMap := map[uint32]*audio.AGCProcessor{
		7: agcProc,
	}

	cmdCh := make(chan DeviceCommand, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, nil, agcMap, nil, slog.Default())

	// Toggle AGC off.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleAGC, DeviceID: 7})
	waitDrain()
	assert.False(t, agcProc.IsEnabled(), "AGC should be disabled after toggle")

	// Toggle AGC back on.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleAGC, DeviceID: 7})
	waitDrain()
	assert.True(t, agcProc.IsEnabled(), "AGC should be re-enabled after second toggle")
}

func TestHandleDeviceCommands_NilMixer(t *testing.T) {
	// Mixer-dependent commands should be gracefully ignored when mixer is nil.
	cmdCh := make(chan DeviceCommand, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, nil, nil, nil, slog.Default())

	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleMute, DeviceID: 1})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceGlobalMute, DeviceID: 0})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeUp, DeviceID: 1})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 1})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleAGC, DeviceID: 1})
	waitDrain()

	// No panic — the test passes if we reach here.
}

func TestHandleDeviceCommands_GlobalMute(t *testing.T) {
	mixer := newTestMixer()
	dummyCh := make(chan []float32, 1)
	mixer.AddSourceWithVolume("device-1", dummyCh, 1.0)

	cmdCh := make(chan DeviceCommand, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, mixer, nil, nil, slog.Default())

	require.False(t, mixer.IsGlobalMuted())

	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceGlobalMute})
	waitDrain()
	assert.True(t, mixer.IsGlobalMuted(), "global mute should be active")

	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceGlobalMute})
	waitDrain()
	assert.False(t, mixer.IsGlobalMuted(), "global mute should be toggled off")
}

func TestHandleDeviceCommands_ContextCancel(t *testing.T) {
	cmdCh := make(chan DeviceCommand, 4)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		HandleDeviceCommands(ctx, cmdCh, nil, nil, nil, slog.Default())
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HandleDeviceCommands did not exit after context cancellation")
	}
}

func TestHandleDeviceCommands_ChannelClose(t *testing.T) {
	cmdCh := make(chan DeviceCommand, 4)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		HandleDeviceCommands(ctx, cmdCh, nil, nil, nil, slog.Default())
		close(done)
	}()

	close(cmdCh)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HandleDeviceCommands did not exit after channel close")
	}
}

func TestDeviceGainControl_VolumeAndMute(t *testing.T) {
	gainCtl := NewDeviceGainControl(1.0)

	cmdCh := make(chan DeviceCommand, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go HandleDeviceCommands(ctx, cmdCh, nil, nil, gainCtl, slog.Default())

	// Volume up.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeUp, DeviceID: 1})
	waitDrain()
	assert.InDelta(t, 1.1, float64(gainCtl.Gain()), 0.01)

	// Volume down x3 → 0.8.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 1})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 1})
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceVolumeDown, DeviceID: 1})
	waitDrain()
	assert.InDelta(t, 0.8, float64(gainCtl.Gain()), 0.01)

	// Mute toggle.
	require.False(t, gainCtl.IsMuted())
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceToggleMute, DeviceID: 1})
	waitDrain()
	assert.True(t, gainCtl.IsMuted())

	// Global mute also works on gain control.
	sendCmd(t, cmdCh, DeviceCommand{Action: DeviceGlobalMute})
	waitDrain()
	assert.False(t, gainCtl.IsMuted(), "second toggle should unmute")
}
