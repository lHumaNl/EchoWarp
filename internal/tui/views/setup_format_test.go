package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact width unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world this is long", 12, "hello world…"},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"empty string", "", 10, ""},
		{"CJK wide chars", "你好世界测试", 9, "你好世界…"},
		{"single char width", "abcdef", 2, "a…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateToWidth(tt.input, tt.maxWidth)
			assert.Equal(t, tt.want, got)
			if tt.maxWidth > 0 && len(got) > 0 {
				assert.True(t, !strings.Contains(got, "…") || len(got) <= tt.maxWidth*4, "result should respect width")
			}
		})
	}
}
