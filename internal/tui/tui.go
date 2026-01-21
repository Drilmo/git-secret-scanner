package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Drilmo/git-secret-scanner/internal/config"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// View represents different screens
type View int

const (
	ViewMenu View = iota
	ViewScan
	ViewScanProgress
	ViewScanResults
	ViewAnalyze
	ViewAnalyzeResults
	ViewClean
	ViewCleanConfirm
	ViewCleanProgress
	ViewCleanResults
	ViewTools
	ViewToolsInstall
	ViewConfig
	ViewConfigView
	ViewConfigCreate
	ViewConfigSelect
	ViewConfigBrowse
	ViewScanConfig        // Config screen accessed from Scan form
	ViewScanConfigSelect  // Config select accessed from Scan form
	ViewScanConfigBrowse  // Config browse accessed from Scan form
	ViewAnalyzeProgress   // Analyze progress screen
)

// Model represents the application state
type Model struct {
	view          View
	width         int
	height        int
	menuIndex     int
	spinner       spinner.Model
	form          *huh.Form
	err           error

	// Scan state (pointers for huh form compatibility)
	scanRepoPath     *string
	scanBranch       *string
	scanMode         *string
	scanSource       *string // current, history, both
	scanOutputPath   *string
	scanConfigPath   string
	scanConfigAction string
	scanConfirm      *bool
	scanProgress     int
	scanTotal        int
	scanFound        int
	scanResult       interface{}

	// Analyze state (pointers for huh form compatibility)
	analyzeInputPath   *string
	analyzeOutputPath  *string
	analyzeConfirm     *bool
	analyzeResult      interface{}
	analyzeCsvExported bool

	// Clean state (pointers for huh form compatibility)
	cleanInputPath  *string
	cleanRepoPath   *string
	cleanTool       *string
	cleanDryRun     *bool
	cleanConfirm    *bool
	cleanResult     interface{}

	// Tools state
	toolIndex     int
	installOutput string
	installing    bool

	// Config state
	configIndex       int
	configPath        string
	configCreatePath  string
	configConfirm     *bool
	currentConfig     *config.Config
	configFromScan    bool // Track if config was opened from scan form

	// File browser state
	browseDir     string
	browseIndex   int
	browseEntries []browserEntry
}

type menuItem struct {
	title       string
	description string
}

var menuItems = []menuItem{
	{"Scan Repository", "Search for passwords and secrets in git history"},
	{"Analyze Results", "View statistics, authors, and frequency of changes"},
	{"Clean History", "Remove secrets from git history (rewrite commits)"},
	{"Check Tools", "Verify and install cleaning tools (git-filter-repo, BFG)"},
	{"Quit", "Exit the application"},
}

// New creates a new Model
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return Model{
		view:    ViewMenu,
		spinner: s,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// ctrl+c always quits
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Don't intercept esc when in form views (let the form handle it)
		isFormView := m.view == ViewScan || m.view == ViewAnalyze ||
			m.view == ViewClean || m.view == ViewCleanConfirm ||
			m.view == ViewConfigCreate

		if !isFormView && msg.String() == "esc" {
			if m.view == ViewMenu {
				return m, tea.Quit
			}
			// Special handling for config views accessed from scan
			if m.view == ViewConfigView {
				if m.configFromScan {
					m.view = ViewScanConfig
					return m, nil
				}
				m.view = ViewConfig
				return m, nil
			}
			m.view = ViewMenu
			return m, nil
		}
	}

	// Handle view-specific updates
	switch m.view {
	case ViewMenu:
		return m.updateMenu(msg)
	case ViewScan:
		return m.updateScanForm(msg)
	case ViewScanProgress:
		return m.updateScanProgress(msg)
	case ViewAnalyze:
		return m.updateAnalyzeForm(msg)
	case ViewClean:
		return m.updateCleanForm(msg)
	case ViewCleanConfirm:
		return m.updateCleanConfirm(msg)
	case ViewCleanProgress:
		return m.updateCleanProgress(msg)
	case ViewTools:
		return m.updateTools(msg)
	case ViewToolsInstall:
		return m.updateToolsInstall(msg)
	case ViewConfig:
		return m.updateConfig(msg)
	case ViewConfigCreate:
		return m.updateConfigCreate(msg)
	case ViewConfigSelect:
		return m.updateConfigSelect(msg)
	case ViewConfigBrowse:
		return m.updateConfigBrowse(msg)
	case ViewScanConfig:
		return m.updateScanConfig(msg)
	case ViewScanConfigSelect:
		return m.updateScanConfigSelect(msg)
	case ViewScanConfigBrowse:
		return m.updateScanConfigBrowse(msg)
	case ViewAnalyzeProgress:
		return m.updateAnalyzeProgress(msg)
	}

	return m, nil
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.menuIndex > 0 {
				m.menuIndex--
			}
		case "down", "j":
			if m.menuIndex < len(menuItems)-1 {
				m.menuIndex++
			}
		case "enter":
			return m.handleMenuSelect()
		}
	}
	return m, nil
}

