package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// buildICEConfig constructs a transport.ICEConfig from the application config.
// Used by both ServerApp and ClientApp to avoid duplicating STUN/TURN setup.
// If udpMux is non-nil, all ICE traffic is multiplexed through a single UDP port.
func buildICEConfig(cfg config.Config, udpMux *transport.UDPMuxManager) transport.ICEConfig {
	iceConfig := transport.ICEConfig{STUNServers: cfg.STUNServers, UDPMux: udpMux}
	iceConfig.TURNServers = append(iceConfig.TURNServers, cfg.TURNServers...)
	return iceConfig
}

// setupAudioDecoder registers an OnAudioTrack callback that creates an Opus decoder
// and feeds decoded PCM frames into playbackCh. Closes playbackCh when the track ends.
// If spectrum is non-nil, each decoded frame is also fed to the spectrum analyzer.
func setupAudioDecoder(logger *slog.Logger, peer transport.PeerManager, sampleRate, channels uint32, playbackCh chan<- []float32, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter) {
	peer.OnAudioTrack(func(inCh <-chan []byte) {
		dec, err := audio.NewOpusDecoder(int(sampleRate), int(channels))
		if err != nil {
			logger.Error("Failed to create opus decoder", "error", err)
			return
		}
		decodeAudioStream(logger, inCh, dec, playbackCh, spectrum, level)
		close(playbackCh)
	})
}

// setupAudioDecoderWithJitter registers an OnAudioTrack callback that decodes audio
// into a JitterBuffer instead of a channel. Signals done via doneCh when the track ends.
func setupAudioDecoderWithJitter(logger *slog.Logger, peer transport.PeerManager, sampleRate, channels uint32, jb *audio.JitterBuffer, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter, doneCh chan<- struct{}) {
	peer.OnAudioTrack(func(inCh <-chan []byte) {
		dec, err := audio.NewOpusDecoder(int(sampleRate), int(channels))
		if err != nil {
			logger.Error("Failed to create opus decoder", "error", err)
			return
		}
		decodeAudioStreamToJitter(logger, inCh, dec, jb, spectrum, level)
		close(doneCh)
	})
}

// decodeAudioStream continuously decodes opus audio to PCM and sends copies to playbackCh.
// If spectrum/level are non-nil, each decoded PCM frame is fed to them.
func decodeAudioStream(logger *slog.Logger, inCh <-chan []byte, dec *audio.OpusDecoder, playbackCh chan<- []float32, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter) {
	for data := range inCh {
		metrics.AudioBytesRecv.Add(float64(len(data)))
		pcm, err := dec.Decode(data)
		if err != nil {
			logger.Warn("Opus decode error", "error", err)
			continue
		}
		// Copy PCM data: dec.Decode returns a slice backed by an internal
		// reusable buffer that will be overwritten on the next Decode call.
		pcmCopy := make([]float32, len(pcm))
		copy(pcmCopy, pcm)
		if spectrum != nil {
			spectrum.Feed(pcmCopy)
		}
		if level != nil {
			level.Feed(pcmCopy)
		}
		select {
		case playbackCh <- pcmCopy:
		default:
		}
	}
}

// decodeAudioStreamToJitter decodes opus audio and writes frames into a JitterBuffer.
// The decoder returns a reused internal buffer; spectrum/level Feed methods and
// jb.Write are all synchronous and either don't retain the slice (Feed) or make
// their own pooled copy (jb.Write), so passing `pcm` directly avoids an extra
// 4KB allocation per frame (~200 KB/sec of GC pressure at 48kHz/20ms frames).
func decodeAudioStreamToJitter(logger *slog.Logger, inCh <-chan []byte, dec *audio.OpusDecoder, jb *audio.JitterBuffer, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter) {
	for data := range inCh {
		metrics.AudioBytesRecv.Add(float64(len(data)))
		pcm, err := dec.Decode(data)
		if err != nil {
			logger.Warn("Opus decode error", "error", err)
			continue
		}
		if spectrum != nil {
			spectrum.Feed(pcm)
		}
		if level != nil {
			level.Feed(pcm)
		}
		jb.Write(pcm)
	}
}

