package editor

import (
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

type Editor struct {
	Argv []string

	// GUI editors detach into their own window; terminal editors need the caller
	// to hand them the tty.
	GUI bool

	lineArg func(file string, line int) []string
}

// Keyed by base name, so "/usr/bin/vim" and "vim" both match.
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

var guiCandidates = []string{"cursor", "code", "zed"}

func gotoArg(file string, line int) []string {
	return []string{file + ":" + strconv.Itoa(line)}
}

func plusArg(file string, line int) []string {
	return []string{"+" + strconv.Itoa(line), file}
}

// Same syntax as gotoArg, but helix is not a GUI editor.
func colonArg(file string, line int) []string {
	return []string{file + ":" + strconv.Itoa(line)}
}

// Resolve picks an editor from GIT_VIEW_EDITOR, VISUAL, or EDITOR, falling back
// to the first GUI editor on PATH.
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

// fromSpec builds an Editor from a command that may carry arguments. Unknown
// editors are assumed to be terminal ones with no line-navigation syntax.
func fromSpec(spec string) Editor {
	fields := dropWait(strings.Fields(spec))
	base := baseName(fields[0])
	if p, ok := profiles[base]; ok {
		// The profile's own Argv is discarded so the user's flags survive.
		p.Argv = fields
		if p.lineArg != nil && !hasGotoFlag(fields, base) {
			p.Argv = append(p.Argv, gotoFlagFor(base)...)
		}
		return p
	}
	return Editor{Argv: fields}
}

// dropWait strips --wait: a GUI editor told to wait never returns to the caller,
// and a terminal editor already holds the tty for as long as it runs.
func dropWait(fields []string) []string {
	kept := fields[:0]
	for _, f := range fields {
		if f != "--wait" {
			kept = append(kept, f)
		}
	}
	return kept
}

func hasGotoFlag(fields []string, base string) bool {
	flag := gotoFlagFor(base)
	if len(flag) == 0 {
		return true
	}
	return slices.Contains(fields, flag[0])
}

// gotoFlagFor returns the flag that makes the editor read "file:line", or nil
// for editors needing none.
func gotoFlagFor(base string) []string {
	switch base {
	case "cursor", "code":
		return []string{"--goto"}
	}
	return nil
}

func baseName(cmd string) string {
	if idx := strings.LastIndexByte(cmd, '/'); idx >= 0 {
		return cmd[idx+1:]
	}
	return cmd
}

// Command opens file at line, or at the top when line is not positive. dir is
// the working directory that relative diff paths resolve against.
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
