package ui

import (
	"time"

	"github.com/fimmtiu/git-view/internal/review"
)

// newReviewState checks for claude once, at startup.
func newReviewState() reviewState {
	return reviewState{
		claudeFound: review.ClaudeAvailable(),
		startJob:    review.Start,
		now:         time.Now,
		task:        nil,
		notice:      nil,
		nextTaskID:  0,
	}
}

// reviewKeys reports which of the C and F keys are available.
func (m Model) reviewKeys() reviewKeys {
	review := m.review.claudeFound && m.review.hasBranchChanges && m.review.task == nil
	fix := review && m.review.findingsExist
	return reviewKeys{
		review: review,
		fix:    fix,
	}
}
