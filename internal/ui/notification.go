package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// How long a popup stays on screen.
const notifDuration = 5 * time.Second

// notifIconPath is the absolute path to the OS notification icon.
var notifIconPath = filepath.Join(os.Getenv("HOME"), "src", "git-view", "img", "terminal_icon.png")

// notifMsg triggers a new notification popup.
type notifMsg struct{ text string }

// clearNotifMsg dismisses the notification whose ID matches the current one.
type clearNotifMsg struct{ id int }

// ShowNotification returns a tea.Cmd that displays text in a transient popup
// in the bottom-right corner of the screen for 5 seconds. It does not steal
// focus and can be returned from any Update handler.
func ShowNotification(text string) tea.Cmd {
	return func() tea.Msg { return notifMsg{text: text} }
}

// hasTerminalNotifier caches whether terminal-notifier is available.
var hasTerminalNotifier = func() bool {
	_, err := exec.LookPath("terminal-notifier")
	return err == nil
}()

// ShowOSNotification returns a tea.Cmd that sends an OS-level notification via
// terminal-notifier. It returns nil when terminal-notifier is not installed.
func ShowOSNotification(text string) tea.Cmd {
	if !hasTerminalNotifier {
		return nil
	}
	return func() tea.Msg {
		args := []string{
			"-title", "git-view",
			"-message", text,
			"-contentImage", notifIconPath,
		}
		// macOS sets this to the bundle ID of the app that launched the
		// terminal session, so a click on the notification brings it forward.
		if bundleID := os.Getenv("__CFBundleIdentifier"); bundleID != "" {
			args = append(args, "-activate", bundleID)
		}
		_ = exec.Command("terminal-notifier", args...).Run()
		return nil
	}
}

// overlayBottomRight draws box over the bottom-right corner of screen, leaving
// the last bottomMargin lines uncovered.
func overlayBottomRight(screen, box string, width, bottomMargin int) string {
	lines := strings.Split(screen, "\n")
	boxLines := strings.Split(box, "\n")
	x := max(width-lipgloss.Width(box), 0)
	y := max(len(lines)-len(boxLines)-bottomMargin, 0)
	for i, boxLine := range boxLines {
		row := y + i
		if row >= len(lines) {
			break
		}
		// The reset stops styles open in the background from bleeding into the box.
		lines[row] = ansi.Truncate(lines[row], x, "") + "\x1b[0m" + ansi.Truncate(boxLine, width, "")
	}
	return strings.Join(lines, "\n")
}
