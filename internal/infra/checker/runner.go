package checker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"local_review/internal/domain"
)

// Runner orchestrates the execution of underlying static analysis tools.
// It acts as the infrastructure adapter for domain.StaticChecker.
type Runner struct{}

// NewRunner creates a new instance of the deterministic checker runner.
func NewRunner() *Runner {
	return &Runner{}
}

// RunChecks executes 'go fmt', 'go vet', and 'go build' sequentially.
// It returns a slice of domain.CheckResult for each validation step.
func (r *Runner) RunChecks(ctx context.Context) ([]domain.CheckResult, error) {
	checks := []struct {
		name string
		args []string
	}{
		{"go fmt", []string{"fmt", "./..."}},
		{"go vet", []string{"vet", "./..."}},
		// Using -o /dev/null ensures build validity without leaving binaries behind.
		{"go build", []string{"build", "-o", "/dev/null", "./..."}},
	}

	var results []domain.CheckResult

	for _, c := range checks {
		cmd := exec.CommandContext(ctx, "go", c.args...)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		err := cmd.Run()
		success := err == nil
		output := out.String()
		if !success {
			// Include stderr for failures to provide actionable feedback.
			output = fmt.Sprintf("%s\n%s", out.String(), stderr.String())
		}

		results = append(results, domain.CheckResult{
			Name:    c.name,
			Success: success,
			Output:  output,
		})
	}

	return results, nil
}
