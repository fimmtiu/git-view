package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewPrompt(t *testing.T) {
	got := ReviewPrompt("aaa111", "bbb222")
	want, err := os.ReadFile("testdata/review_prompt.txt")
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if got != string(want) {
		t.Errorf("ReviewPrompt mismatch\ngot:\n%s\nwant:\n%s", got, string(want))
	}

	// Verify placeholders are replaced
	if strings.Contains(got, "<base commit SHA>") {
		t.Error("ReviewPrompt contains unreplaced <base commit SHA>")
	}
	if strings.Contains(got, "<head commit SHA>") {
		t.Error("ReviewPrompt contains unreplaced <head commit SHA>")
	}
	if strings.Contains(got, "<headsha>") {
		t.Error("ReviewPrompt contains unreplaced <headsha>")
	}

	// Verify substitutions are present
	if !strings.Contains(got, "Base commit: aaa111") {
		t.Error("ReviewPrompt missing 'Base commit: aaa111'")
	}
	if !strings.Contains(got, "HEAD commit: bbb222") {
		t.Error("ReviewPrompt missing 'HEAD commit: bbb222'")
	}
	if !strings.Contains(got, "blob/bbb222/path#Lnn") {
		t.Error("ReviewPrompt missing 'blob/bbb222/path#Lnn'")
	}
}

func TestSummaryPrompt(t *testing.T) {
	got := SummaryPrompt()
	want, err := os.ReadFile("testdata/summary_prompt.txt")
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if got != string(want) {
		t.Errorf("SummaryPrompt mismatch\ngot:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestFixPrompt(t *testing.T) {
	got := FixPrompt()
	want, err := os.ReadFile("testdata/fix_prompt.txt")
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if got != string(want) {
		t.Errorf("FixPrompt mismatch\ngot:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestPromptFileReferences(t *testing.T) {
	summary := SummaryPrompt()
	fix := FixPrompt()

	summaryPath := filepath.Join(Dir, SummaryFile)
	findingsPath := filepath.Join(Dir, FindingsFile)

	if !strings.Contains(summary, summaryPath) {
		t.Errorf("SummaryPrompt does not mention %s", summaryPath)
	}
	if !strings.Contains(fix, findingsPath) {
		t.Errorf("FixPrompt does not mention %s", findingsPath)
	}

	review := ReviewPrompt("base", "head")
	if !strings.Contains(review, findingsPath) {
		t.Errorf("ReviewPrompt does not mention %s", findingsPath)
	}
}

func TestReviewJobs(t *testing.T) {
	jobs := ReviewJobs("basesha", "headsha")
	if len(jobs) != 2 {
		t.Fatalf("ReviewJobs returned %d jobs, want 2", len(jobs))
	}

	// First job should be the review
	if jobs[0].Name != ReviewJobName {
		t.Errorf("jobs[0].Name = %q, want %q", jobs[0].Name, ReviewJobName)
	}
	if jobs[0].Model != ReviewModel {
		t.Errorf("jobs[0].Model = %q, want %q", jobs[0].Model, ReviewModel)
	}
	if jobs[0].Effort != Effort {
		t.Errorf("jobs[0].Effort = %q, want %q", jobs[0].Effort, Effort)
	}
	if jobs[0].Prompt != ReviewPrompt("basesha", "headsha") {
		t.Error("jobs[0].Prompt does not match ReviewPrompt(basesha, headsha)")
	}

	// Second job should be the summary
	if jobs[1].Name != SummaryJobName {
		t.Errorf("jobs[1].Name = %q, want %q", jobs[1].Name, SummaryJobName)
	}
	if jobs[1].Model != SummaryModel {
		t.Errorf("jobs[1].Model = %q, want %q", jobs[1].Model, SummaryModel)
	}
	if jobs[1].Effort != Effort {
		t.Errorf("jobs[1].Effort = %q, want %q", jobs[1].Effort, Effort)
	}
	if jobs[1].Prompt != SummaryPrompt() {
		t.Error("jobs[1].Prompt does not match SummaryPrompt()")
	}
}

func TestFixJob(t *testing.T) {
	job := FixJob()

	if job.Name != FixJobName {
		t.Errorf("job.Name = %q, want %q", job.Name, FixJobName)
	}
	if job.Model != FixModel {
		t.Errorf("job.Model = %q, want %q", job.Model, FixModel)
	}
	if job.Effort != Effort {
		t.Errorf("job.Effort = %q, want %q", job.Effort, Effort)
	}
	if job.Prompt != FixPrompt() {
		t.Error("job.Prompt does not match FixPrompt()")
	}
}

func TestReviewJobPromptsForbidChangesToBranchState(t *testing.T) {
	for _, job := range ReviewJobs("base", "head") {
		if !strings.HasPrefix(job.Prompt, branchStateRule) {
			t.Errorf("%s prompt does not start with the branch-state rule", job.Name)
		}
	}
	if strings.Contains(FixPrompt(), branchStateRule) {
		t.Error("the fix prompt should be free to change the branch")
	}
}
