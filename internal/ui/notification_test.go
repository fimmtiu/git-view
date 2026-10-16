package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func showNotif(t *testing.T, m Model, text string) Model {
	t.Helper()
	updated, _ := m.Update(ShowNotification(text)())
	return updated.(Model)
}

func TestNotification_AppearsAboveTheHintBar(t *testing.T) {
	m := showNotif(t, onSelector(t, 5, 100, 30), "Hello there")

	lines := strings.Split(stripAnsi(m.View()), "\n")
	if len(lines) != 30 {
		t.Fatalf("the popup should not change the screen height, got %d lines", len(lines))
	}
	// The box is three lines tall, so its text sits on the third line from the
	// bottom: one for the box's lower border, one for the hint bar.
	textLine := lines[len(lines)-3]
	if !strings.HasSuffix(strings.TrimRight(textLine, " "), "Hello there  │") {
		t.Errorf("expected the popup at the right edge, got %q", textLine)
	}
	if strings.Contains(lines[len(lines)-1], "╰") {
		t.Error("the popup should leave the hint bar uncovered")
	}
}

func TestNotification_KeepsEveryLineWithinTheScreenWidth(t *testing.T) {
	m := showNotif(t, onSelector(t, 5, 100, 30), "Hello there")
	for i, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 100 {
			t.Errorf("line %d is %d columns wide", i, w)
		}
	}
}

func TestNotification_ClearsWhenItsTimerFires(t *testing.T) {
	m := showNotif(t, onSelector(t, 5, 100, 30), "Hello there")
	cleared, _ := m.Update(clearNotifMsg{id: m.notifID})

	if strings.Contains(stripAnsi(cleared.(Model).View()), "Hello there") {
		t.Error("the popup should be gone after its timer fires")
	}
}

func TestNotification_StaleTimerKeepsTheNewerPopup(t *testing.T) {
	first := showNotif(t, onSelector(t, 5, 100, 30), "First")
	second := showNotif(t, first, "Second")
	updated, _ := second.Update(clearNotifMsg{id: first.notifID})

	if !strings.Contains(stripAnsi(updated.(Model).View()), "Second") {
		t.Error("the first popup's timer should not clear the second popup")
	}
}

func TestNotification_SurvivesTinyTerminal(t *testing.T) {
	m := showNotif(t, onSelector(t, 5, 5, 3), "A much longer message than fits")
	_ = m.View()
}

func TestShowOSNotification_NilWithoutTerminalNotifier(t *testing.T) {
	saved := hasTerminalNotifier
	hasTerminalNotifier = false
	t.Cleanup(func() { hasTerminalNotifier = saved })

	if ShowOSNotification("hi") != nil {
		t.Error("expected no command when terminal-notifier is missing")
	}
}
