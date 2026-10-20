package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranchBase(t *testing.T) {
	dir := newBranchRepo(t)
	
	base, err := BranchBase(dir)
	if err != nil {
		t.Fatalf("BranchBase: %v", err)
	}
	
	// The base should be a full SHA
	if len(base) != 40 {
		t.Errorf("BranchBase = %q, want a full 40-character SHA", base)
	}
	
	// The base should be main's HEAD
	mainHead, err := Output(dir, "rev-parse", "main")
	if err != nil {
		t.Fatalf("rev-parse main: %v", err)
	}
	if base != mainHead {
		t.Errorf("BranchBase = %q, want main's HEAD %q", base, mainHead)
	}
}

func TestBranchBase_OnDefaultBranch(t *testing.T) {
	dir := newBranchRepo(t)
	run(t, dir, "checkout", "main")
	
	base, err := BranchBase(dir)
	if err != nil {
		t.Fatalf("BranchBase: %v", err)
	}
	
	// On main, the merge base is HEAD itself
	head, err := Output(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if base != head {
		t.Errorf("BranchBase on main = %q, want HEAD %q", base, head)
	}
}

func TestHeadCommit(t *testing.T) {
	dir := newBranchRepo(t)
	
	head, err := HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	
	// Should be a full SHA
	if len(head) != 40 {
		t.Errorf("HeadCommit = %q, want a full 40-character SHA", head)
	}
	
	// Should match git rev-parse HEAD
	expected, err := Output(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if head != expected {
		t.Errorf("HeadCommit = %q, want %q", head, expected)
	}
}

func TestHasBranchChanges_OnFeatureBranch(t *testing.T) {
	dir := newBranchRepo(t)
	
	hasChanges, err := HasBranchChanges(dir)
	if err != nil {
		t.Fatalf("HasBranchChanges: %v", err)
	}
	
	if !hasChanges {
		t.Error("HasBranchChanges on feature branch = false, want true")
	}
}

func TestHasBranchChanges_OnDefaultBranch(t *testing.T) {
	dir := newBranchRepo(t)
	run(t, dir, "checkout", "main")
	
	hasChanges, err := HasBranchChanges(dir)
	if err != nil {
		t.Fatalf("HasBranchChanges: %v", err)
	}
	
	if hasChanges {
		t.Error("HasBranchChanges on main = true, want false")
	}
}

func TestIsClean_FreshRepo(t *testing.T) {
	dir := newBranchRepo(t)
	
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	
	if !clean {
		t.Error("IsClean on fresh repo = false, want true")
	}
}

func TestIsClean_UnstagedEdit(t *testing.T) {
	dir := newBranchRepo(t)
	
	// Modify a tracked file
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	
	if clean {
		t.Error("IsClean with unstaged edit = true, want false")
	}
}

func TestIsClean_StagedEdit(t *testing.T) {
	dir := newBranchRepo(t)
	
	// Modify and stage a file
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "one.txt")
	
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	
	if clean {
		t.Error("IsClean with staged edit = true, want false")
	}
}

func TestIsClean_UntrackedFile(t *testing.T) {
	dir := newBranchRepo(t)
	
	// Add an untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	
	if clean {
		t.Error("IsClean with untracked file = true, want false")
	}
}

func TestIsClean_IgnoredFile(t *testing.T) {
	dir := newBranchRepo(t)
	
	// Create .gitignore and commit it
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".gitignore")
	run(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add gitignore")
	
	// Add an ignored file
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	
	clean, err := IsClean(dir)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	
	if !clean {
		t.Error("IsClean with only ignored file = false, want true")
	}
}

func TestRepoName_WithOrigin(t *testing.T) {
	dir := newBranchRepo(t)
	run(t, dir, "remote", "add", "origin", "git@github.com:fimmtiu/git-view.git")
	
	name := RepoName(dir)
	if name != "git-view" {
		t.Errorf("RepoName with origin = %q, want git-view", name)
	}
}

func TestRepoName_WithoutOrigin(t *testing.T) {
	dir := newBranchRepo(t)
	
	name := RepoName(dir)
	expected := filepath.Base(dir)
	if name != expected {
		t.Errorf("RepoName without origin = %q, want directory base %q", name, expected)
	}
}

func TestRepoName_HTTPSOrigin(t *testing.T) {
	dir := newBranchRepo(t)
	run(t, dir, "remote", "add", "origin", "https://github.com/user/my-repo.git")
	
	name := RepoName(dir)
	if name != "my-repo" {
		t.Errorf("RepoName with HTTPS origin = %q, want my-repo", name)
	}
}

// newBranchRepo creates a clean repository with a "main" branch and a checked-out
// "feature" branch one commit ahead of it, and returns its path.
func newBranchRepo(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	
	// Create a feature branch with one extra commit
	run(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "feature.txt")
	run(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "feature commit")
	
	return dir
}

func TestHeadCommit_ErrorContext(t *testing.T) {
	// Use a non-existent directory to trigger an error
	_, err := HeadCommit("/nonexistent/path")
	if err == nil {
		t.Fatal("HeadCommit with invalid path: got nil error, want error")
	}
	
	// Error should include context about what failed
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("HeadCommit error message is empty")
	}
	// The error should mention HEAD or commit to provide context
	hasContext := false
	for _, word := range []string{"HEAD", "commit"} {
		if len(errMsg) >= len(word) {
			for i := 0; i <= len(errMsg)-len(word); i++ {
				if errMsg[i:i+len(word)] == word {
					hasContext = true
					break
				}
			}
		}
	}
	if !hasContext {
		t.Errorf("HeadCommit error = %q, should mention HEAD or commit for context", errMsg)
	}
}
