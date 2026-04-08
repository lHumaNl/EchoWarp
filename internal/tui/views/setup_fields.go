// Package views provides setup screen field components for the TUI.
package views

import (
	"net"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// FieldSource indicates where the field value originated.
type FieldSource int

const (
	SourceDefault FieldSource = iota // default value, not changed
	SourceCLI                        // from CLI flag or config file
	SourceUser                       // changed by user in TUI
	SourceAuto                       // auto-filled from mDNS discovery
	SourceConfig                     // loaded from saved config file
)

// FieldType identifies the kind of setup field.
type FieldType int

const (
	FieldText   FieldType = iota // free-form text input
	FieldNumber                  // numeric input with range validation
	FieldToggle                  // cycles through predefined values
	FieldSelect                  // opens mini-overlay to choose from list
	FieldAction                  // clickable action (e.g. [Advanced ▸])
)

// ValidationResult holds the outcome of field validation.
type ValidationResult struct {
	Valid   bool
	Message string // hint message shown on error
}

// SetupField represents a single configurable parameter in the setup screen.
type SetupField struct {
	Key      string // stable, language-independent identifier (e.g. "port", "tls")
	Label    string
	Type     FieldType
	Source   FieldSource
	Required bool

	// Value state
	Value    string   // current display value
	Options  []string // for Toggle/Select
	optIndex int      // current option index for Toggle/Select

	// Number constraints
	MinVal int
	MaxVal int

	// Text input (for editing mode)
	textInput textinput.Model
	editing   bool
	masked    bool // password field

	// Validation
	validator  func(string) ValidationResult
	lastResult ValidationResult

	// Hint shown next to the field (e.g. "⚠ required when TLS is on")
	Hint string

	// Action callback label
	ActionLabel    string // e.g. "Advanced ▸"
	ActionExpanded bool   // for Advanced toggle

	// Hidden fields are skipped during rendering and navigation.
	Hidden bool
}

// NewTextField creates a free-form text input field.
func NewTextField(key, label, value string, required bool) SetupField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 256
	f := SetupField{
		Key:       key,
		Label:     label,
		Type:      FieldText,
		Source:    SourceDefault,
		Value:     value,
		Required:  required,
		textInput: ti,
	}
	f.validate()
	return f
}

// NewPasswordField creates a masked text input field.
func NewPasswordField(key, label, value string) SetupField {
	f := NewTextField(key, label, value, false)
	f.masked = true
	return f
}

// NewNumberField creates a numeric input field with min/max validation.
func NewNumberField(key, label string, value, minVal, maxVal int) SetupField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 10
	f := SetupField{
		Key:        key,
		Label:      label,
		Type:       FieldNumber,
		Source:     SourceDefault,
		Value:      strconv.Itoa(value),
		MinVal:     minVal,
		MaxVal:     maxVal,
		textInput:  ti,
		lastResult: ValidationResult{Valid: true},
	}
	f.validator = func(s string) ValidationResult {
		if s == "" {
			return ValidationResult{Valid: false, Message: i18n.T("validation_required")}
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return ValidationResult{Valid: false, Message: i18n.T("validation_must_be_number")}
		}
		if n < f.MinVal || n > f.MaxVal {
			return ValidationResult{Valid: false, Message: i18n.Tf("validation_range", f.MinVal, f.MaxVal)}
		}
		return ValidationResult{Valid: true}
	}
	return f
}

// NewToggleField creates a field that cycles through options on Enter/Space.
func NewToggleField(key, label string, options []string, initial int) SetupField {
	if initial >= len(options) {
		initial = 0
	}
	return SetupField{
		Key:        key,
		Label:      label,
		Type:       FieldToggle,
		Source:     SourceDefault,
		Options:    options,
		optIndex:   initial,
		Value:      options[initial],
		lastResult: ValidationResult{Valid: true},
	}
}

