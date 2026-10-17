package editor

import (
	"testing"
)

func TestCommandFiles_VSCodeWithGoto(t *testing.T) {
	e := Editor{Argv: []string{"code", "--goto"}, GUI: true}
	cmd := e.CommandFiles("/tmp/repo", "a.md", "b.md")

	if cmd.Dir != "/tmp/repo" {
		t.Errorf("cmd.Dir = %q, want /tmp/repo", cmd.Dir)
	}

	// Arguments should be the files without --goto
	want := []string{"a.md", "b.md"}
	args := cmd.Args[1:] // Skip the command itself
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i, arg := range args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}

	// Original Argv should be unchanged
	if len(e.Argv) != 2 || e.Argv[0] != "code" || e.Argv[1] != "--goto" {
		t.Errorf("Argv was modified: %v", e.Argv)
	}
}

func TestCommandFiles_VimNoGoto(t *testing.T) {
	e := Editor{Argv: []string{"vim"}}
	cmd := e.CommandFiles("/tmp/repo", "a.md", "b.md")

	if cmd.Dir != "/tmp/repo" {
		t.Errorf("cmd.Dir = %q, want /tmp/repo", cmd.Dir)
	}

	// Arguments should be just the files
	want := []string{"a.md", "b.md"}
	args := cmd.Args[1:]
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i, arg := range args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

func TestCommandFiles_UserSpecWithFlag(t *testing.T) {
	e := Editor{Argv: []string{"nvim", "-p"}}
	cmd := e.CommandFiles("/tmp/repo", "a.md", "b.md")

	if cmd.Dir != "/tmp/repo" {
		t.Errorf("cmd.Dir = %q, want /tmp/repo", cmd.Dir)
	}

	// Arguments should preserve the -p flag
	want := []string{"-p", "a.md", "b.md"}
	args := cmd.Args[1:]
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i, arg := range args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

func TestCommandFiles_PreservesArgv(t *testing.T) {
	e := Editor{Argv: []string{"code", "--goto", "--new-window"}}
	original := make([]string, len(e.Argv))
	copy(original, e.Argv)

	_ = e.CommandFiles("/tmp/repo", "a.md")

	if len(e.Argv) != len(original) {
		t.Fatalf("Argv length changed: %d -> %d", len(original), len(e.Argv))
	}
	for i, arg := range e.Argv {
		if arg != original[i] {
			t.Errorf("Argv[%d] = %q, want %q", i, arg, original[i])
		}
	}
}
