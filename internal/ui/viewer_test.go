package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/diff"
)

// key builds a KeyMsg for a single-character or named key.
func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestNewViewer_RendersContent(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)

	if v.text == "" {
		t.Error("expected rendered diff text")
	}
	if len(v.fileNames) != 2 {
		t.Errorf("expected 2 file names, got %d", len(v.fileNames))
	}
	if len(v.fileStarts) != 2 {
		t.Errorf("expected 2 file starts, got %d", len(v.fileStarts))
	}
	if len(v.collapsed) != 2 {
		t.Errorf("expected collapse state for 2 files, got %d", len(v.collapsed))
	}
	if v.offset != 0 {
		t.Errorf("expected offset 0, got %d", v.offset)
	}
}

func TestNewViewer_EmptyFiles(t *testing.T) {
	v := newViewer(nil, 80, 20)

	if v.text != "" {
		t.Errorf("expected empty text, got %q", v.text)
	}
	if v.totalLines() != 0 {
		t.Errorf("expected 0 total lines, got %d", v.totalLines())
	}
	if got := v.currentFileName(); got != "" {
		t.Errorf("expected no current file, got %q", got)
	}
}

func TestViewer_Scroll(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)

	v.scrollDown(5)
	if v.offset != 5 {
		t.Errorf("offset after scrollDown(5) = %d, want 5", v.offset)
	}
	v.scrollUp(2)
	if v.offset != 3 {
		t.Errorf("offset after scrollUp(2) = %d, want 3", v.offset)
	}
	v.scrollUp(100)
	if v.offset != 0 {
		t.Errorf("scrollUp should clamp at 0, got %d", v.offset)
	}
	v.scrollDown(10000)
	if want := v.totalLines() - v.paneHeight; v.offset != want {
		t.Errorf("scrollDown should clamp at %d, got %d", want, v.offset)
	}
}

func TestViewer_ScrollKeys(t *testing.T) {
	cases := []struct {
		keys []string
		want int
	}{
		{[]string{"down"}, 1},
		{[]string{"down", "down", "up"}, 1},
		{[]string{"pgdown"}, 10},
		{[]string{" "}, 10},
		{[]string{"pgdown", "pgup"}, 0},
		{[]string{"pgdown", "b"}, 0},
		{[]string{"j", "j"}, 2},
		{[]string{"j", "j", "k"}, 1},
		{[]string{"pgdown", "<"}, 0},
	}

	for _, tc := range cases {
		v := newViewer(largeSampleFiles(), 80, 10)
		for _, k := range tc.keys {
			v.handleKey(key(k))
		}
		if v.offset != tc.want {
			t.Errorf("keys %v: offset = %d, want %d", tc.keys, v.offset, tc.want)
		}
	}
}

func TestViewer_JumpToBottom(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key(">"))

	if want := v.totalLines() - v.paneHeight; v.offset != want {
		t.Errorf("offset after > = %d, want %d", v.offset, want)
	}
}

func TestViewer_IgnoresUnhandledKeys(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("z"))

	if v.offset != 0 || v.lineSelectMode {
		t.Errorf("unhandled key changed state: offset=%d lineSelect=%v", v.offset, v.lineSelectMode)
	}
}

func TestViewer_CurrentFileTracksScroll(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)

	if got := v.currentFileIndex(); got != 0 {
		t.Errorf("at the top the current file should be 0, got %d", got)
	}
	v.offset = v.fileStarts[1]
	if got := v.currentFileIndex(); got != 1 {
		t.Errorf("after scrolling to file 2 the current file should be 1, got %d", got)
	}
	if got := v.currentFileName(); got != "second_file.go" {
		t.Errorf("currentFileName = %q, want second_file.go", got)
	}
}

func TestViewer_SetSizeRerendersOnWidthChange(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)
	before := v.text

	v.setSize(40, 20)
	if v.text == before {
		t.Error("changing the pane width should re-render, since backgrounds are width-padded")
	}
	if got := lineWidth(t, v.text); got != 40 {
		t.Errorf("re-rendered content width = %d, want 40", got)
	}
}

func TestViewer_SetSizeClampsOffset(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.scrollDown(1000)
	tall := v.totalLines()

	v.setSize(80, tall+50)
	if v.offset != 0 {
		t.Errorf("growing the pane past the content should clamp offset to 0, got %d", v.offset)
	}
}

// lineWidth returns the visible width of the widest styled line in text.
func lineWidth(t *testing.T, text string) int {
	t.Helper()
	widest := 0
	for _, line := range strings.Split(text, "\n") {
		if w := len([]rune(stripAnsi(line))); w > widest {
			widest = w
		}
	}
	return widest
}

