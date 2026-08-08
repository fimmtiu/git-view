// Command git-view is a full-screen terminal viewer for git diffs.
//
// Usage:
//
//	git-view [<rev>...]
//
// Arguments are passed straight through to `git diff`, so the usual forms all
// work: no arguments shows the working tree, "HEAD~3" shows everything since
// that commit, "main..HEAD" shows a range, and "abc123^!" shows a single
// commit. Flags such as --cached are passed through too.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/diff"
	"github.com/fimmtiu/git-view/internal/git"
	"github.com/fimmtiu/git-view/internal/ui"
)

const usage = `git-view — a full-screen terminal viewer for git diffs

Usage:
  git-view [<rev>...]

Revision arguments are passed through to 'git diff':
  git-view                    changes in the working tree
  git-view --cached           staged changes
  git-view HEAD~3             everything since HEAD~3
  git-view main..HEAD         a commit range
  git-view abc123^!           a single commit

Environment:
  GIT_VIEW_EDITOR, VISUAL, EDITOR   editor launched by the E key
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "git-view:", err)
		os.Exit(1)
	}
}

// run resolves the repository, fetches and parses the diff, and hands control
// to the TUI. It returns an error rather than exiting so main owns the exit
// path.
func run(args []string) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(usage)
			return nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}

	raw, err := git.Diff(repoRoot, args)
	if err != nil {
		return err
	}
	files := diff.Parse(raw)
	if len(files) == 0 {
		fmt.Println("No changes to show.")
		return nil
	}

	model := ui.NewModel(files, repoRoot, git.DescribeRevs(repoRoot, args))
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
