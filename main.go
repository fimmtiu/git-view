// Command git-view is a full-screen terminal viewer for git diffs.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/ui"
)

const usage = `git-view — a full-screen terminal viewer for git diffs

Usage:
  git-view

Run it inside a git repository. It opens on a commit selector; pick a commit
or a range and press Tab or Enter to view the diff.

Environment:
  GIT_VIEW_EDITOR, VISUAL, EDITOR   editor launched by the E key
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "git-view:", err)
		os.Exit(1)
	}
}

// Returns errors rather than exiting so main owns the exit path.
func run(args []string) error {
	if len(args) > 0 {
		fmt.Print(usage)
		if args[0] != "-h" && args[0] != "--help" {
			return fmt.Errorf("unexpected argument %q", args[0])
		}
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}

	model := ui.NewModel(repoRoot, git.RepoLabel(repoRoot))
	restoreCursorPos := saveCursorPos(os.Stdout)
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	restoreCursorPos()
	return err
}

// DECSC and DECRC: save the cursor position, then put the cursor back.
const (
	saveCursorSeq    = "\x1b7"
	restoreCursorSeq = "\x1b8"
)

// saveCursorPos notes where the cursor is and returns a function that moves it
// back there.
//
// The alternate screen is meant to make this unnecessary. A terminal that
// leaves it restores the main screen, and is supposed to restore the cursor
// with it. Some terminals bring back the contents only, so the shell draws its
// next prompt at the top of the screen, on top of the history that just came
// back. Saving the position and setting it again makes the result the same
// everywhere: the history stays where it was, and the prompt comes out where
// the command was typed. A terminal that does restore the cursor gets the same
// coordinates twice, which changes nothing.
func saveCursorPos(out *os.File) func() {
	if info, err := out.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return func() {} // not a terminal, so there is no cursor to move
	}
	fmt.Fprint(out, saveCursorSeq)
	return func() { fmt.Fprint(out, restoreCursorSeq) }
}
