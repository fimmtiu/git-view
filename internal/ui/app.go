// Package ui implements the full-screen terminal diff viewer.
package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/git-view/internal/diff"
	"github.com/fimmtiu/git-view/internal/editor"
	"github.com/fimmtiu/git-view/internal/git"
)

// Layout rows consumed by chrome outside the content panes:
//   - statusBarRows: the status bar's own top border plus its text line.
//   - paneBorderRows: the content pane's top and bottom borders.
//   - hintBarRows: the help text at the bottom of the screen.
//
// The selector's status bar is one line and the viewer's is two, so the viewer
// gets one row less of content.
const (
	statusBarRows       = 2
	viewerStatusBarRows = 3
	paneBorderRows      = 2
	hintBarRows         = 1
)

// paneSideBorders is the number of columns a pane border consumes.
const paneSideBorders = 2

// ── Messages ─────────────────────────────────────────────────────────────────

// commitListMsg carries the result of the async commit-list fetch.
type commitListMsg struct {
	commits        []git.CommitEntry
	forkPointIdx   int
	hasUncommitted bool
	err            string
}

// showStatMsg carries `git show --stat` output for one commit.
type showStatMsg struct {
	hash   string
	output string
}

// diffContentMsg carries the parsed diff that opens the viewer screen.
type diffContentMsg struct {
	files []diff.File
	err   string
}

// statusMsg carries a transient message for the status bar, replacing the
// right-hand side until the next keystroke. It is how actions that leave the
// TUI — opening an editor or a browser — report that they failed.
type statusMsg struct {
	text  string
	isErr bool
}

// ── Model ────────────────────────────────────────────────────────────────────

// Model is the root bubbletea model. It owns two screens: the commit selector,
// and the diff viewer that opens on top of it. The viewer is active exactly
// when viewer is non-nil.
type Model struct {
	width  int
	height int

	repoRoot  string
	repoLabel string

	selector selector
	viewer   *viewer

	// The commit range the viewer is showing, for its status bar.
	viewStart git.CommitEntry
	viewEnd   git.CommitEntry

	// editor is the resolved editor; editorFound records whether one was found
	// at startup, so the E key can explain itself instead of doing nothing.
	editor      editor.Editor
	editorFound bool

	// errorMsg is a sticky error from loading commits or a diff.
	errorMsg string

	// status is a transient message shown in place of the status bar's
	// right-hand side.
	status      string
	statusIsErr bool

	quitting bool
}

// NewModel builds the root model for the repository at repoRoot.
func NewModel(repoRoot, repoLabel string) Model {
	ed, found := editor.Resolve()
	return Model{
		repoRoot:    repoRoot,
		repoLabel:   repoLabel,
		editor:      ed,
		editorFound: found,
	}
}

// Init kicks off the commit-list fetch that populates the selector.
func (m Model) Init() tea.Cmd {
	return fetchCommitsCmd(m.repoRoot)
}

// ── Commands ─────────────────────────────────────────────────────────────────

// fetchCommitsCmd loads the commit list, the fork point, and whether the
// working tree is dirty.
func fetchCommitsCmd(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		commits, err := git.FetchCommitList(repoRoot, maxCommits)
		if err != nil {
			return commitListMsg{forkPointIdx: -1, err: err.Error()}
		}
		hasUncommitted, _ := git.HasUncommittedChanges(repoRoot)
		return commitListMsg{
			commits:        commits,
			forkPointIdx:   git.ForkPointIndex(repoRoot, commits),
			hasUncommitted: hasUncommitted,
		}
	}
}

// fetchStatCmd loads `git show --stat` for one commit.
func fetchStatCmd(repoRoot, hash string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.FetchShowStat(repoRoot, hash)
		if err != nil {
			if hash == git.UncommittedHash {
				return showStatMsg{hash: hash, output: "(no changes)"}
			}
			return showStatMsg{hash: hash, output: "(error)"}
		}
		return showStatMsg{hash: hash, output: out}
	}
}

// fetchDiffCmd loads and parses the diff for a commit range.
func fetchDiffCmd(repoRoot string, start, end git.CommitEntry) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.FetchDiff(repoRoot, start, end)
		if err != nil {
			return diffContentMsg{err: err.Error()}
		}
		return diffContentMsg{files: diff.Parse(raw)}
	}
}

// statusCmd returns a command that posts a transient status-bar message.
func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isErr: isErr}
	}
}

// ── Dimensions ───────────────────────────────────────────────────────────────

// contentWidth returns the columns available inside the screen's borders.
func (m Model) contentWidth() int {
	return max(m.width-paneSideBorders, 1)
}

// listHeight returns the content rows available to the selector's panes.
func (m Model) listHeight() int {
	return max(m.height-statusBarRows-paneBorderRows-hintBarRows, 1)
}

