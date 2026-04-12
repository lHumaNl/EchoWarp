package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestSources() (capture, playback, clients []RecordingSource) {
	capture = []RecordingSource{
		{ID: "cap-1", Name: "Mic 1"},
		{ID: "cap-2", Name: "Mic 2"},
	}
	playback = []RecordingSource{
		{ID: "play-1", Name: "Speaker"},
	}
	clients = []RecordingSource{
		{ID: "client-a", Name: "Alice"},
		{ID: "client-b", Name: "Bob"},
		{ID: "client-c", Name: "Carol"},
	}
	return
}

func TestRecordingOverlay_ShowDefaults(t *testing.T) {
	o := NewRecordingOverlay()
	cap, play, cli := makeTestSources()
	o.ShowStart(cap, play, cli)

	assert.True(t, o.Visible)
	assert.True(t, o.IsStartState())

	// All sources should be selected by default.
	for _, d := range o.CaptureDevices {
		assert.True(t, d.Selected, "capture %s should be selected", d.ID)
	}
	for _, d := range o.PlaybackDevices {
		assert.True(t, d.Selected, "playback %s should be selected", d.ID)
	}
	for _, c := range o.Clients {
		assert.True(t, c.Selected, "client %s should be selected", c.ID)
	}

	// Cursor should be at first interactive item (not a section header).
	require.Greater(t, len(o.items), 0)
	assert.NotEqual(t, itemSectionHeader, o.items[o.cursor].kind)
}

func TestRecordingOverlay_Navigation(t *testing.T) {
	o := NewRecordingOverlay()
	cap, play, cli := makeTestSources()
	o.ShowStart(cap, play, cli)

	startCursor := o.cursor

	// Move down — cursor should advance past section headers.
	o.Down()
	assert.Greater(t, o.cursor, startCursor)

	// Move up — should go back.
	prev := o.cursor
	o.Up()
	assert.Less(t, o.cursor, prev)
}

func TestRecordingOverlay_Toggle(t *testing.T) {
	o := NewRecordingOverlay()
	cap, _, cli := makeTestSources()
	// No playback, >1 clients — shows capture + clients + mode.
	o.ShowStart(cap, nil, cli)

	// Find first capture item and navigate there.
	for i, it := range o.items {
		if it.kind == itemCapture {
			o.cursor = i
			break
		}
	}

	idx := o.items[o.cursor].sourceIdx
	assert.True(t, o.CaptureDevices[idx].Selected)

	o.Toggle()
	assert.False(t, o.CaptureDevices[idx].Selected)

	o.Toggle()
	assert.True(t, o.CaptureDevices[idx].Selected)
}

func TestRecordingOverlay_SelectAllNone(t *testing.T) {
	o := NewRecordingOverlay()
	cap, _, cli := makeTestSources()
	o.ShowStart(cap, nil, cli)

	// Find the clientAll item and toggle it off.
	for i, it := range o.items {
		if it.kind == itemClientAll {
			o.cursor = i
			break
		}
	}

	// Initially all selected, toggle = deselect all.
	o.Toggle()
	for _, c := range o.Clients {
		assert.False(t, c.Selected, "client %s should be deselected", c.ID)
	}

	// Toggle again = select all.
	o.Toggle()
	for _, c := range o.Clients {
		assert.True(t, c.Selected, "client %s should be selected", c.ID)
	}
}

func TestRecordingOverlay_SelectedMode(t *testing.T) {
	o := NewRecordingOverlay()
	cap, _, cli := makeTestSources()
	o.ShowStart(cap, nil, cli)

	assert.Equal(t, RecordMix, o.SelectedMode())

	// Navigate to mode items.
	for i, it := range o.items {
		if it.kind == itemModeTracks {
			o.cursor = i
			break
		}
	}
	o.Toggle()
	assert.Equal(t, RecordTracks, o.SelectedMode())
}

func TestRecordingOverlay_SelectedSources(t *testing.T) {
	o := NewRecordingOverlay()
	cap, _, cli := makeTestSources()
	o.ShowStart(cap, nil, cli)

	ids := o.SelectedLocalDevices()
	assert.Equal(t, []string{"cap-1", "cap-2"}, ids)

	remote := o.SelectedRemoteSources()
	assert.Equal(t, []string{"client-a", "client-b", "client-c"}, remote)
}

func TestRecordingOverlay_Render(t *testing.T) {
	o := NewRecordingOverlay()
	cap, _, cli := makeTestSources()
	o.ShowStart(cap, nil, cli)

	output := o.Render(80, 40)
	assert.NotEmpty(t, output)
}
