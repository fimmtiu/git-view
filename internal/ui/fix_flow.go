package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/review"
)

// startFix handles the F key on the commit selector.
func (m Model) startFix() (tea.Model, tea.Cmd) {
	// Check if the key is available
	if !m.reviewKeys().fix {
		return m, nil
	}

	// Check if findings file still exists (it could have been deleted)
	if !review.FindingsExist(m.repoRoot) {
		m.review.findingsExist = false
		return m, nil
	}

	// Start the fix task
	return m.startTask(fixDialogTitle, []review.JobSpec{review.FixJob()}, finishFix)
}

func finishFix(m Model, t *aiTask) (tea.Model, tea.Cmd) {
	refresh := fetchCommitsCmd(m.repoRoot)

	// If cancelled, just refresh
	if t.cancelling {
		return m, refresh
	}

	name := git.RepoName(m.repoRoot)

	// Check if the job failed
	if len(t.jobs) > 0 && t.jobs[0].state == jobFailed {
		return m, tea.Batch(
			refresh,
			ShowNotification(fmt.Sprintf(fixFailedFormat, name)),
		)
	}

	// Job succeeded
	text := fmt.Sprintf(fixDoneFormat, name)
	return m, tea.Batch(
		ShowNotification(text),
		ShowOSNotification(text),
		refresh,
		m.openInEditor(review.FindingsPath(m.repoRoot)),
	)
}
