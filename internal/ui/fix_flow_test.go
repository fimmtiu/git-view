package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/review"
)

func TestStartFix_StartsFixJob(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	
	// Create findings file
	findingsPath := review.FindingsPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(findingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(findingsPath, []byte("# Findings\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m = withReviewState(m, starter, true)

	updated, cmd := m.startFix()
	m = updated.(Model)

	if m.review.task == nil {
		t.Fatal("startFix should create a task")
	}

	if m.review.task.title != fixDialogTitle {
		t.Errorf("task.title = %q, want %q", m.review.task.title, fixDialogTitle)
	}

	if len(m.review.task.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(m.review.task.jobs))
	}

	job := m.review.task.jobs[0]
	if job.name != review.FixJobName {
		t.Errorf("job.name = %q, want %q", job.name, review.FixJobName)
	}

	if len(starter.procs) != 1 {
		t.Fatalf("expected 1 process started, got %d", len(starter.procs))
	}

	proc := starter.procs[0]
	if proc.spec.Name != review.FixJobName {
		t.Errorf("spec.Name = %q, want %q", proc.spec.Name, review.FixJobName)
	}
	if proc.spec.Prompt != review.FixPrompt() {
		t.Errorf("spec.Prompt doesn't match FixPrompt")
	}

	// Verify the view shows the dialog
	v, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true")
	}
	if v.title != fixDialogTitle {
		t.Errorf("dialog.title = %q, want %q", v.title, fixDialogTitle)
	}

	if cmd == nil {
		t.Fatal("startFix should return a command")
	}
}

func TestStartFix_DoesNothingWhenKeyUnavailable(t *testing.T) {
	repoRoot := newBranchRepo(t)
	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot

	// Test without findingsExist
	m = withReviewState(m, starter, false)
	updated, cmd := m.startFix()
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("startFix should not create a task when findingsExist is false")
	}
	if cmd != nil {
		t.Error("startFix should return nil cmd when findingsExist is false")
	}

	// Test without claudeFound
	m = withReviewState(m, starter, true)
	m.review.claudeFound = false
	updated, cmd = m.startFix()
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("startFix should not create a task when claudeFound is false")
	}
	if cmd != nil {
		t.Error("startFix should return nil cmd when claudeFound is false")
	}

	// Test without hasBranchChanges
	m = withReviewState(m, starter, true)
	m.review.hasBranchChanges = false
	updated, cmd = m.startFix()
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("startFix should not create a task when hasBranchChanges is false")
	}
	if cmd != nil {
		t.Error("startFix should return nil cmd when hasBranchChanges is false")
	}

	// Test with a task already running
	m = withReviewState(m, starter, true)
	m.review.task = &aiTask{id: 1}
	updated, cmd = m.startFix()
	m = updated.(Model)

	if m.review.task.id != 1 {
		t.Error("startFix should not replace an existing task")
	}
	if cmd != nil {
		t.Error("startFix should return nil cmd when task is already running")
	}
}

func TestStartFix_ChecksFindingsExistAgain(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	
	// Set findingsExist to true, but don't actually create the file
	m = withReviewState(m, starter, true)
	
	updated, cmd := m.startFix()
	m = updated.(Model)

	// Should not create a task because the file doesn't exist
	if m.review.task != nil {
		t.Error("startFix should not create a task when file is missing")
	}
	if cmd != nil {
		t.Error("startFix should return nil cmd when file is missing")
	}
	// Should update findingsExist to false
	if m.review.findingsExist {
		t.Error("findingsExist should be set to false when file is missing")
	}
}

