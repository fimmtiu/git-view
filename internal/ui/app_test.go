package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
	"github.com/fimmtiu/git-view/internal/git"
)

func onSelector(t *testing.T, n, width, height int) Model {
	t.Helper()
	m, _ := NewModel("/tmp/repo", "📁 myrepo ⎇  main").
		Update(tea.WindowSizeMsg{Width: width, Height: height})
	loaded, _ := m.(Model).Update(commitListMsg{commits: sampleCommits(n), forkPointIdx: -1})
	return loaded.(Model)
}

func onViewer(t *testing.T, files []diff.File, width, height int) Model {
	t.Helper()
	m := onSelector(t, 5, width, height)
	opened, _ := m.Update(diffContentMsg{files: files})
	return opened.(Model)
}

func send(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key(k))
	return updated.(Model), cmd
}

// ── Startup ──────────────────────────────────────────────────────────────────

func TestInit_LoadsTheCommitList(t *testing.T) {
	cmd := NewModel("/tmp/repo", "📁 myrepo ⎇  main").Init()
	if cmd == nil {
		t.Fatal("Init should start the commit-list fetch")
	}
	// /tmp/repo is not a repository, so the fetch must report an error rather
	// than hang or panic.
	msg, ok := cmd().(commitListMsg)
	if !ok {
		t.Fatalf("expected a commitListMsg, got %T", cmd())
	}
	if msg.err == "" {
		t.Error("expected an error for a non-repository path")
	}
}

func TestStartsOnTheCommitSelector(t *testing.T) {
	m := onSelector(t, 5, 100, 30)

	if m.viewer != nil {
		t.Fatal("the app should open on the selector, not the viewer")
	}
	view := stripAnsi(m.View())
	if !strings.Contains(view, "commit message 0") {
		t.Errorf("expected the commit list, got %q", view)
	}
	if !strings.Contains(view, "1 commit selected") {
		t.Error("expected the selection count in the status bar")
	}
}

func TestCommitListError_ShowsInTheStatusBar(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	failed, _ := m.Update(commitListMsg{err: "fatal: bad object"})

	if !strings.Contains(stripAnsi(failed.(Model).View()), "fatal: bad object") {
		t.Error("a commit-list error should be visible in the status bar")
	}
}

// ── Layout ───────────────────────────────────────────────────────────────────

func TestView_FillsTheScreenOnBothScreens(t *testing.T) {
	const width, height = 100, 30

	for name, m := range map[string]Model{
		"selector": onSelector(t, 20, width, height),
		"viewer":   onViewer(t, largeSampleFiles(), width, height),
	} {
		lines := strings.Split(m.View(), "\n")
		if len(lines) != height {
			t.Errorf("%s: view has %d lines, want %d", name, len(lines), height)
		}
		// Every row but the hint bar is bordered chrome and spans the screen.
		for i, line := range lines[:len(lines)-1] {
			if w := lipgloss.Width(line); w != width {
				t.Errorf("%s: line %d is %d columns wide, want %d", name, i, w, width)
			}
		}
		if w := lipgloss.Width(lines[len(lines)-1]); w > width {
			t.Errorf("%s: the hint bar is %d columns wide, want at most %d", name, w, width)
		}
	}
}

func TestView_HasNoFunctionKeyBar(t *testing.T) {
	for _, m := range []Model{
		onSelector(t, 5, 100, 30),
		onViewer(t, sampleFiles(), 100, 30),
	} {
		view := stripAnsi(m.View())
		for _, tab := range []string{"F1", "F2", "F3", "F4", "F5", "Diffs", "Projects"} {
			if strings.Contains(view, tab) {
				t.Errorf("view should not contain the code-factory tab bar, found %q", tab)
			}
		}
	}
}

func TestView_HasNoTicketConcepts(t *testing.T) {
	for _, m := range []Model{
		onSelector(t, 5, 100, 30),
		onViewer(t, sampleFiles(), 100, 30),
	} {
		view := strings.ToLower(stripAnsi(m.View()))
		for _, word := range []string{"ticket", "phase", "worktree", "project:"} {
			if strings.Contains(view, word) {
				t.Errorf("view should not mention %q, got %q", word, view)
			}
		}
	}
}

