package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// UncommittedHash is the sentinel hash for the "uncommitted changes"
// pseudo-commit.
const UncommittedHash = "????"

type CommitEntry struct {
	Hash    string
	Message string
}

func Output(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return "", wrapGitError(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoRoot returns the working tree's top level; diff paths are relative to it.
func RepoRoot(dir string) (string, error) {
	root, err := Output(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return root, nil
}

const (
	repoIcon = "📁 "
	// The branch glyph is ambiguous-width, so the trailing double space keeps
	// the branch name clear of it however the terminal renders it.
	branchIcon = " ⎇  "
)

// RepoLabel returns "📁 <repo-name> ⎇  <branch>", naming the repo from its
// origin remote and falling back to the directory name.
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

func repoNameFromOrigin(repoRoot string) string {
	originURL, err := Output(repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return repoNameFromURL(originURL)
}

// repoNameFromURL handles the HTTPS, SSH, scp-like, and local-path forms git
// accepts: "git@github.com:fimmtiu/git-view.git" → "git-view".
func repoNameFromURL(url string) string {
	// Trailing slashes come before the .git suffix: ".../repo.git/" is valid.
	url = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(url), "/"), ".git")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return url
}

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

// ForkPointIndex locates where the current branch diverged from the default
// branch, or -1 if it is not in commits. merge-base is used rather than
// --fork-point, which relies on the reflog and is unreliable in fresh clones.
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
	// out of the list; fall back to its nearest non-merge ancestor.
	fallback, err := Output(repoRoot, "log", "--no-merges", "--format=%H", "-1", forkHash)
	if err != nil || fallback == "" {
		return -1
	}
	return indexOfHash(commits, fallback)
}

func indexOfHash(commits []CommitEntry, hash string) int {
	for i, c := range commits {
		if c.Hash == hash {
			return i
		}
	}
	return -1
}

// HasUncommittedChanges reports whether plain `git diff` would produce output.
// Matching FetchDiff exactly — so untracked and staged-only changes don't count
// — keeps the pseudo-commit from opening an empty diff.
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

// FetchShowStat returns the commit message followed by its --stat file list.
func FetchShowStat(repoRoot, commitHash string) (string, error) {
	if commitHash == UncommittedHash {
		out, err := Output(repoRoot, "diff", "--stat")
		if err != nil {
			return "", err
		}
		// The blank line stands in for the absent subject, so the preview pane's
		// "bold the first line" rule doesn't bold a stat row.
		return "\n" + out, nil
	}
	return Output(repoRoot, "show", "--stat", "--format=%B", commitHash)
}

// FetchDiff returns the raw diff for a commit range, oldest first. A range
// starts at startCommit's parent so its own changes are included.
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

// git's well-known empty tree, standing in for the parent a root commit lacks.
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// diffBase returns hash's parent, or the empty tree for a root commit — git
// rejects "<root>^.." outright.
func diffBase(repoRoot, hash string) string {
	if _, err := Output(repoRoot, "rev-parse", "--verify", "--quiet", hash+"^{commit}"); err != nil {
		return hash + "^"
	}
	if _, err := Output(repoRoot, "rev-parse", "--verify", "--quiet", hash+"^^{commit}"); err != nil {
		return emptyTreeHash
	}
	return hash + "^"
}

// wrapGitError folds in the stderr that exec.Cmd.Output captures but omits from
// its error; without it a bad revision surfaces only as "exit status 128".
func wrapGitError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}
