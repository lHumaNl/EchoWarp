package views

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/probe"
)

// ProbeServerResult is an alias for probe.ProbeServerResult so existing TUI code compiles unchanged.
type ProbeServerResult = probe.ProbeServerResult

// ProbeServerMsg is sent when a server probe completes.
type ProbeServerMsg struct {
	Result *ProbeServerResult // nil on failure
	Err    error
	Addr   string // address that was probed (to discard stale results)
}

// ProbeDebounceMsg is sent after the debounce timer to trigger the actual probe.
type ProbeDebounceMsg struct {
	Addr string
	Port string
}

// ProbeDebounceDelay is the delay before probing the server after address/port change.
const ProbeDebounceDelay = 500 * time.Millisecond

// StartProbeDebounceCmd returns a tea.Cmd that fires ProbeDebounceMsg after a delay.
func StartProbeDebounceCmd(addr, port string) tea.Cmd {
	return tea.Tick(ProbeDebounceDelay, func(time.Time) tea.Msg {
		return ProbeDebounceMsg{Addr: addr, Port: port}
	})
}

// ProbeServer delegates to probe.ProbeServer.
// Kept here for backward compatibility with TUI code.
func ProbeServer(addr string, port int) (*ProbeServerResult, error) {
	return probe.ProbeServer(addr, port)
}

// StartProbeCmd returns a tea.Cmd that probes the server's session info endpoint.
// This is a thin wrapper around ProbeServer for use in Bubble Tea programs.
func StartProbeCmd(addr, port string) tea.Cmd {
	return func() tea.Msg {
		target := fmt.Sprintf("%s:%s", addr, port)
		portNum, _ := strconv.Atoi(port) //nolint:errcheck
		result, err := ProbeServer(addr, portNum)
		if err != nil {
			return ProbeServerMsg{Err: err, Addr: target}
		}
		return ProbeServerMsg{Result: result, Addr: target}
	}
}

// ApplyProbeResult fills setup fields from a successful server probe.
// Mode becomes read-only (SourceAuto). Returns updated isDuplexMode.
func ApplyProbeResult(fields []SetupField, advFields []SetupField, result *ProbeServerResult) ([]SetupField, []SetupField, bool) {
	isDuplex := result.Mode == "duplex" || result.Mode == "conference"
	for i := range fields {
		switch fields[i].Key {
		case "password":
			if result.PasswordRequired {
				fields[i].Hidden = false
				fields[i].Hint = "⚠ required by server"
				fields[i].Required = true
			} else {
				fields[i].Hidden = true
				fields[i].Hint = "(not required)"
				fields[i].Required = false
				fields[i].Value = ""
			}
		case "nickname":
			if result.HWIDRequired {
				fields[i].Hint = "\u26a0 Server collects device ID (HWID) — may be used for banning"
			}
		case "mode":
			modeKey := result.Mode
			if desc, ok := modeKeyToDescriptiveMap()[modeKey]; ok {
				fields[i].SetValue(desc, SourceAuto)
			} else {
				fields[i].SetValue(result.Mode, SourceAuto)
			}
			fields[i].Hint = i18n.T("hint_server")
			isDuplex = modeKey == "duplex" || modeKey == "conference"
		}
	}

	for i := range advFields {
		switch advFields[i].Key {
		case "tls":
			if result.TLSRequired {
				advFields[i].SetValue("on", SourceAuto)
				if result.TLSSelfSigned {
					advFields[i].Hint = i18n.T("hint_server") + " " + i18n.T("hint_tls_self_signed")
				} else {
					advFields[i].Hint = i18n.T("hint_server")
				}
			} else {
				advFields[i].SetValue("off", SourceAuto)
				advFields[i].Hint = i18n.T("hint_server")
			}
		case "sample_rate":
			if result.SampleRate > 0 {
				advFields[i].SetValue(FormatSampleRate(result.SampleRate), SourceAuto)
				advFields[i].Hint = i18n.T("hint_server")
			}
		case "channels":
			switch result.Channels {
			case 2:
				advFields[i].SetValue("stereo", SourceAuto)
			case 1:
				advFields[i].SetValue("mono", SourceAuto)
			}
			if result.Channels > 0 {
				advFields[i].Hint = i18n.T("hint_server")
			}
		case "opus_bitrate":
			if result.OpusBitrate > 0 {
				advFields[i].SetValue(FormatBitrate(result.OpusBitrate), SourceAuto)
				advFields[i].Hint = i18n.T("hint_server")
			}
		}
	}

	return fields, advFields, isDuplex
}

// ServerProbeMsg is sent when a parallel server probe completes.
type ServerProbeMsg struct {
	Address string // "addr:port" identifier
	Result  *probe.ProbeServerResult
	Err     error
}

// ServerProbeSpinnerMsg triggers a spinner frame advance for probing servers.
type ServerProbeSpinnerMsg struct{}

// StartServerProbeCmd returns a tea.Cmd that probes a single server with a 2s timeout
// and returns a ServerProbeMsg.
func StartServerProbeCmd(addr string, port int) tea.Cmd {
	return func() tea.Msg {
		id := fmt.Sprintf("%s:%d", addr, port)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		result, err := probe.ProbeServerWithContext(ctx, addr, port)
		if err != nil {
			return ServerProbeMsg{Address: id, Err: err}
		}
		return ServerProbeMsg{Address: id, Result: result}
	}
}

// StartAllProbesCmd returns a tea.Batch that probes all given server entries in parallel.
func StartAllProbesCmd(entries []ServerEntry) tea.Cmd {
	cmds := make([]tea.Cmd, len(entries))
	for i, e := range entries {
		cmds[i] = StartServerProbeCmd(e.Address, e.Port)
	}
	return tea.Batch(cmds...)
}
