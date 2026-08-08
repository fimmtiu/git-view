package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Without a tty lipgloss degrades to the Ascii profile and renders every style
// as plain text, silently making the styling assertions vacuous.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}
