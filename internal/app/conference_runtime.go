package app

import (
	"context"
	"math/rand/v2"

	"github.com/pion/rtp"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

var (
	_ echowarp.AudioRouteController = (*ServerApp)(nil)
	_ echowarp.AudioRouteController = (*ClientApp)(nil)
)

func (s *ServerApp) SetAudioRoute(ctx context.Context, rule echowarp.AudioRouteRule) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.cfg.Conference || s.conferenceRoom == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Conference is not active")
	}
	if err := s.conferenceRoom.SetRule(ConferenceServerID, true, rule); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConfigValidation, "Invalid conference route")
	}
	return nil
}

func (s *ServerApp) AudioRoutes(ctx context.Context) (echowarp.AudioRouteState, error) {
	if err := ctx.Err(); err != nil {
		return echowarp.AudioRouteState{}, err
	}
	if !s.cfg.Conference || s.conferenceRoom == nil {
		return echowarp.AudioRouteState{}, ewerrors.NewError(ewerrors.ErrNotRunning, "Conference is not active")
	}
	return s.conferenceRoom.State(ConferenceServerID, true), nil
}

func (c *ClientApp) currentConference() *conferenceClientSession {
	c.conferenceMu.RLock()
	defer c.conferenceMu.RUnlock()
	return c.conferenceSession
}

func (c *ClientApp) SetAudioRoute(ctx context.Context, rule echowarp.AudioRouteRule) error {
	if session := c.currentConference(); session != nil {
		return session.SetAudioRoute(ctx, rule)
	}
	return ewerrors.NewError(ewerrors.ErrNotRunning, "Conference is not active")
}

func (c *ClientApp) AudioRoutes(ctx context.Context) (echowarp.AudioRouteState, error) {
	if session := c.currentConference(); session != nil {
		return session.AudioRoutes(ctx)
	}
	return echowarp.AudioRouteState{}, ewerrors.NewError(ewerrors.ErrNotRunning, "Conference is not active")
}

func (s *ServerApp) setConferenceServerReceiveMute(clientID string, muted bool) {
	if s.conferenceRoom == nil || s.cfg.ServerMuted {
		return
	}
	if err := s.conferenceRoom.SetRule(clientID, false, echowarp.AudioRouteRule{
		Scope: conferenceReceive, Source: ConferenceServerID, Recipient: conferenceSelf, Muted: muted,
	}); err != nil {
		s.logger.Debug("Cannot change server source subscription", "error", err)
	}
}

func (c *ClientApp) setupConferenceClientPipeline(ctx context.Context, peer transport.PeerManager, audioDone chan error) error {
	session, err := newConferenceClientSession(ctx, peer, c.cfg, c.logger)
	if err != nil {
		return err
	}
	c.conferenceMu.Lock()
	c.conferenceSession = session
	c.conferenceMu.Unlock()
	session.recording = &c.RecordingMixin
	if err := c.setupSendAudioPipeline(session.ctx, peer, audioDone); err != nil {
		session.Close()
		return err
	}
	out := make(chan []float32, 5)
	initialGain := float32(1)
	devices := c.cfg.PlaybackDevices()
	if len(devices) > 0 {
		initialGain = float32(devices[0].Volume)
	}
	gain := NewDeviceGainControl(initialGain)
	agcMap := buildAGCProcessors(devices, c.cfg.SampleRate)
	var agc *audio.AGCProcessor
	if len(devices) > 0 {
		agc = agcMap[devices[0].ID]
	}
	go func() {
		err := session.Run(out, func(frame []float32) {
			applyPlaybackGainMute(session.ctx, frame, gain, &c.incomingMuted, agc)
			if c.spectrum != nil {
				c.spectrum.Feed(frame)
			}
			if c.levelMeter != nil {
				c.levelMeter.Feed(frame)
			}
		})
		select {
		case audioDone <- err:
		case <-ctx.Done():
		}
	}()
	playbackCfg := c.cfg
	if devices := c.cfg.PlaybackDevices(); len(devices) > 0 {
		id := devices[0].ID
		playbackCfg.DeviceID = &id
	} else if c.cfg.OutputDeviceID != nil {
		playbackCfg.DeviceID = c.cfg.OutputDeviceID
	}
	startAudioPlayer(session.ctx, c.logger, playbackCfg, out, audioDone, nil)
	return nil
}

// The ordinary capture pipeline owns one encoder. Its pooled output is borrowed
// by WriteSource (which clones fan-out packets) and returned exactly once here.
func (s *ServerApp) runServerConferenceCapture(ctx context.Context) {
	frames := make(chan []byte, 5)
	go func() {
		defer close(frames)
		if err := s.runCapturePipeline(ctx, frames); err != nil && ctx.Err() == nil {
			s.logger.Error("Conference server capture failed", "error", err)
			s.conferenceRoom.SetSourcePaused(ConferenceServerID, true)
		}
	}()
	forwardConferenceCapture(ctx, s.conferenceRoom, frames)
}

func forwardConferenceCapture(ctx context.Context, room *ConferenceRoom, frames <-chan []byte) {
	sequence := uint16(rand.Uint32())
	timestamp, ssrc := rand.Uint32(), rand.Uint32()
	for data := range frames {
		if ctx.Err() == nil {
			room.WriteSource(ConferenceServerID, &rtp.Packet{Header: rtp.Header{
				Version: 2, SequenceNumber: sequence, Timestamp: timestamp, SSRC: ssrc,
			}, Payload: data})
		}
		audio.PutOpusOutput(data)
		sequence++
		timestamp += 960 // Opus RTP clock is always 48 kHz, including mono/low-rate output.
	}
}