// NewSelectField creates a field that opens a mini-overlay to choose a value.
func NewSelectField(key, label string, options []string, initial int) SetupField {
	if initial >= len(options) {
		initial = 0
	}
	return SetupField{
		Key:        key,
		Label:      label,
		Type:       FieldSelect,
		Source:     SourceDefault,
		Options:    options,
		optIndex:   initial,
		Value:      options[initial],
		lastResult: ValidationResult{Valid: true},
	}
}

// NewActionField creates a clickable action item (e.g. [Advanced ▸]).
func NewActionField(key, label, actionLabel string) SetupField {
	return SetupField{
		Key:         key,
		Label:       label,
		Type:        FieldAction,
		ActionLabel: actionLabel,
		lastResult:  ValidationResult{Valid: true},
	}
}

// SetValue updates the field value and marks it with the given source.
func (f *SetupField) SetValue(val string, source FieldSource) {
	f.Value = val
	f.Source = source
	if f.Type == FieldToggle || f.Type == FieldSelect {
		for i, opt := range f.Options {
			if opt == val {
				f.optIndex = i
				break
			}
		}
	}
	f.validate()
}

// SetValidator sets a custom validation function.
func (f *SetupField) SetValidator(fn func(string) ValidationResult) {
	f.validator = fn
	f.validate()
}

// IsValid returns whether the current value passes validation.
func (f *SetupField) IsValid() bool {
	return f.lastResult.Valid
}

// IsEditing returns whether the field is in edit mode.
func (f *SetupField) IsEditing() bool {
	return f.editing
}

// StartEditing begins inline editing for text/number fields.
func (f *SetupField) StartEditing() {
	if f.Type != FieldText && f.Type != FieldNumber {
		return
	}
	f.editing = true
	f.textInput.SetValue(f.Value)
	f.textInput.Focus()
}

// CancelEditing discards changes and exits edit mode.
func (f *SetupField) CancelEditing() {
	f.editing = false
	f.textInput.Blur()
}

// ConfirmEditing saves the edited value and exits edit mode.
func (f *SetupField) ConfirmEditing() bool {
	val := f.textInput.Value()
	f.validate()
	if f.validator != nil {
		result := f.validator(val)
		if !result.Valid {
			f.lastResult = result
			return false
		}
	}
	f.Value = val
	f.Source = SourceUser
	f.editing = false
	f.textInput.Blur()
	f.validate()
	return true
}

// Toggle cycles to the next option (for Toggle fields).
func (f *SetupField) Toggle() {
	if f.Type != FieldToggle {
		return
	}
	f.optIndex = (f.optIndex + 1) % len(f.Options)
	f.Value = f.Options[f.optIndex]
	f.Source = SourceUser
}

// Update handles bubbletea messages for the field (mainly textinput updates).
func (f *SetupField) Update(msg tea.Msg) tea.Cmd {
	if !f.editing {
		return nil
	}
	var cmd tea.Cmd
	f.textInput, cmd = f.textInput.Update(msg)

	// Live validation while typing
	if f.validator != nil {
		f.lastResult = f.validator(f.textInput.Value())
	}

	return cmd
}

// Render draws the field as a single line.
// focused: this field has cursor; labelWidth: label column width (0 = auto).
func (f *SetupField) Render(focused bool, labelWidth int) string {
	if f.Type == FieldAction {
		return f.renderAction(focused)
	}

	if labelWidth <= 0 {
		labelWidth = 18
	}
	lw := lipgloss.Width(f.Label)
	if lw >= labelWidth {
		labelWidth = lw + 1
	}
	label := styles.ConnParamLabel.Width(labelWidth).Render(f.Label)

	var valueStr string
	if f.editing {
		valueStr = f.textInput.View()
	} else {
		valueStr = f.renderValue()
	}

	// Source marker
	marker := f.sourceMarker()
	if marker != "" {
		valueStr += " " + marker
	}

	// Hint
	if f.Hint != "" {
		valueStr += " " + styles.SetupDimValue.Render(f.Hint)
	}

	// Validation indicator
	if !f.editing {
		valueStr += " " + f.validationIndicator()
	} else if !f.lastResult.Valid {
		valueStr += "  " + styles.StatValueError.Render("⚠ "+f.lastResult.Message)
	}

	// Cursor indicator for focused field
	cursor := "  "
	if focused {
		cursor = styles.SelectedItem.Render(styles.CursorGlyph + " ")
	}

	return cursor + label + valueStr
}

