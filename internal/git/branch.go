package git

// BranchBase returns the full SHA of the merge base of the default branch and HEAD.
func BranchBase(repoRoot string) (string, error) { panic("scaffold stub") }

// HeadCommit returns the full SHA of HEAD.
func HeadCommit(repoRoot string) (string, error) { panic("scaffold stub") }

// HasBranchChanges reports whether HEAD has at least one commit that BranchBase does not.
func HasBranchChanges(repoRoot string) (bool, error) { panic("scaffold stub") }

// IsClean reports whether the working tree has no staged, unstaged, or untracked
// changes. Ignored files do not count.
func IsClean(repoRoot string) (bool, error) { panic("scaffold stub") }

// RepoName names the repository from its origin remote, falling back to the
// directory name.
func RepoName(repoRoot string) string { panic("scaffold stub") }
