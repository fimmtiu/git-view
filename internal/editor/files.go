package editor

import "os/exec"

// CommandFiles opens every file in one editor invocation, with no line positions.
func (e Editor) CommandFiles(dir string, files ...string) *exec.Cmd {
	args := append(e.ArgsWithoutLinePositioning(), files...)
	cmd := exec.Command(e.Argv[0], args...)
	cmd.Dir = dir
	return cmd
}
