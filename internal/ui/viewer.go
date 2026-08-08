package ui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

// ansiEscapeRe matches ANSI escape sequences (CSI and OSC).
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x1b\\|\x1b\][^\x07]*\x07`)

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// viewer holds the scrollable diff pane: the rendered text, the scroll offset,
// collapse state, and the line-select cursor. The app model owns the status bar
// and passes in only the content-pane dimensions, so the viewer has no copy of
// the full terminal size.
type viewer struct {
	text       string // pre-rendered diff content
	fileStarts []int  // line offset where each file begins
	fileNames  []string
	lineMeta   []diffLineMeta // per-line selectability and file ownership
	offset     int            // first visible line in the viewer pane

	// Collapse state: stored so we can re-render when the user toggles a file.
	files     []diff.File
	collapsed []bool

	// Content-pane dimensions (excluding border and chrome). Set by the app
	// model on creation and resize via setSize.
	paneWidth  int
	paneHeight int

	// Line select mode state.
	lineSelectMode bool // true when the user is selecting individual lines
	selectedLine   int  // index of the currently selected line in the rendered text
	frozenFileIdx  int  // file index frozen on exit from line select; -1 when not frozen
}

// newViewer creates a viewer from parsed diff files. paneWidth and paneHeight
// are the dimensions of the content area only.
func newViewer(files []diff.File, paneWidth, paneHeight int) *viewer {
	m := &viewer{
		paneWidth:     paneWidth,
		paneHeight:    paneHeight,
		files:         files,
		collapsed:     make([]bool, len(files)),
		frozenFileIdx: -1,
	}

	if len(files) == 0 {
		return m
	}

	m.rerender()
	m.fileNames = fileNamesFromDiff(files)
	return m
}

// setSize updates the content-pane dimensions and re-renders, since the diff
// text is wrapped and background-padded to the pane width.
func (m *viewer) setSize(paneWidth, paneHeight int) {
	widthChanged := paneWidth != m.paneWidth
	m.paneWidth = paneWidth
	m.paneHeight = paneHeight
	if widthChanged && len(m.files) > 0 {
		m.rerender()
	}
	m.clampScroll()
}

// totalLines returns the total number of lines in the rendered diff.
func (m *viewer) totalLines() int {
	if m.text == "" {
		return 0
	}
	return len(strings.Split(m.text, "\n"))
}

// ── Scroll ───────────────────────────────────────────────────────────────────

// scrollDown scrolls the viewer down by n lines.
func (m *viewer) scrollDown(n int) {
	m.offset += n
	m.clampScroll()
}

// scrollUp scrolls the viewer up by n lines.
func (m *viewer) scrollUp(n int) {
	m.offset -= n
	m.clampScroll()
}

// clampScroll ensures the viewer offset stays in bounds.
func (m *viewer) clampScroll() {
	total := m.totalLines()
	maxOffset := total - m.paneHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// ── File tracking ────────────────────────────────────────────────────────────

// currentFileIndex returns the 0-based index of the file whose diff is
// currently displayed. In line-select mode, this is the file owning the
// selected line. After exiting line-select mode, the file index is frozen
// until the user scrolls. Otherwise it is the file at the top of the pane.
func (m *viewer) currentFileIndex() int {
	if m.lineSelectMode && m.selectedLine >= 0 && m.selectedLine < len(m.lineMeta) {
		return m.lineMeta[m.selectedLine].fileIndex
	}
	if m.frozenFileIdx >= 0 {
		return m.frozenFileIdx
	}
	if len(m.fileStarts) == 0 {
		return 0
	}
	idx := 0
	for i, start := range m.fileStarts {
		if start <= m.offset {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// currentFileName returns the name of the file currently being displayed, or
// the empty string when there is no diff content.
func (m *viewer) currentFileName() string {
	idx := m.currentFileIndex()
	if idx < 0 || idx >= len(m.fileNames) {
		return ""
	}
	return m.fileNames[idx]
}

// ── Collapse/expand ─────────────────────────────────────────────────────────

// toggleCollapse toggles the collapsed state of the current file.
// It is a no-op for files with no hunks to display.
func (m *viewer) toggleCollapse() {
	idx := m.currentFileIndex()
	if idx < 0 || idx >= len(m.files) {
		return
	}
	if len(m.files[idx].Hunks) == 0 {
		return
	}
	m.collapsed[idx] = !m.collapsed[idx]
	wasLineSelect := m.lineSelectMode
	m.lineSelectMode = false
	m.rerender()
	// Scroll to the toggled file's header so the user sees the change.
	if idx < len(m.fileStarts) {
		m.offset = m.fileStarts[idx]
	}
	m.clampScroll()
	if wasLineSelect {
		m.enterLineSelect()
	}
}

// toggleCollapseAll collapses all files if any are expanded, or expands all
// files if all are already collapsed.
func (m *viewer) toggleCollapseAll() {
	if len(m.files) == 0 {
		return
	}
	// Determine target state: collapse all unless every collapsible file is
	// already collapsed.
	allCollapsed := true
	for i, f := range m.files {
		if len(f.Hunks) > 0 && !m.collapsed[i] {
			allCollapsed = false
			break
		}
	}
	target := !allCollapsed
	for i, f := range m.files {
		if len(f.Hunks) > 0 {
			m.collapsed[i] = target
		}
	}
	wasLineSelect := m.lineSelectMode
	m.lineSelectMode = false
	m.rerender()
	if wasLineSelect {
		m.enterLineSelect()
	}
}

// rerender re-renders the diff text from the stored files and collapse state.
func (m *viewer) rerender() {
	w := m.paneWidth
	if w < 1 {
		w = 1
	}
	rd := renderDiff(m.files, w, m.collapsed)
	m.text = rd.text
	m.fileStarts = rd.fileStarts
	m.lineMeta = rd.lineMeta
	m.clampScroll()
}

// ── Line select mode ────────────────────────────────────────────────────────

// isSelectable returns true if line at index i is a hunk content line.
func (m *viewer) isSelectable(i int) bool {
	return i >= 0 && i < len(m.lineMeta) && m.lineMeta[i].kind == diffLineHunkContent
}

// nearestSelectable searches outward from start in both directions and returns
// the nearest selectable line, or -1 if none exists within [lo, hi).
func (m *viewer) nearestSelectable(start, lo, hi int) int {
	if hi > len(m.lineMeta) {
		hi = len(m.lineMeta)
	}
	if lo < 0 {
		lo = 0
	}
	// Clamp start into [lo, hi). When the diff content is shorter than half the
	// pane, callers pass a start (the pane midpoint) past the end of the content;
	// the bounded outward search below would then never reach the content window.
	if start < lo {
		start = lo
	}
	if start >= hi {
		start = hi - 1
	}
	for d := 0; d < hi-lo; d++ {
		up := start - d
		down := start + d
		if up >= lo && up < hi && m.isSelectable(up) {
			return up
		}
		if down >= lo && down < hi && m.isSelectable(down) {
			return down
		}
	}
	return -1
}

// enterLineSelect enters line-select mode. The selection is placed on the
// selectable line nearest to the vertical midpoint of the visible pane.
// If no selectable line is visible, the mode is not entered.
func (m *viewer) enterLineSelect() {
	mid := m.offset + m.paneHeight/2
	sel := m.nearestSelectable(mid, m.offset, m.offset+m.paneHeight)
	if sel == -1 {
		return
	}
	m.lineSelectMode = true
	m.selectedLine = sel
	m.frozenFileIdx = -1
}

// exitLineSelect leaves line-select mode and freezes the current file index
// so it persists until the user scrolls.
func (m *viewer) exitLineSelect() {
	if !m.lineSelectMode {
		return
	}
	m.frozenFileIdx = m.currentFileIndex()
	m.lineSelectMode = false
}

// nextSelectableLine returns the next selectable line from the current
// selectedLine in the given direction (+1 for down, -1 for up), or -1
// if there is no selectable line in that direction.
func (m *viewer) nextSelectableLine(direction int) int {
	i := m.selectedLine + direction
	for i >= 0 && i < len(m.lineMeta) {
		if m.isSelectable(i) {
			return i
		}
		i += direction
	}
	return -1
}

// moveSelection moves the selected line by n steps in the given direction,
// skipping non-selectable lines and scrolling to keep the selection visible.
func (m *viewer) moveSelection(n, direction int) {
	for i := 0; i < n; i++ {
		next := m.nextSelectableLine(direction)
		if next == -1 {
			break
		}
		m.selectedLine = next
	}
	m.scrollToSelection()
}

// scrollToSelection adjusts the scroll offset so the selected line is visible.
func (m *viewer) scrollToSelection() {
	if m.selectedLine < m.offset {
		m.offset = m.selectedLine
	}
	if m.selectedLine >= m.offset+m.paneHeight {
		m.offset = m.selectedLine - m.paneHeight + 1
	}
	m.clampScroll()
}

// ── Selected location ────────────────────────────────────────────────────────

// selectedLocation returns the file name and line number of the currently
// selected line. The line number is zero when not in line-select mode, in
// which case the file name is still the file at the top of the pane — that is
// what "open the current file" means outside line-select mode.
func (m *viewer) selectedLocation() (fileName string, lineNum int) {
	if !m.lineSelectMode || m.selectedLine < 0 || m.selectedLine >= len(m.lineMeta) {
		return m.currentFileName(), 0
	}
	meta := m.lineMeta[m.selectedLine]
	if meta.fileIndex >= 0 && meta.fileIndex < len(m.fileNames) {
		fileName = m.fileNames[meta.fileIndex]
	}
	return fileName, meta.lineNum
}

// ── Update ───────────────────────────────────────────────────────────────────

// handleKey processes scroll, collapse, and line-select keys. Keys that belong
// to the app (quit, edit, GitHub) are handled by the app model before this is
// reached. It returns nil; no viewer key produces a command.
func (m *viewer) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.lineSelectMode {
		return m.handleLineSelectKey(msg)
	}

	switch msg.String() {
	case "up", "k":
		m.clearFrozenFileIdx()
		m.scrollUp(1)
	case "down", "j":
		m.clearFrozenFileIdx()
		m.scrollDown(1)
	case "pgup", "b":
		m.clearFrozenFileIdx()
		m.scrollUp(m.paneHeight)
	case "pgdown", " ":
		m.clearFrozenFileIdx()
		m.scrollDown(m.paneHeight)
	case "<", "home":
		m.clearFrozenFileIdx()
		m.offset = 0
	case ">", "end":
		m.clearFrozenFileIdx()
		m.offset = m.totalLines()
		m.clampScroll()
	case "enter":
		m.enterLineSelect()
	case "c":
		m.toggleCollapse()
	case "C":
		m.toggleCollapseAll()
	}
	return nil
}

// handleLineSelectKey processes key events while in line-select mode.
func (m *viewer) handleLineSelectKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		m.moveSelection(1, -1)
	case "down", "j":
		m.moveSelection(1, 1)
	case "pgup", "b":
		m.moveSelection(m.paneHeight, -1)
	case "pgdown", " ":
		m.moveSelection(m.paneHeight, 1)
	case "<", "home":
		m.moveSelection(len(m.lineMeta), -1)
	case ">", "end":
		m.moveSelection(len(m.lineMeta), 1)
	case "esc":
		m.exitLineSelect()
	case "c":
		m.toggleCollapse()
	case "C":
		m.toggleCollapseAll()
	}
	return nil
}

// clearFrozenFileIdx removes the frozen file index so that scrolling
// resumes normal file tracking.
func (m *viewer) clearFrozenFileIdx() {
	m.frozenFileIdx = -1
}

// ── Rendering ────────────────────────────────────────────────────────────────

// renderPane renders the bordered diff content pane with its scrollbar.
func (m *viewer) renderPane() string {
	paneW := m.paneWidth
	if paneW < 1 {
		paneW = 1
	}

	var content string
	if m.text == "" {
		content = lipgloss.Place(paneW, m.paneHeight, lipgloss.Center, lipgloss.Center,
			theme.EmptyStateStyle.Render("No diff content"))
	} else {
		lines := strings.Split(m.text, "\n")
		end := m.offset + m.paneHeight
		if end > len(lines) {
			end = len(lines)
		}
		start := m.offset
		if start > len(lines) {
			start = len(lines)
		}
		visible := lines[start:end]

		// Highlight the selected line in line-select mode.
		if m.lineSelectMode && m.selectedLine >= start && m.selectedLine < end {
			idx := m.selectedLine - start
			visible[idx] = theme.LineSelectStyle.Width(paneW).Render(
				truncateLine(stripAnsi(visible[idx]), paneW))
		}

		content = strings.Join(visible, "\n")
	}

	rendered := theme.PaneStyle.Width(paneW).Height(m.paneHeight).Render(clipLines(content, m.paneHeight))
	return injectScrollbar(rendered, "│", "█", m.offset, m.totalLines(), m.paneHeight)
}

// hintPairs returns alternating key/description pairs for the help text at the
// bottom of the screen, ordered most to least important: a narrow terminal
// elides from the end (see buildHintFit), except for the trailing quit pair.
func (m *viewer) hintPairs() []string {
	if m.lineSelectMode {
		return []string{
			"↑↓", "move", "E", "edit line", "g", "github",
			"c/C", "collapse", "PgUp/Dn", "page", "</>", "ends",
			"Esc", "exit select", "Q", "quit",
		}
	}
	return []string{
		"↑↓", "scroll", "Enter", "select lines", "E", "edit file",
		"g", "github", "c/C", "collapse", "PgUp/Dn", "page",
		"</>", "ends", "Q", "quit",
	}
}
