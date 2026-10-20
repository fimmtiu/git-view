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

// outputFiles is the list of output files created by review and fix jobs.
var outputFiles = []string{SummaryFile, FindingsFile}

// outputPath returns the absolute path for a named output file.
func outputPath(repoRoot, filename string) string {
	return filepath.Join(repoRoot, Dir, filename)
}

func SummaryPath(repoRoot string) string {
	return outputPath(repoRoot, SummaryFile)
}

func FindingsPath(repoRoot string) string {
	return outputPath(repoRoot, FindingsFile)
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
	
	for _, filename := range outputFiles {
		if err := removeIfExists(outputPath(repoRoot, filename)); err != nil {
			return err
		}
	}
	
	return nil
}

func FindingsExist(repoRoot string) bool {
	return isRegularFile(FindingsPath(repoRoot))
}

// MissingOutputs lists, repo-relative and summary first, the outputs that do not exist.
func MissingOutputs(repoRoot string) []string {
	var missing []string
	
	for _, filename := range outputFiles {
		if !isRegularFile(outputPath(repoRoot, filename)) {
			missing = append(missing, filepath.Join(Dir, filename))
		}
	}
	
	if len(missing) == 0 {
		return nil
	}
	
	return missing
}

// ExistingOutputs lists, as absolute paths and summary first, the outputs that exist.
func ExistingOutputs(repoRoot string) []string {
	var existing []string
	
	for _, filename := range outputFiles {
		path := outputPath(repoRoot, filename)
		if isRegularFile(path) {
			existing = append(existing, path)
		}
	}
	
	return existing
}
