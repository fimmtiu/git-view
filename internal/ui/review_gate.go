package ui

// newReviewState checks for claude once, at startup.
func newReviewState() reviewState {
	return reviewState{
		task:       nil,
		notice:     nil,
		nextTaskID: 0,
	}
}

// reviewKeys reports which of the C and F keys are available.
func (m Model) reviewKeys() reviewKeys {
	return reviewKeys{
		review: m.review.claudeFound && m.review.hasBranchChanges,
		fix:    m.review.claudeFound && m.review.findingsExist,
	}
}
