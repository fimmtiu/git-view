package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// hintSeparator sits between adjacent key/description pairs.
const hintSeparator = "  "

// buildHint renders alternating key/description pairs with each key bolded.
// Example: buildHint("Q", "quit", "?", "help") → bold("Q")+" quit  "+bold("?")+" help"
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

// buildHintFit renders as many key/description pairs as fit within width
// columns, always keeping the final pair — that is the quit binding, which is
// the one a user stuck in the viewer most needs to see. Pairs are dropped from
// the end of the elidable run and replaced with an ellipsis, so the help text
// stays exactly one row tall no matter how narrow the terminal is.
func buildHintFit(width int, pairs ...string) string {
	full := buildHint(pairs...)
	if width <= 0 || lipgloss.Width(full) <= width || len(pairs) < 4 {
		return full
	}

	// Reserve room for the ellipsis and the final pair.
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

// hintPair renders one bolded key and its description.
func hintPair(key, desc string) string {
	return theme.HintKeyStyle.Render(key) + theme.HintDescStyle.Render(" "+desc)
}

// clipLines truncates content to at most maxLines lines, preventing overflow
// when lipgloss line-wrapping produces more lines than the pane expects.
// lipgloss's Height() pads short content but does not clip tall content, so
// without this guard a wrapped line pushes the bottom border off-screen.
func clipLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

// truncateLine truncates s to at most maxWidth visible runes, appending an
// ellipsis if truncation occurred.
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

// leftTruncateFilename truncates a filename from the left with an ellipsis
// if it exceeds maxWidth runes, e.g. "…ernal/db/project_context_test.go".
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
	// Keep the rightmost (maxWidth-1) runes plus ellipsis.
	return "…" + string(runes[len(runes)-(maxWidth-1):])
}

// scrollbarThumb calculates the 0-indexed start row and height of a scroll
// thumb within the innerH content rows of a pane.
// Returns (0, 0) when all content fits and no indicator is needed.
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

// injectScrollbar replaces the right border character on the appropriate rows
// of a lipgloss-rendered pane with thumbChar to show a scroll position indicator.
//
// borderChar must match the pane's right-border character (e.g. "│" for
// NormalBorder/RoundedBorder).
// innerH is the number of content rows (pane height minus the two border rows).
func injectScrollbar(rendered, borderChar, thumbChar string, offset, total, innerH int) string {
	thumbStart, thumbSize := scrollbarThumb(innerH, offset, total)
	if thumbSize == 0 {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	// Drop a trailing empty element produced by a trailing newline, if present.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		if i == 0 || i == len(lines)-1 {
			continue // skip top and bottom border rows
		}
		contentRow := i - 1 // 0-indexed within content rows
		if contentRow >= thumbStart && contentRow < thumbStart+thumbSize {
			idx := strings.LastIndex(line, borderChar)
			if idx >= 0 {
				lines[i] = line[:idx] + thumbChar + line[idx+len(borderChar):]
			}
		}
	}
	return strings.Join(lines, "\n")
}

// connectPaneTop replaces the top-left and top-right rounded corners of a
// bordered pane with T-junctions so the pane visually connects to the status
// bar border directly above it.
func connectPaneTop(rendered string) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) == 0 {
		return rendered
	}
	lines[0] = strings.Replace(lines[0], "╭", "├", 1)
	if idx := strings.LastIndex(lines[0], "╮"); idx >= 0 {
		lines[0] = lines[0][:idx] + "┤" + lines[0][idx+len("╮"):]
	}
	return strings.Join(lines, "\n")
}
