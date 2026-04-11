// Package cli provides the command-line interface for EchoWarp using Cobra.
// It defines subcommands for server, client, daemon, devices, config, update, and version.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/version"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
)

// NewRootCmd creates the root command with all subcommands attached.
// The root command handles global flags and delegates to subcommands.
// When run without arguments it launches an interactive quick-start menu.
func NewRootCmd() *cobra.Command {
	i18n.SetLanguage(i18n.LoadLanguage())
	auth.Version = version.Version

	rootCmd := &cobra.Command{
		Use:   "echowarp",
		Short: i18n.T("cli_root_short"),
		Long: fmt.Sprintf(`EchoWarp v%s — network audio streaming tool

EchoWarp is a network tool for real-time audio streaming between two hosts.
It captures audio from a device on one computer and plays it on another in real-time over the network.`, version.Version),
		Example: `  echowarp                           # Interactive quick-start menu
  echowarp server                      # Start server with TUI device selection
  echowarp server -p 4415 -d 0         # Start server on port 4415 with device 0
  echowarp client -a 192.168.1.10      # Connect to server
  echowarp client --discover           # Auto-discover server in LAN
  echowarp devices                     # List audio devices
  echowarp doctor                      # Diagnose environment`,
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          rootRunE,
	}

	rootCmd.AddCommand(
		newVersionCmd(),
		newDevicesCmd(),
		newServerCmd(),
		newClientCmd(),
		newConfigCmd(),
		newDaemonCmd(),
		newUpdateCmd(),
		newCompletionCmd(),
		newDoctorCmd(),
	)

	return rootCmd
}

// rootRunE is executed when echowarp is invoked without a subcommand.
// Launches an interactive quick-start menu.
func rootRunE(cmd *cobra.Command, args []string) error {
	cfgPath := defaultConfigPath()
	hasConfig := false
	if _, err := os.Stat(cfgPath); err == nil {
		hasConfig = true
	}

	m := newQuickStartModel(hasConfig, cfgPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result, ok := finalModel.(quickStartModel)
	if !ok || result.quitting {
		return nil
	}

	return result.executeChoice(cmd)
}

// ─── Quick-start model ────────────────────────────────────────────────────────

type quickStartChoice int

const (
	choiceStartServer quickStartChoice = iota
	choiceStartClient
	choiceConfigShow
	choiceDoctor
	choiceDevices
	choiceHelp
)

type quickStartScreen int

const (
	qsScreenMenu    quickStartScreen = iota
	qsScreenLoading                  // async content loading (e.g. doctor)
	qsScreenResult                   // showing loaded content in a viewport
)

// resultReadyMsg carries async-loaded content for the result screen.
type resultReadyMsg struct {
	title   string
	content string
}

type quickStartItem struct {
	choice quickStartChoice
	title  string
	desc   string
}

func (i quickStartItem) Title() string       { return i.title }
func (i quickStartItem) Description() string { return i.desc }
func (i quickStartItem) FilterValue() string { return i.title }

type qsColumn int

const (
	qsColumnMenu qsColumn = iota
	qsColumnLang
)

type quickStartModel struct {
	screen      quickStartScreen
	list        list.Model
	viewport    viewport.Model
	spinner     spinner.Model
	hasConfig   bool
	cfgPath     string
	selected    *quickStartChoice
	quitting    bool
	resultTitle string
	width       int
	height      int
	column      qsColumn // active column: menu or language
	langCursor  int      // cursor in the language list
}

var (
	resultHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				PaddingLeft(1)

	resultFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				PaddingLeft(1)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2).
			PaddingTop(2)
)

func newQuickStartModel(hasConfig bool, cfgPath string) quickStartModel {
	var items []list.Item

	items = []list.Item{
		quickStartItem{choiceStartServer, i18n.T("qs_server_title"), i18n.T("qs_server_desc")},
		quickStartItem{choiceStartClient, i18n.T("qs_client_title"), i18n.T("qs_client_desc")},
	}
	if hasConfig {
		items = append(items, quickStartItem{choiceConfigShow, i18n.T("qs_config_title"), i18n.T("qs_config_desc")})
	}
	items = append(items,
		quickStartItem{choiceDoctor, i18n.T("qs_doctor_title"), i18n.T("qs_doctor_desc")},
		quickStartItem{choiceDevices, i18n.T("qs_devices_title"), i18n.T("qs_devices_desc")},
		quickStartItem{choiceHelp, i18n.T("qs_help_title"), i18n.T("qs_help_desc")},
	)

	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 60, 14+len(items)*2)
	l.Title = fmt.Sprintf("EchoWarp v%s", version.Version)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.AdditionalShortHelpKeys = nil
	l.AdditionalFullHelpKeys = nil
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)

	if hasConfig {
		l.Title = fmt.Sprintf("EchoWarp v%s — %s", version.Version, shortenHome(cfgPath))
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().PaddingLeft(1)

	// Pre-select current language in side panel
	langIdx := 0
	cur := i18n.CurrentLanguage()
	for idx, lang := range i18n.AvailableLanguages() {
		if lang == cur {
			langIdx = idx
			break
		}
	}

	return quickStartModel{
		screen:     qsScreenMenu,
		list:       l,
		viewport:   vp,
		spinner:    sp,
		hasConfig:  hasConfig,
		cfgPath:    cfgPath,
		width:      80,
		height:     24,
		langCursor: langIdx,
	}
}

