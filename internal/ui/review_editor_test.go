package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fimmtiu/git-view/internal/editor"
)

func TestOpenInEditor_NoEditorFound(t *testing.T) {
	m := Model{
		repoRoot:    "/tmp/repo",
		editorFound: false,
	}

	cmd := m.openInEditor("a.md", "b.md")
	if cmd == nil {
		t.Fatal("expected a command")
	}

	msg := cmd()
	statusMsg, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("expected statusMsg, got %T", msg)
	}

	if !statusMsg.isErr {
		t.Error("expected isErr to be true")
	}

	want := "no editor found; set $EDITOR or $GIT_VIEW_EDITOR"
	if statusMsg.text != want {
		t.Errorf("text = %q, want %q", statusMsg.text, want)
	}
}

func TestOpenInEditor_GUIEditor(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake executable that ignores all arguments
	truePath := filepath.Join(tmpDir, "true")
	script := "#!/bin/sh\n# Ignore all arguments and exit successfully\nexit 0\n"
	if err := os.WriteFile(truePath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	m := Model{
		repoRoot:    tmpDir, // Use tmpDir as repoRoot so relative paths work
		editor:      editor.Editor{Argv: []string{truePath}, GUI: true},
		editorFound: true,
	}

	cmd := m.openInEditor("a.md", "b.md")
	if cmd == nil {
		t.Fatal("expected a command")
	}

	// waitForChildCmd runs cmd.Wait() in a goroutine
	done := make(chan interface{})
	go func() {
		msg := cmd()
		done <- msg
	}()

	select {
	case msg := <-done:
		_, ok := msg.(childExitedMsg)
		if !ok {
			t.Fatalf("expected childExitedMsg, got %T: %+v", msg, msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command to complete")
	}
}

func TestOpenInEditor_TerminalEditor(t *testing.T) {
	m := Model{
		repoRoot:    "/tmp/repo",
		editor:      editor.Editor{Argv: []string{"vim"}, GUI: false},
		editorFound: true,
	}

	cmd := m.openInEditor("a.md", "b.md")
	if cmd == nil {
		t.Error("expected a non-nil command")
	}

	// For terminal editors, we return tea.ExecProcess which doesn't start
	// a process immediately, so we can't easily test the msg type without
	// actually running the editor. Just verify we got a command back.
}

func TestOpenInEditor_GUIEditorStartFailure(t *testing.T) {
	m := Model{
		repoRoot:    "/tmp/repo",
		editor:      editor.Editor{Argv: []string{"/nonexistent/editor"}, GUI: true},
		editorFound: true,
	}

	cmd := m.openInEditor("a.md", "b.md")
	if cmd == nil {
		t.Fatal("expected a command")
	}

	msg := cmd()
	statusMsg, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("expected statusMsg, got %T", msg)
	}

	if !statusMsg.isErr {
		t.Error("expected isErr to be true")
	}

	if statusMsg.text == "" {
		t.Error("expected error message to be non-empty")
	}

	// Should contain "editor failed:"
	if len(statusMsg.text) < 15 || statusMsg.text[:15] != "editor failed: " {
		t.Errorf("expected error message to start with 'editor failed: ', got %q", statusMsg.text)
	}
}
