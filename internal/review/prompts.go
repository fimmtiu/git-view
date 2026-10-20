package review

import "strings"

const (
	ReviewModel  = "sonnet"
	SummaryModel = "claude-opus-5-5"
	FixModel     = "claude-opus-5-5"
	Effort       = "high"
)

const (
	ReviewJobName  = "Code review"
	SummaryJobName = "PR summary"
	FixJobName     = "Fixes"
)

// The review jobs and their sub-agents share one checkout at the same time, so
// any change to the branch by one agent breaks the others.
const branchStateRule = "IMPORTANT: Other agents are working in this same checkout at the same time. You MUST NOT change the state of the git branch: do NOT run `git checkout`, `git switch`, `git stash`, `git reset`, `git restore`, `git rebase`, `git merge`, `git pull`, `git cherry-pick`, or `git commit --amend`, and do NOT create, delete, or rename branches. Doing any of these will interfere with the other running agents. If you spawn sub-agents, include this rule in their instructions.\n"

const reviewPromptTemplate = "Do an adversarial code review of the current branch. This is a read-only review; do NOT push to GitHub or modify existing commits.\n" +
	"Base commit: <base>\n" +
	"HEAD commit: <head>\n" +
	"\n" +
	"Setup:\n" +
	"- Use the GitHub MCP tools (`mcp__github__*`, load via ToolSearch if needed) to check for PR metadata: title, body, head SHA, base, CI status, and comments. The branch may not have been pushed or have an open PR yet; these are useful optional context, not required fields.\n" +
	"- Read any relevant `.claude/skills/*/SKILL.md` files for context.\n" +
	"- Read all of the code changes between the branch's base commit and the HEAD commit.\n" +
	"\n" +
	"Spawn a pool of six Sonnet sub-agents to examine each of the following aspects of the code changes in parallel:\n" +
	"\n" +
	"### Design\n" +
	"- Is this the best approach to accomplish what the code is trying to do?\n" +
	"- Is this code sustainable? Is it a design that can easily grow over time, or will it lock us into a potentially problematic pattern as we scale and the code evolves?\n" +
	"- Suggest better solutions if any exist.\n" +
	"\n" +
	"### Abstraction\n" +
	"- Is this the right place for this change to live?\n" +
	"- Does this code own the data that it's modifying?\n" +
	"- If we're passing parameters, would it make more sense for them to be member data of the object that's being operated on instead?\n" +
	"- Does this change increase the number of files we'll have to touch the next time this functionality has to change?\n" +
	"- Is there a deeper structural change we could make that would make this change simpler?\n" +
	"\n" +
	"### Code quality\n" +
	"- Unclear code, where the intent isn't obvious from the code itself and no comments explain it\n" +
	"- Useless comments that just explain what the code is doing without explaining why\n" +
	"- Misspellings in variable names or comments\n" +
	"- Ignoring established patterns that already exist in the codebase\n" +
	"- Places where necessary error handling is missing, or errors are being unnecessarily suppressed\n" +
	"- Stale comments, docstrings, or skills that no longer accurately describe the code\n" +
	"\n" +
	"### Performance\n" +
	"- Inefficient procedures (O(n^2) algorithms, duplicated work, etc.)\n" +
	"- Database queries that are likely to be problematic\n" +
	"- Excessive queries to databases or APIs which could be batched into fewer queries\n" +
	"- Memory leaks, or large memory allocations that don't get freed by garbage collection\n" +
	"- Inefficient, money-wasting use of LLM tokens or cloud resources\n" +
	"- Anything else that could be unexpectedly slow or resource-intensive\n" +
	"\n" +
	"### Security\n" +
	"- Do the changes potentially introduce any security holes that an attacker could use to leak data, impersonate users, escalate privileges, run untrusted commands, or cause any other unwanted security violation?\n" +
	"- You MUST include an explanation of why it worries you and a description of how an attacker could make use of it. Only report an issue if you can describe a concrete, realistic attack: what specific input or action the attacker takes, what system boundary they cross, and what they gain. Speculative chains of events (\"could potentially lead to...\") do not qualify.\n" +
	"- Code quality concerns, error handling gaps, and reliability issues are NOT security findings unless they directly enable a violation with no additional assumptions.\n" +
	"- For each candidate finding, argue against it: what would have to be true for this to actually be exploitable? If the preconditions are implausible or outside an attacker's control, discard it.\n" +
	"\n" +
	"### Testing\n" +
	"- Are any important pieces of logic untested?\n" +
	"- Would any tests fail if the code were reverted?\n" +
	"- Are any of the pre-existing tests no longer relevant?\n" +
	"- Are any of the tests brittle, or don't provide enough protection to be worthwhile?\n" +
	"\n" +
	"Instructions for each sub-agent: \"Hunt adversarially. Try to break the code. VERIFY every candidate issue before including it. Cite file:line at the PR head, quote the code or prompt text, and drop anything you can't substantiate. Classify each finding as Blocker, Should Fix, or Nit based on severity. Each one gets a title, a file:line link (https://github.com/<owner>/<repo>/blob/<headsha>/path#Lnn), evidence, a concrete failing example where relevant, and a suggested fix.\"\n" +
	"\n" +
	"Deliverable: write `.gv-review/code-review-findings.md` containing:\n" +
	"- Title, PR link (if any), head SHA, base SHA, today's date in `YYYY-MM-DD` format, and a one-line scope.\n" +
	"- A one-line verdict, plus counts of Blockers / Should Fix / Nits.\n" +
	"- Findings grouped by severity as Blockers, Should Fix, and Nits.\n" +
	"Keep it tight. Do not put any model names in the file. If the file already exists, replace it — the pre-existing contents are no longer useful.\n"

