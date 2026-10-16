package ui

import (
	"testing"
	"time"

	"github.com/fimmtiu/git-view/internal/review"
)

// fakeProcess is a review.Process whose Wait returns err at once.
type fakeProcess struct {
	spec      review.JobSpec
	err       error
	cancelled bool
}

func (p *fakeProcess) Wait() error { panic("scaffold stub") }

func (p *fakeProcess) Cancel() { panic("scaffold stub") }

// fakeStarter records the jobs it starts. Start fails for the job named failName,
// and a job whose name is a key of errs gets that error from Wait.
type fakeStarter struct {
	procs    []*fakeProcess
	failName string
	errs     map[string]error
}

func (f *fakeStarter) start(repoRoot string, spec review.JobSpec) (review.Process, error) {
	panic("scaffold stub")
}

var fixedNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// withReviewState returns m with claude found, branch changes present, the
// findings file present or not as given, starter.start as the job starter, and
// a clock stopped at fixedNow.
func withReviewState(m Model, starter *fakeStarter, findings bool) Model {
	panic("scaffold stub")
}

// newBranchRepo creates a clean repository with a "main" branch and a checked-out
// "feature" branch one commit ahead of it, and returns its path.
func newBranchRepo(t *testing.T) string { panic("scaffold stub") }
