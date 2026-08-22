package ui

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/git-view/internal/diff"
)

// searchFilesWithMatchesAt builds one file of 60 numbered added lines, holding
// "target" on the content lines named. Content line i renders at visual line
// i+3, after the separator, the file header, and the @@ header.
func searchFilesWithMatchesAt(at ...int) []diff.File {
	var lines []diff.Line
	for i := range 60 {
		content := "line " + strconv.Itoa(i)
		if slices.Contains(at, i) {
			content = "the target line"
		}
		lines = append(lines, diff.Line{Type: diff.LineAdded, Content: content})
	}
	return []diff.File{{
		Name:  "first_file.go",
		Type:  diff.Normal,
		Hunks: []diff.Hunk{{Context: "func A()", NewStart: 1, NewCount: 60, Lines: lines}},
	}}
}

func searchSampleFiles() []diff.File {
	return searchFilesWithMatchesAt(10, 40)
}

const (
	firstMatchLine  = 13 // content line 10
	secondMatchLine = 43 // content line 40
)

// searchingViewer returns a viewer with term already committed.
func searchingViewer(t *testing.T, term string) *viewer {
	t.Helper()
	return searchIn(t, newViewer(searchSampleFiles(), 80, 20), term)
}

func searchIn(t *testing.T, v *viewer, term string) *viewer {
	t.Helper()
	v.handleKey(key("/"))
	for _, r := range term {
		v.handleKey(key(string(r)))
	}
	v.handleKey(key("enter"))
	return v
}

// ── Matching ─────────────────────────────────────────────────────────────────