func TestStartFix_AllowsDirtyTree(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	
	// Create findings file
	findingsPath := review.FindingsPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(findingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(findingsPath, []byte("# Findings\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	// Make the tree dirty
	if err := os.WriteFile(filepath.Join(repoRoot, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	clean, _ := git.IsClean(repoRoot)
	if clean {
		t.Fatal("repository should be dirty")
	}

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m = withReviewState(m, starter, true)

	updated, cmd := m.startFix()
	m = updated.(Model)

	// Should still create a task even with dirty tree
	if m.review.task == nil {
		t.Fatal("startFix should create a task even with dirty tree")
	}
	if cmd == nil {
		t.Fatal("startFix should return a command even with dirty tree")
	}
}

func TestFinishFix_JobSucceeded(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m.editorFound = false  // So we can check the statusMsg
	m = withReviewState(m, starter, true)

	// Create a task with one completed job
	task := &aiTask{
		id:    1,
		title: fixDialogTitle,
		jobs: []taskJob{
			{
				name:  review.FixJobName,
				proc:  &fakeProcess{},
				state: jobDone,
				err:   nil,
			},
		},
		started:    fixedNow,
		cancelling: false,
		onFinish:   finishFix,
	}

	updated, cmd := finishFix(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("finishFix should return a command batch")
	}

	// Check for notification
	notif, ok := findMsg[notifMsg](cmd)
	if !ok {
		t.Fatal("should have a notifMsg")
	}
	repoName := git.RepoName(repoRoot)
	expectedText := fmt.Sprintf(fixDoneFormat, repoName)
	if notif.text != expectedText {
		t.Errorf("notif.text = %q, want %q", notif.text, expectedText)
	}

	// Check for commitListMsg (refresh)
	_, ok = findMsg[commitListMsg](cmd)
	if !ok {
		t.Fatal("should have a commitListMsg")
	}

	// Check for statusMsg (from openInEditor with editorFound=false)
	status, ok := findMsg[statusMsg](cmd)
	if !ok {
		t.Fatal("should have a statusMsg from openInEditor")
	}
	if !strings.Contains(status.text, "no editor found") {
		t.Errorf("status should be about editor, got %q", status.text)
	}
}

func TestFinishFix_JobFailed(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m = withReviewState(m, starter, true)

	// Create a task with one failed job
	task := &aiTask{
		id:    1,
		title: fixDialogTitle,
		jobs: []taskJob{
			{
				name:  review.FixJobName,
				proc:  &fakeProcess{},
				state: jobFailed,
				err:   errors.New("job failed"),
			},
		},
		started:    fixedNow,
		cancelling: false,
		onFinish:   finishFix,
	}

	updated, cmd := finishFix(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("finishFix should return a command batch")
	}

	// Check for notification
	notif, ok := findMsg[notifMsg](cmd)
	if !ok {
		t.Fatal("should have a notifMsg")
	}
	repoName := git.RepoName(repoRoot)
	expectedText := fmt.Sprintf(fixFailedFormat, repoName)
	if notif.text != expectedText {
		t.Errorf("notif.text = %q, want %q", notif.text, expectedText)
	}

	// Check for commitListMsg (refresh)
	_, ok = findMsg[commitListMsg](cmd)
	if !ok {
		t.Fatal("should have a commitListMsg")
	}
}

func TestFinishFix_Cancelled(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m = withReviewState(m, starter, true)

	// Create a cancelled task
	task := &aiTask{
		id:    1,
		title: fixDialogTitle,
		jobs: []taskJob{
			{
				name:  review.FixJobName,
				proc:  &fakeProcess{},
				state: jobCancelled,
				err:   review.ErrCancelled,
			},
		},
		started:    fixedNow,
		cancelling: true,
		onFinish:   finishFix,
	}

	updated, cmd := finishFix(m, task)
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("finishFix should return a command for refresh")
	}

	// Should have refresh
	_, ok := findMsg[commitListMsg](cmd)
	if !ok {
		t.Fatal("should have a commitListMsg for refresh")
	}

	// Should NOT have notification
	_, ok = findMsg[notifMsg](cmd)
	if ok {
		t.Error("should not have notification when cancelled")
	}
}

func TestFixFlow_EndToEnd(t *testing.T) {
	// Save and disable OS notifications for tests
	oldHasTerminalNotifier := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = oldHasTerminalNotifier })

	repoRoot := newBranchRepo(t)
	
	// Create findings file
	findingsPath := review.FindingsPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(findingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(findingsPath, []byte("# Findings\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	starter := &fakeStarter{}
	m := onSelector(t, 5, 100, 30)
	m.repoRoot = repoRoot
	m.editorFound = false
	m = withReviewState(m, starter, true)

	// Press F to start fix
	updated, _ := m.startFix()
	m = updated.(Model)

	if m.review.task == nil {
		t.Fatal("F should start a fix task")
	}

	taskID := m.review.task.id

	// Simulate job exit with success
	updated, cmd := m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: nil})
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("task should be cleared after job exits")
	}

	// Check the batch has all expected messages
	if cmd == nil {
		t.Fatal("should have a command batch")
	}

	// Should have notification
	notif, ok := findMsg[notifMsg](cmd)
	if !ok {
		t.Fatal("should have a notifMsg")
	}
	if !strings.Contains(notif.text, "fixes are done") {
		t.Errorf("notification should mention fixes done, got %q", notif.text)
	}

	// Should have commit list refresh
	_, ok = findMsg[commitListMsg](cmd)
	if !ok {
		t.Fatal("should have a commitListMsg")
	}

	// Should have tried to open editor (results in statusMsg because editorFound=false)
	_, ok = findMsg[statusMsg](cmd)
	if !ok {
		t.Fatal("should have a statusMsg from openInEditor")
	}
}
