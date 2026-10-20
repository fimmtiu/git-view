package review

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSummaryPath(t *testing.T) {
	root := t.TempDir()
	got := SummaryPath(root)
	want := filepath.Join(root, Dir, SummaryFile)
	if got != want {
		t.Errorf("SummaryPath(%q) = %q, want %q", root, got, want)
	}
}

func TestFindingsPath(t *testing.T) {
	root := t.TempDir()
	got := FindingsPath(root)
	want := filepath.Join(root, Dir, FindingsFile)
	if got != want {
		t.Errorf("FindingsPath(%q) = %q, want %q", root, got, want)
	}
}

func TestPrepare_CreatesDirectory(t *testing.T) {
	root := t.TempDir()
	
	err := Prepare(root)
	if err != nil {
		t.Fatalf("Prepare(%q) = %v, want nil", root, err)
	}
	
	dirPath := filepath.Join(root, Dir)
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory %q not created: %v", dirPath, err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", dirPath)
	}
}

func TestPrepare_RemovesOldOutputs(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and old output files
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	summaryPath := filepath.Join(dirPath, SummaryFile)
	findingsPath := filepath.Join(dirPath, FindingsFile)
	
	if err := os.WriteFile(summaryPath, []byte("old summary"), 0o644); err != nil {
		t.Fatalf("Failed to create old summary: %v", err)
	}
	if err := os.WriteFile(findingsPath, []byte("old findings"), 0o644); err != nil {
		t.Fatalf("Failed to create old findings: %v", err)
	}
	
	// Prepare should remove them
	err := Prepare(root)
	if err != nil {
		t.Fatalf("Prepare(%q) = %v, want nil", root, err)
	}
	
	// Check that files are removed
	if _, err := os.Stat(summaryPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Summary file still exists after Prepare")
	}
	if _, err := os.Stat(findingsPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Findings file still exists after Prepare")
	}
}

func TestPrepare_LeavesOtherFilesAlone(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and another file
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	otherFile := filepath.Join(dirPath, "other.txt")
	if err := os.WriteFile(otherFile, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("Failed to create other file: %v", err)
	}
	
	// Prepare should not remove it
	err := Prepare(root)
	if err != nil {
		t.Fatalf("Prepare(%q) = %v, want nil", root, err)
	}
	
	// Check that other file still exists
	data, err := os.ReadFile(otherFile)
	if err != nil {
		t.Errorf("Other file was removed: %v", err)
	}
	if string(data) != "keep me" {
		t.Errorf("Other file content changed: got %q, want %q", data, "keep me")
	}
}

func TestPrepare_DirectoryAlreadyExists(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory first
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	// Prepare should succeed
	err := Prepare(root)
	if err != nil {
		t.Fatalf("Prepare(%q) = %v, want nil when directory exists", root, err)
	}
}

func TestFindingsExist_MissingFile(t *testing.T) {
	root := t.TempDir()
	
	got := FindingsExist(root)
	if got {
		t.Errorf("FindingsExist(%q) = true, want false when file missing", root)
	}
}

func TestFindingsExist_Directory(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory structure where findings path is a directory
	if err := os.MkdirAll(filepath.Join(dirPath, FindingsFile), 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	got := FindingsExist(root)
	if got {
		t.Errorf("FindingsExist(%q) = true, want false when path is a directory", root)
	}
}

func TestFindingsExist_File(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and findings file
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	findingsPath := filepath.Join(dirPath, FindingsFile)
	if err := os.WriteFile(findingsPath, []byte("findings"), 0o644); err != nil {
		t.Fatalf("Failed to create findings file: %v", err)
	}
	
	got := FindingsExist(root)
	if !got {
		t.Errorf("FindingsExist(%q) = false, want true when file exists", root)
	}
}

func TestMissingOutputs_BothExist(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and both files
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	summaryPath := filepath.Join(dirPath, SummaryFile)
	findingsPath := filepath.Join(dirPath, FindingsFile)
	
	if err := os.WriteFile(summaryPath, []byte("summary"), 0o644); err != nil {
		t.Fatalf("Failed to create summary: %v", err)
	}
	if err := os.WriteFile(findingsPath, []byte("findings"), 0o644); err != nil {
		t.Fatalf("Failed to create findings: %v", err)
	}
	
	got := MissingOutputs(root)
	if got != nil {
		t.Errorf("MissingOutputs(%q) = %v, want nil when both exist", root, got)
	}
}

func TestMissingOutputs_BothMissing(t *testing.T) {
	root := t.TempDir()
	
	got := MissingOutputs(root)
	want := []string{
		filepath.Join(Dir, SummaryFile),
		filepath.Join(Dir, FindingsFile),
	}
	
	if len(got) != len(want) {
		t.Fatalf("MissingOutputs(%q) returned %d items, want %d", root, len(got), len(want))
	}
	
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MissingOutputs(%q)[%d] = %q, want %q", root, i, got[i], want[i])
		}
	}
}

func TestMissingOutputs_OnlySummaryExists(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and only summary file
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	summaryPath := filepath.Join(dirPath, SummaryFile)
	if err := os.WriteFile(summaryPath, []byte("summary"), 0o644); err != nil {
		t.Fatalf("Failed to create summary: %v", err)
	}
	
	got := MissingOutputs(root)
	want := []string{filepath.Join(Dir, FindingsFile)}
	
	if len(got) != len(want) {
		t.Fatalf("MissingOutputs(%q) returned %d items, want %d", root, len(got), len(want))
	}
	
	if got[0] != want[0] {
		t.Errorf("MissingOutputs(%q)[0] = %q, want %q", root, got[0], want[0])
	}
}

func TestMissingOutputs_OnlyFindingsExists(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	// Create directory and only findings file
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	findingsPath := filepath.Join(dirPath, FindingsFile)
	if err := os.WriteFile(findingsPath, []byte("findings"), 0o644); err != nil {
		t.Fatalf("Failed to create findings: %v", err)
	}
	
	got := MissingOutputs(root)
	want := []string{filepath.Join(Dir, SummaryFile)}
	
	if len(got) != len(want) {
		t.Fatalf("MissingOutputs(%q) returned %d items, want %d", root, len(got), len(want))
	}
	
	if got[0] != want[0] {
		t.Errorf("MissingOutputs(%q)[0] = %q, want %q", root, got[0], want[0])
	}
}

func TestExistingOutputs_NoneExist(t *testing.T) {
	root := t.TempDir()
	
	if got := ExistingOutputs(root); len(got) != 0 {
		t.Errorf("ExistingOutputs(%q) = %v, want none", root, got)
	}
}

func TestExistingOutputs_OnlyFindingsExists(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(FindingsPath(root), []byte("findings"), 0o644); err != nil {
		t.Fatalf("Failed to create findings: %v", err)
	}
	
	got := ExistingOutputs(root)
	if len(got) != 1 || got[0] != FindingsPath(root) {
		t.Errorf("ExistingOutputs(%q) = %v, want [%s]", root, got, FindingsPath(root))
	}
}

func TestExistingOutputs_BothExist(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, Dir)
	
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(SummaryPath(root), []byte("summary"), 0o644); err != nil {
		t.Fatalf("Failed to create summary: %v", err)
	}
	if err := os.WriteFile(FindingsPath(root), []byte("findings"), 0o644); err != nil {
		t.Fatalf("Failed to create findings: %v", err)
	}
	
	got := ExistingOutputs(root)
	want := []string{SummaryPath(root), FindingsPath(root)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ExistingOutputs(%q) = %v, want %v", root, got, want)
	}
}
