package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces a colour profile for the whole package. Without a tty
// lipgloss degrades to the Ascii profile and every style renders as plain
// text, which would silently make the styling assertions below vacuous.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}