func (m Model) handleMenuSelect() (tea.Model, tea.Cmd) {
	switch m.menuIndex {
	case 0: // Scan
		m.view = ViewScan
		m.form = m.createScanForm()
		return m, m.form.Init()
	case 1: // Analyze
		m.view = ViewAnalyze
		m.form = m.createAnalyzeForm()
		return m, m.form.Init()
	case 2: // Clean
		m.view = ViewClean
		m.form = m.createCleanForm()
		return m, m.form.Init()
	case 3: // Tools
		m.view = ViewTools
		return m, nil
	case 4: // Quit
		return m, tea.Quit
	}
	return m, nil
}

// View renders the UI
func (m Model) View() string {
	switch m.view {
	case ViewMenu:
		return m.viewMenu()
	case ViewScan:
		return m.viewScanForm()
	case ViewScanProgress:
		return m.viewScanProgress()
	case ViewScanResults:
		return m.viewScanResults()
	case ViewAnalyze:
		return m.viewAnalyzeForm()
	case ViewAnalyzeProgress:
		return m.viewAnalyzeProgress()
	case ViewAnalyzeResults:
		return m.viewAnalyzeResults()
	case ViewClean:
		return m.viewCleanForm()
	case ViewCleanConfirm:
		return m.viewCleanConfirm()
	case ViewCleanProgress:
		return m.viewCleanProgress()
	case ViewCleanResults:
		return m.viewCleanResults()
	case ViewTools:
		return m.viewTools()
	case ViewToolsInstall:
		return m.viewToolsInstall()
	case ViewConfig:
		return m.viewConfig()
	case ViewConfigView:
		return m.viewConfigView()
	case ViewConfigCreate:
		return m.viewConfigCreate()
	case ViewConfigSelect:
		return m.viewConfigSelect()
	case ViewConfigBrowse:
		return m.viewConfigBrowse()
	case ViewScanConfig:
		return m.viewScanConfig()
	case ViewScanConfigSelect:
		return m.viewScanConfigSelect()
	case ViewScanConfigBrowse:
		return m.viewScanConfigBrowse()
	default:
		return "Unknown view"
	}
}

func (m Model) viewMenu() string {
	var sb strings.Builder

	sb.WriteString(renderLogo())
	sb.WriteString("\n\n")

	// Menu items
	for i, item := range menuItems {
		cursor := "  "
		style := menuItemStyle
		if i == m.menuIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		title := style.Render(cursor + item.title)
		desc := subtitleStyle.Render("  " + item.description)
		sb.WriteString(title + "\n" + desc + "\n\n")
	}

	// Help
	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: quit")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

type toolInfo struct {
	name        string
	check       func() bool
	desc        string
	installCmds []installCmd
}

type installCmd struct {
	name    string
	cmd     string
	args    []string
}

var availableTools = []toolInfo{
	{
		name:  "git-filter-repo",
		check: hasFilterRepo,
		desc:  "Recommended - Fast and safe",
		installCmds: []installCmd{
			{"Homebrew (macOS)", "brew", []string{"install", "git-filter-repo"}},
			{"pip (Python)", "pip", []string{"install", "git-filter-repo"}},
			{"pip3 (Python 3)", "pip3", []string{"install", "git-filter-repo"}},
			{"apt (Ubuntu/Debian)", "sudo", []string{"apt", "install", "-y", "git-filter-repo"}},
		},
	},
	{
		name:  "bfg",
		check: hasBFG,
		desc:  "Alternative - Java based",
		installCmds: []installCmd{
			{"Homebrew (macOS)", "brew", []string{"install", "bfg"}},
		},
	},
	{
		name:  "git-filter-branch",
		check: func() bool { return true },
		desc:  "Built-in - Slow but always available",
		installCmds: nil,
	},
}

func (m Model) updateTools(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.toolIndex > 0 {
				m.toolIndex--
			}
		case "down", "j":
			if m.toolIndex < len(availableTools)-1 {
				m.toolIndex++
			}
		case "enter", "i":
			tool := availableTools[m.toolIndex]
			if !tool.check() && len(tool.installCmds) > 0 {
				m.view = ViewToolsInstall
				m.installOutput = ""
				m.installing = false
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) viewTools() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🔧 Available Tools"))
	sb.WriteString("\n\n")

	for i, tool := range availableTools {
		cursor := "  "
		style := menuItemStyle
		if i == m.toolIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		status := errorStyle.Render("✗ Not installed")
		installHint := ""
		if tool.check() {
			status = successStyle.Render("✓ Installed")
		} else if len(tool.installCmds) > 0 {
			installHint = lipgloss.NewStyle().Foreground(mutedColor).Render(" (press Enter to install)")
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, tool.name)) + installHint + "\n")
		sb.WriteString(fmt.Sprintf("    %s\n", tool.desc))
		sb.WriteString(fmt.Sprintf("    Status: %s\n\n", status))
	}

	help := helpStyle.Render("↑/↓: navigate • enter: install • esc: back")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

