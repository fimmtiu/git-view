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

// Layout rows consumed by chrome outside the diff content:
//   - statusBarRows: the status bar's own top border plus its single text line.
//   - paneBorderRows: the diff pane's top and bottom borders.
//   - hintBarRows: the help text at the bottom of the screen.
const (
	statusBarRows  = 2
	paneBorderRows = 2
	hintBarRows    = 1
	chromeRows     = statusBarRows + paneBorderRows + hintBarRows
)

// paneSideBorders is the number of columns the pane border consumes.
const paneSideBorders = 2

// ── Messages ─────────────────────────────────────────────────────────────────

// statusMsg carries a transient message to show in the status bar, replacing
// the commit label until the next action. Used to report why opening an editor
// or a browser failed, since the app has nowhere else to print.
type statusMsg struct {
	text  string
	isErr bool
}

// ── Model ────────────────────────────────────────────────────────────────────

// Model is the root bubbletea model: a status bar, a full-screen diff pane,
// and a line of help text.
type Model struct {
	width  int
	height int

	repoRoot string
	revLabel string // human-readable description of what is being diffed

	viewer *viewer

	// editor is the resolved editor, and editorFound records whether one was
	// found at startup so the E key can explain itself instead of doing nothing.
	editor      editor.Editor
	editorFound bool

	// status holds a transient message shown in place of the commit label.
	status      string
	statusIsErr bool

	quitting bool
}

// NewModel builds the root model for the given parsed diff. repoRoot is used to
// resolve diff-relative paths for the editor and GitHub commands, and revLabel
// describes the revisions being viewed.
func NewModel(files []diff.File, repoRoot, revLabel string) Model {
	ed, found := editor.Resolve()
	return Model{
		repoRoot:    repoRoot,
		revLabel:    revLabel,
		viewer:      newViewer(files, 0, 0),
		editor:      ed,
		editorFound: found,
	}
}

// Init requests nothing; the diff is already loaded before the program starts.
func (m Model) Init() tea.Cmd {
	return nil
}

// ── Dimensions ───────────────────────────────────────────────────────────────

// paneHeight returns the number of diff content rows available.
func (m Model) paneHeight() int {
	h := m.height - chromeRows
	if h < 1 {
		h = 1
	}
	return h
}

// paneWidth returns the number of diff content columns available.
func (m Model) paneWidth() int {
	w := m.width - paneSideBorders
	if w < 1 {
		w = 1
	}
	return w
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewer.setSize(m.paneWidth(), m.paneHeight())
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

// handleKey dispatches app-level keys and passes everything else to the viewer.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "Q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "e", "E":
		return m.openEditor()
	case "g":
		return m.openGitHub()
	}

	// Any other key clears a stale status message before the viewer handles it.
	m.status = ""
	m.statusIsErr = false
	m.viewer.handleKey(msg)
	return m, nil
}

// openEditor opens the current file in the resolved editor, positioned at the
// selected line when line-select mode is active. GUI editors are launched
// detached; terminal editors take over the tty via tea.ExecProcess, which
// suspends and then restores the TUI.
func (m Model) openEditor() (tea.Model, tea.Cmd) {
	if !m.editorFound {
		return m, statusCmd("no editor found; set $EDITOR or $GIT_VIEW_EDITOR", true)
	}
	file, line := m.viewer.selectedLocation()
	if file == "" {
		return m, nil
	}

	cmd := m.editor.Command(m.repoRoot, file, line)
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

// statusCmd returns a command that posts a transient status-bar message.
func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isErr: isErr}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	innerW := m.paneWidth()
	statusBar := theme.StatusBarStyle.Width(innerW).Render(m.renderStatusBar(innerW))
	pane := connectPaneTop(m.viewer.renderPane())
	// The hint bar's own padding eats two columns, so it fits into innerW.
	hint := theme.HintBarStyle.Render(buildHintFit(innerW, m.viewer.hintPairs()...))

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, pane, hint)
}

// renderStatusBar renders the single status line: the current filename on the
// left, and "File X of Y" plus the revision label on the right. A pending
// status message takes over the whole bar, since messages are long enough that
// pairing one with a filename leaves room for neither.
//
// Everything is truncated to fit width. The bar must stay exactly one line: it
// is rendered inside a fixed-width bordered style, which would otherwise wrap
// and push the diff pane down off the bottom of the screen.
func (m Model) renderStatusBar(width int) string {
	if m.status != "" {
		text := truncateLine(m.status, width)
		if m.statusIsErr {
			return theme.ErrorStyle.Render(text)
		}
		return text
	}

	right := m.revLabel
	if total := len(m.viewer.fileNames); total > 0 {
		right = fmt.Sprintf("File %d of %d  •  %s", m.viewer.currentFileIndex()+1, total, m.revLabel)
	}
	right = truncateLine(right, width)

	left := theme.LabelBoldStyle.Render(
		leftTruncateFilename(m.viewer.currentFileName(), width-lipgloss.Width(right)-2))

	spacer := max(width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	return left + strings.Repeat(" ", spacer) + right
}
