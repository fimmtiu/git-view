# git-view

A full-screen terminal viewer for git diffs, extracted from the diff viewer pane
of [code-factory](https://github.com/fimmtiu/code-factory).

It cleans up raw `git diff` output for reading on a terminal: file headers
collapse to a bold filename, `@@` hunk headers lose their line-number ranges,
`+`/`-` prefixes become full-width background colours, and added lines carry
line numbers in a left-hand gutter.

## Install

```
go build -o git-view .
```

## Usage

Revision arguments are passed straight through to `git diff`, so the usual forms
all work:

```
git-view                 # changes in the working tree
git-view --cached        # staged changes
git-view HEAD~3          # everything since HEAD~3
git-view main..HEAD      # a commit range
git-view abc123^!        # a single commit
```

## Keys

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | Scroll, or move the cursor in line-select mode |
| `PgUp` `PgDn` / `b` `Space` | Page up and down |
| `<` `>` / `Home` `End` | Jump to the top or bottom |
| `Enter` | Enter line-select mode |
| `Esc` | Leave line-select mode |
| `c` | Collapse or expand the current file |
| `C` | Collapse or expand every file |
| `E` | Open the current file in your editor, at the selected line |
| `g` | Open the current file on GitHub, anchored at the selected line |
| `q` / `Ctrl-C` | Quit |

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
