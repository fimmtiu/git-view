package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

func TestRenderDiff_NormalFile(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func main()",
					NewStart: 10,
					NewCount: 4,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "fmt.Println(\"hello\")"},
						{Type: diff.LineRemoved, Content: "fmt.Println(\"old\")"},
						{Type: diff.LineAdded, Content: "fmt.Println(\"new\")"},
						{Type: diff.LineContext, Content: "fmt.Println(\"end\")"},
					},
				},
			},
		},
	}

	lines := strings.Split(renderDiff(files, 60, nil).text, "\n")

	if lines[0] != "" {
		t.Errorf("expected blank separator line first, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "main.go") {
		t.Errorf("expected filename on line 1, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "@@") || !strings.Contains(lines[2], "func main()") {
		t.Errorf("expected hunk header with context on line 2, got %q", lines[2])
	}
	if len(lines) != 7 {
		t.Fatalf("expected 7 lines (blank, name, header, 4 content), got %d", len(lines))
	}
}

func TestRenderDiff_LineNumbers(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 10,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "context"},
						{Type: diff.LineRemoved, Content: "removed"},
						{Type: diff.LineAdded, Content: "added"},
					},
				},
			},
		},
	}

	lines := strings.Split(renderDiff(files, 60, nil).text, "\n")
	// lines[0] blank, [1] filename, [2] hunk header, [3..] content.
	plain := func(i int) string { return stripAnsi(lines[i]) }

	if !strings.HasPrefix(plain(3), "10 ") {
		t.Errorf("context line should start with its line number, got %q", plain(3))
	}
	if !strings.HasPrefix(plain(4), "   ") {
		t.Errorf("removed line should have a blank line-number column, got %q", plain(4))
	}
	if strings.Contains(plain(4), "11") {
		t.Errorf("removed line should not consume a new-file line number, got %q", plain(4))
	}
	if !strings.HasPrefix(plain(5), "11 ") {
		t.Errorf("added line should start with line number 11, got %q", plain(5))
	}
}

func TestRenderDiff_LineNumberColumnWidth(t *testing.T) {
	files := []diff.File{
		{
			Name: "big.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 998,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "a"},
						{Type: diff.LineAdded, Content: "b"},
					},
				},
			},
		},
	}

	lines := strings.Split(renderDiff(files, 60, nil).text, "\n")
	// Max line number is 998+3 = 1001, so four columns.
	if got := stripAnsi(lines[3]); !strings.HasPrefix(got, " 998 ") {
		t.Errorf("expected four-column line number, got %q", got)
	}
}

func TestRenderDiff_NoPlusMinusPrefixes(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "old code"},
						{Type: diff.LineAdded, Content: "new code"},
					},
				},
			},
		},
	}

	for _, line := range strings.Split(renderDiff(files, 60, nil).text, "\n") {
		trimmed := strings.TrimSpace(stripAnsi(line))
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			t.Errorf("content line should not keep its +/- prefix: %q", trimmed)
		}
	}
}

func TestRenderDiff_BackgroundsSpanPaneWidth(t *testing.T) {
	files := []diff.File{
		{
			Name: "pad.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineRemoved, Content: "old"},
						{Type: diff.LineAdded, Content: "new"},
					},
				},
			},
		},
	}

	const paneWidth = 40
	lines := strings.Split(renderDiff(files, paneWidth, nil).text, "\n")
	// Hunk header (2), removed (3), added (4) all carry a background and must
	// fill the pane so the colour does not stop at the end of the text.
	for _, i := range []int{2, 3, 4} {
		if got := lipgloss.Width(lines[i]); got != paneWidth {
			t.Errorf("line %d width = %d, want %d (%q)", i, got, paneWidth, stripAnsi(lines[i]))
		}
	}
}

func TestRenderDiff_WrapsLongLinesToFullWidth(t *testing.T) {
	files := []diff.File{
		{
			Name: "long.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines:    []diff.Line{{Type: diff.LineAdded, Content: strings.Repeat("x", 50)}},
				},
			},
		},
	}

	const paneWidth = 20
	rd := renderDiff(files, paneWidth, nil)
	lines := strings.Split(rd.text, "\n")

	// The single added line wraps across several visual lines, each padded.
	wrapped := lines[3:]
	if len(wrapped) < 3 {
		t.Fatalf("expected the long line to wrap, got %d visual lines", len(wrapped))
	}
	for i, l := range wrapped {
		if got := lipgloss.Width(l); got != paneWidth {
			t.Errorf("wrapped line %d width = %d, want %d", i, got, paneWidth)
		}
	}
	// Metadata must count every visual line, or the line-select cursor drifts.
	if len(rd.lineMeta) != len(lines) {
		t.Errorf("lineMeta has %d entries, rendered text has %d lines", len(rd.lineMeta), len(lines))
	}
}

