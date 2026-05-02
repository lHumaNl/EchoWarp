package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

func TestNewTextField(t *testing.T) {
	f := NewTextField("server_address", "Server address", "192.168.1.1", true)
	assert.Equal(t, "server_address", f.Key)
	assert.Equal(t, "Server address", f.Label)
	assert.Equal(t, FieldText, f.Type)
	assert.Equal(t, "192.168.1.1", f.Value)
	assert.True(t, f.Required)
	assert.Equal(t, SourceDefault, f.Source)
	assert.True(t, f.IsValid())
}

func TestNewPasswordField(t *testing.T) {
	f := NewPasswordField("password", "Password", "secret")
	assert.Equal(t, FieldText, f.Type)
	assert.True(t, f.masked)
	assert.Equal(t, "secret", f.Value)
}

func TestNewNumberField(t *testing.T) {
	f := NewNumberField("port", "Port", 4415, 1, 65535)
	assert.Equal(t, FieldNumber, f.Type)
	assert.Equal(t, "4415", f.Value)
	assert.Equal(t, 1, f.MinVal)
	assert.Equal(t, 65535, f.MaxVal)
	assert.True(t, f.IsValid())
	assert.Equal(t, 4415, f.IntValue())
}

func TestNumberFieldValidation(t *testing.T) {
	f := NewNumberField("port", "Port", 4415, 1, 65535)

	f.SetValue("99999", SourceUser)
	assert.False(t, f.IsValid())

	f.SetValue("0", SourceUser)
	assert.False(t, f.IsValid())

	f.SetValue("8080", SourceUser)
	assert.True(t, f.IsValid())
	assert.Equal(t, 8080, f.IntValue())

	f.SetValue("abc", SourceUser)
	assert.False(t, f.IsValid())
}

func TestNewToggleField(t *testing.T) {
	f := NewToggleField("tls", "TLS", []string{"off", "on", "insecure"}, 0)
	assert.Equal(t, FieldToggle, f.Type)
	assert.Equal(t, "off", f.Value)
	assert.Equal(t, 0, f.OptionIndex())

	f.Toggle()
	assert.Equal(t, "on", f.Value)
	assert.Equal(t, SourceUser, f.Source)

	f.Toggle()
	assert.Equal(t, "insecure", f.Value)

	f.Toggle()
	assert.Equal(t, "off", f.Value)
}

func TestNewSelectField(t *testing.T) {
	f := NewSelectField("sample_rate", "Sample rate", []string{"48000", "24000", "16000", "8000"}, 1)
	assert.Equal(t, FieldSelect, f.Type)
	assert.Equal(t, "24000", f.Value)
	assert.Equal(t, 1, f.OptionIndex())
}

func TestFieldSetValue(t *testing.T) {
	f := NewTextField("addr", "Addr", "", true)
	assert.False(t, f.IsValid()) // required and empty

	f.SetValue("10.0.0.1", SourceCLI)
	assert.Equal(t, "10.0.0.1", f.Value)
	assert.Equal(t, SourceCLI, f.Source)
	assert.True(t, f.IsValid())
}

func TestToggleSetValueFindsCorrectIndex(t *testing.T) {
	f := NewToggleField("mode", "Mode", []string{"normal", "reverse"}, 0)
	f.SetValue("reverse", SourceCLI)
	assert.Equal(t, "reverse", f.Value)
	assert.Equal(t, 1, f.OptionIndex())
}

func TestActionField(t *testing.T) {
	f := NewActionField("advanced", "", "Advanced ▸")
	assert.Equal(t, FieldAction, f.Type)
	assert.Equal(t, "Advanced ▸", f.ActionLabel)
	assert.False(t, f.ActionExpanded)
	assert.True(t, f.IsValid())
}

func TestFieldEditing(t *testing.T) {
	f := NewTextField("port", "Port", "4415", false)
	assert.False(t, f.IsEditing())

	f.StartEditing()
	assert.True(t, f.IsEditing())

	f.CancelEditing()
	assert.False(t, f.IsEditing())
	assert.Equal(t, "4415", f.Value)
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"192.168.1.1", true},
		{"::1", true},
		{"example.com", true},
		{"my-host.local", true},
		{"", false},
		{"hello world", false}, // spaces
	}
	for _, tt := range tests {
		result := ValidateAddress(tt.input)
		assert.Equal(t, tt.valid, result.Valid, "input: %q", tt.input)
	}
}

func TestFieldRender(t *testing.T) {
	f := NewTextField("port", "Port", "4415", false)
	rendered := f.Render(false, 60)
	assert.Contains(t, rendered, "Port")
	assert.Contains(t, rendered, "4415")

	renderedFocused := f.Render(true, 60)
	assert.Contains(t, renderedFocused, styles.CursorGlyph)
}

func TestPasswordFieldRenderMasked(t *testing.T) {
	f := NewPasswordField("password", "Password", "secret")
	rendered := f.Render(false, 60)
	assert.Contains(t, rendered, "●●●●●●")
	assert.NotContains(t, rendered, "secret")
}
