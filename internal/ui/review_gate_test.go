package ui

import (
	"os"
	"path/filepath"
	"testing"
	"os/exec"


	"github.com/fimmtiu/git-view/internal/review"
)

// TestReviewKeys tests the reviewKeys() method with all combinations of conditions.
func TestReviewKeys(t *testing.T) {
	tests := []struct {
		name             string
		claudeFound      bool
		hasBranchChanges bool
		findingsExist    bool
		taskRunning      bool
		wantReview       bool
		wantFix          bool
	}{
		{
			name:             "all conditions met",
			claudeFound:      true,
			hasBranchChanges: true,
			findingsExist:    true,
			taskRunning:      false,
			wantReview:       true,
			wantFix:          true,
		},
		{
			name:             "claude not found",
			claudeFound:      false,
			hasBranchChanges: true,
			findingsExist:    true,
			taskRunning:      false,
			wantReview:       false,
			wantFix:          false,
		},
		{
			name:             "no branch changes",
			claudeFound:      true,
			hasBranchChanges: false,
			findingsExist:    true,
			taskRunning:      false,
			wantReview:       false,
			wantFix:          false,
		},
		{
			name:             "task running",
			claudeFound:      true,
			hasBranchChanges: true,
			findingsExist:    true,
			taskRunning:      true,
			wantReview:       false,
			wantFix:          false,
		},
		{
			name:             "no findings",
			claudeFound:      true,
			hasBranchChanges: true,
			findingsExist:    false,
			taskRunning:      false,
			wantReview:       true,
			wantFix:          false,
		},
		{
			name:             "claude not found and no changes",
			claudeFound:      false,
			hasBranchChanges: false,
			findingsExist:    false,
			taskRunning:      false,
			wantReview:       false,
			wantFix:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel("/tmp/test", "test")
			m.review.claudeFound = tt.claudeFound
			m.review.hasBranchChanges = tt.hasBranchChanges
			m.review.findingsExist = tt.findingsExist
			if tt.taskRunning {
				m.review.task = &aiTask{id: 1}
			}

			keys := m.reviewKeys()

			if keys.review != tt.wantReview {
				t.Errorf("review = %v, want %v", keys.review, tt.wantReview)
			}
			if keys.fix != tt.wantFix {
				t.Errorf("fix = %v, want %v", keys.fix, tt.wantFix)
			}
		})
	}
}

// TestSelectorHintPairs tests the hint text shown on the selector.
func TestSelectorHintPairs(t *testing.T) {
	tests := []struct {
		name       string
		keys       reviewKeys
		width      int
		wantC      bool
		wantF      bool
		wantQ      bool // quit should always be present
	}{
		{
			name:  "both keys on",
			keys:  reviewKeys{review: true, fix: true},
			width: 200,
			wantC: true,
			wantF: true,
			wantQ: true,
		},
		{
			name:  "both keys off",
			keys:  reviewKeys{review: false, fix: false},
			width: 200,
			wantC: false,
			wantF: false,
			wantQ: true,
		},
		{
			name:  "only review on",
			keys:  reviewKeys{review: true, fix: false},
			width: 200,
			wantC: true,
			wantF: false,
			wantQ: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := onSelector(t, 5, tt.width, 30)
			m.review.claudeFound = tt.keys.review
			m.review.hasBranchChanges = tt.keys.review
			m.review.findingsExist = tt.keys.fix

			view := m.View()

			// Check for the presence of hint text
			hasC := containsHint(view, reviewHintKey, reviewHintDesc)
			hasF := containsHint(view, fixHintKey, fixHintDesc)
			hasQ := containsHint(view, "Q", "quit")

			if hasC != tt.wantC {
				t.Errorf("hint %q %q present = %v, want %v", reviewHintKey, reviewHintDesc, hasC, tt.wantC)
			}
			if hasF != tt.wantF {
				t.Errorf("hint %q %q present = %v, want %v", fixHintKey, fixHintDesc, hasF, tt.wantF)
			}
			if hasQ != tt.wantQ {
				t.Errorf("hint Q quit present = %v, want %v", hasQ, tt.wantQ)
			}
		})
	}
}

