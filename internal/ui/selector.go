package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/git"
)

// maxCommits is the number of commits offered in the selector. The full history
// is never needed to pick a range to review.
const maxCommits = 100

// commitRow is one row in the commit list. When separator is true the row is a
// non-selectable divider drawn above the fork-point commit, marking where the
// current branch diverged from the default branch.
type commitRow struct {
	commit    git.CommitEntry
	separator bool
}

// selector is the commit-picker screen: a scrollable list of commits on the
// left, and `git show --stat` for the cursor's commit on the right. The
// selection can be a single commit or a range extended with Shift+arrows.
type selector struct {
	rows []commitRow

	// cursor is the actively-moving end of the selection; anchor is the fixed
	// end. They are equal for a single-commit selection.
	cursor int
	anchor int
	offset int // first visible row in the commit list

	// Cached `git show --stat` output for the right-hand pane, and the hash it
	// was fetched for.
	statOutput string
	statHash   string

	// Pane dimensions, set by the app model on resize.
	width      int
	listHeight int
}

// setSize records the dimensions the selector renders into.
func (s *selector) setSize(width, listHeight int) {
	s.width = width
	s.listHeight = listHeight
	s.clampScroll()
}

// setRows installs a freshly fetched commit list and re-validates the selection.
func (s *selector) setRows(rows []commitRow) {
	s.rows = rows
	s.clampSelected()
	s.clampScroll()
}

// ── Dimensions ───────────────────────────────────────────────────────────────

// leftPaneWidth returns the width of the commit list pane, about a third of the
// screen.
func (s selector) leftPaneWidth() int {
	return max(s.width/3, 10)
}

// rightPaneWidth returns the width of the stat preview pane.
func (s selector) rightPaneWidth() int {
	return max(s.width-s.leftPaneWidth(), 10)
}

// ── Selection ────────────────────────────────────────────────────────────────