func (m quickStartModel) Init() tea.Cmd {
	return nil
}

func (m quickStartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		m.viewport = viewport.New(msg.Width, m.viewportHeight())
		m.viewport.Style = lipgloss.NewStyle().PaddingLeft(1)
		return m, nil

	case resultReadyMsg:
		m.viewport.SetContent(msg.content)
		m.viewport.GotoTop()
		m.resultTitle = msg.title
		m.screen = qsScreenResult
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch m.screen {
		case qsScreenMenu:
			return m.updateMenu(msg)
		case qsScreenLoading:
			// Only allow quit during loading
			if msg.String() == "ctrl+c" || msg.String() == "ctrl+q" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case qsScreenResult:
			return m.updateResult(msg)
		}
	}

	// Pass through to viewport in result screen
	if m.screen == qsScreenResult {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	// Pass through to list in menu screen
	if m.screen == qsScreenMenu {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m quickStartModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab", "right":
		if m.column == qsColumnMenu {
			m.column = qsColumnLang
		} else {
			m.column = qsColumnMenu
		}
		return m, nil
	case "left":
		if m.column == qsColumnLang {
			m.column = qsColumnMenu
		}
		return m, nil
	}

	if m.column == qsColumnLang {
		return m.updateLangPanel(msg)
	}

	if msg.String() == "enter" {
		if item, ok := m.list.SelectedItem().(quickStartItem); ok {
			return m.handleChoice(item.choice)
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m quickStartModel) updateLangPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	langs := i18n.AvailableLanguages()
	switch msg.Type {
	case tea.KeyUp:
		if m.langCursor > 0 {
			m.langCursor--
		}
	case tea.KeyDown:
		if m.langCursor < len(langs)-1 {
			m.langCursor++
		}
	case tea.KeyEnter:
		lang := langs[m.langCursor]
		i18n.SetLanguage(lang)
		_ = i18n.SaveLanguage(lang)
		m = m.rebuildMenuItems()
	}
	return m, nil
}

func (m quickStartModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.screen = qsScreenMenu
		return m, nil
	case "ctrl+q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// handleChoice processes a menu selection.
// Launch choices (server/client) quit TUI; view choices load content inline.
func (m quickStartModel) handleChoice(choice quickStartChoice) (tea.Model, tea.Cmd) {
	switch choice {
	case choiceStartServer, choiceStartClient:
		m.selected = &choice
		return m, tea.Quit

	case choiceDevices:
		content := renderDevicesContent(m.width)
		m.viewport.SetContent(content)
		m.viewport.GotoTop()
		m.resultTitle = i18n.T("layout_result_title_devices")
		m.screen = qsScreenResult
		return m, nil

	case choiceConfigShow:
		content := renderConfigContent(m.cfgPath, m.width)
		m.viewport.SetContent(content)
		m.viewport.GotoTop()
		m.resultTitle = i18n.T("layout_result_title_config")
		m.screen = qsScreenResult
		return m, nil

	case choiceHelp:
		content := renderHelpContent(m.width)
		m.viewport.SetContent(content)
		m.viewport.GotoTop()
		m.resultTitle = i18n.T("layout_result_title_help")
		m.screen = qsScreenResult
		return m, nil

	case choiceDoctor:
		m.screen = qsScreenLoading
		cfgPath := m.cfgPath
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				var sb strings.Builder
				RunDoctorToWriter(&sb, "", 4415, "", cfgPath)
				return resultReadyMsg{title: i18n.T("layout_result_title_diagnostics"), content: sb.String()}
			},
		)
	}

	return m, nil
}

