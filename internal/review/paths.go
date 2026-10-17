// Package review runs the Claude jobs behind git-view's code review and fix commands, and owns the .gv-review directory they write to.
package review

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	Dir          = ".gv-review"
	SummaryFile  = "summary.md"
	FindingsFile = "code-review-findings.md"
)

func SummaryPath(repoRoot string) string {
	return filepath.Join(repoRoot, Dir, SummaryFile)
}

func FindingsPath(repoRoot string) string {
	return filepath.Join(repoRoot, Dir, FindingsFile)
}

// isRegularFile checks if the path exists and is a regular file.
func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// removeIfExists removes a file, ignoring "not exist" errors.
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Prepare creates Dir and removes old outputs, so a missing file after a run means the job did not write it.
func Prepare(repoRoot string) error {
	dirPath := filepath.Join(repoRoot, Dir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	
	if err := removeIfExists(SummaryPath(repoRoot)); err != nil {
		return err
	}
	
	if err := removeIfExists(FindingsPath(repoRoot)); err != nil {
		return err
	}
	
	return nil
}

func FindingsExist(repoRoot string) bool {
	return isRegularFile(FindingsPath(repoRoot))
}

// MissingOutputs lists, repo-relative and summary first, the outputs that do not exist.
func MissingOutputs(repoRoot string) []string {
	var missing []string
	
	if !isRegularFile(SummaryPath(repoRoot)) {
		missing = append(missing, filepath.Join(Dir, SummaryFile))
	}
	
	if !isRegularFile(FindingsPath(repoRoot)) {
		missing = append(missing, filepath.Join(Dir, FindingsFile))
	}
	
	if len(missing) == 0 {
		return nil
	}
	
	return missing
}
