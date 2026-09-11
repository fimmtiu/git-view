package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/git"
)

// Enough to pick a range to review; the full history never is needed.
const maxCommits = 100

// A separator row is the non-selectable divider above the fork-point commit.
type commitRow struct {
	commit    git.CommitEntry
	separator bool
}

// selector is the commit-picker screen: the commit list on the left, and
// `git show --stat` for the cursor's commit on the right.
type selector struct {
	rows []commitRow

	// cursor is the moving end of the selection, anchor the fixed one; they are
	// equal unless a range is being extended.
	cursor int
	anchor int
	offset int // first visible row

	statOutput string
	statHash   string // the commit statOutput was fetched for

	width      int
	listHeight int
}

func (s *selector) setSize(width, listHeight int) {
	s.width = width
	s.listHeight = listHeight
	s.clampScroll()
}

func (s *selector) setRows(rows []commitRow) {
	s.rows = rows
	s.clampSelected()
	s.clampScroll()
}

// ── Dimensions ───────────────────────────────────────────────────────────────

func (s selector) leftPaneWidth() int {
	return max(s.width/3, 10)
}

func (s selector) rightPaneWidth() int {
	return max(s.width-s.leftPaneWidth(), 10)
}

// ── Selection ────────────────────────────────────────────────────────────────

// selectionRange orders the ends, whichever way the range was extended.
func (s selector) selectionRange() (int, int) {
	lo, hi := s.anchor, s.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// selectedCount ignores the separator when the range spans it.
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

func (s selector) currentCommit() *git.CommitEntry {
	if s.cursor < 0 || s.cursor >= len(s.rows) || s.rows[s.cursor].separator {
		return nil
	}
	return &s.rows[s.cursor].commit
}

// selectedRange returns the selection's oldest and newest commits, or ok=false
// when it holds none.
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

// move collapses any range back to a single commit.
func (s *selector) move(n, direction int) {
	if len(s.rows) == 0 {
		return
	}
	s.cursor = s.advanceCursor(s.cursor, n, direction)
	s.anchor = s.cursor
	s.clampScroll()
}

// extendRange leaves the anchor fixed so only the cursor end moves.
func (s *selector) extendRange(n, direction int) {
	if len(s.rows) == 0 {
		return
	}
	s.cursor = s.advanceCursor(s.cursor, n, direction)
	s.clampScroll()
}

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

// clampSelected re-validates the selection after the commit list changes.
func (s *selector) clampSelected() {
	if len(s.rows) == 0 {
		s.cursor, s.anchor = 0, 0
		return
	}
	s.cursor = min(max(s.cursor, 0), len(s.rows)-1)

	// Search downward first, then upward for a cursor left on a trailing separator.
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

func buildCommitRows(commits []git.CommitEntry, forkPointIdx int, hasUncommitted bool) []commitRow {
	var rows []commitRow

	if hasUncommitted {
		rows = append(rows, commitRow{
			commit: git.CommitEntry{Hash: git.UncommittedHash, Message: "Uncommitted changes"},
		})
	}

	for i, c := range commits {
		// A separator on the first row would divide the list from nothing.
		if i == forkPointIdx && len(rows) > 0 {
			rows = append(rows, commitRow{separator: true})
		}
		rows = append(rows, commitRow{commit: c})
	}

	return rows
}

// ── Rendering ────────────────────────────────────────────────────────────────

// Plain text; renderRow styles the hash separately on unselected rows.
func commitLabel(c git.CommitEntry) string {
	return shortHash(c.Hash) + " " + sanitize(c.Message)
}

// shortHash leaves the uncommitted sentinel intact.
func shortHash(h string) string {
	if h != git.UncommittedHash && len(h) > 4 {
		return h[:4]
	}
	return h
}

func (s selector) renderPanes() string {
	left := connectPaneCorners(s.renderList(), true, false)
	right := connectPaneCorners(s.renderStat(), false, true)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

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

// The cursor's row and the rest of the range are styled differently so the user
// can see which end moves.
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

func (s selector) renderStat() string {
	w := max(s.rightPaneWidth()-paneSideBorders, 1)
	h := s.listHeight

	content := theme.EmptyStateStyle.Render("(no preview)")
	if s.statOutput != "" {
		content = renderStatContent(sanitize(s.statOutput), w)
	}
	return theme.PaneStyle.Width(w).Height(h).Render(clipLines(content, h))
}

// renderStatContent bolds the commit subject and tints the '+'/'-' bars the way
// git colourises them in a terminal.
func renderStatContent(content string, width int) string {
	var out []string
	for i, line := range strings.Split(content, "\n") {
		// git indents its file rows; trim before wrapping to the pane width.
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

// A --stat file row ("foo.go | 5 ++---") or its summary ("1 file changed, …").
func isStatLine(line string) bool {
	if strings.Contains(line, "|") && (strings.ContainsAny(line, "+-") || strings.Contains(line, "Bin")) {
		return true
	}
	return strings.Contains(line, "file changed") || strings.Contains(line, "files changed")
}

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

// Ordered most to least important, since buildHintFit elides from the end.
func (s selector) hintPairs() []string {
	return []string{
		"↑↓", "navigate", "Tab/Enter", "view diff", "Shift+↑↓", "extend range",
		"PgUp/Dn", "page", "E", "edit repo", "Q", "quit",
	}
}