// ── Collapse ─────────────────────────────────────────────────────────────────

func TestViewer_ToggleCollapseHidesContent(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)
	v.handleKey(key("c"))

	if !v.collapsed[0] {
		t.Fatal("expected file 0 to be collapsed")
	}
	if strings.Contains(stripAnsi(v.text), "fmt.Println(\"hello\")") {
		t.Error("collapsed file still shows hunk content")
	}

	v.handleKey(key("c"))
	if v.collapsed[0] {
		t.Fatal("expected file 0 to be expanded again")
	}
	if !strings.Contains(stripAnsi(v.text), "fmt.Println(\"hello\")") {
		t.Error("expanded file does not show hunk content")
	}
}

func TestViewer_ToggleCollapseScrollsToFileHeader(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.offset = v.fileStarts[1] + 5
	v.handleKey(key("c"))

	// The toggled file's header must be on screen so the user sees what
	// changed. It cannot always reach the top row: collapsing shortens the
	// diff, and the scroll offset is clamped to the new end of the content.
	header := v.fileStarts[1] + 1
	if header < v.offset || header >= v.offset+v.paneHeight {
		t.Errorf("collapsed file header at %d is outside the pane [%d, %d)",
			header, v.offset, v.offset+v.paneHeight)
	}
}

func TestViewer_ToggleCollapseNoOpForHunklessFile(t *testing.T) {
	v := newViewer([]diff.File{{Name: "bin/blob", Type: diff.Binary}}, 80, 20)
	before := v.text
	v.handleKey(key("c"))

	if v.collapsed[0] {
		t.Error("a file with no hunks should not collapse")
	}
	if v.text != before {
		t.Error("collapsing a hunkless file should not change the rendering")
	}
}

func TestViewer_ToggleCollapseAll(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)
	v.handleKey(key("C"))

	for i, c := range v.collapsed {
		if !c {
			t.Errorf("file %d should be collapsed", i)
		}
	}

	v.handleKey(key("C"))
	for i, c := range v.collapsed {
		if c {
			t.Errorf("file %d should be expanded", i)
		}
	}
}

func TestViewer_ToggleCollapseAllCollapsesWhenMixed(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)
	v.collapsed[0] = true
	v.handleKey(key("C"))

	for i, c := range v.collapsed {
		if !c {
			t.Errorf("file %d should be collapsed when the set was mixed", i)
		}
	}
}

// ── Line select ──────────────────────────────────────────────────────────────

func TestLineSelect_EnterPlacesSelectionOnSelectableLine(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))

	if !v.lineSelectMode {
		t.Fatal("expected line-select mode")
	}
	if !v.isSelectable(v.selectedLine) {
		t.Errorf("selected line %d is not selectable", v.selectedLine)
	}
	if v.selectedLine < v.offset || v.selectedLine >= v.offset+v.paneHeight {
		t.Errorf("selected line %d is outside the visible pane [%d, %d)",
			v.selectedLine, v.offset, v.offset+v.paneHeight)
	}
}

func TestLineSelect_EnterNoOpWhenNothingSelectable(t *testing.T) {
	v := newViewer([]diff.File{{Name: "bin/blob", Type: diff.Binary}}, 80, 10)
	v.handleKey(key("enter"))

	if v.lineSelectMode {
		t.Error("should not enter line-select mode with no selectable lines")
	}
}

func TestLineSelect_EnterNoOpOnEmptyDiff(t *testing.T) {
	v := newViewer(nil, 80, 10)
	v.handleKey(key("enter"))

	if v.lineSelectMode {
		t.Error("should not enter line-select mode on an empty diff")
	}
}

func TestLineSelect_Move(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))
	start := v.selectedLine

	v.handleKey(key("down"))
	if v.selectedLine <= start {
		t.Errorf("down should advance the selection past %d, got %d", start, v.selectedLine)
	}
	if !v.isSelectable(v.selectedLine) {
		t.Error("selection landed on a non-selectable line")
	}

	v.handleKey(key("up"))
	if v.selectedLine != start {
		t.Errorf("up should return to %d, got %d", start, v.selectedLine)
	}
}

func TestLineSelect_MoveClampsAtEnds(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))

	v.handleKey(key(">"))
	last := v.selectedLine
	v.handleKey(key("down"))
	if v.selectedLine != last {
		t.Errorf("selection should clamp at the last selectable line %d, got %d", last, v.selectedLine)
	}

	v.handleKey(key("<"))
	first := v.selectedLine
	v.handleKey(key("up"))
	if v.selectedLine != first {
		t.Errorf("selection should clamp at the first selectable line %d, got %d", first, v.selectedLine)
	}
}

