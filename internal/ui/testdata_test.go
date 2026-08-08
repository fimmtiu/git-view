package ui

import (
	"strconv"

	"github.com/fimmtiu/git-view/internal/diff"
	"github.com/fimmtiu/git-view/internal/git"
)

// sampleFiles returns a small two-file diff used across the UI tests.
func sampleFiles() []diff.File {
	return []diff.File{
		{
			Name: "internal/ui/app.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func main()",
					NewStart: 10,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "fmt.Println(\"hello\")"},
						{Type: diff.LineRemoved, Content: "fmt.Println(\"old\")"},
						{Type: diff.LineAdded, Content: "fmt.Println(\"new\")"},
					},
				},
			},
		},
		{
			Name: "internal/db/project_context_test.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func TestFoo()",
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "package db"},
						{Type: diff.LineAdded, Content: ""},
					},
				},
			},
		},
	}
}

// largeSampleFiles returns a diff long enough to require scrolling at any
// reasonable pane height.
func largeSampleFiles() []diff.File {
	var lines []diff.Line
	for range 30 {
		lines = append(lines, diff.Line{Type: diff.LineAdded, Content: "line content"})
	}
	return []diff.File{
		{
			Name:  "first_file.go",
			Type:  diff.Normal,
			Hunks: []diff.Hunk{{Context: "func A()", NewStart: 1, NewCount: 30, Lines: lines}},
		},
		{
			Name:  "second_file.go",
			Type:  diff.Normal,
			Hunks: []diff.Hunk{{Context: "func B()", NewStart: 1, NewCount: 30, Lines: lines}},
		},
	}
}

// sampleCommits returns n commits, newest first, with predictable hashes:
// commit i has hash "c<i>" padded to 8 characters.
func sampleCommits(n int) []git.CommitEntry {
	commits := make([]git.CommitEntry, n)
	for i := range n {
		id := strconv.Itoa(i)
		commits[i] = git.CommitEntry{
			Hash:    "c" + id + "abcdef0123456789"[:7-len(id)] + id,
			Message: "commit message " + id,
		}
	}
	return commits
}
