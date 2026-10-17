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
	m.review.claudeFound = true
	m.review.hasBranchChanges = true
	m.review.findingsExist = findings
	m.startJob = starter.start
	m.timeNow = func() time.Time { return fixedNow }
	return m
}

// newBranchRepo creates a clean repository with a "main" branch and a checked-out
// "feature" branch one commit ahead of it, and returns its path.
func newBranchRepo(t *testing.T) string { panic("scaffold stub") }

func TestReviewKeys(t *testing.T) {
	tests := []struct {
		name             string
		claudeFound      bool
		hasBranchChanges bool
		findingsExist    bool
		wantReview       bool
		wantFix          bool
	}{
		{
			name:             "no claude found",
			claudeFound:      false,
			hasBranchChanges: false,
			findingsExist:    false,
			wantReview:       false,
			wantFix:          false,
		},
		{
			name:             "claude found with branch changes but no findings",
			claudeFound:      true,
			hasBranchChanges: true,
			findingsExist:    false,
			wantReview:       true,
			wantFix:          false,
		},
		{
			name:             "claude found with findings but no branch changes",
			claudeFound:      true,
			hasBranchChanges: false,
			findingsExist:    true,
			wantReview:       false,
			wantFix:          true,
		},
		{
			name:             "claude found with both branch changes and findings",
			claudeFound:      true,
			hasBranchChanges: true,
			findingsExist:    true,
			wantReview:       true,
			wantFix:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				review: reviewState{
					claudeFound:      tt.claudeFound,
					hasBranchChanges: tt.hasBranchChanges,
					findingsExist:    tt.findingsExist,
				},
			}

			keys := m.reviewKeys()

			if keys.review != tt.wantReview {
				t.Errorf("review = %v, want %v", keys.review, tt.wantReview)
			}
			if keys.fix != tt.wantFix {
				t.Errorf("fix = %v, want %v", keys.fix, tt.wantFix)
			}
		})
	}
}

func TestNewReviewState_NoDependencies(t *testing.T) {
	rs := newReviewState()

	if rs.task != nil {
		t.Error("newReviewState should create reviewState with nil task")
	}
	if rs.notice != nil {
		t.Error("newReviewState should create reviewState with nil notice")
	}
	if rs.nextTaskID != 0 {
		t.Errorf("newReviewState should initialize nextTaskID to 0, got %d", rs.nextTaskID)
	}
}
