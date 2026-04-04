package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

// DiscoveryResultMsg is sent when mDNS discovery completes.
type DiscoveryResultMsg struct {
	Servers []discovery.ServiceInfo
	Err     error
}

// StartDiscoveryCmd returns a tea.Cmd that runs mDNS discovery in the background.
func StartDiscoveryCmd(timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		disc := discovery.NewDiscoverer()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		ch, err := disc.Discover(ctx, timeout)
		if err != nil {
			return DiscoveryResultMsg{Err: err}
		}

		var servers []discovery.ServiceInfo
		for s := range ch {
			servers = append(servers, s)
		}

		return DiscoveryResultMsg{Servers: servers}
	}
}

// ApplyDiscoveredServer fills setup fields from a discovered server.
func ApplyDiscoveredServer(fields []SetupField, advFields []SetupField, server discovery.ServiceInfo) ([]SetupField, []SetupField) {
	ip := ""
	if len(server.AddrIPv4) > 0 {
		ip = server.AddrIPv4[0].String()
	} else if len(server.AddrIPv6) > 0 {
		ip = server.AddrIPv6[0].String()
	}

	for i := range fields {
		switch fields[i].Label {
		case "Server address":
			fields[i].SetValue(ip, SourceAuto)
		case "Port":
			fields[i].SetValue(fmt.Sprintf("%d", server.Port), SourceAuto)
		case "Password":
			if server.AuthReq {
				fields[i].Hint = "(required)"
			}
		}
	}

	for i := range advFields {
		switch advFields[i].Label {
		case "Mode":
			mode := server.Mode
			if mode == "" {
				mode = "normal"
			}
			if desc, ok := modeKeyToDescriptive[mode]; ok {
				advFields[i].SetValue(desc, SourceAuto)
			} else {
				advFields[i].SetValue(mode, SourceAuto)
			}
		case "TLS":
			if server.TLS {
				advFields[i].SetValue("on", SourceAuto)
			} else {
				advFields[i].SetValue("off", SourceAuto)
			}
		}
	}

	return fields, advFields
}

// DiscoveryAddressHint returns the hint text for the server address field during/after discovery.
func DiscoveryAddressHint(scanning bool, serverCount int, singleServerName string) string {
	if scanning {
		return "⠋ scanning..."
	}
	if serverCount == 1 && singleServerName != "" {
		return "(discovered: " + singleServerName + ")"
	}
	if serverCount > 1 {
		return fmt.Sprintf("[tab or space: choose from %d found]", serverCount)
	}
	return ""
}
