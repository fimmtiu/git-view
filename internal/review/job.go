package review

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const ClaudeBinary = "claude"

// CancelGracePeriod is how long Cancel waits after SIGTERM before it sends SIGKILL.
const CancelGracePeriod = 5 * time.Second

// ErrCancelled is what Wait returns for a job that Cancel stopped.
var ErrCancelled = errors.New("cancelled")

// stderrBufferSize is the maximum number of bytes of stderr to keep for error messages.
const stderrBufferSize = 4096

// claudeBinary is the name of the claude executable. Tests can override this.
var claudeBinary = ClaudeBinary

type JobSpec struct {
	Name   string // shown in the dialog
	Model  string
	Effort string
	Prompt string // written to the job's stdin
}

// Args returns the claude arguments for s. The prompt is not one of them.
func (s JobSpec) Args() []string {
	return []string{"--print", "--model", s.Model, "--effort", s.Effort}
}

type Process interface {
	Wait() error
	Cancel()
}

type StartFunc func(repoRoot string, spec JobSpec) (Process, error)

// Start runs claude in repoRoot, in its own process group.
func Start(repoRoot string, spec JobSpec) (Process, error) {
	cmd := exec.Command(claudeBinary, spec.Args()...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(spec.Prompt)
	cmd.Stdout = io.Discard

	stderrBuf := &limitedBuffer{limit: stderrBufferSize}
	cmd.Stderr = stderrBuf

	// Create a new process group so the TUI's Ctrl+C doesn't reach this job
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &process{
		cmd:       cmd,
		spec:      spec,
		stderrBuf: stderrBuf,
	}, nil
}

// ClaudeAvailable reports whether the claude binary is on PATH.
func ClaudeAvailable() bool {
	_, err := exec.LookPath(claudeBinary)
	return err == nil
}

// process implements the Process interface.
type process struct {
	cmd       *exec.Cmd
	spec      JobSpec
	stderrBuf *limitedBuffer

	mu         sync.Mutex
	waitCalled bool
	exited     bool
	cancelled  bool
	waitErr    error
	killTimer  *time.Timer
}

// Wait waits for the process to exit. It can be called only once.
func (p *process) Wait() error {
	p.mu.Lock()
	if p.waitCalled {
		err := p.waitErr
		p.mu.Unlock()
		return err
	}
	p.waitCalled = true
	p.mu.Unlock()

	err := p.cmd.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.exited = true

	if p.killTimer != nil {
		p.killTimer.Stop()
		p.killTimer = nil
	}

	if p.cancelled {
		p.waitErr = ErrCancelled
		return ErrCancelled
	}

	if err != nil {
		stderr := strings.TrimSpace(p.stderrBuf.String())
		if stderr != "" {
			stderr = ": " + stderr
		}
		p.waitErr = fmt.Errorf("%s: %w%s", p.spec.Name, err, stderr)
		return p.waitErr
	}

	return nil
}

// Cancel asks the process to stop. It is safe to call multiple times and
// concurrently with Wait.
func (p *process) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelled || p.exited {
		return
	}

	p.cancelled = true

	if p.cmd.Process == nil {
		return
	}

	pid := p.cmd.Process.Pid

	_ = syscall.Kill(-pid, syscall.SIGTERM)

	p.killTimer = time.AfterFunc(CancelGracePeriod, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if !p.exited && p.cmd.Process != nil {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	})
}

// limitedBuffer keeps only the last limit bytes written to it.
type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (n int, err error) {
	n = len(p)

	b.buf.Write(p)

	if b.buf.Len() > b.limit {
		data := b.buf.Bytes()
		b.buf.Reset()
		b.buf.Write(data[len(data)-b.limit:])
	}

	return n, nil
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}
