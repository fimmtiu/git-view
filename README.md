# git-view

A full-screen terminal viewer for git diffs, extracted from the Diffs pane of
[code-factory](https://github.com/fimmtiu/code-factory).

It cleans up raw `git diff` output for reading on a terminal: file headers
collapse to a bold filename, `@@` hunk headers lose their line-number ranges,
`+`/`-` prefixes become full-width background colours, and added lines carry
line numbers in a left-hand gutter.

## Install

```
go build -o git-view .
```

## Usage

```
git-view
```

Run it inside a git repository. It takes no arguments.

## Commit selector

The app opens on the commit selector. The left pane lists the most recent 100
commits on the current branch, newest first, and the right pane shows
`git show --stat` for whichever one the cursor is on.

- Uncommitted changes to tracked files appear at the top as `???? Uncommitted
  changes`. Untracked files are not included, since `git diff` would not show
  them anyway.
- A horizontal rule marks where the current branch diverged from `main` or
  `master`, separating your commits from the ones you branched off.
- Merge commits are omitted.

Move the cursor to pick a single commit, or hold Shift while moving to extend
the selection into a range. Press Tab or Enter to read the diff for that
selection; the diff runs from the parent of the oldest selected commit through
the newest, so the whole range's changes are included.

## Diff viewer

Tab or Escape returns to the commit selector, with your selection intact.

Press Enter to enter line-select mode, which puts a cursor on an individual
line of the diff so that `E` and `g` can target it. Escape leaves line-select
mode; Tab still returns to the selector.

## Keys

| Key | Commit selector | Diff viewer |
|-----|-----------------|-------------|
| `↑` `↓` / `j` `k` | Move the cursor | Scroll, or move the line cursor |
| `Shift`+`↑` `↓` | Extend the selection | — |
| `PgUp` `PgDn` / `b` `Space` | Page up and down | Page up and down |
| `<` `>` / `Home` `End` | Jump to the ends | Jump to the ends |
| `Tab` / `Enter` | View the selected diff | `Tab` returns to the selector |
| `Enter` | — | Enter line-select mode |
| `Esc` | — | Leave line-select mode, or the viewer |
| `c` / `C` | — | Collapse or expand one file / every file |
| `E` | Open the repository in your editor | Open the current file, at the selected line |
| `g` | — | Open the current file on GitHub |
| `q` / `Ctrl-C` | Quit | Quit |

Outside line-select mode, "the current file" is the one at the top of the pane,
and `E` and `g` open it without a line number.

## Editor

`E` uses the first of `$GIT_VIEW_EDITOR`, `$VISUAL`, or `$EDITOR` that is set,
falling back to the first of `cursor`, `code`, or `zed` found on `PATH`.
Line-number syntax is handled per editor (`--goto file:line` for VS Code-family
editors, `+line file` for the vi family). GUI editors are launched in the
background; terminal editors take over the screen and return to the viewer when
they exit.

## GitHub

`g` opens `https://github.com/<owner>/<repo>/blob/<ref>/<path>` via the `open`
command, appending `#L<line>` when a line is selected. The ref is the current
branch, or the HEAD commit hash when the working tree is on a detached HEAD —
either way, the commit must have been pushed for the link to resolve.
