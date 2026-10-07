package ui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

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
	matchLines []int // ascending; lines holding a search match
}

type renderContext struct {
	paneWidth  int
	meta       []diffLineMeta
	fileIndex  int
	searchTerm string
	matchLines []int
}

func (rc *renderContext) appendMeta(n int, kind diffLineKind, lineNum int) {
	for range n {
		rc.meta = append(rc.meta, diffLineMeta{kind: kind, fileIndex: rc.fileIndex, lineNum: lineNum})
	}
}

// appendContentMeta takes one entry per visual line the content occupies, and
// notes which of them a search match landed on.
func (rc *renderContext) appendContentMeta(matched []bool, lineNum int) {
	for _, hit := range matched {
		if hit {
			rc.matchLines = append(rc.matchLines, len(rc.meta))
		}
		rc.meta = append(rc.meta, diffLineMeta{
			kind: diffLineHunkContent, fileIndex: rc.fileIndex, lineNum: lineNum,
		})
	}
}

// renderDiff formats files for display, padding backgrounds out to paneWidth.
// collapsed is per-file; nil means all expanded. searchTerm, when set, is lit up
// wherever it occurs in the changed text — never in the headers, which are not
// part of the files.
func renderDiff(files []diff.File, paneWidth int, collapsed []bool, searchTerm string) renderedDiff {
	if len(files) == 0 {
		return renderedDiff{}
	}

	var sb strings.Builder
	lineCount := 0
	fileStarts := make([]int, 0, len(files))
	rc := &renderContext{paneWidth: paneWidth, searchTerm: searchTerm}

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
		sb.WriteString(theme.FileHeaderStyle.Render(indicator + sanitize(f.Name) + ":"))
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
			sb.WriteString(sanitize(f.RenameTo))
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
		matchLines: rc.matchLines,
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
	styled, n, _ := padToWidth(theme.HunkHeaderStyle, header, rc.paneWidth, nil)
	sb.WriteString(styled)
	sb.WriteString("\n")
	rc.appendMeta(n, diffLineNonSelectable, 0)
	lines += n

	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		text := expandTabs(sanitize(line.Content))
		switch line.Type {
		case diff.LineRemoved:
			// Removed lines take no new-file line number, so the gutter is blank.
			prefix := strings.Repeat(" ", numWidth) + " "
			styled, n, matched := padToWidth(theme.RemovedStyle, prefix+text, rc.paneWidth, rc.spans(prefix, text))
			sb.WriteString(styled)
			rc.appendContentMeta(matched, lineNum)
			lines += n
		case diff.LineAdded:
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			styled, n, matched := padToWidth(theme.AddedStyle, prefix+text, rc.paneWidth, rc.spans(prefix, text))
			sb.WriteString(styled)
			rc.appendContentMeta(matched, lineNum)
			lines += n
			lineNum++
		case diff.LineContext:
			// Wrapped here, not by lipgloss at render time, because the viewer
			// counts each line of this text as one screen row.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			wrapped, n, matched := padToWidth(plainStyle, prefix+text, rc.paneWidth, rc.spans(prefix, text))
			sb.WriteString(strings.TrimRight(wrapped, " "))
			rc.appendContentMeta(matched, lineNum)
			lines += n
			lineNum++
		}
		sb.WriteString("\n")
	}
	return lines
}

// spans locates the search term in text, offset past the line-number gutter so
// the gutter itself never matches.
func (rc *renderContext) spans(prefix, text string) [][2]int {
	if rc.searchTerm == "" {
		return nil
	}
	return offsetSpans(matchSpans(text, rc.searchTerm), utf8.RuneCountInString(prefix))
}

// matchSpans returns the non-overlapping [start, end) rune ranges where term
// occurs in text, ascending. Matching is exact, so case counts.
func matchSpans(text, term string) [][2]int {
	if term == "" {
		return nil
	}
	termLen := utf8.RuneCountInString(term)

	var spans [][2]int
	runePos, bytePos := 0, 0
	for bytePos < len(text) {
		i := strings.Index(text[bytePos:], term)
		if i < 0 {
			break
		}
		runePos += utf8.RuneCountInString(text[bytePos : bytePos+i])
		spans = append(spans, [2]int{runePos, runePos + termLen})
		runePos += termLen
		bytePos += i + len(term)
	}
	return spans
}

func offsetSpans(spans [][2]int, by int) [][2]int {
	for i := range spans {
		spans[i][0] += by
		spans[i][1] += by
	}
	return spans
}

// padToWidth pads and wraps text so the style's background fills every visual
// line. It returns the styled text, its line count, and which lines a span
// landed on. spans are search-match rune ranges over text, and may be nil.
func padToWidth(style lipgloss.Style, text string, paneWidth int, spans [][2]int) (string, int, []bool) {
	if paneWidth <= 0 {
		styled, hit := renderSpans(style, text, spans, 0)
		return styled, 1, []bool{hit}
	}
	textWidth := lipgloss.Width(text)
	if textWidth <= paneWidth {
		if textWidth < paneWidth {
			text += strings.Repeat(" ", paneWidth-textWidth)
		}
		styled, hit := renderSpans(style, text, spans, 0)
		return styled, 1, []bool{hit}
	}
	var parts []string
	var matched []bool
	runes := []rune(text)
	// Where this chunk starts within text, so the spans stay aligned across the
	// wrap: a match split by one is lit on both lines.
	offset := 0
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
		styled, hit := renderSpans(style, chunk, spans, offset)
		parts = append(parts, styled)
		matched = append(matched, hit)
		offset += end
	}
	return strings.Join(parts, "\n"), len(parts), matched
}

// renderSpans styles chunk, laying the match highlight over the runs of it that
// fall inside spans. offset is where chunk begins within the line the spans were
// measured against. It reports whether any of them showed up here.
func renderSpans(style lipgloss.Style, chunk string, spans [][2]int, offset int) (string, bool) {
	if len(spans) == 0 {
		return style.Render(chunk), false
	}

	runes := []rune(chunk)
	end := offset + len(runes)
	var sb strings.Builder
	hit := false
	pos := offset
	for _, span := range spans {
		if span[1] <= pos {
			continue
		}
		if span[0] >= end {
			break
		}
		lo, hi := max(span[0], pos), min(span[1], end)
		if lo > pos {
			sb.WriteString(style.Render(string(runes[pos-offset : lo-offset])))
		}
		sb.WriteString(theme.SearchMatchStyle.Render(string(runes[lo-offset : hi-offset])))
		hit = true
		pos = hi
	}
	if pos < end {
		sb.WriteString(style.Render(string(runes[pos-offset:])))
	}
	return sb.String(), hit
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

// Escape sequences, and the other control characters that are not text.
var controlCharRe = regexp.MustCompile(
	`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b.|[\x00-\x08\x0b-\x1f\x7f]`)

// sanitize strips control characters from text that came from git. Diffs and
// commit messages can contain escape sequences that would otherwise control the
// terminal, for example by showing the cursor.
func sanitize(s string) string {
	return controlCharRe.ReplaceAllString(s, "")
}

func fileNamesFromDiff(files []diff.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = sanitize(f.Name)
	}
	return names
}