// jitterPlaybackPump reads frames from a JitterBuffer on a 20ms ticker and sends them
// to playbackCh for the audio player. When the JitterBuffer returns nil (underrun),
// PLC (Packet Loss Concealment) is used to generate a smooth continuation frame
// instead of silence. The ticker is started only after the first frame is available
// to synchronize playback phase with incoming packet timing.
// Periodically calls AdaptiveAdjust based on the observed underrun rate, with
// aggressive adaptation during the initial warmup period.
// jitterPlaybackPump drains decoded frames from the jitter buffer on a 20ms
// tick and forwards them to playbackCh. When muteFlag is non-nil and set,
// the pump substitutes a silence frame of identical length so the downstream
// player keeps its timing (bypassing it entirely would cause underruns and
// audible pops on unmute).
// applyPlaybackGainMute applies AGC, gain, and mute in-place to the PCM frame.
// Order: AGC (normalize) -> gain -> soft-clip. Mute takes precedence over all.
// Any of the parameters may be nil — skipped individually.
func applyPlaybackGainMute(ctx context.Context, frame []float32, gainCtl *DeviceGainControl, muteFlag *atomic.Bool, agcProc *audio.AGCProcessor) {
	muted := (muteFlag != nil && muteFlag.Load()) || (gainCtl != nil && gainCtl.IsMuted())
	if muted {
		for i := range frame {
			frame[i] = 0
		}
		return
	}
	// AGC: normalize loudness before user gain. Process() is a no-op when
	// the processor is disabled, so always calling it is safe.
	if agcProc != nil {
		if processed, err := agcProc.Process(ctx, frame); err == nil && len(processed) == len(frame) {
			copy(frame, processed)
		}
	}
	if gainCtl != nil {
		gain := gainCtl.Gain()
		if gain != 1.0 {
			audio.MixGain(frame, gain)
			// Soft-clip only when amplifying (gain > 1.0). Without this,
			// samples that exceed ±1.0 are hard-clipped by the player,
			// giving harsh digital distortion. tanh gives smooth analog-
			// style saturation instead, so 150% actually sounds louder
			// rather than just noisier. Uses SIMD (AVX/SSE/NEON/Pure Go).
			if gain > 1.0 {
				audio.MixTanh(frame)
			}
		}
	}
}

func jitterPlaybackPump(ctx context.Context, jb *audio.JitterBuffer, playbackCh chan<- []float32, frameSize int, sampleRate, channels int, logger *slog.Logger, doneCh <-chan struct{}, muteFlag *atomic.Bool, recordingTap func([]float32), gainCtl *DeviceGainControl, agcProc *audio.AGCProcessor, readyCh chan<- struct{}) {
	defer close(playbackCh)
	// Ensure readyCh is always closed on exit so the player goroutine can
	// unblock even if the pump returns before reaching prefill (ctx cancel,
	// decoder track closed before any frame arrived). Uses a captured local
	// so we don't double-close after the successful prefill path.
	readyClosed := false
	closeReady := func() {
		if readyCh != nil && !readyClosed {
			close(readyCh)
			readyClosed = true
		}
	}
	defer closeReady()

	// Create a dedicated PLC decoder to generate concealment frames on underrun.
	plcDec, err := audio.NewOpusDecoder(sampleRate, channels)
	if err != nil {
		logger.Warn("Failed to create PLC decoder, falling back to silence", "error", err)
		plcDec = nil
	}
	silence := make([]float32, frameSize)

	// Wait for the first frame before starting the playback ticker.
	// This aligns the ticker phase with incoming packet timing.
	var firstFrame []float32
	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			return
		default:
			firstFrame = jb.Read()
			if firstFrame != nil {
				sendFrame := func(f []float32) bool {
					if recordingTap != nil {
						recordingTap(f)
					}
					applyPlaybackGainMute(ctx, f, gainCtl, muteFlag, agcProc)
					select {
					case playbackCh <- f:
						return true
					case <-ctx.Done():
						return false
					}
				}
				if !sendFrame(firstFrame) {
					return
				}
				// Prefill playbackCh with extra frames from the warmed-up JB.
				// Without this, pump→player equilibrium settles at 0-1 frames
				// (both run at 20ms cadence), so any pump scheduler/GC delay
				// starves the player → audible crackle. Seeding a few frames
				// moves equilibrium up, absorbing jitter.
				//
				// Prefill at most half of what's left in JB — this leaves the
				// other half for network jitter absorption, regardless of
				// mode (normal has minDepth=10, duplex/conference has 6).
				prefillLimit := jb.Depth() / 2
				if prefillLimit > 4 {
					prefillLimit = 4 // cap at 4 frames (80ms) to limit latency
				}
				for i := 0; i < prefillLimit; i++ {
					f := jb.Read()
					if f == nil {
						break
					}
					if !sendFrame(f) {
						return
					}
				}
				// Signal the player that playbackCh is pre-filled and it can
				// now open the hardware device.
				closeReady()
				goto pumpLoop
			}
			time.Sleep(2 * time.Millisecond)
		}
	}

