package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/review"
)

func TestJobState_String(t *testing.T) {
	cases := []struct {
		state jobState
		want  string
	}{
		{jobRunning, "Running"},
		{jobDone, "Done"},
		{jobFailed, "Failed"},
		{jobCancelled, "Cancelled"},
	}

	for _, tc := range cases {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("jobState(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestStartTask_TwoSpecs(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "First", Model: "sonnet", Effort: "high", Prompt: "first prompt"},
		{Name: "Second", Model: "opus", Effort: "high", Prompt: "second prompt"},
	}

	updated, cmd := m.startTask("Test Task", specs, nil)
	m = updated.(Model)

	if m.review.task == nil {
		t.Fatal("startTask should set m.review.task")
	}
	if m.review.task.title != "Test Task" {
		t.Errorf("task.title = %q, want %q", m.review.task.title, "Test Task")
	}
	if len(m.review.task.jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(m.review.task.jobs))
	}
	if m.review.task.jobs[0].name != "First" {
		t.Errorf("job 0 name = %q, want %q", m.review.task.jobs[0].name, "First")
	}
	if m.review.task.jobs[1].name != "Second" {
		t.Errorf("job 1 name = %q, want %q", m.review.task.jobs[1].name, "Second")
	}
	for i, job := range m.review.task.jobs {
		if job.state != jobRunning {
			t.Errorf("job %d state = %v, want jobRunning", i, job.state)
		}
	}
	if m.review.task.started != fixedNow {
		t.Errorf("task.started = %v, want %v", m.review.task.started, fixedNow)
	}
	if len(starter.procs) != 2 {
		t.Fatalf("expected 2 processes started, got %d", len(starter.procs))
	}

	if cmd == nil {
		t.Fatal("startTask should return a command batch")
	}

	// Check the view shows both jobs as running
	v, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true when task is set")
	}
	if v.title != "Test Task" {
		t.Errorf("dialog title = %q, want %q", v.title, "Test Task")
	}
	if len(v.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(v.lines))
	}
	if v.lines[0] != "First: Running" {
		t.Errorf("line 0 = %q, want %q", v.lines[0], "First: Running")
	}
	if v.lines[1] != "Second: Running" {
		t.Errorf("line 1 = %q, want %q", v.lines[1], "Second: Running")
	}
	if !v.showElapsed {
		t.Error("showElapsed should be true")
	}
	if v.elapsed != 0 {
		t.Errorf("elapsed = %v, want 0 at fixedNow", v.elapsed)
	}
	if v.button != cancelButton {
		t.Errorf("button = %q, want %q", v.button, cancelButton)
	}
}

func TestStartTask_StartJobFails(t *testing.T) {
	starter := &fakeStarter{failName: "Second"}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "First", Model: "sonnet", Effort: "high", Prompt: "first"},
		{Name: "Second", Model: "opus", Effort: "high", Prompt: "second"},
		{Name: "Third", Model: "haiku", Effort: "low", Prompt: "third"},
	}

	updated, cmd := m.startTask("Test", specs, nil)
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("task should be nil when startJob fails")
	}

	// First job should have been cancelled
	if len(starter.procs) < 1 {
		t.Fatal("expected at least one process to have been started")
	}
	if !starter.procs[0].cancelled {
		t.Error("first job should be cancelled when second job fails to start")
	}

	// Should return a status command with the error
	if cmd == nil {
		t.Fatal("expected a status command")
	}
	msg, ok := findMsg[statusMsg](cmd)
	if !ok {
		t.Fatal("expected a statusMsg")
	}
	if !msg.isErr {
		t.Error("status should be an error")
	}
	if !strings.Contains(msg.text, "Second") {
		t.Errorf("error should mention the failing job, got %q", msg.text)
	}
}

func TestDialogView_WithElapsedTime(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Task", specs, nil)
	m = updated.(Model)

	// Advance the clock after starting the task
	elapsed := 65 * time.Second
	m.review.now = func() time.Time {
		return fixedNow.Add(elapsed)
	}

	v, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true")
	}
	if v.elapsed != elapsed {
		t.Errorf("elapsed = %v, want %v", v.elapsed, elapsed)
	}
}

