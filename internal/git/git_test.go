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

// run executes a git command in dir, failing the test on error.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// commits returns the repo's commit list, failing the test on error.
func commits(t *testing.T, dir string) []CommitEntry {
	t.Helper()
	list, err := FetchCommitList(dir, 100)
	if err != nil {
		t.Fatalf("FetchCommitList: %v", err)
	}
	return list
}

func TestFetchCommitList(t *testing.T) {
	list := commits(t, newRepo(t))

	if len(list) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(list))
	}
	// Newest first.
	if list[0].Message != "second commit" {
		t.Errorf("first entry = %q, want the newest commit", list[0].Message)
	}
	if list[1].Message != "first commit" {
		t.Errorf("second entry = %q, want the oldest commit", list[1].Message)
	}
	for _, c := range list {
		if len(c.Hash) != 40 {
			t.Errorf("expected a full hash, got %q", c.Hash)
		}
	}
}

func TestFetchCommitList_RespectsLimit(t *testing.T) {
	list, err := FetchCommitList(newRepo(t), 1)
	if err != nil {
		t.Fatalf("FetchCommitList: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 commit, got %d", len(list))
	}
}

func TestFetchCommitList_OmitsMergeCommits(t *testing.T) {
	dir := newRepo(t)
	run(t, dir, "checkout", "-b", "side", "HEAD~1")
	if err := os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "-c", "user.name=T", "-c", "user.email=t@e.com", "commit", "-m", "side commit")
	run(t, dir, "checkout", "main")
	run(t, dir, "-c", "user.name=T", "-c", "user.email=t@e.com", "merge", "--no-ff", "side", "-m", "merge side")

	for _, c := range commits(t, dir) {
		if strings.HasPrefix(c.Message, "merge ") {
			t.Errorf("merge commit should be omitted, found %q", c.Message)
		}
	}
}

func TestFetchCommitList_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "--initial-branch=main")

	// A repo with no commits must not error; the selector shows "No commits".
	if list, err := FetchCommitList(dir, 100); err == nil && len(list) != 0 {
		t.Errorf("expected no commits, got %d", len(list))
	}
}

