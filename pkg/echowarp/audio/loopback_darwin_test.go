//go:build darwin

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBestBlackHole_ExactMatch(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 2ch", Channels: 2},
		{Name: "BlackHole 16ch", Channels: 16},
	}
	bh := bestBlackHole(blackholes, 2)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 2ch", bh.Name)
}

func TestBestBlackHole_LargerAvailable(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 2ch", Channels: 2},
		{Name: "BlackHole 16ch", Channels: 16},
	}
	bh := bestBlackHole(blackholes, 16)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 16ch", bh.Name)
}

func TestBestBlackHole_FallbackToSmallest(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 16ch", Channels: 16},
		{Name: "BlackHole 64ch", Channels: 64},
	}
	bh := bestBlackHole(blackholes, 2)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 16ch", bh.Name)
}

func TestBestBlackHole_Empty(t *testing.T) {
	bh := bestBlackHole(nil, 2)
	assert.Nil(t, bh)
}
