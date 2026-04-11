package views

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// RecordingMode mirrors audio.RecordingMode for TUI layer.
type RecordingMode int

const (
	RecordMix    RecordingMode = 0
	RecordTracks RecordingMode = 1
	RecordBoth   RecordingMode = 2
)

const overlayWidth = 42

// RecordingOverlayState distinguishes between start and status views.
type RecordingOverlayState int

const (
	recordingOverlayStart  RecordingOverlayState = 0
	recordingOverlayStatus RecordingOverlayState = 1
)

// RecordingSource represents a selectable recording source (local device or remote client).
type RecordingSource struct {
	ID        string
	Name      string // display name
	Selected  bool
	IsCapture bool // true=capture device, false=playback/client
}

// RecordingOverlay renders the role-aware recording overlay with categories.
type RecordingOverlay struct {
	Visible bool
	State   RecordingOverlayState

	// --- Start state fields ---

	// Flat item list for navigation (built dynamically).
	items  []recordingItem
	cursor int

	// Sections available for display.
	showCapture  bool
	showPlayback bool
	showClients  bool
	showMode     bool

	// Source data.
	CaptureDevices     []RecordingSource
	PlaybackDevices    []RecordingSource
	Clients            []RecordingSource
	clientsAllSelected bool

	// Mode (only shown when >1 clients).
	ModeCursor int // 0=mix, 1=tracks, 2=both

	// --- Status state fields ---

	RecordingDir      string
	RecordingFileName string
	RecordingSize     uint64
	RecordingStart    time.Time
}

// recordingItem represents a navigable item in the start overlay.
type recordingItem struct {
	kind      itemKind
	sourceIdx int // index into the respective source slice
}

type itemKind int

const (
	itemSectionHeader itemKind = iota // non-interactive header
	itemCapture
	itemPlayback
	itemClientAll // select/deselect all toggle
	itemClient
	itemModeMix
	itemModeTracks
	itemModeBoth
)

// NewRecordingOverlay creates a hidden recording overlay.
func NewRecordingOverlay() RecordingOverlay {
	return RecordingOverlay{}
}

// ShowStart populates and shows the start overlay.
// captureDevices: local capture devices available.
// playbackDevices: local playback devices available (shown only when 1 client).
// clients: connected remote clients.
// All sources are selected by default.
func (o *RecordingOverlay) ShowStart(captureDevices, playbackDevices []RecordingSource, clients []RecordingSource) {
	o.Visible = true
	o.State = recordingOverlayStart
	o.cursor = 0
	o.ModeCursor = 0

	// Copy and select all.
	o.CaptureDevices = copySelectAll(captureDevices)
	o.PlaybackDevices = copySelectAll(playbackDevices)
	o.Clients = copySelectAll(clients)
	o.clientsAllSelected = true

	totalSources := len(captureDevices) + len(playbackDevices) + len(clients)

	// Determine which sections to show based on rules.
	// Capture devices: shown if there are captures AND at least 1 other source.
	o.showCapture = len(captureDevices) > 0 && (len(playbackDevices) > 0 || len(clients) > 0)
	// Playback devices: shown only when exactly 1 client.
	o.showPlayback = len(playbackDevices) > 0 && len(clients) == 1
	// Clients list: shown when >1 clients.
	o.showClients = len(clients) > 1
	// Mode: shown when >1 clients.
	o.showMode = len(clients) > 1

	// If multiple capture devices but nothing else, show capture list.
	if len(captureDevices) > 1 && len(playbackDevices) == 0 && len(clients) == 0 {
		o.showCapture = true
	}

	// If only 1 source total — empty popup, just Start/Cancel (no items).
	if totalSources <= 1 && !o.showMode {
		o.items = nil
		return
	}

	o.buildItems()

	// Set cursor to first interactive item.
	for i, it := range o.items {
		if it.kind != itemSectionHeader {
			o.cursor = i
			break
		}
	}
}

// ShowStatus shows the recording status overlay.
func (o *RecordingOverlay) ShowStatus(dir, fileName string, size uint64, start time.Time) {
	o.Visible = true
	o.State = recordingOverlayStatus
	o.RecordingDir = dir
	o.RecordingFileName = fileName
	o.RecordingSize = size
	o.RecordingStart = start
}

// UpdateStatus updates the status overlay fields without changing visibility.
func (o *RecordingOverlay) UpdateStatus(size uint64) {
	o.RecordingSize = size
}

// Hide hides the overlay.
func (o *RecordingOverlay) Hide() {
	o.Visible = false
}

// IsStartState returns true if in start mode.
func (o *RecordingOverlay) IsStartState() bool {
	return o.State == recordingOverlayStart
}

// IsStatusState returns true if in status mode.
func (o *RecordingOverlay) IsStatusState() bool {
	return o.State == recordingOverlayStatus
}

