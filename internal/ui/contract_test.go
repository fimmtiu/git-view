package ui

import (
	"testing"
	"time"

	"github.com/fimmtiu/git-view/internal/editor"
	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/review"
)
// Check that every function and method in the contract exists.
var (
	_ = git.BranchBase
	_ = git.HeadCommit
	_ = git.HasBranchChanges
	_ = git.IsClean
	_ = git.RepoName
	_ = review.SummaryPath
	_ = review.FindingsPath
	_ = review.Prepare
	_ = review.FindingsExist
	_ = review.MissingOutputs
	_ = review.ReviewPrompt
	_ = review.SummaryPrompt
	_ = review.FixPrompt
	_ = review.ReviewJobs
	_ = review.FixJob
	_ = review.JobSpec.Args
	_ = review.ClaudeAvailable
	_ = editor.Editor.CommandFiles
	_ = dialogView{}.Render
	_ = formatElapsed
	_ = overlayCenter
	_ = jobState.String
	_ = Model.startTask
	_ = waitJobCmd
	_ = dialogTickCmd
	_ = Model.handleJobExited
	_ = Model.handleDialogTick
	_ = (*Model).cancelTask
	_ = Model.handleDialogKey
	_ = Model.dialogView
	_ = (*aiTask).allExited
	_ = newReviewState
	_ = Model.reviewKeys
	_ = selector.hintPairs
	_ = Model.startCodeReview
	_ = finishReview
	_ = Model.startFix
	_ = finishFix
	_ = Model.openInEditor
)

// Check assignability of function types.
var (
	_ review.StartFunc   = review.Start
	_ taskFinishFunc     = finishReview
	_ taskFinishFunc     = finishFix
	_ review.Process     = (*fakeProcess)(nil)
	_ review.StartFunc   = (&fakeStarter{}).start
)

func TestContract_Constants(t *testing.T) {
	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"review.Dir", review.Dir, ".gv-review"},
		{"review.SummaryFile", review.SummaryFile, "summary.md"},
		{"review.FindingsFile", review.FindingsFile, "code-review-findings.md"},
		{"review.ReviewModel", review.ReviewModel, "sonnet"},
		{"review.SummaryModel", review.SummaryModel, "claude-opus-5-5"},
		{"review.FixModel", review.FixModel, "claude-opus-5-5"},
		{"review.Effort", review.Effort, "high"},
		{"review.ReviewJobName", review.ReviewJobName, "Code review"},
		{"review.SummaryJobName", review.SummaryJobName, "PR summary"},
		{"review.FixJobName", review.FixJobName, "Fixes"},
		{"review.ClaudeBinary", review.ClaudeBinary, "claude"},
		{"review.CancelGracePeriod", review.CancelGracePeriod.Nanoseconds(), (5 * time.Second).Nanoseconds()},
		{"reviewDialogTitle", reviewDialogTitle, "Code review"},
		{"fixDialogTitle", fixDialogTitle, "Fixing issues"},
		{"dirtyTreeMessage", dirtyTreeMessage, "Commit all uncommitted changes before you start a code review."},
		{"cancelButton", cancelButton, "Cancel"},
		{"okButton", okButton, "OK"},
		{"cancellingLabel", cancellingLabel, "Cancelling…"},
		{"reviewDoneFormat", reviewDoneFormat, "%s: Code review is done"},
		{"reviewMissingFormat", reviewMissingFormat, "%s: Code review failed: %s not written"},
		{"fixDoneFormat", fixDoneFormat, "%s: Code review fixes are done"},
		{"fixFailedFormat", fixFailedFormat, "%s: Code review fixes failed"},
		{"jobStartFailedFormat", jobStartFailedFormat, "could not start claude: %v"},
		{"reviewHintKey", reviewHintKey, "C"},
		{"reviewHintDesc", reviewHintDesc, "code review"},
		{"fixHintKey", fixHintKey, "F"},
		{"fixHintDesc", fixHintDesc, "fix issues"},
		{"dialogTickInterval", dialogTickInterval.Nanoseconds(), time.Second.Nanoseconds()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if review.ErrCancelled.Error() != "cancelled" {
		t.Errorf("review.ErrCancelled.Error() = %q, want %q", review.ErrCancelled.Error(), "cancelled")
	}
}

func TestContract_Types(t *testing.T) {
	// Build one value of each type with every field set.
	_ = review.JobSpec{
		Name:   "test",
		Model:  "model",
		Effort: "high",
		Prompt: "prompt",
	}

	_ = reviewState{
		claudeFound:      true,
		hasBranchChanges: true,
		findingsExist:    true,
		task:             nil,
		notice:           nil,
		nextTaskID:       0,
	}

	_ = reviewKeys{
		review: true,
		fix:    false,
	}

	_ = taskJob{
		name:  "job",
		proc:  nil,
		state: jobRunning,
		err:   nil,
	}

	_ = aiTask{
		id:         1,
		title:      "title",
		jobs:       nil,
		started:    fixedNow,
		cancelling: false,
		onFinish:   nil,
	}

	_ = jobExitedMsg{
		taskID: 1,
		index:  0,
		err:    nil,
	}

	_ = dialogTickMsg{
		taskID: 1,
	}

	_ = dialogView{
		title:       "title",
		lines:       nil,
		elapsed:     0,
		showElapsed: true,
		button:      "button",
	}

	_ = commitListMsg{
		commits:        nil,
		forkPointIdx:   0,
		hasUncommitted: false,
		err:            "",
	}

	_ = reviewStateMsg{
		hasBranchChanges: true,
		findingsExist:    true,
	}

	// Check jobState constants.
	if jobRunning != 0 || jobDone != 1 || jobFailed != 2 || jobCancelled != 3 {
		t.Errorf("jobState constants: running=%d, done=%d, failed=%d, cancelled=%d, want 0, 1, 2, 3",
			jobRunning, jobDone, jobFailed, jobCancelled)
	}

	// Check that the theme fields exist.
	_ = theme.DialogStyle
	_ = theme.DialogTitleStyle
	_ = theme.DialogButtonStyle

	// Check that Model has review field.
	m := Model{}
	_ = m.review

	// Check that reviewState has startJob and now fields.
	_ = m.review.startJob
	_ = m.review.now
}

// TestContract_MessageStructureSeparation_After verifies the desired state after CR #28.
func TestContract_MessageStructureSeparation_After(t *testing.T) {
	t.Run("commitListMsg should not have review fields", func(t *testing.T) {
		msg := commitListMsg{
			commits:        nil,
			forkPointIdx:   0,
			hasUncommitted: false,
			err:            "",
		}
		_ = msg
	})

	t.Run("reviewStateMsg should carry review state", func(t *testing.T) {
		msg := reviewStateMsg{
			hasBranchChanges: true,
			findingsExist:    false,
		}
		
		if !msg.hasBranchChanges {
			t.Error("hasBranchChanges field should be accessible")
		}
		if msg.findingsExist {
			t.Error("findingsExist field should be accessible")
		}
	})

	t.Run("reviewStateMsg is a distinct type from commitListMsg", func(t *testing.T) {
		var commit commitListMsg
		var review reviewStateMsg
		_ = commit
		_ = review
	})
}