func TestHandleJobExited_FirstJobCompletes(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "First", Model: "sonnet", Effort: "high", Prompt: "first"},
		{Name: "Second", Model: "opus", Effort: "high", Prompt: "second"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)
	taskID := m.review.task.id

	// First job completes successfully
	updated, cmd := m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: nil})
	m = updated.(Model)

	if m.review.task == nil {
		t.Fatal("task should still exist after first job exits")
	}
	if m.review.task.jobs[0].state != jobDone {
		t.Errorf("job 0 state = %v, want jobDone", m.review.task.jobs[0].state)
	}
	if m.review.task.jobs[1].state != jobRunning {
		t.Errorf("job 1 state = %v, want jobRunning", m.review.task.jobs[1].state)
	}

	// Check view
	v, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true")
	}
	if v.lines[0] != "First: Done" {
		t.Errorf("line 0 = %q, want %q", v.lines[0], "First: Done")
	}
	if v.lines[1] != "Second: Running" {
		t.Errorf("line 1 = %q, want %q", v.lines[1], "Second: Running")
	}

	// afterChild should wrap any command
	if cmd == nil {
		t.Fatal("expected afterChild(nil) command")
	}
}

func TestHandleJobExited_LastJobClosesDialog(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	finishCalled := false
	var finishTask *aiTask
	onFinish := func(m Model, t *aiTask) (tea.Model, tea.Cmd) {
		finishCalled = true
		finishTask = t
		return m, nil
	}

	specs := []review.JobSpec{
		{Name: "Only", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, onFinish)
	m = updated.(Model)
	taskID := m.review.task.id

	updated, cmd := m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: nil})
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("task should be nil after last job exits")
	}
	if !finishCalled {
		t.Error("onFinish should be called when all jobs exit")
	}
	if finishTask == nil {
		t.Fatal("onFinish should receive the task")
	}
	if finishTask.jobs[0].state != jobDone {
		t.Errorf("job state = %v, want jobDone", finishTask.jobs[0].state)
	}

	// onFinish returns nil, so we expect afterChild(nil)
	if cmd == nil {
		t.Fatal("expected afterChild command from wrapped onFinish result")
	}
}

func TestHandleJobExited_JobFails(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)
	taskID := m.review.task.id

	jobErr := errors.New("something went wrong")
	updated, _ = m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: jobErr})
	m = updated.(Model)

	if m.review.task != nil {
		t.Error("task should be nil when single job exits")
	}

	// Can't check dialogView on nil task, but we can check the finished task via onFinish
	// Let's restart with an onFinish that captures the task
	var finishedTask *aiTask
	onFinish := func(m Model, t *aiTask) (tea.Model, tea.Cmd) {
		finishedTask = t
		return m, nil
	}

	m = withReviewState(onSelector(t, 5, 100, 30), starter, false)
	updated, _ = m.startTask("Test", specs, onFinish)
	m = updated.(Model)
	taskID = m.review.task.id

	_, _ = m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: jobErr})

	if finishedTask == nil {
		t.Fatal("onFinish should have been called")
	}
	if finishedTask.jobs[0].state != jobFailed {
		t.Errorf("job state = %v, want jobFailed", finishedTask.jobs[0].state)
	}
	if finishedTask.jobs[0].err != jobErr {
		t.Errorf("job err = %v, want %v", finishedTask.jobs[0].err, jobErr)
	}
}

func TestHandleJobExited_CancelledError(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	var finishedTask *aiTask
	onFinish := func(m Model, t *aiTask) (tea.Model, tea.Cmd) {
		finishedTask = t
		return m, nil
	}

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, onFinish)
	m = updated.(Model)
	taskID := m.review.task.id

	updated, _ = m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: review.ErrCancelled})
	m = updated.(Model)

	if finishedTask == nil {
		t.Fatal("onFinish should have been called")
	}
	if finishedTask.jobs[0].state != jobCancelled {
		t.Errorf("job state = %v, want jobCancelled", finishedTask.jobs[0].state)
	}
}

func TestHandleJobExited_OldTaskID(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	// Send message for a different task ID
	updated, _ = m.handleJobExited(jobExitedMsg{taskID: 999, index: 0, err: nil})
	m = updated.(Model)

	// Nothing should change
	if m.review.task.jobs[0].state != jobRunning {
		t.Errorf("job state = %v, want jobRunning (unchanged)", m.review.task.jobs[0].state)
	}
}

func TestHandleDialogTick_SchedulesNextTick(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)
	taskID := m.review.task.id

	_, cmd := m.handleDialogTick(dialogTickMsg{taskID: taskID})

	if cmd == nil {
		t.Fatal("handleDialogTick should schedule another tick while task is running")
	}
}

func TestHandleDialogTick_OldTaskID(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	// Tick for wrong task ID
	_, cmd := m.handleDialogTick(dialogTickMsg{taskID: 999})

	if cmd != nil {
		t.Error("handleDialogTick should return nil cmd for old task ID")
	}
}

