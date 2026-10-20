package editor

import "os/exec"

// CommandFiles opens every file in one editor invocation, with no line positions.
func (e Editor) CommandFiles(dir string, files ...string) *exec.Cmd {
	args := append(e.ArgsWithoutLinePositioning(), files...)
	return e.newCommand(dir, args)
}
