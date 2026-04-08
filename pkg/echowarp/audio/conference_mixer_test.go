package audio

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFrameSize = 480

func newTestMixer() *ConferenceMixer {
	return NewConferenceMixer(testFrameSize, 48000)
}

// addParticipantNoAGC adds a participant and disables AGC so tests can assert exact sample values.
func addParticipantNoAGC(cm *ConferenceMixer, id string) {
	cm.AddParticipant(id)
	cm.SetParticipantAGC(id, false)
}

func makeFrame(val float32) []float32 {
	f := make([]float32, testFrameSize)
	for i := range f {
		f[i] = val
	}
	return f
}

func TestConferenceMixer_AddRemoveParticipant(t *testing.T) {
	cm := newTestMixer()

	cm.AddParticipant("alice")
	cm.AddParticipant("bob")
	states := cm.GetParticipantStates()
	assert.Len(t, states, 2)

	cm.RemoveParticipant("alice")
	states = cm.GetParticipantStates()
	assert.Len(t, states, 1)
	assert.Equal(t, "bob", states[0].ID)

	// Removing non-existent is a no-op
	cm.RemoveParticipant("charlie")
	assert.Len(t, cm.GetParticipantStates(), 1)

	// Adding duplicate is a no-op
	cm.AddParticipant("bob")
	assert.Len(t, cm.GetParticipantStates(), 1)
}

func TestConferenceMixer_PersonalMixExcludesSelf(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "alice")
	addParticipantNoAGC(cm, "bob")

	aliceAudio := makeFrame(0.3)
	bobAudio := makeFrame(0.5)

	cm.SubmitAudio("alice", aliceAudio)
	cm.SubmitAudio("bob", bobAudio)

	// Alice's personal mix should contain only Bob's audio
	aliceMix := cm.GetPersonalMix("alice")
	require.NotNil(t, aliceMix)
	require.Len(t, aliceMix, testFrameSize)
	for i := range aliceMix {
		assert.InDelta(t, 0.5, aliceMix[i], 0.05, "alice mix sample %d (tanh clipped)", i)
	}

	// Bob's personal mix should contain only Alice's audio
	bobMix := cm.GetPersonalMix("bob")
	require.NotNil(t, bobMix)
	for i := range bobMix {
		assert.InDelta(t, 0.3, bobMix[i], 0.01, "bob mix sample %d (tanh clipped)", i)
	}
}

func TestConferenceMixer_MutedParticipantNotInMix(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "alice")
	addParticipantNoAGC(cm, "bob")

	cm.SetParticipantMuted("bob", true)

	// Submit audio AFTER muting so that SubmitAudio sees the mute flag
	cm.SubmitAudio("alice", makeFrame(0.3))
	cm.SubmitAudio("bob", makeFrame(0.5))

	// Alice should hear silence — Bob is muted
	aliceMix := cm.GetPersonalMix("alice")
	require.NotNil(t, aliceMix)
	for i := range aliceMix {
		assert.InDelta(t, 0.0, aliceMix[i], 1e-5, "alice should hear silence when bob muted, sample %d", i)
	}

	// Bob should still hear Alice (Bob being muted doesn't affect what Bob hears)
	bobMix := cm.GetPersonalMix("bob")
	require.NotNil(t, bobMix)
	for i := range bobMix {
		assert.InDelta(t, 0.3, bobMix[i], 0.01, "bob should hear alice, sample %d (tanh clipped)", i)
	}
}

func TestConferenceMixer_VolumeScaling(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "alice")
	addParticipantNoAGC(cm, "bob")

	cm.SetParticipantVolume("bob", 0.5)

	cm.SubmitAudio("alice", makeFrame(0.4))
	cm.SubmitAudio("bob", makeFrame(0.4))

	// Alice hears Bob at half volume
	aliceMix := cm.GetPersonalMix("alice")
	require.NotNil(t, aliceMix)
	for i := range aliceMix {
		assert.InDelta(t, 0.2, aliceMix[i], 0.01, "alice should hear bob at 0.5 volume, sample %d (tanh clipped)", i)
	}
}

