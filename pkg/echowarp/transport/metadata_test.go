package transport

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeMessage_TrackInfo(t *testing.T) {
	original := TrackInfo{
		Title:    "Test Song",
		Artist:   "Test Artist",
		Album:    "Test Album",
		Duration: 180,
	}

	data, err := EncodeMessage(MetaMsgTrackInfo, original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var msg MetadataMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)

	var decoded TrackInfo
	err = json.Unmarshal(msg.Payload, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Title, decoded.Title)
	assert.Equal(t, original.Artist, decoded.Artist)
	assert.Equal(t, original.Album, decoded.Album)
	assert.Equal(t, original.Duration, decoded.Duration)
}

func TestEncodeMessage_VolumeControl(t *testing.T) {
	original := VolumeControl{
		Level: 0.75,
		Mute:  false,
	}

	data, err := EncodeMessage(MetaMsgVolumeCtrl, original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var msg MetadataMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)

	var decoded VolumeControl
	err = json.Unmarshal(msg.Payload, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Level, decoded.Level)
	assert.Equal(t, original.Mute, decoded.Mute)
}

func TestEncodeMessage_Chat(t *testing.T) {
	original := ChatMessage{
		Sender: "user1",
		Text:   "Hello, world!",
		Time:   1709395200,
	}

	data, err := EncodeMessage(MetaMsgChat, original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var msg MetadataMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)

	var decoded ChatMessage
	err = json.Unmarshal(msg.Payload, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Sender, decoded.Sender)
	assert.Equal(t, original.Text, decoded.Text)
	assert.Equal(t, original.Time, decoded.Time)
}

func TestEncodeMessage_StatsReport(t *testing.T) {
	original := StatsReport{
		PacketsLost: 42,
		Jitter:      0.05,
		RoundTrip:   0.12,
		Bitrate:     128,
	}

	data, err := EncodeMessage(MetaMsgStatsReport, original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var msg MetadataMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)

	var decoded StatsReport
	err = json.Unmarshal(msg.Payload, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.PacketsLost, decoded.PacketsLost)
	assert.Equal(t, original.Jitter, decoded.Jitter)
	assert.Equal(t, original.RoundTrip, decoded.RoundTrip)
	assert.Equal(t, original.Bitrate, decoded.Bitrate)
}

func TestMetadataHandler_HandleTrackInfo(t *testing.T) {
	handler := NewMetadataHandler()

	var received TrackInfo
	handler.SetOnTrackInfo(func(info TrackInfo) {
		received = info
	})

	data, err := EncodeMessage(MetaMsgTrackInfo, TrackInfo{
		Title:    "Test Song",
		Artist:   "Test Artist",
		Album:    "Test Album",
		Duration: 240,
	})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "Test Song", received.Title)
	assert.Equal(t, "Test Artist", received.Artist)
	assert.Equal(t, "Test Album", received.Album)
	assert.Equal(t, 240, received.Duration)
}

func TestMetadataHandler_HandleVolumeControl(t *testing.T) {
	handler := NewMetadataHandler()

	var received VolumeControl
	handler.SetOnVolumeControl(func(vc VolumeControl) {
		received = vc
	})

	data, err := EncodeMessage(MetaMsgVolumeCtrl, VolumeControl{
		Level: 0.5,
		Mute:  true,
	})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	require.NoError(t, err)

	assert.Equal(t, 0.5, received.Level)
	assert.True(t, received.Mute)
}

func TestMetadataHandler_HandleChat(t *testing.T) {
	handler := NewMetadataHandler()

	var received ChatMessage
	handler.SetOnChat(func(msg ChatMessage) {
		received = msg
	})

	data, err := EncodeMessage(MetaMsgChat, ChatMessage{
		Sender: "alice",
		Text:   "Hello from the other side",
		Time:   1709395300,
	})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "alice", received.Sender)
	assert.Equal(t, "Hello from the other side", received.Text)
	assert.Equal(t, int64(1709395300), received.Time)
}

