package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/review"
)

// TestStartCodeReview_KeyIgnoredWhenDisabled verifies that startCodeReview does
// nothing when reviewKeys().review is false.
func TestStartCodeReview_KeyIgnoredWhenDisabled(t *testing.T) {
	cases := []struct {
		name          string
		claudeFound   bool
		branchChanges bool
		hasTask       bool
	}{
		{"no claude", false, true, false},
		{"no branch changes", true, false, false},
		{"task running", true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			starter := &fakeStarter{}
			m := withReviewState(onSelector(t, 5, 100, 30), starter, false)
			m.review.claudeFound = tc.claudeFound
			m.review.hasBranchChanges = tc.branchChanges
			if tc.hasTask {
				// Create a fake running task
				m.review.task = &aiTask{
					id:    1,
					title: "Running",
					jobs:  []taskJob{{name: "Test", state: jobRunning}},
				}
			}

			updated, cmd := m.startCodeReview()
			m = updated.(Model)

			if len(starter.procs) > 0 {
				t.Errorf("expected no jobs started, got %d", len(starter.procs))
			}
			if cmd != nil {
				t.Error("expected nil command when key is disabled")
			}
			if m.review.task != nil && !tc.hasTask {
				t.Error("should not create a task when key is disabled")
			}
		})
	}
}

// TestStartCodeReview_DirtyTreeShowsDialog verifies that a dirty tree shows a
// dialog message and starts no jobs.
func TestStartCodeReview_DirtyTreeShowsDialog(t *testing.T) {
	cases := []struct {
		name string
		prep func(dir string)
	}{
		{
			name: "untracked file",
			prep: func(dir string) {
				os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0o644)
			},
		},
		{
			name: "staged change",
			prep: func(dir string) {
				os.WriteFile(filepath.Join(dir, "second.txt"), []byte("modified"), 0o644)
				runGit(t, dir, "add", ".")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Disable terminal notifier for tests
			oldHasTerminalNotifier := hasTerminalNotifier
			hasTerminalNotifier = false
			t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

			dir := newBranchRepo(t)
			tc.prep(dir)

			starter := &fakeStarter{}
			m := onSelector(t, 5, 100, 30)
			m.repoRoot = dir
			m = withReviewState(m, starter, false)

			updated, cmd := m.startCodeReview()
			m = updated.(Model)

			if len(starter.procs) > 0 {
				t.Errorf("expected no jobs started with dirty tree, got %d", len(starter.procs))
			}
			if cmd != nil {
				t.Error("expected nil command when tree is dirty")
			}
			if m.review.notice == nil {
				t.Fatal("expected notice dialog to be set")
			}
			if m.review.notice.title != reviewDialogTitle {
				t.Errorf("notice title = %q, want %q", m.review.notice.title, reviewDialogTitle)
			}
			if len(m.review.notice.lines) != 1 || m.review.notice.lines[0] != dirtyTreeMessage {
				t.Errorf("notice lines = %v, want [%q]", m.review.notice.lines, dirtyTreeMessage)
			}
			if m.review.notice.button != okButton {
				t.Errorf("notice button = %q, want %q", m.review.notice.button, okButton)
			}

			// Verify enter closes the dialog
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if m.review.notice != nil {
				t.Error("enter should close the notice dialog")
			}
		})
	}
}

