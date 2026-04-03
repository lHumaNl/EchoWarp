package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

var banBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("63")).
	Padding(1, 2)

// BanListParams contains parameters for the ban list overlay.
type BanListParams struct {
	BannedIPs     []string
	SelectedIndex int
	Width         int
	Height        int
}

// BanListView renders the ban list overlay centered on the screen.
func BanListView(p BanListParams) string {
	title := styles.AppTitle.Render("Banned IPs")

	var lines []string
	if len(p.BannedIPs) == 0 {
		lines = append(lines, styles.StatLabel.Render("  No banned IPs"))
	} else {
		for i, ip := range p.BannedIPs {
			prefix := "  "
			if i == p.SelectedIndex {
				prefix = styles.SelectedItem.Render(styles.CursorGlyph + " ")
			}
			lines = append(lines, prefix+styles.StatValueGood.Render(ip))
		}
	}

	help := styles.Help.Render("↑↓: select   enter: unban   esc: close")

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n\n" + help

	boxWidth := 44
	if p.Width < boxWidth+6 {
		boxWidth = p.Width - 6
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	box := banBoxStyle.Width(boxWidth).Render(content)

	// Center horizontally
	boxRenderedWidth := lipgloss.Width(box)
	leftPad := (p.Width - boxRenderedWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Center vertically
	boxHeight := lipgloss.Height(box)
	topPad := (p.Height - boxHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	var result strings.Builder
	if topPad > 0 {
		result.WriteString(strings.Repeat("\n", topPad))
	}
	for _, line := range strings.Split(box, "\n") {
		fmt.Fprintf(&result, "%s%s\n", strings.Repeat(" ", leftPad), line)
	}

	return result.String()
}
