// Package review runs the Claude jobs behind git-view's code review and fix commands, and owns the .gv-review directory they write to.
package review



const (
	Dir          = ".gv-review"
	SummaryFile  = "summary.md"
	FindingsFile = "code-review-findings.md"
)

func SummaryPath(repoRoot string) string { panic("scaffold stub") }

func FindingsPath(repoRoot string) string { panic("scaffold stub") }

// Prepare creates Dir and removes old outputs, so a missing file after a run means the job did not write it.
func Prepare(repoRoot string) error { panic("scaffold stub") }

func FindingsExist(repoRoot string) bool { panic("scaffold stub") }

// MissingOutputs lists, repo-relative and summary first, the outputs that do not exist.
func MissingOutputs(repoRoot string) []string { panic("scaffold stub") }
