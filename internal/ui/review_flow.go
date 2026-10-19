package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/review"
)

// gitErrorCmd returns a status command for a git error.
func gitErrorCmd(err error) tea.Cmd {
	return statusCmd("git error: "+err.Error(), true)
}

// startCodeReview handles the C key on the commit selector.
func (m Model) startCodeReview() (tea.Model, tea.Cmd) {
	if !m.reviewKeys().review {
		return m, nil
	}

	clean, err := git.IsClean(m.repoRoot)
	if err != nil {
		return m, gitErrorCmd(err)
	}
	if !clean {
		m.review.notice = &dialogView{
			title:  reviewDialogTitle,
			lines:  []string{dirtyTreeMessage},
			button: okButton,
		}
		return m, nil
	}

	base, err := git.BranchBase(m.repoRoot)
	if err != nil {
		return m, gitErrorCmd(err)
	}

	head, err := git.HeadCommit(m.repoRoot)
	if err != nil {
		return m, gitErrorCmd(err)
	}

	if err := review.Prepare(m.repoRoot); err != nil {
		return m, statusCmd(err.Error(), true)
	}

	return m.startTask(reviewDialogTitle, review.ReviewJobs(base, head), finishReview)
}

func finishReview(m Model, t *aiTask) (tea.Model, tea.Cmd) {
	if t.cancelling {
		return m, nil
	}

	name := git.RepoName(m.repoRoot)

	missing := review.MissingOutputs(m.repoRoot)
	if len(missing) > 0 {
		text := fmt.Sprintf(reviewMissingFormat, name, strings.Join(missing, ", "))
		return m, ShowNotification(text)
	}

	m.review.findingsExist = true
	text := fmt.Sprintf(reviewDoneFormat, name)
	return m, tea.Batch(
		ShowNotification(text),
		ShowOSNotification(text),
		m.openInEditor(review.SummaryPath(m.repoRoot), review.FindingsPath(m.repoRoot)),
	)
}
