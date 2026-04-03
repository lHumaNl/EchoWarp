package views

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newClientSetup() SetupModel {
	cfg := config.Config{
		Mode:                 config.ModeClient,
		Port:                 4415,
		SampleRate:           48000,
		Channels:             1,
		MaxClients:           1,
		MaxReconnectAttempts: 5,
		MaxFailedAttempts:    5,
		OpusBitrate:          64000,
		LogLevel:             "info",
	}
	dl := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, dl, false, 120, 40)
	// Clear entries loaded from real filesystem to isolate tests
	m.serverList.SetEntries(nil)
	m.serverList.SetHasRecent(false)
	m.serverList.SetScanning(false)
	m.discoveryScanning = false
	return m
}

func newClientSetupWithRecent(entries []ServerEntry) SetupModel {
	m := newClientSetup()
	m.serverList.SetEntries(entries)
	if len(entries) > 0 {
		m.serverList.SetHasRecent(true)
	}
	return m
}

func fieldByLabel(fields []SetupField, label string) *SetupField {
	for i := range fields {
		if fields[i].Label == label {
			return &fields[i]
		}
	}
	return nil
}

func makeProbeResult(mode string, passwordRequired bool) *probe.ProbeServerResult {
	return &probe.ProbeServerResult{
		Mode:             mode,
		PasswordRequired: passwordRequired,
		TLSRequired:      false,
		CurrentClients:   1,
		MaxClients:       5,
		SampleRate:       48000,
		Channels:         1,
		OpusBitrate:      64000,
	}
}

// sendServerProbe sends a ServerProbeMsg and returns updated model.
func sendServerProbe(m SetupModel, addr string, port int, result *probe.ProbeServerResult, err error) SetupModel {
	id := fmt.Sprintf("%s:%d", addr, port)
	m, _ = m.Update(ServerProbeMsg{Address: id, Result: result, Err: err})
	return m
}

// ─── Scenario 1: Full flow — discovery → probe → select → fields filled ─────

