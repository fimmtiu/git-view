package review

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

func ReviewPrompt(base, head string) string { panic("scaffold stub") }

func SummaryPrompt() string { panic("scaffold stub") }

func FixPrompt() string { panic("scaffold stub") }

// ReviewJobs returns the code review job, then the PR summary job.
func ReviewJobs(base, head string) []JobSpec { panic("scaffold stub") }

func FixJob() JobSpec { panic("scaffold stub") }
