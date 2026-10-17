package ui

import (
	"testing"
)

func TestSelector_HintPairs(t *testing.T) {
	s := selector{}
	
	got := s.hintPairs(reviewKeys{})
	
	// Check that base hints are present (no review/fix hints).
	expectedKeys := []string{"↑↓", "Tab/Enter", "Shift+↑↓", "PgUp/Dn", "E", "Q"}
	
	// got is a flat list like ["↑↓", "navigate", "Tab/Enter", "view diff", ...]
	gotKeys := make(map[string]bool)
	for i := 0; i < len(got); i += 2 {
		gotKeys[got[i]] = true
	}
	
	for _, want := range expectedKeys {
		if !gotKeys[want] {
			t.Errorf("hintPairs() missing key %q", want)
		}
	}
	
	// Verify review/fix keys are NOT present.
	if gotKeys["C"] {
		t.Errorf("hintPairs() should not include review key 'C'")
	}
	if gotKeys["F"] {
		t.Errorf("hintPairs() should not include fix key 'F'")
	}
}
