package ui

import (
	"testing"
)

// CR #28: Split concerns in commitListMsg and reviewStateMsg
//
// This file contains tests that verify the refactoring described in CR #28:
// 1. commitListMsg should only carry commit-related data
// 2. reviewStateMsg should carry review-specific state updates
// 3. Review state is initialized at startup instead of piggybacking on commit loading

// TestCR28_commitListMsg_OnlyCarriesCommitData verifies that commitListMsg
// only contains fields related to commit list loading.
func TestCR28_commitListMsg_OnlyCarriesCommitData(t *testing.T) {
	// After refactoring, commitListMsg should only have commit fields
	msg := commitListMsg{
		commits:        nil,
		forkPointIdx:   0,
		hasUncommitted: false,
		err:            "",
	}
	_ = msg
}

// TestCR28_reviewStateMsg_CarriesReviewState verifies that a separate message
// type exists for carrying review state updates.
func TestCR28_reviewStateMsg_CarriesReviewState(t *testing.T) {
	// This should compile after refactoring
	msg := reviewStateMsg{
		hasBranchChanges: true,
		findingsExist:    false,
	}

	if !msg.hasBranchChanges {
		t.Error("hasBranchChanges should be accessible")
	}
	if msg.findingsExist {
		t.Error("findingsExist should be accessible")
	}
}

// TestCR28_Model_HandlesReviewStateMsg verifies that the Model's Update method
// handles reviewStateMsg to update review state.
func TestCR28_Model_HandlesReviewStateMsg(t *testing.T) {
	m := NewModel("/tmp/repo", "test")
	m.review.hasBranchChanges = false
	m.review.findingsExist = false

	// Send a reviewStateMsg
	updated, _ := m.Update(reviewStateMsg{
		hasBranchChanges: true,
		findingsExist:    true,
	})

	result := updated.(Model)
	if !result.review.hasBranchChanges {
		t.Error("hasBranchChanges should be updated from reviewStateMsg")
	}
	if !result.review.findingsExist {
		t.Error("findingsExist should be updated from reviewStateMsg")
	}
}

// TestCR28_Model_Update_DoesNotSetReviewStateFromCommitList verifies that
// after refactoring, receiving commitListMsg does not update review state.
func TestCR28_Model_Update_DoesNotSetReviewStateFromCommitList(t *testing.T) {
	m := NewModel("/tmp/repo", "test")
	m.review.hasBranchChanges = false
	m.review.findingsExist = false

	// After refactoring, commitListMsg should not change review state
	updated, _ := m.Update(commitListMsg{
		commits:        nil,
		forkPointIdx:   -1,
		hasUncommitted: false,
		err:            "",
	})

	result := updated.(Model)
	if result.review.hasBranchChanges {
		t.Error("commitListMsg should not set hasBranchChanges after refactoring")
	}
	if result.review.findingsExist {
		t.Error("commitListMsg should not set findingsExist after refactoring")
	}
}

// TestCR28_ReviewStateInitialization verifies that review state is initialized
// at startup rather than waiting for the first commit list load.
func TestCR28_ReviewStateInitialization(t *testing.T) {
	// After refactoring, Init() should return a command that fetches review state
	m := NewModel("/tmp/repo", "test")
	cmd := m.Init()

	// Verify that Init returns a batch command that includes fetching review state
	// This is implementation-dependent, but the key point is that review state
	// is loaded independently from commit list loading.
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}