// Install tool messages
type installStartMsg struct {
	cmdName string
}
type installDoneMsg struct {
	success bool
	output  string
	err     error
}

func (m Model) updateToolsInstall(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.installing {
			return m, nil // Ignore keys while installing
		}
		switch msg.String() {
		case "up", "k":
			tool := availableTools[m.toolIndex]
			installIdx := m.getInstallIndex()
			if installIdx > 0 {
				m.setInstallIndex(installIdx - 1)
			}
			_ = tool
		case "down", "j":
			tool := availableTools[m.toolIndex]
			installIdx := m.getInstallIndex()
			if installIdx < len(tool.installCmds)-1 {
				m.setInstallIndex(installIdx + 1)
			}
		case "enter":
			tool := availableTools[m.toolIndex]
			installIdx := m.getInstallIndex()
			if installIdx < len(tool.installCmds) {
				m.installing = true
				m.installOutput = "Installing..."
				return m, tea.Batch(m.spinner.Tick, m.runInstall(tool.installCmds[installIdx]))
			}
		case "esc":
			m.view = ViewTools
			m.installOutput = ""
			return m, nil
		}

	case installDoneMsg:
		m.installing = false
		if msg.success {
			m.installOutput = successStyle.Render("✓ Installation successful!\n\n") + msg.output
		} else {
			errMsg := ""
			if msg.err != nil {
				errMsg = msg.err.Error()
			}
			m.installOutput = errorStyle.Render("✗ Installation failed\n\n") + errMsg + "\n" + msg.output
		}
		return m, nil

	default:
		if m.installing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// Store install index in a simple way (reuse scanProgress as temp storage)
func (m *Model) getInstallIndex() int {
	return m.scanProgress
}

func (m *Model) setInstallIndex(idx int) {
	m.scanProgress = idx
}

func (m *Model) runInstall(cmd installCmd) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command(cmd.cmd, cmd.args...)
		output, err := c.CombinedOutput()

		if err != nil {
			return installDoneMsg{
				success: false,
				output:  string(output),
				err:     err,
			}
		}

		return installDoneMsg{
			success: true,
			output:  string(output),
		}
	}
}

func (m Model) viewToolsInstall() string {
	var sb strings.Builder

	tool := availableTools[m.toolIndex]
	sb.WriteString(titleStyle.Render(fmt.Sprintf("📦 Install %s", tool.name)))
	sb.WriteString("\n\n")

	if m.installing {
		sb.WriteString(m.spinner.View() + " Installing...\n")
	} else if m.installOutput != "" {
		sb.WriteString(m.installOutput)
		sb.WriteString("\n\n")
		help := helpStyle.Render("esc: back to tools")
		sb.WriteString(help)
	} else {
		sb.WriteString("Select installation method:\n\n")

		// Filter install commands by OS
		installIdx := m.getInstallIndex()
		for i, cmd := range tool.installCmds {
			// Check if command is likely available
			available := isCommandAvailable(cmd.cmd)

			cursor := "  "
			style := menuItemStyle
			if i == installIdx {
				cursor = "▸ "
				style = selectedMenuItemStyle
			}

			status := ""
			if !available {
				status = lipgloss.NewStyle().Foreground(mutedColor).Render(" (not found)")
			}

			sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, cmd.name)) + status + "\n")
			sb.WriteString(fmt.Sprintf("    %s %s\n\n", cmd.cmd, strings.Join(cmd.args, " ")))
		}

		help := helpStyle.Render("↑/↓: select • enter: install • esc: back")
		sb.WriteString("\n" + help)
	}

	return boxStyle.Render(sb.String())
}