func TestMatchSpans(t *testing.T) {
	tests := []struct {
		name, text, term string
		want             [][2]int
	}{
		{"one match", "hello world", "world", [][2]int{{6, 11}}},
		{"repeated", "aXaXa", "X", [][2]int{{1, 2}, {3, 4}}},
		{"adjacent", "abab", "ab", [][2]int{{0, 2}, {2, 4}}},
		{"no match", "hello", "zzz", nil},
		{"empty term", "hello", "", nil},
		{"case counts", "Hello", "hello", nil},
		{"rune offsets, not byte", "日本語 target", "target", [][2]int{{4, 10}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchSpans(tt.text, tt.term)
			if len(got) != len(tt.want) {
				t.Fatalf("matchSpans(%q, %q) = %v, want %v", tt.text, tt.term, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("span %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRenderDiff_RecordsMatchLines(t *testing.T) {
	rd := renderDiff(searchSampleFiles(), 80, nil, "target")

	want := []int{firstMatchLine, secondMatchLine}
	if len(rd.matchLines) != len(want) {
		t.Fatalf("matchLines = %v, want %v", rd.matchLines, want)
	}
	for i, line := range want {
		if rd.matchLines[i] != line {
			t.Errorf("match %d at line %d, want %d", i, rd.matchLines[i], line)
		}
	}
}

func TestRenderDiff_HighlightsTheMatchedText(t *testing.T) {
	rd := renderDiff(searchSampleFiles(), 80, nil, "target")

	line := strings.Split(rd.text, "\n")[firstMatchLine]
	if !strings.Contains(line, theme.SearchMatchStyle.Render("target")) {
		t.Errorf("expected the match to carry the highlight style, got %q", line)
	}
	// The highlight must not disturb the text itself.
	if !strings.Contains(stripAnsi(line), "the target line") {
		t.Errorf("expected the line's text intact, got %q", stripAnsi(line))
	}
}

func TestRenderDiff_NoSearchTermLeavesTheOutputAlone(t *testing.T) {
	plain := renderDiff(searchSampleFiles(), 80, nil, "")
	if plain.matchLines != nil {
		t.Errorf("expected no matches without a term, got %v", plain.matchLines)
	}
	if plain.text != renderDiff(searchSampleFiles(), 80, nil, "zzz").text {
		t.Error("a term that matches nothing should render the same as no term at all")
	}
}

func TestRenderDiff_SkipsTheHunkHeaderAndFileName(t *testing.T) {
	for _, term := range []string{"func A()", "first_file.go", "@@"} {
		rd := renderDiff(searchSampleFiles(), 80, nil, term)
		if len(rd.matchLines) != 0 {
			t.Errorf("term %q matched the diff's own headers at %v", term, rd.matchLines)
		}
	}
}

func TestRenderDiff_SkipsTheLineNumberGutter(t *testing.T) {
	// Content line 10 is numbered 11 and reads "the target line", so "11" occurs
	// in the gutter alone.
	rd := renderDiff(searchSampleFiles(), 80, nil, "11")

	for _, line := range rd.matchLines {
		if line == firstMatchLine {
			t.Error("the line-number gutter should not be searched")
		}
	}
}

func TestRenderDiff_HighlightsBothHalvesOfAWrappedMatch(t *testing.T) {
	files := []diff.File{{
		Name: "wrap.go",
		Type: diff.Normal,
		Hunks: []diff.Hunk{{
			NewStart: 1, NewCount: 1,
			Lines: []diff.Line{{Type: diff.LineAdded, Content: "aaaaXXXX"}},
		}},
	}}
	// A pane of 6 fits "1 aaaa" then "XXXX", splitting the match down the middle.
	rd := renderDiff(files, 6, nil, "aaXX")

	if len(rd.matchLines) != 2 {
		t.Fatalf("expected both halves recorded, got %v", rd.matchLines)
	}
	lines := strings.Split(rd.text, "\n")
	for _, i := range rd.matchLines {
		if !strings.Contains(lines[i], theme.SearchMatchStyle.Render("")[:2]) {
			t.Errorf("line %d carries no highlight: %q", i, lines[i])
		}
	}
}

// ── Prompt mode ──────────────────────────────────────────────────────────────

func TestViewer_SlashOpensTheSearchPrompt(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))

	if !v.searchActive || !v.searchEditing {
		t.Fatalf("expected an open, editable prompt, got active=%v editing=%v",
			v.searchActive, v.searchEditing)
	}
}

func TestViewer_TypingFillsTheSearchBox(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))
	// Every one of these is a command outside the prompt.
	for _, s := range []string{"n", "p", " ", "<", ">", "c", "j"} {
		v.handleKey(key(s))
	}

	if got := string(v.searchInput); got != "np <>cj" {
		t.Errorf("search box = %q, want %q", got, "np <>cj")
	}
	if v.searchCursor != 7 {
		t.Errorf("cursor at %d, want 7", v.searchCursor)
	}
}

func TestViewer_SearchBoxEditing(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))
	for _, r := range "abc" {
		v.handleKey(key(string(r)))
	}

	v.handleKey(key("ctrl+a"))
	if v.searchCursor != 0 {
		t.Errorf("^A left the cursor at %d, want 0", v.searchCursor)
	}
	v.handleKey(key("right"))
	v.handleKey(key("X"))
	if got := string(v.searchInput); got != "aXbc" {
		t.Errorf("after → and X, box = %q, want %q", got, "aXbc")
	}
	v.handleKey(key("ctrl+e"))
	if v.searchCursor != 4 {
		t.Errorf("^E left the cursor at %d, want 4", v.searchCursor)
	}
	v.handleKey(key("backspace"))
	if got := string(v.searchInput); got != "aXb" {
		t.Errorf("after backspace, box = %q, want %q", got, "aXb")
	}
	v.handleKey(key("left"))
	v.handleKey(key("delete"))
	if got := string(v.searchInput); got != "aX" {
		t.Errorf("after delete, box = %q, want %q", got, "aX")
	}
}