// buildItems constructs the flat navigable items list.
func (o *RecordingOverlay) buildItems() {
	o.items = nil

	if o.showCapture {
		o.items = append(o.items, recordingItem{kind: itemSectionHeader})
		for i := range o.CaptureDevices {
			o.items = append(o.items, recordingItem{kind: itemCapture, sourceIdx: i})
		}
	}

	if o.showPlayback {
		o.items = append(o.items, recordingItem{kind: itemSectionHeader})
		for i := range o.PlaybackDevices {
			o.items = append(o.items, recordingItem{kind: itemPlayback, sourceIdx: i})
		}
	}

	if o.showClients {
		o.items = append(o.items,
			recordingItem{kind: itemSectionHeader},
			recordingItem{kind: itemClientAll},
		)
		for i := range o.Clients {
			o.items = append(o.items, recordingItem{kind: itemClient, sourceIdx: i})
		}
	}

	if o.showMode {
		o.items = append(o.items,
			recordingItem{kind: itemSectionHeader},
			recordingItem{kind: itemModeMix},
			recordingItem{kind: itemModeTracks},
			recordingItem{kind: itemModeBoth},
		)
	}
}

// SelectedMode returns the currently selected recording mode.
func (o *RecordingOverlay) SelectedMode() RecordingMode {
	return RecordingMode(o.ModeCursor)
}

