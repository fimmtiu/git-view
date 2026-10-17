package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// openInEditor opens files together in the user's editor.
func (m Model) openInEditor(files ...string) tea.Cmd {
	if !m.editorFound {
		return statusCmd("no editor found; set $EDITOR or $GIT_VIEW_EDITOR", true)
	}

	cmd := m.editor.CommandFiles(m.repoRoot, files...)
	return m.execEditor(cmd)
}
