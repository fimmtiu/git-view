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
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
