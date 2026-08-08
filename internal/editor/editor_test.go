package editor

import (
	"strings"
	"testing"
)

func TestResolve_PrefersGitViewEditor(t *testing.T) {
	t.Setenv("GIT_VIEW_EDITOR", "nvim")
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")

	ed, ok := Resolve()
	if !ok {
		t.Fatal("expected an editor")
	}
	if ed.Argv[0] != "nvim" {
		t.Errorf("Argv[0] = %q, want nvim", ed.Argv[0])
	}
}

func TestResolve_FallsBackThroughEnvVars(t *testing.T) {
	t.Setenv("GIT_VIEW_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")

	ed, ok := Resolve()
	if !ok {
		t.Fatal("expected an editor")
	}
	if ed.Argv[0] != "nano" {
		t.Errorf("Argv[0] = %q, want nano", ed.Argv[0])
	}
}

func TestResolve_TerminalEditorIsNotGUI(t *testing.T) {
	t.Setenv("GIT_VIEW_EDITOR", "vim")

	ed, _ := Resolve()
	if ed.GUI {
		t.Error("vim should not be treated as a GUI editor; it needs the tty")
	}
}

func TestResolve_KnownGUIEditor(t *testing.T) {
	t.Setenv("GIT_VIEW_EDITOR", "cursor")

	ed, _ := Resolve()
	if !ed.GUI {
		t.Error("cursor should be treated as a GUI editor")
	}
}

func TestFromSpec_KeepsUserFlagsAndAddsGoto(t *testing.T) {
	ed := fromSpec("code --wait")

	if got := strings.Join(ed.Argv, " "); got != "code --wait --goto" {
		t.Errorf("Argv = %q, want %q", got, "code --wait --goto")
	}
}

func TestFromSpec_DoesNotDuplicateGoto(t *testing.T) {
	ed := fromSpec("code --goto")

	if got := strings.Join(ed.Argv, " "); got != "code --goto" {
		t.Errorf("Argv = %q, want %q", got, "code --goto")
	}
}

func TestFromSpec_StripsDirectoryPrefix(t *testing.T) {
	ed := fromSpec("/opt/homebrew/bin/nvim")

	if !ed.GUI && ed.lineArg == nil {
		t.Error("a full path to a known editor should still match its profile")
	}
	if ed.Argv[0] != "/opt/homebrew/bin/nvim" {
		t.Errorf("Argv[0] = %q; the full path should be preserved", ed.Argv[0])
	}
}

func TestFromSpec_UnknownEditor(t *testing.T) {
	ed := fromSpec("myeditor --flag")

	if ed.GUI {
		t.Error("an unknown editor should not be assumed to be a GUI one")
	}
	if ed.lineArg != nil {
		t.Error("an unknown editor should not be sent line-navigation syntax")
	}
}

func TestCommand_LineSyntaxPerEditor(t *testing.T) {
	cases := []struct {
		spec string
		line int
		want string
	}{
		{"code", 42, "code --goto internal/ui/app.go:42"},
		{"cursor", 42, "cursor --goto internal/ui/app.go:42"},
		{"zed", 42, "zed internal/ui/app.go:42"},
		{"vim", 42, "vim +42 internal/ui/app.go"},
		{"nvim", 7, "nvim +7 internal/ui/app.go"},
		{"emacs", 7, "emacs +7 internal/ui/app.go"},
		{"hx", 7, "hx internal/ui/app.go:7"},
		{"myeditor", 42, "myeditor internal/ui/app.go"},
		// No line number: open the file at the top.
		{"code", 0, "code --goto internal/ui/app.go"},
		{"vim", 0, "vim internal/ui/app.go"},
	}

	for _, tc := range cases {
		cmd := fromSpec(tc.spec).Command("/tmp/repo", "internal/ui/app.go", tc.line)
		got := strings.Join(cmd.Args, " ")
		if got != tc.want {
			t.Errorf("%s at line %d → %q, want %q", tc.spec, tc.line, got, tc.want)
		}
	}
}

func TestCommand_RunsInRepoRoot(t *testing.T) {
	cmd := fromSpec("vim").Command("/tmp/repo", "one.txt", 0)

	if cmd.Dir != "/tmp/repo" {
		t.Errorf("Dir = %q, want /tmp/repo; relative diff paths resolve against it", cmd.Dir)
	}
}

func TestCommand_DoesNotMutateTheProfile(t *testing.T) {
	ed := fromSpec("code")
	first := strings.Join(ed.Command("/tmp/repo", "a.go", 1).Args, " ")
	second := strings.Join(ed.Command("/tmp/repo", "b.go", 2).Args, " ")

	if strings.Contains(second, "a.go") {
		t.Errorf("the second command leaked the first file: %q", second)
	}
	if first == second {
		t.Errorf("both commands are identical: %q", first)
	}
}
