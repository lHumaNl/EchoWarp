// Package views provides device selection UI components for the TUI.
package views

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// DeviceSelectView renders the audio device selection list body (without header/status bar).
// Content starts at the top — no vertical centering, no border box.
func DeviceSelectView(deviceList list.Model, isInput bool, width, height int) string {
	title := buildDeviceTitle(isInput)
	titleBar := styles.DeviceTitle.Width(width).Render(title)

	// Size the list to fit available body space.
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}
	innerHeight := height - 4
	if innerHeight < 5 {
		innerHeight = 5
	}
	deviceList.SetSize(innerWidth, innerHeight)
	listView := deviceList.View()

	return titleBar + "\n" + listView
}

func buildDeviceTitle(isInput bool) string {
	if isInput {
		return "Select Input Device"
	}
	return "Select Output Device"
}

// DeviceSelectUpdate handles keyboard and mouse events for the device list.
func DeviceSelectUpdate(deviceList list.Model, msg tea.Msg) (list.Model, tea.Cmd) {
	return deviceList.Update(msg)
}