var (
	langPanelTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	langSelected   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	langDim        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func (m quickStartModel) viewMenu() string {
	// Left column: menu list
	menuWidth := m.width*2/3 - 2
	if menuWidth < 40 {
		menuWidth = 40
	}
	m.list.SetSize(menuWidth, m.height-2)
	leftCol := m.list.View()

	// Right column: language panel
	langs := i18n.AvailableLanguages()
	curLang := i18n.CurrentLanguage()

	langLines := make([]string, 0, 2+len(langs))
	langLines = append(langLines, langPanelTitle.Render(i18n.T("lang_overlay_title")), "")
	for idx, lang := range langs {
		name := i18n.DisplayName(lang)
		bullet := "○ "
		if lang == curLang {
			bullet = "● "
		}
		line := bullet + name
		if m.column == qsColumnLang && idx == m.langCursor {
			line = langSelected.Render("▸ " + name)
			if lang == curLang {
				line = langSelected.Render("● " + name)
			}
		} else if lang != curLang {
			line = langDim.Render(line)
		}
		langLines = append(langLines, "  "+line)
	}

	rightCol := strings.Join(langLines, "\n")

	// Join columns side by side
	joined := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(menuWidth).Render(leftCol),
		lipgloss.NewStyle().PaddingLeft(2).Render(rightCol),
	)

	footer := resultFooterStyle.Render(i18n.T("qs_footer"))
	return joined + "\n" + footer
}

// rebuildMenuItems recreates list items with current language strings.
func (m quickStartModel) rebuildMenuItems() quickStartModel {
	var items []list.Item
	items = []list.Item{
		quickStartItem{choiceStartServer, i18n.T("qs_server_title"), i18n.T("qs_server_desc")},
		quickStartItem{choiceStartClient, i18n.T("qs_client_title"), i18n.T("qs_client_desc")},
	}
	if m.hasConfig {
		items = append(items, quickStartItem{choiceConfigShow, i18n.T("qs_config_title"), i18n.T("qs_config_desc")})
	}
	items = append(items,
		quickStartItem{choiceDoctor, i18n.T("qs_doctor_title"), i18n.T("qs_doctor_desc")},
		quickStartItem{choiceDevices, i18n.T("qs_devices_title"), i18n.T("qs_devices_desc")},
		quickStartItem{choiceHelp, i18n.T("qs_help_title"), i18n.T("qs_help_desc")},
	)
	m.list.SetItems(items)
	return m
}

func (m quickStartModel) viewportHeight() int {
	// header (3) + footer (2) = 5 lines reserved
	h := m.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

func (m quickStartModel) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case qsScreenMenu:
		return m.viewMenu()

	case qsScreenLoading:
		return lipgloss.JoinVertical(lipgloss.Left,
			resultHeaderStyle.Render(fmt.Sprintf("EchoWarp v%s", version.Version)),
			"",
			loadingStyle.Render(i18n.Tf("layout_loading_diagnostics", m.spinner.View())),
		)

	case qsScreenResult:
		return m.viewResult()
	}

	return ""
}

func (m quickStartModel) viewResult() string {
	titleBar := resultHeaderStyle.Render(
		fmt.Sprintf("EchoWarp  ›  %s", m.resultTitle),
	)

	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		pct := int(m.viewport.ScrollPercent() * 100)
		scrollInfo = fmt.Sprintf(" %d%%", pct)
	}

	footerText := i18n.T("layout_result_footer") + scrollInfo
	footer := resultFooterStyle.Render(footerText)

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Render(strings.Repeat("─", m.width))

	// Resize viewport to fit available space
	vpHeight := m.height - lipgloss.Height(titleBar) - lipgloss.Height(separator) - lipgloss.Height(footer) - 1
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Height = vpHeight

	return lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		separator,
		m.viewport.View(),
		separator,
		footer,
	)
}

func (m quickStartModel) executeChoice(rootCmd *cobra.Command) error {
	if m.selected == nil {
		return nil
	}

	var subcmdName string
	switch *m.selected {
	case choiceStartServer:
		subcmdName = "server"
	case choiceStartClient:
		subcmdName = "client"
	default:
		return nil
	}

	// Find the real subcommand and invoke its RunE — identical code path
	// as running `echowarp server` or `echowarp client` directly.
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == subcmdName {
			if m.hasConfig {
				_ = sub.Flags().Set("config", m.cfgPath)
			}
			return sub.RunE(sub, nil)
		}
	}
	return fmt.Errorf("subcommand %q not found", subcmdName)
}

// defaultConfigPath returns the platform-default config file path.
func defaultConfigPath() string {
	return filepath.Join(config.EchoWarpDir(), "config.yaml")
}

// shortenHome replaces the user's home directory prefix with "~".
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return "~/" + rel
}