pumpLoop:
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	const adaptInterval = 50 // every 50 ticks (1 second)
	const warmupTicks = 150  // first 3 seconds: aggressive adaptation
	var tickCount, underruns, totalTicks, playerBackpressure int

	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			// Decoder track ended — drain remaining frames then exit.
			for {
				frame := jb.Read()
				if frame == nil {
					return
				}
				if recordingTap != nil {
					recordingTap(frame)
				}
				applyPlaybackGainMute(ctx, frame, gainCtl, muteFlag, agcProc)
				select {
				case playbackCh <- frame:
				case <-ctx.Done():
					return
				}
			}
		case <-ticker.C:
			frame := jb.Read()
			if frame == nil {
				underruns++
				// Use PLC to generate a smooth continuation frame instead of silence.
				var plcFrame []float32
				if plcDec != nil {
					plcFrame, err = plcDec.DecodePLC()
					if err != nil {
						plcFrame = nil
					} else {
						plcCopy := make([]float32, len(plcFrame))
						copy(plcCopy, plcFrame)
						plcFrame = plcCopy
					}
				}
				if plcFrame == nil {
					plcFrame = silence
				}
				select {
				case playbackCh <- plcFrame:
				default:
				}
			} else {
				// Feed the real frame to PLC decoder to keep its state current.
				if plcDec != nil { //nolint:revive,staticcheck // intentionally empty — PLC state not updated for real frames
					// Re-encode is expensive; instead we just accept that PLC state
					// won't be perfect. The PLC decoder generates smooth fade-out
					// which is still much better than hard silence cuts.
				}
				// Recording tap receives the un-muted, pre-gain frame. The tap
				// must be synchronous (consume before return) and not retain
				// the slice, so no defensive copy is needed here.
				if recordingTap != nil {
					recordingTap(frame)
				}
				// Apply gain/mute in-place. Frame is a freshly-allocated
				// slice owned by this goroutine, so in-place mutation is safe.
				applyPlaybackGainMute(ctx, frame, gainCtl, muteFlag, agcProc)
				select {
				case playbackCh <- frame:
				default:
					// Player is not consuming fast enough — channel full.
					// This causes lost audio frames even though JB has data.
					playerBackpressure++
				}
			}

			tickCount++
			totalTicks++
			if tickCount >= adaptInterval {
				// During warmup: any underrun triggers buffer growth.
				// After warmup: adapt if underrun rate > 5%.
				isWarmup := totalTicks <= warmupTicks
				hasLoss := underruns > 0 && isWarmup || underruns > adaptInterval/20
				jb.AdaptiveAdjust(hasLoss)
				if underruns > 0 || playerBackpressure > 0 {
					// Surface as Info during warmup so users can correlate crackle
					// at startup with the root cause. After warmup, stay at Debug.
					lvl := logger.Debug
					if isWarmup {
						lvl = logger.Info
					}
					lvl("Audio pipeline stats",
						"jb_underruns", underruns,
						"player_drops", playerBackpressure,
						"jb_depth", jb.Depth(),
						"warmup", isWarmup,
					)
				}
				tickCount = 0
				underruns = 0
				playerBackpressure = 0
			}
		}
	}
}

