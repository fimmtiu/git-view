package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ── formatElapsed ────────────────────────────────────────────────────────────

func TestFormatElapsed_Zero(t *testing.T) {
	got := formatElapsed(0)
	if got != "00:00" {
		t.Errorf("formatElapsed(0) = %q, want %q", got, "00:00")
	}
}

func TestFormatElapsed_Seconds(t *testing.T) {
	got := formatElapsed(59*time.Second + 900*time.Millisecond)
	if got != "00:59" {
		t.Errorf("formatElapsed(59.9s) = %q, want %q", got, "00:59")
	}
}

func TestFormatElapsed_OneMinute(t *testing.T) {
	got := formatElapsed(61 * time.Second)
	if got != "01:01" {
		t.Errorf("formatElapsed(61s) = %q, want %q", got, "01:01")
	}
}

func TestFormatElapsed_LargeMinutes(t *testing.T) {
	got := formatElapsed(75*time.Minute + 3*time.Second)
	if got != "75:03" {
		t.Errorf("formatElapsed(75m3s) = %q, want %q", got, "75:03")
	}
}

func TestFormatElapsed_Negative(t *testing.T) {
	got := formatElapsed(-1 * time.Second)
	if got != "00:00" {
		t.Errorf("formatElapsed(-1s) = %q, want %q", got, "00:00")
	}
}

// ── renderDialog ─────────────────────────────────────────────────────────────

func TestRenderDialog_ContainsAllElements(t *testing.T) {
	v := dialogView{
		title:       "Test Dialog",
		lines:       []string{"Line one", "Line two"},
		elapsed:     65 * time.Second,
		showElapsed: true,
		button:      "OK",
	}
	
	got := renderDialog(v, 80)
	plain := stripAnsi(got)
	
	if !strings.Contains(plain, "Test Dialog") {
		t.Errorf("renderDialog output missing title: %q", plain)
	}
	if !strings.Contains(plain, "Line one") {
		t.Errorf("renderDialog output missing first line: %q", plain)
	}
	if !strings.Contains(plain, "Line two") {
		t.Errorf("renderDialog output missing second line: %q", plain)
	}
	if !strings.Contains(plain, "Elapsed: 01:05") {
		t.Errorf("renderDialog output missing elapsed time: %q", plain)
	}
	if !strings.Contains(plain, "OK") {
		t.Errorf("renderDialog output missing button: %q", plain)
	}
}

func TestRenderDialog_NoElapsedWhenShowElapsedFalse(t *testing.T) {
	v := dialogView{
		title:       "Test",
		lines:       []string{"Content"},
		elapsed:     65 * time.Second,
		showElapsed: false,
		button:      "Cancel",
	}
	
	got := renderDialog(v, 80)
	plain := stripAnsi(got)
	
	if strings.Contains(plain, "Elapsed") {
		t.Errorf("renderDialog should not show elapsed when showElapsed is false: %q", plain)
	}
}

func TestRenderDialog_RespectsMaxWidth(t *testing.T) {
	// Create a very long line
	longLine := strings.Repeat("x", 200)
	
	tests := []struct {
		maxWidth int
	}{
		{20},
		{40},
		{120},
	}
	
	for _, tc := range tests {
		v := dialogView{
			title:       "Title",
			lines:       []string{longLine},
			showElapsed: false,
			button:      "OK",
		}
		
		got := renderDialog(v, tc.maxWidth)
		lines := strings.Split(got, "\n")
		
		for i, line := range lines {
			w := lipgloss.Width(line)
			if w > tc.maxWidth {
				t.Errorf("maxWidth=%d: line %d is %d wide, want at most %d", tc.maxWidth, i, w, tc.maxWidth)
			}
		}
	}
}

// ── overlayCenter ────────────────────────────────────────────────────────────

func TestOverlayCenter_PlacesBoxInMiddle(t *testing.T) {
	const width, height = 80, 24
	
	// Create a screen of dots
	var sb strings.Builder
	for i := 0; i < height; i++ {
		sb.WriteString(strings.Repeat(".", width))
		if i < height-1 {
			sb.WriteString("\n")
		}
	}
	screen := sb.String()
	
	// Create a small box (3 lines, 10 columns each including borders)
	box := "┌────────┐\n│  Test  │\n└────────┘"
	boxHeight := 3
	
	got := overlayCenter(screen, box, width, height)
	lines := strings.Split(got, "\n")
	
	// Check that we still have the same number of lines
	if len(lines) != height {
		t.Errorf("overlayCenter changed line count: got %d, want %d", len(lines), height)
	}
	
	// Check each line is exactly 80 wide
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d is %d wide, want %d", i, w, width)
		}
	}
	
	// The box should be centered vertically: (24-3)/2 = 10 (0-indexed)
	expectedRow := (height - boxHeight) / 2
	
	// Check that the box appears at the expected row (middle line contains "Test")
	middleBoxLine := expectedRow + 1
	if !strings.Contains(stripAnsi(lines[middleBoxLine]), "Test") {
		t.Errorf("box not at expected row %d; lines around that position:\n%s\n%s\n%s",
			middleBoxLine,
			stripAnsi(lines[expectedRow]),
			stripAnsi(lines[middleBoxLine]),
			stripAnsi(lines[expectedRow+2]))
	}
	
	// Check that dots are preserved left and right of the box
	for i, line := range lines {
		plain := stripAnsi(line)
		// Lines outside the box should still be all dots
		if i < expectedRow || i >= expectedRow+boxHeight {
			if !strings.Contains(plain, "...") {
				t.Errorf("line %d should still contain dots: %q", i, plain)
			}
		}
		// Lines with the box should have dots on both sides
		if i >= expectedRow && i < expectedRow+boxHeight {
			if !strings.HasPrefix(plain, "...") {
				t.Errorf("line %d should have dots on the left: %q", i, plain)
			}
			if !strings.HasSuffix(plain, "...") {
				t.Errorf("line %d should have dots on the right: %q", i, plain)
			}
		}
	}
}

func TestOverlayCenter_LargerBoxDoesNotPanic(t *testing.T) {
	screen := strings.Repeat(".", 80)
	box := strings.Repeat("X\n", 100)
	
	// Should not panic
	_ = overlayCenter(screen, box, 80, 1)
}