// selectionRange returns the (lo, hi) row indices of the current selection, with
// lo <= hi regardless of which direction the range was extended in.
func (s selector) selectionRange() (int, int) {
	lo, hi := s.anchor, s.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// selectedCount returns the number of commits in the selection, ignoring the
// fork-point separator if the range spans it.
func (s selector) selectedCount() int {
	lo, hi := s.selectionRange()
	count := 0
	for i := lo; i <= hi; i++ {
		if i < len(s.rows) && !s.rows[i].separator {
			count++
		}
	}
	return count
}

// currentCommit returns the commit under the cursor, or nil when the cursor is
// not on a commit.
func (s selector) currentCommit() *git.CommitEntry {
	if s.cursor < 0 || s.cursor >= len(s.rows) || s.rows[s.cursor].separator {
		return nil
	}
	return &s.rows[s.cursor].commit
}

// selectedRange returns the oldest and newest commits in the selection. The
// diff runs from the parent of start through end. Returns false when the
// selection holds no commits.
func (s selector) selectedRange() (start, end git.CommitEntry, ok bool) {
	lo, hi := s.selectionRange()
	if lo < 0 || hi >= len(s.rows) {
		return start, end, false
	}
	// Rows run newest first, so the newest commit is at the low index.
	for i := lo; i <= hi; i++ {
		if !s.rows[i].separator {
			end = s.rows[i].commit
			break
		}
	}
	for i := hi; i >= lo; i-- {
		if !s.rows[i].separator {
			start = s.rows[i].commit
			break
		}
	}
	if start.Hash == "" || end.Hash == "" {
		return start, end, false
	}
	return start, end, true
}

// ── Navigation ───────────────────────────────────────────────────────────────

// nextSelectableRow returns the next non-separator row from `from` in the given
// direction (+1 down, -1 up), or -1 when there is none.
func (s *selector) nextSelectableRow(from, direction int) int {
	next := from + direction
	for next >= 0 && next < len(s.rows) && s.rows[next].separator {
		next += direction
	}
	if next < 0 || next >= len(s.rows) {
		return -1
	}
	return next
}

// advanceCursor moves a row index n steps in the given direction, skipping
// separators and stopping at the ends of the list.
func (s *selector) advanceCursor(cursor, n, direction int) int {
	for range n {
		next := s.nextSelectableRow(cursor, direction)
		if next == -1 {
			break
		}
		cursor = next
	}
	return cursor
}

// move moves the whole selection n steps, collapsing any range back to a single
// commit.
func (s *selector) move(n, direction int) {
	if len(s.rows) == 0 {
		return
	}
	s.cursor = s.advanceCursor(s.cursor, n, direction)
	s.anchor = s.cursor
	s.clampScroll()
}

// extendRange moves the cursor n steps while leaving the anchor fixed, growing
// or shrinking the selected range.
func (s *selector) extendRange(n, direction int) {
	if len(s.rows) == 0 {
		return
	}
	s.cursor = s.advanceCursor(s.cursor, n, direction)
	s.clampScroll()
}

// clampScroll scrolls the list so the cursor stays visible.
func (s *selector) clampScroll() {
	h := s.listHeight
	if h <= 0 || len(s.rows) == 0 {
		s.offset = 0
		return
	}
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+h {
		s.offset = s.cursor - h + 1
	}
	maxOffset := max(len(s.rows)-h, 0)
	s.offset = min(max(s.offset, 0), maxOffset)
}

// clampSelected brings the cursor and anchor back into range after a refresh,
// moving off a separator if it landed on one.
func (s *selector) clampSelected() {
	if len(s.rows) == 0 {
		s.cursor, s.anchor = 0, 0
		return
	}
	s.cursor = min(max(s.cursor, 0), len(s.rows)-1)

	// Prefer the next selectable row below; fall back to searching upward when
	// the cursor landed on a trailing separator.
	for s.cursor < len(s.rows) && s.rows[s.cursor].separator {
		s.cursor++
	}
	if s.cursor >= len(s.rows) {
		s.cursor = len(s.rows) - 1
		for s.cursor >= 0 && s.rows[s.cursor].separator {
			s.cursor--
		}
	}
	s.cursor = max(s.cursor, 0)
	s.anchor = s.cursor
}

// ── Row building ─────────────────────────────────────────────────────────────

// buildCommitRows turns a commit list into rows: an "uncommitted changes"
// pseudo-commit on top when the working tree is dirty, and a separator above the
// fork-point commit. The separator is skipped when it would be the very first
// row, where it would divide the list from nothing.
func buildCommitRows(commits []git.CommitEntry, forkPointIdx int, hasUncommitted bool) []commitRow {
	var rows []commitRow

	if hasUncommitted {
		rows = append(rows, commitRow{
			commit: git.CommitEntry{Hash: git.UncommittedHash, Message: "Uncommitted changes"},
		})
	}

	for i, c := range commits {
		if i == forkPointIdx && len(rows) > 0 {
			rows = append(rows, commitRow{separator: true})
		}
		rows = append(rows, commitRow{commit: c})
	}

	return rows
}

// ── Rendering ────────────────────────────────────────────────────────────────

// commitLabel returns "<short-hash> <subject>" as plain text. The hash is
// styled separately by renderRow for unselected rows.
func commitLabel(c git.CommitEntry) string {
	return shortHash(c.Hash) + " " + c.Message
}

// shortHash abbreviates a commit hash to four characters, leaving the
// uncommitted sentinel ("????") as-is.
func shortHash(h string) string {
	if h != git.UncommittedHash && len(h) > 4 {
		return h[:4]
	}
	return h
}

// renderPanes renders the commit list and stat preview side by side.
func (s selector) renderPanes() string {
	left := connectPaneCorners(s.renderList(), true, false)
	right := connectPaneCorners(s.renderStat(), false, true)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderList renders the scrollable commit list with its scrollbar.
func (s selector) renderList() string {
	w := max(s.leftPaneWidth()-paneSideBorders, 1)
	h := s.listHeight

	if len(s.rows) == 0 {
		return theme.PaneStyle.Width(w).Height(h).
			Render(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center,
				theme.EmptyStateStyle.Render("No commits")))
	}

	lo, hi := s.selectionRange()
	end := min(s.offset+h, len(s.rows))

	var sb strings.Builder
	for i := s.offset; i < end; i++ {
		sb.WriteString(s.renderRow(i, w, lo, hi))
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	rendered := theme.PaneStyle.Width(w).Height(h).Render(clipLines(sb.String(), h))
	return injectScrollbar(rendered, "│", "█", s.offset, len(s.rows), h)
}

// renderRow renders one commit row: the cursor's row is highlighted, other rows
// inside the selected range get the range background, and the rest show a
// styled hash followed by plain text.
func (s selector) renderRow(i, w, lo, hi int) string {
	row := s.rows[i]
	if row.separator {
		return theme.SeparatorStyle.Render(strings.Repeat("─", w))
	}

	label := truncateLine(commitLabel(row.commit), w)
	switch {
	case i == s.cursor:
		return theme.SelectedStyle.Width(w).Render(label)
	case i >= lo && i <= hi:
		return theme.RangeStyle.Width(w).Render(label)
	default:
		hash := shortHash(row.commit.Hash)
		return theme.CommitHashStyle.Render(hash) + label[len(hash):]
	}
}

// renderStat renders the `git show --stat` preview pane.
func (s selector) renderStat() string {
	w := max(s.rightPaneWidth()-paneSideBorders, 1)
	h := s.listHeight

	content := theme.EmptyStateStyle.Render("(no preview)")
	if s.statOutput != "" {
		content = renderStatContent(s.statOutput, w)
	}
	return theme.PaneStyle.Width(w).Height(h).Render(clipLines(content, h))
}

// renderStatContent styles `git show --stat` output for the preview pane: the
// commit subject is bolded, and the '+'/'-' bars are tinted green and red the
// way git colourises them in a terminal.
//
// git indents the file rows, so leading indentation is trimmed before wrapping
// to the pane width.
func renderStatContent(content string, width int) string {
	var out []string
	for i, line := range strings.Split(content, "\n") {
		pieces := wrapLine(strings.TrimLeft(line, " "), width)
		switch {
		case i == 0:
			for j, p := range pieces {
				pieces[j] = theme.LabelBoldStyle.Render(p)
			}
		case isStatLine(line):
			for j, p := range pieces {
				pieces[j] = colourizeStatBars(p)
			}
		}
		out = append(out, pieces...)
	}
	return strings.Join(out, "\n")
}

// isStatLine reports whether a line belongs to the --stat section: either a file
// row ("foo.go | 5 ++---") or the summary ("1 file changed, 3 insertions(+)").
func isStatLine(line string) bool {
	if strings.Contains(line, "|") && (strings.ContainsAny(line, "+-") || strings.Contains(line, "Bin")) {
		return true
	}
	return strings.Contains(line, "file changed") || strings.Contains(line, "files changed")
}

// colourizeStatBars tints runs of '+' green and runs of '-' red.
func colourizeStatBars(line string) string {
	var b strings.Builder
	runes := []rune(line)
	for i := 0; i < len(runes); {
		c := runes[i]
		if c != '+' && c != '-' {
			b.WriteRune(c)
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] == c {
			j++
		}
		run := string(runes[i:j])
		if c == '+' {
			b.WriteString(theme.StatAddStyle.Render(run))
		} else {
			b.WriteString(theme.StatRemoveStyle.Render(run))
		}
		i = j
	}
	return b.String()
}

// wrapLine breaks a line into chunks of at most width visible columns.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	var out []string
	for len(runes) > 0 {
		n := min(width, len(runes))
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

// hintPairs returns the help text for the selector screen, ordered most to least
// important; see buildHintFit for how a narrow terminal elides it.
func (s selector) hintPairs() []string {
	return []string{
		"↑↓", "navigate", "Tab/Enter", "view diff", "Shift+↑↓", "extend range",
		"PgUp/Dn", "page", "E", "edit repo", "Q", "quit",
	}
}
