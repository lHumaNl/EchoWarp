package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

func (m SetupModel) View() string {
	// Overlay rendering takes priority
	if m.overlay != SetupOverlayNone {
		base := m.viewBase()
		overlay := m.viewOverlay()
		return m.compositeOverlay(base, overlay)
	}
	return m.viewBase()
}

func (m SetupModel) viewBase() string {
	if m.width >= 80 {
		return m.viewTwoColumns()
	}
	return m.viewSingleColumn()
}

func (m SetupModel) viewOverlay() string {
	switch m.overlay {
	case SetupOverlaySummary:
		if m.summaryOverlay != nil {
			return m.summaryOverlay.View(m.width)
		}
	case SetupOverlayVirtualDevice:
		if m.virtualDeviceOverlay != nil {
			return m.virtualDeviceOverlay.View(m.width)
		}
	case SetupOverlayRestore:
		if m.restoreOverlay != nil {
			return m.restoreOverlay.View(m.width)
		}
	case SetupOverlayConfigSave:
		if m.configSaveOverlay != nil {
			return m.configSaveOverlay.View(m.width)
		}
	case SetupOverlayConfigLoad:
		if m.configLoadOverlay != nil {
			return m.configLoadOverlay.View(m.width)
		}
	case SetupOverlayVirtualSinkLifecycle:
		if m.virtualSinkLifecycleOverlay != nil {
			return m.virtualSinkLifecycleOverlay.View(m.width)
		}
	}
	return ""
}

func (m SetupModel) compositeOverlay(base, overlay string) string {
	if overlay == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Center overlay vertically
	startY := (len(baseLines) - len(overlayLines)) / 2
	if startY < 0 {
		startY = 0
	}

	for i, line := range overlayLines {
		idx := startY + i
		if idx < len(baseLines) {
			// Center horizontally using full screen width
			overlayW := lipgloss.Width(line)
			startX := (m.width - overlayW) / 2
			if startX < 0 {
				startX = 0
			}
			baseLines[idx] = strings.Repeat(" ", startX) + line
		}
	}

	return strings.Join(baseLines, "\n")
}