func TestViewer_CursorKeysStopAtTheEndsOfTheBox(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))
	v.handleKey(key("a"))

	v.handleKey(key("left"))
	v.handleKey(key("left"))
	if v.searchCursor != 0 {
		t.Errorf("cursor ran past the start to %d", v.searchCursor)
	}
	v.handleKey(key("right"))
	v.handleKey(key("right"))
	if v.searchCursor != 1 {
		t.Errorf("cursor ran past the end to %d", v.searchCursor)
	}
	v.handleKey(key("backspace"))
	v.handleKey(key("backspace"))
	if len(v.searchInput) != 0 {
		t.Errorf("expected an empty box, got %q", string(v.searchInput))
	}
}

func TestViewer_TheWindowStillMovesWhileTyping(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))

	v.handleKey(key("down"))
	v.handleKey(key("down"))
	if v.offset != 2 {
		t.Errorf("offset = %d, want 2", v.offset)
	}
	v.handleKey(key("pgdown"))
	if v.offset != 22 {
		t.Errorf("offset = %d, want 22", v.offset)
	}
	// "<" and ">" are typed text here, so Home and End have to stand in.
	v.handleKey(key("home"))
	if v.offset != 0 {
		t.Errorf("offset = %d, want 0", v.offset)
	}
	if len(v.searchInput) != 0 {
		t.Errorf("navigation keys leaked into the box: %q", string(v.searchInput))
	}
}

func TestViewer_EscapeClosesTheSearchAndItsHighlights(t *testing.T) {
	v := searchingViewer(t, "target")
	withSearch := v.text

	v.handleKey(key("esc"))

	if v.searchActive || v.searchEditing {
		t.Error("expected the prompt closed")
	}
	if v.searchTerm != "" || len(v.matchLines) != 0 {
		t.Errorf("expected the term dropped, got %q with %v", v.searchTerm, v.matchLines)
	}
	if v.text == withSearch {
		t.Error("expected the highlights re-rendered away")
	}
}

func TestViewer_SlashReopensTheBoxWithTheOldTerm(t *testing.T) {
	v := searchingViewer(t, "target")
	v.handleKey(key("/"))

	if !v.searchEditing {
		t.Fatal("expected the box editable again")
	}
	if got := string(v.searchInput); got != "target" {
		t.Errorf("box = %q, want the previous term", got)
	}
	if v.searchCursor != len("target") {
		t.Errorf("cursor at %d, want the end of the term", v.searchCursor)
	}
}

// ── Jumping ──────────────────────────────────────────────────────────────────

// centring puts the match paneHeight/2 rows below the top of the pane.
func wantOffsetFor(line, paneHeight int) int {
	return line - paneHeight/2
}

func TestViewer_EnterCentresTheFirstMatch(t *testing.T) {
	v := searchingViewer(t, "target")

	if v.searchEditing {
		t.Error("Enter should hand the keys back to the viewer")
	}
	if !v.searchActive {
		t.Error("the prompt should stay up after Enter")
	}
	if want := wantOffsetFor(firstMatchLine, 20); v.offset != want {
		t.Errorf("offset = %d, want %d", v.offset, want)
	}
}

func TestViewer_EnterWrapsToTheTopWhenNothingIsBelow(t *testing.T) {
	// Both matches, at visual lines 13 and 23, sit above the last window.
	v := newViewer(searchFilesWithMatchesAt(10, 20), 80, 20)
	v.handleKey(key(">")) // to the end of the diff

	searchIn(t, v, "target")

	if want := wantOffsetFor(13, 20); v.offset != want {
		t.Errorf("offset = %d, want the first match at %d", v.offset, want)
	}
}

func TestViewer_NextAndPreviousMatch(t *testing.T) {
	v := searchingViewer(t, "target")

	v.handleKey(key("n"))
	if want := wantOffsetFor(secondMatchLine, 20); v.offset != want {
		t.Errorf("after n, offset = %d, want %d", v.offset, want)
	}
	v.handleKey(key("p"))
	if want := wantOffsetFor(firstMatchLine, 20); v.offset != want {
		t.Errorf("after p, offset = %d, want %d", v.offset, want)
	}
}

