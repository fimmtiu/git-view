// Package editor resolves which editor to launch and how to point it at a
// specific file and line.
package editor

import (
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// Editor describes how to invoke a particular editor at a file:line location.
type Editor struct {
	// Argv is the command and its leading arguments, e.g. ["cursor", "--goto"].
	Argv []string

	// GUI is true for editors that detach into their own window. Terminal
	// editors must take over the tty instead, which the caller handles by
	// suspending the TUI for the duration of the process.
	GUI bool

	// lineArg formats the file and line into the trailing arguments.
	lineArg func(file string, line int) []string
}

// profiles maps known editor binaries to their invocation style. The key is
// matched against the base name of the resolved editor command, so "/usr/bin/vim"
// and "vim" both hit the same entry.
var profiles = map[string]Editor{
	"cursor": {Argv: []string{"cursor", "--goto"}, GUI: true, lineArg: gotoArg},
	"code":   {Argv: []string{"code", "--goto"}, GUI: true, lineArg: gotoArg},
	"zed":    {Argv: []string{"zed"}, GUI: true, lineArg: gotoArg},
	"subl":   {Argv: []string{"subl"}, GUI: true, lineArg: gotoArg},
	"vim":    {Argv: []string{"vim"}, lineArg: plusArg},
	"nvim":   {Argv: []string{"nvim"}, lineArg: plusArg},
	"vi":     {Argv: []string{"vi"}, lineArg: plusArg},
	"nano":   {Argv: []string{"nano"}, lineArg: plusArg},
	"emacs":  {Argv: []string{"emacs"}, lineArg: plusArg},
	"hx":     {Argv: []string{"hx"}, lineArg: colonArg},
	"helix":  {Argv: []string{"helix"}, lineArg: colonArg},
}

// guiCandidates are tried, in order, when no editor environment variable is set.
var guiCandidates = []string{"cursor", "code", "zed"}

// gotoArg formats "file:line", the form cursor/code/zed/subl accept.
func gotoArg(file string, line int) []string {
	return []string{file + ":" + strconv.Itoa(line)}
}

// plusArg formats "+line file", the form vi-family and emacs accept.
func plusArg(file string, line int) []string {
	return []string{"+" + strconv.Itoa(line), file}
}

// colonArg formats "file:line" for helix, which shares the syntax but not the
// GUI behaviour.
func colonArg(file string, line int) []string {
	return []string{file + ":" + strconv.Itoa(line)}
}

// Resolve picks an editor from GIT_VIEW_EDITOR, VISUAL, or EDITOR, falling back
// to the first GUI editor found on PATH. The environment variables may include
// arguments ("code --wait"), which are preserved. Returns false when no editor
// could be found.
func Resolve() (Editor, bool) {
	for _, env := range []string{"GIT_VIEW_EDITOR", "VISUAL", "EDITOR"} {
		if spec := strings.TrimSpace(os.Getenv(env)); spec != "" {
			return fromSpec(spec), true
		}
	}
	for _, name := range guiCandidates {
		if _, err := exec.LookPath(name); err == nil {
			return profiles[name], true
		}
	}
	return Editor{}, false
}

// fromSpec builds an Editor from a command string that may carry arguments.
// A known base name supplies the line-navigation style and GUI flag; unknown
// editors are treated as terminal editors opened at the top of the file.
func fromSpec(spec string) Editor {
	fields := strings.Fields(spec)
	base := baseName(fields[0])
	if p, ok := profiles[base]; ok {
		// Keep the user's own command and flags, but adopt the known
		// line-navigation style. The profile's own Argv is discarded so
		// "code --wait" does not become "code --goto --wait".
		p.Argv = fields
		if p.lineArg != nil && !hasGotoFlag(fields, base) {
			p.Argv = append(p.Argv, gotoFlagFor(base)...)
		}
		return p
	}
	return Editor{Argv: fields}
}

// hasGotoFlag reports whether the user's command already contains the flag that
// makes the editor interpret a "file:line" argument.
func hasGotoFlag(fields []string, base string) bool {
	flag := gotoFlagFor(base)
	if len(flag) == 0 {
		return true
	}
	return slices.Contains(fields, flag[0])
}

// gotoFlagFor returns the flag needed for file:line navigation, or nil for
// editors that need none.
func gotoFlagFor(base string) []string {
	switch base {
	case "cursor", "code":
		return []string{"--goto"}
	}
	return nil
}

// baseName strips any directory prefix from a command path.
func baseName(cmd string) string {
	if idx := strings.LastIndexByte(cmd, '/'); idx >= 0 {
		return cmd[idx+1:]
	}
	return cmd
}

// Command builds the exec.Cmd that opens file at line. A line of zero or less
// opens the file without positioning the cursor. dir becomes the process's
// working directory so relative diff paths resolve against the repo root.
func (e Editor) Command(dir, file string, line int) *exec.Cmd {
	args := append([]string{}, e.Argv[1:]...)
	switch {
	case line > 0 && e.lineArg != nil:
		args = append(args, e.lineArg(file, line)...)
	default:
		args = append(args, file)
	}
	cmd := exec.Command(e.Argv[0], args...)
	cmd.Dir = dir
	return cmd
}