// startAudioPlayer creates a player and starts playback from playbackCh.
// If the playback device has a MixInputID configured, a local microphone capture
// is started and mixed with the stream before writing to the output device.
// Sends any error to audioDone. Does nothing if deviceID is nil.
func startAudioPlayer(ctx context.Context, logger *slog.Logger, cfg config.Config, playbackCh <-chan []float32, audioDone chan<- error, readyCh <-chan struct{}) {
	startAudioPlayerWithStartup(ctx, logger, cfg, playbackCh, audioDone, readyCh, nil)
}

func startAudioPlayerWithStartup(ctx context.Context, logger *slog.Logger, cfg config.Config, playbackCh <-chan []float32, audioDone chan<- error, readyCh <-chan struct{}, startupCh chan<- error) {
	if cfg.DeviceID == nil {
		logger.Warn("No output device specified, audio will not be played")
		notifyAudioPlayerStartup(ctx, startupCh, errors.New("no output device specified"))
		return
	}

	// Check if the output device has a mix input configured.
	var mixInputID *uint32
	for _, d := range cfg.PlaybackDevices() {
		if d.ID == *cfg.DeviceID && d.MixInputID != nil {
			mixInputID = d.MixInputID
			break
		}
	}

	// If mix input is configured, capture from mic and mix with stream.
	actualPlaybackCh := playbackCh
	if mixInputID != nil {
		mixedCh := make(chan []float32, 4)
		go mixLocalInput(ctx, logger, cfg.SampleRate, cfg.Channels, *mixInputID, playbackCh, mixedCh)
		actualPlaybackCh = mixedCh
	}

	go runAudioPlayer(ctx, logger, cfg, actualPlaybackCh, audioDone, readyCh, startupCh)
}

func runAudioPlayer(ctx context.Context, logger *slog.Logger, cfg config.Config, playbackCh <-chan []float32, audioDone chan<- error, readyCh <-chan struct{}, startupCh chan<- error) {
	player, err := audio.NewPlayer(cfg.SampleRate, cfg.Channels)
	if err != nil {
		logger.Error("Player: NewPlayer failed", "error", err, "deviceID", *cfg.DeviceID)
		reportAudioPlayerStartupFailure(ctx, audioDone, startupCh, err)
		return
	}
	defer func() { _ = player.Close() }() //nolint:errcheck
	if err := waitAudioPlayerReady(ctx, logger, readyCh); err != nil {
		reportAudioPlayerStartupFailure(ctx, audioDone, startupCh, err)
		return
	}
	if err := player.Start(ctx, *cfg.DeviceID, playbackCh); err != nil {
		logger.Error("Player: Start failed — no audio output", "error", err, "deviceID", *cfg.DeviceID)
		reportAudioPlayerStartupFailure(ctx, audioDone, startupCh, err)
		return
	}
	notifyAudioPlayerStartup(ctx, startupCh, nil)
	logger.Info("Player started", "deviceID", *cfg.DeviceID, "sampleRate", cfg.SampleRate, "channels", cfg.Channels)
	go logPlayerSilenceFills(ctx, logger, player)
	<-ctx.Done()
	audioDone <- ctx.Err()
}

func waitAudioPlayerReady(ctx context.Context, logger *slog.Logger, readyCh <-chan struct{}) error {
	if readyCh == nil {
		return nil
	}
	select {
	case <-readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		logger.Warn("Player: pre-fill timeout, starting device anyway")
		return nil
	}
}

