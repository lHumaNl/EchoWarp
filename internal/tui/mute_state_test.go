package tui

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMuteState(t *testing.T) {
	ms := NewMuteState()
	require.NotNil(t, ms)
	assert.False(t, ms.IsMuteAll())
	assert.Empty(t, ms.MutedParticipants())
}

func TestToggleMute(t *testing.T) {
	ms := NewMuteState()

	// First toggle: mute
	assert.True(t, ms.ToggleMute("alice"))
	assert.True(t, ms.IsMuted("alice"))
	assert.False(t, ms.IsMuted("bob"))

	// Second toggle: unmute
	assert.False(t, ms.ToggleMute("alice"))
	assert.False(t, ms.IsMuted("alice"))
}

func TestToggleMuteAll(t *testing.T) {
	ms := NewMuteState()
	ids := []string{"alice", "bob", "charlie"}

	// Enable mute-all
	assert.True(t, ms.ToggleMuteAll(ids))
	assert.True(t, ms.IsMuteAll())
	assert.True(t, ms.IsMuted("alice"))
	assert.True(t, ms.IsMuted("bob"))
	assert.True(t, ms.IsMuted("charlie"))

	// Disable mute-all: all should be unmuted (no one was muted before)
	assert.False(t, ms.ToggleMuteAll(ids))
	assert.False(t, ms.IsMuteAll())
	assert.False(t, ms.IsMuted("alice"))
	assert.False(t, ms.IsMuted("bob"))
	assert.False(t, ms.IsMuted("charlie"))
}

func TestToggleMuteAllPreservesPreState(t *testing.T) {
	ms := NewMuteState()
	ids := []string{"alice", "bob", "charlie"}

	// Pre-mute alice
	ms.ToggleMute("alice")
	assert.True(t, ms.IsMuted("alice"))

	// Enable mute-all
	ms.ToggleMuteAll(ids)
	assert.True(t, ms.IsMuteAll())
	assert.True(t, ms.IsMuted("alice"))
	assert.True(t, ms.IsMuted("bob"))

	// Disable mute-all: alice should still be muted (was muted before)
	ms.ToggleMuteAll(ids)
	assert.False(t, ms.IsMuteAll())
	assert.True(t, ms.IsMuted("alice"))
	assert.False(t, ms.IsMuted("bob"))
	assert.False(t, ms.IsMuted("charlie"))
}

func TestMuteAllThenIndividualUnmute(t *testing.T) {
	ms := NewMuteState()
	ids := []string{"alice", "bob", "charlie"}

	// Mute all
	ms.ToggleMuteAll(ids)
	assert.True(t, ms.IsMuteAll())

	// Unmute bob individually — should clear mute-all flag
	muted := ms.ToggleMute("bob")
	assert.False(t, muted, "bob should be unmuted after individual toggle")
	assert.False(t, ms.IsMuteAll(), "mute-all flag should be cleared")
	assert.True(t, ms.IsMuted("alice"), "alice should still be muted")
	assert.True(t, ms.IsMuted("charlie"), "charlie should still be muted")

	// Re-enable mute-all
	ms.ToggleMuteAll(ids)
	assert.True(t, ms.IsMuteAll())
	assert.True(t, ms.IsMuted("bob"), "bob should be muted again")
}

func TestMutedParticipants(t *testing.T) {
	ms := NewMuteState()
	ms.ToggleMute("alice")
	ms.ToggleMute("charlie")

	mutedList := ms.MutedParticipants()
	sort.Strings(mutedList)
	assert.Equal(t, []string{"alice", "charlie"}, mutedList)
}

func TestMutedMap(t *testing.T) {
	ms := NewMuteState()
	ms.ToggleMute("alice")

	m := ms.MutedMap()
	assert.True(t, m["alice"])
	assert.False(t, m["bob"])

	// Ensure it's a copy
	m["alice"] = false
	assert.True(t, ms.IsMuted("alice"))
}

func TestRemoveParticipant(t *testing.T) {
	ms := NewMuteState()
	ms.ToggleMute("alice")
	assert.True(t, ms.IsMuted("alice"))

	ms.RemoveParticipant("alice")
	assert.False(t, ms.IsMuted("alice"))
}

func TestRemoveParticipantFromPreMuteAllState(t *testing.T) {
	ms := NewMuteState()
	ms.ToggleMute("alice")

	// Enable mute-all to save pre-state
	ms.ToggleMuteAll([]string{"alice", "bob"})
	assert.True(t, ms.IsMuteAll())

	// Remove alice while mute-all is active
	ms.RemoveParticipant("alice")

	// Disable mute-all — alice should not reappear
	ms.ToggleMuteAll([]string{"bob"})
	assert.False(t, ms.IsMuted("alice"))
	assert.False(t, ms.IsMuted("bob"))
}
