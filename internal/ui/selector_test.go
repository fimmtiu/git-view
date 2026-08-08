package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/git"
)

func newSelector(n, width, listHeight int) *selector {
	s := &selector{}
	s.setSize(width, listHeight)
	s.setRows(buildCommitRows(sampleCommits(n), -1, false))
	return s
}

// ── Row building ─────────────────────────────────────────────────────────────

func TestBuildCommitRows_PlainList(t *testing.T) {
	rows := buildCommitRows(sampleCommits(3), -1, false)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for i, r := range rows {
		if r.separator {
			t.Errorf("row %d should not be a separator", i)
		}
	}
}

func TestBuildCommitRows_UncommittedChangesGoOnTop(t *testing.T) {
	rows := buildCommitRows(sampleCommits(2), -1, true)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].commit.Hash != git.UncommittedHash {
		t.Errorf("row 0 = %q, want the uncommitted pseudo-commit", rows[0].commit.Hash)
	}
	if rows[0].commit.Message != "Uncommitted changes" {
		t.Errorf("row 0 message = %q", rows[0].commit.Message)
	}
}

func TestBuildCommitRows_SeparatorAboveTheForkPoint(t *testing.T) {
	rows := buildCommitRows(sampleCommits(4), 2, false)

	if len(rows) != 5 {
		t.Fatalf("expected 5 rows (4 commits + separator), got %d", len(rows))
	}
	if !rows[2].separator {
		t.Errorf("expected a separator at index 2, got %+v", rows[2])
	}
	if rows[3].commit.Hash != sampleCommits(4)[2].Hash {
		t.Error("the fork-point commit should sit directly below the separator")
	}
}

func TestBuildCommitRows_NoLeadingSeparator(t *testing.T) {
	// At its fork point, so there is nothing newer to divide off.
	rows := buildCommitRows(sampleCommits(3), 0, false)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows with no separator, got %d", len(rows))
	}
	if rows[0].separator {
		t.Error("a separator should not be the first row")
	}
}

func TestBuildCommitRows_SeparatorDividesUncommittedFromForkPoint(t *testing.T) {
	// HEAD is the fork point, but the uncommitted row above it still needs dividing.
	rows := buildCommitRows(sampleCommits(3), 0, true)

	if !rows[1].separator {
		t.Errorf("expected a separator below the uncommitted row, got %+v", rows[1])
	}
}

func TestBuildCommitRows_Empty(t *testing.T) {
	if rows := buildCommitRows(nil, -1, false); len(rows) != 0 {
		t.Errorf("expected no rows, got %d", len(rows))
	}
}

// ── Navigation ───────────────────────────────────────────────────────────────

func TestSelector_MoveUpAndDown(t *testing.T) {
	s := newSelector(10, 80, 5)

	s.move(1, 1)
	if s.cursor != 1 || s.anchor != 1 {
		t.Errorf("cursor/anchor = %d/%d, want 1/1", s.cursor, s.anchor)
	}
	s.move(3, 1)
	if s.cursor != 4 {
		t.Errorf("cursor = %d, want 4", s.cursor)
	}
	s.move(2, -1)
	if s.cursor != 2 {
		t.Errorf("cursor = %d, want 2", s.cursor)
	}
}

func TestSelector_MoveClampsAtTheEnds(t *testing.T) {
	s := newSelector(5, 80, 5)

	s.move(100, -1)
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.cursor)
	}
	s.move(100, 1)
	if s.cursor != 4 {
		t.Errorf("cursor = %d, want 4 (the last row)", s.cursor)
	}
}

func TestSelector_MoveSkipsTheSeparator(t *testing.T) {
	s := &selector{}
	s.setSize(80, 10)
	s.setRows(buildCommitRows(sampleCommits(4), 2, false))

	// Rows: [c0, c1, separator, c2, c3]. Moving down from c1 must land on c2.
	s.move(1, 1)
	s.move(1, 1)
	if s.rows[s.cursor].separator {
		t.Fatal("the cursor landed on the separator")
	}
	if s.cursor != 3 {
		t.Errorf("cursor = %d, want 3 (the row below the separator)", s.cursor)
	}
}

