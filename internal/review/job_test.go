package review

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestJobSpec_Args(t *testing.T) {
	spec := JobSpec{
		Name:   "Test Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test prompt",
	}
	
	args := spec.Args()
	expected := []string{"--print", "--model", "sonnet", "--effort", "high"}
	
	if len(args) != len(expected) {
		t.Fatalf("Args() returned %d arguments, expected %d", len(args), len(expected))
	}
	
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("Args()[%d] = %q, expected %q", i, arg, expected[i])
		}
	}
}

func TestClaudeAvailable(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	// Test with existing executable
	claudeBinary = "sh"
	if !ClaudeAvailable() {
		t.Error("ClaudeAvailable() = false for 'sh', expected true")
	}
	
	// Test with non-existent executable
	claudeBinary = "definitely-not-a-real-executable-12345"
	if ClaudeAvailable() {
		t.Error("ClaudeAvailable() = true for missing executable, expected false")
	}
}

func TestStart_PassesArgumentsAndPrompt(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	argsFile := filepath.Join(tmpDir, "args.txt")
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	cwdFile := filepath.Join(tmpDir, "cwd.txt")
	
	// Create a script that records its arguments, stdin, and working directory
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" > %s
cat > %s
pwd > %s
`, argsFile, stdinFile, cwdFile)
	
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Test Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test prompt content",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}
	
	// Check arguments
	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	argsStr := strings.TrimSpace(string(argsData))
	expectedArgs := "--print --model sonnet --effort high"
	if argsStr != expectedArgs {
		t.Errorf("Arguments = %q, expected %q", argsStr, expectedArgs)
	}
	
	// Check stdin
	stdinData, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(stdinData) != spec.Prompt {
		t.Errorf("Stdin = %q, expected %q", string(stdinData), spec.Prompt)
	}
	
	// Check working directory
	cwdData, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatal(err)
	}
	cwd := strings.TrimSpace(string(cwdData))
	if cwd != repoRoot {
		t.Errorf("Working directory = %q, expected %q", cwd, repoRoot)
	}
}

func TestStart_StderrInError(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	// Create a script that writes to stderr and exits with error
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := `#!/bin/sh
echo "boom" >&2
exit 3
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Failing Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	err = proc.Wait()
	if err == nil {
		t.Fatal("Wait() succeeded, expected error")
	}
	
	errStr := err.Error()
	if !strings.Contains(errStr, "Failing Job") {
		t.Errorf("Error %q does not contain job name 'Failing Job'", errStr)
	}
	if !strings.Contains(errStr, "boom") {
		t.Errorf("Error %q does not contain stderr 'boom'", errStr)
	}
}

func TestCancel_StopsProcessGroup(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	childPidFile := filepath.Join(tmpDir, "child.pid")
	readyFile := filepath.Join(tmpDir, "ready")
	
	// Create a script that starts a child process and signals when ready
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := fmt.Sprintf(`#!/bin/sh
sleep 60 &
echo $! > %s
touch %s
sleep 60
`, childPidFile, readyFile)
	
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Parent Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// Wait for the script to be ready
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Read child PID
	pidData, err := os.ReadFile(childPidFile)
	if err != nil {
		t.Fatal(err)
	}
	childPid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	
	// Cancel the process
	done := make(chan error, 1)
	go func() {
		done <- proc.Wait()
	}()
	
	proc.Cancel()
	
	// Wait should return within 2 seconds
	select {
	case err := <-done:
		if !errors.Is(err, ErrCancelled) {
			t.Errorf("Wait() returned %v, expected ErrCancelled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() did not return within 2 seconds after Cancel")
	}
	
	// Give the OS a moment to clean up
	time.Sleep(100 * time.Millisecond)
	
	// Check that the child process is gone
	checkCmd := exec.Command("kill", "-0", strconv.Itoa(childPid))
	if err := checkCmd.Run(); err == nil {
		t.Error("Child process still exists after Cancel")
	}
}

func TestCancel_KillsAfterGracePeriod(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	readyFile := filepath.Join(tmpDir, "ready")
	
	// Create a shell script that ignores SIGTERM.
	// The key is to NOT use exec, so the shell process itself ignores SIGTERM
	// and keeps running until SIGKILL arrives.
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := fmt.Sprintf(`#!/bin/sh
trap '' TERM
touch %s
sleep 60
`, readyFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Stubborn Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// Wait for the script to be ready
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	start := time.Now()
	
	done := make(chan error, 1)
	go func() {
		done <- proc.Wait()
	}()
	
	proc.Cancel()
	
	// Wait should return after the grace period
	select {
	case err := <-done:
		elapsed := time.Since(start)
		if !errors.Is(err, ErrCancelled) {
			t.Errorf("Wait() returned %v, expected ErrCancelled", err)
		}
		// Should take at least the grace period (allowing some slack for scheduling)
		minTime := CancelGracePeriod - 200*time.Millisecond
		if elapsed < minTime {
			t.Errorf("Process killed too quickly: %v < %v", elapsed, minTime)
		}
		// But not much longer (allowing 1 second slack for slow systems)
		maxTime := CancelGracePeriod + time.Second
		if elapsed > maxTime {
			t.Errorf("Process took too long to kill: %v > %v", elapsed, maxTime)
		}
	case <-time.After(CancelGracePeriod + 2*time.Second):
		t.Fatal("Wait() did not return after grace period + 2 seconds")
	}
}

func TestCancel_SafeToCallMultipleTimes(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	readyFile := filepath.Join(tmpDir, "ready")
	
	// Create a script that runs for a bit
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := fmt.Sprintf(`#!/bin/sh
touch %s
sleep 5
`, readyFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Test Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// Wait for the script to be ready
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Start Wait in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- proc.Wait()
	}()
	
	// Call Cancel multiple times concurrently
	for i := 0; i < 5; i++ {
		go proc.Cancel()
	}
	
	// Wait should still work
	err = <-done
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("Wait() returned %v, expected ErrCancelled", err)
	}
	
	// Cancel after Wait should also be safe
	proc.Cancel()
}

func TestWait_ReturnsNilOnSuccess(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	// Create a successful script
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Success Job",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	if err := proc.Wait(); err != nil {
		t.Errorf("Wait() returned %v, expected nil", err)
	}
}