func reportAudioPlayerStartupFailure(ctx context.Context, audioDone chan<- error, startupCh chan<- error, err error) {
	notifyAudioPlayerStartup(ctx, startupCh, err)
	audioDone <- err
}

func notifyAudioPlayerStartup(ctx context.Context, startupCh chan<- error, err error) {
	if startupCh == nil {
		return
	}
	select {
	case startupCh <- err:
	case <-ctx.Done():
	}
}

func logPlayerSilenceFills(ctx context.Context, logger *slog.Logger, player interface{ SilenceFills() uint64 }) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var prev uint64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logPlayerSilenceDelta(logger, player, &prev)
		}
	}
}

func logPlayerSilenceDelta(logger *slog.Logger, player interface{ SilenceFills() uint64 }, prev *uint64) {
	cur := player.SilenceFills()
	delta := cur - *prev
	*prev = cur
	if delta > 0 {
		logger.Info("Player silence-fill events", "in_last_2s", delta, "total", cur)
	}
}

// mixLocalInput captures audio from a local input device and mixes it with the
// incoming stream. The mixed result is sent to mixedCh.
func mixLocalInput(ctx context.Context, logger *slog.Logger, sampleRate, channels uint32, inputDeviceID uint32, streamCh <-chan []float32, mixedCh chan<- []float32) {
	defer close(mixedCh)

	capturer, err := audio.NewCapturer(sampleRate, channels)
	if err != nil {
		logger.Warn("Mix input: failed to create capturer", "error", err)
		// Fallback: pass stream through unmodified.
		for frame := range streamCh {
			select {
			case mixedCh <- frame:
			case <-ctx.Done():
				return
			}
		}
		return
	}
	defer func() { _ = capturer.Close() }() //nolint:errcheck

	micCh := make(chan []float32, 8)
	if err := capturer.Start(ctx, inputDeviceID, micCh); err != nil {
		logger.Warn("Mix input: failed to start capture", "deviceID", inputDeviceID, "error", err)
		for frame := range streamCh {
			select {
			case mixedCh <- frame:
			case <-ctx.Done():
				return
			}
		}
		return
	}

	logger.Info("Mix input started", "inputDeviceID", inputDeviceID)

	// Accumulate mic samples into frames matching the stream frame size.
	var micBuf []float32

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-streamCh:
			if !ok {
				return
			}

			// Drain available mic samples into buffer.
			for {
				select {
				case mic := <-micCh:
					micBuf = append(micBuf, mic...)
				default:
					goto mix
				}
			}

		mix:
			// Mix: add mic samples to stream frame.
			mixed := make([]float32, len(frame))
			copy(mixed, frame)
			micLen := len(mixed)
			if micLen > len(micBuf) {
				micLen = len(micBuf)
			}
			for i := 0; i < micLen; i++ {
				mixed[i] += micBuf[i]
				// Soft clamp to [-1, 1].
				if mixed[i] > 1.0 {
					mixed[i] = 1.0
				} else if mixed[i] < -1.0 {
					mixed[i] = -1.0
				}
			}
			// Consume used mic samples.
			if micLen > 0 {
				micBuf = micBuf[micLen:]
			}

			select {
			case mixedCh <- mixed:
			case <-ctx.Done():
				return
			}
		}
	}
}

