package git

import (
	"errors"
	"fmt"
	"os"
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
	return outputWithEnv(dir, nil, args...)
}

// outputWithEnv adds extra "NAME=value" entries to git's environment.
func outputWithEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
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

// HasUncommittedChanges reports whether the working tree holds modified tracked
// files or untracked ones. Matching what workingTreeDiff shows — so staged-only
// changes don't count — keeps the pseudo-commit from opening an empty diff.
func HasUncommittedChanges(repoRoot string) (bool, error) {
	if len(untrackedFiles(repoRoot)) > 0 {
		return true, nil
	}
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

// untrackedFiles lists the paths git reports as "??": untracked and not ignored.
// Paths are relative to repoRoot, and NUL separation keeps git from quoting the
// unusual ones.
func untrackedFiles(repoRoot string) []string {
	out, err := Output(repoRoot, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil
	}
	var files []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files
}

// workingTreeDiff runs a working-tree diff that includes untracked files by
// marking them intent-to-add in a scratch copy of the index. If that setup
// fails, the diff shows tracked files only.
func workingTreeDiff(repoRoot string, args ...string) (string, error) {
	untracked := untrackedFiles(repoRoot)
	if len(untracked) == 0 {
		return Output(repoRoot, args...)
	}

	indexFile, cleanup, err := copyIndex(repoRoot)
	if err != nil {
		return Output(repoRoot, args...)
	}
	defer cleanup()

	env := []string{"GIT_INDEX_FILE=" + indexFile}
	addArgs := append([]string{"add", "--intent-to-add", "--"}, untracked...)
	if _, err := outputWithEnv(repoRoot, env, addArgs...); err != nil {
		return Output(repoRoot, args...)
	}
	return outputWithEnv(repoRoot, env, args...)
}

// copyIndex writes a scratch copy of the repository's index file and returns its
// path together with a cleanup function.
func copyIndex(repoRoot string) (string, func(), error) {
	indexPath, err := Output(repoRoot, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", nil, err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(repoRoot, indexPath)
	}

	dir, err := os.MkdirTemp("", "git-view-index")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	copyPath := filepath.Join(dir, "index")

	data, err := os.ReadFile(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		// A repo that has never staged anything has no index yet, and an empty
		// file is not a valid one, so leave the copy for `git add` to create.
		return copyPath, cleanup, nil
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	return copyPath, cleanup, nil
}

// FetchShowStat returns the commit message followed by its --stat file list.
func FetchShowStat(repoRoot, commitHash string) (string, error) {
	if commitHash == UncommittedHash {
		out, err := workingTreeDiff(repoRoot, "diff", "--stat")
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
	run := workingTreeDiff
	switch {
	case startCommit.Hash == UncommittedHash && endCommit.Hash == UncommittedHash:
	case endCommit.Hash == UncommittedHash:
		args = append(args, startCommit.Hash)
	default:
		args = append(args, diffBase(repoRoot, startCommit.Hash)+".."+endCommit.Hash)
		// A commit range never reaches the working tree, so untracked files
		// have no place in it.
		run = Output
	}

	out, err := run(repoRoot, args...)
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