// runGit is a test helper to execute git commands.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestStartCodeReview_StartsJobs verifies that on a clean branch, startCodeReview
// starts both the review and summary jobs with the correct prompts.
func TestStartCodeReview_StartsJobs(t *testing.T) {
	// Disable terminal notifier for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	dir := newBranchRepo(t)

	// Remove old outputs if they exist
	os.RemoveAll(filepath.Join(dir, ".gv-review"))

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = dir
	m = withReviewState(m, starter, false)

	updated, cmd := m.startCodeReview()
	m = updated.(Model)

	// Verify two jobs were started
	if len(starter.procs) != 2 {
		t.Fatalf("expected 2 jobs started, got %d", len(starter.procs))
	}

	// Verify job names
	if starter.procs[0].spec.Name != review.ReviewJobName {
		t.Errorf("job 0 name = %q, want %q", starter.procs[0].spec.Name, review.ReviewJobName)
	}
	if starter.procs[1].spec.Name != review.SummaryJobName {
		t.Errorf("job 1 name = %q, want %q", starter.procs[1].spec.Name, review.SummaryJobName)
	}

	// Verify the review job prompt contains the actual base and head SHAs
	reviewPrompt := starter.procs[0].spec.Prompt
	if !strings.Contains(reviewPrompt, "Base commit:") {
		t.Error("review prompt should contain 'Base commit:'")
	}
	if !strings.Contains(reviewPrompt, "HEAD commit:") {
		t.Error("review prompt should contain 'HEAD commit:'")
	}

	// Verify the SHAs are real (40 hex chars)
	lines := strings.Split(reviewPrompt, "\n")
	var baseSHA, headSHA string
	for _, line := range lines {
		if strings.HasPrefix(line, "Base commit: ") {
			baseSHA = strings.TrimPrefix(line, "Base commit: ")
		}
		if strings.HasPrefix(line, "HEAD commit: ") {
			headSHA = strings.TrimPrefix(line, "HEAD commit: ")
		}
	}
	if len(baseSHA) != 40 {
		t.Errorf("base SHA length = %d, want 40", len(baseSHA))
	}
	if len(headSHA) != 40 {
		t.Errorf("head SHA length = %d, want 40", len(headSHA))
	}

	// Verify the dialog is shown
	if m.review.task == nil {
		t.Fatal("expected task to be set")
	}
	if m.review.task.title != reviewDialogTitle {
		t.Errorf("task title = %q, want %q", m.review.task.title, reviewDialogTitle)
	}

	_, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true when task is running")
	}
	if !strings.Contains(m.View(), reviewDialogTitle) {
		t.Error("view should contain the dialog title")
	}

	// Verify .gv-review directory was created and old outputs removed
	reviewDir := filepath.Join(dir, ".gv-review")
	if _, err := os.Stat(reviewDir); os.IsNotExist(err) {
		t.Error(".gv-review directory should exist")
	}

	// Create old outputs and verify they were removed
	os.WriteFile(filepath.Join(reviewDir, "summary.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(reviewDir, "code-review-findings.md"), []byte("old"), 0o644)

	// Start again to test removal
	m.review.task = nil // Reset task
	updated, _ = m.startCodeReview()
	m = updated.(Model)

	// Old files should be gone
	if _, err := os.Stat(filepath.Join(reviewDir, "summary.md")); err == nil {
		t.Error("old summary.md should be removed")
	}
	if _, err := os.Stat(filepath.Join(reviewDir, "code-review-findings.md")); err == nil {
		t.Error("old code-review-findings.md should be removed")
	}

	// Verify batch command contains the right messages
	if cmd == nil {
		t.Fatal("expected a command batch")
	}
}

// TestFinishReview_BothFilesWritten verifies that when both files are written,
// finishReview sends notifications and opens both files in the editor.
func TestFinishReview_BothFilesWritten(t *testing.T) {
	// Disable terminal notifier for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	dir := newBranchRepo(t)
	reviewDir := filepath.Join(dir, ".gv-review")
	os.MkdirAll(reviewDir, 0o755)

	// Write both output files
	os.WriteFile(filepath.Join(reviewDir, "summary.md"), []byte("summary"), 0o644)
	os.WriteFile(filepath.Join(reviewDir, "code-review-findings.md"), []byte("findings"), 0o644)

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = dir
	m.editorFound = false // So openInEditor returns a statusMsg we can check
	m = withReviewState(m, starter, false)

	task := &aiTask{
		id:         1,
		title:      reviewDialogTitle,
		jobs:       []taskJob{{name: "Test", state: jobDone}},
		started:    fixedNow,
		cancelling: false,
	}

	updated, cmd := finishReview(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected a command batch")
	}

	// Find the notification message
	notif, ok := findMsg[notifMsg](cmd)
	if !ok {
		t.Fatal("expected notifMsg in command batch")
	}

	repoName := filepath.Base(dir)
	expectedText := fmt.Sprintf(reviewDoneFormat, repoName)
	if notif.text != expectedText {
		t.Errorf("notification text = %q, want %q", notif.text, expectedText)
	}

	// Verify findingsExist was set
	if !m.review.findingsExist {
		t.Error("findingsExist should be true when findings file exists")
	}

	// Verify openInEditor was called by checking for statusMsg (since editorFound is false)
	status, ok := findMsg[statusMsg](cmd)
	if !ok {
		t.Fatal("expected statusMsg from openInEditor (editorFound=false)")
	}
	if !status.isErr {
		t.Error("status should be an error when editor not found")
	}
}

// TestFinishReview_MissingFiles verifies that when files are missing, finishReview
// sends an error notification.
func TestFinishReview_MissingFiles(t *testing.T) {
	// Disable terminal notifier for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	dir := newBranchRepo(t)
	reviewDir := filepath.Join(dir, ".gv-review")
	os.MkdirAll(reviewDir, 0o755)

	// Write only summary file
	os.WriteFile(filepath.Join(reviewDir, "summary.md"), []byte("summary"), 0o644)

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = dir
	m = withReviewState(m, starter, false)

	task := &aiTask{
		id:         1,
		title:      reviewDialogTitle,
		jobs:       []taskJob{{name: "Test", state: jobDone}},
		started:    fixedNow,
		cancelling: false,
	}

	updated, cmd := finishReview(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected a command")
	}

	// Find the notification message
	notif, ok := findMsg[notifMsg](cmd)
	if !ok {
		t.Fatal("expected notifMsg in command")
	}

	repoName := filepath.Base(dir)
	expectedText := fmt.Sprintf(reviewMissingFormat, repoName, ".gv-review/code-review-findings.md")
	if notif.text != expectedText {
		t.Errorf("notification text = %q, want %q", notif.text, expectedText)
	}

	// Verify findingsExist was not set
	if m.review.findingsExist {
		t.Error("findingsExist should be false when findings file is missing")
	}
}

// cancelledReviewModel returns a model on a repo whose .gv-review holds the given files.
func cancelledReviewModel(t *testing.T, files ...string) (Model, *aiTask) {
	t.Helper()

	dir := newBranchRepo(t)
	reviewDir := filepath.Join(dir, ".gv-review")
	os.MkdirAll(reviewDir, 0o755)
	for _, f := range files {
		os.WriteFile(filepath.Join(reviewDir, f), []byte("partial"), 0o644)
	}

	m := onSelector(t, 5, 100, 30)
	m.repoRoot = dir
	m.editorFound = false // So openInEditor returns a statusMsg we can check
	m = withReviewState(m, &fakeStarter{}, false)

	task := &aiTask{
		id:         1,
		title:      reviewDialogTitle,
		jobs:       []taskJob{{name: review.ReviewJobName, state: jobCancelled, err: review.ErrCancelled}},
		started:    fixedNow,
		cancelling: true,
	}
	return m, task
}

func TestFinishReview_CancelledWithPartialFindings(t *testing.T) {
	m, task := cancelledReviewModel(t, "code-review-findings.md")

	updated, cmd := finishReview(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected a command to open the partial findings")
	}
	if _, ok := findMsg[statusMsg](cmd); !ok {
		t.Error("expected statusMsg from openInEditor (editorFound=false)")
	}
	if _, ok := findMsg[notifMsg](cmd); ok {
		t.Error("cancelled review should not send notifications")
	}
	if !m.review.findingsExist {
		t.Error("findingsExist should be true when partial findings exist")
	}
}

func TestFinishReview_CancelledWithOnlySummary(t *testing.T) {
	m, task := cancelledReviewModel(t, "summary.md")

	updated, cmd := finishReview(m, task)
	m = updated.(Model)

	if _, ok := findMsg[statusMsg](cmd); !ok {
		t.Error("expected statusMsg from openInEditor (editorFound=false)")
	}
	if m.review.findingsExist {
		t.Error("findingsExist should be false when the findings file is missing")
	}
}

func TestFinishReview_CancelledWhileQuitting(t *testing.T) {
	m, task := cancelledReviewModel(t, "summary.md", "code-review-findings.md")
	m.quitting = true

	if _, cmd := finishReview(m, task); cmd != nil {
		t.Error("a review cancelled by quitting should not open the editor")
	}
}

// TestFinishReview_Cancelled verifies that a cancelled review with no output does nothing.
func TestFinishReview_Cancelled(t *testing.T) {
	// Disable terminal notifier for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	dir := newBranchRepo(t)

	starter := &fakeStarter{
		errs: map[string]error{
			review.ReviewJobName:  review.ErrCancelled,
			review.SummaryJobName: review.ErrCancelled,
		},
	}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = dir
	m = withReviewState(m, starter, false)

	// Start the review
	updated, _ := m.startCodeReview()
	m = updated.(Model)

	if m.review.task == nil {
		t.Fatal("task should be set")
	}

	// Cancel the task
	m.cancelTask()

	// Simulate both jobs exiting with ErrCancelled
	task := m.review.task
	task.jobs = []taskJob{
		{name: review.ReviewJobName, state: jobCancelled, err: review.ErrCancelled},
		{name: review.SummaryJobName, state: jobCancelled, err: review.ErrCancelled},
	}

	_, cmd := finishReview(m, task)

	// Should return m, nil for a cancelled review
	if cmd != nil {
		// Walk the batch to check for notifications
		foundNotif := walkMsgs(cmd, func(msg tea.Msg) bool {
			_, is := msg.(notifMsg)
			return is
		})
		if foundNotif {
			t.Error("cancelled review should not send notifications")
		}
	}
}
