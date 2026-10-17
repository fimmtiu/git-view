package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func (p *fakeProcess) Wait() error {
	return p.err
}

func (p *fakeProcess) Cancel() {
	p.cancelled = true
}

// fakeStarter records the jobs it starts. Start fails for the job named failName,
// and a job whose name is a key of errs gets that error from Wait.
type fakeStarter struct {
	procs    []*fakeProcess
	failName string
	errs     map[string]error
}

func (f *fakeStarter) start(repoRoot string, spec review.JobSpec) (review.Process, error) {
	if spec.Name == f.failName {
		return nil, fmt.Errorf("failed to start job %q", spec.Name)
	}

	proc := &fakeProcess{
		spec: spec,
		err:  f.errs[spec.Name],
	}
	f.procs = append(f.procs, proc)
	return proc, nil
}

var fixedNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// withReviewState returns m with claude found, branch changes present, the
// findings file present or not as given, starter.start as the job starter, and
// a clock stopped at fixedNow.
func withReviewState(m Model, starter *fakeStarter, findings bool) Model {
	m.review.claudeFound = true
	m.review.hasBranchChanges = true
	m.review.findingsExist = findings
	m.review.startJob = starter.start
	m.review.now = func() time.Time {
		return fixedNow
	}
	return m
}

// newBranchRepo creates a clean repository with a "main" branch and a checked-out
// "feature" branch one commit ahead of it, and returns its path.
func newBranchRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
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

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Initialize repository with main branch
	run("init", "--initial-branch=main")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")

	// First commit on main
	write("first.txt", "first\n")
	run("add", ".")
	run("commit", "-m", "first commit")

	// Create and checkout feature branch
	run("checkout", "-b", "feature")

	// Second commit on feature
	write("second.txt", "second\n")
	run("add", ".")
	run("commit", "-m", "feature commit")

	return dir
}

// TestNewBranchRepo verifies the test helper creates a valid branch repository.
func TestNewBranchRepo(t *testing.T) {
	dir := newBranchRepo(t)

	// Verify we're on feature branch
	cmd := exec.Command("git", "-C", dir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch: %v", err)
	}
	if strings.TrimSpace(string(out)) != "feature" {
		t.Errorf("expected feature branch, got %q", strings.TrimSpace(string(out)))
	}

	// Verify there are two commits
	cmd = exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Errorf("expected 2 commits, got %q", strings.TrimSpace(string(out)))
	}

	// Verify feature is one commit ahead of main
	cmd = exec.Command("git", "-C", dir, "rev-list", "--count", "main..HEAD")
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git rev-list main..HEAD: %v", err)
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected 1 commit ahead of main, got %q", strings.TrimSpace(string(out)))
	}
}