// TestRenderDiff_EveryLineFitsThePane pins the invariant the viewer depends on:
// one line of rendered text is exactly one screen row. If a line were left
// wider than the pane, lipgloss would wrap it at render time, so the pane would
// show fewer logical lines than it accounts for — pushing content off the bottom
// and misplacing the line-select cursor.
func TestRenderDiff_EveryLineFitsThePane(t *testing.T) {
	long := strings.Repeat("x", 200)
	files := []diff.File{
		{
			Name: "long.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  strings.Repeat("ctx ", 40),
					NewStart: 1,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: long},
						{Type: diff.LineAdded, Content: long},
						{Type: diff.LineRemoved, Content: long},
					},
				},
			},
		},
	}

	for _, paneWidth := range []int{20, 40, 80} {
		rd := renderDiff(files, paneWidth, nil)
		lines := strings.Split(rd.text, "\n")
		for i, line := range lines {
			if w := lipgloss.Width(line); w > paneWidth {
				t.Errorf("pane %d: line %d is %d columns wide: %q", paneWidth, i, w, stripAnsi(line))
			}
		}
		if len(rd.lineMeta) != len(lines) {
			t.Errorf("pane %d: lineMeta has %d entries, text has %d lines",
				paneWidth, len(rd.lineMeta), len(lines))
		}
	}
}

func TestRenderDiff_WrappedContextLineHasNoBackground(t *testing.T) {
	files := []diff.File{
		{
			Name: "long.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines:    []diff.Line{{Type: diff.LineContext, Content: strings.Repeat("x", 60)}},
				},
			},
		},
	}

	// Context lines go through the same wrapping path as added/removed lines,
	// but must not pick up a background colour along the way.
	for _, line := range strings.Split(renderDiff(files, 20, nil).text, "\n")[3:] {
		if strings.Contains(line, "\x1b") {
			t.Errorf("context line should carry no styling, got %q", line)
		}
	}
}

func TestRenderDiff_BinaryFile(t *testing.T) {
	files := []diff.File{{Name: "bin/fooble", Type: diff.Binary}}

	text := renderDiff(files, 60, nil).text
	if !strings.Contains(text, "bin/fooble") {
		t.Errorf("expected filename, got %q", text)
	}
	if !strings.Contains(text, "(binary stuff)") {
		t.Errorf("expected binary message, got %q", text)
	}
}

func TestRenderDiff_DeletedFile(t *testing.T) {
	files := []diff.File{{Name: "gone.go", Type: diff.Delete}}

	text := renderDiff(files, 60, nil).text
	if !strings.Contains(text, "gone.go") || !strings.Contains(text, "Deleted") {
		t.Errorf("expected filename and Deleted message, got %q", text)
	}
}

func TestRenderDiff_RenamedFile(t *testing.T) {
	files := []diff.File{{Name: "old.go", Type: diff.Rename, RenameTo: "new.go"}}

	text := stripAnsi(renderDiff(files, 60, nil).text)
	if !strings.Contains(text, "Renamed to new.go") {
		t.Errorf("expected rename message, got %q", text)
	}
}

func TestRenderDiff_RenamedFileKeepsHunks(t *testing.T) {
	files := []diff.File{
		{
			Name:     "old.go",
			Type:     diff.Rename,
			RenameTo: "new.go",
			Hunks: []diff.Hunk{
				{
					NewStart: 1,
					NewCount: 1,
					Lines:    []diff.Line{{Type: diff.LineAdded, Content: "changed"}},
				},
			},
		},
	}

	text := stripAnsi(renderDiff(files, 60, nil).text)
	if !strings.Contains(text, "Renamed to new.go") {
		t.Errorf("expected rename message, got %q", text)
	}
	if !strings.Contains(text, "changed") {
		t.Errorf("expected hunk content for a rename-with-edits, got %q", text)
	}
}

func TestRenderDiff_HunkHeaderNoContext(t *testing.T) {
	files := []diff.File{
		{
			Name:  "main.go",
			Type:  diff.Normal,
			Hunks: []diff.Hunk{{NewStart: 1, NewCount: 1, Lines: []diff.Line{{Type: diff.LineAdded, Content: "x"}}}},
		},
	}

	header := strings.TrimSpace(stripAnsi(strings.Split(renderDiff(files, 60, nil).text, "\n")[2]))
	if header != "@@" {
		t.Errorf("expected a bare @@ header, got %q", header)
	}
}

func TestRenderDiff_HunkHeaderStripsLineRanges(t *testing.T) {
	files := []diff.File{
		{
			Name: "main.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func handleKey()",
					OldStart: 244, OldCount: 8,
					NewStart: 244, NewCount: 10,
					Lines: []diff.Line{{Type: diff.LineAdded, Content: "x"}},
				},
			},
		},
	}

	header := stripAnsi(strings.Split(renderDiff(files, 60, nil).text, "\n")[2])
	if !strings.Contains(header, "@@ func handleKey()") {
		t.Errorf("expected @@ plus context, got %q", header)
	}
	if strings.Contains(header, "244") {
		t.Errorf("line ranges should be stripped from the header, got %q", header)
	}
}

