package git

import (
	"fmt"
	"strconv"
	"strings"
)

// GitHubFileURL builds a blob URL, optionally anchored at a line. The ref must
// have been pushed for the URL to resolve.
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

// extractGitHubRepo pulls "owner/repo" from an SSH or HTTPS GitHub remote URL,
// returning "" for anything else.
func extractGitHubRepo(url string) string {
	if repo, ok := strings.CutPrefix(url, "git@github.com:"); ok {
		return strings.TrimSuffix(repo, ".git")
	}
	if _, after, found := strings.Cut(url, "github.com/"); found {
		repo := strings.TrimSuffix(after, ".git")
		parts := strings.SplitN(repo, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return ""
}
