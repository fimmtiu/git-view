package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	PaneStyle       lipgloss.Style
	StatusBarStyle  lipgloss.Style
	EmptyStateStyle lipgloss.Style
	HintKeyStyle    lipgloss.Style
	HintDescStyle   lipgloss.Style
	HintBarStyle    lipgloss.Style
	ErrorStyle      lipgloss.Style
	LabelBoldStyle  lipgloss.Style

	// Diff content.
	HunkHeaderStyle lipgloss.Style
	AddedStyle      lipgloss.Style
	RemovedStyle    lipgloss.Style
	FileHeaderStyle lipgloss.Style
	DeletedMsgStyle lipgloss.Style
	RenamedMsgStyle lipgloss.Style
	LineSelectStyle lipgloss.Style

	// Commit selector.
	SelectedStyle   lipgloss.Style
	RangeStyle      lipgloss.Style
	SeparatorStyle  lipgloss.Style
	CommitHashStyle lipgloss.Style
	StatAddStyle    lipgloss.Style
	StatRemoveStyle lipgloss.Style

	// Search.
	SearchMatchStyle  lipgloss.Style
	SearchPromptStyle lipgloss.Style
	SearchCursorStyle lipgloss.Style
}

// Light values are tuned for white terminals (pastel backgrounds, dark
// foregrounds), dark values for black ones.
var (
	colourBorder     = lipgloss.AdaptiveColor{Light: "25", Dark: "24"}
	colourAccent     = lipgloss.AdaptiveColor{Light: "30", Dark: "37"}
	colourOnAccent   = lipgloss.AdaptiveColor{Light: "255", Dark: "16"}
	colourPrimary    = lipgloss.AdaptiveColor{Light: "25", Dark: "25"}
	colourOnPrimary  = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}
	colourMuted      = lipgloss.AdaptiveColor{Light: "244", Dark: "245"}
	colourDimGrey    = lipgloss.AdaptiveColor{Light: "102", Dark: "240"}
	colourSuccess    = lipgloss.AdaptiveColor{Light: "28", Dark: "77"}
	colourSubtle     = lipgloss.AdaptiveColor{Light: "243", Dark: "240"}
	colourDanger     = lipgloss.AdaptiveColor{Light: "124", Dark: "167"}
	colourHunkHeader = lipgloss.AdaptiveColor{Light: "189", Dark: "24"}
	colourAdded      = lipgloss.AdaptiveColor{Light: "194", Dark: "22"}
	colourRemoved    = lipgloss.AdaptiveColor{Light: "224", Dark: "52"}
	colourDeleted    = lipgloss.AdaptiveColor{Light: "88", Dark: "167"}
	colourRenamed    = lipgloss.AdaptiveColor{Light: "25", Dark: "75"}
	// A search match can sit on the added, removed, or terminal background, so
	// it sets both colours, far from all three. The foreground is dark in both
	// palettes because their default foregrounds differ.
	colourMatch   = lipgloss.AdaptiveColor{Light: "220", Dark: "214"}
	colourOnMatch = lipgloss.AdaptiveColor{Light: "16", Dark: "16"}
)

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
	SelectedStyle: lipgloss.NewStyle().
		Background(colourPrimary).
		Foreground(colourOnPrimary),
	RangeStyle: lipgloss.NewStyle().
		Background(colourAccent).
		Foreground(colourOnAccent),
	SeparatorStyle:  lipgloss.NewStyle().Foreground(colourDimGrey),
	CommitHashStyle: lipgloss.NewStyle().Bold(true).Foreground(colourMuted),
	StatAddStyle:    lipgloss.NewStyle().Foreground(colourSuccess),
	StatRemoveStyle: lipgloss.NewStyle().Foreground(colourDanger),
	SearchMatchStyle: lipgloss.NewStyle().
		Background(colourMatch).
		Foreground(colourOnMatch),
	SearchPromptStyle: lipgloss.NewStyle().Bold(true),
	SearchCursorStyle: lipgloss.NewStyle().
		Background(colourAccent).
		Foreground(colourOnAccent),
}