func TestViewer_JumpKeysDoNothingWithoutAMatchThatWay(t *testing.T) {
	v := searchingViewer(t, "target")
	v.handleKey(key("n")) // now on the last match

	before := v.offset
	v.handleKey(key("n"))
	if v.offset != before {
		t.Errorf("n moved to %d with nothing below; want %d", v.offset, before)
	}
	v.handleKey(key("p"))
	v.handleKey(key("p"))
	before = v.offset
	v.handleKey(key("p"))
	if v.offset != before {
		t.Errorf("p moved to %d with nothing above; want %d", v.offset, before)
	}
}

// The window's middle row, not its top, is what "next" counts from — so a match
// the user has scrolled up from is still ahead of them.
func TestViewer_NextMatchReturnsToTheMatchScrolledUpFrom(t *testing.T) {
	v := searchingViewer(t, "target")
	v.handleKey(key("n"))
	onMatch := v.offset

	v.scrollUp(10) // half a page
	v.handleKey(key("n"))

	if v.offset != onMatch {
		t.Errorf("offset = %d, want to be back on the match at %d", v.offset, onMatch)
	}
}

func TestViewer_JumpKeysAreUnboundWithoutASearch(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.scrollDown(5)

	v.handleKey(key("n"))
	v.handleKey(key("p"))

	if v.offset != 5 {
		t.Errorf("offset = %d; n and p should do nothing outside a search", v.offset)
	}
}

func TestViewer_CentringClampsAtTheEndsOfTheDiff(t *testing.T) {
	// A pane taller than the diff cannot centre anything.
	v := newViewer(searchSampleFiles(), 80, 200)
	v.handleKey(key("/"))
	for _, r := range "target" {
		v.handleKey(key(string(r)))
	}
	v.handleKey(key("enter"))

	if v.offset != 0 {
		t.Errorf("offset = %d, want 0", v.offset)
	}
}

func TestViewer_SearchingForNothingFoundReportsNoMatches(t *testing.T) {
	v := searchingViewer(t, "nowhere")

	if len(v.matchLines) != 0 {
		t.Fatalf("expected no matches, got %v", v.matchLines)
	}
	if v.offset != 0 {
		t.Errorf("offset = %d; a fruitless search should not move the window", v.offset)
	}
	if !strings.Contains(stripAnsi(v.renderSearchPrompt(60)), "no matches") {
		t.Errorf("expected the prompt to say so, got %q", stripAnsi(v.renderSearchPrompt(60)))
	}
}

// ── Prompt line ──────────────────────────────────────────────────────────────

func TestViewer_PromptShowsTheTermAndTheCursor(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))
	for _, r := range "tar" {
		v.handleKey(key(string(r)))
	}

	prompt := v.renderSearchPrompt(60)
	if !strings.HasPrefix(stripAnsi(prompt), "/tar") {
		t.Errorf("prompt = %q, want it to start with /tar", stripAnsi(prompt))
	}
	if !strings.Contains(prompt, theme.SearchCursorStyle.Render(" ")) {
		t.Error("expected a cursor block at the end of the term")
	}
	if !strings.Contains(stripAnsi(prompt), "Enter") {
		t.Errorf("expected the Enter hint while typing, got %q", stripAnsi(prompt))
	}
}

func TestViewer_PromptCountsTheMatchesOnceCommitted(t *testing.T) {
	v := searchingViewer(t, "target")

	prompt := stripAnsi(v.renderSearchPrompt(60))
	if !strings.Contains(prompt, "match 1 of 2") {
		t.Errorf("prompt = %q, want the position and the count", prompt)
	}
	if !strings.Contains(prompt, "n/p") {
		t.Errorf("prompt = %q, want the jump keys", prompt)
	}
	if strings.Contains(prompt, "Enter") {
		t.Errorf("prompt = %q, should no longer offer Enter", prompt)
	}
}