func TestCancelTask(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "First", Model: "sonnet", Effort: "high", Prompt: "first"},
		{Name: "Second", Model: "opus", Effort: "high", Prompt: "second"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	m.cancelTask()

	if !m.review.task.cancelling {
		t.Error("task.cancelling should be true after cancelTask")
	}

	// Both processes should be cancelled
	if len(starter.procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(starter.procs))
	}
	for i, proc := range starter.procs {
		if !proc.cancelled {
			t.Errorf("process %d should be cancelled", i)
		}
	}

	// View should show "Cancelling…" for running jobs
	v, _ := m.dialogView()
	if v.lines[0] != fmt.Sprintf("First: %s", cancellingLabel) {
		t.Errorf("line 0 = %q, want First: %s", v.lines[0], cancellingLabel)
	}
	if v.lines[1] != fmt.Sprintf("Second: %s", cancellingLabel) {
		t.Errorf("line 1 = %q, want Second: %s", v.lines[1], cancellingLabel)
	}
}

func TestCancelTask_AlreadyCancelling(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	// First cancel
	m.cancelTask()
	firstCancelled := starter.procs[0].cancelled

	// Try to cancel again
	m.cancelTask()

	// Should not call Cancel again (our fake doesn't track this, but the
	// cancelling flag should prevent the second call)
	if !firstCancelled {
		t.Error("process should have been cancelled the first time")
	}
}

func TestCancelTask_OnlyRunningJobs(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "First", Model: "sonnet", Effort: "high", Prompt: "first"},
		{Name: "Second", Model: "opus", Effort: "high", Prompt: "second"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)
	taskID := m.review.task.id

	// First job exits
	updated, _ = m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: nil})
	m = updated.(Model)

	// Now cancel - only the second job should be cancelled
	m.cancelTask()

	if starter.procs[0].cancelled {
		t.Error("already-exited job should not be cancelled")
	}
	if !starter.procs[1].cancelled {
		t.Error("running job should be cancelled")
	}
}

func TestHandleDialogKey_EnterCancels(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	updated, _ = m.handleDialogKey(key("enter"))
	m = updated.(Model)

	if !m.review.task.cancelling {
		t.Error("enter on task dialog should cancel")
	}
	if !starter.procs[0].cancelled {
		t.Error("process should be cancelled")
	}
}

func TestHandleDialogKey_EscCancels(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	updated, _ = m.handleDialogKey(key("esc"))
	m = updated.(Model)

	if !m.review.task.cancelling {
		t.Error("esc on task dialog should cancel")
	}
}

func TestHandleDialogKey_OtherKeysIgnored(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	for _, k := range []string{"q", "C", "F", "up", "down", "j", "k"} {
		updated, cmd := m.handleDialogKey(key(k))
		m = updated.(Model)

		if m.review.task.cancelling {
			t.Errorf("key %q should not cancel task", k)
		}
		if cmd != nil {
			t.Errorf("key %q should return nil cmd", k)
		}
	}
}

func TestHandleDialogKey_NoticeEnterCloses(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	m.review.notice = &dialogView{
		title:  "Notice",
		lines:  []string{"Something happened"},
		button: okButton,
	}

	updated, _ := m.handleDialogKey(key("enter"))
	m = updated.(Model)

	if m.review.notice != nil {
		t.Error("enter should clear notice")
	}
}

func TestHandleDialogKey_NoticeEscCloses(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	m.review.notice = &dialogView{
		title:  "Notice",
		lines:  []string{"Something happened"},
		button: okButton,
	}

	updated, _ := m.handleDialogKey(key("esc"))
	m = updated.(Model)

	if m.review.notice != nil {
		t.Error("esc should clear notice")
	}
}

func TestHandleDialogKey_NoticeSpaceCloses(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	m.review.notice = &dialogView{
		title:  "Notice",
		lines:  []string{"Something happened"},
		button: okButton,
	}

	updated, _ := m.handleDialogKey(key(" "))
	m = updated.(Model)

	if m.review.notice != nil {
		t.Error("space should clear notice")
	}
}

func TestDialogView_NoDialog(t *testing.T) {
	m := onSelector(t, 5, 100, 30)

	_, ok := m.dialogView()
	if ok {
		t.Error("dialogView should return false when no dialog is open")
	}
}

