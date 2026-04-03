package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

// RecordingSource represents a selectable recording source (local device or remote client).
type RecordingSource struct {
	ID       string
	Name     string // display name
	Selected bool
}

// RecordingOverlay renders the role-aware recording overlay with categories.
type RecordingOverlay struct {
	Visible  bool
	IsServer bool // determines remote section behavior

	// Sections: 0=mode, 1=local, 2=remote
	ActiveSection int

	// Mode section
	ModeCursor int // 0=mix, 1=tracks, 2=both

	// Local devices section
	LocalDevices []RecordingSource
	LocalCursor  int

	// Remote section (server: All + individual clients; client: single "Incoming stream")
	RemoteSources []RecordingSource // for server: individual clients
	RemoteCursor  int
	RemoteAll     bool // master "All" checkbox (server only)
}

// NewRecordingOverlay creates a hidden recording overlay.
func NewRecordingOverlay() RecordingOverlay {
	return RecordingOverlay{}
}

// Show populates and shows the overlay. All devices selected by default.
func (o *RecordingOverlay) Show(isServer bool, localDevices []RecordingSource, remoteSources []RecordingSource) {
	o.Visible = true
	o.IsServer = isServer
	o.ActiveSection = 0
	o.ModeCursor = 0
	o.LocalCursor = 0
	o.RemoteCursor = 0

	// Copy local devices, select all by default.
	o.LocalDevices = make([]RecordingSource, len(localDevices))
	copy(o.LocalDevices, localDevices)
	for i := range o.LocalDevices {
		o.LocalDevices[i].Selected = true
	}

	// Copy remote sources, select all by default.
	o.RemoteSources = make([]RecordingSource, len(remoteSources))
	copy(o.RemoteSources, remoteSources)
	for i := range o.RemoteSources {
		o.RemoteSources[i].Selected = true
	}

	o.RemoteAll = isServer
}

// Hide hides the overlay.
func (o *RecordingOverlay) Hide() {
	o.Visible = false
}

// SelectedMode returns the currently highlighted recording mode.
func (o *RecordingOverlay) SelectedMode() RecordingMode {
	return RecordingMode(o.ModeCursor)
}