func TestSelector_MoveCollapsesARange(t *testing.T) {
	s := newSelector(10, 80, 5)
	s.extendRange(3, 1)

	if s.selectedCount() != 4 {
		t.Fatalf("expected a 4-commit range, got %d", s.selectedCount())
	}
	s.move(1, 1)
	if s.selectedCount() != 1 {
		t.Errorf("moving should collapse the range, got %d commits", s.selectedCount())
	}
}

func TestSelector_ExtendRange(t *testing.T) {
	s := newSelector(10, 80, 5)

	s.extendRange(2, 1)
	if s.anchor != 0 {
		t.Errorf("anchor = %d, want it to stay at 0", s.anchor)
	}
	if s.cursor != 2 {
		t.Errorf("cursor = %d, want 2", s.cursor)
	}
	if got := s.selectedCount(); got != 3 {
		t.Errorf("selectedCount = %d, want 3", got)
	}
}

func TestSelector_ExtendRangeUpwards(t *testing.T) {
	s := newSelector(10, 80, 5)
	s.move(5, 1)
	s.extendRange(2, -1)

	lo, hi := s.selectionRange()
	if lo != 3 || hi != 5 {
		t.Errorf("selection = [%d, %d], want [3, 5]", lo, hi)
	}
	if got := s.selectedCount(); got != 3 {
		t.Errorf("selectedCount = %d, want 3", got)
	}
}

func TestSelector_SelectedCountIgnoresTheSeparator(t *testing.T) {
	s := &selector{}
	s.setSize(80, 10)
	s.setRows(buildCommitRows(sampleCommits(4), 2, false))

	s.extendRange(10, 1)
	if got := s.selectedCount(); got != 4 {
		t.Errorf("selectedCount = %d, want 4 commits (the separator is not one)", got)
	}
}

func TestSelector_ScrollFollowsTheCursor(t *testing.T) {
	s := newSelector(20, 80, 5)

	s.move(7, 1)
	if s.cursor < s.offset || s.cursor >= s.offset+s.listHeight {
		t.Errorf("cursor %d is outside the visible window [%d, %d)",
			s.cursor, s.offset, s.offset+s.listHeight)
	}
	s.move(20, -1)
	if s.offset != 0 {
		t.Errorf("offset = %d, want 0 after returning to the top", s.offset)
	}
}

func TestSelector_ClampSelectedAfterAShorterRefresh(t *testing.T) {
	s := newSelector(20, 80, 5)
	s.move(15, 1)

	s.setRows(buildCommitRows(sampleCommits(3), -1, false))
	if s.cursor >= len(s.rows) {
		t.Errorf("cursor = %d, out of range for %d rows", s.cursor, len(s.rows))
	}
	if s.anchor != s.cursor {
		t.Errorf("a refresh should collapse the selection: cursor=%d anchor=%d", s.cursor, s.anchor)
	}
}

func TestSelector_ClampSelectedMovesOffASeparator(t *testing.T) {
	s := &selector{}
	s.setSize(80, 10)
	s.rows = buildCommitRows(sampleCommits(4), 2, false)
	s.cursor, s.anchor = 2, 2 // the separator
	s.clampSelected()

	if s.rows[s.cursor].separator {
		t.Error("clampSelected left the cursor on a separator")
	}
}

func TestSelector_EmptyListNavigatesSafely(t *testing.T) {
	s := &selector{}
	s.setSize(80, 5)
	s.setRows(nil)

	s.move(1, 1)
	s.extendRange(1, -1)
	if s.cursor != 0 || s.offset != 0 {
		t.Errorf("cursor/offset = %d/%d, want 0/0 for an empty list", s.cursor, s.offset)
	}
	if s.currentCommit() != nil {
		t.Error("expected no current commit")
	}
	if _, _, ok := s.selectedRange(); ok {
		t.Error("expected no selected range")
	}
}

// ── Selected range ───────────────────────────────────────────────────────────

