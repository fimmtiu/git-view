package ui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

// CSI and OSC sequences.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x1b\\|\x1b\][^\x07]*\x07`)

func stripAnsi(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// viewer is the scrollable diff pane. The app model owns the status bar and
// passes in content-pane dimensions only, so the viewer never sees the full
// terminal size.
type viewer struct {
	text       string
	fileStarts []int
	fileNames  []string
	lineMeta   []diffLineMeta
	offset     int // first visible line

	// Kept so a collapse toggle can re-render.
	files     []diff.File
	collapsed []bool

	paneWidth  int
	paneHeight int

	lineSelectMode bool
	selectedLine   int
	frozenFileIdx  int // -1 when not frozen; see exitLineSelect
}

// paneWidth and paneHeight cover the content area only.
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

// setSize re-renders on a width change, since the text is wrapped and padded to
// the pane width.
func (m *viewer) setSize(paneWidth, paneHeight int) {
	widthChanged := paneWidth != m.paneWidth
	m.paneWidth = paneWidth
	m.paneHeight = paneHeight
	if widthChanged && len(m.files) > 0 {
		m.rerender()
	}
	m.clampScroll()
}

func (m *viewer) totalLines() int {
	if m.text == "" {
		return 0
	}
	return len(strings.Split(m.text, "\n"))
}

// ── Scroll ───────────────────────────────────────────────────────────────────

func (m *viewer) scrollDown(n int) {
	m.offset += n
	m.clampScroll()
}

func (m *viewer) scrollUp(n int) {
	m.offset -= n
	m.clampScroll()
}

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

// currentFileIndex is the file owning the selected line in line-select mode, the
// frozen file just after leaving it, and otherwise the file at the top of the
// pane.
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

func (m *viewer) currentFileName() string {
	idx := m.currentFileIndex()
	if idx < 0 || idx >= len(m.fileNames) {
		return ""
	}
	return m.fileNames[idx]
}

// ── Collapse/expand ─────────────────────────────────────────────────────────

// toggleCollapse is a no-op for files with no hunks to hide.
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
	// Scroll to the file's header so the user sees what changed.
	if idx < len(m.fileStarts) {
		m.offset = m.fileStarts[idx]
	}
	m.clampScroll()
	if wasLineSelect {
		m.enterLineSelect()
	}
}

// toggleCollapseAll collapses everything unless it all already is, in which case
// it expands.
func (m *viewer) toggleCollapseAll() {
	if len(m.files) == 0 {
		return
	}
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

func (m *viewer) isSelectable(i int) bool {
	return i >= 0 && i < len(m.lineMeta) && m.lineMeta[i].kind == diffLineHunkContent
}

// nearestSelectable searches outward from start, returning -1 if nothing within
// [lo, hi) is selectable.
func (m *viewer) nearestSelectable(start, lo, hi int) int {
	if hi > len(m.lineMeta) {
		hi = len(m.lineMeta)
	}
	if lo < 0 {
		lo = 0
	}
	// Content shorter than half the pane leaves start (the pane midpoint) past
	// the end, where the bounded search below would never reach the content.
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

// enterLineSelect starts on the selectable line nearest the pane's midpoint, and
// does nothing at all if none is visible.
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

// exitLineSelect freezes the current file index so the status bar keeps naming
// the file the user was on until they scroll away.
func (m *viewer) exitLineSelect() {
	if !m.lineSelectMode {
		return
	}
	m.frozenFileIdx = m.currentFileIndex()
	m.lineSelectMode = false
}

// direction is +1 for down, -1 for up; -1 is returned when nothing is left.
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

// selectedLocation falls back to the file at the top of the pane with no line
// number, which is what "the current file" means outside line-select mode.
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

// handleKey sees only what the app model did not claim first.
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

func (m *viewer) clearFrozenFileIdx() {
	m.frozenFileIdx = -1
}

// ── Rendering ────────────────────────────────────────────────────────────────

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

// Ordered most to least important, since buildHintFit elides from the end.
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
