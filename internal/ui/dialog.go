package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (v dialogView) Render(maxWidth int) string {
	innerWidth := v.calculateDialogWidth(maxWidth)
	
	var content strings.Builder
	content.WriteString(v.formatTitle(innerWidth))
	content.WriteString(v.formatContentLines(innerWidth))
	content.WriteString(v.formatElapsedSection(innerWidth))
	content.WriteString(v.formatButtonSection(innerWidth))
	
	return theme.DialogStyle.Width(innerWidth).Render(content.String())
}

// calculateDialogWidth computes the inner content width after accounting for
// borders and padding. It applies minimum width constraints.
func (v dialogView) calculateDialogWidth(maxWidth int) int {
	const minInnerWidth = 30
	
	_, right, _, left := theme.DialogStyle.GetPadding()
	horizontalPadding := left + right
	borderWidth := 2 // Left border character + right border character
	borderAndPadding := borderWidth + horizontalPadding
	
	innerWidth := maxWidth - borderAndPadding
	if innerWidth < 0 {
		innerWidth = 0
	} else if innerWidth < minInnerWidth && maxWidth >= minInnerWidth+borderAndPadding {
		innerWidth = minInnerWidth
	}
	
	return innerWidth
}

// formatTitle returns the rendered title section with trailing newlines.
func (v dialogView) formatTitle(innerWidth int) string {
	titleText := ansi.Truncate(v.title, innerWidth, "…")
	return theme.DialogTitleStyle.Render(titleText) + "\n\n"
}

// formatContentLines returns the formatted content lines with newlines.
func (v dialogView) formatContentLines(innerWidth int) string {
	var content strings.Builder
	
	for _, line := range v.lines {
		truncated := ansi.Truncate(line, innerWidth, "…")
		content.WriteString(truncated)
		content.WriteString("\n")
	}
	
	return content.String()
}

// formatElapsedSection returns the elapsed time section if showElapsed is true,
// otherwise returns an empty string.
func (v dialogView) formatElapsedSection(innerWidth int) string {
	if !v.showElapsed {
		return ""
	}
	
	var section strings.Builder
	section.WriteString("\n")
	elapsed := "Elapsed: " + formatElapsed(v.elapsed)
	section.WriteString(ansi.Truncate(elapsed, innerWidth, "…"))
	section.WriteString("\n")
	
	return section.String()
}

// formatButtonSection returns the button centered within the available width.
func (v dialogView) formatButtonSection(innerWidth int) string {
	var section strings.Builder
	section.WriteString("\n")
	
	button := theme.DialogButtonStyle.Render(v.button)
	buttonWidth := lipgloss.Width(button)
	padding := (innerWidth - buttonWidth) / 2
	
	if padding > 0 {
		section.WriteString(strings.Repeat(" ", padding))
	}
	section.WriteString(button)
	
	return section.String()
}

// formatElapsed renders d as MM:SS, rounded down to whole seconds. Minutes are not capped at 59.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		return "00:00"
	}
	
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func maxWidth(lines []string) int {
	max := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > max {
			max = w
		}
	}
	return max
}

// overlayCenter draws box over the middle of screen.
func overlayCenter(screen, box string, width, height int) string {
	lines := strings.Split(screen, "\n")
	boxLines := strings.Split(box, "\n")
	
	boxWidth := maxWidth(boxLines)
	boxHeight := len(boxLines)
	
	// Calculate centered position
	x := (width - boxWidth) / 2
	if x < 0 {
		x = 0
	}
	
	y := (height - boxHeight) / 2
	if y < 0 {
		y = 0
	}
	
	// Overlay the box
	for i, boxLine := range boxLines {
		row := y + i
		if row >= len(lines) {
			// Box lines below screen are dropped
			break
		}
		
		left := ansi.Truncate(lines[row], x, "")
		boxLineWidth := lipgloss.Width(boxLine)
		skipChars := x + boxLineWidth
		var right string
		if skipChars < width {
			right = ansi.TruncateLeft(lines[row], skipChars, "")
		}
		
		// The reset stops styles open in the background from bleeding into the box
		lines[row] = left + "\x1b[0m" + boxLine + "\x1b[0m" + right
	}
	
	return strings.Join(lines, "\n")
}
