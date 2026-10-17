package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderDialog draws v as a bordered box no wider than maxWidth: the title, the
// lines, the elapsed time when showElapsed is set, and the button.
func renderDialog(v dialogView, maxWidth int) string {
	const minInnerWidth = 30
	
	// Calculate border and padding from the actual style
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
	
	var content strings.Builder
	
	// Title
	titleText := ansi.Truncate(v.title, innerWidth, "…")
	content.WriteString(theme.DialogTitleStyle.Render(titleText))
	content.WriteString("\n\n")
	
	// Lines
	for _, line := range v.lines {
		truncated := ansi.Truncate(line, innerWidth, "…")
		content.WriteString(truncated)
		content.WriteString("\n")
	}
	
	// Elapsed time
	if v.showElapsed {
		content.WriteString("\n")
		elapsed := "Elapsed: " + formatElapsed(v.elapsed)
		content.WriteString(ansi.Truncate(elapsed, innerWidth, "…"))
		content.WriteString("\n")
	}
	
	// Button (centered)
	content.WriteString("\n")
	button := theme.DialogButtonStyle.Render(v.button)
	buttonWidth := lipgloss.Width(button)
	padding := (innerWidth - buttonWidth) / 2
	if padding > 0 {
		content.WriteString(strings.Repeat(" ", padding))
	}
	content.WriteString(button)
	
	// Apply dialog style with border
	return theme.DialogStyle.Width(innerWidth).Render(content.String())
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
