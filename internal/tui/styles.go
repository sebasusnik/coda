package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette — https://github.com/catppuccin/catppuccin
const (
	cMauve    = "#cba6f7" // purple — track name, progress filled, prompt
	cLavender = "#b4befe" // blue-purple — artist
	cGreen    = "#a6e3a1" // green — playing state, success status
	cRed      = "#f38ba8" // red — error status
	cPeach    = "#fab387" // orange — paused state
	cOverlay1 = "#7f849c" // mid-gray — album, secondary text
	cSurface1 = "#45475a" // dark — progress bar empty, volume empty
	cSurface2 = "#585b70" // mid-dark — box border
)

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cSurface2)).
			Padding(1, 3)

	compactBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cSurface2)).
			Padding(0, 3)

	trackStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cMauve))
	artistStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cLavender))
	albumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(cOverlay1))
	playStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cGreen))
	pauseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(cPeach))
	barOn       = lipgloss.NewStyle().Foreground(lipgloss.Color(cMauve))
	barOff      = lipgloss.NewStyle().Foreground(lipgloss.Color(cSurface1))
	dim         = lipgloss.NewStyle().Foreground(lipgloss.Color(cOverlay1))
	successSt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cGreen))
	errorSt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cRed))
	promptSt    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cMauve))
)