func isCommandAvailable(cmd string) bool {
	// sudo is always "available" in the sense we can try
	if cmd == "sudo" {
		return true
	}

	_, err := exec.LookPath(cmd)
	return err == nil
}

func getOS() string {
	return runtime.GOOS
}

type configMenuItem struct {
	title string
	desc  string
}

var configMenuItems = []configMenuItem{
	{"View Current", "See loaded configuration and patterns"},
	{"Create New", "Create a new configuration file"},
	{"Select Config", "Choose a configuration file to use"},
}

func (m Model) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.configIndex > 0 {
				m.configIndex--
			}
		case "down", "j":
			if m.configIndex < len(configMenuItems)-1 {
				m.configIndex++
			}
		case "enter":
			switch m.configIndex {
			case 0: // View
				m.view = ViewConfigView
				// Load current config
				cfg, _ := config.Load(m.configPath)
				m.currentConfig = cfg
			case 1: // Create
				m.view = ViewConfigCreate
				if m.configCreatePath == "" {
					m.configCreatePath = "patterns.json"
				}
				m.form = m.createConfigForm()
				return m, m.form.Init()
			case 2: // Select
				m.view = ViewConfigSelect
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewConfig() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("⚙️  Configuration"))
	sb.WriteString("\n\n")

	// Show current config
	if m.configPath != "" {
		sb.WriteString(fmt.Sprintf("Current: %s\n\n", successStyle.Render(m.configPath)))
	} else {
		sb.WriteString(fmt.Sprintf("Current: %s\n\n", lipgloss.NewStyle().Foreground(mutedColor).Render("Built-in defaults")))
	}

	// Menu
	for i, item := range configMenuItems {
		cursor := "  "
		style := menuItemStyle
		if i == m.configIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, item.title)) + "\n")
		sb.WriteString(fmt.Sprintf("    %s\n\n", item.desc))
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

func (m Model) viewConfigView() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📋 Current Configuration"))
	sb.WriteString("\n\n")

	if m.currentConfig == nil {
		sb.WriteString("No configuration loaded.\n")
	} else {
		// Keywords
		sb.WriteString(keyStyle.Render("Keywords Groups:") + "\n")
		for _, kw := range m.currentConfig.Keywords {
			sb.WriteString(fmt.Sprintf("  • %s (%d patterns)\n", kw.Name, len(kw.Patterns)))
		}
		sb.WriteString("\n")

		// Settings
		sb.WriteString(keyStyle.Render("Settings:") + "\n")
		sb.WriteString(fmt.Sprintf("  Min length: %d\n", m.currentConfig.Settings.MinSecretLength))
		sb.WriteString(fmt.Sprintf("  Max length: %d\n", m.currentConfig.Settings.MaxSecretLength))
		sb.WriteString(fmt.Sprintf("  Case sensitive: %v\n", m.currentConfig.Settings.CaseSensitive))
		sb.WriteString("\n")

		// Ignored values
		sb.WriteString(keyStyle.Render("Ignored Values:") + "\n")
		for i, v := range m.currentConfig.IgnoredValues {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.currentConfig.IgnoredValues)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("  • %s\n", v))
		}
	}

	help := helpStyle.Render("esc: back")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

func (m *Model) createConfigForm() *huh.Form {
	// Allocate pointer for confirm (shared across Model copies)
	// Default to false (Cancel) - user must explicitly choose to create
	confirm := false
	m.configConfirm = &confirm
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Configuration File Path").
				Description("Where to save the new configuration").
				Value(&m.configCreatePath),

			huh.NewConfirm().
				Title("Create configuration file?").
				Affirmative("Create").
				Negative("Cancel").
				Value(m.configConfirm),
		),
	).WithTheme(huh.ThemeDracula())
}