// SelectedLocalDevices returns IDs of selected capture devices.
func (o *RecordingOverlay) SelectedLocalDevices() []string {
	var ids []string
	for _, d := range o.CaptureDevices {
		if d.Selected {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// SelectedPlaybackDevices returns IDs of selected playback devices.
func (o *RecordingOverlay) SelectedPlaybackDevices() []string {
	var ids []string
	for _, d := range o.PlaybackDevices {
		if d.Selected {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// SelectedRemoteSources returns IDs of selected remote clients.
func (o *RecordingOverlay) SelectedRemoteSources() []string {
	var ids []string
	for _, s := range o.Clients {
		if s.Selected {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// AllSourcesSelected returns true if all sources are selected.
func (o *RecordingOverlay) AllSourcesSelected() bool {
	for _, d := range o.CaptureDevices {
		if !d.Selected {
			return false
		}
	}
	for _, d := range o.PlaybackDevices {
		if !d.Selected {
			return false
		}
	}
	for _, s := range o.Clients {
		if !s.Selected {
			return false
		}
	}
	return true
}

// SelectedCount returns (selected, total) across all sources.
func (o *RecordingOverlay) SelectedCount() (selected, total int) {
	for _, d := range o.CaptureDevices {
		total++
		if d.Selected {
			selected++
		}
	}
	for _, d := range o.PlaybackDevices {
		total++
		if d.Selected {
			selected++
		}
	}
	for _, s := range o.Clients {
		total++
		if s.Selected {
			selected++
		}
	}
	return selected, total
}

// Toggle handles space key: toggles checkbox at cursor.
func (o *RecordingOverlay) Toggle() {
	if o.cursor < 0 || o.cursor >= len(o.items) {
		return
	}
	it := o.items[o.cursor]
	switch it.kind {
	case itemCapture:
		if it.sourceIdx < len(o.CaptureDevices) {
			o.CaptureDevices[it.sourceIdx].Selected = !o.CaptureDevices[it.sourceIdx].Selected
		}
	case itemPlayback:
		if it.sourceIdx < len(o.PlaybackDevices) {
			o.PlaybackDevices[it.sourceIdx].Selected = !o.PlaybackDevices[it.sourceIdx].Selected
		}
	case itemClientAll:
		o.clientsAllSelected = !o.clientsAllSelected
		for i := range o.Clients {
			o.Clients[i].Selected = o.clientsAllSelected
		}
	case itemClient:
		if it.sourceIdx < len(o.Clients) {
			o.Clients[it.sourceIdx].Selected = !o.Clients[it.sourceIdx].Selected
			o.clientsAllSelected = o.allClientsSelected()
		}
	case itemModeMix:
		o.ModeCursor = 0
	case itemModeTracks:
		o.ModeCursor = 1
	case itemModeBoth:
		o.ModeCursor = 2
	}
}

func (o *RecordingOverlay) allClientsSelected() bool {
	for _, s := range o.Clients {
		if !s.Selected {
			return false
		}
	}
	return true
}

// Up moves cursor up, skipping section headers.
func (o *RecordingOverlay) Up() {
	for i := o.cursor - 1; i >= 0; i-- {
		if o.items[i].kind != itemSectionHeader {
			o.cursor = i
			return
		}
	}
}

// Down moves cursor down, skipping section headers.
func (o *RecordingOverlay) Down() {
	for i := o.cursor + 1; i < len(o.items); i++ {
		if o.items[i].kind != itemSectionHeader {
			o.cursor = i
			return
		}
	}
}

// Render draws the recording overlay centered within the given width.
func (o *RecordingOverlay) Render(width, height int) string {
	if !o.Visible {
		return ""
	}

	var box string
	if o.State == recordingOverlayStatus {
		box = o.renderStatus(width)
	} else {
		box = o.renderStart(width)
	}

	// Center vertically.
	boxH := lipgloss.Height(box)
	padY := (height - boxH) / 2
	if padY < 0 {
		padY = 0
	}

	var result strings.Builder
	for i := 0; i < padY; i++ {
		result.WriteString("\n")
	}
	result.WriteString(box)

	return result.String()
}

func (o *RecordingOverlay) renderStart(width int) string {
	dimStyle := styles.OverlayDim

	var lines []string

	if len(o.items) == 0 {
		// No selectable items — just show hint.
		lines = append(lines, dimStyle.Render(i18n.T("overlay_recording_all_sources")))
	} else {
		itemIdx := 0
		sectionHeaders := o.sectionHeaderNames()
		sectionNum := 0

		for _, it := range o.items {
			if it.kind == itemSectionHeader {
				if len(lines) > 0 {
					lines = append(lines, "") // blank separator
				}
				name := ""
				if sectionNum < len(sectionHeaders) {
					name = sectionHeaders[sectionNum]
					sectionNum++
				}
				lines = append(lines, styles.OverlayActive.Render(name))
				continue
			}

			isCurrent := itemIdx == o.cursorItemIndex()
			line := o.formatItem(it, isCurrent)
			lines = append(lines, line)
			itemIdx++
		}
	}

	// Hint line.
	hint := dimStyle.Render(i18n.T("overlay_recording_hint"))
	content := strings.Join(lines, "\n") + "\n\n" + hint

	return o.wrapBox(content, i18n.T("overlay_recording_title"), width)
}

func (o *RecordingOverlay) renderStatus(width int) string {
	dimStyle := styles.OverlayDim

	dur := time.Since(o.RecordingStart)
	durStr := formatRecDuration(dur)

	var lines []string
	if o.RecordingFileName != "" {
		lines = append(lines, fmt.Sprintf("  %s", filepath.Base(o.RecordingFileName)))
	} else if o.RecordingDir != "" {
		lines = append(lines, fmt.Sprintf("  %s", filepath.Base(o.RecordingDir)))
	}
	lines = append(lines, i18n.Tf("overlay_recording_size", formatSize(o.RecordingSize)))

	hint := dimStyle.Render(i18n.T("overlay_recording_status_hint"))
	content := strings.Join(lines, "\n") + "\n\n" + hint

	title := i18n.Tf("overlay_recording_status_title", durStr)
	return o.wrapBox(content, title, width)
}

// sectionHeaderNames returns display names for each section in order.
func (o *RecordingOverlay) sectionHeaderNames() []string {
	var names []string
	if o.showCapture {
		names = append(names, i18n.T("overlay_recording_section_capture"))
	}
	if o.showPlayback {
		names = append(names, i18n.T("overlay_recording_section_playback"))
	}
	if o.showClients {
		names = append(names, i18n.T("overlay_recording_section_clients"))
	}
	if o.showMode {
		names = append(names, i18n.T("overlay_recording_section_mode"))
	}
	return names
}

// cursorItemIndex returns the index among interactive items (excluding headers).
func (o *RecordingOverlay) cursorItemIndex() int {
	count := 0
	for i, it := range o.items {
		if it.kind == itemSectionHeader {
			continue
		}
		if i == o.cursor {
			return count
		}
		count++
	}
	return -1
}

func (o *RecordingOverlay) formatItem(it recordingItem, isCurrent bool) string {
	var line string

	switch it.kind {
	case itemCapture:
		d := o.CaptureDevices[it.sourceIdx]
		check := checkbox(d.Selected)
		line = fmt.Sprintf("  %s %s", check, d.Name)
	case itemPlayback:
		d := o.PlaybackDevices[it.sourceIdx]
		check := checkbox(d.Selected)
		line = fmt.Sprintf("  %s %s", check, d.Name)
	case itemClientAll:
		check := checkbox(o.clientsAllSelected)
		line = fmt.Sprintf("  %s %s", check, i18n.T("overlay_recording_all"))
	case itemClient:
		s := o.Clients[it.sourceIdx]
		check := checkbox(s.Selected)
		line = fmt.Sprintf("  %s %s", check, s.Name)
	case itemModeMix:
		line = o.formatModeItem(0, i18n.T("overlay_recording_mode_mix"))
	case itemModeTracks:
		line = o.formatModeItem(1, i18n.T("overlay_recording_mode_tracks"))
	case itemModeBoth:
		line = o.formatModeItem(2, i18n.T("overlay_recording_mode_both"))
	}

	if isCurrent {
		line = styles.OverlayActive.Render(styles.CursorGlyph + line[1:]) // replace leading space with cursor
	}
	return line
}

func (o *RecordingOverlay) formatModeItem(idx int, label string) string {
	marker := "○"
	if o.ModeCursor == idx {
		marker = "●"
	}
	return fmt.Sprintf("  %s %s", marker, label)
}

func checkbox(selected bool) string {
	if selected {
		return "[✓]"
	}
	return "[ ]"
}

func (o *RecordingOverlay) wrapBox(content, title string, width int) string {
	borderColor := lipgloss.Color("205")
	border := lipgloss.RoundedBorder()

	// Build top border with embedded title manually to avoid ANSI escape issues.
	innerW := overlayWidth
	titleLen := lipgloss.Width(title)
	dashesAfter := innerW - titleLen - 1
	if dashesAfter < 1 {
		dashesAfter = 1
	}
	topLine := lipgloss.NewStyle().Foreground(borderColor).Render(
		border.TopLeft + border.Top + title + strings.Repeat(border.Top, dashesAfter) + border.TopRight)

	// Render content with side borders and padding.
	padded := lipgloss.NewStyle().Padding(1, 2).Width(innerW).Render(content)
	paddedLines := strings.Split(padded, "\n")

	boxLines := make([]string, 0, 1+len(paddedLines)+1)
	boxLines = append(boxLines, topLine)
	left := lipgloss.NewStyle().Foreground(borderColor).Render(border.Left)
	right := lipgloss.NewStyle().Foreground(borderColor).Render(border.Right)
	for _, line := range paddedLines {
		lineW := lipgloss.Width(line)
		rightPad := innerW - lineW
		if rightPad < 0 {
			rightPad = 0
		}
		boxLines = append(boxLines, left+line+strings.Repeat(" ", rightPad)+right)
	}
	bottomLine := lipgloss.NewStyle().Foreground(borderColor).Render(
		border.BottomLeft + strings.Repeat(border.Bottom, innerW) + border.BottomRight)
	boxLines = append(boxLines, bottomLine)

	box := strings.Join(boxLines, "\n")

	// Center horizontally.
	boxW := lipgloss.Width(box)
	pad := (width - boxW) / 2
	if pad < 0 {
		pad = 0
	}

	boxSplit := strings.Split(box, "\n")
	result := make([]string, 0, len(boxSplit))
	for _, line := range boxSplit {
		result = append(result, strings.Repeat(" ", pad)+line)
	}

	return strings.Join(result, "\n")
}

func formatRecDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatSize(bytes uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func copySelectAll(src []RecordingSource) []RecordingSource {
	dst := make([]RecordingSource, len(src))
	copy(dst, src)
	for i := range dst {
		dst[i].Selected = true
	}
	return dst
}

// Legacy compatibility methods for existing code that uses the old API.

// Show is a compatibility wrapper that maps to ShowStart.
func (o *RecordingOverlay) Show(_ bool, localDevices []RecordingSource, remoteSources []RecordingSource) {
	// Map old API to new API: localDevices → captures, remoteSources → clients.
	o.ShowStart(localDevices, nil, remoteSources)
}

// NextSection is kept for compatibility but now does nothing (flat navigation).
func (o *RecordingOverlay) NextSection() {}

// PrevSection is kept for compatibility but now does nothing (flat navigation).
func (o *RecordingOverlay) PrevSection() {}

// SelectAll selects all items of the same kind as the current cursor.
func (o *RecordingOverlay) SelectAll() {
	if o.cursor < 0 || o.cursor >= len(o.items) {
		return
	}
	it := o.items[o.cursor]
	switch it.kind {
	case itemCapture:
		for i := range o.CaptureDevices {
			o.CaptureDevices[i].Selected = true
		}
	case itemPlayback:
		for i := range o.PlaybackDevices {
			o.PlaybackDevices[i].Selected = true
		}
	case itemClientAll, itemClient:
		for i := range o.Clients {
			o.Clients[i].Selected = true
		}
		o.clientsAllSelected = true
	}
}

// SelectNone deselects all items of the same kind as the current cursor.
func (o *RecordingOverlay) SelectNone() {
	if o.cursor < 0 || o.cursor >= len(o.items) {
		return
	}
	it := o.items[o.cursor]
	switch it.kind {
	case itemCapture:
		for i := range o.CaptureDevices {
			o.CaptureDevices[i].Selected = false
		}
	case itemPlayback:
		for i := range o.PlaybackDevices {
			o.PlaybackDevices[i].Selected = false
		}
	case itemClientAll, itemClient:
		for i := range o.Clients {
			o.Clients[i].Selected = false
		}
		o.clientsAllSelected = false
	}
}