func TestRenderDiff_FileStartsAndBlankSeparators(t *testing.T) {
	rd := renderDiff(sampleFiles(), 60, nil)
	lines := strings.Split(rd.text, "\n")

	if len(rd.fileStarts) != 2 {
		t.Fatalf("expected 2 file starts, got %d", len(rd.fileStarts))
	}
	for i, start := range rd.fileStarts {
		if lines[start] != "" {
			t.Errorf("file %d should start at a blank separator line, got %q", i, lines[start])
		}
		if !strings.Contains(lines[start+1], sampleFiles()[i].Name) {
			t.Errorf("file %d header should follow its separator, got %q", i, lines[start+1])
		}
	}
}

func TestRenderDiff_Collapsed(t *testing.T) {
	files := sampleFiles()
	rd := renderDiff(files, 60, []bool{true, false})
	text := stripAnsi(rd.text)

	if strings.Contains(text, "fmt.Println(\"hello\")") {
		t.Error("collapsed file should not render its hunk content")
	}
	if !strings.Contains(text, "package db") {
		t.Error("expanded file should still render its hunk content")
	}
	if !strings.Contains(text, "▶ internal/ui/app.go:") {
		t.Errorf("collapsed file should use the ▶ indicator, got %q", text)
	}
	if !strings.Contains(text, "▽ internal/db/project_context_test.go:") {
		t.Errorf("expanded file should use the ▽ indicator, got %q", text)
	}
}

func TestRenderDiff_CollapsedHasNoSelectableLines(t *testing.T) {
	rd := renderDiff(sampleFiles(), 60, []bool{true, true})
	for i, meta := range rd.lineMeta {
		if meta.kind == diffLineHunkContent {
			t.Fatalf("line %d is selectable in a fully collapsed diff", i)
		}
	}
}

func TestRenderDiff_EmptyInput(t *testing.T) {
	rd := renderDiff(nil, 60, nil)
	if rd.text != "" || rd.fileStarts != nil || rd.lineMeta != nil {
		t.Errorf("expected a zero renderedDiff for no files, got %+v", rd)
	}
}

func TestRenderDiff_LineMetaMatchesLineCount(t *testing.T) {
	rd := renderDiff(sampleFiles(), 60, nil)
	if got, want := len(rd.lineMeta), len(strings.Split(rd.text, "\n")); got != want {
		t.Errorf("lineMeta entries = %d, rendered lines = %d", got, want)
	}
}

func TestRenderDiff_LineMetaMarksOnlyHunkContentSelectable(t *testing.T) {
	rd := renderDiff(sampleFiles(), 60, nil)
	lines := strings.Split(rd.text, "\n")

	for i, meta := range rd.lineMeta {
		plain := stripAnsi(lines[i])
		isChrome := plain == "" || strings.Contains(plain, "▽ ") || strings.HasPrefix(strings.TrimSpace(plain), "@@")
		if isChrome && meta.kind == diffLineHunkContent {
			t.Errorf("line %d (%q) should not be selectable", i, plain)
		}
		if !isChrome && meta.kind != diffLineHunkContent {
			t.Errorf("line %d (%q) should be selectable", i, plain)
		}
	}
}

func TestRenderDiff_LineMetaFileIndex(t *testing.T) {
	files := sampleFiles()
	rd := renderDiff(files, 60, nil)

	for i, meta := range rd.lineMeta {
		want := 0
		if i >= rd.fileStarts[1] {
			want = 1
		}
		if meta.fileIndex != want {
			t.Errorf("line %d fileIndex = %d, want %d", i, meta.fileIndex, want)
		}
	}
}

func TestExpandTabs(t *testing.T) {
	if got := expandTabs("a\tb"); got != "a    b" {
		t.Errorf("expandTabs = %q, want %q", got, "a    b")
	}
}

func TestDigitCount(t *testing.T) {
	cases := map[int]int{0: 1, -5: 1, 1: 1, 9: 1, 10: 2, 999: 3, 1000: 4}
	for in, want := range cases {
		if got := digitCount(in); got != want {
			t.Errorf("digitCount(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestFileNamesFromDiff(t *testing.T) {
	names := fileNamesFromDiff(sampleFiles())
	want := []string{"internal/ui/app.go", "internal/db/project_context_test.go"}
	if len(names) != len(want) {
		t.Fatalf("got %d names, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("name %d = %q, want %q", i, names[i], want[i])
		}
	}
	if got := fileNamesFromDiff(nil); len(got) != 0 {
		t.Errorf("expected no names for no files, got %v", got)
	}
}
