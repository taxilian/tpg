package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/model"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	statusColors = map[model.Status]lipgloss.Color{
		model.StatusOpen:       lipgloss.Color("252"),
		model.StatusInProgress: lipgloss.Color("214"),
		model.StatusBlocked:    lipgloss.Color("196"),
		model.StatusDone:       lipgloss.Color("42"),
		model.StatusCanceled:   lipgloss.Color("245"),
	}

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	detailLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("147"))

	staleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true)

	selectModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(true)

	// Content area padding
	contentPadding = 2

	// Reserved rows for view chrome
	listReservedRows     = 6 // header (2) + footer (3) + padding (1)
	templateReservedRows = 8
	detailReservedRows   = 3 // help line + scroll indicator + padding
)