// viewerHeight returns the content rows available to the diff pane.
func (m Model) viewerHeight() int {
	return max(m.height-viewerStatusBarRows-paneBorderRows-hintBarRows, 1)
}

// applySize pushes the current dimensions into whichever screen is active.
func (m *Model) applySize() {
	m.selector.setSize(m.width, m.listHeight())
	if m.viewer != nil {
		m.viewer.setSize(m.contentWidth(), m.viewerHeight())
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applySize()
		return m, nil

	case commitListMsg:
		if msg.err != "" {
			m.errorMsg = "git error: " + msg.err
			return m, nil
		}
		m.errorMsg = ""
		m.selector.setRows(buildCommitRows(msg.commits, msg.forkPointIdx, msg.hasUncommitted))
		return m, m.fetchStatForCursor()

	case showStatMsg:
		m.selector.statHash = msg.hash
		m.selector.statOutput = msg.output
		return m, nil

	case diffContentMsg:
		if msg.err != "" {
			m.errorMsg = "git error: " + msg.err
			return m, nil
		}
		m.errorMsg = ""
		m.viewer = newViewer(msg.files, m.contentWidth(), m.viewerHeight())
		return m, nil

	case statusMsg:
		m.status = msg.text
		m.statusIsErr = msg.isErr
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey dispatches app-level keys, then routes the rest to the active screen.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "Q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "e", "E":
		return m.openEditor()
	case "g":
		if m.viewer != nil {
			return m.openGitHub()
		}
	}

	// Any other key clears a stale status message before the screen sees it.
	m.status = ""
	m.statusIsErr = false

	if m.viewer != nil {
		return m.handleViewerKey(msg)
	}
	return m.handleSelectorKey(msg)
}

// handleViewerKey routes a key to the diff viewer, closing it on the exit keys.
func (m Model) handleViewerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isViewerExitKey(m.viewer, msg) {
		m.viewer = nil
		return m, nil
	}
	m.viewer.handleKey(msg)
	return m, nil
}

// isViewerExitKey reports whether a key should close the viewer and return to
// the commit selector. In line-select mode only Tab exits, since Escape is
// needed to leave line-select itself.
func isViewerExitKey(v *viewer, msg tea.KeyMsg) bool {
	if v.lineSelectMode {
		return msg.String() == "tab"
	}
	switch msg.String() {
	case "tab", "esc":
		return true
	}
	return false
}

// handleSelectorKey moves the commit selection and opens the viewer.
func (m Model) handleSelectorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prevCursor, prevAnchor := m.selector.cursor, m.selector.anchor

	switch msg.String() {
	case "up", "k":
		m.selector.move(1, -1)
	case "down", "j":
		m.selector.move(1, 1)
	case "pgup", "b":
		m.selector.move(m.selector.listHeight, -1)
	case "pgdown", " ":
		m.selector.move(m.selector.listHeight, 1)
	case "home", "<":
		m.selector.move(len(m.selector.rows), -1)
	case "end", ">":
		m.selector.move(len(m.selector.rows), 1)
	case "shift+up":
		m.selector.extendRange(1, -1)
	case "shift+down":
		m.selector.extendRange(1, 1)
	case "shift+pgup":
		m.selector.extendRange(m.selector.listHeight, -1)
	case "shift+pgdown":
		m.selector.extendRange(m.selector.listHeight, 1)
	case "tab", "enter":
		return m.openViewer()
	default:
		return m, nil
	}

	// Refresh the preview only when the selection actually moved.
	if m.selector.cursor != prevCursor || m.selector.anchor != prevAnchor {
		return m, m.fetchStatForCursor()
	}
	return m, nil
}

// fetchStatForCursor returns a command to load the preview for the commit under
// the cursor, or nil when it is already cached.
func (m Model) fetchStatForCursor() tea.Cmd {
	c := m.selector.currentCommit()
	if c == nil || c.Hash == m.selector.statHash {
		return nil
	}
	return fetchStatCmd(m.repoRoot, c.Hash)
}

// openViewer starts loading the diff for the selected commit range.
func (m Model) openViewer() (tea.Model, tea.Cmd) {
	start, end, ok := m.selector.selectedRange()
	if !ok {
		return m, nil
	}
	m.viewStart, m.viewEnd = start, end
	return m, fetchDiffCmd(m.repoRoot, start, end)
}

// openEditor opens the current file in the resolved editor, positioned at the
// selected line when line-select mode is active. On the selector screen, where
// no file is in view, it opens the repository itself.
//
// GUI editors are launched detached; terminal editors take over the tty via
// tea.ExecProcess, which suspends and then restores the TUI.
func (m Model) openEditor() (tea.Model, tea.Cmd) {
	if !m.editorFound {
		return m, statusCmd("no editor found; set $EDITOR or $GIT_VIEW_EDITOR", true)
	}

	target, line := ".", 0
	if m.viewer != nil {
		file, lineNum := m.viewer.selectedLocation()
		if file == "" {
			return m, nil
		}
		target, line = file, lineNum
	}

	cmd := m.editor.Command(m.repoRoot, target, line)
	if m.editor.GUI {
		if err := cmd.Start(); err != nil {
			return m, statusCmd(fmt.Sprintf("editor failed: %s", err), true)
		}
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return statusMsg{text: fmt.Sprintf("editor failed: %s", err), isErr: true}
		}
		return statusMsg{}
	})
}

