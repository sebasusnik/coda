package tui

import tea "github.com/charmbracelet/bubbletea"

func Start() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