func TestView_HelpTextPerScreen(t *testing.T) {
	selectorHint := lastLine(stripAnsi(onSelector(t, 5, 140, 30).View()))
	for _, want := range []string{"navigate", "view diff", "extend range", "quit"} {
		if !strings.Contains(selectorHint, want) {
			t.Errorf("selector help should mention %q, got %q", want, selectorHint)
		}
	}

	viewerHint := lastLine(stripAnsi(onViewer(t, sampleFiles(), 140, 30).View()))
	for _, want := range []string{"scroll", "select lines", "quit"} {
		if !strings.Contains(viewerHint, want) {
			t.Errorf("viewer help should mention %q, got %q", want, viewerHint)
		}
	}
}

func TestView_HelpTextOmitsTerminalCommandButKeepsEdit(t *testing.T) {
	for name, m := range map[string]Model{
		"selector": onSelector(t, 5, 140, 30),
		"viewer":   onViewer(t, sampleFiles(), 140, 30),
	} {
		hint := lastLine(stripAnsi(m.View()))
		if strings.Contains(hint, "terminal") {
			t.Errorf("%s: the terminal command should have been removed, got %q", name, hint)
		}
		if !strings.Contains(hint, "edit") {
			t.Errorf("%s: the edit command should be kept, got %q", name, hint)
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

func TestView_SelectorStatusBar(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	status := statusLine(m.View(), 1)

	if !strings.Contains(status, "📁 myrepo ⎇  main") {
		t.Errorf("expected the repo label, got %q", status)
	}
	if !strings.Contains(status, "1 commit selected") {
		t.Errorf("expected a singular selection count, got %q", status)
	}

	ranged, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	if got := statusLine(ranged.(Model).View(), 1); !strings.Contains(got, "2 commits selected") {
		t.Errorf("expected a plural selection count, got %q", got)
	}
}

func TestView_ViewerStatusBar(t *testing.T) {
	m := onViewer(t, sampleFiles(), 100, 30)
	line1, line2 := statusLine(m.View(), 1), statusLine(m.View(), 2)

	if !strings.Contains(line1, "Commit ") {
		t.Errorf("line 1 should name the commit range, got %q", line1)
	}
	if !strings.Contains(line1, "📁 myrepo ⎇  main") {
		t.Errorf("line 1 should show the repo label, got %q", line1)
	}
	if !strings.Contains(line2, "internal/ui/app.go") {
		t.Errorf("line 2 should show the current filename, got %q", line2)
	}
	if !strings.Contains(line2, "File 1 of 2") {
		t.Errorf("line 2 should show the file position, got %q", line2)
	}
}

func TestView_ViewerStatusBarNamesARange(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	ranged, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	opened, _ := ranged.(Model).Update(key("enter"))
	shown, _ := opened.(Model).Update(diffContentMsg{files: sampleFiles()})

	if got := statusLine(shown.(Model).View(), 1); !strings.Contains(got, "Commits ") || !strings.Contains(got, " to ") {
		t.Errorf("expected a commit range label, got %q", got)
	}
}

// Row n of a rendered view, with its border characters trimmed.
func statusLine(view string, n int) string {
	return strings.Trim(stripAnsi(strings.Split(view, "\n")[n]), "│ ")
}

func TestView_SurvivesTinyTerminal(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 3}, {20, 6}, {0, 0}} {
		for name, m := range map[string]Model{
			"selector": onSelector(t, 5, size[0], size[1]),
			"viewer":   onViewer(t, sampleFiles(), size[0], size[1]),
		} {
			if got := m.View(); got == "" {
				t.Errorf("%s: view is empty at %dx%d", name, size[0], size[1])
			}
		}
	}
}

func TestView_ResizeReflowsBothScreens(t *testing.T) {
	for name, m := range map[string]Model{
		"selector": onSelector(t, 20, 100, 30),
		"viewer":   onViewer(t, largeSampleFiles(), 100, 30),
	} {
		resized, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
		got := resized.(Model).View()
		if got == m.View() {
			t.Errorf("%s: resizing should change the rendered view", name)
		}
		if lines := strings.Split(got, "\n"); len(lines) != 15 {
			t.Errorf("%s: resized view has %d lines, want 15", name, len(lines))
		}
	}
}

// ── Screen transitions ───────────────────────────────────────────────────────

func TestEnterOpensTheViewer(t *testing.T) {
	m := onSelector(t, 5, 100, 30)

	for _, k := range []string{"enter", "tab"} {
		opened, cmd := send(t, m, k)
		if cmd == nil {
			t.Fatalf("%q should start the diff fetch", k)
		}
		if opened.viewStart.Hash == "" || opened.viewEnd.Hash == "" {
			t.Errorf("%q should record the commit range being viewed", k)
		}
		// The viewer only appears once the diff arrives.
		if opened.viewer != nil {
			t.Errorf("%q should not open the viewer before the diff loads", k)
		}
		shown, _ := opened.Update(diffContentMsg{files: sampleFiles()})
		if shown.(Model).viewer == nil {
			t.Errorf("%q: the viewer should open when the diff arrives", k)
		}
	}
}

func TestEnterIsANoOpWithNoCommits(t *testing.T) {
	m := onSelector(t, 0, 100, 30)

	if _, cmd := send(t, m, "enter"); cmd != nil {
		t.Error("enter should do nothing when there are no commits")
	}
}

func TestDiffError_ShowsInTheStatusBarAndKeepsTheSelector(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	failed, _ := m.Update(diffContentMsg{err: "fatal: bad revision"})

	got := failed.(Model)
	if got.viewer != nil {
		t.Error("a failed diff should leave the user on the selector")
	}
	if !strings.Contains(stripAnsi(got.View()), "fatal: bad revision") {
		t.Error("the diff error should be visible in the status bar")
	}
}

func TestTabAndEscReturnToTheSelector(t *testing.T) {
	for _, k := range []string{"tab", "esc"} {
		back, _ := send(t, onViewer(t, sampleFiles(), 100, 30), k)
		if back.viewer != nil {
			t.Errorf("%q should close the viewer", k)
		}
		if !strings.Contains(stripAnsi(back.View()), "commit message 0") {
			t.Errorf("%q should return to the commit list", k)
		}
	}
}

func TestEscExitsLineSelectBeforeTheViewer(t *testing.T) {
	m := onViewer(t, largeSampleFiles(), 100, 30)
	inSelect, _ := send(t, m, "enter")
	if !inSelect.viewer.lineSelectMode {
		t.Fatal("expected line-select mode")
	}

	afterEsc, _ := send(t, inSelect, "esc")
	if afterEsc.viewer == nil {
		t.Fatal("Esc should leave line-select mode, not close the viewer")
	}
	if afterEsc.viewer.lineSelectMode {
		t.Error("Esc should have left line-select mode")
	}

	// Tab still closes the viewer from within line-select mode.
	reSelected, _ := send(t, inSelect, "tab")
	if reSelected.viewer != nil {
		t.Error("Tab should close the viewer even in line-select mode")
	}
}

func TestReturningToTheSelectorKeepsTheSelection(t *testing.T) {
	m := onSelector(t, 10, 100, 30)
	moved, _ := send(t, m, "down")
	moved, _ = send(t, moved, "down")
	cursor := moved.selector.cursor

	opened, _ := moved.Update(diffContentMsg{files: sampleFiles()})
	back, _ := send(t, opened.(Model), "tab")

	if back.selector.cursor != cursor {
		t.Errorf("cursor = %d after returning, want %d", back.selector.cursor, cursor)
	}
}

// ── Keys ─────────────────────────────────────────────────────────────────────

func TestKeys_Quit(t *testing.T) {
	for _, k := range []string{"q", "Q"} {
		m, cmd := send(t, onSelector(t, 5, 80, 24), k)
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
	_, cmd := onSelector(t, 5, 80, 24).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error("ctrl+c should quit")
	}
}

func TestKeys_SelectorNavigation(t *testing.T) {
	m := onSelector(t, 20, 80, 24)

	moved, cmd := send(t, m, "down")
	if moved.selector.cursor != 1 {
		t.Errorf("down: cursor = %d, want 1", moved.selector.cursor)
	}
	if cmd == nil {
		t.Error("moving the cursor should refresh the stat preview")
	}

	paged, _ := send(t, m, "pgdown")
	if paged.selector.cursor <= 1 {
		t.Errorf("pgdown: cursor = %d, want a full page down", paged.selector.cursor)
	}

	extended, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	if got := extended.(Model).selector.selectedCount(); got != 2 {
		t.Errorf("shift+down: selected %d commits, want 2", got)
	}
}

func TestKeys_SelectorStatPreviewIsCached(t *testing.T) {
	m := onSelector(t, 20, 80, 24)
	moved, cmd := send(t, m, "down")
	if cmd == nil {
		t.Fatal("expected a stat fetch")
	}

	// Once that commit's stat has arrived, returning to it must not refetch.
	msg := showStatMsg{hash: moved.selector.rows[1].commit.Hash, output: "stat output"}
	cached, _ := moved.Update(msg)
	if _, cmd := send(t, cached.(Model), "down"); cmd == nil {
		t.Error("moving to a new commit should fetch its stat")
	}
	back, _ := cached.(Model).Update(key("up"))
	again, cmd := send(t, back.(Model), "down")
	if cmd != nil && again.selector.statHash == msg.hash {
		t.Error("returning to a cached commit should not refetch its stat")
	}
}

func TestKeys_ScrollReachesTheViewer(t *testing.T) {
	m, _ := send(t, onViewer(t, largeSampleFiles(), 80, 24), "down")
	if m.viewer.offset != 1 {
		t.Errorf("down should scroll the viewer, offset = %d", m.viewer.offset)
	}
}

func TestKeys_EditWithoutAnEditorReportsIt(t *testing.T) {
	m := onSelector(t, 5, 80, 24)
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

func TestKeys_EditIsANoOpInTheViewerWithoutAFile(t *testing.T) {
	m := onViewer(t, nil, 80, 24)
	m.editorFound = true

	if _, cmd := send(t, m, "E"); cmd != nil {
		t.Error("E should do nothing when there is no file to open")
	}
}

func TestKeys_GitHubOnlyAppliesInTheViewer(t *testing.T) {
	// With no file on the selector, g falls through to key handling that ignores it.
	if _, cmd := send(t, onSelector(t, 5, 80, 24), "g"); cmd != nil {
		t.Error("g should do nothing on the commit selector")
	}

	// /tmp/repo is not a repository, so the lookup fails and the error must
	// surface rather than be swallowed.
	_, cmd := send(t, onViewer(t, sampleFiles(), 80, 24), "g")
	if cmd == nil {
		t.Fatal("g should return a status command in the viewer")
	}
	msg, ok := cmd().(statusMsg)
	if !ok {
		t.Fatalf("expected a statusMsg, got %T", cmd())
	}
	if !msg.isErr {
		t.Errorf("expected an error status, got %+v", msg)
	}
}

func TestKeys_GitHubWorksInLineSelectMode(t *testing.T) {
	inSelect, _ := send(t, onViewer(t, largeSampleFiles(), 80, 24), "enter")

	if _, cmd := send(t, inSelect, "g"); cmd == nil {
		t.Error("g should still open GitHub while in line-select mode")
	}
}

func TestKeys_ClearStaleStatusMessage(t *testing.T) {
	m := onSelector(t, 20, 80, 24)
	withStatus, _ := m.Update(statusMsg{text: "something happened", isErr: true})

	cleared, _ := send(t, withStatus.(Model), "down")
	if cleared.status != "" {
		t.Errorf("a navigation key should clear the stale status, got %q", cleared.status)
	}
	if strings.Contains(stripAnsi(cleared.View()), "something happened") {
		t.Error("the stale status should no longer be rendered")
	}
}

func TestStatusBar_LongMessagesDoNotChangeTheLayout(t *testing.T) {
	const width, height = 60, 20
	long := strings.Repeat("a very long error message that will not fit ", 5)

	for name, m := range map[string]Model{
		"selector": onSelector(t, 5, width, height),
		"viewer":   onViewer(t, sampleFiles(), width, height),
	} {
		shown, _ := m.Update(statusMsg{text: long, isErr: true})
		view := shown.(Model).View()

		if got := len(strings.Split(view, "\n")); got != height {
			t.Errorf("%s: a long status changed the view height to %d, want %d", name, got, height)
		}
		if !strings.Contains(stripAnsi(view), long[:20]) {
			t.Errorf("%s: the status message should be visible", name)
		}
	}
}

func TestStatusBar_LongRepoLabelDoesNotChangeTheLayout(t *testing.T) {
	const width, height = 60, 20
	m, _ := NewModel("/tmp/repo", "repo ("+strings.Repeat("long-branch-name-", 10)+")").
		Update(tea.WindowSizeMsg{Width: width, Height: height})
	loaded, _ := m.(Model).Update(commitListMsg{commits: sampleCommits(5), forkPointIdx: -1})

	view := loaded.(Model).View()
	if got := len(strings.Split(view, "\n")); got != height {
		t.Errorf("a long repo label changed the view height to %d, want %d", got, height)
	}
	for i, line := range strings.Split(view, "\n")[:height-1] {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d is %d columns wide, want %d", i, w, width)
		}
	}
}

// ── Commit range labels ──────────────────────────────────────────────────────

func TestCommitRangeLabel(t *testing.T) {
	a := git.CommitEntry{Hash: "abcdef0123"}
	b := git.CommitEntry{Hash: "9876543210"}

	if got := commitRangeLabel(a, a); got != "Commit abcd" {
		t.Errorf("single-commit label = %q, want %q", got, "Commit abcd")
	}
	if got := commitRangeLabel(a, b); got != "Commits abcd to 9876" {
		t.Errorf("range label = %q, want %q", got, "Commits abcd to 9876")
	}
	// Named outright when alone, but keeping its hash inside a range.
	uncommitted := git.CommitEntry{Hash: git.UncommittedHash}
	if got := commitRangeLabel(uncommitted, uncommitted); got != "Uncommitted changes" {
		t.Errorf("uncommitted label = %q, want %q", got, "Uncommitted changes")
	}
	if got := commitRangeLabel(a, uncommitted); got != "Commits abcd to ????" {
		t.Errorf("range-to-uncommitted label = %q, want %q", got, "Commits abcd to ????")
	}
}

// ── Cursor ───────────────────────────────────────────────────────────────────

// Bubble Tea hides the cursor once, at startup, so every child process that
// exits is an opportunity for the app to say it again.
func TestChildProcessExit_HidesTheCursorAgain(t *testing.T) {
	m := onSelector(t, 5, 100, 30)

	for name, msg := range map[string]tea.Msg{
		"commit list": commitListMsg{commits: sampleCommits(3), forkPointIdx: -1},
		"list error":  commitListMsg{err: "fatal: bad object"},
		"stat":        showStatMsg{hash: "abcd", output: "a stat"},
		"diff":        diffContentMsg{files: sampleFiles()},
		"diff error":  diffContentMsg{err: "fatal: bad object"},
		"child exit":  childExitedMsg{},
	} {
		_, cmd := m.Update(msg)
		if !hidesCursor(cmd) {
			t.Errorf("%s: the cursor should be hidden again after a child process exits", name)
		}
	}
}

// The commit list also has to keep fetching the preview for the cursor's row.
func TestChildProcessExit_KeepsTheFollowUpCommand(t *testing.T) {
	m := onSelector(t, 5, 100, 30)
	_, cmd := m.Update(commitListMsg{commits: sampleCommits(3), forkPointIdx: -1})

	if _, ok := findMsg[showStatMsg](cmd); !ok {
		t.Error("loading the commit list should still fetch the first preview")
	}
}

// hidesCursor reports whether cmd emits a hide-cursor message, on its own or in
// a batch.
func hidesCursor(cmd tea.Cmd) bool {
	return walkMsgs(cmd, func(msg tea.Msg) bool { return msg == tea.HideCursor() })
}

// findMsg runs cmd, descending into batches, and returns the first message of
// type T that it produces.
func findMsg[T tea.Msg](cmd tea.Cmd) (T, bool) {
	var found T
	ok := walkMsgs(cmd, func(msg tea.Msg) bool {
		typed, is := msg.(T)
		if is {
			found = typed
		}
		return is
	})
	return found, ok
}

// walkMsgs runs cmd and every command in the batches it produces, stopping as
// soon as match accepts a message.
func walkMsgs(cmd tea.Cmd, match func(tea.Msg) bool) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if walkMsgs(sub, match) {
				return true
			}
		}
		return false
	}
	return match(msg)
}
