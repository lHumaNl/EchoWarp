package views

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/probe"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func makeEntry(addr string, port int, source ServerSource, status ServerProbeStatus, lastConnected time.Time) ServerEntry {
	return ServerEntry{
		Address:       addr,
		Port:          port,
		Hostname:      addr,
		Source:        source,
		LastConnected: lastConnected,
		ProbeStatus:   status,
	}
}

func pressKey(m ServerListModel, kt tea.KeyType) (ServerListModel, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: kt})
}

// ─── 1. NewServerListModel ───────────────────────────────────────────────────

func TestNewServerListModel_InitialState(t *testing.T) {
	m := NewServerListModel(80, 24)
	assert.Equal(t, -1, m.selected, "selected should start at -1")
	assert.Empty(t, m.entries, "entries should be empty")
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)
	assert.False(t, m.scanFocused)
}

// ─── 2. SetEntries — sort order ──────────────────────────────────────────────

func TestSetEntries_SortOrder(t *testing.T) {
	now := time.Now()

	onlineRecent := ServerEntry{
		Address: "192.168.1.1", Port: 4415,
		Source: ServerSourceRecent, ProbeStatus: ServerProbeOnline,
		LastConnected: now,
	}
	onlineMDNS := ServerEntry{
		Address: "192.168.1.2", Port: 4415,
		Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline,
		Hostname: "mdns-server",
	}
	offline := ServerEntry{
		Address: "192.168.1.3", Port: 4415,
		Source: ServerSourceMDNS, ProbeStatus: ServerProbeOffline,
	}

	// Feed them in reverse expected order
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{offline, onlineMDNS, onlineRecent})

	entries := m.Entries()
	require.Len(t, entries, 3)
	assert.Equal(t, "192.168.1.1", entries[0].Address, "online recent should be first")
	assert.Equal(t, "192.168.1.2", entries[1].Address, "online mDNS should be second")
	assert.Equal(t, "192.168.1.3", entries[2].Address, "offline should be last")
}

func TestSetEntries_SortOrder_MultipleOnlineRecent(t *testing.T) {
	now := time.Now()
	older := ServerEntry{
		Address: "10.0.0.1", Port: 4415,
		Source: ServerSourceRecent, ProbeStatus: ServerProbeOnline,
		LastConnected: now.Add(-10 * time.Minute),
	}
	newer := ServerEntry{
		Address: "10.0.0.2", Port: 4415,
		Source: ServerSourceRecent, ProbeStatus: ServerProbeOnline,
		LastConnected: now,
	}

	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{older, newer})

	entries := m.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "10.0.0.2", entries[0].Address, "more recent lastConnected should sort first")
}

// ─── 3. SetEntries — selected index preserved ────────────────────────────────

func TestSetEntries_PreservesSelectedByID(t *testing.T) {
	now := time.Now()
	e1 := ServerEntry{Address: "1.1.1.1", Port: 4415, Source: ServerSourceRecent, ProbeStatus: ServerProbeOnline, LastConnected: now}
	e2 := ServerEntry{Address: "2.2.2.2", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}

	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{e1, e2})
	// Select the online-recent entry (index 0 after sort)
	m.selected = 0

	// Re-set with same entries — sort should keep e1 at position 0
	m.SetEntries([]ServerEntry{e2, e1})

	assert.Equal(t, 0, m.selected, "selected index should follow the entry after re-sort")
	assert.Equal(t, "1.1.1.1", m.entries[m.selected].Address)
}

// ─── 4. SetEntries — selected reset when entry removed ──────────────────────

func TestSetEntries_ResetsSelectedWhenEntryRemoved(t *testing.T) {
	e1 := ServerEntry{Address: "1.1.1.1", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}
	e2 := ServerEntry{Address: "2.2.2.2", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}

	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{e1, e2})
	m.selected = 1 // select e2

	// Remove e2 from list
	m.SetEntries([]ServerEntry{e1})

	assert.Equal(t, -1, m.selected, "selected should be reset to -1 when entry is removed")
}

// ─── 5. Navigation — Up/Down ─────────────────────────────────────────────────