// containsHint checks if the view contains both the key and description for a hint.
func containsHint(view, key, desc string) bool {
	stripped := stripAnsi(view)
	// The hints are rendered as "key desc key desc ..."
	// We need to check if both the key and description appear near each other
	keyIdx := -1
	descIdx := -1
	
	// Find key first
	for i := 0; i < len(stripped); i++ {
		if i+len(key) <= len(stripped) && stripped[i:i+len(key)] == key {
			keyIdx = i
			// Look for desc shortly after key
			start := keyIdx + len(key)
			for j := start; j < len(stripped) && j < start+50; j++ {
				if j+len(desc) <= len(stripped) && stripped[j:j+len(desc)] == desc {
					descIdx = j
					return true
				}
			}
		}
	}
	
	return keyIdx >= 0 && descIdx >= 0
}

// TestCommitListMsg_SetsReviewState tests that commitListMsg updates review state.
func TestCommitListMsg_SetsReviewState(t *testing.T) {
	m := NewModel("/tmp/test", "test")
	m.review.claudeFound = true

	// Success case - should set both fields
	msg := commitListMsg{
		commits:          sampleCommits(3),
		forkPointIdx:     -1,
		branchChanges:    true,
		findingsExist:    true,
	}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.review.hasBranchChanges {
		t.Error("hasBranchChanges should be true after successful commitListMsg")
	}
	if !m.review.findingsExist {
		t.Error("findingsExist should be true after successful commitListMsg")
	}

	// Error case - should leave fields unchanged
	m.review.hasBranchChanges = false
	m.review.findingsExist = false
	
	errMsg := commitListMsg{
		err: "some error",
	}
	updated, _ = m.Update(errMsg)
	m = updated.(Model)

	if m.review.hasBranchChanges {
		t.Error("hasBranchChanges should remain false after error commitListMsg")
	}
	if m.review.findingsExist {
		t.Error("findingsExist should remain false after error commitListMsg")
	}
}

// TestFetchCommitsCmd_PopulatesFields tests that fetchCommitsCmd sets the new fields.
func TestFetchCommitsCmd_PopulatesFields(t *testing.T) {
	// Test with a real repository
	repoPath := newBranchRepo(t)

	// Should have branch changes (feature is ahead of main)
	cmd := fetchCommitsCmd(repoPath)
	msg := cmd()
	commitMsg, ok := msg.(commitListMsg)
	if !ok {
		t.Fatalf("expected commitListMsg, got %T", msg)
	}

	if commitMsg.err != "" {
		t.Fatalf("unexpected error: %s", commitMsg.err)
	}

	if !commitMsg.branchChanges {
		t.Error("branchChanges should be true for feature branch ahead of main")
	}

	// Findings file doesn't exist yet
	if commitMsg.findingsExist {
		t.Error("findingsExist should be false when findings file doesn't exist")
	}

	// Create findings file
	findingsDir := filepath.Join(repoPath, review.Dir)
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	findingsPath := filepath.Join(findingsDir, review.FindingsFile)
	if err := os.WriteFile(findingsPath, []byte("test findings"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should now report findings exist
	cmd = fetchCommitsCmd(repoPath)
	msg = cmd()
	commitMsg, ok = msg.(commitListMsg)
	if !ok {
		t.Fatalf("expected commitListMsg, got %T", msg)
	}

	if !commitMsg.findingsExist {
		t.Error("findingsExist should be true after creating findings file")
	}

	// Switch to main branch - should have no branch changes
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("checkout", "main")

	cmd = fetchCommitsCmd(repoPath)
	msg = cmd()
	commitMsg, ok = msg.(commitListMsg)
	if !ok {
		t.Fatalf("expected commitListMsg, got %T", msg)
	}

	if commitMsg.branchChanges {
		t.Error("branchChanges should be false on main branch")
	}
}
