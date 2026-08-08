package git

import (
	"fmt"
	"strconv"
	"strings"
)

// GitHubFileURL builds a github.com blob URL for a file in the repository,
// optionally anchored at a line number. The ref is the current branch when the
// working tree is on one, and the HEAD commit hash when detached — either way,
// the target must have been pushed for the URL to resolve.
func GitHubFileURL(repoRoot, file string, line int) (string, error) {
	originURL, err := Output(repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("no origin remote: %w", err)
	}
	repo := extractGitHubRepo(originURL)
	if repo == "" {
		return "", fmt.Errorf("origin is not a github.com remote: %s", originURL)
	}
	ref, err := currentRef(repoRoot)
	if err != nil {
		return "", err
	}

	url := "https://github.com/" + repo + "/blob/" + ref + "/" + file
	if line > 0 {
		url += "#L" + strconv.Itoa(line)
	}
	return url, nil
}

// currentRef returns the checked-out branch name, falling back to the HEAD
// commit hash when HEAD is detached.
func currentRef(repoRoot string) (string, error) {
	if branch, err := Output(repoRoot, "branch", "--show-current"); err == nil && branch != "" {
		return branch, nil
	}
	hash, err := Output(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot resolve HEAD: %w", err)
	}
	return hash, nil
}

// extractGitHubRepo extracts "owner/repo" from a GitHub remote URL.
// Handles both SSH (git@github.com:owner/repo.git) and HTTPS
// (https://github.com/owner/repo.git) formats. Returns "" for non-GitHub
// remotes.
func extractGitHubRepo(url string) string {
	// SSH format: git@github.com:owner/repo.git
	if repo, ok := strings.CutPrefix(url, "git@github.com:"); ok {
		return strings.TrimSuffix(repo, ".git")
	}
	// HTTPS format: https://github.com/owner/repo.git
	if _, after, found := strings.Cut(url, "github.com/"); found {
		repo := strings.TrimSuffix(after, ".git")
		// Strip trailing path segments beyond owner/repo.
		parts := strings.SplitN(repo, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return ""
}
