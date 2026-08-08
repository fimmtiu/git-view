// Package git wraps the handful of git invocations the viewer needs.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Output runs a git command in the given directory and returns trimmed stdout.
func Output(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return "", wrapGitError(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoRoot returns the absolute path to the top level of the working tree
// containing dir. Diff paths are relative to this directory.
func RepoRoot(dir string) (string, error) {
	root, err := Output(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return root, nil
}

// Diff returns the raw unified diff for the given revision arguments, which
// are passed through to `git diff` untouched. No arguments means the working
// tree diff. Rename detection is on so the renderer can show renames.
func Diff(repoRoot string, revArgs []string) (string, error) {
	args := append([]string{"diff", "--find-renames"}, revArgs...)
	out, err := Output(repoRoot, args...)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// DescribeRevs returns a short human-readable label for the revision arguments,
// shown in the status bar. Revisions are abbreviated via `git rev-parse --short`
// when they resolve to a commit, so "HEAD~3" displays as its actual hash.
func DescribeRevs(repoRoot string, revArgs []string) string {
	if len(revArgs) == 0 {
		return "working tree"
	}
	parts := make([]string, 0, len(revArgs))
	for _, rev := range revArgs {
		parts = append(parts, abbreviateRev(repoRoot, rev))
	}
	return strings.Join(parts, " ")
}

// abbreviateRev replaces a single revision with its short hash when git can
// resolve it. Ranges ("a..b") have each end abbreviated separately; anything
// git cannot resolve (flags like --cached, or exotic notation) is returned
// unchanged.
func abbreviateRev(repoRoot, rev string) string {
	if strings.HasPrefix(rev, "-") {
		return rev
	}
	for _, sep := range []string{"...", ".."} {
		if before, after, found := strings.Cut(rev, sep); found {
			return abbreviateRev(repoRoot, before) + sep + abbreviateRev(repoRoot, after)
		}
	}
	if rev == "" {
		return rev
	}
	short, err := Output(repoRoot, "rev-parse", "--short", rev+"^{commit}")
	if err != nil || short == "" {
		return rev
	}
	return short
}

// wrapGitError attaches git's stderr to the error, which exec.Cmd.Output
// captures but does not include in the error message. Without this, a bad
// revision argument surfaces only as "exit status 128".
func wrapGitError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}