func TestSelector_SelectedRangeForOneCommit(t *testing.T) {
	s := newSelector(5, 80, 5)
	s.move(2, 1)

	start, end, ok := s.selectedRange()
	if !ok {
		t.Fatal("expected a selection")
	}
	if start.Hash != end.Hash {
		t.Errorf("a single selection should have equal ends, got %q and %q", start.Hash, end.Hash)
	}
	if start.Hash != s.rows[2].commit.Hash {
		t.Errorf("range = %q, want the row under the cursor", start.Hash)
	}
}

func TestSelector_SelectedRangeIsOldestToNewest(t *testing.T) {
	s := newSelector(5, 80, 5)
	s.extendRange(2, 1)

	start, end, ok := s.selectedRange()
	if !ok {
		t.Fatal("expected a selection")
	}
	// Rows are newest first.
	if end.Hash != s.rows[0].commit.Hash {
		t.Errorf("end = %q, want the newest commit %q", end.Hash, s.rows[0].commit.Hash)
	}
	if start.Hash != s.rows[2].commit.Hash {
		t.Errorf("start = %q, want the oldest commit %q", start.Hash, s.rows[2].commit.Hash)
	}
}

func TestSelector_SelectedRangeSkipsTheSeparatorAtItsEdges(t *testing.T) {
	s := &selector{}
	s.setSize(80, 10)
	s.setRows(buildCommitRows(sampleCommits(4), 2, false))
	// Rows: [c0, c1, separator, c2, c3]. Select from c1 through the separator.
	s.move(1, 1)
	s.anchor, s.cursor = 1, 2

	start, end, ok := s.selectedRange()
	if !ok {
		t.Fatal("expected a selection")
	}
	if start.Hash == "" || end.Hash == "" {
		t.Error("range ends should be real commits, not the separator")
	}
}

// ── Rendering ────────────────────────────────────────────────────────────────

func TestSelector_RenderPanesDimensions(t *testing.T) {
	const width, listHeight = 100, 12
	s := newSelector(20, width, listHeight)

	lines := strings.Split(s.renderPanes(), "\n")
	if got := len(lines); got != listHeight+paneBorderRows {
		t.Errorf("panes are %d rows tall, want %d", got, listHeight+paneBorderRows)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("row %d is %d columns wide, want %d", i, w, width)
		}
	}
}

func TestSelector_RenderShowsCommitsWithShortHashes(t *testing.T) {
	s := newSelector(3, 100, 10)
	out := stripAnsi(s.renderPanes())

	for _, c := range sampleCommits(3) {
		if !strings.Contains(out, c.Hash[:4]) {
			t.Errorf("expected the short hash %q in the list", c.Hash[:4])
		}
		if !strings.Contains(out, c.Message) {
			t.Errorf("expected %q in the list", c.Message)
		}
	}
	if strings.Contains(out, sampleCommits(1)[0].Hash) {
		t.Error("the full hash should not be shown")
	}
}

func TestSelector_RenderTruncatesLongMessages(t *testing.T) {
	s := &selector{}
	s.setSize(60, 5)
	s.setRows([]commitRow{{commit: git.CommitEntry{
		Hash:    "abcdef0123456789",
		Message: strings.Repeat("very long commit message ", 10),
	}}})

	if !strings.Contains(stripAnsi(s.renderList()), "…") {
		t.Error("expected the message to be truncated with an ellipsis")
	}
}

func TestSelector_RenderShowsTheSeparator(t *testing.T) {
	s := &selector{}
	s.setSize(100, 10)
	s.setRows(buildCommitRows(sampleCommits(4), 2, false))

	if !strings.Contains(stripAnsi(s.renderList()), "───") {
		t.Error("expected a horizontal separator rule in the list")
	}
}

func TestSelector_RenderHighlightsCursorAndRange(t *testing.T) {
	s := newSelector(5, 100, 5)
	single := s.renderList()

	s.extendRange(2, 1)
	ranged := s.renderList()
	if ranged == single {
		t.Fatal("extending the range should change the rendering")
	}

	// Styled differently so the user can see which end moves.
	lines := strings.Split(ranged, "\n")
	cursorRow, rangeRow := lines[1+s.cursor], lines[1+s.anchor]
	if cursorRow == rangeRow {
		t.Error("the cursor row should be styled differently from the rest of the range")
	}
}

