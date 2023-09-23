package ux

import "github.com/charmbracelet/lipgloss"

func Error() lipgloss.Style {
	return lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#E88388"))
}
