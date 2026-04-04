package transport

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPauseActionConstants(t *testing.T) {
	assert.Equal(t, "pause", ActionPause)
	assert.Equal(t, "resume", ActionResume)
	assert.Equal(t, "pause_all", ActionPauseAll)
	assert.Equal(t, "resume_all", ActionResumeAll)
}

func TestPauseControlMessageFormat(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		deviceID string
	}{
		{"pause device", ActionPause, "mic-1"},
		{"resume device", ActionResume, "mic-1"},
		{"pause all", ActionPauseAll, ""},
		{"resume all", ActionResumeAll, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"action": tt.action,
			}
			if tt.deviceID != "" {
				payload["data"] = map[string]string{"deviceID": tt.deviceID}
			}

			outer := map[string]interface{}{
				"type":    TypeControl,
				"payload": payload,
			}

			data, err := json.Marshal(outer)
			require.NoError(t, err)

			var parsed struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			require.NoError(t, json.Unmarshal(data, &parsed))
			assert.Equal(t, TypeControl, parsed.Type)

			var ctrl struct {
				Action string `json:"action"`
				Data   *struct {
					DeviceID string `json:"deviceID"`
				} `json:"data,omitempty"`
			}
			require.NoError(t, json.Unmarshal(parsed.Payload, &ctrl))
			assert.Equal(t, tt.action, ctrl.Action)
			if tt.deviceID != "" {
				require.NotNil(t, ctrl.Data)
				assert.Equal(t, tt.deviceID, ctrl.Data.DeviceID)
			}
		})
	}
}