func TestSelector_RenderEmptyList(t *testing.T) {
	s := &selector{}
	s.setSize(100, 8)
	s.setRows(nil)

	if !strings.Contains(stripAnsi(s.renderList()), "No commits") {
		t.Error("expected the empty-state message")
	}
}

func TestSelector_RenderShowsScrollbarWhenTheListOverflows(t *testing.T) {
	if !strings.Contains(newSelector(50, 100, 5).renderList(), "█") {
		t.Error("expected a scrollbar thumb for a list that overflows")
	}
	if strings.Contains(newSelector(3, 100, 20).renderList(), "█") {
		t.Error("expected no scrollbar thumb when the list fits")
	}
}

func TestSelector_StatPaneEmptyState(t *testing.T) {
	s := newSelector(3, 100, 8)

	if !strings.Contains(stripAnsi(s.renderStat()), "(no preview)") {
		t.Error("expected the empty preview message before the stat loads")
	}
}

func TestSelector_StatPaneShowsOutput(t *testing.T) {
	s := newSelector(3, 100, 8)
	s.statOutput = "commit subject\n\n one.txt | 3 ++-\n 1 file changed, 2 insertions(+), 1 deletion(-)"

	out := stripAnsi(s.renderStat())
	for _, want := range []string{"commit subject", "one.txt", "1 file changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the preview, got %q", want, out)
		}
	}
}

func TestRenderStatContent_ColourisesBars(t *testing.T) {
	out := renderStatContent("subject\n one.txt | 3 ++-\n", 60)

	if !strings.Contains(out, "\x1b") {
		t.Fatal("expected styled output")
	}
	// Differently coloured runs cannot share one escape sequence.
	if strings.Count(out, "\x1b[") < 4 {
		t.Errorf("expected the +/- runs to be styled separately, got %q", out)
	}
}

func TestRenderStatContent_TrimsIndentAndWraps(t *testing.T) {
	long := strings.Repeat("x", 50)
	out := renderStatContent("subject\n   "+long, 20)

	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 20 {
			t.Errorf("line is %d columns wide, want at most 20: %q", w, stripAnsi(line))
		}
	}
	if strings.Contains(stripAnsi(out), "   "+long[:10]) {
		t.Error("git's leading indent should be trimmed")
	}
}

func TestIsStatLine(t *testing.T) {
	cases := map[string]bool{
		" one.txt | 3 ++-":                                true,
		" img/logo.png | Bin 0 -> 512 bytes":              true,
		" 1 file changed, 2 insertions(+), 1 deletion(-)": true,
		" 3 files changed":                                true,
		"commit subject":                                  false,
		"":                                                false,
	}

	for line, want := range cases {
		if got := isStatLine(line); got != want {
			t.Errorf("isStatLine(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestWrapLine(t *testing.T) {
	if got := wrapLine("short", 10); len(got) != 1 || got[0] != "short" {
		t.Errorf("wrapLine of a short line = %v, want [short]", got)
	}
	got := wrapLine(strings.Repeat("x", 25), 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d (%v)", len(got), got)
	}
	if len(got[0]) != 10 || len(got[2]) != 5 {
		t.Errorf("unexpected chunk sizes: %v", got)
	}
	if got := wrapLine("anything", 0); len(got) != 1 {
		t.Errorf("a non-positive width should not wrap, got %v", got)
	}
}

func TestSelector_PaneWidthsSplitTheScreen(t *testing.T) {
	s := newSelector(3, 99, 5)

	if got := s.leftPaneWidth(); got != 33 {
		t.Errorf("leftPaneWidth = %d, want a third of 99", got)
	}
	if got := s.rightPaneWidth(); got != 66 {
		t.Errorf("rightPaneWidth = %d, want the remainder", got)
	}
	if s.leftPaneWidth()+s.rightPaneWidth() != 99 {
		t.Error("the panes should together fill the screen")
	}
}

func TestSelector_PaneWidthsHaveAFloor(t *testing.T) {
	s := newSelector(3, 4, 5)

	if s.leftPaneWidth() < 10 || s.rightPaneWidth() < 10 {
		t.Errorf("panes should not collapse below their floor: %d and %d",
			s.leftPaneWidth(), s.rightPaneWidth())
	}
}
