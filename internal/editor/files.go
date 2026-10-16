package editor

import "os/exec"

// CommandFiles opens every file in one editor invocation, with no line positions.
// dir is the working directory.
func (e Editor) CommandFiles(dir string, files ...string) *exec.Cmd { panic("scaffold stub") }
