package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds every style the viewer uses. Colours are adaptive: lipgloss
// detects whether the terminal has a light or dark background and picks the
// matching value, so the diff backgrounds stay legible either way.
type Theme struct {
	PaneStyle       lipgloss.Style
	StatusBarStyle  lipgloss.Style
	EmptyStateStyle lipgloss.Style
	HintKeyStyle    lipgloss.Style
	HintDescStyle   lipgloss.Style
	HintBarStyle    lipgloss.Style
	ErrorStyle      lipgloss.Style
	LabelBoldStyle  lipgloss.Style

	// Diff content styles.
	HunkHeaderStyle lipgloss.Style
	AddedStyle      lipgloss.Style
	RemovedStyle    lipgloss.Style
	FileHeaderStyle lipgloss.Style
	DeletedMsgStyle lipgloss.Style
	RenamedMsgStyle lipgloss.Style
	LineSelectStyle lipgloss.Style
}

// Adaptive colour values, light background first. The light values are tuned
// for white terminals (pastel backgrounds, dark foregrounds) and the dark
// values for black ones (dark-tinted backgrounds, bright foregrounds).
var (
	colourBorder     = lipgloss.AdaptiveColor{Light: "25", Dark: "24"}
	colourAccent     = lipgloss.AdaptiveColor{Light: "30", Dark: "37"}
	colourOnAccent   = lipgloss.AdaptiveColor{Light: "255", Dark: "16"}
	colourMuted      = lipgloss.AdaptiveColor{Light: "244", Dark: "245"}
	colourSubtle     = lipgloss.AdaptiveColor{Light: "243", Dark: "240"}
	colourDanger     = lipgloss.AdaptiveColor{Light: "124", Dark: "167"}
	colourHunkHeader = lipgloss.AdaptiveColor{Light: "189", Dark: "24"}
	colourAdded      = lipgloss.AdaptiveColor{Light: "194", Dark: "22"}
	colourRemoved    = lipgloss.AdaptiveColor{Light: "224", Dark: "52"}
	colourDeleted    = lipgloss.AdaptiveColor{Light: "88", Dark: "167"}
	colourRenamed    = lipgloss.AdaptiveColor{Light: "25", Dark: "75"}
)

// theme is the single instance used throughout the app. It is built once at
// init because lipgloss styles are immutable values, not resources.
var theme = Theme{
	PaneStyle: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colourBorder),
	StatusBarStyle: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colourBorder).
		BorderBottom(false),
	EmptyStateStyle: lipgloss.NewStyle().
		Foreground(colourSubtle).
		Italic(true),
	HintKeyStyle:    lipgloss.NewStyle().Foreground(colourMuted).Bold(true),
	HintDescStyle:   lipgloss.NewStyle().Foreground(colourMuted),
	HintBarStyle:    lipgloss.NewStyle().Padding(0, 1),
	ErrorStyle:      lipgloss.NewStyle().Foreground(colourDanger),
	LabelBoldStyle:  lipgloss.NewStyle().Bold(true),
	HunkHeaderStyle: lipgloss.NewStyle().Background(colourHunkHeader),
	AddedStyle:      lipgloss.NewStyle().Background(colourAdded),
	RemovedStyle:    lipgloss.NewStyle().Background(colourRemoved),
	FileHeaderStyle: lipgloss.NewStyle().Bold(true),
	DeletedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(colourDeleted),
	RenamedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(colourRenamed),
	LineSelectStyle: lipgloss.NewStyle().
		Background(colourAccent).
		Foreground(colourOnAccent),
}
