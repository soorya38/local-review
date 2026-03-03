package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// INFRASTRUCTURE
// -----------------------------------------------------------------------------

// GitProvider defines the external operations required from a Git client.
type GitProvider interface {
	CurrentBranch(ctx context.Context) (string, error)
	Diff(ctx context.Context, baseBranch, targetBranch string) error
}

// GitClient provides a thin wrapper over the git executable.
// It is responsible only for executing git commands and streaming output.
type GitClient struct {
	outStream io.Writer
	errStream io.Writer
}

// NewGitClient creates a new GitClient with the given output streams.
func NewGitClient(out, err io.Writer) *GitClient {
	return &GitClient{
		outStream: out,
		errStream: err,
	}
}

// CurrentBranch returns the name of the currently checked out branch.
// It queries git rev-parse and trims whitespace from the output.
func (c *GitClient) CurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = c.errStream

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec git rev-parse failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// Diff executes a git diff command.
// An empty baseBranch indicates a diff against the target branch only.
func (c *GitClient) Diff(ctx context.Context, baseBranch, targetBranch string) error {
	args := []string{"diff"}
	if baseBranch != "" {
		args = append(args, baseBranch)
	}
	args = append(args, targetBranch)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = c.outStream
	cmd.Stderr = c.errStream

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec git diff failed: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// APPLICATION / DOMAIN
// -----------------------------------------------------------------------------

// ReviewApp orchestrates the code review process.
// It separates the business logic of branch comparison from the CLI and infrastructure.
type ReviewApp struct {
	git GitProvider
	out io.Writer
}

// NewReviewApp initializes a ReviewApp with its dependencies.
func NewReviewApp(git GitProvider, out io.Writer) *ReviewApp {
	return &ReviewApp{
		git: git,
		out: out,
	}
}

// RunReview performs the code review between baseBranch and targetBranch.
// It optimizes the git diff command if the current branch matches the base branch.
func (a *ReviewApp) RunReview(ctx context.Context, baseBranch, targetBranch string) error {
	fmt.Fprintf(a.out, "Comparing %s ↔ %s\n\n", baseBranch, targetBranch)

	currentBranch, err := a.git.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect current branch: %w", err)
	}

	diffBase := baseBranch
	if currentBranch == baseBranch {
		diffBase = ""
	}

	if err := a.git.Diff(ctx, diffBase, targetBranch); err != nil {
		return fmt.Errorf("failed to execute diff: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// CLI
// -----------------------------------------------------------------------------

// CLI configures and executes the command-line interface.
type CLI struct {
	app *ReviewApp
	out io.Writer
}

// NewCLI creates a new CLI instance injected with the application dependencies.
func NewCLI(app *ReviewApp, out io.Writer) *CLI {
	return &CLI{
		app: app,
		out: out,
	}
}

// Execute parses arguments and runs the root command.
func (c *CLI) Execute(ctx context.Context) error {
	var baseBranch string
	var doReview bool

	rootCmd := &cobra.Command{
		Use:   "lr --branch <base-branch> <target-branch>",
		Short: "Local branch-to-branch code review CLI",
		Long: `lr performs a local code review by diffing two git branches.

Arguments:
  -b, --branch <base-branch>   the branch you are comparing FROM (required)
  <target-branch>              the branch you are comparing TO (required, positional)

Flags:
  -r, --review                 trigger a branch-to-branch review

Behaviour:
  If you are already on <base-branch>, lr runs:
    git diff <target-branch>

  Otherwise it runs:
    git diff <base-branch> <target-branch>`,
		Example: `  lr -b main feature/my-branch
  lr --branch develop feature/my-branch
  lr -r -b main feature/my-branch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !doReview {
				fmt.Fprintln(c.out, "Use -r or --review to trigger review")
				return nil
			}

			targetBranch := args[0]
			return c.app.RunReview(cmd.Context(), baseBranch, targetBranch)
		},
	}

	rootCmd.Flags().StringVarP(&baseBranch, "branch", "b", "", "Base branch to compare from (required)")
	rootCmd.Flags().BoolVarP(&doReview, "review", "r", false, "Trigger branch-to-branch review")
	_ = rootCmd.MarkFlagRequired("branch")

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.Println(err)
		cmd.Println(cmd.UsageString())
		return fmt.Errorf("see help above")
	})

	return rootCmd.ExecuteContext(ctx)
}

func main() {
	ctx := context.Background()
	out := os.Stdout
	errOut := os.Stderr

	gitClient := NewGitClient(out, errOut)
	app := NewReviewApp(gitClient, out)
	cli := NewCLI(app, out)

	if err := cli.Execute(ctx); err != nil {
		log.Fatal(err)
	}
}
