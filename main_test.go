package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCursorPos_SkipsAFileThatIsNotATerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	saveCursorPos(f)()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("redirected output should stay clean, got %q", data)
	}
}

// The pair must be DECSC and DECRC. Other cursor-positioning sequences would
// scroll the screen or overwrite the restored history.
func TestCursorSequences_AreSaveAndRestore(t *testing.T) {
	if saveCursorSeq != "\x1b7" {
		t.Errorf("save = %q, want %q", saveCursorSeq, "\x1b7")
	}
	if restoreCursorSeq != "\x1b8" {
		t.Errorf("restore = %q, want %q", restoreCursorSeq, "\x1b8")
	}
}
