package ui

import "time"

// renderDialog draws v as a bordered box no wider than maxWidth: the title, the
// lines, the elapsed time when showElapsed is set, and the button.
func renderDialog(v dialogView, maxWidth int) string { panic("scaffold stub") }

// formatElapsed renders d as MM:SS, rounded down to whole seconds. Minutes are not capped at 59.
func formatElapsed(d time.Duration) string { panic("scaffold stub") }

// overlayCenter draws box over the middle of screen.
func overlayCenter(screen, box string, width, height int) string { panic("scaffold stub") }
