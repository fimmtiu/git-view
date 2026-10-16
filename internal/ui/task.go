package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fimmtiu/git-view/internal/review"
)

func (s jobState) String() string { panic("scaffold stub") }

// startTask starts every job in specs and opens the task dialog.
func (m Model) startTask(title string, specs []review.JobSpec, onFinish taskFinishFunc) (tea.Model, tea.Cmd) {
	panic("scaffold stub")
}

// waitJobCmd blocks in a Bubble Tea goroutine until p exits.
func waitJobCmd(taskID, index int, p review.Process) tea.Cmd { panic("scaffold stub") }

func dialogTickCmd(taskID int) tea.Cmd { panic("scaffold stub") }

func (m Model) handleJobExited(msg jobExitedMsg) (tea.Model, tea.Cmd) { panic("scaffold stub") }

func (m Model) handleDialogTick(msg dialogTickMsg) (tea.Model, tea.Cmd) { panic("scaffold stub") }

// cancelTask asks every running job in the current task to stop.
func (m *Model) cancelTask() { panic("scaffold stub") }

// handleDialogKey takes every key while a dialog is open.
func (m Model) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { panic("scaffold stub") }

// dialogView returns the open dialog's content, or false when none is open.
func (m Model) dialogView() (dialogView, bool) { panic("scaffold stub") }

func (t *aiTask) allExited() bool { panic("scaffold stub") }