func TestConferenceMixer_SpeakingDetection(t *testing.T) {
	cm := newTestMixer()
	cm.AddParticipant("alice")
	cm.AddParticipant("bob")

	// Alice speaks loudly
	cm.SubmitAudio("alice", makeFrame(0.5))
	// Bob is silent
	cm.SubmitAudio("bob", makeFrame(0.0))

	states := cm.GetParticipantStates()
	stateMap := make(map[string]ParticipantState)
	for _, s := range states {
		stateMap[s.ID] = s
	}

	assert.True(t, stateMap["alice"].Speaking, "alice should be detected as speaking")
	assert.False(t, stateMap["bob"].Speaking, "bob should not be detected as speaking")
}

func TestConferenceMixer_EmptyMixWhenAlone(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "alice")

	cm.SubmitAudio("alice", makeFrame(0.7))

	mix := cm.GetPersonalMix("alice")
	require.NotNil(t, mix)
	for i := range mix {
		assert.InDelta(t, 0.0, mix[i], 1e-5, "alone participant should hear silence, sample %d", i)
	}
}

func TestConferenceMixer_GetPersonalMix_UnknownParticipant(t *testing.T) {
	cm := newTestMixer()
	assert.Nil(t, cm.GetPersonalMix("nobody"))
}

func BenchmarkConferenceMixer_10Participants(b *testing.B) {
	const n = 10
	cm := NewConferenceMixer(960, 48000) // 20ms stereo-equivalent
	ids := make([]string, n)
	frames := make([][]float32, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("user-%d", i)
		cm.AddParticipant(ids[i])
		frames[i] = make([]float32, 960)
		for j := range frames[i] {
			frames[i][j] = float32(i+1) * 0.05
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			cm.SubmitAudio(ids[j], frames[j])
		}
		for j := 0; j < n; j++ {
			mix := cm.GetPersonalMix(ids[j])
			if mix != nil {
				cm.ReturnBuffer(mix)
			}
		}
	}
}

func BenchmarkConferenceMixer_SubmitOnly_10(b *testing.B) {
	const n = 10
	cm := NewConferenceMixer(960, 48000)
	ids := make([]string, n)
	frames := make([][]float32, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("user-%d", i)
		cm.AddParticipant(ids[i])
		frames[i] = make([]float32, 960)
		for j := range frames[i] {
			frames[i][j] = 0.3
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			cm.SubmitAudio(ids[j], frames[j])
		}
	}
}

func TestConferenceMixer_DoubleGetPersonalMix(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "A")
	addParticipantNoAGC(cm, "B")

	cm.SubmitAudio("A", makeFrame(0.5))
	cm.SubmitAudio("B", makeFrame(0.3))

	// First call should contain B's audio for A's personal mix.
	mix1 := cm.GetPersonalMix("A")
	require.NotNil(t, mix1)
	assert.InDelta(t, float64(0.3), float64(mix1[0]), 0.05, "first mix should contain B's audio")
	cm.ReturnBuffer(mix1)

	// Second call without new SubmitAudio — should be silent (dirty cleared).
	mix2 := cm.GetPersonalMix("A")
	require.NotNil(t, mix2)
	for i := range mix2 {
		assert.Equal(t, float32(0), mix2[i], "second mix should be zero (no new audio submitted)")
	}
	cm.ReturnBuffer(mix2)
}

func TestConferenceMixer_SingleParticipant_SilentMix(t *testing.T) {
	cm := newTestMixer()
	addParticipantNoAGC(cm, "A")

	cm.SubmitAudio("A", makeFrame(0.5))

	// A's personal mix excludes A's own audio; no other participants → silence.
	mix := cm.GetPersonalMix("A")
	require.NotNil(t, mix)
	for i := range mix {
		assert.Equal(t, float32(0), mix[i], "single participant should get silent personal mix")
	}
	cm.ReturnBuffer(mix)
}