func (m Model) updateConfigCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle ESC to go back to config menu
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		if m.configFromScan {
			m.view = ViewScanConfig
		} else {
			m.view = ViewConfig
		}
		return m, nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		if m.configConfirm != nil && *m.configConfirm {
			// Create default config file
			cfg := config.DefaultConfig()
			if err := cfg.Save(m.configCreatePath); err != nil {
				m.err = err
			} else {
				m.configPath = m.configCreatePath
				m.currentConfig = cfg
			}
		}
		// Return to appropriate view
		if m.configFromScan {
			m.configFromScan = false
			m.view = ViewScan
			m.form = m.createScanForm()
			return m, m.form.Init()
		}
		m.view = ViewConfig
		return m, nil
	}

	if m.form.State == huh.StateAborted {
		if m.configFromScan {
			m.view = ViewScanConfig
		} else {
			m.view = ViewConfig
		}
	}

	return m, cmd
}

func (m Model) viewConfigCreate() string {
	return boxStyle.Render(
		titleStyle.Render("📝 Create Configuration") + "\n\n" +
			m.form.View(),
	)
}

func (m Model) updateConfigSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	configs := m.findConfigFiles()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			idx := m.getConfigSelectIndex()
			if idx > 0 {
				m.setConfigSelectIndex(idx - 1)
			}
		case "down", "j":
			idx := m.getConfigSelectIndex()
			if idx < len(configs) {
				m.setConfigSelectIndex(idx + 1)
			}
		case "enter":
			idx := m.getConfigSelectIndex()
			if idx == len(configs) {
				// Browse option
				cwd, _ := os.Getwd()
				m.browseDir = cwd
				m.browseIndex = 0
				m.loadBrowseEntries()
				m.view = ViewConfigBrowse
				return m, nil
			}
			if idx < len(configs) {
				selected := configs[idx]
				if selected == "(Built-in defaults)" {
					m.configPath = ""
					m.currentConfig = config.DefaultConfig()
				} else {
					m.configPath = selected
					cfg, _ := config.Load(selected)
					m.currentConfig = cfg
				}
				m.view = ViewConfig
			}
			return m, nil
		case "esc":
			m.view = ViewConfig
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) getConfigSelectIndex() int {
	return m.scanTotal // Reuse scanTotal as temp storage
}

func (m *Model) setConfigSelectIndex(idx int) {
	m.scanTotal = idx
}

func (m Model) findConfigFiles() []string {
	configs := []string{"(Built-in defaults)"}

	// Check common locations
	locations := []string{
		"patterns.json",
		"config/patterns.json",
	}

	// Add home config
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".config", "git-secret-scanner", "patterns.json"))
	}

	// Find all .json files in current directory
	files, _ := filepath.Glob("*.json")
	for _, f := range files {
		if !contains(locations, f) && f != "package.json" && f != "package-lock.json" {
			locations = append(locations, f)
		}
	}

	// Check which exist
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			configs = append(configs, loc)
		}
	}

	return configs
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (m Model) viewConfigSelect() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📂 Select Configuration"))
	sb.WriteString("\n\n")

	configs := m.findConfigFiles()
	idx := m.getConfigSelectIndex()

	for i, cfg := range configs {
		cursor := "  "
		style := menuItemStyle
		if i == idx {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		// Mark current
		current := ""
		if (cfg == "(Built-in defaults)" && m.configPath == "") ||
			cfg == m.configPath {
			current = successStyle.Render(" (current)")
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, cfg)) + current + "\n")
	}

	// Browse option
	cursor := "  "
	style := menuItemStyle
	if idx == len(configs) {
		cursor = "▸ "
		style = selectedMenuItemStyle
	}
	sb.WriteString("\n" + style.Render(fmt.Sprintf("%s📁 Browse...", cursor)) + "\n")

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

// Browser entry types
type browserEntry struct {
	name  string
	isDir bool
	path  string
}

func (m *Model) loadBrowseEntries() {
	m.browseEntries = []browserEntry{}

	// Add parent directory if not at root
	if m.browseDir != "/" {
		m.browseEntries = append(m.browseEntries, browserEntry{
			name:  "..",
			isDir: true,
			path:  filepath.Dir(m.browseDir),
		})
	}

	entries, err := os.ReadDir(m.browseDir)
	if err != nil {
		return
	}

	// Directories first
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			m.browseEntries = append(m.browseEntries, browserEntry{
				name:  e.Name(),
				isDir: true,
				path:  filepath.Join(m.browseDir, e.Name()),
			})
		}
	}

	// Then JSON files
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			m.browseEntries = append(m.browseEntries, browserEntry{
				name:  e.Name(),
				isDir: false,
				path:  filepath.Join(m.browseDir, e.Name()),
			})
		}
	}
}