func (m SetupModel) viewTwoColumns() string {
	leftW := m.leftColumnWidth()
	rightW := m.width - leftW - 3 // 3 = separator + padding

	// Left: device list(s)
	var leftCol string
	if m.unifiedDuplex {
		leftCol = m.renderUnifiedDeviceList(leftW)
	} else if m.HasOutputList {
		leftCol = m.renderDuplexDeviceLists(leftW)
	} else {
		leftTitle := "Select Input Device"
		if !m.IsInput {
			leftTitle = "Select Output Device"
		}
		titleStyle := styles.SetupColumnTitleDim
		if m.activeColumn == ColumnDevices {
			titleStyle = styles.SetupColumnTitle
		}
		leftHeader := titleStyle.Render(leftTitle)

		m.DeviceList.SetSize(leftW-2, m.bodyHeight()-4)
		leftBody := m.DeviceList.View()
		flashLine := m.renderFlashLine()
		if flashLine != "" {
			flashLine += "\n"
		}
		leftCol = leftHeader + "\n" + styles.Separator.Render(strings.Repeat("─", leftW)) + "\n" + flashLine + leftBody
	}

	// Right: settings
	rightTitleStyle := styles.SetupColumnTitleDim
	if m.activeColumn == ColumnSettings {
		rightTitleStyle = styles.SetupColumnTitle
	}
	rightHeader := rightTitleStyle.Render("Settings")
	rightBody := m.renderSettingsPanel(rightW)

	rightCol := rightHeader + "\n" + styles.Separator.Render(strings.Repeat("─", rightW)) + "\n" + m.readyHint() + "\n\n" + rightBody

	// Compose with separator
	leftLines := strings.Split(leftCol, "\n")
	rightLines := strings.Split(rightCol, "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	sep := styles.SetupSeparator.Render(" │ ")
	var result strings.Builder
	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		leftVisible := lipgloss.Width(left)
		if leftVisible < leftW {
			left += strings.Repeat(" ", leftW-leftVisible)
		}

		result.WriteString(left)
		result.WriteString(sep)
		result.WriteString(right)
		if i < maxLines-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m SetupModel) viewSingleColumn() string {
	// Tab bar shown only in narrow mode (width < 80)
	tabBar := m.renderNarrowTabBar()

	if m.activeColumn == ColumnDevices {
		if m.unifiedDuplex {
			body := m.renderUnifiedDeviceList(m.width)
			if tabBar != "" {
				return tabBar + "\n" + body
			}
			return body
		}
		if m.HasOutputList {
			body := m.renderDuplexDeviceLists(m.width)
			if tabBar != "" {
				return tabBar + "\n" + body
			}
			return body
		}
		title := "Select Input Device"
		if !m.IsInput {
			title = "Select Output Device"
		}
		header := styles.SetupColumnTitle.Render(title)
		m.DeviceList.SetSize(m.width-2, m.bodyHeight()-4)
		flashLine := m.renderFlashLine()
		if flashLine != "" {
			flashLine += "\n"
		}
		body := header + "\n" + styles.Separator.Render(strings.Repeat("─", m.width)) + "\n" + flashLine + m.DeviceList.View()
		if tabBar != "" {
			return tabBar + "\n" + body
		}
		return body
	}

	header := styles.SetupColumnTitle.Render("Settings")
	body := m.renderSettingsPanel(m.width - 2)
	result := header + "\n" + styles.Separator.Render(strings.Repeat("─", m.width)) + "\n" + body
	if tabBar != "" {
		return tabBar + "\n" + result
	}
	return result
}

// renderNarrowTabBar renders a [Devices] | Settings or Devices | [Settings] tab bar.
// Returns empty string when width >= 80 (wide mode uses two-column layout instead).
func (m SetupModel) renderNarrowTabBar() string {
	if m.width >= 80 {
		return ""
	}
	var devTab, settTab string
	if m.activeColumn == ColumnDevices {
		devTab = styles.SetupColumnTitle.Render("[Devices]")
		settTab = styles.SetupColumnTitleDim.Render("Settings")
	} else {
		devTab = styles.SetupColumnTitleDim.Render("Devices")
		settTab = styles.SetupColumnTitle.Render("[Settings]")
	}
	sep := styles.SetupSeparator.Render(" | ")
	return devTab + sep + settTab
}

func (m SetupModel) renderSettingsPanel(width int) string {
	var lines []string

	// Server list (client mode only)
	if m.cfg.Mode == config.ModeClient {
		lines = append(lines, m.serverList.View(width, m.serverListFocused), "")
	}

	fields := m.activeFields()
	for i, f := range fields {
		if f.Hidden {
			continue
		}
		focused := m.activeColumn == ColumnSettings && !m.serverListFocused && i == m.FieldCursor
		lines = append(lines, f.Render(focused, width))
	}

	// Spacer before discovery/profile info
	lines = append(lines, "")

	// Discovery scanning indicator (animated spinner)
	if m.discoveryScanning {
		spinnerFrames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		frame := spinnerFrames[m.discoveryFrame%len(spinnerFrames)]
		lines = append(lines, "", styles.SetupDimValue.Render("  "+string(frame)+" Discovering servers..."))
	}

	return strings.Join(lines, "\n")
}

// deviceStatusHint returns the device selection status shown under "Audio Devices" header.
func (m SetupModel) deviceStatusHint() string {
	if m.isDuplexMode {
		capCount, playCount := 0, 0
		for key := range m.multiSelect {
			if strings.HasPrefix(key, "I:") {
				capCount++
			} else {
				playCount++
			}
		}
		if capCount > 0 && playCount > 0 {
			return styles.SetupReadyHint.Render(fmt.Sprintf("  ✓ %d capture, %d playback", capCount, playCount))
		}
		// Conference server: hub mode — devices optional; participant mode — any combo
		if m.isConferenceMode && m.cfg.Mode == config.ModeServer {
			if len(m.multiSelect) == 0 {
				return styles.SetupReadyHint.Render("  ✓ Hub mode — no devices needed")
			}
			return styles.SetupReadyHint.Render(fmt.Sprintf("  ✓ %d capture, %d playback", capCount, playCount))
		}
		if m.isConferenceMode {
			return styles.SetupErrorHint.Render("  ✗ Conference — select mic and speaker")
		}
		return styles.SetupErrorHint.Render("  ✗ Select at least 1 Capture and 1 Playback")
	}
	if len(m.multiSelect) > 0 {
		return styles.SetupReadyHint.Render(fmt.Sprintf("  ✓ %d selected", len(m.multiSelect)))
	}
	return styles.SetupErrorHint.Render("  ✗ Select at least one device")
}

// modeIndicator returns a styled mode description shown under the device status hint.
func (m SetupModel) modeIndicator() string {
	for _, f := range m.Fields {
		if f.Label == "Mode" {
			return styles.SetupModeIndicator.Render("  ◆ " + f.Value)
		}
	}
	// Client mode: mode from probe result
	if m.probeResult != nil {
		mode := m.probeResult.Mode
		if desc, ok := modeKeyToDescriptive[mode]; ok {
			return styles.SetupModeIndicator.Render("  ◆ " + desc)
		}
		if mode != "" {
			return styles.SetupModeIndicator.Render("  ◆ " + mode)
		}
	}
	return ""
}

func (m SetupModel) readyHint() string {
	// Show transient validation error if set (UX-1)
	if m.validationError != "" {
		return styles.SetupErrorHint.Render("  ✗ " + m.validationError)
	}
	// Client mode: require successful server probe
	if m.cfg.Mode == config.ModeClient {
		if m.probeResult == nil {
			if m.probeStatus == "probing" {
				return styles.SetupDimValue.Render("  ⠋ Probing server...")
			}
			if !m.serverList.IsServerSelected() {
				return styles.SetupDimValue.Render("  Select a server to start")
			}
			return styles.SetupErrorHint.Render("  ✗ Server not reachable")
		}
	}
	// Validate required fields
	allFields := m.allFieldsFlat()
	for _, f := range allFields {
		if f.Required && f.Value == "" {
			return styles.SetupErrorHint.Render("  ✗ " + f.Label + " is required")
		}
		if f.Required && !f.IsValid() {
			return styles.SetupErrorHint.Render("  ✗ " + f.Label + ": " + f.lastResult.Message)
		}
	}
	return styles.SetupReadyHint.Render("  ✓ Settings OK")
}

// renderFlashLine returns a styled flash notification line (empty string if no flash).
func (m SetupModel) renderFlashLine() string {
	if m.flashMsg == "" {
		return ""
	}
	return styles.FlashSuccess.Render("  ⟳ " + m.flashMsg)
}

// HelpKeys returns the help keys string for the status bar.
func (m SetupModel) HelpKeys() string {
	if m.overlay != SetupOverlayNone {
		switch m.overlay {
		case SetupOverlaySummary:
			return "enter: start  esc: back  ^Y: copy cmd"
		case SetupOverlayVirtualDevice:
			return "↑↓: navigate  enter: edit/create  space: toggle  esc: cancel"
		case SetupOverlayRestore:
			return "←→: select  enter: confirm  esc: skip"
		case SetupOverlayConfigSave:
			return "enter: save  esc: cancel"
		case SetupOverlayConfigLoad:
			return "↑↓: browse  enter: load  ^D: delete  esc: cancel"
		case SetupOverlayVirtualSinkLifecycle:
			return "↑↓: navigate  ←→/space: toggle  enter: save  esc: defaults"
		}
	}
	if m.inputMode == ModeEditing {
		return "enter: confirm  esc: cancel"
	}
	if m.activeColumn == ColumnDevices {
		if m.unifiedDuplex && m.isDuplexMode {
			return "↑↓: device  tab: section  space: toggle  ^E: virtual  ↵: start  ^Q: quit"
		}
		if m.unifiedDuplex {
			return "↑↓: device  tab: section  space: toggle  ^E: virtual  ↵: start  ^Q: quit"
		}
		if m.HasOutputList {
			return "/: filter  ↑↓: device  ^↑^↓: section  tab: settings  ↵: start  ^Q: quit"
		}
		return "/: filter  ↑↓: device  ^E: virtual  tab: settings  ↵: start  ^Q: quit"
	}
	return "↑↓: navigate  enter/space: edit  tab: devices  ^S: save  ^L: load  ^H: summary  ^Q: quit"
}

// renderUnifiedDeviceList renders two device sections (Input/Output) with checkbox columns.
// Duplex: two columns (Capture, Playback). Normal/Reverse: one column (Select).
func (m SetupModel) renderUnifiedDeviceList(width int) string {
	var result strings.Builder

	// Header: "Audio Devices" + separator (mirrors Settings column)
	titleStyle := styles.SetupColumnTitleDim
	if m.activeColumn == ColumnDevices {
		titleStyle = styles.SetupColumnTitle
	}
	result.WriteString(titleStyle.Render("Audio Devices"))
	result.WriteString("\n")
	result.WriteString(styles.Separator.Render(strings.Repeat("─", width)))
	result.WriteString("\n")

	// Client mode: show placeholder when no server connected
	if m.cfg.Mode == config.ModeClient && !m.isServerReady() {
		result.WriteString("\n")
		result.WriteString(styles.SetupDimValue.Render("   Select a server to configure"))
		result.WriteString("\n")
		result.WriteString(styles.SetupDimValue.Render("   audio devices"))
		result.WriteString("\n")
		return result.String()
	}

	result.WriteString(m.deviceStatusHint())
	result.WriteString("\n")
	if mi := m.modeIndicator(); mi != "" {
		result.WriteString(mi)
		result.WriteString("\n")
	}
	if fl := m.renderFlashLine(); fl != "" {
		result.WriteString(fl)
		result.WriteString("\n")
	}
	result.WriteString("\n")

	showInput, showOutput := m.visibleSections()

	// Calculate available height for each section
	totalH := m.bodyHeight() - 5 // 5 = header + sep + blank + margins
	visibleCount := 0
	if showInput {
		visibleCount++
	}
	if showOutput {
		visibleCount++
	}
	var sectionH int
	if visibleCount == 2 {
		sectionH = (totalH - 5) / 2 // 5 = 2×(section header+sep) + gap between sections
	} else {
		sectionH = totalH - 2 // single section gets all vertical space
	}
	if sectionH < 3 {
		sectionH = 3
	}

	// Render visible sections
	if showInput {
		result.WriteString(m.renderDeviceSection(SectionInput, m.inputDevices, width, sectionH))
		if showOutput {
			result.WriteString("\n\n\n") // generous spacing between sections
		}
	}
	if showOutput {
		result.WriteString(m.renderDeviceSection(SectionOutput, m.outputDevices, width, sectionH))
	}

	return result.String()
}

// renderDeviceSection renders one section (Input or Output) with header, separator, and device rows.
func (m SetupModel) renderDeviceSection(section DeviceSection, devices []deviceRow, width, maxRows int) string {
	isFocused := m.activeColumn == ColumnDevices && m.DeviceSection == section

	// Section header with icon and column headers
	titleStyle := styles.SetupColumnTitleDim
	if isFocused {
		titleStyle = styles.SetupColumnTitle
	}

	icon := "🎤"
	title := "Input"
	if section == SectionOutput {
		icon = "🔊"
		title = "Output"
	}

	// Build column headers right-aligned
	colHeaders := styles.SetupDimValue.Render("✓ Select")

	headerLeft := titleStyle.Render(icon + " " + title)
	// Pad header to right-align column headers
	headerLeftW := lipgloss.Width(headerLeft)
	colHeadersW := lipgloss.Width(colHeaders)
	gap := width - headerLeftW - colHeadersW - 2
	if gap < 2 {
		gap = 2
	}
	header := headerLeft + strings.Repeat(" ", gap) + colHeaders

	sep := styles.Separator.Render(strings.Repeat("─", width))

	// For the output section, use expanded rows (includes mix sub-items under virtual outputs).
	type rowEntry struct {
		dev         deviceRow
		isMixItem   bool
		mixSelected bool
	}
	var flatRows []rowEntry
	if section == SectionOutput {
		for _, r := range m.buildOutputRows() {
			flatRows = append(flatRows, rowEntry{dev: r.device, isMixItem: r.isMixItem, mixSelected: r.mixSelected})
		}
	} else {
		for _, d := range devices {
			flatRows = append(flatRows, rowEntry{dev: d})
		}
	}

	var body strings.Builder
	if len(flatRows) == 0 {
		body.WriteString("\n" + styles.SetupDimValue.Render("    (no devices)"))
	} else {
		maxItems := maxRows / 2
		if maxItems < 1 {
			maxItems = 1
		}

		scroll := m.inputScroll
		if section == SectionOutput {
			scroll = m.outputScroll
		}
		cursor := -1
		if isFocused {
			cursor = m.deviceCursor
			if cursor < scroll {
				scroll = cursor
			}
			if cursor >= scroll+maxItems {
				scroll = cursor - maxItems + 1
			}
		}
		if scroll < 0 {
			scroll = 0
		}

		end := scroll + maxItems
		if end > len(flatRows) {
			end = len(flatRows)
		}

		checkboxAreaW := 8
		infoColW := 26
		nameW := width - 4 - infoColW - checkboxAreaW
		if nameW < 10 {
			nameW = 10
		}

		for i := scroll; i < end; i++ {
			row := flatRows[i]
			dev := row.dev
			isCursor := i == cursor

			if row.isMixItem {
				// Mix sub-item: indented with └─ 🎤 prefix
				cursorGlyph := "  "
				if isCursor {
					cursorGlyph = "▸ "
				}
				mixName := "    └─ 🎤 " + dev.Name
				mixNameW := nameW + infoColW // mix items span both name and info columns
				mixName = TruncateToWidth(mixName, mixNameW)
				nameVisible := lipgloss.Width(mixName)
				padded := mixName + strings.Repeat(" ", mixNameW-nameVisible)
				if isCursor && isFocused {
					padded = styles.SelectedItem.Render(padded)
				} else {
					padded = styles.SetupDimValue.Render(padded)
				}
				checkboxes := "   " + m.renderCheckbox(row.mixSelected, isFocused && isCursor)
				body.WriteString(cursorGlyph + padded + checkboxes)
			} else {
				// Regular device row
				cursorGlyph := "  "
				if isCursor {
					cursorGlyph = "▸ "
				}

				name := dev.Name
				if dev.IsLoopback {
					name = "🔄 " + name
				} else if dev.IsVirtual {
					name = "⟡ " + name
				}
				name = TruncateToWidth(name, nameW)
				nameVisible := lipgloss.Width(name)
				namePadded := name + strings.Repeat(" ", nameW-nameVisible)

				info := formatDeviceInfo(dev)
				infoVisible := lipgloss.Width(info)
				if infoVisible > infoColW {
					info = info[:infoColW]
					infoVisible = infoColW
				}
				infoPadded := info + strings.Repeat(" ", infoColW-infoVisible)
				infoPadded = styles.SetupDimValue.Render(infoPadded)

				if isCursor && isFocused {
					namePadded = styles.SelectedItem.Render(namePadded)
				}

				roles := m.multiSelect[dev.selectKey()]
				selected := roles.Capture || roles.Playback
				checkboxes := "   " + m.renderCheckbox(selected, isFocused && isCursor)

				body.WriteString(cursorGlyph + namePadded + infoPadded + checkboxes)
			}

			if i < end-1 {
				body.WriteString("\n")
			}
		}

		if len(flatRows) > maxItems {
			indicator := fmt.Sprintf("  (%d–%d of %d)", scroll+1, end, len(flatRows))
			body.WriteString("\n" + styles.SetupDimValue.Render(indicator))
		}
	}

	// Legend for special device markers
	hasVirtual, hasLoopback := false, false
	for _, d := range devices {
		if d.IsVirtual {
			hasVirtual = true
		}
		if d.IsLoopback {
			hasLoopback = true
		}
	}
	if hasVirtual || hasLoopback {
		var parts []string
		if hasVirtual {
			parts = append(parts, "⟡ virtual")
		}
		if hasLoopback {
			parts = append(parts, "🔄 loopback")
		}
		body.WriteString("\n" + styles.SetupDimValue.Render("  "+strings.Join(parts, "  ")))
	}

	return header + "\n" + sep + "\n" + body.String()
}

// renderCheckbox renders a [✓] or [ ] checkbox, highlighted if focused.
func (m SetupModel) renderCheckbox(checked bool, focused bool) string {
	box := "[ ]"
	if checked {
		box = "[✓]"
	}
	if focused {
		if checked {
			return styles.StatValueGood.Render(box)
		}
		return styles.SetupColumnTitle.Render(box)
	}
	if checked {
		return styles.StatValueGood.Render(box)
	}
	return styles.SetupDimValue.Render(box)
}

// renderDuplexDeviceLists renders the left column with two device sections (input + output).
func (m SetupModel) renderDuplexDeviceLists(width int) string {
	devH := m.deviceListHeight()

	// Input section
	inputTitleStyle := styles.SetupColumnTitleDim
	if m.activeColumn == ColumnDevices && m.DeviceSection == SectionInput {
		inputTitleStyle = styles.SetupColumnTitle
	}
	inputHeader := inputTitleStyle.Render("Input Device (mic)")
	m.DeviceList.SetSize(width-2, devH)
	inputBody := m.DeviceList.View()

	// Output section
	outputTitleStyle := styles.SetupColumnTitleDim
	if m.activeColumn == ColumnDevices && m.DeviceSection == SectionOutput {
		outputTitleStyle = styles.SetupColumnTitle
	}
	outputHeader := outputTitleStyle.Render("Output Device (speaker)")
	m.OutputDeviceList.SetSize(width-2, devH)
	outputBody := m.OutputDeviceList.View()

	sep := styles.Separator.Render(strings.Repeat("─", width))
	flashLine := m.renderFlashLine()
	if flashLine != "" {
		flashLine += "\n"
	}
	return inputHeader + "\n" + sep + "\n" + flashLine + inputBody + "\n\n" +
		outputHeader + "\n" + sep + "\n" + outputBody
}
