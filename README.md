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

- Uncommitted changes appear at the top as `???? Uncommitted changes`. This
  covers modified tracked files and untracked ones, which show as new files.
  Ignored files and staged-only changes are left out.
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

## Search

Press `/` in the diff viewer. The help line at the bottom becomes a search box.
Type a term and press Enter: every occurrence lights up, and the viewer jumps to
the first match at or below the top of the window — including one already in
view — and puts it in the middle of the screen, as near to the middle as the ends
of the diff allow. Escape closes the box and takes the highlights down.

The right of the prompt keeps count: "match 3 of 27" names the match nearest the
middle of the pane, so it follows you as you jump or scroll.

The term is matched exactly, so case counts. Only the text of the changed files
is searched: the `@@` hunk headers, the file names, and the line-number gutter
are part of the viewer rather than the files, and never match.

While you are typing, every printable character goes into the box — including
`n`, `p`, Space, `<` and `>`, which are commands elsewhere. That leaves `↑` `↓`,
`PgUp` `PgDn`, and `Home` `End` to move the window under the box. Enter hands
the letters back, so `n` and `p` then jump between matches. Press `/` again to
amend the term.

`n` and `p` are measured from the middle row of the pane, not the top, because
that is where a jump leaves its match — from the top, `n` would keep finding the
match you are already on. So the next match is always the one below what you are
looking at: search, jump, scroll up half a page, and `n` brings you back to the
match you just left.

| Key | While typing the term | Once it is committed |
|-----|-----------------------|----------------------|
| Any printable key | Insert it into the term | — |
| `←` `→` | Move within the term | — |
| `Ctrl-A` / `Ctrl-E` | Jump to the start / end of the term | — |
| `Backspace` / `Delete` | Delete a character | — |
| `Enter` | Search, and jump to the first match | Enter line-select mode |
| `n` / `p` | — | Jump to the next / previous match |
| `/` | — | Edit the term again |
| `Esc` | Close the search | Close the search |

## Keys

| Key | Commit selector | Diff viewer |
|-----|-----------------|-------------|
| `↑` `↓` / `j` `k` | Move the cursor | Scroll, or move the line cursor |
| `Shift`+`↑` `↓` | Extend the selection | — |
| `PgUp` `PgDn` / `b` `Space` | Page up and down | Page up and down |
| `<` `>` / `Home` `End` | Jump to the ends | Jump to the ends |
| `Tab` / `Enter` | View the selected diff | `Tab` returns to the selector |
| `Enter` | — | Enter line-select mode |
| `/` | — | Search the diff |
| `Esc` | — | Leave the search, line-select mode, or the viewer |
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