func TestNavigation_DownAndUp(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{
		{Address: "a", Port: 1, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
		{Address: "b", Port: 2, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
		{Address: "c", Port: 3, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	})

	assert.Equal(t, 0, m.cursor)

	m, _ = pressKey(m, tea.KeyDown)
	assert.Equal(t, 1, m.cursor)

	m, _ = pressKey(m, tea.KeyDown)
	assert.Equal(t, 2, m.cursor)

	// At bottom — should not go past last
	m, _ = pressKey(m, tea.KeyDown)
	assert.Equal(t, 2, m.cursor, "cursor should not wrap below last entry")

	m, _ = pressKey(m, tea.KeyUp)
	assert.Equal(t, 1, m.cursor)

	m, _ = pressKey(m, tea.KeyUp)
	assert.Equal(t, 0, m.cursor)

	// At top — should not wrap
	m, _ = pressKey(m, tea.KeyUp)
	assert.Equal(t, 0, m.cursor, "cursor should not wrap above first entry")
}

// ─── 6. Selection — Enter sets selected and returns ServerSelectedMsg ────────

func TestEnter_SetsSelectedAndReturnsMsg(t *testing.T) {
	entry := ServerEntry{Address: "192.168.1.5", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{entry})

	m, cmd := pressKey(m, tea.KeyEnter)
	require.NotNil(t, cmd, "Enter should return a command")

	msg := cmd()
	sel, ok := msg.(ServerSelectedMsg)
	require.True(t, ok, "command should produce ServerSelectedMsg")
	assert.Equal(t, entry.Address, sel.Entry.Address)
	assert.Equal(t, 0, m.selected)
}

func TestEnter_EmptyList_NoCmd(t *testing.T) {
	m := NewServerListModel(80, 24)
	_, cmd := pressKey(m, tea.KeyEnter)
	assert.Nil(t, cmd)
}

// ─── 7. Tab — toggles scanFocused ───────────────────────────────────────────

func TestTab_TogglesScanFocused(t *testing.T) {
	m := NewServerListModel(80, 24)
	assert.False(t, m.scanFocused)

	m, cmd := pressKey(m, tea.KeyTab)
	assert.True(t, m.scanFocused)
	assert.Nil(t, cmd, "Tab should return nil cmd when moving to scan")

	// Tab again while scan focused should toggle back
	m, cmd = pressKey(m, tea.KeyTab)
	assert.False(t, m.scanFocused)
	assert.Nil(t, cmd)
}

// ─── 8. Enter on scan — returns ScanRequestMsg ──────────────────────────────

func TestEnterOnScan_ReturnsScanRequestMsg(t *testing.T) {
	m := NewServerListModel(80, 24)
	m, _ = pressKey(m, tea.KeyTab) // focus scan
	assert.True(t, m.scanFocused)

	m, cmd := pressKey(m, tea.KeyEnter)
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(ScanRequestMsg)
	assert.True(t, ok, "Enter on scan should produce ScanRequestMsg")
	// scanFocused stays true while scan is active
	assert.True(t, m.scanFocused)
}

// ─── 9. ClearSelection ───────────────────────────────────────────────────────

func TestClearSelection(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	})
	m.selected = 0

	m.ClearSelection()
	assert.Equal(t, -1, m.selected)
}

// ─── 10. View — header ───────────────────────────────────────────────────────

func TestView_RendersHeader(t *testing.T) {
	m := NewServerListModel(80, 24)
	out := m.View(80)

	assert.True(t, strings.Contains(out, "Servers"), "header should contain 'Servers'")
	assert.True(t, strings.Contains(out, "Scan"), "header should contain 'Scan'")
}

// ─── 11. View — entry markers ───────────────────────────────────────────────

