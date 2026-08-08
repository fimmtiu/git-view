package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a temporary git repository with two commits and returns its
// path. The first commit adds "one.txt", the second modifies it and adds
// "two.txt", so range diffs have something to show.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "--initial-branch=main")
	write("one.txt", "first\n")
	run("add", ".")
	run("commit", "-m", "first commit")

	write("one.txt", "first\nsecond\n")
	write("two.txt", "hello\n")
	run("add", ".")
	run("commit", "-m", "second commit")

	return dir
}

func TestRepoRoot(t *testing.T) {
	dir := newRepo(t)
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := RepoRoot(sub)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	// macOS reports /private/var for /var, so compare resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(root)
	if gotResolved != wantResolved {
		t.Errorf("RepoRoot = %q, want %q", gotResolved, wantResolved)
	}
}

func TestRepoRoot_NotARepo(t *testing.T) {
	if _, err := RepoRoot(t.TempDir()); err == nil {
		t.Error("expected an error outside a git repository")
	}
}

func TestDiff_WorkingTree(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := Diff(dir, nil)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "diff --git a/one.txt b/one.txt") {
		t.Errorf("expected a diff for one.txt, got %q", out)
	}
	if !strings.Contains(out, "+third") {
		t.Errorf("expected the added line, got %q", out)
	}
}

func TestDiff_CleanWorkingTreeIsEmpty(t *testing.T) {
	out, err := Diff(newRepo(t), nil)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output for a clean tree, got %q", out)
	}
}

func TestDiff_Range(t *testing.T) {
	dir := newRepo(t)

	out, err := Diff(dir, []string{"HEAD~1..HEAD"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "two.txt") {
		t.Errorf("expected two.txt in the range diff, got %q", out)
	}
	if !strings.Contains(out, "+second") {
		t.Errorf("expected the added line, got %q", out)
	}
}

func TestDiff_SingleCommitShorthand(t *testing.T) {
	dir := newRepo(t)

	out, err := Diff(dir, []string{"HEAD^!"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "two.txt") {
		t.Errorf("expected the commit's own changes, got %q", out)
	}
}

func TestDiff_Staged(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "three.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "three.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	out, err := Diff(dir, []string{"--cached"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "three.txt") {
		t.Errorf("expected the staged file, got %q", out)
	}
}

func TestDiff_BadRevisionReportsGitStderr(t *testing.T) {
	_, err := Diff(newRepo(t), []string{"no-such-revision"})
	if err == nil {
		t.Fatal("expected an error for an unknown revision")
	}
	// exec.Cmd.Output() hides stderr in the error, so a bare "exit status 128"
	// would leave the user with no idea what went wrong.
	if !strings.Contains(err.Error(), "no-such-revision") {
		t.Errorf("error should include git's own message, got %q", err)
	}
}

func TestDescribeRevs(t *testing.T) {
	dir := newRepo(t)
	head, err := Output(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := Output(dir, "rev-parse", "--short", "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		args []string
		want string
	}{
		{nil, "working tree"},
		{[]string{"HEAD"}, head},
		{[]string{"HEAD~1..HEAD"}, parent + ".." + head},
		{[]string{"HEAD~1...HEAD"}, parent + "..." + head},
		{[]string{"HEAD~1", "HEAD"}, parent + " " + head},
		{[]string{"--cached"}, "--cached"},
		{[]string{"no-such-revision"}, "no-such-revision"},
	}

	for _, tc := range cases {
		if got := DescribeRevs(dir, tc.args); got != tc.want {
			t.Errorf("DescribeRevs(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestExtractGitHubRepo(t *testing.T) {
	cases := map[string]string{
		"git@github.com:fimmtiu/git-view.git":        "fimmtiu/git-view",
		"git@github.com:fimmtiu/git-view":            "fimmtiu/git-view",
		"https://github.com/fimmtiu/git-view.git":    "fimmtiu/git-view",
		"https://github.com/fimmtiu/git-view":        "fimmtiu/git-view",
		"https://github.com/fimmtiu/git-view/":       "fimmtiu/git-view",
		"ssh://git@github.com/fimmtiu/git-view.git":  "fimmtiu/git-view",
		"git@gitlab.com:fimmtiu/git-view.git":        "",
		"https://bitbucket.org/fimmtiu/git-view.git": "",
		"": "",
	}

	for url, want := range cases {
		if got := extractGitHubRepo(url); got != want {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestGitHubFileURL(t *testing.T) {
	dir := newRepo(t)
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@github.com:fimmtiu/git-view.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	url, err := GitHubFileURL(dir, "internal/ui/app.go", 0)
	if err != nil {
		t.Fatalf("GitHubFileURL: %v", err)
	}
	if want := "https://github.com/fimmtiu/git-view/blob/main/internal/ui/app.go"; url != want {
		t.Errorf("URL = %q, want %q", url, want)
	}

	withLine, err := GitHubFileURL(dir, "internal/ui/app.go", 42)
	if err != nil {
		t.Fatalf("GitHubFileURL: %v", err)
	}
	if want := "https://github.com/fimmtiu/git-view/blob/main/internal/ui/app.go#L42"; withLine != want {
		t.Errorf("URL = %q, want %q", withLine, want)
	}
}

func TestGitHubFileURL_DetachedHeadUsesCommitHash(t *testing.T) {
	dir := newRepo(t)
	for _, args := range [][]string{
		{"remote", "add", "origin", "https://github.com/fimmtiu/git-view.git"},
		{"checkout", "--detach", "HEAD"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	head, err := Output(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	url, err := GitHubFileURL(dir, "one.txt", 3)
	if err != nil {
		t.Fatalf("GitHubFileURL: %v", err)
	}
	if want := "https://github.com/fimmtiu/git-view/blob/" + head + "/one.txt#L3"; url != want {
		t.Errorf("URL = %q, want %q", url, want)
	}
}

func TestGitHubFileURL_NoOrigin(t *testing.T) {
	if _, err := GitHubFileURL(newRepo(t), "one.txt", 1); err == nil {
		t.Error("expected an error when the repo has no origin remote")
	}
}

func TestGitHubFileURL_NonGitHubOrigin(t *testing.T) {
	dir := newRepo(t)
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin",
		"git@gitlab.com:fimmtiu/git-view.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	_, err := GitHubFileURL(dir, "one.txt", 1)
	if err == nil {
		t.Fatal("expected an error for a non-GitHub origin")
	}
	if !strings.Contains(err.Error(), "github.com") {
		t.Errorf("error should explain the remote is not GitHub, got %q", err)
	}
}
