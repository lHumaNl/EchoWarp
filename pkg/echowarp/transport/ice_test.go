package transport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICEConfig_DefaultSTUNServers(t *testing.T) {
	cfg := DefaultICEConfig()
	assert.NotEmpty(t, cfg.STUNServers)
	assert.Contains(t, cfg.STUNServers, "stun:stun.l.google.com:19302")
	assert.Contains(t, cfg.STUNServers, "stun:stun.cloudflare.com:3478")
}

func TestICEConfig_CustomSTUNServer(t *testing.T) {
	cfg := ICEConfig{
		STUNServers: []string{"stun:custom.example.com:3478"},
	}
	assert.Len(t, cfg.STUNServers, 1)
	assert.Equal(t, "stun:custom.example.com:3478", cfg.STUNServers[0])
}

func TestICEConfig_WithTURN(t *testing.T) {
	cfg := ICEConfig{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		TURNServers: []TURNServer{
			{
				URL:        "turn:turn.example.com:3478",
				Username:   "user",
				Credential: "pass",
			},
		},
	}
	assert.Len(t, cfg.TURNServers, 1)
	assert.Equal(t, "turn:turn.example.com:3478", cfg.TURNServers[0].URL)
}

func TestICEConfig_ToWebRTCConfig(t *testing.T) {
	cfg := ICEConfig{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		TURNServers: []TURNServer{
			{
				URL:        "turn:turn.example.com:3478",
				Username:   "user",
				Credential: "pass",
			},
		},
	}
	webrtcCfg := cfg.ToWebRTCConfig()
	assert.NotEmpty(t, webrtcCfg.ICEServers)
}