func TestIntegration_FullFlow_DiscoveryProbeSelectFields(t *testing.T) {
	m := newClientSetup()

	// Add 2 servers via mDNS entries directly (bypassing actual mDNS)
	mdnsEntries := []ServerEntry{
		{Address: "192.168.1.50", Port: 4415, Hostname: "HomePC", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
		{Address: "192.168.1.60", Port: 4415, Hostname: "WorkPC", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(mdnsEntries)

	// Probe both — first needs password, second doesn't
	m = sendServerProbe(m, "192.168.1.50", 4415, makeProbeResult("normal", true), nil)
	m = sendServerProbe(m, "192.168.1.60", 4415, makeProbeResult("duplex", false), nil)

	// Both should be online now
	entries := m.serverList.Entries()
	require.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, ServerProbeOnline, e.ProbeStatus)
	}

	// Select the first server (HomePC, password required)
	// Find its index after sort
	var homePCIdx int
	for i, e := range entries {
		if e.Address == "192.168.1.50" {
			homePCIdx = i
			break
		}
	}
	m.serverList.cursor = homePCIdx
	m.serverList.selected = homePCIdx
	m, _ = m.Update(ServerSelectedMsg{Entry: entries[homePCIdx]})

	// Check Address/Port filled
	addrField := fieldByLabel(m.Fields, "Server address")
	require.NotNil(t, addrField)
	assert.Equal(t, "192.168.1.50", addrField.Value)

	portField := fieldByLabel(m.Fields, "Port")
	require.NotNil(t, portField)
	assert.Equal(t, "4415", portField.Value)
}

// ─── Scenario 2: Recent + mDNS merge and sorting ────────────────────────────

func TestIntegration_RecentMDNSMergeAndSort(t *testing.T) {
	now := time.Now()

	// Start with 2 recent servers
	recentEntries := []ServerEntry{
		{Address: "10.0.0.1", Port: 4415, Hostname: "OldServer", Source: ServerSourceRecent,
			LastConnected: now.Add(-1 * time.Hour), ProbeStatus: ServerProbePending},
		{Address: "10.0.0.2", Port: 4415, Hostname: "RecentServer", Source: ServerSourceRecent,
			LastConnected: now, ProbeStatus: ServerProbePending},
	}
	m := newClientSetupWithRecent(recentEntries)

	// Probe recent: first online, second offline
	m = sendServerProbe(m, "10.0.0.1", 4415, makeProbeResult("normal", false), nil)
	m = sendServerProbe(m, "10.0.0.2", 4415, nil, fmt.Errorf("connection refused"))

	// mDNS discovers 1 new + 1 matching existing
	mdnsNew := []ServerEntry{
		{Address: "10.0.0.3", Port: 4415, Hostname: "MDNSNew", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
		{Address: "10.0.0.1", Port: 4415, Hostname: "OldServerMDNS", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	newEntries := m.serverList.MergeDiscoveredEntries(mdnsNew)

	// Only 10.0.0.3 should be new (10.0.0.1 merged)
	require.Len(t, newEntries, 1)
	assert.Equal(t, "10.0.0.3", newEntries[0].Address)

	// Probe the new mDNS server
	m = sendServerProbe(m, "10.0.0.3", 4415, makeProbeResult("normal", false), nil)

	// Total should be 3 (not 4 — dedup)
	entries := m.serverList.Entries()
	require.Len(t, entries, 3)

	// Sort order: recent online (10.0.0.1) first, then mDNS online (10.0.0.3), then recent offline (10.0.0.2)
	assert.Equal(t, "10.0.0.1", entries[0].Address, "recent online should be first")
	assert.Equal(t, ServerSourceBoth, entries[0].Source, "merged entry should be ServerSourceBoth")
	assert.Equal(t, "10.0.0.3", entries[1].Address, "mDNS online should be second")
	assert.Equal(t, "10.0.0.2", entries[2].Address, "recent offline should be last")
}

// ─── Scenario 3: Select → manual edit → selection cleared ───────────────────

func TestIntegration_SelectThenManualEdit_ClearsSelection(t *testing.T) {
	m := newClientSetup()

	entry := ServerEntry{Address: "192.168.1.10", Port: 4415, Hostname: "TestSrv",
		Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline}
	m.serverList.MergeDiscoveredEntries([]ServerEntry{entry})
	m = sendServerProbe(m, "192.168.1.10", 4415, makeProbeResult("normal", false), nil)

	// Select it
	m.serverList.selected = 0
	m, _ = m.Update(ServerSelectedMsg{Entry: m.serverList.Entries()[0]})

	// Verify selected
	assert.True(t, m.serverList.IsServerSelected())
	addrField := fieldByLabel(m.Fields, "Server address")
	require.NotNil(t, addrField)
	assert.Equal(t, "192.168.1.10", addrField.Value)

	// Simulate manual edit by directly changing value and clearing selection
	// (In real UI, typing in Address field triggers ClearSelection)
	m.serverList.ClearSelection()
	assert.Equal(t, -1, m.serverList.selected, "selection should be cleared after manual edit")
}

// ─── Scenario 4: Password visibility toggle ─────────────────────────────────

func TestIntegration_PasswordVisibilityToggle(t *testing.T) {
	m := newClientSetup()

	// Add 2 servers
	entries := []ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Hostname: "NoPwd", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
		{Address: "2.2.2.2", Port: 4415, Hostname: "WithPwd", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(entries)

	// Probe both
	m = sendServerProbe(m, "1.1.1.1", 4415, makeProbeResult("normal", false), nil)
	m = sendServerProbe(m, "2.2.2.2", 4415, makeProbeResult("normal", true), nil)

	// Select server WITHOUT password
	sortedEntries := m.serverList.Entries()
	var noPwdIdx int
	for i, e := range sortedEntries {
		if e.Address == "1.1.1.1" {
			noPwdIdx = i
			break
		}
	}
	m.serverList.selected = noPwdIdx
	m, _ = m.Update(ServerSelectedMsg{Entry: sortedEntries[noPwdIdx]})

	// Now manually apply probe result for the selected server (simulating ProbeServerMsg after selection)
	m = sendServerProbe(m, "1.1.1.1", 4415, makeProbeResult("normal", false), nil)

	pwdField := fieldByLabel(m.Fields, "Password")
	require.NotNil(t, pwdField)
	assert.True(t, pwdField.Hidden, "Password should be hidden when not required")

	// Select server WITH password
	sortedEntries = m.serverList.Entries()
	var pwdIdx int
	for i, e := range sortedEntries {
		if e.Address == "2.2.2.2" {
			pwdIdx = i
			break
		}
	}
	m.serverList.selected = pwdIdx
	m, _ = m.Update(ServerSelectedMsg{Entry: sortedEntries[pwdIdx]})
	m = sendServerProbe(m, "2.2.2.2", 4415, makeProbeResult("normal", true), nil)

	pwdField = fieldByLabel(m.Fields, "Password")
	require.NotNil(t, pwdField)
	assert.False(t, pwdField.Hidden, "Password should be visible when required")
	assert.True(t, pwdField.Required)

	// Switch back to server without password
	m.serverList.selected = noPwdIdx
	m, _ = m.Update(ServerSelectedMsg{Entry: m.serverList.Entries()[noPwdIdx]})
	m = sendServerProbe(m, "1.1.1.1", 4415, makeProbeResult("normal", false), nil)

	pwdField = fieldByLabel(m.Fields, "Password")
	require.NotNil(t, pwdField)
	assert.True(t, pwdField.Hidden, "Password should be hidden again")
	assert.Empty(t, pwdField.Value, "Password value should be cleared")
}

// ─── Scenario 6: Scan button re-probes ──────────────────────────────────────

func TestIntegration_ScanButtonReProbes(t *testing.T) {
	m := newClientSetup()

	// Add 2 online servers
	entries := []ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Hostname: "Srv1", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
		{Address: "2.2.2.2", Port: 4415, Hostname: "Srv2", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(entries)
	m = sendServerProbe(m, "1.1.1.1", 4415, makeProbeResult("normal", false), nil)
	m = sendServerProbe(m, "2.2.2.2", 4415, makeProbeResult("normal", false), nil)

	// Both online
	for _, e := range m.serverList.Entries() {
		assert.Equal(t, ServerProbeOnline, e.ProbeStatus)
	}

	// Trigger scan
	m, cmd := m.Update(ScanRequestMsg{})
	require.NotNil(t, cmd, "Scan should return commands")

	// All entries should be probing
	for _, e := range m.serverList.Entries() {
		assert.Equal(t, ServerProbeProbing, e.ProbeStatus, "all entries should be probing after scan")
	}
	assert.True(t, m.discoveryScanning)
	assert.True(t, m.serverList.scanning)

	// One comes back offline
	m = sendServerProbe(m, "2.2.2.2", 4415, nil, fmt.Errorf("timeout"))
	m = sendServerProbe(m, "1.1.1.1", 4415, makeProbeResult("normal", false), nil)

	// Check sort: online first, offline last
	sorted := m.serverList.Entries()
	require.Len(t, sorted, 2)
	assert.Equal(t, ServerProbeOnline, sorted[0].ProbeStatus)
	assert.Equal(t, ServerProbeOffline, sorted[1].ProbeStatus)
}

// ─── Scenario 7: No auto-select — single mDNS server stays unselected ───────

func TestIntegration_NoAutoSelectSingleMDNS(t *testing.T) {
	m := newClientSetup()

	// Simulate DiscoveryResultMsg with 1 server
	mdnsEntries := []ServerEntry{
		{Address: "192.168.1.1", Port: 4415, Hostname: "HomeSrv", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(mdnsEntries)

	// No auto-select: user must pick manually
	assert.Equal(t, -1, m.serverList.selected, "nothing should be auto-selected")
	assert.Len(t, m.serverList.Entries(), 1)
}

// ─── Scenario 8: No auto-select with recent servers either ──────────────────

func TestIntegration_NoAutoSelectWhenRecentExists(t *testing.T) {
	recentEntries := []ServerEntry{
		{Address: "10.0.0.1", Port: 4415, Hostname: "Recent", Source: ServerSourceRecent,
			LastConnected: time.Now(), ProbeStatus: ServerProbePending},
	}
	m := newClientSetupWithRecent(recentEntries)

	mdnsNew := []ServerEntry{
		{Address: "10.0.0.2", Port: 4415, Hostname: "MDNSSrv", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(mdnsNew)

	assert.Equal(t, -1, m.serverList.selected, "nothing should be auto-selected")
	assert.Len(t, m.serverList.Entries(), 2)
}

// ─── Scenario 9: Server goes offline after selection ─────────────────────────

func TestIntegration_ServerGoesOfflineAfterSelection(t *testing.T) {
	m := newClientSetup()

	// Add and probe a server
	entries := []ServerEntry{
		{Address: "192.168.1.50", Port: 4415, Hostname: "MySrv", Source: ServerSourceMDNS, ProbeStatus: ServerProbePending},
	}
	m.serverList.MergeDiscoveredEntries(entries)
	m = sendServerProbe(m, "192.168.1.50", 4415, makeProbeResult("normal", false), nil)

	// Select it
	m.serverList.selected = 0
	m, _ = m.Update(ServerSelectedMsg{Entry: m.serverList.Entries()[0]})

	// Verify fields filled
	addrField := fieldByLabel(m.Fields, "Server address")
	require.NotNil(t, addrField)
	assert.Equal(t, "192.168.1.50", addrField.Value)

	// Scan triggered — server goes offline
	m, _ = m.Update(ScanRequestMsg{})
	m = sendServerProbe(m, "192.168.1.50", 4415, nil, fmt.Errorf("connection refused"))

	// Fields should NOT be cleared
	addrField = fieldByLabel(m.Fields, "Server address")
	require.NotNil(t, addrField)
	assert.Equal(t, "192.168.1.50", addrField.Value, "address should not be cleared when server goes offline")
	assert.Equal(t, "⚠ server went offline", addrField.Hint, "hint should warn about offline server")
}

// ─── Scrolling ──────────────────────────────────────────────────────────────

func TestView_ScrollingIndicators(t *testing.T) {
	m := NewServerListModel(80, 24)

	// Add 7 entries (> maxVisibleServers=5)
	var entries []ServerEntry
	for i := 0; i < 7; i++ {
		entries = append(entries, ServerEntry{
			Address:     fmt.Sprintf("10.0.0.%d", i+1),
			Port:        4415,
			Hostname:    fmt.Sprintf("Server%d", i+1),
			Source:      ServerSourceMDNS,
			ProbeStatus: ServerProbeOnline,
		})
	}
	m.SetEntries(entries)

	// Initial view — cursor at 0, scrollOffset at 0
	out := m.View(80)
	assert.NotContains(t, out, "▲", "no up indicator when at top")
	assert.Contains(t, out, "▼", "down indicator when more entries below")

	// Move cursor to bottom
	for i := 0; i < 6; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	out = m.View(80)
	assert.Contains(t, out, "▲", "up indicator when scrolled down")
	assert.NotContains(t, out, "▼", "no down indicator when at bottom")
}

func TestView_NoScrollForFewEntries(t *testing.T) {
	m := NewServerListModel(80, 24)
	entries := []ServerEntry{
		{Address: "1.1.1.1", Port: 4415, Hostname: "A", Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
		{Address: "2.2.2.2", Port: 4415, Hostname: "B", Source: ServerSourceMDNS, ProbeStatus: ServerProbeOnline},
	}
	m.SetEntries(entries)

	out := m.View(80)
	assert.NotContains(t, out, "▲")
	assert.NotContains(t, out, "▼")
}

// ─── Empty state with scanning ──────────────────────────────────────────────

func TestView_EmptyStateScanning(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetScanning(true)

	out := m.View(80)
	assert.Contains(t, out, "Scanning...", "should show 'Scanning...' while scan in progress")
	assert.NotContains(t, out, "No servers found")
}

func TestView_EmptyStateAfterScan(t *testing.T) {
	m := NewServerListModel(80, 24)
	m.SetScanning(false)

	out := m.View(80)
	assert.Contains(t, out, "No servers found", "should show 'No servers found' after scan completes")
	assert.NotContains(t, out, "Scanning...")
}