func (f *SetupField) renderValue() string {
	if f.masked && f.Value != "" {
		return styles.ConnParamValue.Render(strings.Repeat("●", len(f.Value)))
	}

	style := f.valueStyle()
	val := f.Value
	if val == "" && f.Required {
		return styles.SetupRequired.Render("___")
	}
	if val == "" {
		return styles.SetupDimValue.Render(i18n.T("value_none"))
	}
	return style.Render(val)
}

func (f *SetupField) renderAction(focused bool) string {
	cursor := "  "
	if focused {
		cursor = styles.SelectedItem.Render(styles.CursorGlyph + " ")
	}
	label := f.ActionLabel
	if f.Type == FieldAction && f.ActionExpanded {
		label = strings.Replace(label, "▸", "▾", 1)
	}
	return cursor + styles.SetupActionLabel.Render("["+label+"]")
}

func (f *SetupField) valueStyle() lipgloss.Style {
	if !f.lastResult.Valid {
		return styles.StatValueError
	}
	switch f.Source {
	case SourceCLI:
		return styles.SetupCLIValue
	case SourceDefault:
		return styles.SetupDimValue
	case SourceAuto:
		return styles.SetupAutoValue
	default:
		return styles.ConnParamValue
	}
}

func (f *SetupField) sourceMarker() string {
	switch f.Source {
	case SourceCLI:
		return styles.SetupCLIValue.Render("*")
	case SourceAuto:
		return ""
	default:
		return ""
	}
}

func (f *SetupField) validationIndicator() string {
	// Don't show indicator for empty optional fields
	if f.Value == "" && !f.Required {
		return ""
	}
	// Don't show for defaults that haven't been touched
	if f.Source == SourceDefault {
		return ""
	}
	// Don't show ✓ when hint indicates a problem (e.g. "⚠ Server unavailable")
	if f.lastResult.Valid && strings.Contains(f.Hint, "⚠") {
		return ""
	}
	if f.lastResult.Valid {
		return styles.StatValueGood.Render("✓")
	}
	msg := "✗"
	if f.lastResult.Message != "" {
		msg += " (" + f.lastResult.Message + ")"
	}
	return styles.StatValueError.Render(msg)
}

func (f *SetupField) validate() {
	if f.validator != nil {
		f.lastResult = f.validator(f.Value)
		return
	}
	if f.Required && f.Value == "" {
		f.lastResult = ValidationResult{Valid: false, Message: i18n.T("validation_required")}
		return
	}
	f.lastResult = ValidationResult{Valid: true}
}

// IntValue returns the numeric value of a number field (0 if invalid).
func (f *SetupField) IntValue() int {
	n, _ := strconv.Atoi(f.Value) //nolint:errcheck
	return n
}

// OptionIndex returns the current selected option index.
func (f *SetupField) OptionIndex() int {
	return f.optIndex
}

// ValidateAddress checks if a string is a valid IPv4, IPv6, or hostname.
func ValidateAddress(s string) ValidationResult {
	if s == "" {
		return ValidationResult{Valid: false, Message: i18n.T("validation_required")}
	}
	// Try IP parse
	if ip := net.ParseIP(s); ip != nil {
		return ValidationResult{Valid: true}
	}
	// Check hostname format (simplified)
	if len(s) > 253 {
		return ValidationResult{Valid: false, Message: i18n.T("validation_too_long")}
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '.' {
			return ValidationResult{Valid: false, Message: i18n.T("validation_invalid_char")}
		}
	}
	return ValidationResult{Valid: true}
}