func TestViewer_PromptFitsItsWidth(t *testing.T) {
	v := searchingViewer(t, "target")

	for _, width := range []int{80, 40, 20, 10, 4, 1} {
		v.handleKey(key("/")) // widest state: cursor plus the Enter hints
		prompt := v.renderSearchPrompt(width)
		if got := len([]rune(stripAnsi(prompt))); got > width {
			t.Errorf("width %d: prompt is %d columns wide: %q", width, got, stripAnsi(prompt))
		}
		v.handleKey(key("enter"))
	}
}

func TestViewer_LongTermSlidesToKeepTheCursorVisible(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)
	v.handleKey(key("/"))
	for _, r := range "abcdefghijklmnopqrstuvwxyz" {
		v.handleKey(key(string(r)))
	}

	line := stripAnsi(v.searchInputLine(10))
	if len([]rune(line)) != 10 {
		t.Fatalf("input line = %q, want 10 columns", line)
	}
	if !strings.HasSuffix(line, "z ") {
		t.Errorf("input line = %q, want the end of the term with the cursor", line)
	}
}

// ── Hints ────────────────────────────────────────────────────────────────────

func TestViewer_HintsOfferSearch(t *testing.T) {
	v := newViewer(searchSampleFiles(), 80, 20)

	hints := strings.Join(v.hintPairs(), " ")
	if !strings.Contains(hints, "search") {
		t.Errorf("expected a search hint, got %q", hints)
	}
}

// ── The app model around the prompt ──────────────────────────────────────────

// typing opens the prompt and types term into it, one key at a time.
func typing(t *testing.T, m Model, term string) Model {
	t.Helper()
	m, _ = send(t, m, "/")
	for _, r := range term {
		m, _ = send(t, m, string(r))
	}
	return m
}

func TestKeys_TheGlobalLettersYieldToTheSearchBox(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "qeg")

	if m.quitting {
		t.Error("q should be text while the box is open, not quit")
	}
	if got := string(m.viewer.searchInput); got != "qeg" {
		t.Errorf("search box = %q, want %q", got, "qeg")
	}
	if m.status != "" {
		t.Errorf("E and g should not have acted, status = %q", m.status)
	}
}

func TestKeys_TheGlobalLettersComeBackAfterEnter(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "target")
	m, _ = send(t, m, "enter")

	quit, _ := send(t, m, "q")
	if !quit.quitting {
		t.Error("q should quit once the term is committed")
	}
}

func TestKeys_CtrlCQuitsEvenFromTheSearchBox(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "tar")

	quit, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !quit.(Model).quitting {
		t.Error("Ctrl-C should always quit")
	}
}

func TestEscClosesTheSearchBeforeTheViewer(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "target")
	m, _ = send(t, m, "enter")

	afterEsc, _ := send(t, m, "esc")
	if afterEsc.viewer == nil {
		t.Fatal("Esc should close the search, not the viewer")
	}
	if afterEsc.viewer.searchActive {
		t.Error("Esc should have closed the search")
	}

	closed, _ := send(t, afterEsc, "esc")
	if closed.viewer != nil {
		t.Error("a second Esc should close the viewer")
	}
}

func TestEscFromTheSearchBoxKeepsTheViewer(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "tar")

	afterEsc, _ := send(t, m, "esc")
	if afterEsc.viewer == nil {
		t.Fatal("Esc should cancel the search, not close the viewer")
	}
	if afterEsc.viewer.searchActive {
		t.Error("Esc should have cancelled the search")
	}
}

func TestTabStillLeavesTheViewerDuringASearch(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "tar")

	back, _ := send(t, m, "tab")
	if back.viewer != nil {
		t.Error("Tab should close the viewer even with the search open")
	}
}

