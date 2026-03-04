package domain

import "context"

// ReviewRequest encapsulates the parameters required to perform a code review.
// It specifies the branches to compare and whether strict mode is enabled.
type ReviewRequest struct {
	BaseBranch   string
	TargetBranch string
	StrictMode   bool
}

// CheckResult represents the outcome of a single deterministic check.
// It includes the check's name, whether it succeeded, and its output.
type CheckResult struct {
	Name    string
	Success bool
	Output  string
}

// ReviewReport represents the final, structured output from the LLM.
// It contains a summary of the review, the estimated severity of issues,
// and detailed feedback.
type ReviewReport struct {
	Summary  string
	Severity string
	Details  string
}

// GitProvider defines the external Git operations required by the application.
// It abstracts away the specific git CLI commands.
type GitProvider interface {
	// CurrentBranch returns the name of the currently checked-out branch.
	CurrentBranch(ctx context.Context) (string, error)
	// BranchExists verifies whether the specified branch exists in the repository.
	BranchExists(ctx context.Context, branch string) (bool, error)
	// Diff generates the unified diff between the base and target branches.
	// If baseBranch is empty, it diffs the target branch against the current state.
	Diff(ctx context.Context, baseBranch, targetBranch string) (string, error)
}

// StaticChecker defines the interface for running deterministic code quality checks.
// Examples include `go fmt`, `go vet`, or `go build`.
type StaticChecker interface {
	// RunChecks executes all configured deterministic checks and returns their results.
	RunChecks(ctx context.Context) ([]CheckResult, error)
}

// LLMClient defines the interface for interacting with the language model.
// It is responsible for semantic reasoning based on the provided diff.
type LLMClient interface {
	// Review prompts the LLM with the generated diff and coding standards,
	// returning a structured ReviewReport.
	Review(ctx context.Context, diff string, standards string) (*ReviewReport, error)
}