// SelectedLocalDevices returns IDs of selected local devices.
func (o *RecordingOverlay) SelectedLocalDevices() []string {
	var ids []string
	for _, d := range o.LocalDevices {
		if d.Selected {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// SelectedRemoteSources returns IDs of selected remote sources.
// For server: returns IDs of selected individual clients.
// For client: returns ["incoming"] if selected.
func (o *RecordingOverlay) SelectedRemoteSources() []string {
	if !o.IsServer {
		if len(o.RemoteSources) > 0 && o.RemoteSources[0].Selected {
			return []string{"incoming"}
		}
		return nil
	}
	var ids []string
	for _, s := range o.RemoteSources {
		if s.Selected {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// AllSourcesSelected returns true if all local and all remote sources are selected.
func (o *RecordingOverlay) AllSourcesSelected() bool {
	for _, d := range o.LocalDevices {
		if !d.Selected {
			return false
		}
	}
	for _, s := range o.RemoteSources {
		if !s.Selected {
			return false
		}
	}
	return true
}

// SelectedCount returns (selected, total) across local and remote sources.
func (o *RecordingOverlay) SelectedCount() (selected, total int) {
	for _, d := range o.LocalDevices {
		total++
		if d.Selected {
			selected++
		}
	}
	for _, s := range o.RemoteSources {
		total++
		if s.Selected {
			selected++
		}
	}
	return selected, total
}

// Toggle handles space key: in mode section selects mode, in local/remote toggles checkbox.
func (o *RecordingOverlay) Toggle() {
	switch o.ActiveSection {
	case 0:
		// Mode section — cursor already selects mode, nothing extra needed.
	case 1:
		if o.LocalCursor >= 0 && o.LocalCursor < len(o.LocalDevices) {
			o.LocalDevices[o.LocalCursor].Selected = !o.LocalDevices[o.LocalCursor].Selected
		}
	case 2:
		o.toggleRemote()
	}
}

func (o *RecordingOverlay) toggleRemote() {
	if o.IsServer {
		// Row 0 = "All" master checkbox.
		if o.RemoteCursor == 0 {
			o.RemoteAll = !o.RemoteAll
			for i := range o.RemoteSources {
				o.RemoteSources[i].Selected = o.RemoteAll
			}
		} else {
			// Individual client: offset by 2 (row 0=All, row 1=separator).
			idx := o.RemoteCursor - 2
			if idx >= 0 && idx < len(o.RemoteSources) {
				o.RemoteSources[idx].Selected = !o.RemoteSources[idx].Selected
				// Update RemoteAll state.
				o.RemoteAll = o.allRemoteSelected()
			}
		}
	} else {
		if len(o.RemoteSources) > 0 {
			o.RemoteSources[0].Selected = !o.RemoteSources[0].Selected
		}
	}
}

func (o *RecordingOverlay) allRemoteSelected() bool {
	for _, s := range o.RemoteSources {
		if !s.Selected {
			return false
		}
	}
	return true
}

// NextSection moves to the next section (Tab).
func (o *RecordingOverlay) NextSection() {
	o.ActiveSection = (o.ActiveSection + 1) % 3
}

// PrevSection moves to the previous section (Shift+Tab).
func (o *RecordingOverlay) PrevSection() {
	o.ActiveSection = (o.ActiveSection + 2) % 3
}

// Up moves cursor up within the current section.
func (o *RecordingOverlay) Up() {
	switch o.ActiveSection {
	case 0:
		if o.ModeCursor > 0 {
			o.ModeCursor--
		}
	case 1:
		if o.LocalCursor > 0 {
			o.LocalCursor--
		}
	case 2:
		o.remoteUp()
	}
}

// Down moves cursor down within the current section.
func (o *RecordingOverlay) Down() {
	switch o.ActiveSection {
	case 0:
		if o.ModeCursor < 2 {
			o.ModeCursor++
		}
	case 1:
		if o.LocalCursor < len(o.LocalDevices)-1 {
			o.LocalCursor++
		}
	case 2:
		o.remoteDown()
	}
}

func (o *RecordingOverlay) remoteUp() {
	if o.IsServer {
		if o.RemoteCursor > 0 {
			o.RemoteCursor--
			// Skip separator row (row 1).
			if o.RemoteCursor == 1 {
				o.RemoteCursor = 0
			}
		}
	}
	// Single item: cursor stays at 0.
}

func (o *RecordingOverlay) remoteDown() {
	if o.IsServer {
		maxRow := 1 + len(o.RemoteSources) // row0=All, row1=sep, row2..N=clients
		if o.RemoteCursor < maxRow {
			o.RemoteCursor++
			// Skip separator row (row 1).
			if o.RemoteCursor == 1 {
				o.RemoteCursor = 2
			}
			// Clamp.
			if o.RemoteCursor > maxRow {
				o.RemoteCursor = maxRow
			}
		}
	}
}

// SelectAll selects all items in the current section (Ctrl+A).
func (o *RecordingOverlay) SelectAll() {
	switch o.ActiveSection {
	case 1:
		for i := range o.LocalDevices {
			o.LocalDevices[i].Selected = true
		}
	case 2:
		for i := range o.RemoteSources {
			o.RemoteSources[i].Selected = true
		}
		if o.IsServer {
			o.RemoteAll = true
		}
	}
}

// SelectNone deselects all items in the current section (Ctrl+N).
func (o *RecordingOverlay) SelectNone() {
	switch o.ActiveSection {
	case 1:
		for i := range o.LocalDevices {
			o.LocalDevices[i].Selected = false
		}
	case 2:
		for i := range o.RemoteSources {
			o.RemoteSources[i].Selected = false
		}
		if o.IsServer {
			o.RemoteAll = false
		}
	}
}

// Render draws the recording overlay centered within the given width.
func (o *RecordingOverlay) Render(width int) string {
	if !o.Visible {
		return ""
	}

	activeLabelStyle := styles.OverlayActive
	dimLabelStyle := styles.OverlayDim

	var sections []string

	// --- Mode section ---
	{
		label := "Mode:"
		if o.ActiveSection == 0 {
			label = activeLabelStyle.Render(label)
		} else {
			label = dimLabelStyle.Render(label)
		}
		lines := []string{label}
		modes := []string{"Mix (single file)", "Separate tracks", "Mix + tracks"}
		for i, m := range modes {
			prefix := "  "
			if i == o.ModeCursor {
				prefix = styles.CursorGlyph + " "
			}
			line := prefix + m
			if o.ActiveSection == 0 && i == o.ModeCursor {
				line = styles.OverlayActive.Render(line)
			}
			lines = append(lines, line)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// --- Local devices section ---
	{
		label := "Local devices:"
		if o.ActiveSection == 1 {
			label = activeLabelStyle.Render(label)
		} else {
			label = dimLabelStyle.Render(label)
		}
		lines := []string{label}
		for i, d := range o.LocalDevices {
			check := "[ ]"
			if d.Selected {
				check = "[✓]"
			}
			line := fmt.Sprintf("  %s %s", check, d.Name)
			if o.ActiveSection == 1 && i == o.LocalCursor {
				line = styles.OverlayActive.Render(line)
			}
			lines = append(lines, line)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// --- Remote section ---
	{
		var label string
		if o.IsServer {
			label = "Remote clients:"
		} else {
			label = "Remote (server mix):"
		}
		if o.ActiveSection == 2 {
			label = activeLabelStyle.Render(label)
		} else {
			label = dimLabelStyle.Render(label)
		}
		lines := []string{label}

		if o.IsServer {
			// "All" master checkbox (row 0).
			allCheck := "[ ]"
			if o.RemoteAll {
				allCheck = "[✓]"
			}
			allLine := fmt.Sprintf("  %s All", allCheck)
			if o.ActiveSection == 2 && o.RemoteCursor == 0 {
				allLine = styles.OverlayActive.Render(allLine)
			}
			// Separator (row 1).
			lines = append(lines, allLine, "  ── ── ── ── ──")

			// Individual clients (rows 2+).
			for i, s := range o.RemoteSources {
				check := "[ ]"
				if s.Selected {
					check = "[✓]"
				}
				line := fmt.Sprintf("  %s %s", check, s.Name)
				rowIdx := i + 2
				if o.RemoteAll {
					// When All is selected, individual items are dimmed and non-interactive.
					line = styles.OverlayDim.Render(line)
				} else if o.ActiveSection == 2 && o.RemoteCursor == rowIdx {
					line = styles.OverlayActive.Render(line)
				}
				lines = append(lines, line)
			}
		} else if len(o.RemoteSources) > 0 {
			// Client: single "Incoming stream" checkbox.
			check := "[ ]"
			if o.RemoteSources[0].Selected {
				check = "[✓]"
			}
			line := fmt.Sprintf("  %s Incoming stream", check)
			if o.ActiveSection == 2 && o.RemoteCursor == 0 {
				line = styles.OverlayActive.Render(line)
			}
			lines = append(lines, line)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// Combine sections with blank line separators.
	body := strings.Join(sections, "\n\n")

	// Hint line.
	hint := dimLabelStyle.Render("tab: section  space: toggle\n^A: all  ^N: none  enter: start\nesc: cancel")

	content := body + "\n\n" + hint

	box := styles.OverlayBorder.Width(overlayWidth).Render(content)

	// Inject title into the top border.
	boxLines := strings.Split(box, "\n")
	if len(boxLines) > 0 {
		title := " Start recording "
		topBorder := boxLines[0]
		// Replace part of the top border with the title.
		runes := []rune(topBorder)
		titleRunes := []rune(title)
		if len(runes) > len(titleRunes)+2 {
			// Insert title starting at position 2.
			copy(runes[2:2+len(titleRunes)], titleRunes)
			boxLines[0] = string(runes)
		}
	}
	box = strings.Join(boxLines, "\n")

	// Center horizontally.
	boxW := lipgloss.Width(box)
	pad := (width - boxW) / 2
	if pad < 0 {
		pad = 0
	}

	var result []string
	for _, line := range strings.Split(box, "\n") {
		result = append(result, strings.Repeat(" ", pad)+line)
	}

	return strings.Join(result, "\n")
}
