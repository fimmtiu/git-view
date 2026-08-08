package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
)

// sized returns a Model laid out at the given terminal dimensions.
func sized(t *testing.T, files []diff.File, width, height int) Model {
	t.Helper()
	m, _ := NewModel(files, "/tmp/repo", "abc1234").
		Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m.(Model)
}

// send delivers a key to the model and returns the updated model and command.
func send(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key(k))
	return updated.(Model), cmd
}

func TestView_FillsTheScreen(t *testing.T) {
	const width, height = 100, 30
	m := sized(t, largeSampleFiles(), width, height)

	lines := strings.Split(m.View(), "\n")
	if len(lines) != height {
		t.Errorf("view has %d lines, want %d (the full terminal height)", len(lines), height)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d columns wide, want at most %d", i, w, width)
		}
	}
	// Every row but the hint bar is part of the bordered status bar or pane, so
	// they all span the full width.
	for i, line := range lines[:len(lines)-1] {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d is %d columns wide, want exactly %d", i, w, width)
		}
	}
}

func TestView_HasNoFunctionKeyBar(t *testing.T) {
	m := sized(t, sampleFiles(), 100, 30)
	view := stripAnsi(m.View())

	for _, tab := range []string{"F1", "F2", "F3", "F4", "F5", "Diffs", "Projects"} {
		if strings.Contains(view, tab) {
			t.Errorf("view should not contain the code-factory tab bar, found %q", tab)
		}
	}
}

func TestView_HasHelpTextAtTheBottom(t *testing.T) {
	m := sized(t, sampleFiles(), 100, 30)
	lines := strings.Split(m.View(), "\n")
	hint := stripAnsi(lines[len(lines)-1])

	for _, want := range []string{"scroll", "select lines", "quit"} {
		if !strings.Contains(hint, want) {
			t.Errorf("bottom help text should mention %q, got %q", want, hint)
		}
	}
}

func TestView_HelpTextOmitsTerminalCommandButKeepsEdit(t *testing.T) {
	m := sized(t, sampleFiles(), 100, 30)
	hint := stripAnsi(m.View())

	if strings.Contains(hint, "terminal") {
		t.Error("the terminal command should have been removed")
	}
	if !strings.Contains(hint, "edit") {
		t.Error("the edit command should be kept")
	}
}

func TestView_StatusBarShowsFileAndPosition(t *testing.T) {
	m := sized(t, sampleFiles(), 100, 30)
	status := stripAnsi(strings.Split(m.View(), "\n")[1])

	if !strings.Contains(status, "internal/ui/app.go") {
		t.Errorf("status bar should show the current filename, got %q", status)
	}
	if !strings.Contains(status, "File 1 of 2") {
		t.Errorf("status bar should show the file position, got %q", status)
	}
	if !strings.Contains(status, "abc1234") {
		t.Errorf("status bar should show the revision label, got %q", status)
	}
}

func TestView_StatusBarIsOneTextLine(t *testing.T) {
	m := sized(t, sampleFiles(), 100, 30)
	lines := strings.Split(m.View(), "\n")

	// Row 0 is the status bar's top border; row 1 is its single text line;
	// row 2 is the pane's top border, joined to the status bar by T-junctions.
	if !strings.Contains(lines[2], "├") || !strings.Contains(lines[2], "┤") {
		t.Errorf("pane top border should connect to the status bar, got %q", lines[2])
	}
}

func TestView_StatusBarTruncatesLongFilenameFromTheLeft(t *testing.T) {
	long := "internal/some/very/deeply/nested/directory/tree/with/a/long/path/file_name_test.go"
	files := []diff.File{{
		Name:  long,
		Type:  diff.Normal,
		Hunks: []diff.Hunk{{NewStart: 1, NewCount: 1, Lines: []diff.Line{{Type: diff.LineAdded, Content: "x"}}}},
	}}
	m := sized(t, files, 60, 30)
	status := stripAnsi(strings.Split(m.View(), "\n")[1])

	if !strings.Contains(status, "…") {
		t.Errorf("expected a left-truncation ellipsis, got %q", status)
	}
	if !strings.Contains(status, "file_name_test.go") {
		t.Errorf("truncation should keep the end of the path, got %q", status)
	}
}

func TestView_StatusBarTracksScrolling(t *testing.T) {
	m := sized(t, largeSampleFiles(), 100, 30)
	m.viewer.offset = m.viewer.fileStarts[1]

	status := stripAnsi(strings.Split(m.View(), "\n")[1])
	if !strings.Contains(status, "second_file.go") {
		t.Errorf("status bar should follow the scroll position, got %q", status)
	}
	if !strings.Contains(status, "File 2 of 2") {
		t.Errorf("status bar should show file 2 of 2, got %q", status)
	}
}

func TestView_EmptyDiff(t *testing.T) {
	m := sized(t, nil, 80, 24)
	view := stripAnsi(m.View())

	if !strings.Contains(view, "No diff content") {
		t.Error("expected the empty-state message")
	}
	if got := len(strings.Split(m.View(), "\n")); got != 24 {
		t.Errorf("view has %d lines, want 24", got)
	}
}

func TestView_SurvivesTinyTerminal(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 3}, {20, 6}, {0, 0}} {
		m := sized(t, sampleFiles(), size[0], size[1])
		if got := m.View(); got == "" {
			t.Errorf("view is empty at %dx%d", size[0], size[1])
		}
	}
}

