package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"local_review/internal/app"
	"local_review/internal/domain"
	"local_review/internal/infra/checker"
	"local_review/internal/infra/git"
	"local_review/internal/infra/llm"

	"github.com/spf13/cobra"
)

// CLI configures and executes the command-line interface.
type CLI struct {
	out io.Writer
}

// NewCLI creates a new CLI instance.
func NewCLI(out io.Writer) *CLI {
	if out == nil {
		out = os.Stdout
	}
	return &CLI{out: out}
}

// Execute parses arguments and runs the root command.
func (c *CLI) Execute(ctx context.Context) error {
	var baseBranch string
	var doReview bool
	var groqKey string

	rootCmd := &cobra.Command{
		Use:   "lr --branch <base-branch> <target-branch>",
		Short: "Local branch-to-branch code review CLI",
		Long: `lr performs a local code review by diffing two git branches.
It executes deterministic checks (go fmt, go vet, etc.) before invoking an LLM.

Arguments:
  -b, --branch <base-branch>   the branch you are comparing FROM (required)
  <target-branch>              the branch you are comparing TO (required, positional)

Flags:
  -r, --review                 trigger a branch-to-branch review
  -k, --groq-key <api-key>     Groq API key (overrides GROQ_API_KEY env var)

Behaviour:
  If you are already on <base-branch>, lr runs:
    git diff <target-branch>

  Otherwise it runs:
    git diff <base-branch> <target-branch>`,
		Example: `  lr -b main feature/my-branch
  lr --branch develop feature/my-branch
  lr -r -b main feature/my-branch
  lr -r -k gsk_xxx -b main feature/my-branch
  GROQ_API_KEY=gsk_xxx lr -r -b main feature/my-branch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !doReview {
				fmt.Fprintln(c.out, "Use -r or --review to trigger a deterministic-first review pipeline.")
				return nil
			}

			// Resolve Groq API key: flag > env var
			resolvedKey := groqKey
			if resolvedKey == "" {
				resolvedKey = os.Getenv("GROQ_API_KEY")
			}
			if resolvedKey == "" {
				return fmt.Errorf(
					"Groq API key is required.\n" +
						"  Set it via the environment variable:  export GROQ_API_KEY=gsk_...\n" +
						"  Or pass it as a flag:                 lr -r -k gsk_... -b main feature/x",
				)
			}

			// Wire infrastructure
			gitClient := git.NewClient()
			checkRunner := checker.NewRunner()
			llmClient := llm.NewGroqClient(resolvedKey, "")

			// Build engine
			engine := app.NewReviewEngine(gitClient, checkRunner, llmClient, c.out)

			targetBranch := args[0]
			req := domain.ReviewRequest{
				BaseBranch:   baseBranch,
				TargetBranch: targetBranch,
				StrictMode:   true,
			}

			report, err := engine.Run(cmd.Context(), req)
			if err != nil {
				return err
			}

			if report != nil {
				fmt.Fprintln(c.out, "\n==============================================")
				fmt.Fprintln(c.out, "REVIEW REPORT")
				fmt.Fprintln(c.out, "==============================================")
				fmt.Fprintf(c.out, "Summary  : %s\n", report.Summary)
				fmt.Fprintf(c.out, "Severity : %s\n", report.Severity)
				fmt.Fprintln(c.out, "----------------------------------------------")
				fmt.Fprintln(c.out, report.Details)
				fmt.Fprintln(c.out, "==============================================")
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&baseBranch, "branch", "b", "", "Base branch to compare from (required)")
	rootCmd.Flags().BoolVarP(&doReview, "review", "r", false, "Trigger branch-to-branch review")
	rootCmd.Flags().StringVarP(&groqKey, "groq-key", "k", "", "Groq API key (overrides GROQ_API_KEY env var)")
	_ = rootCmd.MarkFlagRequired("branch")

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println(err)
		cmd.Println(cmd.UsageString())
		return fmt.Errorf("see help above")
	})

	return rootCmd.ExecuteContext(ctx)
}
