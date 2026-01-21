package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	dangerColor    = lipgloss.Color("#EF4444") // Red
	warningColor   = lipgloss.Color("#F59E0B") // Orange
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F9FAFB") // White

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	successBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	errorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dangerColor).
			Padding(1, 2)

	// Menu styles
	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedMenuItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(primaryColor).
				Bold(true)

	// Stats styles
	statLabelStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	statValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	// Progress styles
	progressBarStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(mutedColor)

	tableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Key/value styles
	keyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	maskedValueStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	// Status styles
	successStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	// Help styles
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Logo
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)
)

// Logo ASCII art
const logo = `
   _____ _ _     _____                     _
  / ____(_) |   / ____|                   | |
 | |  __ _| |_ | (___   ___  ___ _ __ ___ | |_
 | | |_ | | __|\___ \ / _ \/ __| '__/ _ \| __|
 | |__| | | |_ ____) |  __/ (__| | |  __/| |_
  \_____|_|\__|_____/ \___|\___|_|  \___| \__|
                                Scanner & Cleaner
`

func renderLogo() string {
	return logoStyle.Render(logo)
}
