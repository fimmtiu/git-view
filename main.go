// Command git-view is a full-screen terminal viewer for git diffs.
package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/ui"
)

const usage = `git-view — a full-screen terminal viewer for git diffs

Usage:
  git-view [BASE]

Run it inside a git repository. It opens on a commit selector; pick a commit
or a range and press Tab or Enter to view the diff.

BASE is the ref that the current branch is compared against. If you do not
give it, git-view uses main, or master if main does not exist.

Environment:
  GIT_VIEW_EDITOR, VISUAL, EDITOR   editor launched by the E key
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "git-view:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	baseRef, help, err := parseArgs(args)
	if help || err != nil {
		fmt.Print(usage)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}

	if baseRef == "" {
		baseRef = git.DetectDefaultBranch(repoRoot)
	} else if err := git.VerifyCommit(repoRoot, baseRef); err != nil {
		return err
	}

	model := ui.NewModel(repoRoot, git.RepoLabel(repoRoot), baseRef)
	restoreCursorPos := saveCursorPos(os.Stdout)
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	restoreCursorPos()
	return err
}

// parseArgs returns the base ref, or "" if there is none, and whether the user
// asked for help.
func parseArgs(args []string) (baseRef string, help bool, err error) {
	switch {
	case len(args) == 0:
		return "", false, nil
	case args[0] == "-h" || args[0] == "--help":
		return "", true, nil
	case strings.HasPrefix(args[0], "-"):
		return "", false, fmt.Errorf("unknown option %q", args[0])
	case len(args) > 1:
		return "", false, fmt.Errorf("unexpected argument %q", args[1])
	}
	return args[0], false, nil
}

// DECSC and DECRC: save the cursor position, then put the cursor back.
const (
	saveCursorSeq    = "\x1b7"
	restoreCursorSeq = "\x1b8"
)

// saveCursorPos saves the cursor position and returns a function that restores
// it. Some terminals restore the screen contents but not the cursor when they
// leave the alternate screen, so the shell draws its next prompt at the top of
// the screen, over the restored history.
func saveCursorPos(out *os.File) func() {
	if info, err := out.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return func() {} // not a terminal, so there is no cursor to move
	}
	fmt.Fprint(out, saveCursorSeq)
	return func() { fmt.Fprint(out, restoreCursorSeq) }
}
