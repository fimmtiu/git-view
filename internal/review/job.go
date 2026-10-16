package review

import (
	"errors"
	"time"
)

const ClaudeBinary = "claude"

// CancelGracePeriod is how long Cancel waits after SIGTERM before it sends SIGKILL.
const CancelGracePeriod = 5 * time.Second

// ErrCancelled is what Wait returns for a job that Cancel stopped.
var ErrCancelled = errors.New("cancelled")

type JobSpec struct {
	Name   string // shown in the dialog
	Model  string
	Effort string
	Prompt string // written to the job's stdin
}

// Args returns the claude arguments for s. The prompt is not one of them.
func (s JobSpec) Args() []string { panic("scaffold stub") }

type Process interface {
	Wait() error
	Cancel()
}

type StartFunc func(repoRoot string, spec JobSpec) (Process, error)

// Start runs claude in repoRoot, in its own process group.
func Start(repoRoot string, spec JobSpec) (Process, error) { panic("scaffold stub") }

// ClaudeAvailable reports whether the claude binary is on PATH.
func ClaudeAvailable() bool { panic("scaffold stub") }
