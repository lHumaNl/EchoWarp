package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTone(amplitude float32, n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = amplitude
	}
	return s
}

func TestStreamPrioritizer_PrimarySpeaker(t *testing.T) {
	sp := NewStreamPrioritizer()

	sp.UpdateRMS("a", makeTone(0.5, 960))
	sp.UpdateRMS("b", makeTone(0.1, 960))

	assert.Equal(t, PriorityPrimary, sp.GetPriority("a"))
	assert.Equal(t, PrioritySecondary, sp.GetPriority("b"))
	assert.Equal(t, "a", sp.GetPrimaryID())
}

func TestStreamPrioritizer_SilentParticipant(t *testing.T) {
	sp := NewStreamPrioritizer()

	sp.UpdateRMS("a", makeTone(0.5, 960))
	sp.UpdateRMS("b", makeTone(0.0001, 960)) // very quiet

	assert.Equal(t, PriorityPrimary, sp.GetPriority("a"))
	assert.Equal(t, PrioritySilent, sp.GetPriority("b"))
}

func TestStreamPrioritizer_AllSilent(t *testing.T) {
	sp := NewStreamPrioritizer()

	sp.UpdateRMS("a", makeTone(0.0001, 960))
	sp.UpdateRMS("b", makeTone(0.0001, 960))

	assert.Equal(t, PrioritySilent, sp.GetPriority("a"))
	assert.Equal(t, PrioritySilent, sp.GetPriority("b"))
	assert.Equal(t, "", sp.GetPrimaryID())
}

func TestStreamPrioritizer_SpeakerSwitch(t *testing.T) {
	sp := NewStreamPrioritizer()

	// A is primary
	sp.UpdateRMS("a", makeTone(0.5, 960))
	sp.UpdateRMS("b", makeTone(0.1, 960))
	assert.Equal(t, "a", sp.GetPrimaryID())

	// B becomes louder
	sp.UpdateRMS("b", makeTone(0.8, 960))
	assert.Equal(t, "b", sp.GetPrimaryID())
	assert.Equal(t, PrioritySecondary, sp.GetPriority("a"))
}

func TestStreamPrioritizer_RemoveParticipant(t *testing.T) {
	sp := NewStreamPrioritizer()

	sp.UpdateRMS("a", makeTone(0.5, 960))
	sp.UpdateRMS("b", makeTone(0.3, 960))
	sp.RemoveParticipant("a")

	assert.Equal(t, PrioritySilent, sp.GetPriority("a")) // returns default
	assert.Equal(t, PriorityPrimary, sp.GetPriority("b"))
	assert.Equal(t, "b", sp.GetPrimaryID())
}

func TestStreamPrioritizer_GetAllPriorities(t *testing.T) {
	sp := NewStreamPrioritizer()

	sp.UpdateRMS("a", makeTone(0.5, 960))
	sp.UpdateRMS("b", makeTone(0.1, 960))

	all := sp.GetAllPriorities()
	assert.Len(t, all, 2)
	assert.Equal(t, PriorityPrimary, all["a"])
	assert.Equal(t, PrioritySecondary, all["b"])
}

func TestBitrateForPriority(t *testing.T) {
	assert.Equal(t, 64000, BitrateForPriority(64000, PriorityPrimary))
	assert.Equal(t, 32000, BitrateForPriority(64000, PrioritySecondary))
	assert.Equal(t, 6000, BitrateForPriority(64000, PrioritySilent))
}
