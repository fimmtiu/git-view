package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/review"
)

const (
	reviewDialogTitle    = "Code review"
	fixDialogTitle       = "Fixing issues"
	dirtyTreeMessage     = "Commit all uncommitted changes before you start a code review."
	cancelButton         = "Cancel"
	okButton             = "OK"
	cancellingLabel      = "Cancelling…"
	reviewDoneFormat     = "%s: Code review is done"
	reviewMissingFormat  = "%s: Code review failed: %s not written"
	fixDoneFormat        = "%s: Code review fixes are done"
	fixFailedFormat      = "%s: Code review fixes failed"
	jobStartFailedFormat = "could not start claude: %v"

	reviewHintKey  = "C"
	reviewHintDesc = "code review"
	fixHintKey     = "F"
	fixHintDesc    = "fix issues"
)

// How often an open task dialog redraws its timer.
const dialogTickInterval = time.Second

// reviewState holds what the code review and fix keys need.
type reviewState struct {
	claudeFound      bool // checked once, at startup
	hasBranchChanges bool
	findingsExist    bool
	startJob         review.StartFunc
	now              func() time.Time
	task             *aiTask     // the running review or fix, or nil
	notice           *dialogView // a message dialog with an OK button, or nil
	nextTaskID       int
}

type reviewKeys struct {
	review bool
	fix    bool
}

type jobState int

const (
	jobRunning jobState = iota
	jobDone
	jobFailed
	jobCancelled
)

type taskJob struct {
	name  string
	proc  review.Process
	state jobState
	err   error
}

// taskFinishFunc runs once, after every job in t has exited and t is no longer m's task.
type taskFinishFunc func(m Model, t *aiTask) (tea.Model, tea.Cmd)

// aiTask is a set of Claude jobs that run together under one dialog.
type aiTask struct {
	id         int
	title      string
	jobs       []taskJob
	started    time.Time
	cancelling bool
	onFinish   taskFinishFunc
}

type jobExitedMsg struct {
	taskID int
	index  int
	err    error
}

type dialogTickMsg struct{ taskID int }

type reviewStateMsg struct {
	hasBranchChanges bool
	findingsExist    bool
}

type dialogView struct {
	title       string
	lines       []string
	elapsed     time.Duration
	showElapsed bool
	button      string
}
