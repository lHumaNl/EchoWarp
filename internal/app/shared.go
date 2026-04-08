package app

import (
	"context"
	"encoding/json"
	"log/slog"
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
func decodeAudioStreamToJitter(logger *slog.Logger, inCh <-chan []byte, dec *audio.OpusDecoder, jb *audio.JitterBuffer, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter) {
	for data := range inCh {
		metrics.AudioBytesRecv.Add(float64(len(data)))
		pcm, err := dec.Decode(data)
		if err != nil {
			logger.Warn("Opus decode error", "error", err)
			continue
		}
		pcmCopy := make([]float32, len(pcm))
		copy(pcmCopy, pcm)
		if spectrum != nil {
			spectrum.Feed(pcmCopy)
		}
		if level != nil {
			level.Feed(pcmCopy)
		}
		jb.Write(pcmCopy)
	}
}

// jitterPlaybackPump reads frames from a JitterBuffer on a 20ms ticker and sends them
// to playbackCh for the audio player. When the JitterBuffer returns nil (underrun),
// PLC (Packet Loss Concealment) is used to generate a smooth continuation frame
// instead of silence. The ticker is started only after the first frame is available
// to synchronize playback phase with incoming packet timing.
// Periodically calls AdaptiveAdjust based on the observed underrun rate, with
// aggressive adaptation during the initial warmup period.
func jitterPlaybackPump(ctx context.Context, jb *audio.JitterBuffer, playbackCh chan<- []float32, frameSize int, sampleRate, channels int, logger *slog.Logger, doneCh <-chan struct{}) {
	defer close(playbackCh)

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
				select {
				case playbackCh <- firstFrame:
				case <-ctx.Done():
					return
				}
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
	var tickCount, underruns, totalTicks int

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
				select {
				case playbackCh <- frame:
				default:
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
				if underruns > 0 {
					logger.Debug("Jitter buffer adaptive adjust",
						"underruns", underruns,
						"depth", jb.Depth(),
						"warmup", isWarmup,
					)
				}
				tickCount = 0
				underruns = 0
			}
		}
	}
}

// startAudioPlayer creates a player and starts playback from playbackCh.
// If the playback device has a MixInputID configured, a local microphone capture
// is started and mixed with the stream before writing to the output device.
// Sends any error to audioDone. Does nothing if deviceID is nil.
func startAudioPlayer(ctx context.Context, logger *slog.Logger, cfg config.Config, playbackCh <-chan []float32, audioDone chan<- error) {
	if cfg.DeviceID == nil {
		logger.Warn("No output device specified, audio will not be played")
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

	go func() {
		player, err := audio.NewPlayer(cfg.SampleRate, cfg.Channels)
		if err != nil {
			audioDone <- err
			return
		}
		defer func() { _ = player.Close() }() //nolint:errcheck
		if err := player.Start(ctx, *cfg.DeviceID, actualPlaybackCh); err != nil {
			audioDone <- err
			return
		}
		<-ctx.Done()
		audioDone <- ctx.Err()
	}()
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
// maxFrames is the upper limit for adaptive buffer growth.
func startJitteredPlayback(ctx context.Context, logger *slog.Logger, cfg config.Config, peer transport.PeerManager, spectrum *audio.SpectrumAnalyzer, level *audio.LevelMeter, audioDone chan<- error) {
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

	// playbackCh bridges JitterBuffer → Player. Small buffer since JB handles timing.
	playbackCh := make(chan []float32, 2)
	doneCh := make(chan struct{})

	setupAudioDecoderWithJitter(logger, peer, cfg.SampleRate, cfg.Channels, jb, spectrum, level, doneCh)
	go jitterPlaybackPump(ctx, jb, playbackCh, frameSize, int(cfg.SampleRate), int(cfg.Channels), logger, doneCh)
	startAudioPlayer(ctx, logger, cfg, playbackCh, audioDone)

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

// buildAGCProcessors creates AGC processors for devices that have AGC enabled.
// Returns nil if no devices have AGC enabled.
func buildAGCProcessors(devices []config.DeviceEntry, sampleRate uint32) map[uint32]*audio.AGCProcessor {
	var result map[uint32]*audio.AGCProcessor
	for _, dev := range devices {
		if dev.AGC {
			if result == nil {
				result = make(map[uint32]*audio.AGCProcessor)
			}
			agcCfg := audio.DefaultAGCConfig()
			agcCfg.SampleRate = sampleRate
			result[dev.ID] = audio.NewAGCProcessor(agcCfg)
		}
	}
	return result
}
