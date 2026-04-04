package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderKickedView_WithReason(t *testing.T) {
	t.Parallel()
	result := RenderKickedView(KickedViewParams{
		Reason:         "spam",
		SelectedButton: 0,
		Width:          60,
		Height:         20,
	})
	assert.Contains(t, result, "Kicked by server")
	assert.Contains(t, result, "Reason: spam")
	assert.Contains(t, result, "[Reconnect]")
	assert.Contains(t, result, "[Settings]")
	assert.Contains(t, result, "[Quit]")
}

func TestRenderKickedView_EmptyReason(t *testing.T) {
	t.Parallel()
	result := RenderKickedView(KickedViewParams{
		Reason:         "",
		SelectedButton: 1,
		Width:          60,
		Height:         20,
	})
	assert.Contains(t, result, "Kicked by server")
	assert.NotContains(t, result, "Reason:")
}

func TestRenderKickedView_ButtonSelection(t *testing.T) {
	t.Parallel()
	// Each button index should produce valid output
	for i := 0; i < 3; i++ {
		result := RenderKickedView(KickedViewParams{
			Reason:         "test",
			SelectedButton: i,
			Width:          60,
			Height:         20,
		})
		assert.NotEmpty(t, result)
	}
}

func TestRenderBannedView_WithReasonAndCriteria(t *testing.T) {
	t.Parallel()
	result := RenderBannedView(BannedViewParams{
		Reason:         "toxic behavior",
		Criteria:       []string{"IP", "Nickname"},
		SelectedButton: 0,
		Width:          60,
		Height:         20,
	})
	assert.Contains(t, result, "Banned by server")
	assert.Contains(t, result, "Reason: toxic behavior")
	assert.Contains(t, result, "Criteria: IP, Nickname")
	assert.Contains(t, result, "[Settings]")
	assert.Contains(t, result, "[Quit]")
	// Banned screen should NOT have Reconnect button
	assert.NotContains(t, result, "[Reconnect]")
}

func TestRenderBannedView_EmptyReason(t *testing.T) {
	t.Parallel()
	result := RenderBannedView(BannedViewParams{
		Reason:         "",
		Criteria:       nil,
		SelectedButton: 0,
		Width:          60,
		Height:         20,
	})
	assert.Contains(t, result, "Banned by server")
	assert.NotContains(t, result, "Reason:")
	assert.NotContains(t, result, "Criteria:")
}

func TestRenderButtons(t *testing.T) {
	t.Parallel()
	result := renderButtons([]string{"A", "B", "C"}, 1)
	assert.True(t, strings.Contains(result, "[A]"))
	assert.True(t, strings.Contains(result, "[B]"))
	assert.True(t, strings.Contains(result, "[C]"))
}