const summaryPromptText = "Generate a detailed Markdown summary of the code changes on this branch with the following sections:\n" +
	"\n" +
	"- Purpose\n" +
	"- How it works\n" +
	"- Code changes\n" +
	"  - For each changed file that isn't a test, include:\n" +
	"    - File name, hyperlinked to GitHub if the GitHub branch is up to date\n" +
	"    - Change size (e.g., `(+5,–6)`)\n" +
	"    - A description of what the change does\n" +
	"- Test changes\n" +
	"  - For each changed test file, include:\n" +
	"    - File name, hyperlinked to GitHub if the GitHub branch is up to date\n" +
	"    - Change size (e.g., `(+5,–6)`)\n" +
	"    - A description of what the change does\n" +
	"\n" +
	"This is meant to be a high-level summary to save time for reviewers, so be brief.\n" +
	"\n" +
	"Write the output to `.gv-review/summary.md`. If the file already exists, replace it — the pre-existing contents are no longer useful.\n"

const fixPromptText = "Read `.gv-review/code-review-findings.md` and fix all of the issues described therein. Commit each fix separately, but group together closely related fixes where possible to minimize the number of commits. You are permitted to push back and ignore an issue if you can provide a well-supported justification for why it's incorrect.\n" +
	"\n" +
	"When all issues are fixed, append a new section to the bottom of that file which explains for each issue what you fixed, or why you pushed back.\n"

func ReviewPrompt(base, head string) string {
	s := branchStateRule + "\n" + reviewPromptTemplate
	s = strings.ReplaceAll(s, "<base>", base)
	s = strings.ReplaceAll(s, "<head>", head)
	s = strings.ReplaceAll(s, "<headsha>", head)
	return s
}

func SummaryPrompt() string {
	return branchStateRule + "\n" + summaryPromptText
}

func FixPrompt() string {
	return fixPromptText
}

// ReviewJobs returns the code review job, then the PR summary job.
func ReviewJobs(base, head string) []JobSpec {
	return []JobSpec{
		{
			Name:   ReviewJobName,
			Model:  ReviewModel,
			Effort: Effort,
			Prompt: ReviewPrompt(base, head),
		},
		{
			Name:   SummaryJobName,
			Model:  SummaryModel,
			Effort: Effort,
			Prompt: SummaryPrompt(),
		},
	}
}

func FixJob() JobSpec {
	return JobSpec{
		Name:   FixJobName,
		Model:  FixModel,
		Effort: Effort,
		Prompt: FixPrompt(),
	}
}
