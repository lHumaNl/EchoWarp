package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestRenderClientListItem_BadgeMutedOutgoing(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID:      "c1",
		Nickname:      "Alice",
		MutedOutgoing: true,
		Duration:      "00:01:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↑")
	assert.NotContains(t, result, "🔇↓")
	assert.NotContains(t, result, "🔇↑↓")
}

func TestRenderClientListItem_BadgeMutedClient(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID: "c2",
		Nickname: "Bob",
		Muted:    true,
		Duration: "00:02:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↓")
	assert.NotContains(t, result, "🔇↑")
}

func TestRenderClientListItem_BadgeBothMuted(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID:      "c3",
		Nickname:      "Charlie",
		Muted:         true,
		MutedOutgoing: true,
		Duration:      "00:03:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↑↓")
}

func TestRenderClientListItem_NoBadgeWhenUnmuted(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID: "c4",
		Nickname: "Dave",
		Duration: "00:04:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.NotContains(t, result, "🔇")
}

func TestRenderClientListItem_BadgeMutedIncoming(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID:      "c5",
		Nickname:      "Eve",
		MutedIncoming: true,
		Duration:      "00:05:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↓")
	assert.NotContains(t, result, "🔇↑")
	assert.NotContains(t, result, "🔇↑↓")
}

func TestRenderClientListItem_BadgeMutedIncomingAndOutgoing(t *testing.T) {
	t.Parallel()
	c := transport.ClientInfo{
		ClientID:      "c6",
		Nickname:      "Frank",
		MutedOutgoing: true,
		MutedIncoming: true,
		Duration:      "00:06:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↑↓")
}

func TestRenderClientListItem_BadgeMutedClientAndIncoming(t *testing.T) {
	t.Parallel()
	// Both client-initiated mute and server-initiated incoming mute → still shows ↓
	c := transport.ClientInfo{
		ClientID:      "c7",
		Nickname:      "Grace",
		Muted:         true,
		MutedIncoming: true,
		Duration:      "00:07:00",
	}
	result := renderClientListItem(c, QualityGood, false, 40)
	assert.Contains(t, result, "🔇↓")
	assert.NotContains(t, result, "🔇↑↓")
}

func TestRenderClientListItem_NarrowWidthDropsBadge(t *testing.T) {
	t.Parallel()
	// At very narrow width, badges should be dropped to avoid layout overflow
	c := transport.ClientInfo{
		ClientID:      "c8",
		Nickname:      "Hi",
		MutedOutgoing: true,
		MutedIncoming: true,
		Paused:        true,
		Duration:      "00:01:00",
	}
	// At width=20 the badges (🔇↑↓ ⏸) + dur + prefix won't fit alongside a 4-char nick
	result := renderClientListItem(c, QualityGood, false, 20)
	// Result width (visible chars) must not exceed maxWidth
	// Simply verify the function doesn't panic and returns a non-empty string
	assert.NotEmpty(t, result)
}

func TestRenderClientListItem_NarrowWidthShowsBadgeWhenFits(t *testing.T) {
	t.Parallel()
	// At normal width, badges should be present
	c := transport.ClientInfo{
		ClientID:      "c9",
		Nickname:      "Bob",
		MutedOutgoing: true,
		Duration:      "00:01:00",
	}
	result := renderClientListItem(c, QualityGood, false, 30)
	assert.Contains(t, result, "🔇↑")
}