func TestView_ResizeReflows(t *testing.T) {
	m := sized(t, largeSampleFiles(), 100, 30)
	wide := m.View()

	narrow, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	got := narrow.(Model).View()
	if got == wide {
		t.Error("resizing should change the rendered view")
	}
	if lines := strings.Split(got, "\n"); len(lines) != 15 {
		t.Errorf("resized view has %d lines, want 15", len(lines))
	}
}

// ── Keys ─────────────────────────────────────────────────────────────────────

func TestKeys_Quit(t *testing.T) {
	for _, k := range []string{"q", "Q"} {
		m, cmd := send(t, sized(t, sampleFiles(), 80, 24), k)
		if cmd == nil {
			t.Errorf("%q should return a command", k)
			continue
		}
		if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
			t.Errorf("%q should quit", k)
		}
		if got := m.View(); got != "" {
			t.Errorf("view should be empty while quitting, got %q", got)
		}
	}
}

func TestKeys_CtrlCQuits(t *testing.T) {
	_, cmd := sized(t, sampleFiles(), 80, 24).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error("ctrl+c should quit")
	}
}

func TestKeys_ScrollReachesTheViewer(t *testing.T) {
	m, _ := send(t, sized(t, largeSampleFiles(), 80, 24), "down")
	if m.viewer.offset != 1 {
		t.Errorf("down should scroll the viewer, offset = %d", m.viewer.offset)
	}
}

func TestKeys_EnterStartsLineSelect(t *testing.T) {
	m, _ := send(t, sized(t, largeSampleFiles(), 80, 24), "enter")
	if !m.viewer.lineSelectMode {
		t.Error("enter should start line-select mode")
	}
}

func TestKeys_EditWithoutAnEditorReportsIt(t *testing.T) {
	m := sized(t, sampleFiles(), 80, 24)
	m.editorFound = false

	_, cmd := send(t, m, "E")
	if cmd == nil {
		t.Fatal("E should report that no editor was found")
	}
	msg, ok := cmd().(statusMsg)
	if !ok {
		t.Fatalf("expected a statusMsg, got %T", cmd())
	}
	if !msg.isErr || !strings.Contains(msg.text, "no editor") {
		t.Errorf("expected a no-editor error, got %+v", msg)
	}
}

func TestKeys_EditIsANoOpWithoutAFile(t *testing.T) {
	m := sized(t, nil, 80, 24)
	m.editorFound = true

	if _, cmd := send(t, m, "E"); cmd != nil {
		t.Error("E should do nothing when there is no file to open")
	}
}

func TestKeys_GitHubReportsFailureInTheStatusBar(t *testing.T) {
	// /tmp/repo is not a git repository, so the URL lookup fails and the error
	// must surface in the status bar rather than being swallowed.
	m := sized(t, sampleFiles(), 80, 24)

	_, cmd := send(t, m, "g")
	if cmd == nil {
		t.Fatal("g should return a status command")
	}
	msg, ok := cmd().(statusMsg)
	if !ok {
		t.Fatalf("expected a statusMsg, got %T", cmd())
	}
	if !msg.isErr {
		t.Errorf("expected an error status, got %+v", msg)
	}
	if !strings.Contains(msg.text, "not a github.com remote") &&
		!strings.Contains(msg.text, "no origin remote") {
		t.Errorf("expected an explanation of why the URL could not be built, got %q", msg.text)
	}
}

func TestStatusBar_ShowsMessagesAndStaysOneLine(t *testing.T) {
	const width, height = 60, 20
	m := sized(t, sampleFiles(), width, height)

	for _, text := range []string{
		"opened https://github.com/o/r/blob/main/f.go#L12",
		strings.Repeat("a very long error message that will not fit ", 5),
	} {
		shown, _ := m.Update(statusMsg{text: text, isErr: true})
		view := shown.(Model).View()

		if got := len(strings.Split(view, "\n")); got != height {
			t.Errorf("a %d-char status changed the view height to %d, want %d", len(text), got, height)
		}
		status := strings.Trim(stripAnsi(strings.Split(view, "\n")[1]), "│ ")
		if !strings.HasPrefix(status, text[:20]) {
			t.Errorf("status bar = %q, want it to start with %q", status, text[:20])
		}
	}
}

func TestKeys_GitHubIsANoOpWithoutAFile(t *testing.T) {
	if _, cmd := send(t, sized(t, nil, 80, 24), "g"); cmd != nil {
		t.Error("g should do nothing when there is no file to open")
	}
}

func TestKeys_ClearStaleStatusMessage(t *testing.T) {
	m := sized(t, largeSampleFiles(), 80, 24)
	withStatus, _ := m.Update(statusMsg{text: "something happened", isErr: true})

	cleared, _ := send(t, withStatus.(Model), "down")
	if cleared.status != "" {
		t.Errorf("a scroll key should clear the stale status, got %q", cleared.status)
	}
	if strings.Contains(stripAnsi(cleared.View()), "something happened") {
		t.Error("the stale status should no longer be rendered")
	}
}

func TestKeys_GKeyIsNotSwallowedByLineSelect(t *testing.T) {
	m := sized(t, largeSampleFiles(), 80, 24)
	inSelect, _ := send(t, m, "enter")

	_, cmd := send(t, inSelect, "g")
	if cmd == nil {
		t.Error("g should still open GitHub while in line-select mode")
	}
}