func TestView_EntryMarkers(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{
		{Address: "a.a.a.a", Port: 4415, Hostname: "serverA", Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
		{Address: "b.b.b.b", Port: 4415, Hostname: "serverB", Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	})
	// Select first entry
	m.selected = 0

	out := m.View(80)
	assert.True(t, strings.Contains(out, "●"), "selected entry should show filled circle")
	assert.True(t, strings.Contains(out, "○"), "unselected entry should show open circle")
}

// ─── 12. View — empty state ──────────────────────────────────────────────────

func TestView_EmptyState(t *testing.T) {
	m := NewServerListModel(80, 24)
	out := m.View(80)

	assert.True(t, strings.Contains(out, "No servers found"), "empty state message should appear")
	assert.False(t, strings.Contains(out, "●"), "no markers when list is empty")
}

// ─── 13. renderEntryDetails — online with TLS, password, mode, clients ───────

func TestRenderEntryDetails_OnlineWithProbeResult(t *testing.T) {
	m := NewServerListModel(80, 24)
	e := ServerEntry{
		Address:     "1.2.3.4",
		Port:        4415,
		ProbeStatus: ServerProbeOnline,
		ProbeResult: &probe.ProbeServerResult{
			TLSRequired:      true,
			TLSSelfSigned:    false,
			PasswordRequired: true,
			Mode:             "stereo",
			CurrentClients:   1,
			MaxClients:       5,
		},
	}
	out := m.renderEntryDetails(e)

	assert.True(t, strings.Contains(out, "🔒"), "TLS should show lock icon")
	assert.True(t, strings.Contains(out, "🔑"), "password required should show key icon")
	assert.True(t, strings.Contains(out, "stereo"), "mode should be rendered")
	assert.True(t, strings.Contains(out, "1/5"), "client count should be rendered")
}

func TestRenderEntryDetails_OnlineFullServer(t *testing.T) {
	m := NewServerListModel(80, 24)
	e := ServerEntry{
		ProbeStatus: ServerProbeOnline,
		ProbeResult: &probe.ProbeServerResult{
			TLSRequired:    false,
			Mode:           "mono",
			CurrentClients: 5,
			MaxClients:     5,
		},
	}
	out := m.renderEntryDetails(e)

	assert.False(t, strings.Contains(out, "🔒"), "no TLS should not show lock icon")
	assert.True(t, strings.Contains(out, "5/5"), "client count shown")
	assert.True(t, strings.Contains(out, "full"), "full warning should appear")
}

func TestRenderEntryDetails_OnlineNilProbeResult(t *testing.T) {
	m := NewServerListModel(80, 24)
	e := ServerEntry{ProbeStatus: ServerProbeOnline, ProbeResult: nil}
	out := m.renderEntryDetails(e)
	assert.True(t, strings.Contains(out, "online"), "nil probe result should show 'online'")
}

// ─── 14. renderEntryDetails — offline ───────────────────────────────────────

func TestRenderEntryDetails_Offline(t *testing.T) {
	m := NewServerListModel(80, 24)
	e := ServerEntry{ProbeStatus: ServerProbeOffline}
	out := m.renderEntryDetails(e)
	assert.True(t, strings.Contains(out, "offline"), "offline entry should show 'offline'")
}

// ─── 15. renderEntryDetails — probing ────────────────────────────────────────

func TestRenderEntryDetails_Probing(t *testing.T) {
	m := NewServerListModel(80, 24)
	for _, status := range []ServerProbeStatus{ServerProbePending, ServerProbeProbing} {
		e := ServerEntry{ProbeStatus: status}
		out := m.renderEntryDetails(e)
		assert.True(t, strings.Contains(out, "probing..."), "probing status should show 'probing...' for status %d", status)
	}
}

// ─── 16. HasEntries, IsServerSelected, SelectedEntry ────────────────────────

func TestHasEntries(t *testing.T) {
	m := NewServerListModel(80, 24)
	assert.False(t, m.HasEntries())

	m.SetEntries([]ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	})
	assert.True(t, m.HasEntries())
}

func TestIsServerSelected(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	})

	assert.False(t, m.IsServerSelected(), "no server selected initially")

	m.selected = 0
	assert.True(t, m.IsServerSelected())

	m.selected = -1
	assert.False(t, m.IsServerSelected())

	// Out-of-bounds index
	m.selected = 99
	assert.False(t, m.IsServerSelected(), "out-of-bounds selected index should return false")
}

func TestSelectedEntry(t *testing.T) {
	entry := ServerEntry{Address: "1.1.1.1", Port: 4415, Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}
	m := NewServerListModel(80, 24)
	m.SetEntries([]ServerEntry{entry})

	assert.Nil(t, m.SelectedEntry(), "no selection should return nil")

	m.selected = 0
	sel := m.SelectedEntry()
	require.NotNil(t, sel)
	assert.Equal(t, entry.Address, sel.Address)
}

func TestServerEntry_ID(t *testing.T) {
	e := ServerEntry{Address: "192.168.1.100", Port: 4415}
	assert.Equal(t, "192.168.1.100:4415", e.ID())
}