func (m Model) updateConfigBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.browseIndex > 0 {
				m.browseIndex--
			}
		case "down", "j":
			if m.browseIndex < len(m.browseEntries)-1 {
				m.browseIndex++
			}
		case "enter":
			if m.browseIndex < len(m.browseEntries) {
				entry := m.browseEntries[m.browseIndex]
				if entry.isDir {
					// Navigate into directory
					m.browseDir = entry.path
					m.browseIndex = 0
					m.loadBrowseEntries()
				} else {
					// Select file
					m.configPath = entry.path
					cfg, _ := config.Load(entry.path)
					m.currentConfig = cfg
					m.view = ViewConfig
				}
			}
			return m, nil
		case "backspace":
			// Go up one directory
			if m.browseDir != "/" {
				m.browseDir = filepath.Dir(m.browseDir)
				m.browseIndex = 0
				m.loadBrowseEntries()
			}
			return m, nil
		case "esc":
			m.view = ViewConfigSelect
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewConfigBrowse() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📁 Browse Files"))
	sb.WriteString("\n\n")

	// Current path
	sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("Path: "))
	sb.WriteString(m.browseDir)
	sb.WriteString("\n\n")

	// Entries
	maxVisible := 15
	startIdx := 0
	if m.browseIndex >= maxVisible {
		startIdx = m.browseIndex - maxVisible + 1
	}

	for i := startIdx; i < len(m.browseEntries) && i < startIdx+maxVisible; i++ {
		entry := m.browseEntries[i]
		cursor := "  "
		style := menuItemStyle
		if i == m.browseIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		icon := "📄"
		if entry.isDir {
			icon = "📁"
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s %s", cursor, icon, entry.name)) + "\n")
	}

	if len(m.browseEntries) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("  (no JSON files or directories)") + "\n")
	}

	if len(m.browseEntries) > maxVisible {
		sb.WriteString(fmt.Sprintf("\n  ... %d/%d items", m.browseIndex+1, len(m.browseEntries)))
	}

	help := helpStyle.Render("↑/↓: navigate • enter: open/select • backspace: up • esc: back")
	sb.WriteString("\n\n" + help)

	return boxStyle.Render(sb.String())
}

// Scan Config handlers (config accessed from scan form)
func (m Model) updateScanConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.configIndex > 0 {
				m.configIndex--
			}
		case "down", "j":
			if m.configIndex < len(configMenuItems)-1 {
				m.configIndex++
			}
		case "enter":
			switch m.configIndex {
			case 0: // View
				m.configFromScan = true
				m.view = ViewConfigView
				cfg, _ := config.Load(m.configPath)
				m.currentConfig = cfg
			case 1: // Create
				m.configFromScan = true
				m.view = ViewConfigCreate
				if m.configCreatePath == "" {
					m.configCreatePath = "patterns.json"
				}
				m.form = m.createConfigForm()
				return m, m.form.Init()
			case 2: // Select
				m.view = ViewScanConfigSelect
			}
			return m, nil
		case "esc":
			// Return to scan form
			m.configFromScan = false
			m.view = ViewScan
			m.form = m.createScanForm()
			return m, m.form.Init()
		}
	}
	return m, nil
}

func (m Model) viewScanConfig() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("⚙️  Pattern Configuration"))
	sb.WriteString("\n\n")

	// Show current config
	if m.configPath != "" {
		sb.WriteString(fmt.Sprintf("Current: %s\n\n", successStyle.Render(m.configPath)))
	} else {
		sb.WriteString(fmt.Sprintf("Current: %s\n\n", lipgloss.NewStyle().Foreground(mutedColor).Render("Built-in defaults")))
	}

	// Menu
	for i, item := range configMenuItems {
		cursor := "  "
		style := menuItemStyle
		if i == m.configIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, item.title)) + "\n")
		sb.WriteString(fmt.Sprintf("    %s\n\n", item.desc))
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back to scan")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

