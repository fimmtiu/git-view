package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

// Lets unstyled lines share padToWidth's wrapping without gaining a background.
var plainStyle = lipgloss.NewStyle()

type diffLineKind int

const (
	diffLineNonSelectable diffLineKind = iota
	diffLineHunkContent
)

type diffLineMeta struct {
	kind      diffLineKind
	fileIndex int
	lineNum   int // new-file line number; 0 for non-selectable lines
}

// Offsets are computed during rendering to avoid re-parsing the output string.
type renderedDiff struct {
	text       string
	fileStarts []int // where each file's blank separator line begins
	lineMeta   []diffLineMeta
}

type renderContext struct {
	paneWidth int
	meta      []diffLineMeta
	fileIndex int
}

func (rc *renderContext) appendMeta(n int, kind diffLineKind, lineNum int) {
	for range n {
		rc.meta = append(rc.meta, diffLineMeta{kind: kind, fileIndex: rc.fileIndex, lineNum: lineNum})
	}
}

// renderDiff formats files for display, padding backgrounds out to paneWidth.
// collapsed is per-file; nil means all expanded.
func renderDiff(files []diff.File, paneWidth int, collapsed []bool) renderedDiff {
	if len(files) == 0 {
		return renderedDiff{}
	}

	var sb strings.Builder
	lineCount := 0
	fileStarts := make([]int, 0, len(files))
	rc := &renderContext{paneWidth: paneWidth}

	for i, f := range files {
		rc.fileIndex = i
		isCollapsed := len(collapsed) > i && collapsed[i]
		fileStarts = append(fileStarts, lineCount)
		sb.WriteString("\n") // separator before every file, the first included
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
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, rc)
			}
		}
	}

	return renderedDiff{
		text:       strings.TrimRight(sb.String(), "\n"),
		fileStarts: fileStarts,
		lineMeta:   rc.meta,
	}
}

// renderHunk writes the @@ header and content lines, returning the number of
// visual lines written.
func renderHunk(sb *strings.Builder, h diff.Hunk, rc *renderContext) int {
	lines := 0

	header := "@@"
	if h.Context != "" {
		header += " " + h.Context
	}
	styled, n := padToWidth(theme.HunkHeaderStyle, header, rc.paneWidth)
	sb.WriteString(styled)
	sb.WriteString("\n")
	rc.appendMeta(n, diffLineNonSelectable, 0)
	lines += n

	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		text := expandTabs(line.Content)
		switch line.Type {
		case diff.LineRemoved:
			// Removed lines take no new-file line number, so the gutter is blank.
			prefix := strings.Repeat(" ", numWidth) + " "
			styled, n := padToWidth(theme.RemovedStyle, prefix+text, rc.paneWidth)
			sb.WriteString(styled)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
		case diff.LineAdded:
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			styled, n := padToWidth(theme.AddedStyle, prefix+text, rc.paneWidth)
			sb.WriteString(styled)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
			lineNum++
		case diff.LineContext:
			// Wrapped here rather than left to lipgloss at render time: the viewer
			// counts one line of this text as one screen row, so a line that
			// silently became two would push the pane's bottom off-screen and
			// misplace the line-select cursor.
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

// padToWidth pads and wraps text so the style's background fills every visual
// line, returning the styled text and how many lines it occupies.
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
			end = 1 // never loop forever on a rune wider than the pane
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

// expandTabs keeps width maths honest: lipgloss.Width counts a tab as one
// column, but terminals render it wider, so padded lines would overshoot.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

func fileNamesFromDiff(files []diff.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}