func TestStart_EmptyStderrNotInError(t *testing.T) {
	// Save original
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	// Create a script that exits with error but no stderr
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Silent Failure",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	err = proc.Wait()
	if err == nil {
		t.Fatal("Wait() succeeded, expected error")
	}
	
	errStr := err.Error()
	if !strings.Contains(errStr, "Silent Failure") {
		t.Errorf("Error %q does not contain job name", errStr)
	}
	// Should not have trailing ": " when stderr is empty
	if strings.HasSuffix(errStr, ": ") {
		t.Errorf("Error %q has trailing ': ' with empty stderr", errStr)
	}
}

func TestCancel_BeforeProcessStarts(t *testing.T) {
	// Test that calling Cancel immediately after Start is safe
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := `#!/bin/sh
sleep 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Quick Cancel",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// Cancel immediately
	proc.Cancel()
	
	err = proc.Wait()
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("Wait() returned %v, expected ErrCancelled", err)
	}
}

func TestWait_CalledOnce(t *testing.T) {
	// Test that Wait can only be called once
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := `#!/bin/sh
echo "test error" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "Double Wait",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// First Wait
	err1 := proc.Wait()
	if err1 == nil {
		t.Fatal("First Wait() succeeded, expected error")
	}
	
	// Second Wait should return the same error
	err2 := proc.Wait()
	if err2 == nil {
		t.Fatal("Second Wait() succeeded, expected error")
	}
	
	if err1.Error() != err2.Error() {
		t.Errorf("Second Wait() returned different error: %v vs %v", err1, err2)
	}
}

func TestCancel_SendsSignalToProcessGroup(t *testing.T) {
	// Verify that we're actually using process groups
	original := claudeBinary
	defer func() { claudeBinary = original }()
	
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	
	pgidFile := filepath.Join(tmpDir, "pgid.txt")
	readyFile := filepath.Join(tmpDir, "ready")
	
	// Script that writes its PGID
	scriptPath := filepath.Join(tmpDir, "fake-claude")
	script := fmt.Sprintf(`#!/bin/sh
ps -o pgid= -p $$ > %s
touch %s
sleep 60
`, pgidFile, readyFile)
	
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	
	claudeBinary = scriptPath
	
	spec := JobSpec{
		Name:   "PGID Test",
		Model:  "sonnet",
		Effort: "high",
		Prompt: "test",
	}
	
	proc, err := Start(repoRoot, spec)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	
	// Get the PID of the process
	p := proc.(*process)
	pid := p.cmd.Process.Pid
	
	// Wait for ready
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Read PGID
	pgidData, err := os.ReadFile(pgidFile)
	if err != nil {
		t.Fatal(err)
	}
	pgid, err := strconv.Atoi(strings.TrimSpace(string(pgidData)))
	if err != nil {
		t.Fatal(err)
	}
	
	// PGID should equal PID (the process is the group leader)
	if pgid != pid {
		t.Errorf("Process PGID %d != PID %d, process is not in its own group", pgid, pid)
	}
	
	proc.Cancel()
	proc.Wait()
}
