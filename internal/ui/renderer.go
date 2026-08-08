package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

// plainStyle applies no attributes. It lets unstyled content lines go through
// padToWidth for wrapping without picking up a background colour.
var plainStyle = lipgloss.NewStyle()

// diffLineKind classifies each rendered line for the line-select feature.
type diffLineKind int

const (
	diffLineNonSelectable diffLineKind = iota // blank, filename, "Deleted", hunk header, etc.
	diffLineHunkContent                       // context, added, or removed line inside a hunk
)

// diffLineMeta records per-line metadata produced during rendering. The viewer
// uses this to determine which lines are selectable and which file they belong to.
type diffLineMeta struct {
	kind      diffLineKind
	fileIndex int // index into the files slice (-1 for non-selectable)
	lineNum   int // new-file line number for selectable lines; 0 otherwise
}

// renderedDiff holds the formatted diff output together with the line offsets
// where each file section starts. Computing offsets during rendering avoids
// fragile re-parsing of the output string.
type renderedDiff struct {
	text       string         // the formatted diff output
	fileStarts []int          // line offset where each file's blank-separator line begins
	lineMeta   []diffLineMeta // per-line metadata for the entire rendered output
}

// renderContext bundles per-render state that would otherwise be threaded
// through renderDiff -> renderHunk as individual parameters.
type renderContext struct {
	paneWidth int
	meta      []diffLineMeta // per-line metadata accumulated during rendering
	fileIndex int            // current file index during rendering
}

// appendMeta appends n metadata entries for the current file with the given
// kind and line number.
func (rc *renderContext) appendMeta(n int, kind diffLineKind, lineNum int) {
	for range n {
		rc.meta = append(rc.meta, diffLineMeta{kind: kind, fileIndex: rc.fileIndex, lineNum: lineNum})
	}
}

// renderDiff formats parsed diff files for display and returns the text along
// with per-file start offsets and per-line metadata. Each file begins with a
// blank line (including the first). paneWidth controls the full-width
// background padding for hunk headers and added/removed lines. collapsed
// controls per-file collapse state; nil means all expanded.
func renderDiff(files []diff.File, paneWidth int, collapsed []bool) renderedDiff {
	if len(files) == 0 {
		return renderedDiff{}
	}

	var sb strings.Builder
	lineCount := 0 // tracks the current line number in the output
	fileStarts := make([]int, 0, len(files))
	rc := &renderContext{paneWidth: paneWidth}

	for i, f := range files {
		rc.fileIndex = i
		isCollapsed := len(collapsed) > i && collapsed[i]
		fileStarts = append(fileStarts, lineCount)
		sb.WriteString("\n") // blank line before each file (including the first)
		rc.appendMeta(1, diffLineNonSelectable, 0)
		lineCount++

		indicator := "▽ "
		if isCollapsed {
			indicator = "▶ "
		}
		sb.WriteString(theme.FileHeaderStyle.Render(indicator + f.Name + ":"))
		sb.WriteString("\n")
		rc.appendMeta(1, diffLineNonSelectable, 0)
		lineCount++

		if isCollapsed {
			continue
		}

		switch f.Type {
		case diff.Binary:
			sb.WriteString("  ")
			sb.WriteString(theme.EmptyStateStyle.Render("(binary stuff)"))
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
		case diff.Delete:
			sb.WriteString("  ")
			sb.WriteString(theme.DeletedMsgStyle.Render("Deleted"))
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
		case diff.Rename:
			sb.WriteString("  ")
			sb.WriteString(theme.RenamedMsgStyle.Render("Renamed to "))
			sb.WriteString(f.RenameTo)
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, rc)
			}
		default:
			// Normal and New files: render hunks.
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, rc)
			}
		}
	}

	// Trim the trailing newline so callers get clean output.
	return renderedDiff{
		text:       strings.TrimRight(sb.String(), "\n"),
		fileStarts: fileStarts,
		lineMeta:   rc.meta,
	}
}

// renderHunk renders a single hunk: the @@ header followed by content lines.
// It returns the number of lines written and appends per-line metadata to rc.
func renderHunk(sb *strings.Builder, h diff.Hunk, rc *renderContext) int {
	lines := 0

	// Hunk header.
	header := "@@"
	if h.Context != "" {
		header += " " + h.Context
	}
	styled, n := padToWidth(theme.HunkHeaderStyle, header, rc.paneWidth)
	sb.WriteString(styled)
	sb.WriteString("\n")
	rc.appendMeta(n, diffLineNonSelectable, 0)
	lines += n

	// Determine the line-number column width from the max line number in this hunk.
	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		text := expandTabs(line.Content)
		switch line.Type {
		case diff.LineRemoved:
			// Blank line-number space, then content with a red background.
			prefix := strings.Repeat(" ", numWidth) + " "
			styled, n := padToWidth(theme.RemovedStyle, prefix+text, rc.paneWidth)
			sb.WriteString(styled)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
		case diff.LineAdded:
			// Line number on the left, then content with a green background.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			styled, n := padToWidth(theme.AddedStyle, prefix+text, rc.paneWidth)
			sb.WriteString(styled)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
			lineNum++
		case diff.LineContext:
			// Line number on the left, plain text (no background). Context
			// lines are wrapped here rather than left for lipgloss to wrap at
			// render time: the viewer treats one line of this text as one
			// screen row when scrolling and when tracking per-line metadata, so
			// a line that silently became two rows would push the bottom of the
			// pane off-screen and misplace the line-select cursor.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			wrapped, n := padToWidth(plainStyle, prefix+text, rc.paneWidth)
			sb.WriteString(strings.TrimRight(wrapped, " "))
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
			lineNum++
		}
		sb.WriteString("\n")
	}
	return lines
}

// padToWidth pads text with spaces to fill paneWidth, then applies the style.
// If the text is wider than paneWidth, it wraps into multiple lines so that
// background colours extend to the full pane width on every visual line.
// Returns the styled text and the number of visual lines it occupies.
func padToWidth(style lipgloss.Style, text string, paneWidth int) (string, int) {
	if paneWidth <= 0 {
		return style.Render(text), 1
	}
	textWidth := lipgloss.Width(text)
	if textWidth <= paneWidth {
		if textWidth < paneWidth {
			text += strings.Repeat(" ", paneWidth-textWidth)
		}
		return style.Render(text), 1
	}
	// Text is wider than the pane: wrap into multiple lines, each padded
	// to paneWidth, so the background colour fills every visual line.
	var parts []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := 0
		for end < len(runes) {
			if lipgloss.Width(string(runes[:end+1])) > paneWidth {
				break
			}
			end++
		}
		if end == 0 {
			end = 1 // at least one rune per line
		}
		chunk := string(runes[:end])
		runes = runes[end:]
		if w := lipgloss.Width(chunk); w < paneWidth {
			chunk += strings.Repeat(" ", paneWidth-w)
		}
		parts = append(parts, style.Render(chunk))
	}
	return strings.Join(parts, "\n"), len(parts)
}

// digitCount returns the number of decimal digits in n.
func digitCount(n int) int {
	if n <= 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

// expandTabs replaces tab characters with spaces. Terminals render tabs at
// variable widths, but lipgloss.Width counts each tab as 1 column, so styled
// lines padded to the pane width overshoot and wrap. Using fixed 4-space tabs
// keeps the width calculation and the terminal rendering in agreement.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// fileNamesFromDiff returns ordered file names from the parsed diff.
func fileNamesFromDiff(files []diff.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}