func TestView_SearchPromptReplacesTheHintBar(t *testing.T) {
	m := onViewer(t, searchSampleFiles(), 100, 30)
	hinted := stripAnsi(m.View())
	if !strings.Contains(hinted, "scroll") {
		t.Fatal("expected the usual hints before a search")
	}

	m = typing(t, m, "target")
	searching := stripAnsi(m.View())
	if strings.Contains(searching, "scroll") {
		t.Error("the prompt should have taken the hint bar over")
	}
	if !strings.Contains(searching, "/target") {
		t.Error("expected the prompt to show the term")
	}

	m, _ = send(t, m, "esc")
	if !strings.Contains(stripAnsi(m.View()), "scroll") {
		t.Error("Esc should bring the hints back")
	}
}

func TestView_SearchKeepsTheLayoutOneLine(t *testing.T) {
	m := typing(t, onViewer(t, searchSampleFiles(), 100, 30), "a long search term here")

	if got := len(strings.Split(m.View(), "\n")); got != 30 {
		t.Errorf("the screen is %d rows, want 30", got)
	}
}

func TestViewer_PromptFollowsThePositionThroughTheMatches(t *testing.T) {
	v := searchingViewer(t, "target")

	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 1 of 2") {
		t.Errorf("prompt = %q, want match 1 of 2", got)
	}
	v.handleKey(key("n"))
	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 2 of 2") {
		t.Errorf("after n, prompt = %q, want match 2 of 2", got)
	}
	v.handleKey(key("p"))
	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 1 of 2") {
		t.Errorf("after p, prompt = %q, want match 1 of 2", got)
	}
}

// The position names the match the window is nearest, so scrolling away from one
// does not renumber it until another is closer.
func TestViewer_PositionHoldsWhileScrollingNearTheSameMatch(t *testing.T) {
	v := searchingViewer(t, "target")
	v.handleKey(key("n"))

	v.scrollUp(10) // half a page, the same distance n measures from
	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 2 of 2") {
		t.Errorf("prompt = %q, want match 2 of 2", got)
	}
	v.handleKey(key("home"))
	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 1 of 2") {
		t.Errorf("at the top, prompt = %q, want match 1 of 2", got)
	}
}

func TestViewer_PromptShortensThePositionOnANarrowLine(t *testing.T) {
	v := searchingViewer(t, "target")

	if got := stripAnsi(v.renderSearchPrompt(16)); !strings.Contains(got, "1/2") {
		t.Errorf("prompt = %q, want the short form", got)
	}
}

// The jump starts from the top of the window, not its middle row: a match in the
// first few lines of the diff must not be passed over for a later one.
func TestViewer_EnterTakesAMatchAboveTheMiddleRow(t *testing.T) {
	// Visual line 5, well above the middle row of a pane 20 tall.
	v := searchIn(t, newViewer(searchFilesWithMatchesAt(2, 40), 80, 20), "target")

	if v.offset != 0 {
		t.Errorf("offset = %d, want the top of the diff", v.offset)
	}
	if got := stripAnsi(v.renderSearchPrompt(60)); !strings.Contains(got, "match 1 of 2") {
		t.Errorf("prompt = %q, want the first match", got)
	}
}

func TestViewer_EnterStartsFromTheTopOfTheWindow(t *testing.T) {
	v := newViewer(searchFilesWithMatchesAt(32, 57), 80, 20)
	v.scrollDown(30) // the first match, at visual line 35, is now five rows down

	searchIn(t, v, "target")

	if want := wantOffsetFor(35, 20); v.offset != want {
		t.Errorf("offset = %d, want the match in view centred at %d", v.offset, want)
	}
}

// Landing on a match must not leave n with nowhere to go, however the centring
// was clamped.
func TestViewer_NextStillAdvancesAfterLandingAtTheTop(t *testing.T) {
	v := searchIn(t, newViewer(searchFilesWithMatchesAt(2, 40), 80, 20), "target")

	v.handleKey(key("n"))

	if want := wantOffsetFor(43, 20); v.offset != want {
		t.Errorf("offset = %d, want the second match at %d", v.offset, want)
	}
}
