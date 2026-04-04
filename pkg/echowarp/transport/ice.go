package transport

import "github.com/pion/webrtc/v4"

// DefaultICEConfig returns an ICE configuration with public STUN servers.
// Uses Google and Cloudflare STUN servers for NAT traversal.
func DefaultICEConfig() ICEConfig {
	return ICEConfig{
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun.cloudflare.com:3478",
		},
	}
}

// ToWebRTCConfig converts ICEConfig to pion/webrtc Configuration.
func (c ICEConfig) ToWebRTCConfig() webrtc.Configuration {
	iceServers := make([]webrtc.ICEServer, 0)

	if len(c.STUNServers) > 0 {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: c.STUNServers,
		})
	}

	for _, turn := range c.TURNServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{turn.URL},
			Username:   turn.Username,
			Credential: turn.Credential,
		})
	}

	return webrtc.Configuration{
		ICEServers: iceServers,
	}
}
