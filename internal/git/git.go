// Package git wraps the handful of git invocations the viewer needs.
package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// UncommittedHash is the sentinel hash for the "uncommitted changes"
// pseudo-commit that heads the commit list when the working tree is dirty.
const UncommittedHash = "????"

// CommitEntry represents one commit in a log listing.
type CommitEntry struct {
	Hash    string
	Message string
}

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

// Icons for the status bar label. The branch symbol is followed by two spaces
// because terminals disagree on its width — it is an ambiguous-width character,
// and the extra space keeps the branch name clear of it either way.
const (
	repoIcon   = "📁 "
	branchIcon = " ⎇  "
)

// RepoLabel returns "📁 <repo-name> ⎇  <branch>" for the status bar. The name
// comes from the origin remote, falling back to the working tree's directory
// name when there is no origin or its URL cannot be parsed. A detached HEAD
// shows a short commit hash in place of the branch, and the branch segment is
// dropped entirely if neither can be read.
func RepoLabel(repoRoot string) string {
	name := repoNameFromOrigin(repoRoot)
	if name == "" {
		name = filepath.Base(repoRoot)
	}
	label := repoIcon + name

	if branch, err := Output(repoRoot, "branch", "--show-current"); err == nil && branch != "" {
		return label + branchIcon + branch
	}
	if hash, err := Output(repoRoot, "rev-parse", "--short", "HEAD"); err == nil && hash != "" {
		return label + branchIcon + "detached at " + hash
	}
	return label
}

// repoNameFromOrigin returns the repository name taken from the origin remote's
// URL, or "" when there is no origin remote or nothing can be parsed from it.
func repoNameFromOrigin(repoRoot string) string {
	originURL, err := Output(repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return repoNameFromURL(originURL)
}

// repoNameFromURL extracts the repository name from a remote URL. It handles the
// HTTPS, SSH, scp-like, and local-path forms git accepts:
//
//	https://github.com/fimmtiu/git-view.git  → git-view
//	git@github.com:fimmtiu/git-view.git      → git-view
//	ssh://git@host:2222/~user/repo.git/      → repo
//	/srv/git/repo.git                        → repo
func repoNameFromURL(url string) string {
	// Trailing slashes come before the .git suffix: ".../repo.git/" is valid.
	url = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(url), "/"), ".git")
	// The name is whatever follows the last path or scp-style separator.
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return url
}

// DetectDefaultBranch returns "main" or "master" depending on which branch
// exists in the repository.
func DetectDefaultBranch(repoRoot string) string {
	if out, err := Output(repoRoot, "rev-parse", "--verify", "main"); err == nil && out != "" {
		return "main"
	}
	return "master"
}

// FetchCommitList returns the most recent commits, newest first. Merge commits
// are omitted; the viewer has nothing useful to show for them.
func FetchCommitList(repoRoot string, maxCommits int) ([]CommitEntry, error) {
	out, err := Output(repoRoot, "log", "--no-merges",
		"--format=%H %s", fmt.Sprintf("-%d", maxCommits))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var commits []CommitEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		hash, message, _ := strings.Cut(line, " ")
		commits = append(commits, CommitEntry{Hash: hash, Message: message})
	}
	return commits, nil
}

// ForkPointIndex returns the index in commits of the point where the current
// branch diverged from the default branch, or -1 if it cannot be located. The
// commit list is used as given, so the caller decides how far back to look.
//
// merge-base is used rather than --fork-point, which relies on the reflog and
// is unreliable in fresh clones and worktrees.
func ForkPointIndex(repoRoot string, commits []CommitEntry) int {
	defaultBranch := DetectDefaultBranch(repoRoot)
	forkHash, err := Output(repoRoot, "merge-base", defaultBranch, "HEAD")
	if err != nil || forkHash == "" {
		return -1
	}
	if idx := indexOfHash(commits, forkHash); idx >= 0 {
		return idx
	}
	// The fork point may itself be a merge commit, which --no-merges filtered
	// out of the list. Fall back to its nearest non-merge ancestor.
	fallback, err := Output(repoRoot, "log", "--no-merges", "--format=%H", "-1", forkHash)
	if err != nil || fallback == "" {
		return -1
	}
	return indexOfHash(commits, fallback)
}

// indexOfHash returns the position of hash in commits, or -1.
func indexOfHash(commits []CommitEntry, hash string) int {
	for i, c := range commits {
		if c.Hash == hash {
			return i
		}
	}
	return -1
}

// HasUncommittedChanges reports whether `git diff` would produce output — that
// is, whether there are unstaged modifications to tracked files. This
// intentionally matches FetchDiff and FetchShowStat, both of which call plain
// `git diff`, so the "uncommitted changes" pseudo-commit is only offered when
// selecting it would actually show something. Untracked files and staged-only
// changes do not count.
func HasUncommittedChanges(repoRoot string) (bool, error) {
	err := exec.Command("git", "-C", repoRoot, "diff", "--quiet").Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

// FetchShowStat returns the commit message followed by the --stat file list for
// the given commit. For the uncommitted pseudo-commit it returns `git diff
// --stat` prefixed with a blank line, so the preview pane treats its first line
// the same as a commit subject (which it bolds).
func FetchShowStat(repoRoot, commitHash string) (string, error) {
	if commitHash == UncommittedHash {
		out, err := Output(repoRoot, "diff", "--stat")
		if err != nil {
			return "", err
		}
		return "\n" + out, nil
	}
	return Output(repoRoot, "show", "--stat", "--format=%B", commitHash)
}

// FetchDiff returns the raw diff for a range of commits, oldest first.
// It handles three cases:
//   - Both ends uncommitted: the working tree diff.
//   - The newer end uncommitted: from startCommit to the working tree.
//   - A normal range: from the parent of startCommit through endCommit, so the
//     starting commit's own changes are included.
func FetchDiff(repoRoot string, startCommit, endCommit CommitEntry) (string, error) {
	args := []string{"diff", "--find-renames"}
	switch {
	case startCommit.Hash == UncommittedHash && endCommit.Hash == UncommittedHash:
	case endCommit.Hash == UncommittedHash:
		args = append(args, startCommit.Hash)
	default:
		args = append(args, diffBase(repoRoot, startCommit.Hash)+".."+endCommit.Hash)
	}

	out, err := Output(repoRoot, args...)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// emptyTreeHash is git's well-known hash for an empty tree. Diffing against it
// yields "everything in this commit was added", which is what the parent of the
// root commit would produce if it existed.
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// diffBase returns the revision to diff from so that hash's own changes are
// included: normally its parent, but the empty tree for a root commit, which
// has no parent and would otherwise make git reject the range outright.
func diffBase(repoRoot, hash string) string {
	if _, err := Output(repoRoot, "rev-parse", "--verify", "--quiet", hash+"^{commit}"); err != nil {
		return hash + "^"
	}
	if _, err := Output(repoRoot, "rev-parse", "--verify", "--quiet", hash+"^^{commit}"); err != nil {
		return emptyTreeHash
	}
	return hash + "^"
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