// openGitHub opens the current file's github.com page in the default browser,
// anchored at the selected line when line-select mode is active.
func (m Model) openGitHub() (tea.Model, tea.Cmd) {
	file, line := m.viewer.selectedLocation()
	if file == "" {
		return m, nil
	}
	url, err := git.GitHubFileURL(m.repoRoot, file, line)
	if err != nil {
		return m, statusCmd(err.Error(), true)
	}
	if err := exec.Command("open", url).Start(); err != nil {
		return m, statusCmd(fmt.Sprintf("open failed: %s", err), true)
	}
	return m, statusCmd("opened "+url, false)
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	innerW := m.contentWidth()
	var bar, panes string
	var hints []string

	if m.viewer != nil {
		bar = m.renderViewerStatusBar(innerW)
		panes = connectPaneCorners(m.viewer.renderPane(), true, true)
		hints = m.viewer.hintPairs()
	} else {
		bar = m.renderSelectorStatusBar(innerW)
		panes = m.selector.renderPanes()
		hints = m.selector.hintPairs()
	}

	statusBar := theme.StatusBarStyle.Width(innerW).Render(bar)
	// The hint bar's own padding eats two columns, so it fits into innerW.
	hint := theme.HintBarStyle.Render(buildHintFit(innerW, hints...))

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, panes, hint)
}

// renderSelectorStatusBar renders the selector's single status line: the repo
// and branch on the left, the selection size on the right.
func (m Model) renderSelectorStatusBar(width int) string {
	// A long branch name must not push the bar past its fixed width, or the
	// bordered style wraps it and the panes lose a row.
	left := theme.LabelBoldStyle.Render(truncateLine(m.repoLabel, max(width/2, 1)))

	// Whatever goes on the right shares the line with the label, so it gets
	// only the room left over.
	room := max(width-lipgloss.Width(left)-2, 0)

	var right string
	switch {
	case m.status != "":
		right = m.statusText(room)
	case m.errorMsg != "":
		right = theme.ErrorStyle.Render(truncateLine(m.errorMsg, room))
	default:
		noun := "commits"
		if n := m.selector.selectedCount(); n == 1 {
			noun = "commit"
		}
		right = fmt.Sprintf("%d %s selected", m.selector.selectedCount(), noun)
	}

	return joinEnds(left, right, width)
}

// renderViewerStatusBar renders the viewer's two status lines: the commit range
// and file position on the first, the current filename on the second.
func (m Model) renderViewerStatusBar(width int) string {
	total := len(m.viewer.fileNames)

	left1 := theme.LabelBoldStyle.Render(commitRangeLabel(m.viewStart, m.viewEnd))
	right1 := ""
	if total > 0 {
		right1 = fmt.Sprintf("File %d of %d", m.viewer.currentFileIndex()+1, total)
	}
	line1 := joinEnds(left1, right1, width)

	// A status message takes the whole of line 2; the filename yields to it.
	right2 := m.repoLabel
	if m.status != "" {
		right2 = m.statusText(width - 2)
	}
	left2 := leftTruncateFilename(m.viewer.currentFileName(), width-lipgloss.Width(right2)-2)
	line2 := joinEnds(left2, right2, width)

	return line1 + "\n" + line2
}

// statusText renders the transient status message, truncated to fit. The status
// bar must stay a fixed number of lines: it is rendered inside a fixed-width
// bordered style, which would otherwise wrap and push the panes off the bottom
// of the screen.
func (m Model) statusText(width int) string {
	text := truncateLine(m.status, width)
	if m.statusIsErr {
		return theme.ErrorStyle.Render(text)
	}
	return text
}

// joinEnds pads left and right apart to fill width, leaving at least two spaces
// between them.
func joinEnds(left, right string, width int) string {
	spacer := max(width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	return left + strings.Repeat(" ", spacer) + right
}

// commitRangeLabel describes the commits being viewed, e.g. "Commit a1b2" or
// "Commits a1b2 to c3d4". Viewing the uncommitted pseudo-commit on its own is
// named outright; inside a range it keeps its "????" hash, which lines up with
// how the commit list shows it.
func commitRangeLabel(start, end git.CommitEntry) string {
	if start.Hash == end.Hash {
		if end.Hash == git.UncommittedHash {
			return "Uncommitted changes"
		}
		return "Commit " + shortHash(end.Hash)
	}
	return "Commits " + shortHash(start.Hash) + " to " + shortHash(end.Hash)
}
