package recent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartupLastModeJSONCompatibility(t *testing.T) {
	var server Server
	require.NoError(t, json.Unmarshal([]byte(`{"address":"localhost","port":4415,"nickname":"Legacy","last_mode":"duplex"}`), &server))
	assert.Equal(t, "duplex", server.LastMode)
	assert.Equal(t, "Legacy", server.Hostname)
	require.NoError(t, json.Unmarshal([]byte(`{"address":"localhost","port":4415}`), &server))
	assert.Empty(t, server.LastMode)
}

func TestStartupLastModeYAMLRoundTrip(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, Save([]Server{{Address: "localhost", Port: 4415, LastMode: "conference"}}))
	servers, err := Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "conference", servers[0].LastMode)
}
