package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const hintSeparator = "  "

func buildHint(pairs ...string) string {
	var sb strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			sb.WriteString(theme.HintDescStyle.Render(hintSeparator))
		}
		sb.WriteString(hintPair(pairs[i], pairs[i+1]))
	}
	return sb.String()
}

// buildHintFit elides pairs from the end, always keeping the last one — the quit
// binding — so the help text stays one row tall however narrow the terminal is.
func buildHintFit(width int, pairs ...string) string {
	full := buildHint(pairs...)
	if width <= 0 || lipgloss.Width(full) <= width || len(pairs) < 4 {
		return full
	}

	last := hintPair(pairs[len(pairs)-2], pairs[len(pairs)-1])
	sep := theme.HintDescStyle.Render(hintSeparator)
	ellipsis := theme.HintDescStyle.Render("…")
	tail := sep + ellipsis + sep + last
	budget := width - lipgloss.Width(tail)

	var kept strings.Builder
	for i := 0; i+1 < len(pairs)-2; i += 2 {
		segment := hintPair(pairs[i], pairs[i+1])
		if i > 0 {
			segment = sep + segment
		}
		if lipgloss.Width(kept.String())+lipgloss.Width(segment) > budget {
			break
		}
		kept.WriteString(segment)
	}
	return kept.String() + tail
}

func joinHint(parts ...string) string {
	return strings.Join(parts, theme.HintDescStyle.Render(hintSeparator))
}

// firstThatFits returns the first candidate no wider than width, or "" if none
// fits. Truncating styled text by rune could split an escape sequence, so
// callers supply shorter wordings instead.
func firstThatFits(width int, candidates ...string) string {
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return ""
}

func hintPair(key, desc string) string {
	return theme.HintKeyStyle.Render(key) + theme.HintDescStyle.Render(" "+desc)
}

// clipLines guards the pane borders: lipgloss's Height() pads short content but
// does not clip tall content, so a wrapped line would push the bottom border
// off-screen.
func clipLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

func truncateLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth > 1 {
		return string(runes[:maxWidth-1]) + "…"
	}
	return string(runes[:maxWidth])
}

// leftTruncateFilename keeps the end of a path: "…ernal/db/context_test.go".
func leftTruncateFilename(name string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= maxWidth {
		return name
	}
	if maxWidth == 1 {
		return "…"
	}
	return "…" + string(runes[len(runes)-(maxWidth-1):])
}

// scrollbarThumb returns the thumb's 0-indexed start row and height within
// innerH content rows, or (0, 0) when all content fits.
func scrollbarThumb(innerH, offset, total int) (start, size int) {
	if total <= innerH || innerH <= 0 {
		return 0, 0
	}
	size = max(1, innerH*innerH/total)
	maxOffset := total - innerH
	if maxOffset <= 0 {
		return 0, size
	}
	start = (offset * (innerH - size)) / maxOffset
	return start, size
}

// injectScrollbar overwrites part of a rendered pane's right border with
// thumbChar. borderChar must match the pane's own border character, and innerH
// excludes the two border rows.
func injectScrollbar(rendered, borderChar, thumbChar string, offset, total, innerH int) string {
	thumbStart, thumbSize := scrollbarThumb(innerH, offset, total)
	if thumbSize == 0 {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		if i == 0 || i == len(lines)-1 {
			continue
		}
		contentRow := i - 1
		if contentRow >= thumbStart && contentRow < thumbStart+thumbSize {
			idx := strings.LastIndex(line, borderChar)
			if idx >= 0 {
				lines[i] = line[:idx] + thumbChar + line[idx+len(borderChar):]
			}
		}
	}
	return strings.Join(lines, "\n")
}

// connectPaneCorners turns a pane's rounded corners into T-junctions so it joins
// the status bar above. Side-by-side panes pass only their outer corners.
func connectPaneCorners(rendered string, left, right bool) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) == 0 {
		return rendered
	}
	if left {
		lines[0] = strings.Replace(lines[0], "╭", "├", 1)
	}
	if right {
		if idx := strings.LastIndex(lines[0], "╮"); idx >= 0 {
			lines[0] = lines[0][:idx] + "┤" + lines[0][idx+len("╮"):]
		}
	}
	return strings.Join(lines, "\n")
}