func TestForkPointIndex_OnABranch(t *testing.T) {
	dir := newRepo(t)
	run(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("f\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "-c", "user.name=T", "-c", "user.email=t@e.com", "commit", "-m", "feature commit")

	list := commits(t, dir)
	// Newest first: [feature commit, second commit, first commit]. The branch
	// forked from main at "second commit", index 1.
	if got := ForkPointIndex(dir, list); got != 1 {
		t.Errorf("ForkPointIndex = %d, want 1 (%q)", got, list[1].Message)
	}
}

func TestForkPointIndex_OnTheDefaultBranch(t *testing.T) {
	dir := newRepo(t)

	// On main, the merge base with main is HEAD itself: the newest commit.
	if got := ForkPointIndex(dir, commits(t, dir)); got != 0 {
		t.Errorf("ForkPointIndex = %d, want 0", got)
	}
}

func TestForkPointIndex_ForkPointOutsideTheList(t *testing.T) {
	dir := newRepo(t)
	run(t, dir, "checkout", "-b", "feature")

	// Only the newest commit is in the list, but the fork point is older.
	list, err := FetchCommitList(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := ForkPointIndex(dir, list); got != 0 && got != -1 {
		t.Errorf("ForkPointIndex = %d, want 0 or -1 when the fork point is out of range", got)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	dir := newRepo(t)

	dirty, err := HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if dirty {
		t.Error("a freshly committed repo should be clean")
	}

	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err = HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !dirty {
		t.Error("a modified tracked file should count as uncommitted changes")
	}
}

func TestHasUncommittedChanges_IgnoresUntrackedFiles(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	// FetchDiff runs plain `git diff`, which ignores untracked files, so
	// offering the pseudo-commit here would open an empty diff.
	if dirty {
		t.Error("untracked files should not count as uncommitted changes")
	}
}

func TestFetchShowStat_Commit(t *testing.T) {
	dir := newRepo(t)
	head := commits(t, dir)[0]

	out, err := FetchShowStat(dir, head.Hash)
	if err != nil {
		t.Fatalf("FetchShowStat: %v", err)
	}
	if !strings.HasPrefix(out, "second commit") {
		t.Errorf("output should start with the commit subject, got %q", out)
	}
	if !strings.Contains(out, "two.txt") || !strings.Contains(out, "files changed") {
		t.Errorf("expected a --stat listing, got %q", out)
	}
}

func TestFetchShowStat_Uncommitted(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := FetchShowStat(dir, UncommittedHash)
	if err != nil {
		t.Fatalf("FetchShowStat: %v", err)
	}
	// A leading blank line stands in for the missing commit subject, so the
	// preview pane's "bold the first line" rule does not bold a stat row.
	if !strings.HasPrefix(out, "\n") {
		t.Errorf("expected a leading blank line, got %q", out)
	}
	if !strings.Contains(out, "one.txt") {
		t.Errorf("expected the changed file, got %q", out)
	}
}

func TestFetchDiff_Range(t *testing.T) {
	dir := newRepo(t)
	list := commits(t, dir)

	// The range covers both commits, so the diff runs from the empty tree.
	out, err := FetchDiff(dir, list[1], list[0])
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	for _, want := range []string{"one.txt", "two.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %s in the range diff, got %q", want, out)
		}
	}
}

func TestFetchDiff_SingleCommitIncludesItsOwnChanges(t *testing.T) {
	dir := newRepo(t)
	head := commits(t, dir)[0]

	out, err := FetchDiff(dir, head, head)
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	if !strings.Contains(out, "two.txt") {
		t.Errorf("expected the commit's own changes, got %q", out)
	}
	if !strings.Contains(out, "+second") {
		t.Errorf("expected the commit's added line, got %q", out)
	}
}

func TestFetchDiff_RootCommit(t *testing.T) {
	dir := newRepo(t)
	root := commits(t, dir)[1]

	// The root commit has no parent, so "<root>^.." is not a range git accepts;
	// it has to be diffed against the empty tree instead.
	out, err := FetchDiff(dir, root, root)
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	if !strings.Contains(out, "one.txt") {
		t.Errorf("expected the root commit's files, got %q", out)
	}
	if !strings.Contains(out, "+first") {
		t.Errorf("expected the root commit's contents as additions, got %q", out)
	}
}

func TestFetchDiff_UncommittedOnly(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	uncommitted := CommitEntry{Hash: UncommittedHash}

	out, err := FetchDiff(dir, uncommitted, uncommitted)
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	if !strings.Contains(out, "+third") {
		t.Errorf("expected the working tree change, got %q", out)
	}
}

func TestFetchDiff_CommitThroughUncommitted(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("first\nsecond\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := commits(t, dir)

	out, err := FetchDiff(dir, list[0], CommitEntry{Hash: UncommittedHash})
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	if !strings.Contains(out, "+third") {
		t.Errorf("expected the working tree change, got %q", out)
	}
}

func TestFetchDiff_BadRevisionReportsGitStderr(t *testing.T) {
	bogus := CommitEntry{Hash: "no-such-revision"}

	_, err := FetchDiff(newRepo(t), bogus, bogus)
	if err == nil {
		t.Fatal("expected an error for an unknown revision")
	}
	// exec.Cmd.Output() hides stderr in the error, so a bare "exit status 128"
	// would leave the user with no idea what went wrong.
	if !strings.Contains(err.Error(), "no-such-revision") {
		t.Errorf("error should include git's own message, got %q", err)
	}
}

func TestRepoLabel_UsesTheOriginRemoteName(t *testing.T) {
	dir := newRepo(t)
	run(t, dir, "remote", "add", "origin", "git@github.com:fimmtiu/git-view.git")

	if got, want := RepoLabel(dir), "📁 git-view ⎇  main"; got != want {
		t.Errorf("RepoLabel = %q, want %q", got, want)
	}
}

func TestRepoLabel_FallsBackToTheDirectoryName(t *testing.T) {
	dir := newRepo(t)

	// With no origin remote, the working tree's own directory name stands in.
	got := RepoLabel(dir)
	if !strings.HasPrefix(got, "📁 "+filepath.Base(dir)) {
		t.Errorf("RepoLabel = %q, want it to name the directory %q", got, filepath.Base(dir))
	}
	if !strings.HasSuffix(got, "main") {
		t.Errorf("RepoLabel = %q, want it to end with the branch name", got)
	}
}

func TestRepoLabel_DetachedHead(t *testing.T) {
	dir := newRepo(t)
	run(t, dir, "checkout", "--detach", "HEAD")

	if got := RepoLabel(dir); !strings.Contains(got, "detached at ") {
		t.Errorf("RepoLabel = %q, want a detached-HEAD label", got)
	}
}

func TestRepoNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/fimmtiu/git-view.git":  "git-view",
		"https://github.com/fimmtiu/git-view":      "git-view",
		"https://github.com/fimmtiu/git-view.git/": "git-view",
		"git@github.com:fimmtiu/git-view.git":      "git-view",
		"git@github.com:fimmtiu/git-view":          "git-view",
		"ssh://git@host:2222/~user/repo.git/":      "repo",
		"git@gitlab.com:group/subgroup/thing.git":  "thing",
		"https://user:pass@example.com/a/b/c.git":  "c",
		"/srv/git/repo.git":                        "repo",
		"file:///srv/git/repo":                     "repo",
		"repo.git":                                 "repo",
		"":                                         "",
	}

	for url, want := range cases {
		if got := repoNameFromURL(url); got != want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestDetectDefaultBranch(t *testing.T) {
	if got := DetectDefaultBranch(newRepo(t)); got != "main" {
		t.Errorf("DetectDefaultBranch = %q, want main", got)
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
