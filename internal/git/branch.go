package git

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// BranchBase returns the full SHA of the merge base of the default branch and HEAD.
func BranchBase(repoRoot string) (string, error) {
	defaultBranch := DetectDefaultBranch(repoRoot)
	base, err := Output(repoRoot, "merge-base", defaultBranch, "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot find the branch base: %w", err)
	}
	return base, nil
}

// HeadCommit returns the full SHA of HEAD.
func HeadCommit(repoRoot string) (string, error) {
	head, err := Output(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot get HEAD commit: %w", err)
	}
	return head, nil
}

// HasBranchChanges reports whether HEAD has at least one commit that BranchBase does not.
func HasBranchChanges(repoRoot string) (bool, error) {
	base, err := BranchBase(repoRoot)
	if err != nil {
		return false, err
	}
	
	count, err := Output(repoRoot, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return false, err
	}
	
	n, err := strconv.Atoi(count)
	if err != nil {
		return false, fmt.Errorf("invalid commit count %q: %w", count, err)
	}
	
	return n > 0, nil
}

// IsClean reports whether the working tree has no staged, unstaged, or untracked
// changes. Ignored files do not count.
func IsClean(repoRoot string) (bool, error) {
	out, err := Output(repoRoot, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// RepoName names the repository from its origin remote, falling back to the
// directory name.
func RepoName(repoRoot string) string {
	name := repoNameFromOrigin(repoRoot)
	if name == "" {
		name = filepath.Base(repoRoot)
	}
	return name
}