func (m Model) updateScanConfigSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	configs := m.findConfigFiles()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			idx := m.getConfigSelectIndex()
			if idx > 0 {
				m.setConfigSelectIndex(idx - 1)
			}
		case "down", "j":
			idx := m.getConfigSelectIndex()
			if idx < len(configs) {
				m.setConfigSelectIndex(idx + 1)
			}
		case "enter":
			idx := m.getConfigSelectIndex()
			if idx == len(configs) {
				// Browse option
				cwd, _ := os.Getwd()
				m.browseDir = cwd
				m.browseIndex = 0
				m.loadBrowseEntries()
				m.view = ViewScanConfigBrowse
				return m, nil
			}
			if idx < len(configs) {
				selected := configs[idx]
				if selected == "(Built-in defaults)" {
					m.configPath = ""
					m.currentConfig = config.DefaultConfig()
				} else {
					m.configPath = selected
					cfg, _ := config.Load(selected)
					m.currentConfig = cfg
				}
				// Return to scan form with updated config
				m.view = ViewScan
				m.form = m.createScanForm()
				return m, m.form.Init()
			}
			return m, nil
		case "esc":
			m.view = ViewScanConfig
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewScanConfigSelect() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📂 Select Configuration"))
	sb.WriteString("\n\n")

	configs := m.findConfigFiles()
	idx := m.getConfigSelectIndex()

	for i, cfg := range configs {
		cursor := "  "
		style := menuItemStyle
		if i == idx {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		// Mark current
		current := ""
		if (cfg == "(Built-in defaults)" && m.configPath == "") ||
			cfg == m.configPath {
			current = successStyle.Render(" (current)")
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, cfg)) + current + "\n")
	}

	// Browse option
	cursor := "  "
	style := menuItemStyle
	if idx == len(configs) {
		cursor = "▸ "
		style = selectedMenuItemStyle
	}
	sb.WriteString("\n" + style.Render(fmt.Sprintf("%s📁 Browse...", cursor)) + "\n")

	help := helpStyle.Render("↑/↓: navigate • enter: select • esc: back")
	sb.WriteString("\n" + help)

	return boxStyle.Render(sb.String())
}

func (m Model) updateScanConfigBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.browseIndex > 0 {
				m.browseIndex--
			}
		case "down", "j":
			if m.browseIndex < len(m.browseEntries)-1 {
				m.browseIndex++
			}
		case "enter":
			if m.browseIndex < len(m.browseEntries) {
				entry := m.browseEntries[m.browseIndex]
				if entry.isDir {
					// Navigate into directory
					m.browseDir = entry.path
					m.browseIndex = 0
					m.loadBrowseEntries()
				} else {
					// Select file and return to scan form
					m.configPath = entry.path
					cfg, _ := config.Load(entry.path)
					m.currentConfig = cfg
					m.view = ViewScan
					m.form = m.createScanForm()
					return m, m.form.Init()
				}
			}
			return m, nil
		case "backspace":
			// Go up one directory
			if m.browseDir != "/" {
				m.browseDir = filepath.Dir(m.browseDir)
				m.browseIndex = 0
				m.loadBrowseEntries()
			}
			return m, nil
		case "esc":
			m.view = ViewScanConfigSelect
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewScanConfigBrowse() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("📁 Browse Files"))
	sb.WriteString("\n\n")

	// Current path
	sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("Path: "))
	sb.WriteString(m.browseDir)
	sb.WriteString("\n\n")

	// Entries
	maxVisible := 15
	startIdx := 0
	if m.browseIndex >= maxVisible {
		startIdx = m.browseIndex - maxVisible + 1
	}

	for i := startIdx; i < len(m.browseEntries) && i < startIdx+maxVisible; i++ {
		entry := m.browseEntries[i]
		cursor := "  "
		style := menuItemStyle
		if i == m.browseIndex {
			cursor = "▸ "
			style = selectedMenuItemStyle
		}

		icon := "📄"
		if entry.isDir {
			icon = "📁"
		}

		sb.WriteString(style.Render(fmt.Sprintf("%s%s %s", cursor, icon, entry.name)) + "\n")
	}

	if len(m.browseEntries) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("  (no JSON files or directories)") + "\n")
	}

	if len(m.browseEntries) > maxVisible {
		sb.WriteString(fmt.Sprintf("\n  ... %d/%d items", m.browseIndex+1, len(m.browseEntries)))
	}

	help := helpStyle.Render("↑/↓: navigate • enter: open/select • backspace: up • esc: back")
	sb.WriteString("\n\n" + help)

	return boxStyle.Render(sb.String())
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
