package ui

import (
	"fmt"
	"regexp"
	"slices"
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

// viewer is the scrollable diff pane. Its dimensions cover the content pane
// only; the app model owns the status bar.
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

	// Search. The prompt line stays up while searchActive, but only takes typed
	// characters while searchEditing; see handleSearchEditKey. searchTerm is the
	// committed input — what the highlights and matchLines were built from.
	searchActive  bool
	searchEditing bool
	searchInput   []rune
	searchCursor  int // an index into searchInput; len(searchInput) is the end
	searchTerm    string
	matchLines    []int
}

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
	rd := renderDiff(m.files, w, m.collapsed, m.searchTerm)
	m.text = rd.text
	m.fileStarts = rd.fileStarts
	m.lineMeta = rd.lineMeta
	m.matchLines = rd.matchLines
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

// ── Search ───────────────────────────────────────────────────────────────────

// startSearch opens the prompt. Any earlier term stays in the box, so a second
// "/" is a chance to amend it rather than retype it.
func (m *viewer) startSearch() {
	m.searchActive = true
	m.searchEditing = true
	m.searchCursor = len(m.searchInput)
}

// endSearch takes the highlights down along with the prompt.
func (m *viewer) endSearch() {
	if !m.searchActive {
		return
	}
	m.searchActive = false
	m.searchEditing = false
	if m.searchTerm != "" {
		m.searchTerm = ""
		m.rerender()
	}
}

// commitSearch highlights the typed term and jumps to the first match at or
// below the top of the pane, wrapping to the first match in the diff if none is
// below. It starts from the top rather than searchOrigin so that a match already
// in view is chosen.
func (m *viewer) commitSearch() {
	m.searchEditing = false
	m.searchTerm = string(m.searchInput)
	m.rerender()

	if len(m.matchLines) == 0 {
		return
	}
	for _, line := range m.matchLines {
		if line >= m.offset {
			m.centreOn(line)
			return
		}
	}
	m.centreOn(m.matchLines[0])
}

// searchOrigin is the position n and p measure from. It is the middle row
// because centreOn leaves a match there, so the next "n" moves past it.
func (m *viewer) searchOrigin() int {
	return m.offset + m.paneHeight/2
}

// centreOn puts a line in the middle of the pane, as near to it as the ends of
// the content allow.
func (m *viewer) centreOn(line int) {
	m.offset = line - m.paneHeight/2
	m.clampScroll()
	m.clearFrozenFileIdx()
}

// currentMatch returns the index of the match nearest the pane's middle row, or
// -1 if there are none.
func (m *viewer) currentMatch() int {
	origin := m.searchOrigin()
	best, bestGap := -1, 0
	for i, line := range m.matchLines {
		gap := line - origin
		if gap < 0 {
			gap = -gap
		}
		if best == -1 || gap < bestGap {
			best, bestGap = i, gap
			continue
		}
		// The lines ascend, so once the gap starts growing it keeps growing.
		break
	}
	return best
}

// jumpToMatch moves to the nearest match on one side of the current position,
// and stays put when there is none that way. direction is +1 for the next match,
// -1 for the previous.
func (m *viewer) jumpToMatch(direction int) {
	origin := m.searchOrigin()
	if direction > 0 {
		for _, line := range m.matchLines {
			if line > origin {
				m.centreOn(line)
				return
			}
		}
		return
	}
	for i := len(m.matchLines) - 1; i >= 0; i-- {
		if m.matchLines[i] < origin {
			m.centreOn(m.matchLines[i])
			return
		}
	}
}

func (m *viewer) insertSearchRunes(runes []rune) {
	m.searchInput = slices.Insert(m.searchInput, m.searchCursor, runes...)
	m.searchCursor += len(runes)
}

func (m *viewer) deleteSearchRune(at int) {
	if at < 0 || at >= len(m.searchInput) {
		return
	}
	m.searchInput = slices.Delete(m.searchInput, at, at+1)
	if m.searchCursor > at {
		m.searchCursor--
	}
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
	if m.searchEditing {
		return m.handleSearchEditKey(msg)
	}
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
	case "/":
		m.startSearch()
	// n and p belong to the search, so they stay unbound until it opens.
	case "n":
		if m.searchActive {
			m.jumpToMatch(1)
		}
	case "p":
		if m.searchActive {
			m.jumpToMatch(-1)
		}
	case "esc":
		// The app model routes Escape here only while a search is up; without one
		// it closes the viewer.
		m.endSearch()
	}
	return nil
}