func TestMetadataHandler_HandleStatsReport(t *testing.T) {
	handler := NewMetadataHandler()

	var received StatsReport
	handler.SetOnStatsReport(func(sr StatsReport) {
		received = sr
	})

	data, err := EncodeMessage(MetaMsgStatsReport, StatsReport{
		PacketsLost: 100,
		Jitter:      0.02,
		RoundTrip:   0.08,
		Bitrate:     256,
	})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	require.NoError(t, err)

	assert.Equal(t, uint32(100), received.PacketsLost)
	assert.Equal(t, 0.02, received.Jitter)
	assert.Equal(t, 0.08, received.RoundTrip)
	assert.Equal(t, 256, received.Bitrate)
}

func TestMetadataHandler_UnknownType_ReturnsError(t *testing.T) {
	handler := NewMetadataHandler()

	data, err := EncodeMessage("unknown_type", map[string]string{"foo": "bar"})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metadata message type")
}

func TestMetadataHandler_InvalidJSON_ReturnsError(t *testing.T) {
	handler := NewMetadataHandler()

	invalidData := []byte{0xff, 0xfe, 0xfd, 0xfc}

	err := handler.HandleMessage(invalidData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal metadata message")
}

func TestMetadataHandler_NoHandler_NoError(t *testing.T) {
	handler := NewMetadataHandler()

	data, err := EncodeMessage(MetaMsgTrackInfo, TrackInfo{
		Title:  "No Handler Song",
		Artist: "No Handler Artist",
	})
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	assert.NoError(t, err)
}

func TestEncodeMessage_RoundTrip(t *testing.T) {
	handler := NewMetadataHandler()

	var received ChatMessage
	handler.SetOnChat(func(msg ChatMessage) {
		received = msg
	})

	original := ChatMessage{
		Sender: "bob",
		Text:   "Round trip test message",
		Time:   1709395400,
	}

	data, err := EncodeMessage(MetaMsgChat, original)
	require.NoError(t, err)

	err = handler.HandleMessage(data)
	require.NoError(t, err)

	assert.Equal(t, original.Sender, received.Sender)
	assert.Equal(t, original.Text, received.Text)
	assert.Equal(t, original.Time, received.Time)
}

func TestMetadataHandler_HandlePing_NoError(t *testing.T) {
	handler := NewMetadataHandler()

	data := []byte(`{"type":"ping"}`)

	err := handler.HandleMessage(data)
	assert.NoError(t, err)
}

func TestMetadataHandler_ConcurrentAccess(t *testing.T) {
	handler := NewMetadataHandler()

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 5; i++ {
		go func(id int) {
			defer wg.Done()
			handler.SetOnChat(func(msg ChatMessage) {
			})
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			chatData := []byte(`{"type":"chat","payload":{"sender":"test","text":"hello","time":123456}}`)
			_ = handler.HandleMessage(chatData)
		}()
	}

	wg.Wait()
}

func TestMetadataHandler_ValidType_MalformedPayload(t *testing.T) {
	handler := NewMetadataHandler()

	data := []byte(`{"type":"track_info","payload":"not_an_object"}`)

	err := handler.HandleMessage(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal track info")
}

func TestEncodeMessage_NilPayload(t *testing.T) {
	data, err := EncodeMessage(MetaMsgPing, nil)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var msg MetadataMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)

	assert.Equal(t, MetaMsgPing, msg.Type)
	assert.True(t, msg.Payload == nil || len(msg.Payload) == 0)
}

func TestMetadataHandler_SetOnPong(t *testing.T) {
	handler := NewMetadataHandler()

	called := false
	handler.SetOnPong(func() {
		called = true
	})

	data := []byte(`{"type":"pong"}`)
	err := handler.HandleMessage(data)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMetadataHandler_SetOnPing(t *testing.T) {
	handler := NewMetadataHandler()

	called := false
	handler.SetOnPing(func() {
		called = true
	})

	data := []byte(`{"type":"ping"}`)
	err := handler.HandleMessage(data)
	require.NoError(t, err)
	assert.True(t, called)
}