// startJitteredPlayback creates a JitterBuffer, wires decoder→JB→player pipeline.
// maxFrames is the upper limit for adaptive buffer growth. When muteFlag is
// non-nil, the pump substitutes silence for decoded frames whenever the flag
// is set — this is how Node.SetMuted (MuteController) drops incoming audio
// without tearing down the playback device. Pass nil for server-mode callers
// where there is no single "incoming stream" to mute.
func startJitteredPlayback(ctx context.Context, logger *slog.Logger, cfg config.Config, peer transport.PeerManager, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter, audioDone chan<- error, muteFlag *atomic.Bool, recordingTap func([]float32), gainCtl *DeviceGainControl, agcProc *audio.AGCProcessor) {
	targetFrames := cfg.EffectiveAudioBufferFrames()
	maxFrames := targetFrames * 3
	if maxFrames < 10 {
		maxFrames = 10
	}
	if maxFrames > 30 {
		maxFrames = 30
	}

	jb := audio.NewJitterBuffer(targetFrames, maxFrames)
	frameSize := int(cfg.SampleRate) / 50 * int(cfg.Channels) // 20ms frame

	// playbackCh bridges JitterBuffer → Player. Sized for 5 frames (100ms)
	// — gives the pump some slack to absorb brief scheduler/GC delays without
	// dropping frames on the non-blocking ticker send, while keeping latency
	// low. JitterBuffer remains the primary jitter absorber.
	playbackCh := make(chan []float32, 5)
	doneCh := make(chan struct{})
	// readyCh is closed by the pump after it has completed prefill (first
	// frame + a few extras from the warmed JB). The audio player waits on
	// this before opening the hardware device, so the very first malgo
	// callback sees a non-empty playbackCh and never has to emit silence.
	// This eliminates startup dropouts (~60 silence-fills in first 2s).
	readyCh := make(chan struct{})

	setupAudioDecoderWithJitter(logger, peer, cfg.SampleRate, cfg.Channels, jb, spectrum, level, doneCh)
	go jitterPlaybackPump(ctx, jb, playbackCh, frameSize, int(cfg.SampleRate), int(cfg.Channels), logger, doneCh, muteFlag, recordingTap, gainCtl, agcProc, readyCh)
	startAudioPlayer(ctx, logger, cfg, playbackCh, audioDone, readyCh)

	logger.Info("Jitter buffer enabled",
		"target", targetFrames,
		"max", maxFrames,
		"frameSize", frameSize,
	)
}

// handleCandidateMsg unmarshals an ICE candidate and adds it to the peer.
func handleCandidateMsg(logger *slog.Logger, payload json.RawMessage, peer transport.PeerManager, logFields ...interface{}) {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(payload, &candidate); err != nil {
		logger.Warn("Failed to parse ICE candidate", append(logFields, "error", err)...)
		return
	}
	if err := peer.AddICECandidate(candidate); err != nil {
		logger.Warn("Failed to add ICE candidate", append(logFields, "error", err)...)
	}
}

// sendControlViaTCP sends a control message through the TCP signaling channel.
func sendControlViaTCP(signaler transport.Signaler, action string) error {
	payload, _ := json.Marshal(struct { //nolint:errcheck
		Action string `json:"action"`
	}{Action: action})
	return signaler.Send(transport.SignalingMessage{Type: "control", Payload: payload})
}

// handleControlMsg unmarshals a control message and returns true if the action is "stop".
func handleControlMsg(logger *slog.Logger, payload json.RawMessage, logFields ...interface{}) bool {
	var ctrl struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(payload, &ctrl); err != nil {
		logger.Warn("Failed to parse control message", append(logFields, "error", err)...)
		return false
	}
	return ctrl.Action == "stop"
}

// buildAGCProcessors creates AGC processors for all capture devices so runtime
// Ctrl+D toggling can enable/disable AGC without restart. Initial enabled state
// matches DeviceEntry.AGC from setup. Returns nil only if the devices slice is
// empty (so callers can still check for absence of any processors).
func buildAGCProcessors(devices []config.DeviceEntry, sampleRate uint32) map[uint32]*audio.AGCProcessor {
	if len(devices) == 0 {
		return nil
	}
	result := make(map[uint32]*audio.AGCProcessor, len(devices))
	for _, dev := range devices {
		agcCfg := audio.DefaultAGCConfig()
		agcCfg.SampleRate = sampleRate
		proc := audio.NewAGCProcessor(agcCfg)
		proc.SetEnabled(dev.AGC) // honor setup choice; can be toggled at runtime
		result[dev.ID] = proc
	}
	return result
}