// handleSearchEditKey runs while the user types a term. All printable
// characters go into the box, so only non-printable keys scroll the pane.
func (m *viewer) handleSearchEditKey(msg tea.KeyMsg) tea.Cmd {
	if !msg.Alt {
		switch msg.Type {
		case tea.KeyRunes:
			m.insertSearchRunes(msg.Runes)
			return nil
		case tea.KeySpace:
			m.insertSearchRunes([]rune(" "))
			return nil
		}
	}

	switch msg.String() {
	case "enter":
		m.commitSearch()
	case "esc":
		m.endSearch()

	// Edit line.
	case "ctrl+a":
		m.searchCursor = 0
	case "ctrl+e":
		m.searchCursor = len(m.searchInput)
	case "left":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
	case "right":
		if m.searchCursor < len(m.searchInput) {
			m.searchCursor++
		}
	case "backspace":
		m.deleteSearchRune(m.searchCursor - 1)
	case "delete":
		m.deleteSearchRune(m.searchCursor)

	// Scrolling. "<" and ">" are text here, so only Home and End reach the ends.
	case "up":
		m.clearFrozenFileIdx()
		m.scrollUp(1)
	case "down":
		m.clearFrozenFileIdx()
		m.scrollDown(1)
	case "pgup":
		m.clearFrozenFileIdx()
		m.scrollUp(m.paneHeight)
	case "pgdown":
		m.clearFrozenFileIdx()
		m.scrollDown(m.paneHeight)
	case "home":
		m.clearFrozenFileIdx()
		m.offset = 0
	case "end":
		m.clearFrozenFileIdx()
		m.offset = m.totalLines()
		m.clampScroll()
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
		"↑↓", "scroll", "Enter", "select lines", "/", "search",
		"E", "edit file", "g", "github", "c/C", "collapse",
		"PgUp/Dn", "page", "</>", "ends", "Q", "quit",
	}
}

// renderSearchPrompt replaces the hint bar while a search is open: the term on
// the left, and the status in the remaining width on the right.
func (m *viewer) renderSearchPrompt(width int) string {
	left := m.searchInputLine(width)
	right := m.searchStatus(max(width-lipgloss.Width(left)-2, 0))
	if right == "" {
		return left
	}
	return joinEnds(left, right, width)
}

// searchInputLine draws "/term", with a block cursor while it is being typed.
// A term too long for the line slides left to keep the cursor in view.
func (m *viewer) searchInputLine(width int) string {
	runes := append([]rune{'/'}, m.searchInput...)
	if !m.searchEditing {
		return theme.SearchPromptStyle.Render(truncateLine(string(runes), width))
	}

	// The cursor needs a cell of its own when it sits past the last character.
	runes = append(runes, ' ')
	cursor := m.searchCursor + 1

	start := 0
	if cursor >= width {
		start = cursor - width + 1
	}
	end := min(len(runes), start+width)

	return theme.SearchPromptStyle.Render(string(runes[start:cursor])) +
		theme.SearchCursorStyle.Render(string(runes[cursor:cursor+1])) +
		theme.SearchPromptStyle.Render(string(runes[cursor+1:end]))
}

// searchStatus reports the search result and the keys that act on it, dropping
// the keys first when the line is narrow.
func (m *viewer) searchStatus(width int) string {
	if m.searchEditing {
		return firstThatFits(width, buildHint("Enter", "search", "Esc", "cancel"))
	}
	if m.searchTerm == "" {
		return firstThatFits(width, buildHint("Esc", "exit search"))
	}

	keys := buildHint("n/p", "next/prev", "Esc", "exit search")
	if len(m.matchLines) == 0 {
		found := theme.ErrorStyle.Render("no matches")
		return firstThatFits(width, joinHint(found, keys), found)
	}

	position := m.currentMatch() + 1
	total := len(m.matchLines)
	found := theme.HintDescStyle.Render(fmt.Sprintf("match %d of %d", position, total))
	short := theme.HintDescStyle.Render(fmt.Sprintf("%d/%d", position, total))
	return firstThatFits(width, joinHint(found, keys), found, short)
}