func TestDialogView_Notice(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	m.review.notice = &dialogView{
		title:  "Test Notice",
		lines:  []string{"Line 1", "Line 2"},
		button: okButton,
	}

	v, ok := m.dialogView()
	if !ok {
		t.Fatal("dialogView should return true when notice is set")
	}
	if v.title != "Test Notice" {
		t.Errorf("title = %q, want Test Notice", v.title)
	}
	if len(v.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(v.lines))
	}
	if v.button != okButton {
		t.Errorf("button = %q, want %q", v.button, okButton)
	}
}

func TestAllExited_AllRunning(t *testing.T) {
	task := &aiTask{
		jobs: []taskJob{
			{name: "First", state: jobRunning},
			{name: "Second", state: jobRunning},
		},
	}

	if task.allExited() {
		t.Error("allExited should be false when jobs are running")
	}
}

func TestAllExited_SomeDone(t *testing.T) {
	task := &aiTask{
		jobs: []taskJob{
			{name: "First", state: jobDone},
			{name: "Second", state: jobRunning},
		},
	}

	if task.allExited() {
		t.Error("allExited should be false when some jobs are running")
	}
}

func TestAllExited_AllDone(t *testing.T) {
	task := &aiTask{
		jobs: []taskJob{
			{name: "First", state: jobDone},
			{name: "Second", state: jobFailed},
			{name: "Third", state: jobCancelled},
		},
	}

	if !task.allExited() {
		t.Error("allExited should be true when no jobs are running")
	}
}

func TestHandleJobExited_CancellingTask(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)

	var finishedTask *aiTask
	onFinish := func(m Model, t *aiTask) (tea.Model, tea.Cmd) {
		finishedTask = t
		return m, nil
	}

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, onFinish)
	m = updated.(Model)
	taskID := m.review.task.id

	// Cancel the task
	m.cancelTask()

	// Job exits with any error (not necessarily ErrCancelled)
	someErr := errors.New("interrupted")
	updated, _ = m.handleJobExited(jobExitedMsg{taskID: taskID, index: 0, err: someErr})
	m = updated.(Model)

	// Should be marked as cancelled, not failed, because the task is cancelling
	if finishedTask == nil {
		t.Fatal("onFinish should have been called")
	}
	if finishedTask.jobs[0].state != jobCancelled {
		t.Errorf("job state = %v, want jobCancelled when task is cancelling", finishedTask.jobs[0].state)
	}
}

func TestWaitJobCmd_ReturnsJobExitedMsg(t *testing.T) {
	proc := &fakeProcess{err: nil}

	cmd := waitJobCmd(42, 3, proc)
	if cmd == nil {
		t.Fatal("waitJobCmd should return a command")
	}

	msg := cmd()
	exitMsg, ok := msg.(jobExitedMsg)
	if !ok {
		t.Fatalf("expected jobExitedMsg, got %T", msg)
	}
	if exitMsg.taskID != 42 {
		t.Errorf("taskID = %d, want 42", exitMsg.taskID)
	}
	if exitMsg.index != 3 {
		t.Errorf("index = %d, want 3", exitMsg.index)
	}
	if exitMsg.err != nil {
		t.Errorf("err = %v, want nil", exitMsg.err)
	}
}

func TestWaitJobCmd_WithError(t *testing.T) {
	testErr := errors.New("job failed")
	proc := &fakeProcess{err: testErr}

	cmd := waitJobCmd(10, 0, proc)
	msg := cmd()
	exitMsg := msg.(jobExitedMsg)

	if exitMsg.err != testErr {
		t.Errorf("err = %v, want %v", exitMsg.err, testErr)
	}
}

func TestDialogTickCmd_ReturnsDialogTickMsg(t *testing.T) {
	cmd := dialogTickCmd(55)
	if cmd == nil {
		t.Fatal("dialogTickCmd should return a command")
	}

	// We can't easily test tea.Tick directly, but we can verify it returns a function
}

func TestStartTask_NextTaskIDIncremented(t *testing.T) {
	starter := &fakeStarter{}
	m := withReviewState(onSelector(t, 5, 100, 30), starter, false)
	m.review.nextTaskID = 10

	specs := []review.JobSpec{
		{Name: "Job", Model: "sonnet", Effort: "high", Prompt: "test"},
	}
	updated, _ := m.startTask("Test", specs, nil)
	m = updated.(Model)

	if m.review.task.id != 10 {
		t.Errorf("task.id = %d, want 10", m.review.task.id)
	}
	if m.review.nextTaskID != 11 {
		t.Errorf("nextTaskID = %d, want 11", m.review.nextTaskID)
	}
}
