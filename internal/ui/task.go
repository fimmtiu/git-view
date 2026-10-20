package ui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fimmtiu/git-view/internal/review"
)

func (s jobState) String() string {
	switch s {
	case jobRunning:
		return "Running"
	case jobDone:
		return "Done"
	case jobFailed:
		return "Failed"
	case jobCancelled:
		return "Cancelled"
	default:
		return fmt.Sprintf("jobState(%d)", s)
	}
}

func (m Model) startTask(title string, specs []review.JobSpec, onFinish taskFinishFunc) (tea.Model, tea.Cmd) {
	var procs []review.Process
	var jobs []taskJob

	for _, spec := range specs {
		proc, err := m.review.startJob(m.repoRoot, spec)
		if err != nil {
			for _, p := range procs {
				p.Cancel()
			}
			return m, statusCmd(fmt.Sprintf(jobStartFailedFormat, err), true)
		}
		procs = append(procs, proc)
		jobs = append(jobs, taskJob{
			name:  spec.Name,
			proc:  proc,
			state: jobRunning,
		})
	}

	taskID := m.review.nextTaskID
	m.review.nextTaskID++
	m.review.task = &aiTask{
		id:       taskID,
		title:    title,
		jobs:     jobs,
		started:  m.review.now(),
		onFinish: onFinish,
	}

	cmds := make([]tea.Cmd, 0, len(jobs)+1)
	for i, proc := range procs {
		cmds = append(cmds, waitJobCmd(taskID, i, proc))
	}
	cmds = append(cmds, dialogTickCmd(taskID))

	return m, tea.Batch(cmds...)
}

// waitJobCmd blocks in a Bubble Tea goroutine until p exits.
func waitJobCmd(taskID, index int, p review.Process) tea.Cmd {
	return func() tea.Msg {
		err := p.Wait()
		return jobExitedMsg{taskID: taskID, index: index, err: err}
	}
}

func dialogTickCmd(taskID int) tea.Cmd {
	return tea.Tick(dialogTickInterval, func(time.Time) tea.Msg {
		return dialogTickMsg{taskID: taskID}
	})
}

func (m Model) handleJobExited(msg jobExitedMsg) (tea.Model, tea.Cmd) {
	// Ignore if no task or wrong task
	if m.review.task == nil || m.review.task.id != msg.taskID {
		return m, afterChild(nil)
	}

	// Copy the jobs slice before modifying
	t := *m.review.task
	t.jobs = make([]taskJob, len(m.review.task.jobs))
	copy(t.jobs, m.review.task.jobs)

	// Update the job's state
	job := &t.jobs[msg.index]
	job.err = msg.err
	job.state = t.determineJobState(msg.err)

	m.review.task = &t

	// If all jobs have exited, call onFinish and clear the task
	if t.allExited() {
		m.review.task = nil
		if t.onFinish != nil {
			m, cmd := t.onFinish(m, &t)
			return m, afterChild(cmd)
		}
		return m, afterChild(nil)
	}

	return m, afterChild(nil)
}

func (m Model) handleDialogTick(msg dialogTickMsg) (tea.Model, tea.Cmd) {
	// Only schedule next tick if the task is still running
	if m.review.task != nil && m.review.task.id == msg.taskID {
		return m, dialogTickCmd(msg.taskID)
	}
	return m, nil
}

// cancelTask asks every running job in the current task to stop.
func (m *Model) cancelTask() {
	if m.review.task == nil || m.review.task.cancelling {
		return
	}

	// Copy the task before modifying
	t := *m.review.task
	t.cancelling = true
	m.review.task = &t

	// Cancel all running jobs
	for i := range t.jobs {
		if t.jobs[i].state == jobRunning {
			t.jobs[i].proc.Cancel()
		}
	}
}

// isDialogCloseKey returns true if the key closes dialogs.
func isDialogCloseKey(key string) bool {
	return key == "enter" || key == "esc"
}

// handleDialogKey takes every key while a dialog is open.
func (m Model) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Notice dialog: enter, esc, or space closes it
	if m.review.notice != nil {
		if isDialogCloseKey(key) || key == " " {
			m.review.notice = nil
		}
		return m, nil
	}

	// Task dialog: enter or esc cancels
	if m.review.task != nil {
		if isDialogCloseKey(key) {
			m.cancelTask()
		}
		return m, nil
	}

	return m, nil
}

// dialogView returns the open dialog's content, or false when none is open.
func (m Model) dialogView() (dialogView, bool) {
	// Task first
	if m.review.task != nil {
		t := m.review.task
		lines := make([]string, len(t.jobs))
		for i, job := range t.jobs {
			state := job.state.String()
			if t.cancelling && job.state == jobRunning {
				state = cancellingLabel
			}
			lines[i] = fmt.Sprintf("%s: %s", job.name, state)
		}
		return dialogView{
			title:       t.title,
			lines:       lines,
			elapsed:     m.review.now().Sub(t.started),
			showElapsed: true,
			button:      cancelButton,
		}, true
	}

	// Notice
	if m.review.notice != nil {
		return *m.review.notice, true
	}

	return dialogView{}, false
}

func (t *aiTask) allExited() bool {
	for _, job := range t.jobs {
		if job.state == jobRunning {
			return false
		}
	}
	return true
}

func (t *aiTask) determineJobState(err error) jobState {
	if errors.Is(err, review.ErrCancelled) || t.cancelling {
		return jobCancelled
	} else if err != nil {
		return jobFailed
	}
	return jobDone
}