func TestLineSelect_ScrollsToKeepSelectionVisible(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))

	for range 40 {
		v.handleKey(key("down"))
	}
	if v.selectedLine < v.offset || v.selectedLine >= v.offset+v.paneHeight {
		t.Errorf("selection %d scrolled out of the pane [%d, %d)",
			v.selectedLine, v.offset, v.offset+v.paneHeight)
	}
}

func TestLineSelect_UpdatesCurrentFile(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))
	v.selectedLine = v.fileStarts[1] + 3

	if got := v.currentFileIndex(); got != 1 {
		t.Errorf("current file should follow the selection, got %d", got)
	}
}

func TestLineSelect_ExitFreezesFileUntilScroll(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))
	// Select a line in the second file while the pane is still scrolled to the first.
	v.selectedLine = v.fileStarts[1] + 3
	v.handleKey(key("esc"))

	if v.lineSelectMode {
		t.Fatal("Esc should leave line-select mode")
	}
	if got := v.currentFileIndex(); got != 1 {
		t.Errorf("file index should stay frozen at 1 after exit, got %d", got)
	}

	v.handleKey(key("up"))
	if got := v.currentFileIndex(); got != 0 {
		t.Errorf("scrolling should unfreeze the file index, got %d", got)
	}
}

func TestLineSelect_SurvivesCollapseToggle(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	v.handleKey(key("enter"))
	v.handleKey(key("c"))

	if !v.lineSelectMode {
		t.Error("collapsing should keep line-select mode active")
	}
	if !v.isSelectable(v.selectedLine) {
		t.Errorf("selection %d is not selectable after collapse", v.selectedLine)
	}
}

func TestLineSelect_SelectedLocation(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)
	v.handleKey(key("enter"))

	file, line := v.selectedLocation()
	if file == "" {
		t.Error("expected a file name for the selected line")
	}
	if line <= 0 {
		t.Errorf("expected a positive line number, got %d", line)
	}
	if want := v.lineMeta[v.selectedLine].lineNum; line != want {
		t.Errorf("line = %d, want %d", line, want)
	}
}

func TestSelectedLocation_OutsideLineSelectHasNoLine(t *testing.T) {
	v := newViewer(sampleFiles(), 80, 20)

	file, line := v.selectedLocation()
	if file != "internal/ui/app.go" {
		t.Errorf("expected the file at the top of the pane, got %q", file)
	}
	if line != 0 {
		t.Errorf("expected no line number outside line-select mode, got %d", line)
	}
}

// ── Rendering ────────────────────────────────────────────────────────────────

func TestViewer_RenderPaneHeight(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	// Content rows plus the top and bottom border rows.
	if got, want := len(strings.Split(v.renderPane(), "\n")), 12; got != want {
		t.Errorf("rendered pane height = %d, want %d", got, want)
	}
}

func TestViewer_RenderPaneEmptyDiff(t *testing.T) {
	v := newViewer(nil, 40, 6)
	if !strings.Contains(stripAnsi(v.renderPane()), "No diff content") {
		t.Error("expected the empty-state message")
	}
}

func TestViewer_RenderPaneHighlightsSelection(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	before := v.renderPane()
	v.handleKey(key("enter"))

	if v.renderPane() == before {
		t.Error("entering line-select mode should change the rendered pane")
	}
}

func TestViewer_RenderPaneShowsScrollbar(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)
	if !strings.Contains(v.renderPane(), "█") {
		t.Error("expected a scrollbar thumb when the content overflows the pane")
	}

	short := newViewer(sampleFiles(), 80, 200)
	if strings.Contains(short.renderPane(), "█") {
		t.Error("expected no scrollbar thumb when all content fits")
	}
}

func TestViewer_HintPairsChangeWithMode(t *testing.T) {
	v := newViewer(largeSampleFiles(), 80, 10)

	scrollHint := strings.Join(v.hintPairs(), " ")
	if !strings.Contains(scrollHint, "select lines") {
		t.Errorf("scroll-mode hint should offer line select, got %q", scrollHint)
	}
	if strings.Contains(scrollHint, "terminal") {
		t.Errorf("the terminal command should be gone, got %q", scrollHint)
	}

	v.handleKey(key("enter"))
	selectHint := strings.Join(v.hintPairs(), " ")
	if !strings.Contains(selectHint, "exit select") {
		t.Errorf("line-select hint should offer an exit, got %q", selectHint)
	}
	for _, want := range []string{"edit line", "github"} {
		if !strings.Contains(selectHint, want) {
			t.Errorf("line-select hint should mention %q, got %q", want, selectHint)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	styled := theme.AddedStyle.Render("hello")
	if got := stripAnsi(styled); got != "hello" {
		t.Errorf("stripAnsi = %q, want %q", got, "hello")
	}
}
